package engine

import (
	"errors"
	"testing"
)

func TestSoundSchedulerObservedOriginalPhases(t *testing.T) {
	for _, row := range soundSchedulerFixture(t) {
		state, queue := soundSchedulerDecode(row.Before), row.Before.Queue
		h := SoundSchedulerHooks{Operation: func(op SoundScheduleOperation, apply func() error) error { return apply() }, Request: func(op SoundRequestOperation, apply func() error) error { return apply() }}
		var err error
		switch row.Kind {
		case "interrupt", "begin-interrupt":
			var drive bool
			drive, err = BeginSoundInterruptObserved(SoundSchedulerOwners{State: &state, Queue: &queue}, h)
			if err == nil && drive != (len(row.Drives) != 0) {
				t.Fatal("observed driver decision differs", row.Name)
			}
			if err == nil && drive {
				if state != soundSchedulerDecode(row.Drives[0]) || queue != row.Drives[0].Queue {
					t.Fatal("observed scheduler call fields differ", row.Name)
				}
				if row.Kind == "interrupt" {
					err = FinishSoundInterruptObserved(&state, h.Operation)
				}
			}
		case "catchup":
			for range row.Drives {
				var drive bool
				drive, err = BeginSoundCatchupObserved(&state, h.Operation)
				if err != nil || !drive {
					t.Fatal("observed catchup skipped driver", row.Name, err)
				}
				if err = FinishSoundCatchupObserved(&state, h.Operation); err != nil {
					break
				}
			}
			if err == nil {
				var drive bool
				drive, err = BeginSoundCatchupObserved(&state, h.Operation)
				if drive {
					t.Fatal("observed catchup failed to stop", row.Name)
				}
			}
		case "begin-catchup":
			var drive bool
			drive, err = BeginSoundCatchupObserved(&state, h.Operation)
			if drive != (len(row.Drives) != 0) {
				t.Fatal("observed catchup decision differs", row.Name)
			}
		case "finish-interrupt":
			err = FinishSoundInterruptObserved(&state, h.Operation)
		case "finish-catchup":
			err = FinishSoundCatchupObserved(&state, h.Operation)
		default:
			t.Fatal("unknown scheduler original phase")
		}
		if err != nil || state != soundSchedulerDecode(row.After) || queue != row.After.Queue {
			t.Fatal("observed scheduler changed original", row.Name, err, state, row.After)
		}
	}
}

func TestSoundRequestLiveReadsAndSchedulerLeases(t *testing.T) {
	q := [4]uint8{130, 131, 132, 133}
	reads := 0
	accepted, err := EnqueueSoundObserved(&q, 6, func(op SoundRequestOperation, apply func() error) error {
		if op.Kind == RequestCompareRead && op.Slot == 0 {
			q[0] = 6
			reads++
		}
		return apply()
	})
	if err != nil || !accepted || q[0] != 6 || reads != 1 {
		t.Fatal("request compare did not read live queue", q, accepted, err)
	}
	for _, mode := range []string{"missing", "twice", "expired", "error"} {
		state := SoundSchedulerState{Delay: 1}
		queue := [4]uint8{}
		var saved func() error
		_, err := BeginSoundInterruptObserved(SoundSchedulerOwners{State: &state, Queue: &queue}, SoundSchedulerHooks{Operation: func(op SoundScheduleOperation, apply func() error) error {
			if op.Kind != ScheduleDelayRead {
				return apply()
			}
			switch mode {
			case "missing":
				return nil
			case "twice":
				if e := apply(); e != nil {
					return e
				}
				_ = apply()
				return nil
			case "error":
				return errors.New("scheduler observation failed")
			default:
				saved = apply
				return apply()
			}
		}})
		if mode == "expired" {
			if err != nil || saved == nil || saved() == nil {
				t.Fatal("expired scheduler effect usable", err)
			}
		} else if err == nil {
			t.Fatal("invalid scheduler lease accepted", mode)
		}
	}
	state := SoundSchedulerState{Backlog: 1, Busy: 1}
	err = FinishSoundCatchupObserved(&state, func(op SoundScheduleOperation, apply func() error) error {
		if op.Kind == ScheduleBacklogDecrement {
			state.Backlog = 9
		}
		return apply()
	})
	if err != nil || state.Backlog != 8 || state.Busy != 0 {
		t.Fatal("catchup decremented stale backlog", state, err)
	}
}
