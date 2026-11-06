package score

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

type tick struct {
	driver     *Driver
	out        Output
	feedback   DACFeedback
	hooks      *ObservedHooks
	err        error
	operations uint32
	position   EventPosition
	eventKind  string
	parserPlan *parserPlan
}

func (d *Driver) Advance(input Input) (Output, error) {
	if input.SoundID != 0 && !supported(input.SoundID) {
		return Output{}, fmt.Errorf("unrestored dispatch %02x", input.SoundID)
	}
	if input.DAC.Priority&0x3f != 0 {
		return Output{}, fmt.Errorf("invalid DAC priority")
	}
	return d.advance(input.SoundID, input.DAC, nil)
}

func (d *Driver) advance(soundID uint8, feedback DACFeedback, hooks *ObservedHooks) (Output, error) {
	if soundID != 0 && !supported(soundID) {
		return Output{}, fmt.Errorf("unrestored dispatch %02x", soundID)
	}
	tx, err := d.beginTransaction(hooks, feedback, stageBegun)
	if err != nil {
		return Output{}, err
	}
	if err = tx.AdvanceTempo(); err != nil {
		return Output{}, err
	}
	if err = tx.Dispatch(soundID); err != nil {
		return Output{}, err
	}
	if err = tx.AdvanceVoices(); err != nil {
		return Output{}, err
	}
	return tx.Finish()
}

func (t *tick) emit(intent Intent) {
	t.emitFor(-1, intent)
}
func (t *tick) emitFor(slot int, intent Intent) {
	if t.err != nil {
		return
	}
	intent.Order = len(t.out.Intents)
	kind := OpChipWrite
	if intent.Kind == "queue" {
		kind = OpQueueReplace
	}
	t.operation(Operation{Kind: kind, VoiceSlot: int8(slot), OtherSlot: -1, Writes: [2]Intent{intent}, WriteCount: 1})
	if t.err != nil {
		return
	}
	t.out.Intents = append(t.out.Intents, intent)
}
func (t *tick) ym(bank, reg, value uint8) {
	t.ymFor(-1, bank, reg, value)
}
func (t *tick) ymFor(slot int, bank, reg, value uint8) {
	// These masks describe individual hardware-ignored fields. Request count,
	// bank, register and order remain unmodified and are all compared to source.
	switch reg & 0xf0 {
	case 0x30:
		value &= 0x7f
	case 0x40:
		value &= 0x7f
	case 0x50:
		value &= 0xdf
	case 0x60:
		value &= 0x9f
	case 0x70:
		value &= 0x1f
	}
	if reg >= 0xb0 && reg <= 0xb2 {
		value &= 0x3f
	}
	if reg >= 0xb4 && reg <= 0xb6 {
		value &= 0xf7
	}
	if reg >= 0xa4 && reg <= 0xae {
		value &= 0x3f
	}
	t.emitFor(slot, Intent{Kind: "ym", Bank: bank, Register: reg, Value: value})
}
func (t *tick) psgFor(slot int, value uint8) { t.emitFor(slot, Intent{Kind: "psg", Value: value}) }
func (t *tick) psg(value uint8)              { t.psgFor(-1, value) }
func (t *tick) psgPitch(slot int, latch, data uint8) {
	if t.err != nil {
		return
	}
	offset := len(t.out.Intents)
	writes := [2]Intent{{Order: offset, Kind: "psg", Value: latch}, {Order: offset + 1, Kind: "psg", Value: data}}
	t.operation(Operation{Kind: OpPSGPitchWrite, Routine: RoutinePitch, VoiceSlot: int8(slot), OtherSlot: -1, Writes: writes, WriteCount: 2})
	if t.err == nil {
		t.out.Intents = append(t.out.Intents, writes[:]...)
	}
}
func channelCode(name string) uint8 {
	switch name {
	case "fm2":
		return 1
	case "fm3":
		return 2
	case "fm4":
		return 4
	case "fm5":
		return 5
	case "fm6", "dac":
		return 6
	case "psg1":
		return 0x80
	case "psg2":
		return 0xa0
	case "psg3":
		return 0xc0
	case "psg-noise":
		return 0xe0
	}
	return 0
}
func (t *tick) voiceYM(slot int, reg, value uint8) {
	v := &t.driver.state.Voices[slot]
	channel := channelCode(v.Channel)
	bank := uint8(0)
	if channel&4 != 0 {
		bank = 1
	}
	t.ymFor(slot, bank, reg+(channel&3), value)
}
func (t *tick) keyOff(i int) {
	defer t.enter(RoutineKey, i, BranchKeyOff)()
	v := &t.driver.state.Voices[i]
	if v.Claimed {
		t.operation(Operation{Kind: OpParameter, Routine: RoutineKey, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchClaimed})
		return
	}
	if isPSG(i) {
		t.psgFor(i, channelCode(v.Channel)|0x1f)
	} else if !v.Tie {
		t.ymFor(i, 0, 0x28, channelCode(v.Channel))
	} else {
		t.operation(Operation{Kind: OpParameter, Routine: RoutineKey, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchTied})
	}
}
func (t *tick) keyOn(i int) {
	defer t.enter(RoutineKey, i, BranchKeyOn)()
	v := &t.driver.state.Voices[i]
	if !v.NoteOff && !v.Claimed {
		t.ymFor(i, 0, 0x28, channelCode(v.Channel)|0xf0)
	} else {
		branch := BranchNoteOff
		if v.Claimed {
			branch = BranchClaimed
		}
		t.operation(Operation{Kind: OpParameter, Routine: RoutineKey, VoiceSlot: int8(i), OtherSlot: -1, Branch: branch})
	}
}

func (t *tick) initialize(slot int, s definition, v voice) {
	defer t.enter(RoutineInitialize, slot, BranchInitialLoad)()
	bytes := uint16(4)
	if isPSG(slot) {
		bytes = 6
	}
	t.operation(Operation{Kind: OpParameter, Routine: RoutineInitialize, VoiceSlot: int8(slot), OtherSlot: -1, SoundID: s.ID, EventKind: "voice-header", Iterations: bytes})
	if t.err != nil {
		return
	}
	state := VoiceState{Owned: true, Owner: Owner{s.ID, v.ID}, Channel: v.Channel, Active: true, Transpose: v.Initial.Transpose, BaseLevel: v.Initial.Level, Tone: v.Initial.PatchReference, VolumeEnvelope: v.Initial.VolumeEnvelope, PitchControl: PitchControl{uint16(v.Initial.PitchControl & 127), v.Initial.PitchControl&128 != 0}, NoteWait: 1, PanDefinition: param.PanDefinition{Mode: param.PanDisabled}}
	if !isPSG(slot) {
		state.Pan.Left = true
		state.Pan.Right = true
	}
	t.driver.state.Voices[slot] = state
}
func (t *tick) startMusic(s definition) error {
	// Primary stop, secondary stop and music reset are distinct ordered phases.
	leavePrimary := t.enter(RoutineStopPrimary, -1, BranchStopPrimary)
	t.priority(OpPriorityClear, -1, BranchStopPrimary)
	t.ym(0, 0x27, 0)
	for i := 10; i < 16; i++ {
		t.operation(Operation{Kind: OpVoiceScan, Routine: RoutineStopPrimary, VoiceSlot: int8(i), OtherSlot: -1})
		if !t.driver.state.Voices[i].Active {
			continue
		}
		t.driver.state.Voices[i].Active = false
		t.keyOff(i)
		if err := t.restoreVoice(i, true); err != nil {
			return err
		}
		if t.err != nil {
			return t.err
		}
	}
	leavePrimary()
	leaveSecondary := t.enter(RoutineStopSecondary, -1, BranchSecondary)
	for i := 16; i < 18; i++ {
		t.operation(Operation{Kind: OpVoiceScan, Routine: RoutineStopSecondary, VoiceSlot: int8(i), OtherSlot: -1})
		if t.driver.state.Voices[i].Active {
			return fmt.Errorf("secondary stop is outside restored profile")
		}
	}
	leaveSecondary()
	leaveReset := t.enter(RoutineMusicReset, -1, BranchMusicReset)
	t.ym(0, 0x27, 0)
	t.priority(OpPrioritySave, -1, BranchMusicReset)
	t.operation(Operation{Kind: OpHeaderClear, Routine: RoutineMusicReset, VoiceSlot: -1, OtherSlot: -1, Branch: BranchMusicReset, Iterations: 16})
	if t.err != nil {
		return t.err
	}
	for i := 0; i < 10; i++ {
		t.driver.state.Voices[i] = VoiceState{}
	}
	t.driver.state.TempoPeriod = 0
	t.driver.state.TempoRemaining = 0
	t.priority(OpPriorityRestore, -1, BranchMusicReset)
	t.operation(Operation{Kind: OpSelectedReset, Routine: RoutineMusicReset, VoiceSlot: -1, OtherSlot: -1, Branch: BranchMusicReset, SoundID: 0xff})
	leaveReset()
	t.operation(Operation{Kind: OpParameter, Routine: RoutineInitialize, VoiceSlot: -1, OtherSlot: -1, SoundID: s.ID, EventKind: "music-header", Iterations: 6})
	t.driver.state.TempoPeriod = s.InitialTempo
	t.driver.state.TempoRemaining = s.InitialTempo
	for _, v := range s.Voices {
		slot, _ := slotFor(v.Channel, true)
		t.initialize(slot, s, v)
	}
	for i := 1; i <= 6; i++ {
		t.keyOff(i)
	}
	for i := 7; i <= 9; i++ {
		t.keyOff(i)
	}
	if !t.driver.state.Voices[9].Claimed {
		t.psgFor(9, 0xff)
	}
	return t.err
}
func musicSlot(i int) int {
	switch i {
	case 10:
		return 3
	case 11:
		return 4
	case 12:
		return 5
	case 13:
		return 7
	case 14:
		return 8
	case 15:
		return 9
	}
	return -1
}
func (t *tick) startEffect(s definition) error {
	t.operation(Operation{Kind: OpParameter, Routine: RoutineInitialize, VoiceSlot: -1, OtherSlot: -1, SoundID: s.ID, EventKind: "primary-header", Iterations: 4})
	for _, v := range s.Voices {
		i, err := slotFor(v.Channel, false)
		if err != nil {
			return err
		}
		target := musicSlot(i)
		t.operation(Operation{Kind: OpClaim, Routine: RoutineOwnership, VoiceSlot: int8(target), OtherSlot: int8(i), Branch: BranchPrimary, SoundID: s.ID})
		if t.err != nil {
			return t.err
		}
		t.driver.state.Voices[target].Claimed = true
		if v.Channel == "psg3" {
			t.psgFor(i, 0xdf)
			t.psgFor(i, 0xff)
		}
		t.initialize(i, s, v)
	}
	if t.driver.state.Voices[11].Active {
		t.operation(Operation{Kind: OpClaim, Routine: RoutineOwnership, VoiceSlot: 16, OtherSlot: 11, Branch: BranchSecondary})
		t.driver.state.Voices[16].Claimed = true
	}
	if t.driver.state.Voices[15].Active {
		t.operation(Operation{Kind: OpClaim, Routine: RoutineOwnership, VoiceSlot: 17, OtherSlot: 15, Branch: BranchSecondary})
		t.driver.state.Voices[17].Claimed = true
	}
	return t.err
}
func (t *tick) restoreVoice(i int, stoppingAll bool) error {
	defer t.enter(RoutineOwnership, i, BranchTrackStop)()
	target := musicSlot(i)
	// The all-primary stop restores the shared third PSG lane through its
	// secondary slot, even when inactive. Ordinary track-end uses musicSlot.
	if stoppingAll && isPSG(i) && (t.driver.state.Voices[i].Channel == "psg3" || t.driver.state.Voices[i].Channel == "psg-noise") {
		target = 17
	}
	if target < 0 {
		return fmt.Errorf("unsupported restore domain")
	}
	v := &t.driver.state.Voices[target]
	if !isPSG(i) {
		if !stoppingAll && !v.Active {
			t.operation(Operation{Kind: OpParameter, Routine: RoutineOwnership, VoiceSlot: int8(target), OtherSlot: int8(i), Branch: BranchInactive})
			return nil
		}
		t.operation(Operation{Kind: OpRestore, Routine: RoutineOwnership, VoiceSlot: int8(target), OtherSlot: int8(i), Branch: BranchRest})
		if t.err != nil {
			return t.err
		}
		v.Claimed = false
		v.NoteOff = true
		return t.loadPatch(target)
	}
	t.operation(Operation{Kind: OpRestore, Routine: RoutineOwnership, VoiceSlot: int8(target), OtherSlot: int8(i), Branch: BranchRest})
	if t.err != nil {
		return t.err
	}
	v.Claimed = false
	v.NoteOff = true
	if v.Channel == "psg-noise" {
		t.psgFor(target, v.NoiseControl)
	}
	return t.err
}

func (t *tick) nextEvent(v *VoiceState) (event, error) {
	s, ok := t.driver.catalog.scores[v.Owner.ScoreID]
	if !ok || v.Owner.VoiceID < 0 || v.Owner.VoiceID >= len(s.Voices) {
		return event{}, fmt.Errorf("invalid event owner")
	}
	timeline := s.Voices[v.Owner.VoiceID].Timeline
	entry := EntryContinue
	for n := 0; n < 2; n++ {
		events := timeline.Prefix
		if v.Cycle {
			events = timeline.Cycle
		}
		if v.EventIndex < len(events) {
			e := events[v.EventIndex]
			t.position = EventPosition{Valid: true, Owner: v.Owner, Cycle: v.Cycle, Index: v.EventIndex, Entry: entry}
			t.eventKind = e.Kind
			t.parserPlan = nil
			if t.hooks != nil && t.hooks.Parser != nil {
				plan, err := t.hooks.Parser.plan(t.position)
				if err != nil {
					return event{}, err
				}
				t.parserPlan = plan
				t.parserSite(SitePrelude)
			} else {
				t.operation(Operation{Kind: OpParserUnavailable, Routine: RoutineParser, VoiceSlot: int8(mustSlot(s, v.Owner.VoiceID)), OtherSlot: -1, Position: t.position, EventKind: e.Kind})
			}
			if t.err != nil {
				return event{}, t.err
			}
			v.EventIndex++
			t.operation(Operation{Kind: OpEvent, Routine: RoutineParser, VoiceSlot: int8(mustSlot(s, v.Owner.VoiceID)), OtherSlot: -1, Position: t.position, EventKind: e.Kind, Duration: uint16(e.Duration)})
			return e, t.err
		}
		if len(timeline.Cycle) == 0 {
			return event{}, fmt.Errorf("event cursor passed finite end")
		}
		entry = EntryFromPrefix
		if v.Cycle {
			entry = EntryCycleWrap
		}
		v.Cycle = true
		v.EventIndex = 0
	}
	return event{}, fmt.Errorf("empty event cycle")
}
func mustSlot(s definition, voice int) int {
	slot, _ := slotFor(s.Voices[voice].Channel, s.Kind == "music")
	return slot
}
func (t *tick) advanceVoice(i int) (bool, error) {
	defer t.enter(RoutineVoice, i, BranchActive)()
	v := &t.driver.state.Voices[i]
	t.operation(Operation{Kind: OpCounter, Routine: RoutineVoice, VoiceSlot: int8(i), OtherSlot: -1, Counter: CounterNoteWait, Before: uint16(v.NoteWait), After: uint16(v.NoteWait - 1), Branch: BranchStep})
	if t.err != nil {
		return false, t.err
	}
	v.NoteWait--
	if v.NoteWait == 0 {
		t.operation(Operation{Kind: OpParameter, Routine: RoutineVoice, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchNoteBoundary})
		if i != 0 {
			v.Tie = false
			v.NoteOff = false
		}
		for events := 0; events < 100000; events++ {
			e, err := t.nextEvent(v)
			if err != nil {
				return false, err
			}
			if e.Kind == "note" {
				if err := t.note(i, e); err != nil {
					return false, err
				}
				if i == 0 {
					return false, t.err
				}
				if err := t.pitch(i, true); err != nil {
					return false, err
				}
				if isPSG(i) {
					return false, t.volume(i)
				}
				if err := t.pan(i, true); err != nil {
					return false, err
				}
				t.keyOn(i)
				return false, t.err
			}
			if e.Kind == "track-end" {
				branch := BranchTrackStop
				if i == 0 {
					branch = BranchDACTrackStop
				}
				t.operation(Operation{Kind: OpParameter, Routine: RoutineVoice, VoiceSlot: int8(i), OtherSlot: -1, Branch: branch})
				v.Active = false
				v.Tie = false
				if i != 0 {
					t.keyOff(i)
				}
				if i >= 10 {
					t.priority(OpPriorityClear, i, BranchPrimaryTrackStop)
					if err := t.restoreVoice(i, false); err != nil {
						return false, err
					}
					if i == 10 {
						t.ym(0, 0x27, 0)
					}
				}
				return e.EndsDriverTick, t.err
			}
			if err := t.control(i, e); err != nil {
				return false, err
			}
		}
		return false, fmt.Errorf("same-tick event budget exhausted")
	}
	if i == 0 {
		return false, t.err
	}
	t.operation(Operation{Kind: OpParameter, Routine: RoutineVoice, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchHeldNote})
	if v.GateRemaining != 0 {
		t.operation(Operation{Kind: OpCounter, Routine: RoutineGate, VoiceSlot: int8(i), OtherSlot: -1, Counter: CounterGate, Before: uint16(v.GateRemaining), After: uint16(v.GateRemaining - 1), Branch: BranchStep})
		v.GateRemaining--
		if v.GateRemaining == 0 {
			t.operation(Operation{Kind: OpParameter, Routine: RoutineGate, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchGateExpired})
			v.NoteOff = true
			t.keyOff(i)
			return false, t.err
		}
	}
	if isPSG(i) {
		if v.VolumeEnvelope != 0 {
			if err := t.volume(i); err != nil {
				return false, err
			}
		}
	} else {
		if err := t.pan(i, false); err != nil {
			return false, err
		}
	}
	t.modulate(i)
	return false, t.pitch(i, false)
}

func (t *tick) control(i int, e event) error {
	defer t.enter(RoutineControl, i, BranchNormal)()
	projection := ControlProjection{ImmediateFM: e.ImmediateFM, PanDisabled: e.Kind == "pan-automation" && e.Definition != nil && e.Definition.Mode == param.PanDisabled, Enabled: e.Enabled}
	if e.Kind == "psg-noise" {
		projection.Noise = 0xe0 | e.Clock
		if e.White {
			projection.Noise |= 4
		}
	}
	t.operation(Operation{Kind: OpParameter, Routine: RoutineControl, VoiceSlot: int8(i), OtherSlot: -1, EventKind: e.Kind, Control: projection})
	if t.err != nil {
		return t.err
	}
	v := &t.driver.state.Voices[i]
	switch e.Kind {
	case "pan":
		if !isPSG(i) {
			v.Pan.Left = e.Left
			v.Pan.Right = e.Right
			t.writePan(i)
		}
	case "detune":
		v.Detune = e.Value
	case "pan-automation":
		if e.Definition == nil {
			return fmt.Errorf("missing automation")
		}
		v.PanDefinition = *e.Definition
		if v.PanDefinition.Mode == param.PanDisabled {
			v.PanState = param.PanState{}
			t.writePan(i)
		} else {
			v.PanState = param.InitialPanState(v.PanDefinition)
		}
	case "level-add":
		v.BaseLevel += uint8(e.Delta)
		if !isPSG(i) && e.ImmediateFM {
			return t.level(i)
		}
	case "tie":
		v.Tie = true
	case "gate":
		v.GateTicks = e.Ticks
		v.GateRemaining = e.Ticks
	case "tempo":
		t.driver.state.TempoPeriod = e.Period
		t.driver.state.TempoRemaining = e.Period
	case "enqueue-replace-first":
		t.emitFor(i, Intent{Kind: "queue", Action: "replace-first", SoundID: e.SoundID})
	case "fm-patch":
		v.Tone = e.PatchID
		return t.loadPatch(i)
	case "fm-patch-select":
		v.Tone = e.PatchID
	case "modulation":
		v.HasModulation = true
		v.Modulation = Modulation{e.Delay, e.Rate, e.Delta, e.Steps}
		v.PitchControl.ModulationEnabled = true
		t.resetModulation(i, BranchInitialLoad)
	case "modulation-enable":
		v.PitchControl.ModulationEnabled = e.Enabled
	case "psg-noise":
		v.Channel = "psg-noise"
		v.NoiseControl = projection.Noise
		if !v.Claimed {
			t.psgFor(i, v.NoiseControl)
		}
	case "volume-envelope":
		v.VolumeEnvelope = e.EnvelopeID
	case "pitch-control":
		v.PitchControl = PitchControl{e.EnvelopeID, e.ModulationEnabled}
	case "transpose-add":
		v.Transpose = int8(uint8(v.Transpose) + uint8(e.Semitones))
	case "all-key-off":
		for _, slot := range []uint8{0x80, 0x84, 0x88, 0x8c} {
			t.voiceYM(i, slot, 0x0f)
		}
	default:
		return fmt.Errorf("unsupported control %q", e.Kind)
	}
	return t.err
}

func (t *tick) note(i int, e event) error {
	defer t.enter(RoutineNote, i, BranchNoteBoundary)()
	v := &t.driver.state.Voices[i]
	if isFM(i) {
		t.keyOff(i)
	}
	t.parserSite(SitePitch)
	if t.err != nil {
		return t.err
	}
	switch e.Pitch.Kind {
	case "rest":
		if i == 0 {
			v.DACDescriptor = ""
		} else if isPSG(i) {
			v.NoteOff = true
			v.BasePitch = 0xffff
			t.cleanup(i, BranchEarlyCleanup)
			t.keyOff(i)
		} else {
			v.NoteOff = true
			v.BasePitch = 0
			t.cleanup(i, BranchEarlyCleanup)
		}
	case "dac":
		v.DACDescriptor = e.Pitch.DescriptorID
	case "chromatic":
		if isPSG(i) {
			index := (e.Pitch.Semitone + int(uint8(v.Transpose))) & 127
			value, ok := t.driver.catalog.psg[index]
			if !ok {
				return fmt.Errorf("PSG pitch not recovered")
			}
			v.BasePitch = value
			t.operation(Operation{Kind: OpParameter, Routine: RoutineNote, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchChromatic, PitchIndex: uint8(index), Duration: uint16(e.Duration)})
			t.cleanup(i, BranchEarlyCleanup)
		} else {
			index := (e.Pitch.Semitone + 1 + int(uint8(v.Transpose))) & 127
			v.BasePitch = t.driver.catalog.fm[index%12] | uint16((index/12)&7)<<11
			t.operation(Operation{Kind: OpParameter, Routine: RoutineNote, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchChromatic, PitchIndex: uint8(index), Duration: uint16(e.Duration)})
		}
	case "previous":
		t.operation(Operation{Kind: OpParameter, Routine: RoutineNote, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchPrevious, Duration: uint16(e.Duration)})
	default:
		return fmt.Errorf("unrestored note")
	}
	t.parserSite(SiteDuration)
	t.operation(Operation{Kind: OpCounter, Routine: RoutineNote, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchReload, Counter: CounterNoteWait, Before: uint16(v.NoteWait), After: uint16(uint8(e.Duration)), Duration: uint16(e.Duration)})
	if t.err != nil {
		return t.err
	}
	v.NoteWait = uint8(e.Duration)
	if i == 0 {
		t.parserSite(SiteFinalCommit)
		if v.DACDescriptor != "" {
			descriptor := t.driver.catalog.descriptors[v.DACDescriptor]
			feedback, err := t.readDAC(FeedbackRead{Kind: ReadBlockedPriority, VoiceSlot: int8(i), DescriptorID: v.DACDescriptor, DescriptorPriority: descriptor.Flags & 0xc0, Pan: Stereo{v.Pan.Left, v.Pan.Right}, TrackPan: v.Pan, WritePan: v.Pan})
			if err != nil {
				return err
			}
			if !feedback.Blocked && descriptor.Flags&0xc0 >= feedback.Priority {
				t.emitFor(i, Intent{Kind: "dac", DescriptorID: v.DACDescriptor, Pan: &Stereo{v.Pan.Left, v.Pan.Right}})
			}
		}
		return t.err
	}
	t.cleanup(i, BranchFinalCleanup)
	return t.err
}

func (t *tick) cleanup(i int, branch Branch) {
	v := &t.driver.state.Voices[i]
	site := SiteFinalCommit
	if branch == BranchEarlyCleanup {
		site = SiteEarlyCommit
	}
	t.parserSite(site)
	t.operation(Operation{Kind: OpNoteCleanup, Routine: RoutineNote, VoiceSlot: int8(i), OtherSlot: -1, Branch: branch})
	if t.err != nil {
		return
	}
	if !v.Tie {
		v.GateRemaining = v.GateTicks
		v.VolumeCursor = param.EnvelopeCursor{}
		v.PitchCursor = param.EnvelopeCursor{}
		v.PitchScale = 0
		if v.PitchControl.ModulationEnabled {
			t.resetModulation(i, BranchReload)
		}
	} else {
		t.operation(Operation{Kind: OpParameter, Routine: RoutineNote, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchTied})
	}
}

func (t *tick) resetModulation(i int, branch Branch) {
	v := &t.driver.state.Voices[i]
	t.operation(Operation{Kind: OpParameter, Routine: RoutineModulation, VoiceSlot: int8(i), OtherSlot: -1, Branch: branch, EventKind: "definition-setup", Iterations: 4})
	if t.err != nil {
		return
	}
	v.ModDelay = v.Modulation.Delay
	v.ModRate = v.Modulation.Rate
	v.ModDelta = v.Modulation.Delta
	v.ModSteps = v.Modulation.Steps >> 1
	v.ModOffset = 0
}
func (t *tick) modulate(i int) {
	defer t.enter(RoutineModulation, i, BranchHeldNote)()
	v := &t.driver.state.Voices[i]
	if !v.PitchControl.ModulationEnabled {
		t.operation(Operation{Kind: OpParameter, Routine: RoutineModulation, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchDisabled})
		return
	}
	if v.ModDelay != 0 {
		t.operation(Operation{Kind: OpCounter, Routine: RoutineModulation, VoiceSlot: int8(i), OtherSlot: -1, Counter: CounterModDelay, Before: uint16(v.ModDelay), After: uint16(v.ModDelay - 1), Branch: BranchDelay})
		v.ModDelay--
		return
	}
	t.operation(Operation{Kind: OpCounter, Routine: RoutineModulation, VoiceSlot: int8(i), OtherSlot: -1, Counter: CounterModRate, Before: uint16(v.ModRate), After: uint16(v.ModRate - 1), Branch: BranchRate})
	v.ModRate--
	if v.ModRate != 0 {
		return
	}
	v.ModRate = v.Modulation.Rate
	if v.ModSteps == 0 {
		t.operation(Operation{Kind: OpParameter, Routine: RoutineModulation, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchReverse})
		v.ModSteps = v.Modulation.Steps
		v.ModDelta = -v.ModDelta
		return
	}
	t.operation(Operation{Kind: OpCounter, Routine: RoutineModulation, VoiceSlot: int8(i), OtherSlot: -1, Counter: CounterModSteps, Before: uint16(v.ModSteps), After: uint16(v.ModSteps - 1), Branch: BranchStep})
	v.ModSteps--
	t.operation(Operation{Kind: OpCounter, Routine: RoutineModulation, VoiceSlot: int8(i), OtherSlot: -1, Counter: CounterModOffset, Before: uint16(v.ModOffset), After: uint16(v.ModOffset) + uint16(int16(v.ModDelta)), Branch: BranchStep})
	v.ModOffset = int16(uint16(v.ModOffset) + uint16(int16(v.ModDelta)))
}

func (t *tick) pitch(i int, onNote bool) error {
	branch := BranchHeldNote
	if onNote {
		branch = BranchNoteBoundary
	}
	defer t.enter(RoutinePitch, i, branch)()
	v := &t.driver.state.Voices[i]
	if onNote {
		if isPSG(i) && int16(v.BasePitch) < 0 || !isPSG(i) && v.BasePitch == 0 {
			v.NoteOff = true
			t.operation(Operation{Kind: OpParameter, Routine: RoutinePitch, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchRest})
			return t.err
		}
	} else if v.PitchControl.EnvelopeID == 0 && !v.PitchControl.ModulationEnabled {
		t.operation(Operation{Kind: OpParameter, Routine: RoutinePitch, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchDisabled})
		return t.err
	}
	if v.NoteOff || v.Claimed {
		branch := BranchNoteOff
		if v.Claimed {
			branch = BranchClaimed
		}
		t.operation(Operation{Kind: OpParameter, Routine: RoutinePitch, VoiceSlot: int8(i), OtherSlot: -1, Branch: branch})
		return t.err
	}
	word := v.BasePitch + uint16(int16(v.Detune))
	if v.PitchControl.EnvelopeID != 0 {
		t.operation(Operation{Kind: OpResourceUnavailable, Routine: RoutinePitch, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchEnvelope})
		if t.err != nil {
			return t.err
		}
		envelope := t.driver.catalog.pitch[v.PitchControl.EnvelopeID]
		tick, next, err := envelope.Next(v.PitchCursor)
		if err != nil {
			return err
		}
		v.PitchCursor = next
		mod := uint16(0)
		if v.PitchControl.ModulationEnabled {
			mod = uint16(v.ModOffset)
		}
		value, scale, err := param.PitchWord(tick, v.PitchScale, v.BasePitch, v.Detune, mod)
		if err != nil {
			return err
		}
		word = value
		v.PitchScale = scale
		if tick.KeyOff {
			v.NoteOff = true
			t.keyOff(i)
		}
	} else if v.PitchControl.ModulationEnabled {
		word += uint16(v.ModOffset)
	}
	if isPSG(i) {
		code := channelCode(v.Channel)
		if code == 0xe0 {
			code = 0xc0
		}
		t.psgPitch(i, code|uint8(word&15), uint8(word>>4)&63)
	} else {
		t.voiceYM(i, 0xa4, uint8(word>>8))
		t.voiceYM(i, 0xa0, uint8(word))
	}
	return t.err
}

func (t *tick) volume(i int) error {
	defer t.enter(RoutineVolume, i, BranchNormal)()
	v := &t.driver.state.Voices[i]
	level := v.BaseLevel
	if v.VolumeEnvelope != 0 {
		t.operation(Operation{Kind: OpResourceUnavailable, Routine: RoutineVolume, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchEnvelope})
		if t.err != nil {
			return t.err
		}
		envelope := t.driver.catalog.volume[v.VolumeEnvelope]
		step, next, err := envelope.Next(v.VolumeCursor)
		if err != nil {
			return err
		}
		v.VolumeCursor = next
		if step.KeyOff {
			t.operation(Operation{Kind: OpParameter, Routine: RoutineVolume, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchKeyOff})
			v.NoteOff = true
			t.keyOff(i)
			return t.err
		}
		value, write, err := param.PSGAttenuation(step, v.BaseLevel)
		if err != nil {
			return err
		}
		if !write {
			t.operation(Operation{Kind: OpParameter, Routine: RoutineVolume, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchHold})
			return t.err
		}
		level = value
	}
	if v.NoteOff || v.Claimed || v.Tie && v.GateTicks != 0 && v.GateRemaining == 0 {
		branch := BranchNoteOff
		if v.Claimed {
			branch = BranchClaimed
		} else if v.Tie && v.GateTicks != 0 && v.GateRemaining == 0 {
			branch = BranchTied
		}
		t.operation(Operation{Kind: OpParameter, Routine: RoutineVolume, VoiceSlot: int8(i), OtherSlot: -1, Branch: branch})
		return t.err
	}
	t.psgFor(i, (level|channelCode(v.Channel))+0x10)
	return t.err
}

func (t *tick) pan(i int, onNote bool) error {
	branch := BranchHeldNote
	if onNote {
		branch = BranchNoteBoundary
	}
	defer t.enter(RoutinePan, i, branch)()
	v := &t.driver.state.Voices[i]
	if v.PanDefinition.Mode == "" || v.PanDefinition.Mode == param.PanDisabled {
		t.operation(Operation{Kind: OpParameter, Routine: RoutinePan, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchDisabled})
		return t.err
	}
	step, err := param.AdvancePan(v.PanDefinition, t.driver.catalog.patterns, v.PanState, onNote, v.NoteOff)
	if err != nil {
		return err
	}
	t.operation(Operation{Kind: OpCounter, Routine: RoutinePan, VoiceSlot: int8(i), OtherSlot: -1, Counter: CounterPanStep, Before: uint16(v.PanState.Step), After: uint16(step.State.Step), Branch: BranchPanAutomation})
	t.operation(Operation{Kind: OpCounter, Routine: RoutinePan, VoiceSlot: int8(i), OtherSlot: -1, Counter: CounterPanElapsed, Before: uint16(v.PanState.Elapsed), After: uint16(step.State.Elapsed), Branch: BranchPanAutomation})
	if t.err != nil {
		return t.err
	}
	v.PanState = step.State
	if step.Pan != nil {
		bits := step.Pan.Bits()
		automatic := v.Pan
		automatic.Left = bits&128 != 0
		automatic.Right = bits&64 != 0
		t.writePanValue(i, automatic)
	}
	return t.err
}
func (t *tick) writePan(i int) {
	t.writePanValue(i, t.driver.state.Voices[i].Pan)
}
func (t *tick) writePanValue(i int, value Pan) {
	v := &t.driver.state.Voices[i]
	if v.Claimed || isPSG(i) {
		return
	}
	if channelCode(v.Channel) == 6 {
		feedback, err := t.readDAC(FeedbackRead{Kind: ReadPanRedirect, VoiceSlot: int8(i), Pan: Stereo{v.Pan.Left, v.Pan.Right}, TrackPan: v.Pan, WritePan: value})
		if err != nil {
			return
		}
		if feedback.PanRedirect {
			t.emitFor(i, Intent{Kind: "dac-pan", Pan: &Stereo{v.Pan.Left, v.Pan.Right}})
			return
		}
	}
	t.voiceYM(i, 0xb4, value.bits())
}

func (t *tick) loadPatch(i int) error {
	defer t.enter(RoutinePatch, i, BranchInitialLoad)()
	v := &t.driver.state.Voices[i]
	if v.Claimed {
		t.operation(Operation{Kind: OpParameter, Routine: RoutinePatch, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchClaimed})
		return t.err
	}
	selected := v.Tone
	if !v.Owned {
		selected = t.driver.catalog.fallbackPatchID
	}
	patch, ok := t.driver.catalog.patch(selected)
	if !ok {
		return fmt.Errorf("missing selected patch")
	}
	t.operation(Operation{Kind: OpResourceUnavailable, Routine: RoutinePatch, VoiceSlot: int8(i), OtherSlot: -1, DefinitionID: selected, Branch: BranchInitialLoad, Iterations: 25})
	if t.err != nil {
		return t.err
	}
	v.LoadedAlgorithm = patch.Algorithm
	writes, err := param.PatchWrites(patch, v.BaseLevel)
	if err != nil {
		return err
	}
	for _, write := range writes {
		t.voiceYM(i, write.Register, write.Value)
	}
	t.writePan(i)
	return t.err
}
func (t *tick) level(i int) error {
	defer t.enter(RoutineLevel, i, BranchOrdinaryLevel)()
	v := &t.driver.state.Voices[i]
	if v.Claimed {
		t.operation(Operation{Kind: OpParameter, Routine: RoutineLevel, VoiceSlot: int8(i), OtherSlot: -1, Branch: BranchClaimed})
		return t.err
	}
	patch, ok := t.driver.catalog.patch(v.Tone)
	if !ok {
		return fmt.Errorf("missing selected patch")
	}
	t.operation(Operation{Kind: OpResourceUnavailable, Routine: RoutineLevel, VoiceSlot: int8(i), OtherSlot: -1, DefinitionID: v.Tone, Branch: BranchOrdinaryLevel, Iterations: 4})
	if t.err != nil {
		return t.err
	}
	// Selected source TL and the last loaded carrier algorithm are distinct
	// stages. This temporary parameter view does not persist a second patch.
	patch.Algorithm = v.LoadedAlgorithm
	mask := [8]uint8{8, 8, 8, 8, 10, 14, 14, 15}[v.LoadedAlgorithm]
	for source, slot := range [4]int{0, 2, 1, 3} {
		patch.Operators[slot].Carrier = mask&(1<<source) != 0
	}
	writes, err := param.FMLevelUpdates(patch, v.BaseLevel, false)
	if err != nil {
		return err
	}
	for _, write := range writes {
		t.voiceYM(i, 0x40+write.Slot*4, write.TotalLevel)
	}
	return t.err
}
