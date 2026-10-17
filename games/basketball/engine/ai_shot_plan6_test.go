package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestShotPlanSixOriginalFunction(t *testing.T) {
	var rows []struct{ Actor, Action, Heading, Parameter, AfterAction, AfterMode uint8 }
	readPropAttackFixture(t, "testdata/ai-shot-plan6-function.json.gz", &rows)
	if len(rows) != 18432 {
		t.Fatal("incomplete original B4E3 matrix", len(rows))
	}
	for i, row := range rows {
		heading := HeadingNone
		if row.Heading != 255 {
			heading = Heading(row.Heading / 32)
		}
		initial := AIControlDecision{Control: AIControlState{Mode: 6, Entered: true, Plan: 17, PlanParameter: row.Parameter, Counter: 43, Destination: 0x65, TrackedObject: 3}, Action: Action(row.Action & 127), ActionEntered: row.Action&128 != 0, Heading: heading, Random: 0x12345678}
		got, want := initial, initial
		want.Action, want.ActionEntered = Action(row.AfterAction&127), row.AfterAction&128 != 0
		want.Control.Mode, want.Control.Entered = AIMode(row.AfterMode&127), row.AfterMode&128 != 0
		view := AIControlView{Actor: row.Actor, Action: initial.Action, ActionEntered: initial.ActionEntered}
		if err := advanceOriginalShotPlan(&got, view, AIShotContext{}); err != nil || got != want {
			t.Fatalf("original case %d: got=%+v want=%+v error=%v", i, got, want, err)
		}
	}
	t.Log("matched 18432 original B4E3 calls; full decision preserves unrelated fields")
}

func TestShotPlanSixRealFailureContinuation(t *testing.T) {
	var s RuntimeState
	readPropAttackFixture(t, "testdata/ai-shot-plan6-runtime-before.json.gz", &s)
	if s.VideoFrames != 2624 {
		t.Fatal("wrong retained first failure", s.VideoFrames)
	}
	for tick := 0; tick < 120; tick++ {
		encoded, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var restored RuntimeState
		if err = json.Unmarshal(encoded, &restored); err != nil {
			t.Fatal(err)
		}
		got, err := StepVideo(s, [4]Buttons{})
		if err != nil {
			t.Fatalf("original configuration video %d: %v", s.VideoFrames, err)
		}
		replay, err := StepVideo(restored, [4]Buttons{})
		if err != nil || !reflect.DeepEqual(got, replay) {
			t.Fatal("JSON restore changed continuation", tick, err)
		}
		s = got.State
	}
	t.Log("continued original first-failure state for 120 video frames with identical per-frame JSON replay")
}
