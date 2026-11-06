package scoreclock

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"

type trackPhase uint8

const (
	trackKey trackPhase = iota + 1
	trackPriority
	trackPriorityPending
	trackOwnerEnter
	trackRestore
	trackPatchEnter
	trackResource
	trackWrites
	trackPan
	trackPatchReturn
	trackNoise
	trackOwnerReturn
	trackExtraYM
	trackVoiceReturn
	trackComplete
)

type trackEndTransfer struct {
	s                         *service
	catalog                   *score.Catalog
	chip                      KeyGateChip
	slot, target              int8
	channel                   uint8
	position                  score.EventPosition
	owner                     score.Owner
	targetOwner               score.Owner
	targetOwned               bool
	sequence                  uint32
	intentOffset              int
	phase                     trackPhase
	key                       *KeyTransfer
	keyPart                   uint8
	activeTarget, noiseTarget bool
	instrument                *InstrumentTransfer
	pan                       *PanRouting
	priorityOp                score.Operation
	done                      bool
	failure                   error
}

func restoredMusicSlot(slot int8) int8 {
	switch slot {
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
func observedProjectionMatches(op score.Operation, v score.VoiceState) bool {
	return op.Version == 1 && op.Channel == v.Channel && op.Flags == (score.VoiceFlags{Active: v.Active, NoteOff: v.NoteOff, Claimed: v.Claimed, Tie: v.Tie, ModulationEnabled: v.PitchControl.ModulationEnabled}) && op.BaseLevel == v.BaseLevel && op.BasePitch == v.BasePitch && op.Transpose == v.Transpose && op.GateTicks == v.GateTicks
}
func beginTrackEnd(s *service, op score.Operation, state score.State, c *score.Catalog, chip KeyGateChip) (*trackEndTransfer, error) {
	if op.VoiceSlot < 0 || int(op.VoiceSlot) >= len(state.Voices) || !heldOperation(op, score.OpParameter, score.RoutineVoice) || !op.Position.Valid || op.OtherSlot != -1 {
		return nil, ErrVoiceSequence
	}
	v := state.Voices[op.VoiceSlot]
	branch := score.BranchTrackStop
	if op.VoiceSlot == 0 {
		branch = score.BranchDACTrackStop
	}
	if !v.Active || !v.Owned || op.Branch != branch || op.Position.Owner != v.Owner || !observedProjectionMatches(op, v) {
		return nil, ErrVoiceSequence
	}
	channel, ok := heldChannel(v.Channel)
	if op.VoiceSlot == 0 && v.Channel == "dac" {
		channel, ok = 6, true
	}
	if !ok {
		return nil, ErrNoteVoiceUnavailable
	}
	t := &trackEndTransfer{s: s, catalog: c, chip: chip, slot: op.VoiceSlot, target: restoredMusicSlot(op.VoiceSlot), channel: channel, position: op.Position, owner: v.Owner, sequence: op.Sequence + 1, intentOffset: op.IntentOffset, phase: trackKey}
	if t.slot >= 10 {
		if t.target < 0 {
			return nil, ErrNoteVoiceUnavailable
		}
		t.activeTarget = state.Voices[t.target].Active
		t.targetOwner, t.targetOwned = state.Voices[t.target].Owner, state.Voices[t.target].Owned
		t.noiseTarget = state.Voices[t.target].Channel == "psg-noise"
		if channel == 4 && state.Voices[16].Active || channel >= 128 && state.Voices[17].Active {
			return nil, ErrNoteVoiceUnavailable
		}
	}
	if err := s.ordinaryCosts(16, 16, 12); err != nil {
		return nil, err
	}
	if err := s.branch(channel >= 128, false); err != nil {
		return nil, err
	}
	if channel < 128 {
		if err := s.spend(12); err != nil {
			return nil, err
		}
		if err := s.branch(op.VoiceSlot == 0, true); err != nil {
			return nil, err
		}
		if op.VoiceSlot == 0 {
			if err := t.end(); err != nil {
				return nil, err
			}
			return t, nil
		}
	}
	if err := s.spend(18); err != nil {
		return nil, err
	}
	key, err := beginKey(s, KeyRequest{Channel: channel, NoteOff: v.NoteOff, Claimed: v.Claimed, Tied: false}, chip)
	if err != nil {
		return nil, err
	}
	t.key = key
	return t, nil
}
func (t *trackEndTransfer) fail(err error) error { t.failure = err; return err }
func (t *trackEndTransfer) end() error {
	if err := t.s.ordinaryCosts(8, 16); err != nil {
		return err
	}
	t.phase = trackVoiceReturn
	return nil
}

func (t *trackEndTransfer) Consume(op score.Operation, state score.State) error {
	positionMatches := op.Position == t.position
	if t.phase == trackExtraYM {
		positionMatches = !op.Position.Valid
	}
	if t.done || t.failure != nil || t.phase == trackComplete || op.Sequence != t.sequence || !positionMatches || op.IntentOffset != t.intentOffset || state.Voices[t.slot].Owner != t.owner || !state.Voices[t.slot].Owned {
		return t.fail(ErrVoiceSequence)
	}
	if t.target >= 0 && (state.Voices[t.target].Owner != t.targetOwner || state.Voices[t.target].Owned != t.targetOwned) {
		return t.fail(ErrVoiceSequence)
	}
	if op.VoiceSlot >= 0 {
		if int(op.VoiceSlot) >= len(state.Voices) || !observedProjectionMatches(op, state.Voices[op.VoiceSlot]) {
			return t.fail(ErrVoiceSequence)
		}
	} else if op.Version != 1 {
		return t.fail(ErrVoiceSequence)
	}
	if err := t.consume(op, state); err != nil {
		return t.fail(err)
	}
	t.sequence++
	if op.Kind == score.OpChipWrite {
		t.intentOffset += int(op.WriteCount)
	}
	return nil
}
func (t *trackEndTransfer) consume(op score.Operation, state score.State) error {
	s := t.s
	switch t.phase {
	case trackKey:
		if op.VoiceSlot != t.slot || op.OtherSlot != -1 {
			return ErrVoiceSequence
		}
		switch t.keyPart {
		case 0:
			if !heldOperation(op, score.OpEnter, score.RoutineKey) || op.Branch != score.BranchKeyOff {
				return ErrVoiceSequence
			}
			t.keyPart = 1
		case 1:
			if effect, ok := t.key.ExpectedEffect(); ok {
				if !heldWrite(op, score.OpChipWrite, keyIntent(effect, t.intentOffset)) {
					return ErrVoiceSequence
				}
				if err := t.key.Apply(effect); err != nil {
					return err
				}
			} else {
				v := state.Voices[t.slot]
				branch := score.BranchTied
				if v.Claimed {
					branch = score.BranchClaimed
				}
				if !heldOperation(op, score.OpParameter, score.RoutineKey) || op.Branch != branch {
					return ErrVoiceSequence
				}
			}
			t.keyPart = 2
		case 2:
			if !heldOperation(op, score.OpLeave, score.RoutineKey) || op.Branch != score.BranchKeyOff {
				return ErrVoiceSequence
			}
			if _, err := t.key.Finish(); err != nil {
				return err
			}
			t.key = nil
			if t.channel < 128 {
				if err := s.spend(10); err != nil {
					return err
				}
			}
			if err := s.spend(12); err != nil {
				return err
			}
			if err := s.branch(t.slot < 10, true); err != nil {
				return err
			}
			if t.slot < 10 {
				return t.end()
			}
			if t.target < 0 {
				return ErrNoteVoiceUnavailable
			}
			t.phase = trackPriority
		default:
			return ErrVoiceSequence
		}
	case trackPriority:
		if !heldOperation(op, score.OpPriorityClear, score.RoutineOwnership) || op.VoiceSlot != t.slot || op.OtherSlot != -1 || op.Branch != score.BranchPrimaryTrackStop {
			return ErrVoiceSequence
		}
		t.priorityOp = op
		t.phase = trackPriorityPending
	case trackOwnerEnter:
		if !heldOperation(op, score.OpEnter, score.RoutineOwnership) || op.VoiceSlot != t.slot || op.OtherSlot != -1 || op.Branch != score.BranchTrackStop {
			return ErrVoiceSequence
		}
		t.phase = trackRestore
	case trackRestore:
		if op.VoiceSlot != t.target || op.OtherSlot != t.slot {
			return ErrVoiceSequence
		}
		if t.channel < 128 && !t.activeTarget {
			if !heldOperation(op, score.OpParameter, score.RoutineOwnership) || op.Branch != score.BranchInactive {
				return ErrVoiceSequence
			}
			t.phase = trackOwnerReturn
			return nil
		}
		if !heldOperation(op, score.OpRestore, score.RoutineOwnership) || op.Branch != score.BranchRest {
			return ErrVoiceSequence
		}
		if t.channel < 128 {
			t.phase = trackPatchEnter
		} else if t.noiseTarget {
			t.phase = trackNoise
		} else {
			t.phase = trackOwnerReturn
		}
	case trackPatchEnter:
		if !heldOperation(op, score.OpEnter, score.RoutinePatch) || op.VoiceSlot != t.target || op.OtherSlot != -1 || op.Branch != score.BranchInitialLoad {
			return ErrVoiceSequence
		}
		v := state.Voices[t.target]
		if v.Claimed || !v.NoteOff {
			return ErrVoiceSequence
		}
		work, patch, err := t.catalog.InstrumentParameters(v.Tone)
		if err != nil {
			return err
		}
		channel, ok := heldChannel(v.Channel)
		if !ok || channel >= 128 {
			return ErrNoteVoiceUnavailable
		}
		request := InstrumentRequest{Mode: "load", Domain: "music", Channel: channel, Work: work, Patch: patch, BaseLevel: v.BaseLevel, LoadedAlgorithm: v.LoadedAlgorithm}
		if err = validInstrumentRequest(request); err != nil {
			return err
		}
		t.instrument, err = beginRestoredInstrument(s, request)
		if err != nil {
			return err
		}
		t.phase = trackResource
	case trackResource:
		if !heldOperation(op, score.OpResourceUnavailable, score.RoutinePatch) || op.VoiceSlot != t.target || op.OtherSlot != -1 || op.DefinitionID != t.instrument.request.Work.ID || op.Iterations != 25 || op.Branch != score.BranchInitialLoad {
			return ErrVoiceSequence
		}
		t.phase = trackWrites
	case trackWrites:
		if op.VoiceSlot != t.target || op.OtherSlot != -1 {
			return ErrVoiceSequence
		}
		write := t.instrument.writes[t.instrument.cursor]
		if !heldWrite(op, score.OpChipWrite, keyIntent(KeyEffect{Kind: "ym", YM: write}, t.intentOffset)) {
			return ErrVoiceSequence
		}
		if err := t.instrument.WriteNext(write); err != nil {
			return err
		}
		if t.instrument.cursor == len(t.instrument.writes) {
			t.phase = trackPan
		}
	case trackPan:
		if op.VoiceSlot != t.target || op.OtherSlot != -1 {
			return ErrVoiceSequence
		}
		if _, err := t.instrument.Finish(); err != nil {
			return err
		}
		v := state.Voices[t.target]
		channel, ok := heldChannel(v.Channel)
		if !ok || channel >= 128 {
			return ErrNoteVoiceUnavailable
		}
		request := PanRequest{Entry: "instrument", Channel: channel, Pan: v.Pan}
		if !validPanRequest(request) {
			return ErrVoiceSequence
		}
		var err error
		t.pan, err = beginPanRouting(s, request, nil)
		if err != nil {
			return err
		}
		effect, ok := t.pan.ExpectedEffect()
		if !ok || effect.Kind != "ym" || !heldWrite(op, score.OpChipWrite, keyIntent(KeyEffect{Kind: "ym", YM: effect.Write}, t.intentOffset)) {
			return ErrVoiceSequence
		}
		if err = t.pan.Apply(effect); err != nil {
			return err
		}
		t.phase = trackPatchReturn
	case trackPatchReturn:
		if !heldOperation(op, score.OpLeave, score.RoutinePatch) || op.VoiceSlot != t.target || op.OtherSlot != -1 || op.Branch != score.BranchInitialLoad {
			return ErrVoiceSequence
		}
		if _, err := t.pan.Finish(); err != nil {
			return err
		}
		t.pan = nil
		t.instrument = nil
		t.phase = trackOwnerReturn
	case trackNoise:
		v := state.Voices[t.target]
		if op.VoiceSlot != t.target || op.OtherSlot != -1 || !heldWrite(op, score.OpChipWrite, score.Intent{Order: t.intentOffset, Kind: "psg", Value: v.NoiseControl}) {
			return ErrVoiceSequence
		}
		if err := s.cost(24, func() error { return t.chip.WritePSG(s.clock.Cycle, v.NoiseControl) }); err != nil {
			return err
		}
		t.phase = trackOwnerReturn
	case trackOwnerReturn:
		if !heldOperation(op, score.OpLeave, score.RoutineOwnership) || op.VoiceSlot != t.slot || op.OtherSlot != -1 || op.Branch != score.BranchTrackStop {
			return ErrVoiceSequence
		}
		return t.footer()
	case trackExtraYM:
		if op.VoiceSlot != -1 || op.OtherSlot != -1 || !heldWrite(op, score.OpChipWrite, score.Intent{Order: t.intentOffset, Kind: "ym", Register: 0x27}) {
			return ErrVoiceSequence
		}
		if err := s.ordinaryCosts(4, 4, 18, 10); err != nil {
			return err
		}
		if err := s.serveYM(Request{Register: 0x27}); err != nil {
			return err
		}
		if err := s.spend(10); err != nil {
			return err
		}
		return t.end()
	case trackVoiceReturn:
		if !heldOperation(op, score.OpLeave, score.RoutineVoice) || op.VoiceSlot != t.slot || op.OtherSlot != -1 || op.Branch != score.BranchActive || state.Voices[t.slot].Active {
			return ErrVoiceSequence
		}
		t.phase = trackComplete
	default:
		return ErrVoiceSequence
	}
	return nil
}

func (t *trackEndTransfer) PriorityClear(op score.Operation, apply func() error) error {
	if t.done || t.failure != nil || t.phase != trackPriorityPending || apply == nil || op.Version != 1 || op.Sequence != t.priorityOp.Sequence || op.Position != t.priorityOp.Position || op.Kind != t.priorityOp.Kind || op.VoiceSlot != t.slot || op.OtherSlot != -1 || op.IntentOffset != t.priorityOp.IntentOffset {
		return t.fail(ErrVoiceSequence)
	}
	if err := t.s.cost(16, apply); err != nil {
		return t.fail(err)
	}
	if err := t.restorePrefix(); err != nil {
		return t.fail(err)
	}
	return nil
}
func (t *trackEndTransfer) restorePrefix() error {
	s := t.s
	if err := s.ordinaryCosts(4, 12); err != nil {
		return err
	}
	if err := s.branch(t.channel >= 128, false); err != nil {
		return err
	}
	// The legal canonical profile has no active secondary layer. The actual
	// target observations are supplied with the priority store by the parent.
	if t.channel < 128 {
		if err := s.ordinaryCosts(8, 4, 8); err != nil {
			return err
		}
		if err := s.branch(t.channel != 4, false); err != nil {
			return err
		}
		if t.channel == 4 {
			if err := s.spend(12); err != nil {
				return err
			}
			if err := s.branch(true, false); err != nil {
				return err
			}
		}
		if err := s.ordinaryCosts(4, 10, 18, 8); err != nil {
			return err
		}
		if err := s.branch(!t.activeTarget, false); err != nil {
			return err
		}
		if t.activeTarget {
			if err := s.ordinaryCosts(16, 16, 16, 12, 18); err != nil {
				return err
			}
		}
	} else {
		if err := s.ordinaryCosts(8, 8); err != nil {
			return err
		}
		if err := s.branch(true, false); err != nil {
			return err
		}
		if err := s.ordinaryCosts(8, 12, 18, 16, 16, 16); err != nil {
			return err
		}
		if err := s.branch(!t.noiseTarget, false); err != nil {
			return err
		}
	}
	t.phase = trackOwnerEnter
	return nil
}
func (t *trackEndTransfer) footer() error {
	if t.channel < 128 {
		if err := t.s.ordinaryCosts(4, 16); err != nil {
			return err
		}
		if err := t.s.branch(t.channel != 2, false); err != nil {
			return err
		}
		if t.channel == 2 {
			if err := t.s.spend(12); err != nil {
				return err
			}
			if err := t.s.branch(false, false); err != nil {
				return err
			}
			t.phase = trackExtraYM
			return nil
		}
	}
	return t.end()
}
func (t *trackEndTransfer) Finish() (Clock, error) {
	if t.done || t.failure != nil || t.phase != trackComplete {
		return Clock{}, t.fail(ErrVoiceSequence)
	}
	t.done = true
	return t.s.clock, nil
}
