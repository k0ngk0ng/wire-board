package snooker_test

// Real module strokes from the natural rack, including maximum force into the
// packed reds. These are stroke stress cases, not complete match counts.
import (
	"fmt"
	"testing"
)

func TestSnookerPackedRackPowerAndSpinStrokes(t *testing.T) {
	for i, a := range []float64{-.1, -.03, 0, .025} {
		for j, tip := range [][3]float64{{.2, 0, 0}, {.5, 0, 0}, {1, 0, 0}, {1, .8, 0}, {1, -.8, 0}, {1, 0, .8}, {1, 0, -.8}, {.8, .6, -.6}} {
			t.Run(fmt.Sprintf("angle%d-tip%d", i, j), func(t *testing.T) {
				mod, ctx, m := begin(t, 2)
				m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": a, "power": tip[0], "top": tip[1], "side": tip[2]})
				m = finishStroke(t, mod, ctx, m)
				if _, e := mod.View(m, -1); e != nil {
					t.Fatal("maximum-force stroke produced unrestorable state", e)
				}
				trace(t, m)
			})
		}
	}
}
