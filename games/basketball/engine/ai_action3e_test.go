package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalAction3EDispatchAndPossession(t *testing.T) {
	f, err := os.Open("testdata/ai-action3e-function.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Gates []struct {
			Mode, Actor, Holder, AfterMode   uint8
			Entered, ActionEntered, Dispatch bool
		}
		Waits []struct {
			Actor, Holder, AfterMode      uint8
			Entered, ActionEntered, Human bool
			Carried                       int8
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if len(oracle.Gates) != 2016 || len(oracle.Waits) != 288 {
		t.Fatal("incomplete original Action3E execution matrix", len(oracle.Gates), len(oracle.Waits))
	}
	for i, r := range oracle.Gates {
		state := AIControlState{Mode: AIMode(r.Mode), Entered: r.Entered, Counter: 17, Plan: 17, PlanParameter: 14, Destination: 85, TrackedObject: 1}
		got, dispatch, err := originalAIControlDispatch(state, Action(62), r.Actor == r.Holder)
		want := state
		want.Mode, want.Entered = AIMode(r.AfterMode&127), r.AfterMode&128 != 0
		if err != nil || dispatch != r.Dispatch || got != want {
			t.Fatal("gate", i, r, got, dispatch, err)
		}
	}
	for i, r := range oracle.Waits {
		state := AIControlState{Mode: 1, Entered: r.Entered, Counter: 17, Plan: 17, PlanParameter: 14, Destination: 85, TrackedObject: 1}
		v := AIControlView{Actor: r.Actor, Action: 62, ActionEntered: r.ActionEntered, HasBall: r.Actor == r.Holder, BallHolder: int8(r.Holder), TeamHasHuman: r.Human, Random: 0x12345678, PursuitWorld: &WorldActionState{CarriedPlayer: r.Carried}}
		got, err := StepOriginalAIControl(state, v)
		want := AIControlDecision{Control: state, Action: 62, ActionEntered: r.ActionEntered, BindHoop: -1, Random: v.Random}
		want.Control.Mode, want.Control.Entered = AIMode(r.AfterMode&127), r.AfterMode&128 != 0
		if err != nil || got != want {
			t.Fatal("complete mode1", i, r, got, want, err)
		}
	}
	t.Log("matched 2016 original Action3E gates and 288 full original possession calls; CarriedPlayer is not a routing predicate")
}
