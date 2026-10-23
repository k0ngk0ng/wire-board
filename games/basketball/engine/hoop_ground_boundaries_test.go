package engine

import (
	"reflect"
	"slices"
	"testing"
)

func TestHoopGroundOriginalBoundaries(t *testing.T) {
	var f struct{ Calls, Stages []hoopAirThrowOracle }
	readPropAttackFixture(t, "testdata/hoop-ground-boundaries.json.gz", &f)
	if len(f.Calls) != 128 || len(f.Stages) != 8 {
		t.Fatal("missing original edge/air coverage", len(f.Calls), len(f.Stages))
	}
	for i, r := range f.Calls {
		result := WorldActionStep{State: r.Before}
		ctx := r.context(t)
		court := NewYorkGroundCourt()
		var err error
		if ctx.CourtID != 1 {
			court, err = originalWorldMotionCourt(&result, ctx, 0)
			if err != nil {
				t.Fatal(i, err)
			}
		}
		if err = advanceWorldPlayer(&result, 0, ctx, court); err != nil {
			t.Fatal(i, err)
		}
		if result.State != r.After {
			t.Fatal(i, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(result.SoundIDs, r.Sounds) {
			t.Fatal(i, "sounds", result.SoundIDs, r.Sounds)
		}
	}
	for i, r := range f.Stages {
		result, err := StepWorldActions(r.Before, r.context(t))
		if err != nil {
			t.Fatal("stage", i, err)
		}
		if result.State != r.After {
			t.Fatal("stage", i, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(result.Calls, r.Calls) || !slices.Equal(result.SoundIDs, r.Sounds) {
			t.Fatal("stage scheduler/sounds", i)
		}
	}
}
