package score_test

import (
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

func controlReviewCall(tx *score.TickTransaction, stage string) (score.Output, error) {
	switch stage {
	case "tempo":
		return score.Output{}, tx.AdvanceTempo()
	case "control":
		return score.Output{}, tx.ApplyMusicControl(score.ActionFadeMusicStep)
	case "dispatch":
		return score.Output{}, tx.Dispatch(0)
	case "voices":
		return score.Output{}, tx.AdvanceVoices()
	case "finish":
		return tx.Finish()
	case "suspended":
		return tx.FinishSuspended()
	}
	panic("unknown review stage")
}

func controlReviewMatch(stage string, op score.Operation) bool {
	switch stage {
	case "tempo":
		return op.Kind == score.OpEnter && op.Routine == score.RoutineTempo
	case "control":
		return op.Kind == score.OpEnter && op.Routine == score.RoutineMusicControl
	case "dispatch":
		return op.Kind == score.OpParameter && op.Routine == score.RoutineDispatch && op.Branch == score.BranchNoDispatch
	case "voices":
		return op.Kind == score.OpDomain && op.Routine == score.RoutineTick
	case "finish", "suspended":
		return op.Kind == score.OpLeave && op.Routine == score.RoutineTick
	}
	return false
}

func controlReviewPrepare(t *testing.T, tx *score.TickTransaction, stage string) {
	t.Helper()
	if stage == "tempo" || stage == "suspended" {
		return
	}
	if err := tx.AdvanceTempo(); err != nil {
		t.Fatal(err)
	}
	if stage == "control" || stage == "dispatch" {
		return
	}
	if err := tx.Dispatch(0); err != nil {
		t.Fatal(err)
	}
	if stage == "finish" {
		if err := tx.AdvanceVoices(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestControlReviewSameTransactionReentryPoisonsOuterAndHoldsLease(t *testing.T) {
	for _, stage := range []string{"tempo", "control", "dispatch", "voices", "finish", "suspended"} {
		t.Run(stage, func(t *testing.T) {
			_, d := controlSeed(t)
			before := d.Snapshot()
			var tx *score.TickTransaction
			var nestedErr, hijackErr error
			armed, attempted, inside := false, false, false
			nestedOperations, nestedReads := 0, 0
			hooks := controlHooks()
			hooks.ReadDAC = func(score.FeedbackRead) (score.DACFeedback, error) {
				if inside {
					nestedReads++
				}
				return score.DACFeedback{}, nil
			}
			hooks.Operation = func(op score.Operation) error {
				if inside {
					nestedOperations++
				}
				if !armed || attempted || !controlReviewMatch(stage, op) {
					return nil
				}
				attempted = true
				working, err := tx.Snapshot()
				if err != nil {
					t.Errorf("read-only working snapshot in callback rejected: %v", err)
				} else {
					changed := working
					changed.Voices[1].BaseLevel++
					again, err := tx.Snapshot()
					if err != nil || again != working || changed == again {
						t.Error("working snapshot aliases transaction")
					}
				}
				inside = true
				_, nestedErr = controlReviewCall(tx, stage)
				// Rejection must poison the outer call but retain its lease until
				// that call unwinds; otherwise this callback can steal authority.
				_, hijackErr = d.Advance(score.Input{})
				inside = false
				return nil // Deliberately do not propagate the nested error.
			}
			var err error
			tx, err = d.BeginTickObserved(hooks)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Abort()
			controlReviewPrepare(t, tx, stage)
			armed = true
			out, outerErr := controlReviewCall(tx, stage)
			if !attempted {
				t.Fatal("review callback site was not reached")
			}
			if nestedErr == nil || outerErr == nil {
				t.Errorf("same transaction reentry accepted or outer not poisoned: nested=%v outer=%v", nestedErr, outerErr)
			}
			if hijackErr == nil {
				t.Error("driver lease released inside running callback")
			}
			if nestedOperations != 0 || nestedReads != 0 {
				t.Errorf("reentry executed more external callbacks: operations=%d reads=%d", nestedOperations, nestedReads)
			}
			if len(out.Intents) != 0 || out.ClearPrimaryPriority || d.Snapshot() != before {
				t.Errorf("failed stage committed score or partial output: ticks=%d want=%d", d.Snapshot().Ticks, before.Ticks)
			}
			if _, err := tx.Snapshot(); err == nil {
				t.Error("poisoned transaction remained open")
			}
			if _, err := d.Advance(score.Input{}); err != nil {
				t.Errorf("outer return did not release driver lease: %v", err)
			}
		})
	}
}

func TestControlReviewAbortInCallbackRetainsLeaseUntilOuterReturn(t *testing.T) {
	_, d := controlSeed(t)
	before := d.Snapshot()
	var tx *score.TickTransaction
	var hijackErr error
	armed, aborted := false, false
	hooks := controlHooks()
	hooks.Operation = func(op score.Operation) error {
		if armed && !aborted && op.Kind == score.OpEnter && op.Routine == score.RoutineTempo {
			aborted = true
			tx.Abort()
			_, hijackErr = d.Advance(score.Input{})
		}
		return nil
	}
	var err error
	tx, err = d.BeginTickObserved(hooks)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Abort()
	armed = true
	err = tx.AdvanceTempo()
	if !aborted || err == nil {
		t.Fatal("callback Abort did not cancel outer stage", err)
	}
	if hijackErr == nil || d.Snapshot() != before {
		t.Error("callback Abort released lease early and allowed authority hijack")
	}
	if _, err := tx.Snapshot(); err == nil {
		t.Error("aborted working branch remained available")
	}
	if _, err := d.Advance(score.Input{}); err != nil {
		t.Error("aborted outer return retained driver lease", err)
	}
}
