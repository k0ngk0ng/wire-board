package scoreclock

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

var ErrHeldVoiceSequence = errors.New("invalid held voice sequence")
var ErrHeldVoiceUnavailable = errors.New("held voice caller unavailable")

type heldPhase uint8

const (
	heldWait heldPhase = iota + 1
	heldMarker
	heldGateCounter
	heldGateExpiry
	heldAmbient
	heldPan
	heldVolumeResource
	heldVolumeBody
	heldVolumeReturn
	heldModEnter
	heldModBody
	heldPitchEnter
	heldPitchResource
	heldPitchSuppressed
	heldKey
	heldFrequency
	heldPitchReturn
	heldVoiceReturn
	heldComplete
)

type heldKeyReason uint8

const (
	heldGateKey heldKeyReason = iota + 1
	heldVolumeKey
	heldPitchKey
	heldNoteKey
)

type heldModPart uint8

const (
	heldModDisabled heldModPart = iota + 1
	heldModDelay
	heldModRate
	heldModReverse
	heldModSteps
	heldModOffset
	heldModReturn
)

// HeldVoiceTransfer consumes one actual score voice call with a non-expiring
// NoteWait. It composes the existing leaves on one temporary service. No score
// state, source program, original cursor or persistent prepared word lives here.
// Callers supply the transaction's current working voice at every operation,
// and commit all detached score/bus/chip owners together only after success.
type HeldVoiceTransfer struct {
	s                *service
	catalog          *score.Catalog
	chip             KeyGateChip
	slot             int8
	owner            score.Owner
	channel          uint8
	channelName      string
	position         score.EventPosition
	sequence         uint32
	intentOffset     int
	wait, remaining  uint8
	phase            heldPhase
	part             uint8
	modPart          heldModPart
	panEnabled       bool
	panTick          param.PanTick
	pan              *PanAutomationTransfer
	volume           *VolumeTransfer
	volumeTick       param.VolumeTick
	gate             *GateTransfer
	key              *KeyTransfer
	keyReason        heldKeyReason
	pitchSuppression score.Branch
	prepared         uint16
	frequency        *FrequencyTransfer
	onNote           bool
	done             bool
	failure          error
}

func heldChannel(name string) (uint8, bool) {
	switch name {
	case "fm1":
		return 0, true
	case "fm2":
		return 1, true
	case "fm3":
		return 2, true
	case "fm4":
		return 4, true
	case "fm5":
		return 5, true
	case "psg1":
		return 128, true
	case "psg2":
		return 160, true
	case "psg3":
		return 192, true
	case "psg-noise":
		return 224, true
	}
	return 0, false
}

// BeginHeldVoice starts at the FM/PSG voice body entry, before NoteWait's
// decrement. New notes, DAC and unproved FM6 are explicit unsupported entries;
// the enclosing voice-scan caller and full tick/world costs are not included.
func BeginHeldVoice(clock Clock, op score.Operation, v score.VoiceState, catalog *score.Catalog, bus Bus, chip KeyGateChip, limits Limits) (*HeldVoiceTransfer, error) {
	if catalog == nil || op.Kind != score.OpEnter || op.Routine != score.RoutineVoice || op.Branch != score.BranchActive || op.Position.Valid || !v.Active || !v.Owned || !observedVoiceMatches(op, v) || op.Sequence >= score.MaxObservedOperations {
		return nil, ErrHeldVoiceSequence
	}
	channel, ok := heldChannel(v.Channel)
	if !ok || v.NoteWait == 1 {
		return nil, ErrHeldVoiceUnavailable
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return nil, err
	}
	t := &HeldVoiceTransfer{s: s, catalog: catalog, chip: chip, slot: op.VoiceSlot, owner: v.Owner, channel: channel, channelName: v.Channel, position: op.Position, sequence: op.Sequence + 1, intentOffset: op.IntentOffset, wait: v.NoteWait, remaining: v.GateRemaining, phase: heldWait}
	if err = s.spend(16); err != nil {
		return nil, err
	}
	if err = s.branch(true, false); err != nil {
		return nil, err
	}
	if err = s.spend(18); err != nil {
		return nil, err
	}
	t.gate, err = beginGateTransfer(s, GateRequest{Channel: channel, Remaining: v.GateRemaining, Claimed: v.Claimed, Tied: v.Tie}, chip)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *HeldVoiceTransfer) Clock() Clock         { return t.s.clock }
func (t *HeldVoiceTransfer) fail(err error) error { t.failure = err; return err }

func heldOperation(op score.Operation, kind score.OperationKind, routine score.Routine) bool {
	return op.Kind == kind && op.Routine == routine
}

func heldCounter(op score.Operation, routine score.Routine, counter score.CounterKind, before, after uint16, branch score.Branch) bool {
	return heldOperation(op, score.OpCounter, routine) && op.Counter == counter && op.Before == before && op.After == after && op.Branch == branch
}

func (t *HeldVoiceTransfer) Consume(op score.Operation, v score.VoiceState) error {
	if t == nil {
		return ErrHeldVoiceSequence
	}
	if t.done || t.failure != nil || t.phase == heldComplete || op.Sequence != t.sequence || op.VoiceSlot != t.slot || op.OtherSlot != -1 || op.Position != t.position || op.IntentOffset != t.intentOffset || v.Owner != t.owner || !v.Active || !v.Owned || v.Channel != t.channelName || !observedVoiceMatches(op, v) {
		return t.fail(ErrHeldVoiceSequence)
	}
	if err := t.consume(op, v); err != nil {
		return t.fail(err)
	}
	t.sequence++
	if op.Kind == score.OpChipWrite || op.Kind == score.OpPSGPitchWrite {
		t.intentOffset += int(op.WriteCount)
	}
	return nil
}

func (t *HeldVoiceTransfer) finishGate() error {
	if _, err := t.gate.Finish(); err != nil {
		return err
	}
	t.gate = nil
	t.phase = heldAmbient
	return nil
}

func (t *HeldVoiceTransfer) consume(op score.Operation, v score.VoiceState) error {
	switch t.phase {
	case heldWait:
		if !heldCounter(op, score.RoutineVoice, score.CounterNoteWait, uint16(t.wait), uint16(t.wait-1), score.BranchStep) || v.NoteWait != t.wait {
			return ErrHeldVoiceSequence
		}
		t.phase = heldMarker
	case heldMarker:
		if !heldOperation(op, score.OpParameter, score.RoutineVoice) || op.Branch != score.BranchHeldNote || v.NoteWait != t.wait-1 || v.GateRemaining != t.remaining {
			return ErrHeldVoiceSequence
		}
		if t.remaining == 0 {
			return t.finishGate()
		}
		t.phase = heldGateCounter
	case heldGateCounter:
		if !heldCounter(op, score.RoutineGate, score.CounterGate, uint16(t.remaining), uint16(t.remaining-1), score.BranchStep) || v.GateRemaining != t.remaining {
			return ErrHeldVoiceSequence
		}
		if t.gate.Expired() {
			t.phase = heldGateExpiry
		} else {
			return t.finishGate()
		}
	case heldGateExpiry:
		if !heldOperation(op, score.OpParameter, score.RoutineGate) || op.Branch != score.BranchGateExpired || v.GateRemaining != 0 {
			return ErrHeldVoiceSequence
		}
		t.phase, t.keyReason, t.part = heldKey, heldGateKey, 0
	case heldAmbient:
		return t.beginAmbient(op, v)
	case heldPan:
		return t.consumePan(op, v)
	case heldVolumeResource:
		if !heldOperation(op, score.OpResourceUnavailable, score.RoutineVolume) || op.Branch != score.BranchEnvelope {
			return ErrHeldVoiceSequence
		}
		t.phase = heldVolumeBody
	case heldVolumeBody:
		return t.consumeVolume(op, v)
	case heldVolumeReturn:
		if !heldOperation(op, score.OpLeave, score.RoutineVolume) {
			return ErrHeldVoiceSequence
		}
		if _, err := t.volume.Finish(); err != nil {
			return err
		}
		t.volume = nil
		t.phase = heldModEnter
		if t.onNote {
			t.phase = heldVoiceReturn
		}
	case heldModEnter:
		return t.beginModulation(op, v)
	case heldModBody:
		return t.consumeModulation(op, v)
	case heldPitchEnter:
		return t.beginPitch(op, v)
	case heldPitchResource:
		if !heldOperation(op, score.OpResourceUnavailable, score.RoutinePitch) || op.Branch != score.BranchEnvelope {
			return ErrHeldVoiceSequence
		}
		return t.calculatePitch(v)
	case heldPitchSuppressed:
		if !heldOperation(op, score.OpParameter, score.RoutinePitch) || op.Branch != t.pitchSuppression {
			return ErrHeldVoiceSequence
		}
		t.phase = heldPitchReturn
	case heldKey:
		return t.consumeKey(op, v)
	case heldFrequency:
		return t.consumeFrequency(op)
	case heldPitchReturn:
		if !heldOperation(op, score.OpLeave, score.RoutinePitch) || op.Branch != t.pitchBranch() {
			return ErrHeldVoiceSequence
		}
		if t.frequency != nil {
			if _, err := t.frequency.Finish(); err != nil {
				return err
			}
			t.frequency = nil
		}
		t.phase = heldVoiceReturn
		if t.onNote {
			t.phase = heldAmbient
		}
	case heldVoiceReturn:
		if !heldOperation(op, score.OpLeave, score.RoutineVoice) || op.Branch != score.BranchActive {
			return ErrHeldVoiceSequence
		}
		t.phase = heldComplete
	default:
		return ErrHeldVoiceSequence
	}
	return nil
}

func (t *HeldVoiceTransfer) beginAmbient(op score.Operation, v score.VoiceState) error {
	s := t.s
	if t.channel < 128 {
		if !heldOperation(op, score.OpEnter, score.RoutinePan) || op.Branch != t.pitchBranch() {
			return ErrHeldVoiceSequence
		}
		if err := s.spend(18); err != nil {
			return err
		}
		definition := v.PanDefinition
		t.panEnabled = definition.Mode != "" && definition.Mode != param.PanDisabled
		if definition.Mode == "" {
			definition.Mode = param.PanDisabled
		}
		tick, err := t.catalog.PanAutomationStep(definition, v.PanState, t.onNote, v.NoteOff)
		if err != nil {
			return err
		}
		request := PanAutomationRequest{Channel: t.channel, Definition: definition, State: v.PanState, OnNote: t.onNote, NoteOff: v.NoteOff, Claimed: v.Claimed, Pan: v.Pan}
		panRequest := PanRequest{Entry: "voice", Channel: t.channel, Claimed: v.Claimed, Pan: v.Pan}
		if !validPanRequest(panRequest) {
			return ErrHeldVoiceSequence
		}
		t.pan, err = beginPanAutomation(s, request, panRequest, tick, nil)
		if err != nil {
			return err
		}
		t.panTick, t.part, t.phase = tick, 0, heldPan
		return nil
	}
	if !t.onNote && v.VolumeEnvelope == 0 {
		// The held PSG wrapper is still called when it skips its body.
		if err := s.ordinaryCosts(18, 12); err != nil {
			return err
		}
		if err := s.branch(true, true); err != nil {
			return err
		}
		if err := s.spend(16); err != nil {
			return err
		}
		return t.beginModulation(op, v)
	}
	if !heldOperation(op, score.OpEnter, score.RoutineVolume) {
		return ErrHeldVoiceSequence
	}
	if t.onNote {
		if err := s.spend(10); err != nil {
			return err
		}
	} else {
		if err := s.ordinaryCosts(18, 12); err != nil {
			return err
		}
		if err := s.branch(false, true); err != nil {
			return err
		}
	}
	r := EnvelopeVolumeRequest{HasEnvelope: v.VolumeEnvelope != 0, BaseLevel: v.BaseLevel, Channel: t.channel, NoteOff: v.NoteOff, Claimed: v.Claimed, Tied: v.Tie, GateTicks: v.GateTicks, GateRemaining: v.GateRemaining}
	var err error
	if r.HasEnvelope {
		r.Tick, r.Work, err = t.catalog.VolumeEnvelopeStep(v.VolumeEnvelope, v.VolumeCursor)
		if err != nil {
			return err
		}
	}
	t.volume, err = beginEnvelopeVolume(s, r, t.chip)
	if err != nil {
		return err
	}
	t.volumeTick, t.phase = r.Tick, heldVolumeBody
	if r.HasEnvelope {
		t.phase = heldVolumeResource
	}
	return nil
}

func (t *HeldVoiceTransfer) consumePan(op score.Operation, v score.VoiceState) error {
	if t.part == 0 && !t.panEnabled {
		if !heldOperation(op, score.OpParameter, score.RoutinePan) || op.Branch != score.BranchDisabled {
			return ErrHeldVoiceSequence
		}
		t.part = 2
		return nil
	}
	if t.part < 2 {
		counter, before, after := score.CounterPanStep, v.PanState.Step, t.panTick.State.Step
		if t.part == 1 {
			counter, before, after = score.CounterPanElapsed, v.PanState.Elapsed, t.panTick.State.Elapsed
		}
		if !heldCounter(op, score.RoutinePan, counter, uint16(before), uint16(after), score.BranchPanAutomation) {
			return ErrHeldVoiceSequence
		}
		t.part++
		return nil
	}
	if effect, ok := t.pan.ExpectedEffect(); ok && t.part == 2 {
		if effect.Kind != "ym" || !heldWrite(op, score.OpChipWrite, keyIntent(KeyEffect{Kind: "ym", YM: effect.Write}, t.intentOffset)) {
			return ErrHeldVoiceSequence
		}
		if err := t.pan.Apply(effect); err != nil {
			return err
		}
		t.part++
		return nil
	}
	if !heldOperation(op, score.OpLeave, score.RoutinePan) || op.Branch != t.pitchBranch() {
		return ErrHeldVoiceSequence
	}
	if _, err := t.pan.Finish(); err != nil {
		return err
	}
	t.pan = nil
	t.panTick = param.PanTick{}
	t.phase = heldModEnter
	if t.onNote {
		t.phase, t.keyReason, t.part = heldKey, heldNoteKey, 0
	}
	return nil
}

func keyIntent(effect KeyEffect, order int) score.Intent {
	if effect.Kind == "psg" {
		return score.Intent{Order: order, Kind: "psg", Value: effect.PSG}
	}
	return score.Intent{Order: order, Kind: "ym", Bank: effect.YM.Bank, Register: effect.YM.Register, Value: effect.YM.Value}
}

func heldWrite(op score.Operation, kind score.OperationKind, intent score.Intent) bool {
	return op.Kind == kind && op.Routine == "" && op.WriteCount == 1 && sameObservedIntent(op.Writes[0], intent)
}

func (t *HeldVoiceTransfer) writeVolume(op score.Operation) error {
	value, ok := t.volume.ExpectedWrite()
	if !ok || !heldWrite(op, score.OpChipWrite, score.Intent{Order: t.intentOffset, Kind: "psg", Value: value}) {
		return ErrHeldVoiceSequence
	}
	return t.volume.Write(value)
}

func (t *HeldVoiceTransfer) consumeVolume(op score.Operation, v score.VoiceState) error {
	if t.volumeTick.KeyOff {
		if !heldOperation(op, score.OpParameter, score.RoutineVolume) || op.Branch != score.BranchKeyOff {
			return ErrHeldVoiceSequence
		}
		t.phase, t.keyReason, t.part = heldKey, heldVolumeKey, 0
		return nil
	}
	if v.VolumeEnvelope != 0 && !t.volumeTick.WritesLevel {
		if !heldOperation(op, score.OpParameter, score.RoutineVolume) || op.Branch != score.BranchHold {
			return ErrHeldVoiceSequence
		}
	} else if _, ok := t.volume.ExpectedWrite(); ok {
		if err := t.writeVolume(op); err != nil {
			return err
		}
	} else {
		branch := score.BranchNoteOff
		if v.Claimed {
			branch = score.BranchClaimed
		} else if v.Tie && v.GateTicks != 0 && v.GateRemaining == 0 {
			branch = score.BranchTied
		}
		if !heldOperation(op, score.OpParameter, score.RoutineVolume) || op.Branch != branch {
			return ErrHeldVoiceSequence
		}
	}
	t.volumeTick = param.VolumeTick{}
	t.phase = heldVolumeReturn
	return nil
}

func (t *HeldVoiceTransfer) beginModulation(op score.Operation, v score.VoiceState) error {
	if !heldOperation(op, score.OpEnter, score.RoutineModulation) || op.Branch != score.BranchHeldNote {
		return ErrHeldVoiceSequence
	}
	if err := t.s.spend(18); err != nil {
		return err
	}
	if err := t.s.modulationWork(ModulationWork{Enabled: v.PitchControl.ModulationEnabled, Delay: v.ModDelay, Rate: v.ModRate, Steps: v.ModSteps}); err != nil {
		return err
	}
	t.modPart = heldModRate
	if !v.PitchControl.ModulationEnabled {
		t.modPart = heldModDisabled
	} else if v.ModDelay != 0 {
		t.modPart = heldModDelay
	}
	t.phase = heldModBody
	return nil
}

func (t *HeldVoiceTransfer) consumeModulation(op score.Operation, v score.VoiceState) error {
	switch t.modPart {
	case heldModDisabled, heldModReverse:
		branch := score.BranchDisabled
		if t.modPart == heldModReverse {
			branch = score.BranchReverse
		}
		if !heldOperation(op, score.OpParameter, score.RoutineModulation) || op.Branch != branch {
			return ErrHeldVoiceSequence
		}
		t.modPart = heldModReturn
	case heldModDelay:
		if !heldCounter(op, score.RoutineModulation, score.CounterModDelay, uint16(v.ModDelay), uint16(v.ModDelay-1), score.BranchDelay) {
			return ErrHeldVoiceSequence
		}
		t.modPart = heldModReturn
	case heldModRate:
		if !heldCounter(op, score.RoutineModulation, score.CounterModRate, uint16(v.ModRate), uint16(v.ModRate-1), score.BranchRate) {
			return ErrHeldVoiceSequence
		}
		t.modPart = heldModReturn
		if v.ModRate == 1 {
			t.modPart = heldModSteps
			if v.ModSteps == 0 {
				t.modPart = heldModReverse
			}
		}
	case heldModSteps:
		if !heldCounter(op, score.RoutineModulation, score.CounterModSteps, uint16(v.ModSteps), uint16(v.ModSteps-1), score.BranchStep) {
			return ErrHeldVoiceSequence
		}
		t.modPart = heldModOffset
	case heldModOffset:
		if !heldCounter(op, score.RoutineModulation, score.CounterModOffset, uint16(v.ModOffset), uint16(v.ModOffset)+uint16(int16(v.ModDelta)), score.BranchStep) {
			return ErrHeldVoiceSequence
		}
		t.modPart = heldModReturn
	case heldModReturn:
		if !heldOperation(op, score.OpLeave, score.RoutineModulation) || op.Branch != score.BranchHeldNote {
			return ErrHeldVoiceSequence
		}
		t.phase = heldPitchEnter
	default:
		return ErrHeldVoiceSequence
	}
	return nil
}

func (t *HeldVoiceTransfer) beginPitch(op score.Operation, v score.VoiceState) error {
	if !heldOperation(op, score.OpEnter, score.RoutinePitch) || op.Branch != t.pitchBranch() {
		return ErrHeldVoiceSequence
	}
	s, fm := t.s, t.channel < 128
	disabled := false
	if t.onNote {
		if err := s.spend(12); err != nil {
			return err
		}
		rest := fm && v.BasePitch == 0 || !fm && int16(v.BasePitch) < 0
		if err := s.branch(!rest, false); err != nil {
			return err
		}
		if rest {
			if err := s.ordinaryCosts(16, 16); err != nil {
				return err
			}
			t.pitchSuppression, t.phase = score.BranchRest, heldPitchSuppressed
			return nil
		}
	} else {
		if err := s.ordinaryCosts(10, 12); err != nil {
			return err
		}
		disabled = v.PitchControl.EnvelopeID == 0 && !v.PitchControl.ModulationEnabled
		if err := s.branch(disabled, fm); err != nil {
			return err
		}
	}
	t.pitchSuppression = score.BranchDisabled
	if !disabled {
		if err := s.spend(12); err != nil {
			return err
		}
		if err := s.branch(v.NoteOff, fm); err != nil {
			return err
		}
		if !v.NoteOff {
			if err := s.spend(12); err != nil {
				return err
			}
			if err := s.branch(v.Claimed, fm); err != nil {
				return err
			}
		}
		if !v.NoteOff && !v.Claimed {
			if err := s.spend(18); err != nil {
				return err
			}
			if v.PitchControl.EnvelopeID != 0 {
				t.phase = heldPitchResource
				return nil
			}
			return t.calculatePitch(v)
		}
		t.pitchSuppression = score.BranchNoteOff
		if v.Claimed {
			t.pitchSuppression = score.BranchClaimed
		}
	}
	if err := s.spend(16); err != nil {
		return err
	}
	t.phase = heldPitchSuppressed
	return nil
}

func (t *HeldVoiceTransfer) calculatePitch(v score.VoiceState) error {
	r := PitchCalculationRequest{Family: "fm", HasEnvelope: v.PitchControl.EnvelopeID != 0, Scale: v.PitchScale, ModulationEnabled: v.PitchControl.ModulationEnabled}
	if t.channel >= 128 {
		r.Family = "psg"
	}
	var err error
	word := v.BasePitch + uint16(int16(v.Detune))
	modulation := uint16(0)
	if v.PitchControl.ModulationEnabled {
		modulation = uint16(v.ModOffset)
	}
	if r.HasEnvelope {
		r.Tick, r.Work, err = t.catalog.PitchEnvelopeStep(v.PitchControl.EnvelopeID, v.PitchCursor)
		if err != nil {
			return err
		}
		word, _, err = param.PitchWord(r.Tick, v.PitchScale, v.BasePitch, v.Detune, modulation)
		if err != nil {
			return err
		}
	} else {
		word += modulation
	}
	result, err := pricePitchCalculation(t.s, r)
	if err != nil {
		return err
	}
	if result.Next == "key-off" {
		// The canonical cursor has not advanced at this resource callback.
		// Retain only the resolved transient request until its real key return.
		t.prepared = word
		t.phase, t.keyReason, t.part = heldKey, heldPitchKey, 0
		return nil
	}
	return t.beginFrequency(word)
}

func (t *HeldVoiceTransfer) beginFrequency(word uint16) error {
	var err error
	t.frequency, err = beginFrequencyTransfer(t.s, FrequencyRequest{Channel: t.channel, Word: word}, t.chip)
	if err != nil {
		return err
	}
	t.prepared = 0
	t.phase = heldFrequency
	return nil
}

func (t *HeldVoiceTransfer) keyEffect() (KeyEffect, bool) {
	switch t.keyReason {
	case heldGateKey:
		return t.gate.ExpectedEffect()
	case heldVolumeKey:
		value, ok := t.volume.ExpectedWrite()
		return KeyEffect{Kind: "psg", PSG: value}, ok
	case heldPitchKey, heldNoteKey:
		return t.key.ExpectedEffect()
	}
	return KeyEffect{}, false
}

func (t *HeldVoiceTransfer) consumeKey(op score.Operation, v score.VoiceState) error {
	if t.part == 0 {
		branch := score.BranchKeyOff
		if t.keyReason == heldNoteKey {
			branch = score.BranchKeyOn
		}
		if !heldOperation(op, score.OpEnter, score.RoutineKey) || op.Branch != branch || t.keyReason != heldNoteKey && !v.NoteOff {
			return ErrHeldVoiceSequence
		}
		if t.keyReason == heldPitchKey || t.keyReason == heldNoteKey {
			var err error
			if t.keyReason == heldNoteKey {
				if err = t.s.spend(10); err != nil {
					return err
				}
			}
			t.key, err = beginKey(t.s, KeyRequest{Channel: t.channel, On: t.keyReason == heldNoteKey, NoteOff: v.NoteOff, Claimed: v.Claimed, Tied: v.Tie}, t.chip)
			if err != nil {
				return err
			}
		}
		t.part = 1
		return nil
	}
	if t.part == 1 {
		if effect, ok := t.keyEffect(); ok {
			if !heldWrite(op, score.OpChipWrite, keyIntent(effect, t.intentOffset)) {
				return ErrHeldVoiceSequence
			}
			var err error
			switch t.keyReason {
			case heldGateKey:
				err = t.gate.Apply(effect)
			case heldVolumeKey:
				err = t.writeVolume(op)
			case heldPitchKey, heldNoteKey:
				err = t.key.Apply(effect)
			}
			if err != nil {
				return err
			}
		} else {
			branch := score.BranchTied
			if t.keyReason == heldNoteKey {
				branch = score.BranchNoteOff
			}
			if v.Claimed {
				branch = score.BranchClaimed
			}
			if !heldOperation(op, score.OpParameter, score.RoutineKey) || op.Branch != branch {
				return ErrHeldVoiceSequence
			}
		}
		t.part = 2
		return nil
	}
	branch := score.BranchKeyOff
	if t.keyReason == heldNoteKey {
		branch = score.BranchKeyOn
	}
	if !heldOperation(op, score.OpLeave, score.RoutineKey) || op.Branch != branch {
		return ErrHeldVoiceSequence
	}
	switch t.keyReason {
	case heldGateKey:
		if _, err := t.gate.Finish(); err != nil {
			return err
		}
		t.gate = nil
		t.phase = heldVoiceReturn
	case heldVolumeKey:
		t.volumeTick = param.VolumeTick{}
		t.phase = heldVolumeReturn
	case heldPitchKey:
		if _, err := t.key.Finish(); err != nil {
			return err
		}
		t.key = nil
		return t.beginFrequency(t.prepared)
	case heldNoteKey:
		if _, err := t.key.Finish(); err != nil {
			return err
		}
		t.key = nil
		t.phase = heldVoiceReturn
	}
	return nil
}

func (t *HeldVoiceTransfer) pitchBranch() score.Branch {
	if t.onNote {
		return score.BranchNoteBoundary
	}
	return score.BranchHeldNote
}

func (t *HeldVoiceTransfer) consumeFrequency(op score.Operation) error {
	if t.channel < 128 {
		effect := t.frequency.effects[t.frequency.next]
		if !heldWrite(op, score.OpChipWrite, keyIntent(effect, t.intentOffset)) {
			return ErrHeldVoiceSequence
		}
		if err := t.frequency.WriteNext(effect); err != nil {
			return err
		}
	} else {
		if !heldOperation(op, score.OpPSGPitchWrite, score.RoutinePitch) || op.WriteCount != 2 {
			return ErrHeldVoiceSequence
		}
		for i, effect := range t.frequency.effects {
			if !sameObservedIntent(op.Writes[i], keyIntent(effect, t.intentOffset+i)) {
				return ErrHeldVoiceSequence
			}
		}
		for _, effect := range t.frequency.effects {
			if err := t.frequency.WriteNext(effect); err != nil {
				return err
			}
		}
	}
	if t.frequency.next == len(t.frequency.effects) {
		t.phase = heldPitchReturn
	}
	return nil
}

func (t *HeldVoiceTransfer) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrHeldVoiceSequence
	}
	if t.done || t.failure != nil || t.phase != heldComplete {
		return Clock{}, t.fail(ErrHeldVoiceSequence)
	}
	t.done = true
	return t.s.clock, nil
}
