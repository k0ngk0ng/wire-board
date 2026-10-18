package engine

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

type selfHeldBallOracle struct {
	worldActionFixture
	Prepared, AfterMotion WorldActionState
	Sounds                []uint8
}

func selfHeldBallRows(t *testing.T) ([]selfHeldBallOracle, []selfHeldBallOracle, selfHeldBallOracle) {
	t.Helper()
	var f struct {
		Calls, Tails []selfHeldBallOracle
		Stage        selfHeldBallOracle
	}
	readPropAttackFixture(t, "testdata/self-held-ball.json.gz", &f)
	if len(f.Calls) != 432 || len(f.Tails) != 48 {
		t.Fatal("missing original self-held cases", len(f.Calls), len(f.Tails))
	}
	return f.Calls, f.Tails, f.Stage
}
func TestSelfHeldBallOriginalCalls(t *testing.T) {
	rows, _, _ := selfHeldBallRows(t)
	for i, r := range rows {
		s := r.Before
		result := WorldActionStep{State: s}
		court, e := originalWorldMotionCourt(&result, r.context(t), 6)
		if e != nil {
			t.Fatal(e)
		}
		sounds, e := advanceWorldBallStep(&result.State, r.context(t), court)
		if e != nil {
			t.Fatal(i, e)
		}
		if result.State != r.After {
			t.Fatal(i, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(sounds, r.Sounds) {
			t.Fatal(i, "sounds", sounds, r.Sounds)
		}
	}
}
func TestSelfHeldBallReadsMotionResult(t *testing.T) {
	_, tails, _ := selfHeldBallRows(t)
	for i, r := range tails {
		got, e := applySelfReferencedHeldBallOffset(r.Before.Ball)
		if e != nil || got != r.After.Ball {
			t.Fatal(i, "post-motion source tail", got, r.After.Ball, e)
		}
	}
}
func TestSelfHeldBallPreparationAndOwnerBoundary(t *testing.T) {
	rows, _, _ := selfHeldBallRows(t)
	for _, r := range rows {
		result := WorldActionStep{State: r.Before}
		c := r.context(t)
		seen := false
		court := FlatCourt{originalMotion: &courtMotionBinding{object: 6, court: 1,
			load: func() CourtFloorState {
				seen = true
				if result.State != r.Prepared {
					t.Fatal("late preparation", firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.Prepared), "world"))
				}
				return worldFloorState(result.State, c)
			}, store: func(s CourtFloorState, selected uint8) { storeWorldFloorState(&result.State, s, selected) },
		}}
		if _, e := advanceWorldBallStep(&result.State, c, court); e != nil {
			t.Fatal(e)
		}
		if !seen || result.State != r.After {
			t.Fatal("world motion boundary", seen)
		}
	}
	b := rows[0].Prepared.Ball
	if _, e := completeSelfReferencedHeldBall(b, NewYorkBallCourt(), func() int8 { return 3 }); e == nil {
		t.Fatal("changed motion owner silently aliased to self")
	}
}
func TestSelfHeldBallOriginalWholeStage(t *testing.T) {
	_, _, r := selfHeldBallRows(t)
	result, e := StepWorldActions(r.Before, r.context(t))
	if e != nil {
		t.Fatal(e)
	}
	if result.State != r.After {
		t.Fatal(firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
	}
	if !slices.Equal(result.Calls, r.Calls) || !slices.Equal(result.SoundIDs, r.Sounds) {
		t.Fatal("original phase ordering", result.Calls, r.Calls, result.SoundIDs, r.Sounds)
	}
}
func TestSelfHeldBallExactMode128FailureContinuation(t *testing.T) {
	var s RuntimeState
	readPropAttackFixture(t, "testdata/self-held-ball-runtime-before.json.gz", &s)
	if s.VideoFrames != 21719 || s.Config.Match.Mode != 128 {
		t.Fatal("wrong failure origin", s.VideoFrames, s.Config.Match.Mode)
	}
	for i := 0; i < 300; i++ {
		raw, e := json.Marshal(s)
		if e != nil {
			t.Fatal(e)
		}
		var restored RuntimeState
		if e = json.Unmarshal(raw, &restored); e != nil {
			t.Fatal(e)
		}
		a, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(i, s.VideoFrames, e)
		}
		b, e := StepVideo(restored, [4]Buttons{})
		if e != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("restore", i, e)
		}
		s = a.State
	}
}
