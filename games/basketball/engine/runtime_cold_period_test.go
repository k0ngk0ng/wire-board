package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
)

// This is a cold runtime integration gate, not an original-ROM trace oracle.
// Never shorten the clock, seed the world from a fixture, or replace the AI.
// Failures to cover before extending runtime: video/logic clock drift, a natural
// period that never ends, lost scores across a reset, changed RNG/input/display
// after restoring JSON, and nondeterminism from retained state between runs.
func TestRuntimeColdFirstPeriodAndRestore(t *testing.T) {
	config := DefaultRuntimeConfig()
	config.Match.Seed = 0x12345678
	verifyColdVideoRuntime(t, config, false)
}

func TestRuntimeColdCompleteMatchAndRestore(t *testing.T) {
	config := DefaultRuntimeConfig()
	config.Match.Seed = 0x12345678
	verifyColdVideoRuntime(t, config, true)
}

func TestRuntimeAllComputerCompleteMatchAndRestore(t *testing.T) {
	config := DefaultRuntimeConfig()
	config.Match.Seed = 0x12345678
	config.Match.ControllerAssignments = [4]int8{-1, -1, -1, -1}
	verifyColdVideoRuntime(t, config, true)
}

func verifyColdVideoRuntime(t *testing.T, config RuntimeConfig, completeMatch bool) {
	t.Helper()
	maximumVideoFrames := 24000
	if completeMatch {
		maximumVideoFrames = 48000
	}
	var expectedControls [4]uint8
	for i, controller := range config.Match.ControllerAssignments {
		expectedControls[i] = 128
		if controller >= 0 {
			expectedControls[i] = uint8(controller)
		}
	}
	var firstDigest string
	for repeat := 0; repeat < 2; repeat++ {
		state, err := NewRuntime(config)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.New()
		seenPlaying, seenIntermission, seenSecondPeriod, finished := false, false, false, false
		var intermissionScores [2]uint16
		var lastScores [2]uint16
		restores := 0
		var lastBefore []byte
		var lastFrame int
		for frame := 0; frame < maximumVideoFrames; frame++ {
			before, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			lastBefore, lastFrame = before, frame
			fail := func(reason any) {
				t.Helper()
				t.Fatalf("runtime cold seed 12345678 repeat %d video %d/%d logic %d: %v\nbefore=%s", repeat, frame, maximumVideoFrames, state.LogicSteps, reason, before)
			}
			var held [4]Buttons
			// The only user action is to confirm the original intermission.
			if state.AwaitingNextPeriod && state.WaitVideoFrames == 0 {
				held[0] = ButtonStart
			}
			step, err := StepVideo(state, held)
			if err != nil {
				fail(err)
			}
			if frame%300 == 0 || state.AwaitingNextPeriod {
				var restored RuntimeState
				if err := json.Unmarshal(before, &restored); err != nil {
					fail(err)
				}
				if err := ValidateRuntimeState(restored); err != nil {
					fail(err)
				}
				restoredStep, err := StepVideo(restored, held)
				if err != nil || !reflect.DeepEqual(step, restoredStep) {
					fail("restored runtime changed next video tick, display or sounds")
				}
				restores++
			}
			state = step.State
			if state.Playing != nil {
				if state.Playing.Context.Actions.Controls != expectedControls {
					fail("runtime controller assignments differ from the requested configuration")
				}
				lastScores = state.Playing.World.Preparation.Scores
			}
			if step.LogicAdvanced {
				encoded, err := json.Marshal(step)
				if err != nil {
					t.Fatal(err)
				}
				digest.Write(encoded)
			}
			if state.Playing != nil && state.Playing.Match.Period == 0 {
				seenPlaying = true
			}
			if state.AwaitingNextPeriod && !seenIntermission {
				if !seenPlaying || state.Transition == nil || state.Transition.Scene.Transition.Match.Period != 1 || frame < 10800 {
					fail("first period skipped or ended before its natural three-minute clock")
				}
				intermissionScores = state.Transition.Scene.Transition.Scores
				seenIntermission = true
			}
			if state.Playing != nil && state.Playing.Match.Period == 1 && !seenSecondPeriod {
				if !seenIntermission || state.Playing.World.Preparation.Scores != intermissionScores {
					fail("next period lost the natural intermission score")
				}
				seenSecondPeriod = true
				if !completeMatch {
					finished = true
					break
				}
			}
			if completeMatch && state.Outcome != nil && state.Outcome.Confirmed {
				o := state.Outcome
				if !seenSecondPeriod || frame < 21600 || o.Scores != lastScores || o.WinningTeam > 1 || o.Scores[o.WinningTeam] <= o.Scores[o.WinningTeam^1] {
					fail("final outcome skipped regulation play or disagrees with actual scores")
				}
				if state.Display.Visible.Result == nil || state.Display.Visible.Result.WinningTeam != o.WinningTeam {
					fail("confirmed match has no original result display")
				}
				finished = true
				break
			}
		}
		if !finished || restores < 36 {
			t.Fatalf("runtime cold seed 12345678 repeat %d video %d/%d logic %d: natural period did not finish with periodic restoration coverage; videos=%d restores=%d\nbefore=%s", repeat, lastFrame, maximumVideoFrames, state.LogicSteps, state.VideoFrames, restores, lastBefore)
		}
		sum := hex.EncodeToString(digest.Sum(nil))
		if repeat == 0 {
			firstDigest = sum
		} else if sum != firstDigest {
			t.Fatalf("runtime cold seed 12345678 repeat %d video %d/%d logic %d: cold video runtime is nondeterministic; expected=%s actual=%s\nbefore=%s", repeat, lastFrame, maximumVideoFrames, state.LogicSteps, firstDigest, sum, lastBefore)
		}
		t.Logf("runtime cold repeat=%d videos=%d logic=%d restores=%d scores=%v trajectory_sha256=%s complete_match=%t; controller inputs only, not a ROM oracle or campaign/menu proof", repeat, state.VideoFrames, state.LogicSteps, restores, lastScores, sum, completeMatch)
	}
}
