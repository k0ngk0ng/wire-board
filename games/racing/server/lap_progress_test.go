package racing

// Isolated numerical protections, listed before implementation in
// ../tests/lap-reset-failures.md. Actual Module drive is the primary evidence.
import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"testing"
	"time"
)

func TestClassicWindingHasNoLateralGate(t *testing.T) {
	for _, id := range []string{"france", "coastal", "uk", "park"} {
		for _, extra := range []float64{0, 100, 200, 1000} {
			t.Run(fmt.Sprintf("%s-extra%.0f", id, extra), func(t *testing.T) {
				ctx := gameapi.Context{Players: 2, Now: time.Unix(1000, 0)}
				options, _ := json.Marshal(map[string]any{"track": id, "laps": 3})
				m, err := Module().Start(ctx, options)
				if err != nil {
					t.Fatal(err)
				}
				s, err := decode(m)
				if err != nil {
					t.Fatal(err)
				}
				s.Phase = "racing"
				c := &s.Cars[0]
				at := s.StartAt
				move := func(target Point, index int) {
					origin := Point{c.X, c.Y}
					steps := int(math.Ceil(dist(origin, target) / 10))
					if steps < 1 {
						steps = 1
					}
					for k := 1; k <= steps; k++ {
						before := Point{c.X, c.Y}
						f := float64(k) / float64(steps)
						c.X = origin.X + (target.X-origin.X)*f
						c.Y = origin.Y + (target.Y-origin.Y)*f
						c.RouteIndex = index
						r, p := s.Track.Road[index], s.Track.Points[index]
						c.Height = r.Height - (r.NX*(c.X-p.X)+r.NZ*(c.Y-p.Y))/math.Max(.1, r.NY)
						c.Speed = dist(before, Point{c.X, c.Y}) / .02
						at += 20
						updateProgress(&s, c, before, at)
					}
				}
				for i := 0; i <= len(s.Track.Points); i++ {
					j := i % len(s.Track.Points)
					p, r := s.Track.Points[j], s.Track.Road[j]
					if extra > 0 && j >= 20 && j <= 35 {
						l := math.Hypot(r.RX, r.RZ)
						p.X -= r.RX / l * (r.Left + extra)
						p.Y -= r.RZ / l * (r.Left + extra)
					}
					move(p, j)
				}
				if c.Lap != 1 {
					t.Fatalf("continuous circuit blocked by lateral offset: lap=%d gate=%d checkpoint=%d", c.Lap, c.Gate, c.Checkpoint)
				}
				p, v := s.Track.Points[0], tangent(s.Track, 0)
				for k := 0; k < 20; k++ {
					move(Point{p.X - v.X*2, p.Y - v.Y*2}, len(s.Track.Points)-1)
					move(Point{p.X + v.X*2, p.Y + v.Y*2}, 0)
				}
				if c.Lap != 1 {
					t.Fatal("finish oscillation awarded or removed a lap", c.Lap)
				}
				c.Speed, c.VX, c.VY = 0, 0, 0
				for k := 0; k < 10; k++ {
					at += 20
					updateProgress(&s, c, Point{c.X, c.Y}, at)
				}
				if c.Lap != 1 {
					t.Fatal("stationary finish changed lap")
				}
				// Trust the authoritative engine step, including multi-car
				// separation, when its origin matches the last tracked position.
				beforeCorrection := Point{c.X, c.Y}
				c.X, c.Y = s.Track.Points[3].X, s.Track.Points[3].Y
				c.Height = s.Track.Road[3].Height
				c.RouteIndex = 3
				at += 20
				updateProgress(&s, c, beforeCorrection, at)
				if c.LapTrack.Turns != c.Lap {
					t.Fatal("authoritative position correction lost winding")
				}
				before := Point{c.X, c.Y}
				j := len(s.Track.Points) / 2
				c.X = s.Track.Points[j].X
				c.Y = s.Track.Points[j].Y
				c.RouteIndex = j
				// The next real engine step starts from the externally relocated
				// point, which does not match the stored tracking origin.
				before = Point{c.X, c.Y}
				at += 20
				updateProgress(&s, c, before, at)
				if c.Lap != 1 {
					t.Fatal("nonphysical jump changed lap")
				}
				if c.LapTrack.Turns != c.Lap-1 {
					t.Fatal("external relocation did not reanchor without lap credit")
				}
			})
		}
	}
}
