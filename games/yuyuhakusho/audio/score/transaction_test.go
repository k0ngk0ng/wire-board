package score_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

func TestTickTransactionStagesMatchExistingAdvance(t *testing.T) {
	c := catalog(t)
	staged, _ := score.New(c)
	legacy, _ := score.New(c)
	for step := 0; step < 220; step++ {
		id := uint8(0)
		if step == 0 {
			id = 0x83
		}
		if step == 40 {
			id = 0xa7
		}
		before := staged.Snapshot()
		tx, err := staged.BeginTickObserved(controlHooks())
		if err != nil {
			t.Fatal(err)
		}
		if err = tx.AdvanceTempo(); err != nil {
			t.Fatal(err)
		}
		if staged.Snapshot() != before {
			t.Fatal("working phase leaked before Finish")
		}
		working, err := tx.Snapshot()
		if err != nil || working.Ticks != before.Ticks {
			t.Fatal("stage advanced tick early")
		}
		if err = tx.Dispatch(id); err != nil {
			t.Fatal(err)
		}
		if err = tx.AdvanceVoices(); err != nil {
			t.Fatal(err)
		}
		out, err := tx.Finish()
		if err != nil {
			t.Fatal(err)
		}
		want, err := legacy.AdvanceObserved(score.ObservedInput{SoundID: id}, controlHooks())
		if err != nil || !reflect.DeepEqual(out, want) || staged.Snapshot() != legacy.Snapshot() {
			t.Fatal("staged tick differs", step, err)
		}
	}
}

func TestTickTransactionControlDoesNotRepeatTempo(t *testing.T) {
	c, d := controlSeed(t)
	before := d.Snapshot()
	manual, _ := score.Restore(c, before)
	manual.ApplyMusicControlObserved(score.ActionStopMusicDACFlags, controlHooks())
	manualBefore := manual.Snapshot()
	tx, err := d.BeginTickObserved(controlHooks())
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.AdvanceTempo(); err != nil {
		t.Fatal(err)
	}
	if err = tx.ApplyMusicControl(score.ActionStopMusicDACFlags); err != nil {
		t.Fatal(err)
	}
	if err = tx.Dispatch(0); err != nil {
		t.Fatal(err)
	}
	if err = tx.AdvanceVoices(); err != nil {
		t.Fatal(err)
	}
	out, err := tx.Finish()
	if err != nil {
		t.Fatal(err)
	}
	want, err := manual.Advance(score.Input{})
	if err != nil || !reflect.DeepEqual(out, want) || d.Snapshot() != manual.Snapshot() || d.Snapshot().Ticks != manualBefore.Ticks+1 {
		t.Fatal("control/tick repeated or skipped a normal phase", err)
	}
}

func TestTickTransactionSuspendedAndLease(t *testing.T) {
	_, d := controlSeed(t)
	before := d.Snapshot()
	tx, err := d.BeginTickObserved(controlHooks())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.BeginTickObserved(controlHooks()); err == nil {
		t.Fatal("second live score transaction accepted")
	}
	if _, err = d.Advance(score.Input{}); err == nil {
		t.Fatal("Advance reentered leased driver")
	}
	if _, err = d.ApplyMusicControlObserved(score.ActionResetScore, controlHooks()); err == nil {
		t.Fatal("control reentered leased driver")
	}
	out, err := tx.FinishSuspended()
	if err != nil {
		t.Fatal(err)
	}
	want := before
	want.Ticks++
	if d.Snapshot() != want || len(out.Intents) != 0 {
		t.Fatal("suspended tick advanced voices or did not count once")
	}
	if _, err = tx.FinishSuspended(); err == nil || d.Snapshot() != want {
		t.Fatal("double finish counted tick again")
	}
}

func TestTickTransactionWrongOrderAndAbortKeepScore(t *testing.T) {
	for _, stage := range []string{"dispatch-first", "voices-first", "control-first", "finish-first", "tempo-twice", "suspend-after-tempo", "abort"} {
		t.Run(stage, func(t *testing.T) {
			_, d := controlSeed(t)
			before := d.Snapshot()
			tx, err := d.BeginTickObserved(controlHooks())
			if err != nil {
				t.Fatal(err)
			}
			switch stage {
			case "dispatch-first":
				err = tx.Dispatch(0)
			case "voices-first":
				err = tx.AdvanceVoices()
			case "control-first":
				err = tx.ApplyMusicControl(score.ActionFadeMusicStep)
			case "finish-first":
				_, err = tx.Finish()
			case "tempo-twice":
				if err = tx.AdvanceTempo(); err == nil {
					err = tx.AdvanceTempo()
				}
			case "suspend-after-tempo":
				if err = tx.AdvanceTempo(); err == nil {
					_, err = tx.FinishSuspended()
				}
			case "abort":
				tx.AdvanceTempo()
				tx.Abort()
				err = tx.Dispatch(0)
			}
			if err == nil || d.Snapshot() != before {
				t.Fatal("bad stage partially committed")
			}
			if _, err = d.Advance(score.Input{}); err != nil {
				t.Fatal("closed transaction retained driver lease", err)
			}
		})
	}
}

func TestTickTransactionFailureClosesWithoutScoreCommit(t *testing.T) {
	_, d := controlSeed(t)
	before := d.Snapshot()
	fail := false
	sentinel := errors.New("failed staged service")
	hooks := controlHooks()
	hooks.Operation = func(score.Operation) error {
		if fail {
			return sentinel
		}
		return nil
	}
	tx, err := d.BeginTickObserved(hooks)
	if err != nil {
		t.Fatal(err)
	}
	tx.AdvanceTempo()
	fail = true
	if err = tx.Dispatch(0); err == nil || d.Snapshot() != before {
		t.Fatal("failed stage committed score")
	}
	if _, err = tx.Snapshot(); err == nil {
		t.Fatal("failed transaction still exposes a working branch")
	}
	if _, err = d.Advance(score.Input{}); err != nil {
		t.Fatal("failed transaction retained lease", err)
	}
}

func TestMusicResetHeaderEffectsAreExplicitAndOrdered(t *testing.T) {
	c := catalog(t)
	d, _ := score.New(c)
	priority := uint8(0x22)
	selected := uint8(0x82)
	latch := uint8(0)
	events := []score.OperationKind{}
	hooks := controlHooks()
	hooks.Operation = func(op score.Operation) error {
		switch op.Kind {
		case score.OpPriorityClear:
			priority = 0
		case score.OpPrioritySave:
			latch = priority
			events = append(events, op.Kind)
		case score.OpHeaderClear:
			if op.Routine != score.RoutineMusicReset || op.Iterations != 16 {
				t.Fatal("unexpected header bulk shape")
			}
			priority = 0
			selected = 0
			events = append(events, op.Kind)
		case score.OpPriorityRestore:
			priority = latch
			events = append(events, op.Kind)
		case score.OpSelectedReset:
			if op.SoundID != 0xff {
				t.Fatal("selected idle value missing")
			}
			selected = op.SoundID
			events = append(events, op.Kind)
		}
		if op.WriteCount == 1 && op.Writes[0].Kind == "ym" && op.Writes[0].Register == 0x27 && op.IntentOffset == 1 {
			priority = 0x55
		}
		return nil
	}
	if _, err := d.AdvanceObserved(score.ObservedInput{SoundID: 0x82}, hooks); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []score.OperationKind{score.OpPrioritySave, score.OpHeaderClear, score.OpPriorityRestore, score.OpSelectedReset}) || priority != 0x55 || selected != 0xff {
		t.Fatal("header reset/save/selected order lost", events, priority, selected)
	}
}
