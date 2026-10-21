package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Retained original failed runtime continuations supplement cold whole-match
// acceptance. No scores/clock/RNG/positions are changed; only the explicitly
// selected persisted repair policy and its validated action-context copy.
func TestSuspendedBallRecoveryRetainedRuntime(t *testing.T) {
	var cases []struct {
		Name  string
		State RuntimeState
	}
	readAITeamModeFixture(t, "suspended-ball-runtime", &cases)
	if len(cases) != 3 {
		t.Fatal("missing retained original failure configurations")
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.State.Config.RecoverUnsupportedBall || tc.State.Playing == nil || tc.State.Playing.Context.Actions.RecoverUnsupportedBall {
				t.Fatal("source fixture opted into repair")
			}
			strict := tc.State
			initial := strict.Playing.World.Preparation.Actions.Ball.Body
			for n := 0; n < 120; n++ {
				step, e := StepVideo(strict, [4]Buttons{})
				if e != nil {
					t.Fatal(e)
				}
				strict = step.State
			}
			if strict.Playing == nil || strict.Playing.World.Preparation.Actions.Ball.Body != initial {
				t.Fatal("original-mode suspended evidence changed")
			}
			state := tc.State
			// Copy nested playing state before changing its context, retaining the
			// strict source snapshot and its shared actor/profile data unchanged.
			playing := *state.Playing
			state.Playing = &playing
			state.Config.RecoverUnsupportedBall = true
			state.Playing.Context.Actions.RecoverUnsupportedBall = true
			if e := ValidateRuntimeState(state); e != nil {
				t.Fatal(e)
			}
			restores := 0
			releaseSeen := false
			for state.VideoFrames < 48000 && (state.Outcome == nil || !state.Outcome.Confirmed) {
				var held [4]Buttons
				if state.AwaitingNextPeriod && state.WaitVideoFrames == 0 {
					held[0] = ButtonStart
				}
				step, e := StepVideo(state, held)
				if e != nil {
					raw, _ := json.Marshal(state)
					t.Logf("before=%s", raw)
					t.Fatalf("video%d: %v", state.VideoFrames, e)
				}
				if state.VideoFrames%300 == 0 || restores < 12 {
					raw, e := json.Marshal(state)
					if e != nil {
						t.Fatal(e)
					}
					var restored RuntimeState
					if e = json.Unmarshal(raw, &restored); e != nil {
						t.Fatal(e)
					}
					again, e := StepVideo(restored, held)
					if e != nil || !reflect.DeepEqual(again, step) {
						t.Fatal("restored repair diverges", e)
					}
					restores++
				}
				state = step.State
				if state.Playing != nil && state.Playing.World.Preparation.Actions.Ball.Action == 10 {
					releaseSeen = true
				}
			}
			if !releaseSeen || state.Outcome == nil || !state.Outcome.Confirmed || restores < 12 {
				t.Fatal("retained original failure did not recover and finish", state.VideoFrames, restores, state.Outcome)
			}
			t.Logf("retained %s video%d → confirmed video%d, scores=%v, restores=%d; original snapshot unchanged except explicit repair policy", tc.Name, tc.State.VideoFrames, state.VideoFrames, state.Outcome.Scores, restores)
		})
	}
}
