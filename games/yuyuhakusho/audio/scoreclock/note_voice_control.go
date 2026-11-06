package scoreclock

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"

func (t *NoteVoiceTransfer) finishControl(op score.Operation) error {
	if !heldOperation(op, score.OpLeave, score.RoutineControl) || op.EventKind != t.action {
		return ErrNoteVoiceSequence
	}
	if t.pan != nil {
		if _, err := t.pan.Finish(); err != nil {
			return err
		}
		t.pan = nil
	}
	if t.controlNext == "psg-noise" {
		if err := t.s.spend(16); err != nil {
			return err
		}
	}
	if err := t.s.spend(10); err != nil {
		return err
	}
	t.phase = notePrelude
	t.positionSet = false
	t.action = ""
	t.token = ""
	t.controlNext = ""
	t.instrumentMode = ""
	t.instrument = nil
	t.instrumentFinished = false
	t.readControl = false
	return nil
}

func (t *NoteVoiceTransfer) consumeControl(op score.Operation, v score.VoiceState) error {
	s := t.s
	switch t.phase {
	case noteControlEnter:
		if !heldOperation(op, score.OpEnter, score.RoutineControl) || op.EventKind != t.action {
			return ErrNoteVoiceSequence
		}
		t.phase = noteControlParameter
	case noteControlParameter:
		if !heldOperation(op, score.OpParameter, score.RoutineControl) || op.EventKind != t.action {
			return ErrNoteVoiceSequence
		}
		work := ControlParameterWork{Action: t.action, Family: t.family, ImmediateFM: op.Control.ImmediateFM, PanDisabled: op.Control.PanDisabled, Enabled: op.Control.Enabled, Claimed: v.Claimed, Noise: op.Control.Noise}
		if !validControlWork(work) {
			return ErrNoteVoiceUnavailable
		}
		next, err := s.controlPrefix(work)
		if err != nil {
			return err
		}
		t.controlNext = next
		switch next {
		case "complete":
			t.phase = noteControlReturn
			if t.action == "modulation" {
				t.phase = noteControlSetup
			}
		case "pan":
			t.phase = noteControlPan
		case "psg-noise":
			t.noise = op.Control.Noise
			t.phase = noteControlNoise
		case "instrument", "level":
			t.phase = noteInstrumentEnter
		case "queue":
			if !t.allowStop {
				return ErrNoteVoiceUnavailable
			}
			t.phase = noteControlQueue
		default:
			return ErrNoteVoiceUnavailable
		}
	case noteControlSetup:
		if !heldOperation(op, score.OpParameter, score.RoutineModulation) || op.EventKind != "definition-setup" || op.Branch != score.BranchInitialLoad || op.Iterations != 4 {
			return ErrNoteVoiceSequence
		}
		t.phase = noteControlReturn
	case noteControlNoise:
		if !heldWrite(op, score.OpChipWrite, score.Intent{Order: t.intentOffset, Kind: "psg", Value: t.noise}) {
			return ErrNoteVoiceSequence
		}
		if err := s.cost(24, func() error { return t.chip.WritePSG(s.clock.Cycle, t.noise) }); err != nil {
			return err
		}
		t.phase = noteControlReturn
	case noteControlPan:
		if heldOperation(op, score.OpLeave, score.RoutineControl) {
			if !v.Claimed {
				return ErrNoteVoiceSequence
			}
			if err := t.beginControlPan(v, "voice"); err != nil {
				return err
			}
			return t.finishControl(op)
		}
		if heldOperation(op, score.OpFeedbackRead, score.RoutineDAC) {
			if t.family != "dac" || op.Read.Kind != score.ReadPanRedirect || op.Read.TrackPan != v.Pan || op.Read.WritePan != v.Pan {
				return ErrNoteVoiceSequence
			}
			if err := t.beginControlPan(v, "voice"); err != nil {
				return err
			}
			effect, ok := t.pan.ExpectedEffect()
			if !ok {
				return ErrNoteVoiceSequence
			}
			t.read, t.feedback, t.readReady, t.readUsed, t.readControl = op.Read, score.DACFeedback{PanRedirect: effect.Kind == "dac-pan"}, true, false, true
			t.phase = noteFeedbackResult
			return nil
		}
		if t.family != "fm" {
			return ErrNoteVoiceSequence
		}
		if err := t.beginControlPan(v, "voice"); err != nil {
			return err
		}
		if err := t.applyControlPan(op, v); err != nil {
			return err
		}
		t.phase = noteControlReturn
	case noteControlPanWrite:
		if err := t.applyControlPan(op, v); err != nil {
			return err
		}
		t.phase = noteControlReturn
	case noteControlReturn:
		return t.finishControl(op)
	case noteControlQueue:
		if op.Kind != score.OpQueueReplace || op.WriteCount != 1 || op.Writes[0].Kind != "queue" || op.Writes[0].Action != "replace-first" || op.Writes[0].Order != op.IntentOffset {
			return ErrNoteVoiceSequence
		}
		t.queueOp, t.phase = op, noteQueuePending
	case noteInstrumentEnter:
		mode, routine, branch := "load", score.RoutinePatch, score.BranchInitialLoad
		if t.controlNext == "level" {
			mode, routine, branch = "level", score.RoutineLevel, score.BranchOrdinaryLevel
		}
		if !heldOperation(op, score.OpEnter, routine) || op.Branch != branch {
			return ErrNoteVoiceSequence
		}
		work, patch, err := t.catalog.InstrumentParameters(v.Tone)
		if err != nil {
			return err
		}
		channel, ok := heldChannel(v.Channel)
		if !ok || channel >= 128 {
			return ErrNoteVoiceUnavailable
		}
		domain := "music"
		if t.slot >= 10 {
			domain = "primary"
		}
		if t.slot >= 16 {
			domain = "secondary"
		}
		request := InstrumentRequest{Mode: mode, Domain: domain, Channel: channel, Claimed: v.Claimed, Work: work, Patch: patch, BaseLevel: v.BaseLevel, LoadedAlgorithm: v.LoadedAlgorithm}
		if err = validInstrumentRequest(request); err != nil {
			return err
		}
		route := request.Domain
		if t.canonicalDomain && domain != "music" {
			// The historical isolated route labels use positive for global
			// and negative for voice-local. Actual primary slots use negative.
			route = "secondary"
			if t.slot >= 16 {
				route = "primary"
			}
		}
		t.instrument, err = beginInstrumentOnRoute(s, request, route)
		if err != nil {
			return err
		}
		t.instrumentMode, t.phase = mode, noteInstrumentResource
		if v.Claimed {
			t.phase = noteInstrumentClaimed
		}
	case noteInstrumentClaimed:
		if !heldOperation(op, score.OpParameter, t.instrumentRoutine()) || op.Branch != score.BranchClaimed {
			return ErrNoteVoiceSequence
		}
		t.phase = noteInstrumentReturn
	case noteInstrumentResource:
		count, branch := uint16(25), score.BranchInitialLoad
		if t.instrumentMode == "level" {
			count, branch = 4, score.BranchOrdinaryLevel
		}
		if !heldOperation(op, score.OpResourceUnavailable, t.instrumentRoutine()) || op.DefinitionID != t.instrument.request.Work.ID || op.Iterations != count || op.Branch != branch {
			return ErrNoteVoiceSequence
		}
		t.phase = noteInstrumentBody
		if len(t.instrument.writes) == 0 {
			t.phase = noteInstrumentReturn
		}
	case noteInstrumentBody:
		write := t.instrument.writes[t.instrument.cursor]
		if !heldWrite(op, score.OpChipWrite, keyIntent(KeyEffect{Kind: "ym", YM: write}, t.intentOffset)) {
			return ErrNoteVoiceSequence
		}
		if err := t.instrument.WriteNext(write); err != nil {
			return err
		}
		if t.instrument.cursor == len(t.instrument.writes) {
			t.phase = noteInstrumentReturn
			if t.instrumentMode == "load" {
				t.phase = noteInstrumentPan
			}
		}
	case noteInstrumentPan:
		if _, err := t.instrument.Finish(); err != nil {
			return err
		}
		t.instrumentFinished = true
		if err := t.beginControlPan(v, "instrument"); err != nil {
			return err
		}
		if err := t.applyControlPan(op, v); err != nil {
			return err
		}
		t.phase = noteInstrumentReturn
	case noteInstrumentReturn:
		if !heldOperation(op, score.OpLeave, t.instrumentRoutine()) {
			return ErrNoteVoiceSequence
		}
		if !t.instrumentFinished {
			if _, err := t.instrument.Finish(); err != nil {
				return err
			}
			t.instrumentFinished = true
		}
		if t.pan != nil {
			if _, err := t.pan.Finish(); err != nil {
				return err
			}
			t.pan = nil
		}
		t.phase = noteControlReturn
	default:
		return ErrNoteVoiceSequence
	}
	return nil
}

func (t *NoteVoiceTransfer) instrumentRoutine() score.Routine {
	if t.instrumentMode == "level" {
		return score.RoutineLevel
	}
	return score.RoutinePatch
}

func (t *NoteVoiceTransfer) beginControlPan(v score.VoiceState, entry string) error {
	channel, ok := heldChannel(v.Channel)
	if v.Channel == "dac" {
		channel, ok = 6, true
	}
	if !ok || channel >= 128 {
		return ErrNoteVoiceUnavailable
	}
	request := PanRequest{Entry: entry, Channel: channel, Claimed: v.Claimed, Pan: v.Pan}
	if !validPanRequest(request) {
		return ErrNoteVoiceSequence
	}
	bus, _ := t.s.bus.(PanBus)
	if channel == 6 && bus == nil {
		return ErrNoteVoiceUnavailable
	}
	var err error
	t.pan, err = beginPanRouting(t.s, request, bus)
	return err
}

func (t *NoteVoiceTransfer) applyControlPan(op score.Operation, v score.VoiceState) error {
	effect, ok := t.pan.ExpectedEffect()
	if !ok {
		return ErrNoteVoiceSequence
	}
	intent := keyIntent(KeyEffect{Kind: "ym", YM: effect.Write}, t.intentOffset)
	if effect.Kind == "dac-pan" {
		intent = score.Intent{Order: t.intentOffset, Kind: "dac-pan", Pan: &score.Stereo{Left: v.Pan.Left, Right: v.Pan.Right}}
	}
	if !heldWrite(op, score.OpChipWrite, intent) {
		return ErrNoteVoiceSequence
	}
	return t.pan.Apply(effect)
}
