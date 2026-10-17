package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func allComputerColdOpening(t *testing.T) OpeningState {
	t.Helper()
	cfg := DefaultOriginalMatchConfiguration()
	cfg.Seed = 0x12345678
	cfg.ControllerAssignments = [4]int8{-1, -1, -1, -1}
	state, err := NewOriginalMatch(cfg)
	if err != nil {
		t.Fatal(err)
	}
	court, err := InitializeOriginalCourt(state)
	if err != nil {
		t.Fatal(err)
	}
	start, err := StartOriginalCourt(court.State)
	if err != nil {
		t.Fatal(err)
	}
	state = start.State
	for i := 0; i < 33; i++ {
		state.Transition.CameraScroll[0], state.Transition.CameraScroll[1] = state.Transition.CameraScroll[2], state.Transition.CameraScroll[3]
		state.Projection = resetOriginalWorldDrawList(state.Projection)
		state.Transition.Random, _ = NextRandom(state.Transition.Random)
		next, err := StepOriginalRefereeOpening(state)
		if err != nil {
			t.Fatal(i, err)
		}
		state = next.State
	}
	period, err := InitializeOpeningPeriod(state)
	if err != nil {
		t.Fatal(err)
	}
	return period.State
}

func TestAllComputerColdWorldSixThousandUpdates(t *testing.T) {
	const target = 6000
	var first string
	for repeat := 0; repeat < 2; repeat++ {
		opening := allComputerColdOpening(t)
		state, ctx, err := WorldFromOriginalOpening(opening, OpeningWorldOptions{LogicFrame: 35})
		if err != nil {
			t.Fatal(err)
		}
		if ctx.Actions.CourtID != 1 || ctx.Actions.Teams != ([2]uint8{0, 1}) || ctx.Actions.Controls != ([4]uint8{128, 128, 128, 128}) {
			t.Fatal("all-computer cold configuration drift", ctx)
		}
		digest := sha256.New()
		for update := 0; update < target; update++ {
			step, err := StepWorldLoop(state, WorldLoopInput{}, ctx)
			if err != nil {
				before, _ := json.Marshal(state)
				t.Fatalf("all-computer cold seed 12345678, update %d/%d: %v\nbefore=%s", update, target, err, before)
			}
			state = step.State
			encoded, _ := json.Marshal(state)
			digest.Write(encoded)
		}
		sum := hex.EncodeToString(digest.Sum(nil))
		if repeat == 0 {
			first = sum
		} else if sum != first {
			t.Fatal("all-computer cold nondeterminism", first, sum)
		}
		if state.Updates != target {
			t.Fatal(state.Updates)
		}
		t.Logf("all-computer cold repeat=%d updates=%d scores=%v trajectory_sha256=%s; original AI, one initialization, no snapshot replay or video-clock claim", repeat, target, state.Preparation.Scores, sum)
	}
}
