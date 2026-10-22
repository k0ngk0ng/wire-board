package fortressblue

// Failures specified before implementation: ground pose is cosmetic only;
// facing reversal applies the slope with the wrong sign; steep uphill shots are
// clamped to vertical; missing support borrows a distant platform; bot and human
// paths differ; default options reject a full minute; legacy durations break.
import (
	"encoding/json"
	"math"
	"testing"

	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
)

func TestLaunchIncludesTerrainSlopeForBothDirections(t *testing.T) {
	m := fbmaps.Map{Width: 400, Height: 400, MaskCols: 100, MaskRows: 100}
	for _, grade := range []float64{-0.5, 0, 0.5} {
		mask := make([]byte, 1250)
		for cy := 0; cy < 100; cy++ {
			for cx := 0; cx < 100; cx++ {
				if float64(cy*4) >= 200+float64(cx*4-200)*grade {
					i := cy*100 + cx
					mask[i>>3] |= 1 << (7 - (i & 7))
				}
			}
		}
		for _, facing := range []int{-1, 1} {
			arc := simulateArc(m, mask, nil, -1, 200, 200, 45, 60, facing, 1, 0, 4)
			p, q := arc.Path[0], arc.Path[1]
			actual := math.Atan2(-(q.Y-p.Y), (q.X-p.X)*float64(facing)) * 180 / math.Pi
			// Compare the continuous grade across the 24 px wheelbase; collision
			// cells quantize each support height independently.
			want := 45 - math.Atan(grade)*180/math.Pi*float64(facing)
			if math.Abs(actual-want) > 5 {
				t.Fatalf("grade=%v facing=%d launch=%v want≈%v", grade, facing, actual, want)
			}
		}
	}
}

func TestDefaultTurnIsOneMinuteAndLegacyOptionsRemainValid(t *testing.T) {
	raw, err := configure(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	var o Options
	if err := json.Unmarshal(raw, &o); err != nil {
		t.Fatal(err)
	}
	if o.TurnSeconds != 60 {
		t.Fatalf("default = %d, want 60", o.TurnSeconds)
	}
	for _, duration := range []int{15, 20, 25, 30, 40, 60} {
		raw, _ := json.Marshal(Options{Map: "stage-16", TurnSeconds: duration, BotDifficulty: "normal"})
		if _, err := configure(2, raw); err != nil {
			t.Fatal(duration, err)
		}
	}
}
