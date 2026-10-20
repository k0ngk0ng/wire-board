package tests

// Review failure cases established before runtime spawn implementation: the
// catalog's bottom-up positions can start inside foreground or under a low
// ceiling, with no movement and every shot damaging self. Validate public
// Start+Select, not edited positions: all maps, capacities, and tank types must
// have supported, clear bodies and an actual non-self-damaging firing route.
import (
	"encoding/json"
	"fmt"
	fbdata "github.com/k0ngk0ng/wire-board/games/fortressblue/data"
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	fb "github.com/k0ngk0ng/wire-board/games/fortressblue/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"testing"
	"time"
)

func spawnSolid(m fbmaps.Map, mask []byte, x, y float64) bool {
	if x < 0 || y < 0 || x >= float64(m.Width) || y >= float64(m.Height) {
		return false
	}
	idx := int(y/float64(fbmaps.Cell()))*m.MaskCols + int(x/float64(fbmaps.Cell()))
	return mask[idx>>3]&(1<<uint(7-(idx&7))) != 0
}
func TestAllNativeStartsHaveClearSupportedPlayableSpawns(t *testing.T) {
	catalog, e := fbmaps.All()
	if e != nil {
		t.Fatal(e)
	}
	for _, stage := range catalog.Maps {
		for _, players := range []int{2, 4, 6, 8} {
			for _, id := range fbdata.IDs() {
				t.Run(fmt.Sprintf("%s/%d/tank-%d", stage.ID, players, id), func(t *testing.T) {
					mod := fb.Module()
					now := time.Unix(1800000000, 0)
					opts, _ := json.Marshal(fb.Options{Map: stage.ID, TurnSeconds: 20})
					m, e := mod.Start(gameapi.Context{Players: players, Now: now}, opts)
					if e != nil {
						t.Fatal(e)
					}
					s, _ := timeline(t, m)
					for seat := 0; seat < players; seat++ {
						raw, _ := json.Marshal(fb.Action{Type: "select", Epoch: s.Epoch, Tank: id})
						m, e = mod.Apply(gameapi.Context{Players: players, Actor: seat, Now: now}, m, raw)
						if e != nil {
							t.Fatal(e)
						}
					}
					spec, _ := fbdata.TankByID(id)
					half, height := 10.0, 28.0 // actual movement body, not transparent sprite bounds
					mask := fbmaps.BaseMask(stage)
					for turn := 0; turn < players; turn++ {
						s, _ = timeline(t, m)
						seat := s.Current
						tank := s.Tanks[seat]
						for dx := -half; dx <= half; dx += 2 {
							for dy := 4.0; dy <= height; dy += 2 {
								if spawnSolid(stage, mask, tank.X+dx, tank.Y-dy) {
									t.Fatalf("seat %d tank %d intersects terrain at spawn (%v,%v), body offset (%v,-%v)", seat, id, tank.X, tank.Y, dx, dy)
								}
							}
						}
						supports := 0
						for dx := -10.0; dx <= 10; dx += 2 {
							if spawnSolid(stage, mask, tank.X+dx, tank.Y+1) {
								supports++
							}
						}
						if supports < 2 {
							t.Fatalf("seat %d unsupported spawn (%v,%v)", seat, tank.X, tank.Y)
						}
						moved := false
						for _, dir := range []int{-1, 1} {
							raw, _ := json.Marshal(fb.Action{Type: "move", Epoch: s.Epoch, Turn: s.Turn, Dir: dir})
							next, e := mod.Apply(gameapi.Context{Players: players, Actor: seat, Now: now}, m, raw)
							if e == nil {
								after, _ := timeline(t, next)
								if after.Tanks[seat].X != tank.X {
									moved = true
								}
							}
						}
						if !moved {
							t.Fatalf("seat %d cannot move either way at (%v,%v)", seat, tank.X, tank.Y)
						}
						viable := false
						angles := []float64{float64(spec.MinAngle), (float64(spec.MinAngle) + float64(spec.Angle)) / 2, float64(spec.Angle)}
						for _, dir := range []int{tank.Facing, -tank.Facing} {
							for _, angle := range angles {
								for power := 0.0; power <= 100; power += 10 {
									raw, _ := json.Marshal(fb.Action{Type: "fire", Epoch: s.Epoch, Turn: s.Turn, Angle: angle, Power: power, Facing: dir, Weapon: "primary"})
									next, e := mod.Apply(gameapi.Context{Players: players, Actor: seat, Now: now}, m, raw)
									if e != nil {
										t.Fatal(e)
									}
									after, _ := timeline(t, next)
									arc := after.LastShot.Arcs[0]
									if arc.OutOfBounds || arc.Hit == seat || math.Hypot(arc.Impact.X-tank.X, arc.Impact.Y-(tank.Y-18)) < 100 {
										continue
									}
									if arc.Hit >= 0 && s.Tanks[arc.Hit].Team == tank.Team {
										continue
									}
									viable = true
									break
								}
								if viable {
									break
								}
							}
							if viable {
								break
							}
						}
						if !viable {
							t.Fatalf("seat %d tank %d has no useful non-self firing route at (%v,%v)", seat, id, tank.X, tank.Y)
						}
						m = timerAt(t, mod, m, s.Deadline)
						now = time.UnixMilli(s.Deadline)
					}
					t.Logf("verified %d actual starts and firing routes", players)
				})
			}
		}
	}
}
