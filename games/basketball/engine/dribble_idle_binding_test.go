package engine

import (
	"reflect"
	"slices"
	"testing"
)

type dribbleBindingOracle struct {
	worldActionFixture
	Actor        uint8
	Bound        bool
	BeforeMotion WorldActionState
	Sounds       []uint8
}

func dribbleBindingRows(t *testing.T) ([]dribbleBindingOracle, dribbleBindingOracle) {
	t.Helper()
	var f struct {
		Calls []dribbleBindingOracle
		Stage dribbleBindingOracle
	}
	readPropAttackFixture(t, "testdata/dribble-idle-binding.json.gz", &f)
	if len(f.Calls) != 360 {
		t.Fatal("missing original binding cases", len(f.Calls))
	}
	return f.Calls, f.Stage
}
func TestOriginalDribbleIdleBindingCalls(t *testing.T) {
	rows, _ := dribbleBindingRows(t)
	for i, r := range rows {
		c := r.context(t)
		result := WorldActionStep{State: r.Before}
		court, e := originalWorldMotionCourt(&result, c, r.Actor)
		if e != nil {
			t.Fatal(e)
		}
		if e = advanceWorldPlayer(&result, r.Actor, c, court); e != nil {
			t.Fatal(i, e)
		}
		if result.State != r.After {
			t.Fatal(i, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
		}
	}
}
func TestOriginalDribbleIdleBindCommand(t *testing.T) {
	rows, _ := dribbleBindingRows(t)
	for i, r := range rows {
		a := r.Before.Actors[r.Actor]
		c := r.context(t)
		got, e := StepCarrierAction(Carrier{Player: a.Player, ShotCycle: a.ShotCycle}, CarrierContext{HasBall: r.Before.Holder == int8(r.Actor), Computer: c.Controls[r.Actor]&128 != 0, BallAction: r.Before.Ball.Action}, c.Profiles[r.Actor], NewYorkGroundCourt())
		if e != nil || got.BindBall != r.Bound {
			t.Fatal(i, "entry command", got.BindBall, r.Bound, e)
		}
	}
}
func TestOriginalDribbleIdleBindingBeforeMotion(t *testing.T) {
	rows, _ := dribbleBindingRows(t)
	for i, r := range rows {
		result := WorldActionStep{State: r.Before}
		c := r.context(t)
		seen := false
		court := FlatCourt{originalMotion: &courtMotionBinding{object: r.Actor, court: 1, load: func() CourtFloorState {
			seen = true
			if result.State.Holder != r.BeforeMotion.Holder {
				t.Fatal(i, "ownership committed after source motion boundary", result.State.Holder, r.BeforeMotion.Holder)
			}
			return worldFloorState(result.State, c)
		}, store: func(s CourtFloorState, selected uint8) { storeWorldFloorState(&result.State, s, selected) }}}
		if e := advanceWorldPlayer(&result, r.Actor, c, court); e != nil {
			t.Fatal(e)
		}
		if !seen {
			t.Fatal("missing original motion")
		}
	}
}
func TestOriginalDribbleIdleBindingWholePhase(t *testing.T) {
	_, r := dribbleBindingRows(t)
	got, e := StepWorldActions(r.Before, r.context(t))
	if e != nil {
		t.Fatal(e)
	}
	if got.State != r.After {
		t.Fatal(firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.After), "world"))
	}
	if !slices.Equal(got.Calls, r.Calls) || !slices.Equal(got.SoundIDs, r.Sounds) {
		t.Fatal("stage scheduling")
	}
}
func TestOriginalSelfHeldContinuousWorlds(t *testing.T) {
	var s RuntimeState
	readPropAttackFixture(t, "testdata/self-held-ball-runtime-before.json.gz", &s)
	var expected []WorldLoopState
	readPropAttackFixture(t, "testdata/self-held-original-worlds.json.gz", &expected)
	if len(expected) != 66 {
		t.Fatal("incomplete original full-machine chain", len(expected))
	}
	for i, want := range expected {
		if i > 0 {
			for {
				step, e := StepVideo(s, [4]Buttons{})
				if e != nil {
					t.Fatal(i, e)
				}
				s = step.State
				if step.LogicAdvanced {
					break
				}
			}
		}

		var got WorldLoopState
		if s.Playing != nil {
			got = s.Playing.World
		} else {
			if i != len(expected)-1 || s.Carry == nil || s.Transition == nil || s.Transition.Scene.Transition.Match.Phase != MatchPeriodEnding {
				t.Fatal("unexpected transition", i)
			}
			got = *s.Carry
		}
		// Original zero-page candidates are scratch after preparation; projection
		// has independent visual oracles. All persistent gameplay fields stay checked.
		want.Preparation.Contacts.Candidates = got.Preparation.Contacts.Candidates
		want.Projection = got.Projection
		if !reflect.DeepEqual(got, want) {
			t.Fatal(i, firstWorldLoopDifference(reflect.ValueOf(got), reflect.ValueOf(want), "world"))
		}
	}
}
