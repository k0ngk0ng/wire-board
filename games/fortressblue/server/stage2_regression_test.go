package fortressblue

// Failure scenarios declared before stage 2 implementation: source minimum
// angles ignored by humans/bots; a visible, walkable floating platform ignored
// by shells; a tank's centre falling through a slit still bridged by its tracks;
// body penetrating a tall wall before its centre touches it; bots never moving
// out of a blocked firing lane. Fixture coordinates are this project's map data,
// not a claim that original Fortress has been played at these exact pixels.
import (
	"encoding/json"
	fbdata "github.com/k0ngk0ng/wire-board/games/fortressblue/data"
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"testing"
	"time"
)

func TestSourceAngleBoundsAndBot(t *testing.T) {
	for _, id := range fbdata.IDs() {
		spec, _ := tankSpec(id)
		t.Run(spec.Name, func(t *testing.T) {
			s, m := startAt(t, 2, "stage-16")
			m, s = selectAll(t, s, m, []int{id, 1})
			s.Tanks[0].Items = nil
			m = mustMatch(t, s)
			now := time.Unix(1700000002, 0)
			for _, angle := range []float64{float64(spec.MinAngle) - .5, float64(spec.Angle) + .5} {
				a, _ := json.Marshal(Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: angle, Power: 40, Facing: 1, Weapon: "primary"})
				if _, e := apply(ctx(2, 0, now), m, a); e == nil {
					t.Fatalf("accepted out-of-source angle %v (allowed %d..%d)", angle, spec.MinAngle, spec.Angle)
				}
			}
			for _, angle := range []float64{float64(spec.MinAngle), float64(spec.Angle)} {
				a, _ := json.Marshal(Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: angle, Power: 40, Facing: 1, Weapon: "primary"})
				if _, e := apply(ctx(2, 0, now), m, a); e != nil {
					t.Fatalf("source bound rejected: %v", e)
				}
			}
			fired := false
			for n := 0; n < 20; n++ {
				raw, e := botAction(ctx(2, 0, now), m)
				if e != nil {
					t.Fatal(e)
				}
				var a Action
				if e = json.Unmarshal(raw, &a); e != nil {
					t.Fatal(e)
				}
				if a.Type == "fire" {
					fired = true
					if a.Angle < float64(spec.MinAngle) || a.Angle > float64(spec.Angle) {
						t.Fatalf("bot angle %v outside source range", a.Angle)
					}
					break
				}
				m, e = apply(ctx(2, 0, now), m, raw)
				if e != nil {
					t.Fatal(e)
				}
				var pending struct{ Resolution any }
				_ = json.Unmarshal(m.Data, &pending)
				if pending.Resolution != nil {
					m = finishTestResolution(t, m)
				}
			}
			if !fired {
				t.Fatal("bot did not fire after bounded movement")
			}
		})
	}
}
func TestFloatingPlatformBlocksTankAndShell(t *testing.T) {
	m, _ := fbmaps.ByID("stage-4")
	ground := maskFor(m, nil)
	if !solid(m, ground, 520, 272) || groundY(m, ground, 520, 240) != 272 {
		t.Fatal("independent platform fixture changed")
	}
	bullets := bulletMaskFor(m, nil)
	if !solid(m, bullets, 520, 272) {
		t.Fatal("standing platform was removed from shell collision")
	}
	arc := simulateArc(m, bullets, []Tank{{Seat: 0, Alive: true, X: 500, Y: 240}}, 0, 500, 240, 0, 20, 1, 1, 0, MaxPath)
	if arc.OutOfBounds || arc.Impact.Y > 308 || !solid(m, ground, arc.Impact.X, arc.Impact.Y) {
		t.Fatalf("shell passed through floating platform: %+v", arc.Impact)
	}
}
func TestTankFootprintBridgesNarrowGapAndStopsBeforeWall(t *testing.T) {
	m := fbmaps.Map{Width: 160, Height: 160, MaskCols: 40, MaskRows: 40}
	mask := make([]byte, 200)
	set := func(x, y int) { idx := y*40 + x; mask[idx>>3] |= 1 << uint(7-(idx&7)) }
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if y >= 30 && (x < 19 || x > 20) {
				set(x, y)
			}
			if x >= 30 && y >= 15 {
				set(x, y)
			}
		}
	}
	if y, ok := walkGround(m, mask, 75, 120, 79); !ok || y != 120 {
		t.Fatalf("tracks fell through narrow gap: y=%v ok=%v", y, ok)
	}
	if _, ok := walkGround(m, mask, 107, 120, 111); ok {
		t.Fatal("front of tracks/body entered wall before centre")
	}
}
func TestBotMovesAwayFromCloseHighObstacle(t *testing.T) {
	s, m := startAt(t, 2, "stage-4")
	m, s = selectAll(t, s, m, []int{3, 1})
	s.Wind = 0
	s.Tanks[0].X, s.Tanks[0].Y = 378, 560
	s.Tanks[1].X, s.Tanks[1].Y = 440, 296
	s.Tanks[0].Items = nil
	m = mustMatch(t, s)
	raw, e := botAction(ctx(2, 0, time.Unix(1700000002, 0)), m)
	if e != nil {
		t.Fatal(e)
	}
	var a Action
	_ = json.Unmarshal(raw, &a)
	if a.Type != "move" {
		t.Fatalf("blocked bot did not reposition: %s", raw)
	}
	next, e := apply(ctx(2, 0, time.Unix(1700000002, 0)), m, raw)
	if e != nil {
		t.Fatal(e)
	}
	after, e := decode(next)
	if e != nil {
		t.Fatal(e)
	}
	if after.Tanks[0].X >= s.Tanks[0].X || after.Tanks[0].Fuel >= s.Tanks[0].Fuel {
		t.Fatal("bot did not retreat to create firing clearance")
	}
}
