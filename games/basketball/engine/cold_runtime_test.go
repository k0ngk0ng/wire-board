package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestColdConfiguredWorldSixThousandUpdates(t *testing.T) {
	const target = 6000
	var first string
	for repeat := 0; repeat < 2; repeat++ {
		opening := coldOpeningForTest(t)
		world, ctx, err := WorldFromOriginalOpening(opening, OpeningWorldOptions{LogicFrame: 35})
		if err != nil {
			t.Fatal(err)
		}
		if ctx.Actions.CourtID != 1 || ctx.Actions.Teams != ([2]uint8{0, 1}) || ctx.Actions.Controls != ([4]uint8{0, 128, 128, 128}) {
			t.Fatal("cold default configuration drift", ctx)
		}
		digest := sha256.New()
		for update := 0; update < target; update++ {
			step, err := StepWorldLoop(world, WorldLoopInput{}, ctx)
			if err != nil {
				before, _ := json.Marshal(world)
				t.Fatalf("cold seed 12345678, update %d/%d: %v\nbefore=%s", update, target, err, before)
			}
			world = step.State
			encoded, _ := json.Marshal(world)
			digest.Write(encoded)
		}
		sum := hex.EncodeToString(digest.Sum(nil))
		if repeat == 0 {
			first = sum
		} else if sum != first {
			t.Fatal("cold runtime is nondeterministic", first, sum)
		}
		if world.Updates != target {
			t.Fatal(world.Updates)
		}
		t.Logf("cold original configuration repeat=%d updates=%d scores=%v trajectory_sha256=%s; controller frames only, no expected-state replay or video-clock claim", repeat, target, world.Preparation.Scores, sum)
	}
}
