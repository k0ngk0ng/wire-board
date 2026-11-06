package scoreclock

import (
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type QueueField struct {
	Kind  string
	At    uint64
	Slot  int8
	Value uint8
}
type QueueResult struct {
	Clock  Clock
	Fields []QueueField
}
type queueClock struct {
	s        *service
	root     *rootcontrol.State
	queue    *[4]uint8
	sequence uint32
	fields   []QueueField
}

// ConsumeQueue starts at the declared consumer entry, after its caller.
// Canonical rules read the real unique owners; this service only times their
// actual effect leases and returns invocation-local diagnostics.
func ConsumeQueue(clock Clock, root *rootcontrol.State, queue *[4]uint8, table engine.SoundPriorityTable, limits Limits) (QueueResult, error) {
	if root == nil || queue == nil {
		return QueueResult{}, ErrRunningProfile
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return QueueResult{}, err
	}
	q := &queueClock{s: s, root: root, queue: queue}
	if err = s.ordinaryCosts(8, 8); err != nil {
		return QueueResult{}, err
	}
	err = engine.ConsumeSoundQueueObserved(engine.SoundQueueOwners{Priority: &root.Priority, Selected: &root.Selected, Queue: queue}, table, q.operation)
	if err != nil {
		return QueueResult{}, err
	}
	return QueueResult{Clock: s.clock, Fields: q.fields}, nil
}
func (q *queueClock) field(cpu uint64, op engine.SoundQueueOperation, apply func() error) error {
	return q.s.cost(cpu, func() error {
		if err := apply(); err != nil {
			return err
		}
		value := uint8(0)
		switch op.Kind {
		case engine.QueueReadPriority:
			value = q.root.Priority
		case engine.QueueReadSlot, engine.QueueClearSlot:
			value = q.queue[op.Slot]
		case engine.QueueSelectedWrite:
			value = q.root.Selected
		case engine.QueuePriorityWrite:
			value = q.root.Priority
		}
		slot := int8(-1)
		if op.Kind == engine.QueueReadSlot || op.Kind == engine.QueueClearSlot {
			slot = op.Slot
		}
		q.fields = append(q.fields, QueueField{Kind: string(op.Kind), At: q.s.clock.Cycle, Slot: slot, Value: value})
		return nil
	})
}
func (q *queueClock) operation(op engine.SoundQueueOperation, apply func() error) error {
	if op.Sequence != q.sequence || apply == nil {
		return ErrRunningSequence
	}
	q.sequence++
	s := q.s
	switch op.Kind {
	case engine.QueueReadPriority:
		if err := q.field(12, op, apply); err != nil {
			return err
		}
		return s.spend(4)
	case engine.QueueReadSlot:
		return q.field(10, op, apply)
	case engine.QueueClearSlot:
		if !op.Lower {
			if err := s.spend(4); err != nil {
				return err
			}
		}
		return q.field(12, op, apply)
	case engine.QueueConsider:
		if err := s.spend(4); err != nil {
			return err
		}
		if err := s.branch(op.Code == 0, false); err != nil {
			return err
		}
		if op.Code != 0 {
			if err := s.ordinaryCosts(8, 14, 8); err != nil {
				return err
			}
			if err := s.branch(op.Special, true); err != nil {
				return err
			}
			if !op.Lower && !op.Special {
				if err := s.ordinaryCosts(4, 8, 4, 8, 4); err != nil {
					return err
				}
				if err := s.branch(!op.Accepted, false); err != nil {
					return err
				}
			}
		}
		return apply()
	case engine.QueueSelectedWrite:
		if !op.Special {
			if err := s.spend(4); err != nil {
				return err
			}
		}
		if err := q.field(12, op, apply); err != nil {
			return err
		}
		if op.Special {
			return s.spend(10)
		}
		return nil
	case engine.QueueSlotDone:
		if err := s.pauseLoopEnd(op.Slot == 0); err != nil {
			return err
		}
		return apply()
	case engine.QueuePriorityDecision:
		if err := s.spend(4); err != nil {
			return err
		}
		if err := s.branch(op.Candidate >= 128, false); err != nil {
			return err
		}
		return apply()
	case engine.QueuePriorityWrite:
		return q.field(12, op, apply)
	case engine.QueueReturn:
		if err := s.spend(16); err != nil {
			return err
		}
		return apply()
	default:
		return ErrRunningSequence
	}
}
