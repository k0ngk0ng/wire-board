package engine

import "errors"

type SoundQueueOperationKind string

const (
	QueueReadPriority     SoundQueueOperationKind = "priority-read"
	QueueReadSlot         SoundQueueOperationKind = "queue-read"
	QueueClearSlot        SoundQueueOperationKind = "queue-clear"
	QueueConsider         SoundQueueOperationKind = "consider"
	QueueSelectedWrite    SoundQueueOperationKind = "selected-write"
	QueueSlotDone         SoundQueueOperationKind = "slot-done"
	QueuePriorityDecision SoundQueueOperationKind = "priority-decision"
	QueuePriorityWrite    SoundQueueOperationKind = "priority-write"
	QueueReturn           SoundQueueOperationKind = "return"
)

type SoundQueueOperation struct {
	Sequence                  uint32
	Kind                      SoundQueueOperationKind
	Slot                      int8
	Code, Priority, Candidate uint8
	Lower, Special, Accepted  bool
}
type SoundQueueOwners struct {
	Priority, Selected *uint8
	Queue              *[4]uint8
}
type SoundQueueHook func(SoundQueueOperation, func() error) error

var ErrSoundQueueSequence = errors.New("invalid sound queue operation sequence")

// ConsumeSoundQueueObserved reads and writes the actual owners in source order.
// Locals hold only this invocation's sampled code/candidate. Hook effects are
// synchronous one-shot leases; there is no second queue or execution cursor.
func ConsumeSoundQueueObserved(owners SoundQueueOwners, priorities SoundPriorityTable, hook SoundQueueHook) error {
	if owners.Priority == nil || owners.Selected == nil || owners.Queue == nil {
		return ErrSoundQueueSequence
	}
	sequence := uint32(0)
	emit := func(op SoundQueueOperation, effect func() error) error {
		op.Sequence = sequence
		sequence++
		if hook == nil {
			return effect()
		}
		active, called, violated := true, false, false
		apply := func() error {
			if !active || called {
				violated = true
				return ErrSoundQueueSequence
			}
			called = true
			return effect()
		}
		err := hook(op, apply)
		active = false
		if err != nil {
			return err
		}
		if !called || violated {
			return ErrSoundQueueSequence
		}
		return nil
	}
	noop := func() error { return nil }
	var candidate uint8
	if err := emit(SoundQueueOperation{Kind: QueueReadPriority, Slot: -1}, func() error { candidate = *owners.Priority; return nil }); err != nil {
		return err
	}
	special := false
	for slot := int8(3); slot >= 0; slot-- {
		var code uint8
		if err := emit(SoundQueueOperation{Kind: QueueReadSlot, Slot: slot, Lower: special}, func() error { code = owners.Queue[slot]; return nil }); err != nil {
			return err
		}
		priority := uint8(0)
		if code != 0 {
			priority = priorities[code-1]
		}
		if special {
			if err := emit(SoundQueueOperation{Kind: QueueConsider, Slot: slot, Code: code, Priority: priority, Candidate: candidate, Lower: true, Special: code != 0 && priority == 255}, noop); err != nil {
				return err
			}
			if code == 0 || priority != 255 {
				if err := emit(SoundQueueOperation{Kind: QueueClearSlot, Slot: slot, Code: code, Lower: true}, func() error { owners.Queue[slot] = 0; return nil }); err != nil {
					return err
				}
			}
		} else {
			if err := emit(SoundQueueOperation{Kind: QueueClearSlot, Slot: slot, Code: code}, func() error { owners.Queue[slot] = 0; return nil }); err != nil {
				return err
			}
			accepted := code != 0 && (priority == 255 || priority&127 >= candidate&127)
			if err := emit(SoundQueueOperation{Kind: QueueConsider, Slot: slot, Code: code, Priority: priority, Candidate: candidate, Special: code != 0 && priority == 255, Accepted: accepted}, noop); err != nil {
				return err
			}
			if accepted {
				if priority == 255 {
					special = true
				} else {
					candidate = priority
				}
				if err := emit(SoundQueueOperation{Kind: QueueSelectedWrite, Slot: slot, Code: code, Priority: priority, Candidate: candidate, Special: priority == 255}, func() error { *owners.Selected = code; return nil }); err != nil {
					return err
				}
			}
		}
		if err := emit(SoundQueueOperation{Kind: QueueSlotDone, Slot: slot, Lower: special}, noop); err != nil {
			return err
		}
	}
	if !special {
		if err := emit(SoundQueueOperation{Kind: QueuePriorityDecision, Slot: -1, Candidate: candidate}, noop); err != nil {
			return err
		}
		if candidate < 128 {
			if err := emit(SoundQueueOperation{Kind: QueuePriorityWrite, Slot: -1, Candidate: candidate}, func() error { *owners.Priority = candidate; return nil }); err != nil {
				return err
			}
		}
	}
	return emit(SoundQueueOperation{Kind: QueueReturn, Slot: -1, Special: special}, noop)
}
