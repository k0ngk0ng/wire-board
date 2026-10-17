package engine

// Failure inventory: missing dispatch, entry commands invisible to court
// motion, stale shared state replayed after devices, reordered sounds, lost
// animation transition, or a different result after JSON restore. Compare the
// bound original court operation with independent CPU calls, not mock physics.
import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestRecoveredSpecialBallWorldBinding(t *testing.T) {
	for _, action := range []BallAction{16, 17, 19, 22} {
		t.Run(fmt.Sprint(action), func(t *testing.T) {
			var f struct {
				Calls []struct {
					worldActionFixture
					Sounds []uint8
					Label  string
				}
			}
			readPropAttackFixture(t, fmt.Sprintf("testdata/ball%d.json.gz", action), &f)
			for i, r := range f.Calls {
				result := WorldActionStep{State: r.Before}
				c := r.context(t)
				court, e := originalWorldMotionCourt(&result, c, 6)
				if e != nil {
					t.Fatal(i, r.Label, e)
				}
				switch action {
				case 16:
					e = advanceWorldBall16(&result, c, court)
				case 17:
					e = advanceWorldBall17(&result, c, court)
				case 19:
					e = advanceWorldBall19(&result, c, court)
				case 22:
					e = advanceWorldBall22(&result, c, court)
				}
				if e != nil {
					t.Fatal(i, r.Label, e)
				}
				if result.State != r.After {
					t.Fatalf("case%d %s: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
				}
				if !slices.Equal(result.SoundIDs, r.Sounds) {
					t.Fatal(i, "sounds", result.SoundIDs, r.Sounds)
				}
			}
		})
	}
}

func TestRecoveredSpecialBallOriginalScheduler(t *testing.T) {
	var rows []struct {
		worldActionFixture
		Sounds []uint8
		Label  string
	}
	readPropAttackFixture(t, "testdata/recovered-special-world.json.gz", &rows)
	if len(rows) != 16 {
		t.Fatal("missing original scheduler scenarios")
	}
	for i, r := range rows {
		result, e := StepWorldActions(r.Before, r.context(t))
		if e != nil {
			t.Fatal(i, r.Label, e)
		}
		if result.State != r.After {
			t.Fatalf("case%d %s: %s", i, r.Label, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(result.Calls, r.Calls) || !slices.Equal(result.SoundIDs, r.Sounds) {
			t.Fatal(i, "scheduler or sound order differs", result.Calls, r.Calls, result.SoundIDs, r.Sounds)
		}
	}
}
