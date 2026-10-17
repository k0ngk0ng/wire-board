package engine

import "testing"

func TestOriginalScoreStrategyMode32(t *testing.T) {
	for _, tc := range []struct {
		actor, tier uint8
		want        AIMode
	}{{0, 0, 12}, {0, 1, 12}, {0, 2, 8}, {0, 3, 8}, {0, 129, 12}, {0, 130, 9}, {0, 131, 9}, {2, 0, 12}, {2, 1, 12}, {2, 2, 9}, {2, 3, 9}, {2, 129, 12}, {2, 130, 8}, {2, 131, 8}} {
		s := AIControlState{Mode: 32, Plan: 2, Counter: 1, TrackedObject: 2, Destination: 16, PlanParameter: 10}
		v := AIControlView{Actor: tc.actor, Action: ActionRun, ActionEntered: true, Heading: HeadingRight, Random: 0x12345678, ScoreDifference: tc.tier}
		got, err := StepOriginalAIControl(s, v)
		want := s
		want.Mode = tc.want
		if err != nil || got.Control != want || got.Action != ActionRun || !got.ActionEntered || got.Random != v.Random || got.Heading != HeadingRight {
			t.Fatal(tc, got, err)
		}
		s.Entered = true
		got, err = StepOriginalAIControl(s, v)
		if err != nil || got.Control != s {
			t.Fatal("already-entered selector repeated", tc, got, err)
		}
	}
}
