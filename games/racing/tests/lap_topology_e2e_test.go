package racing_test

// Extended failure inventory is written first in lap-reset-failures.md.
// Every trajectory starts at the production grid and sends normal drive/reset.
// No checkpoint repair, bot action, car-position/result or clock-state injection.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/racing/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestClassicContinuousOffroadLaps(t *testing.T) {
	zones := map[string]struct{ start, end, side int }{
		"france": {562, 594, 1}, "coastal": {8, 17, -1},
		"uk": {803, 831, -1}, "park": {176, 192, 1},
	}
	for _, id := range classicTracks {
		for _, scenario := range []string{"normal", "offroad", "offroad-reset-three"} {
			extra := 100.
			if scenario == "normal" {
				extra = 0
			}
			doReset := scenario == "offroad-reset-three"
			if doReset && (id == "coastal" || id == "park") {
				extra = 200
			}
			t.Run(fmt.Sprintf("%s-%s", id, scenario), func(t *testing.T) {
				laps := 1
				if doReset {
					laps = 3
				}
				mod, ctx, m := begin(t, 2, laps, id)
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
				zone, phase := zones[id], 0
				if extra == 0 {
					phase = 4
				}
				resetDone, returned := false, false
				earnedResetDone := false
				maxExtra, travelled, previous := 0., 0., math.NaN()
				trace := []any{}
				params := game.ClassicVehicle("p911")
				defer func() {
					classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "production original grid, ordinary manual drive leaving actual paved edge and returning; authorized R and Restore; no bot/position/checkpoint/result injection", "track": id, "requestedExtraDM": extra, "maximumActualExtraDM": maxExtra, "returned": returned, "resetDone": resetDone, "physicalRouteTravelDM": travelled, "routeLengthDM": total, "trace": trace, "final": view(t, mod, m, 0), "saved": json.RawMessage(m.Data)})
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
					arc := arcs[road.Index] + road.Fraction*span
					if !math.IsNaN(previous) {
						delta := arc - previous
						if delta < -total/2 {
							delta += total
						}
						if delta > total/2 {
							delta -= total
						}
						travelled += delta
					}
					previous = arc
					outside := math.Max(0, math.Max(road.Side-road.Right, -road.Side-road.Left))
					maxExtra = math.Max(maxExtra, outside)
					if phase == 0 && road.Index >= zone.start && road.Index <= zone.start+3 {
						phase = 1
					}
					if phase == 1 && outside >= extra {
						phase = 2
					}
					if phase == 2 && road.Index >= zone.end-4 {
						phase = 3
					}
					if phase == 3 && outside < 3 && road.Index >= zone.end-3 {
						phase, returned = 4, true
					}
					if step%20 == 0 {
						trace = append(trace, map[string]any{"at": v.ServerAt, "car": c, "road": road, "phase": phase, "outside": outside, "travelled": travelled})
					}
					if c.Lap == laps {
						if maxExtra < extra || extra > 0 && !returned || doReset && (!resetDone || !earnedResetDone) || c.FinishAt == 0 {
							t.Fatal("race did not cover the complete excursion/reset workload")
						}
						return
					}
					if doReset && c.Lap >= 1 && !earnedResetDone {
						ctx.Actor = 0
						var err error
						m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "reset", "epoch": v.Epoch}))
						if err != nil {
							t.Fatal("ordinary moving R after earned lap rejected", err)
						}
						after := view(t, mod, m, 0).Cars[0]
						if after.Lap != c.Lap || math.Abs(after.Progress-c.Progress) > 1 || math.Hypot(after.X-c.X, after.Y-c.Y) > 150 {
							t.Fatal("moving R changed earned lap/progress or used distant anchor")
						}
						trace = append(trace, map[string]any{"earnedLapReset": true, "resetBefore": c, "resetAfter": after})
						earnedResetDone = true
						continue
					}
					if travelled > total*float64(c.Lap+1)+100 {
						t.Fatalf("real complete circuit stayed at lap=%d; route=%d gate=%d checkpoint=%d maxExtra=%.2fm", c.Lap, road.Index, c.Gate, c.Checkpoint, maxExtra/10)
					}
					index := (road.Index + 3) % len(track.Points)
					lane, speed := -20., 14.
					if id == "coastal" {
						speed = 17
					}
					if id == "uk" {
						speed = 12
					}
					if phase == 1 {
						index, speed = zone.start+3, 4
					}
					if phase == 2 {
						index, speed = min(road.Index+3, zone.end-2), 4
					}
					if phase == 3 {
						index, speed = zone.end-1, 4
					}
					if phase == 1 || phase == 2 {
						r := track.Road[index]
						width := r.Right
						if zone.side < 0 {
							width = r.Left
						}
						goalExtra := extra
						lane = float64(zone.side) * (width + goalExtra + 10)
					}
					// Preview actual route curvature, independent of checkpoints.
					if phase == 0 || phase == 4 {
						for k := 0; k < 20; k++ {
							a, b, d := track.Points[(road.Index+k)%len(track.Points)], track.Points[(road.Index+k+1)%len(track.Points)], track.Points[(road.Index+k+2)%len(track.Points)]
							l1, l2 := math.Hypot(b.X-a.X, b.Y-a.Y), math.Hypot(d.X-b.X, d.Y-b.Y)
							turn := math.Abs(math.Atan2((b.X-a.X)*(d.Y-b.Y)-(b.Y-a.Y)*(d.X-b.X), (b.X-a.X)*(d.X-b.X)+(b.Y-a.Y)*(d.Y-b.Y)))
							curvature := turn / math.Max(.1, (l1+l2)/20)
							if curvature > 1e-5 {
								speed = math.Min(speed, math.Sqrt(4/curvature+2*6*float64(k)*4.6))
							}
						}
					}
					target, r := track.Points[index], track.Road[index]
					length := math.Hypot(r.RX, r.RZ)
					target.X += r.RX / length * lane
					target.Y += r.RZ / length * lane
					dx, dz := (target.X-c.X)/10, (target.Y-c.Y)/10
					difference := math.Atan2(math.Sin(math.Atan2(dz, dx)-c.Angle), math.Cos(math.Atan2(dz, dx)-c.Angle))
					steer := math.Max(-1, math.Min(1, math.Atan2(2*params.Wheelbase*math.Sin(difference), math.Max(4, math.Hypot(dx, dz)))/math.Atan(params.Wheelbase/params.TurningRadius)))
					actual := c.Speed / 10
					throttle := math.Max(0, math.Min(1, .18+(speed-actual)/4))
					pressure := math.Max(0, math.Min(1, (actual-speed-.5)/3))
					if pressure > 0 {
						throttle = 0
					}
					if doReset && returned && !resetDone {
						throttle, pressure = 0, 1
						if c.Speed < 3 {
							ctx.Actor = 0
							var err error
							m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "reset", "epoch": v.Epoch}))
							if err != nil {
								t.Fatal(err)
							}
							after := view(t, mod, m, 0).Cars[0]
							trace = append(trace, map[string]any{"resetBefore": c, "resetAfter": after})
							if after.Lap != c.Lap || math.Abs(after.Progress-c.Progress) > 1 {
								t.Fatal("R changed completed lap/progress")
							}
							if math.Hypot(after.X-c.X, after.Y-c.Y) > 150 {
								t.Fatalf("R returned %.1fm to a stale gate", math.Hypot(after.X-c.X, after.Y-c.Y)/10)
							}
							restored, e := mod.Restore(ctx, m)
							if e != nil || !bytes.Equal(restored.Data, m.Data) {
								t.Fatal("Restore changed reset race", e)
							}
							m = restored
							resetDone = true
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
				t.Fatal("ordinary excursion driver did not complete within real race deadline")
			})
		}
	}
}

func TestClassicMovingResetHasOnlyAnimationLock(t *testing.T) {
	mod, ctx, m := begin(t, 2, 3, "france")
	ctx.Actor = 0
	v := view(t, mod, m, 0)
	if _, err := mod.Apply(ctx, m, raw(map[string]any{"type": "reset", "epoch": v.Epoch})); err == nil || !strings.Contains(err.Error(), "发车后可复位") {
		t.Error("countdown reset must explain the start lock", err)
	}
	var track game.Track
	for _, candidate := range game.Tracks() {
		if candidate.ID == "france" {
			track = candidate
		}
	}
	for step := 0; step < 100; step++ {
		v = view(t, mod, m, 0)
		if v.Phase == "racing" && v.Cars[0].Speed > 100 {
			break
		}
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		i := (v.Cars[0].RouteIndex + 3) % len(track.Points)
		m = classicPropDrive(t, mod, &ctx, m, 0, track.Points[i], 20)
		ctx.Actor = -1
		var err error
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
	}
	v = view(t, mod, m, 0)
	before := v.Cars[0]
	if before.Speed <= 100 {
		t.Fatal("ordinary driver did not exercise high-speed R")
	}
	if !v.Controls.CanReset {
		t.Error("moving classic car cannot use R")
	}
	ctx.Actor = 0
	var err error
	m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "reset", "epoch": v.Epoch}))
	if err != nil {
		t.Fatal("moving R rejected", err)
	}
	afterView := view(t, mod, m, 0)
	after := afterView.Cars[0]
	if after.Lap != before.Lap || math.Abs(after.Progress-before.Progress) > 1 || after.Speed != 0 || math.Hypot(after.X-before.X, after.Y-before.Y) > 150 {
		t.Fatal("moving R changed lap/progress or used stale/distant gate", before, after)
	}
	if afterView.Epoch != v.Epoch || afterView.StartAt != v.StartAt || afterView.MaxRaceAt != v.MaxRaceAt {
		t.Fatal("moving R changed race identity or deadlines")
	}
	if afterView.Controls.CanReset {
		t.Fatal("reset animation did not lock R")
	}
	if _, err := mod.Apply(ctx, m, raw(map[string]any{"type": "reset", "epoch": v.Epoch})); err == nil || !strings.Contains(err.Error(), "正在复位") {
		t.Fatal("animation lock returned wrong feedback", err)
	}
	ctx.Now = ctx.Now.Add(2100 * time.Millisecond)
	ctx.Actor = -1
	m, err = mod.OnTimer(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	ready := view(t, mod, m, 0)
	if !ready.Controls.CanReset || ready.Cars[0].Lap != before.Lap {
		t.Fatal("extra five-second cooldown or reset lap credit remained")
	}
	ctx.Actor = 0
	m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "reset", "epoch": ready.Epoch}))
	if err != nil {
		t.Fatal("second R after animation still rejected", err)
	}
	classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "production original-grid ordinary acceleration then high-speed R; two-second animation lock only, no position/progress injection", "before": v, "after": afterView, "secondReset": classicView(t, mod, m, 0)})
}

func TestClassicHistoricalOffroadSaveCanContinue(t *testing.T) {
	for _, id := range classicTracks {
		t.Run(id, func(t *testing.T) {
			payload, err := os.ReadFile(filepath.Join("fixtures", "stale-v3-laps", id+".json"))
			if err != nil {
				t.Fatal(err)
			}
			mod := game.Module()
			m := gameapi.Match{Schema: 1, Data: payload}
			before := view(t, mod, m, 0)
			ctx := gameapi.Context{Players: 2, Actor: 0, Now: time.UnixMilli(before.ServerAt)}
			m, err = mod.Restore(ctx, m)
			if err != nil || !bytes.Equal(m.Data, payload) {
				t.Fatal("historical Restore changed original bytes", err)
			}
			if c := view(t, mod, m, 0).Cars[0]; c.Lap != 0 || c.FinishAt != 0 {
				t.Fatal("restore invented unknown historical laps")
			}
			var track game.Track
			for _, candidate := range game.Tracks() {
				if candidate.ID == id {
					track = candidate
				}
			}
			resetDone := false
			trace := []any{}
			defer func() {
				classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "genuine old ordinary-drive offroad save; no position/checkpoint/progress edits; future normal driving and legal R/Restore", "track": id, "before": before, "resetDone": resetDone, "trace": trace, "final": view(t, mod, m, 0)})
			}()
			for step := 0; step < 12000; step++ {
				v := view(t, mod, m, 0)
				c := v.Cars[0]
				if c.Lap == 1 {
					if !resetDone || c.FinishAt == 0 {
						t.Fatal("recovery omitted reset/finish")
					}
					return
				}
				if v.Epoch != before.Epoch || v.StartAt != before.StartAt || v.MaxRaceAt != before.MaxRaceAt {
					t.Fatal("historical recovery changed race identity or deadline")
				}
				road := game.ClassicRoadAt(track, c.X, c.Y, c.Height, c.RouteIndex)
				i := (road.Index + 3) % len(track.Points)
				target, r := track.Points[i], track.Road[i]
				l := math.Hypot(r.RX, r.RZ)
				target.X -= r.RX / l * 20
				target.Y -= r.RZ / l * 20
				speed := 14.
				if id == "uk" {
					speed = 12
				}
				if id == "coastal" {
					speed = 17
				}
				for k := 0; k < 20; k++ {
					a, b, d := track.Points[(road.Index+k)%len(track.Points)], track.Points[(road.Index+k+1)%len(track.Points)], track.Points[(road.Index+k+2)%len(track.Points)]
					l1, l2 := math.Hypot(b.X-a.X, b.Y-a.Y), math.Hypot(d.X-b.X, d.Y-b.Y)
					turn := math.Abs(math.Atan2((b.X-a.X)*(d.Y-b.Y)-(b.Y-a.Y)*(d.X-b.X), (b.X-a.X)*(d.X-b.X)+(b.Y-a.Y)*(d.Y-b.Y)))
					curvature := turn / math.Max(.1, (l1+l2)/20)
					if curvature > 1e-5 {
						speed = math.Min(speed, math.Sqrt(4/curvature+2*6*float64(k)*4.6))
					}
				}
				ctx.Now = ctx.Now.Add(100 * time.Millisecond)
				if !resetDone && road.Index >= 40 && road.Index < 70 {
					if c.Speed < 3 {
						ctx.Actor = 0
						m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "reset", "epoch": v.Epoch}))
						if err != nil {
							t.Fatal(err)
						}
						after := view(t, mod, m, 0).Cars[0]
						if after.Lap != c.Lap || math.Abs(after.Progress-c.Progress) > 1 || math.Hypot(after.X-c.X, after.Y-c.Y) > 150 {
							t.Fatal("historical R used stale gate or changed lap/progress", c, after)
						}
						trace = append(trace, map[string]any{"resetBefore": c, "resetAfter": after})
						resetDone = true
						m, err = mod.Restore(ctx, m)
						if err != nil {
							t.Fatal(err)
						}
					} else {
						ctx.Actor = 0
						m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": c.Seq + 1, "steer": 0, "throttle": 0, "brake": true, "drift": false}))
						if err != nil {
							t.Fatal(err)
						}
					}
				} else {
					m = classicPropDrive(t, mod, &ctx, m, 0, target, speed)
				}
				ctx.Actor = -1
				m, err = mod.OnTimer(ctx, m)
				if err != nil {
					t.Fatal(err)
				}
				if step%20 == 0 {
					trace = append(trace, view(t, mod, m, 0))
				}
			}
			t.Fatal("old stale-gate save remained unable to count a future circuit")
		})
	}
}
