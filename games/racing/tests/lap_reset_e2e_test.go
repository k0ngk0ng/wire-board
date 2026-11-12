package racing_test

// Failure inventory precedes this implementation: lap-reset-failures.md.
// Drive the original course through production inputs, without bot gate repair.
import (
	"fmt"
	"math"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/racing/server"
)

func TestClassicHumanLinesCountCompletedCircuit(t *testing.T) {
	for _, id := range []string{"uk"} {
		for _, mode := range []string{"normal", "shoulder", "shoulder-reset-three"} {
			offset := -20.
			t.Run(fmt.Sprintf("%s-%s", id, mode), func(t *testing.T) {
				wantedLaps := 1
				if mode == "shoulder-reset-three" {
					wantedLaps = 3
				}
				mod, ctx, m := begin(t, 2, wantedLaps, id)
				resetDone := false
				var track game.Track
				for _, candidate := range game.Tracks() {
					if candidate.ID == id {
						track = candidate
					}
				}
				arcs := make([]float64, len(track.Points))
				total := 0.
				for i, p := range track.Points {
					arcs[i] = total
					j := (i + 1) % len(track.Points)
					total += math.Sqrt(math.Pow(track.Points[j].X-p.X, 2) + math.Pow(track.Points[j].Y-p.Y, 2) + math.Pow(track.Road[j].Height-track.Road[i].Height, 2))
				}
				params := game.ClassicVehicle("p911")
				previous, travelled := math.NaN(), 0.
				trace := []any{}
				defer func() {
					classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "original-grid human-like constant-lane ordinary drive; no checkpoint repair/teleport/reset", "track": id, "offsetDM": offset, "travelledDM": travelled, "lengthDM": total, "trace": trace, "final": view(t, mod, m, 0)})
				}()
				for step := 0; step < 12000; step++ {
					v := view(t, mod, m, 0)
					c := v.Cars[0]
					road := game.ClassicRoadAt(track, c.X, c.Y, c.Height, c.RouteIndex)
					j := (road.Index + 1) % len(track.Points)
					span := total - arcs[road.Index]
					if j > road.Index {
						span = arcs[j] - arcs[road.Index]
					}
					at := arcs[road.Index] + road.Fraction*span
					if !math.IsNaN(previous) {
						delta := at - previous
						if delta < -total/2 {
							delta += total
						}
						if delta > total/2 {
							delta -= total
						}
						if math.Abs(delta) < 200 {
							travelled += delta
						}
					}
					previous = at
					if step%20 == 0 {
						trace = append(trace, map[string]any{"at": v.ServerAt, "car": c, "road": road, "travelled": travelled})
					}
					if travelled > total*float64(c.Lap+1)+100 || c.Lap >= wantedLaps {
						if c.Lap != wantedLaps || c.FinishAt == 0 || travelled < total*float64(wantedLaps)-100 {
							t.Fatalf("physically completed circuit without lap: travelled=%.1fm length=%.1fm gate=%d checkpoint=%d route=%d lap=%d", travelled/10, total/10, c.Gate, c.Checkpoint, c.RouteIndex, c.Lap)
						}
						return
					}
					i := (road.Index + 3) % len(track.Points)
					target := track.Points[i]
					r := track.Road[i]
					rl := math.Hypot(r.RX, r.RZ)
					lane := math.Max(-r.Left+12, math.Min(r.Right-12, offset))
					if mode != "normal" && i >= 20 && i <= 35 {
						lane = -r.Left - 18
					}
					if rl > 0 {
						target.X += r.RX / rl * lane
						target.Y += r.RZ / rl * lane
					}
					dx, dz := (target.X-c.X)/10, (target.Y-c.Y)/10
					diff := math.Atan2(math.Sin(math.Atan2(dz, dx)-c.Angle), math.Cos(math.Atan2(dz, dx)-c.Angle))
					steer := math.Max(-1, math.Min(1, math.Atan2(2*params.Wheelbase*math.Sin(diff), math.Max(4, math.Hypot(dx, dz)))/math.Atan(params.Wheelbase/params.TurningRadius)))
					speed := 12.
					if math.Abs(diff) > .7 {
						speed = 6
					}
					actual := c.Speed / 10
					pressure := math.Max(0, math.Min(1, (actual-speed-.5)/3))
					throttle := math.Max(0, math.Min(1, .18+(speed-actual)/4))
					if pressure > 0 {
						throttle = 0
					}
					if mode == "shoulder-reset-three" && !resetDone && road.Index >= 65 && road.Index < 85 {
						throttle = 0
						pressure = 1
						if c.Speed < 3 {
							ctx.Actor = 0
							before := c
							var err error
							m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "reset", "epoch": v.Epoch}))
							if err != nil {
								t.Fatal(err)
							}
							after := view(t, mod, m, 0).Cars[0]
							trace = append(trace, map[string]any{"resetBefore": before, "resetAfter": after})
							if math.Hypot(after.X-before.X, after.Y-before.Y) > 150 {
								t.Fatalf("reset jumped to stale checkpoint: %.1fm; gate=%d route=%d", math.Hypot(after.X-before.X, after.Y-before.Y)/10, before.Gate, before.RouteIndex)
							}
							if after.Lap != before.Lap || math.Abs(after.Progress-before.Progress) > 1 {
								t.Fatal("reset changed earned lap/progress")
							}
							resetDone = true
							m, err = mod.Restore(ctx, m)
							if err != nil {
								t.Fatal(err)
							}
							continue
						}
					}
					ctx.Now = ctx.Now.Add(100 * time.Millisecond)
					ctx.Actor = 0
					var err error
					m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": c.Seq + 1, "steer": steer, "throttle": throttle, "brake": pressure > 0, "brakeAmount": pressure, "drift": false}))
					if err != nil {
						t.Fatal(err)
					}
					ctx.Actor = -1
					m, err = mod.OnTimer(ctx, m)
					if err != nil {
						t.Fatal(err)
					}
				}
				t.Fatalf("driver did not complete physical circuit, travelled=%.1fm", travelled/10)
			})
		}
	}
}
