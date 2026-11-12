package racing

// Isolated geometry failure cases are listed before implementation in
// ../tests/lap-reset-failures.md. Driven Module coverage remains the primary gate.
import (
	"math"
	"testing"
)

func TestClassicGateRoadShoulderAndProtection(t *testing.T) {
	for _, id := range []string{"uk", "france", "coastal", "park"} {
		track, _ := classicTrackByID(id)
		for _, test := range []struct {
			name                         string
			shoulder, height             float64
			reverse, stationary, allowed bool
		}{
			{"road", -10, 0, false, false, true},
			{"near_shoulder", 15, 0, false, false, true},
			{"outer_shoulder", 29, 0, false, false, true},
			{"detached_shortcut", 65, 0, false, false, false},
			{"different_elevation", 15, 100, false, false, false},
			{"reverse", 15, 0, true, false, false},
			{"stationary", 15, 0, false, true, false},
		} {
			t.Run(id+"/"+test.name, func(t *testing.T) {
				i := 10
				p, v, r := track.Points[i], tangent(track, i), track.Road[i]
				l := math.Hypot(r.RX, r.RZ)
				x, y := p.X-r.RX/l*(r.Left+test.shoulder), p.Y-r.RZ/l*(r.Left+test.shoulder)
				height := r.Height - (r.NX*(x-p.X)+r.NZ*(y-p.Y))/math.Max(.1, r.NY) + test.height
				before := Point{x - v.X*2, y - v.Y*2}
				after := Point{x + v.X*2, y + v.Y*2}
				if test.reverse {
					before, after = after, before
				}
				if test.stationary {
					before = after
				}
				s := State{Physics: 3, Options: Options{Track: id, Laps: 3}, Track: track, Cars: []Car{{Seat: 0, Vehicle: "p911", Checkpoint: i, Gate: i - 1, X: after.X, Y: after.Y, Height: height}}}
				updateProgress(&s, &s.Cars[0], before, 1020)
				advanced := s.Cars[0].Checkpoint == i+1
				if advanced != test.allowed {
					t.Fatalf("gate advanced=%v want %v: %+v", advanced, test.allowed, s.Cars[0])
				}
				if s.Cars[0].Lap != 0 || s.Cars[0].FinishAt != 0 {
					t.Fatal("non-finish gate changed lap")
				}
			})
		}
	}
}
