package scoreclock

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

var ErrVoiceSequence = errors.New("invalid complete voice sequence")

// VoiceTransfer binds actual driver slots and the current working score to
// the same held/new-note services. It also handles cross-voice restoration.
// Whole driver/world timing remains the enclosing coordinator's responsibility.
type VoiceTransfer struct {
	note    *NoteVoiceTransfer
	held    *HeldVoiceTransfer
	stop    *trackEndTransfer
	s       *service
	slot    int8
	done    bool
	failure error
}

func BeginVoice(clock Clock, op score.Operation, state score.State, c *score.Catalog, bus Bus, chip KeyGateChip, limits Limits) (*VoiceTransfer, error) {
	if op.VoiceSlot < 0 || int(op.VoiceSlot) >= len(state.Voices) {
		return nil, ErrVoiceSequence
	}
	v := state.Voices[op.VoiceSlot]
	t := &VoiceTransfer{slot: op.VoiceSlot}
	var err error
	if v.NoteWait == 1 {
		t.note, err = BeginNoteVoice(clock, op, v, c, bus, chip, limits)
		if err != nil {
			return nil, err
		}
		t.note.allowStop, t.note.canonicalDomain = true, true
		t.s = t.note.s
	} else {
		t.held, err = BeginHeldVoice(clock, op, v, c, bus, chip, limits)
		if err != nil {
			return nil, err
		}
		t.s = t.held.s
	}
	return t, nil
}
func (t *VoiceTransfer) Clock() Clock         { return t.s.clock }
func (t *VoiceTransfer) fail(err error) error { t.failure = err; return err }
func (t *VoiceTransfer) Consume(op score.Operation, state score.State) error {
	if t == nil {
		return ErrVoiceSequence
	}
	if t.done || t.failure != nil {
		return t.fail(ErrVoiceSequence)
	}
	var err error
	if t.stop != nil {
		err = t.stop.Consume(op, state)
	} else if t.note != nil && t.note.phase == noteTrackEnd {
		t.stop, err = beginTrackEnd(t.s, op, state, t.note.catalog, t.note.chip)
	} else {
		if op.VoiceSlot < 0 || int(op.VoiceSlot) >= len(state.Voices) {
			return t.fail(ErrVoiceSequence)
		}
		if t.note != nil {
			err = t.note.Consume(op, state.Voices[op.VoiceSlot])
		} else {
			err = t.held.Consume(op, state.Voices[op.VoiceSlot])
		}
	}
	if err != nil {
		return t.fail(err)
	}
	return nil
}
func (t *VoiceTransfer) ReadDAC(read score.FeedbackRead) (score.DACFeedback, error) {
	if t == nil || t.done || t.failure != nil || t.note == nil || t.stop != nil {
		return score.DACFeedback{}, ErrVoiceSequence
	}
	return t.note.ReadDAC(read)
}
func (t *VoiceTransfer) QueueReplace(op score.Operation, apply func() error) error {
	if t == nil {
		return ErrVoiceSequence
	}
	if t.done || t.failure != nil || t.note == nil || t.note.phase != noteQueuePending || apply == nil {
		return t.fail(ErrVoiceSequence)
	}
	n := t.note
	expected := n.queueOp
	if op.Sequence != expected.Sequence || op.VoiceSlot != expected.VoiceSlot || op.Position != expected.Position || op.IntentOffset != expected.IntentOffset || op.Kind != expected.Kind || op.WriteCount != 1 || !sameObservedIntent(op.Writes[0], expected.Writes[0]) {
		return t.fail(ErrVoiceSequence)
	}
	if err := t.s.cost(16, apply); err != nil {
		return t.fail(err)
	}
	if err := t.s.spend(16); err != nil {
		return t.fail(err)
	}
	n.phase = noteControlReturn
	n.queueOp = score.Operation{}
	return nil
}
func (t *VoiceTransfer) PriorityClear(op score.Operation, apply func() error) error {
	if t == nil {
		return ErrVoiceSequence
	}
	if t.done || t.failure != nil || t.stop == nil {
		return t.fail(ErrVoiceSequence)
	}
	if err := t.stop.PriorityClear(op, apply); err != nil {
		return t.fail(err)
	}
	return nil
}
func (t *VoiceTransfer) EndsDriverTick() bool { return t != nil && t.stop != nil && t.slot == 0 }
func (t *VoiceTransfer) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrVoiceSequence
	}
	if t.done || t.failure != nil {
		return Clock{}, t.fail(ErrVoiceSequence)
	}
	var clock Clock
	var err error
	if t.stop != nil {
		clock, err = t.stop.Finish()
	} else if t.note != nil {
		clock, err = t.note.Finish()
	} else {
		clock, err = t.held.Finish()
	}
	if err != nil {
		return Clock{}, t.fail(err)
	}
	t.done = true
	return clock, nil
}
