package engine

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

type selfDribbleCall struct {
	worldActionFixture
	Sounds []uint8
}

func selfDribbleFixture(t *testing.T) ([]selfDribbleCall, selfDribbleCall) {
	t.Helper()
	var f struct {
		Calls []selfDribbleCall
		Stage selfDribbleCall
	}
	readPropAttackFixture(t, "testdata/self-dribble.json.gz", &f)
	if len(f.Calls) != 24 {
		t.Fatal("missing original self-dribble calls", len(f.Calls))
	}
	return f.Calls, f.Stage
}
func TestSelfDribbleOriginalCalls(t *testing.T) {
	rows, _ := selfDribbleFixture(t)
	for i, r := range rows {
		s := r.Before
		sounds, e := advanceWorldBallStep(&s, r.context(t), NewYorkBallCourt())
		if e != nil {
			t.Fatal(i, e)
		}
		if s != r.After {
			t.Fatalf("%d: %s", i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(sounds, r.Sounds) {
			t.Fatal("sounds", i)
		}
	}
}
func TestSelfDribbleOriginalWholeStage(t *testing.T) {
	_, r := selfDribbleFixture(t)
	result, e := StepWorldActions(r.Before, r.context(t))
	if e != nil {
		t.Fatal(e)
	}
	if result.State != r.After {
		t.Fatal(firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
	}
	if !slices.Equal(result.Calls, r.Calls) || !slices.Equal(result.SoundIDs, r.Sounds) {
		t.Fatal("scheduler or sound order", result.Calls, r.Calls, result.SoundIDs, r.Sounds)
	}
}
func TestSelfDribbleDoesNotBroadenHeldOwner(t *testing.T) {
	rows, _ := selfDribbleFixture(t)
	for _, action := range []BallAction{BallDribble, BallHeld} {
		for _, holder := range []int8{-1, 4, 5, 6, 7, 13} {
			// Both self-indexed source handlers now have independent original evidence.
			if holder == 6 {
				continue
			}
			s := rows[0].Before
			s.Ball.Action = action
			s.Ball.Request.Pending = false
			s.Holder = holder
			if _, e := advanceWorldBallStep(&s, rows[0].context(t), NewYorkBallCourt()); e == nil {
				t.Fatal("unsupported holder accepted", action, holder)
			}
		}
	}
}
func TestSelfDribbleRealFailureContinuation(t *testing.T) {
	var s RuntimeState
	readPropAttackFixture(t, "testdata/self-dribble-runtime-before.json.gz", &s)
	if s.VideoFrames != 19313 {
		t.Fatal("wrong first failure state", s.VideoFrames)
	}
	for i := 0; i < 300; i++ {
		encoded, e := json.Marshal(s)
		if e != nil {
			t.Fatal(e)
		}
		var restored RuntimeState
		if e = json.Unmarshal(encoded, &restored); e != nil {
			t.Fatal(e)
		}
		a, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(i, s.VideoFrames, e)
		}
		b, e := StepVideo(restored, [4]Buttons{})
		if e != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("JSON replay", i, e)
		}
		s = a.State
	}
}
