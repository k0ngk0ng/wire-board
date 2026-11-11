package racing_test

// Written before the v3 integration. Failure cases and evidence requirements:
// docs/research/racing-nfs/IMPLEMENTATION-PLAN.md. These exercise the production
// module, not a replacement vehicle, checkpoint, or computer implementation.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/racing/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

var classicTracks = []string{"france", "coastal", "uk", "park"}
var classicVehicles = []string{"p911", "fm50", "ldia", "mcf1", "ccor"}

func classicJSON(t *testing.T, value any) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw(value), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func classicView(t *testing.T, mod gameapi.Module, m gameapi.Match, seat int) map[string]any {
	t.Helper()
	v, err := mod.View(m, seat)
	if err != nil {
		t.Fatal(err)
	}
	return classicJSON(t, v)
}

func classicTrace(t *testing.T, value any) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", ".local", "racing-nfs-backend", "module")
	if base := os.Getenv("WIRE_BOARD_RACING_EVIDENCE"); base != "" {
		dir = filepath.Join(base, "module")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(value, "", "  ")
	name := strings.ReplaceAll(t.Name(), "/", "--") + ".json"
	if err := os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestClassicPreparationAuthenticatesVehicleSelection(t *testing.T) {
	mod := game.Module()
	if mod.Prepare == nil {
		t.Fatal("classic vehicles must be selected in the shared waiting room")
	}
	options, err := mod.Configure(6, raw(map[string]any{"track": "france", "laps": 1}))
	if err != nil {
		t.Fatal(err)
	}
	for seat, vehicle := range classicVehicles {
		before := bytes.Clone(options)
		options, err = mod.Prepare(gameapi.PreparationContext{Actor: seat, Capacity: 6, Bots: []bool{false, false, false, false, false, true}}, options, raw(map[string]any{"type": "vehicle", "vehicle": vehicle}))
		if err != nil {
			t.Fatalf("seat %d choose %s: %v", seat, vehicle, err)
		}
		if len(before) == 0 {
			t.Fatal("empty initial options")
		}
		configured, configureErr := mod.Configure(6, options)
		if configureErr != nil || !bytes.Equal(configured, options) {
			t.Fatalf("preparation does not produce canonical options: %v", configureErr)
		}
	}
	for _, actor := range []int{-1, 6, 99} {
		if _, err = mod.Prepare(gameapi.PreparationContext{Actor: actor, Capacity: 6, Bots: []bool{false, false, false, false, false, true}}, options, raw(map[string]any{"type": "vehicle", "vehicle": "p911"})); err == nil {
			t.Fatalf("invalid waiting-room actor %d selected a vehicle", actor)
		}
	}
	for _, action := range []map[string]any{
		{"type": "vehicle", "vehicle": "unknown"},
		{"type": "vehicle", "vehicle": "p911", "seat": 1},
		{"type": "vehicle", "vehicle": "p911", "mass": 1},
	} {
		if _, err = mod.Prepare(gameapi.PreparationContext{Actor: 0, Capacity: 6, Bots: []bool{false, false, false, false, false, true}}, options, raw(action)); err == nil {
			t.Fatalf("invalid or cross-seat preparation admitted: %v", action)
		}
	}
	ctx := gameapi.Context{Players: 5, Now: time.Unix(100000, 0)}
	m, err := mod.Start(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	v := classicView(t, mod, m, 0)
	if v["physics"] != float64(3) {
		t.Fatalf("new race did not freeze classic physics v3: %v", v["physics"])
	}
	for i, c := range v["cars"].([]any) {
		if c.(map[string]any)["vehicle"] != classicVehicles[i] {
			t.Fatalf("seat %d lost its prepared vehicle", i)
		}
	}
	classicTrace(t, map[string]any{"passed": true, "scope": "production Configure/Prepare/Start/View, authenticated per-seat preparation", "view": v})
}

func TestClassicRoadDataAndCompactProjection(t *testing.T) {
	for _, id := range classicTracks {
		t.Run(id, func(t *testing.T) {
			var found map[string]any
			for _, track := range game.Tracks() {
				if track.ID == id {
					found = classicJSON(t, track)
					break
				}
			}
			if found == nil {
				t.Fatalf("missing original route %s", id)
			}
			points, ok := found["points"].([]any)
			if !ok || len(points) < 300 {
				t.Fatalf("original route was replaced with a few control points: %v", found["points"])
			}
			expectedCounts := map[string]int{"france": 973, "coastal": 1075, "uk": 839, "park": 939}
			if len(points) != expectedCounts[id] {
				t.Fatalf("%s lost original VROAD samples: got %d want %d", id, len(points), expectedCounts[id])
			}
			road, ok := found["road"].([]any)
			if !ok || len(road) != len(points) || found["geometry"] == "" {
				t.Fatal("route height/width samples or immutable geometry identity absent")
			}
			lo, hi := math.Inf(1), math.Inf(-1)
			for _, value := range road {
				p := value.(map[string]any)
				h := p["height"].(float64)
				lo, hi = math.Min(lo, h), math.Max(hi, h)
				if p["left"].(float64) <= 0 || p["right"].(float64) <= 0 {
					t.Fatal("nonpositive original paved road width")
				}
			}
			if hi-lo < 10 {
				t.Fatal("original hills were flattened")
			}
			mod, ctx, m := begin(t, 6, 1, id)
			v := classicView(t, mod, m, 0)
			wire, _ := json.Marshal(v)
			if len(wire) > 8*1024 {
				t.Fatalf("six-car dynamic projection exceeds frozen budget: %d bytes", len(wire))
			}
			wireTrack := v["track"].(map[string]any)
			for _, field := range []string{"road", "points", "barriers", "props"} {
				if rows, ok := wireTrack[field].([]any); ok && len(rows) != 0 {
					t.Fatalf("static %s repeated in dynamic projection", field)
				}
			}
			if wireTrack["geometry"] != found["geometry"] {
				t.Fatal("client and server geometry identities differ")
			}
			restored, err := mod.Restore(ctx, m)
			if err != nil || !bytes.Equal(m.Data, restored.Data) {
				t.Fatalf("restore changed frozen geometry or initial race: %v", err)
			}
			classicTrace(t, map[string]any{"passed": true, "track": id, "routeSamples": len(points), "heightRange": hi - lo, "dynamicBytes": len(wire), "view": v})
		})
	}
}

func TestClassicPreparedVehiclesHaveDistinctProductionDynamics(t *testing.T) {
	mod := game.Module()
	ctx := gameapi.Context{Players: 5, Now: time.Unix(100000, 0)}
	m, err := mod.Start(ctx, raw(map[string]any{"track": "france", "laps": 1, "vehicles": classicVehicles}))
	if err != nil {
		t.Fatal(err)
	}
	var initial []any
	var final map[string]any
	for step := 0; step < 70; step++ {
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		v := view(t, mod, m, 0)
		for seat := range classicVehicles {
			ctx.Actor = seat
			m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": v.Cars[seat].Seq + 1, "steer": 0, "throttle": 1, "brake": false, "drift": false}))
			if err != nil {
				t.Fatal(err)
			}
		}
		ctx.Actor = -1
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
		if step == 29 {
			initial = classicView(t, mod, m, 0)["cars"].([]any)
		}
	}
	final = classicView(t, mod, m, 0)
	different := map[int]bool{}
	for _, value := range final["cars"].([]any) {
		c := value.(map[string]any)
		different[int(c["rpm"].(float64))] = true
		if c["gear"].(float64) < 1 || c["rpm"].(float64) <= 0 || c["vehicle"] == "" {
			t.Fatal("transmission state is absent from authoritative vehicle")
		}
	}
	if len(different) < 3 {
		t.Fatal("prepared cars still share one speed-derived engine")
	}
	classicTrace(t, map[string]any{"passed": true, "scope": "production inputs and timers, no state injection", "initial": initial, "final": final})
}

func TestClassicNaturalCompleteRaces(t *testing.T) {
	for _, id := range classicTracks {
		for _, n := range []int{2, 3, 4, 5, 6} {
			for _, laps := range []int{1, 3, 5} {
				t.Run(fmt.Sprintf("%s-%d-seats-%d-laps", id, n, laps), func(t *testing.T) {
					mod, ctx, m := begin(t, n, laps, id)
					bots := make([]int, n)
					for i := range bots {
						bots[i] = i
					}
					first := view(t, mod, m, -1)
					for seat, car := range first.Cars {
						if car.Vehicle != classicVehicles[seat%len(classicVehicles)] {
							t.Fatalf("default mixed fleet missing: seat %d is %q, want %q", seat, car.Vehicle, classicVehicles[seat%len(classicVehicles)])
						}
					}
					steps := 0
					for ; m.Result == nil && steps < 12040; steps++ {
						m = tick(t, mod, &ctx, m, bots)
					}
					last := view(t, mod, m, -1)
					defer func() {
						classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "production Module, normal drive-only computers, simulated wall clock; no teleport/reset/result injection", "track": id, "players": n, "laps": laps, "steps": steps, "initialAt": first.ServerAt, "final": last, "result": m.Result})
					}()
					if m.Result == nil || last.ServerAt >= last.MaxRaceAt {
						t.Fatal("classic race did not finish naturally before maximum duration")
					}
					for _, c := range last.Cars {
						if c.FinishAt == 0 || c.Lap != laps || c.LastReset != 0 {
							t.Fatalf("seat %d failed drive-only completion: lap=%d checkpoint=%d progress=%.1f speed=%.1f", c.Seat, c.Lap, c.Checkpoint, c.Progress, c.Speed)
						}
					}
				})
			}
		}
	}
}

func TestClassicEveryVehicleNaturalRace(t *testing.T) {
	for _, vehicle := range classicVehicles {
		t.Run(vehicle, func(t *testing.T) {
			mod := game.Module()
			ctx := gameapi.Context{Players: 2, Now: time.Unix(100000, 0)}
			m, err := mod.Start(ctx, raw(map[string]any{"track": "france", "laps": 1, "vehicles": []string{vehicle, vehicle}}))
			if err != nil {
				t.Fatal(err)
			}
			steps := 0
			for ; m.Result == nil && steps < 12040; steps++ {
				m = tick(t, mod, &ctx, m, []int{0, 1})
			}
			last := view(t, mod, m, -1)
			defer func() {
				classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "explicit equal-car production baseline, ordinary drive only", "vehicle": vehicle, "steps": steps, "final": last, "result": m.Result})
			}()
			if m.Result == nil || last.ServerAt >= last.MaxRaceAt {
				t.Fatal("vehicle could not complete an ordinary original course")
			}
			for _, c := range last.Cars {
				if c.FinishAt == 0 || c.Lap != 1 || c.LastReset != 0 {
					t.Fatalf("vehicle failed natural completion: %+v", c)
				}
			}
		})
	}
}

func TestClassicRuleCoordinatesMatchConvertedScene(t *testing.T) {
	for _, id := range classicTracks {
		t.Run(id, func(t *testing.T) {
			path := filepath.Join("..", "assets", "public", "racing", "v3", "tracks", id+".route.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var source struct {
				GeometryHash string `json:"geometryHash"`
				Points       []struct{ X, Y, Z, LeftWidth, RightWidth float64 }
				Barriers     []map[string]any
			}
			if err := json.Unmarshal(data, &source); err != nil {
				t.Fatal(err)
			}
			var track map[string]any
			for _, v := range game.Tracks() {
				if v.ID == id {
					track = classicJSON(t, v)
				}
			}
			if track == nil {
				t.Fatal("original track missing")
			}
			if track["geometry"] != source.GeometryHash {
				t.Fatal("server geometry identity differs from exact scene geometry hash")
			}
			points := track["points"].([]any)
			road := track["road"].([]any)
			for i, p := range source.Points {
				x := points[i].(map[string]any)
				r := road[i].(map[string]any)
				for _, pair := range [][2]float64{{x["x"].(float64), p.X * 10}, {x["y"].(float64), p.Z * 10}, {r["height"].(float64), p.Y * 10}, {r["left"].(float64), p.LeftWidth * 10}, {r["right"].(float64), p.RightWidth * 10}} {
					if math.Abs(pair[0]-pair[1]) > .00001 {
						t.Fatalf("road sample %d shifted, flattened, rescaled or lost its original width: %v", i, pair)
					}
				}
			}
			if len(track["barriers"].([]any)) < 500 {
				t.Fatal("native solid boundaries missing")
			}
			classicTrace(t, map[string]any{"passed": true, "track": id, "samples": len(points), "geometry": source.GeometryHash, "scope": "production numeric rules compared with converted original scene coordinates"})
		})
	}
}

func TestClassicLegacySnapshotKeepsItsOriginalRules(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "metro")
	saved := classicJSON(t, m.Data)
	delete(saved, "physics")
	track := saved["track"].(map[string]any)
	points := track["points"].([]any)
	point := points[2].(map[string]any)
	point["x"] = point["x"].(float64) + 17.25
	saved["maxRaceAt"] = saved["maxRaceAt"].(float64) + 1234
	m.Data = raw(saved)
	before := bytes.Clone(m.Data)
	restored, err := mod.Restore(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, restored.Data) {
		t.Fatal("restore rewrote a physics-zero historical snapshot")
	}
	v := classicView(t, mod, restored, 0)
	if physics, ok := v["physics"]; ok && physics != float64(0) {
		t.Fatalf("legacy physics changed: %v", physics)
	}
	back := v["track"].(map[string]any)["points"].([]any)[2].(map[string]any)
	if back["x"] != point["x"] || v["maxRaceAt"] != saved["maxRaceAt"] {
		t.Fatal("restored legacy geometry or deadline was replaced")
	}
	classicTrace(t, map[string]any{"passed": true, "scope": "explicit old-ID compatibility branch, not a delivered classic track", "view": v})
}

func TestClassicPersistentMotionReverseAndProtection(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "france")
	for i := 0; i < 32; i++ {
		m = tick(t, mod, &ctx, m, nil)
	}
	for i := 0; i < 30; i++ {
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		ctx.Actor = 0
		v := view(t, mod, m, 0)
		var err error
		m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": v.Cars[0].Seq + 1, "steer": 0, "throttle": 0, "brake": true, "brakeAmount": 1, "drift": false}))
		if err != nil {
			t.Fatal(err)
		}
		ctx.Actor = -1
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
	}
	v := classicView(t, mod, m, 0)
	c := v["cars"].([]any)[0].(map[string]any)
	if c["gear"] != float64(-1) || c["motion"] == nil {
		t.Fatal("held valid brake did not enter persistent reverse motion")
	}
	restored, err := mod.Restore(ctx, m)
	if err != nil || !bytes.Equal(restored.Data, m.Data) {
		t.Fatalf("motion restore changed current race: %v", err)
	}
	ctx.Actor = 0
	m, err = mod.Reclaim(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		m = tick(t, mod, &ctx, m, nil)
	}
	v = classicView(t, mod, m, 0)
	c = v["cars"].([]any)[0].(map[string]any)
	if c["speed"].(float64) > .5 {
		t.Fatalf("protection brake kept driving: speed=%v", c["speed"])
	}
	if c["lastReset"].(float64) != 0 {
		t.Fatal("protection silently reset the car")
	}
	classicTrace(t, map[string]any{"passed": true, "scope": "production drive, timer, restore and reclaim; reverse uses ordinary inputs", "view": v})
}

func TestClassicClientRoadMirrorMatchesProduction(t *testing.T) {
	tracks := []game.Track{}
	probes := []map[string]any{}
	collisions := []map[string]any{}
	for _, track := range game.Tracks() {
		if track.Version != 3 {
			continue
		}
		mod, _, m := begin(t, 2, 1, track.ID)
		if classicView(t, mod, m, 0)["track"].(map[string]any)["geometry"] != track.Geometry {
			t.Fatal("production room selected a different immutable geometry")
		}
		tracks = append(tracks, track)
		for k := 0; k < 9; k++ {
			i := k * len(track.Points) / 9
			j := (i + 1) % len(track.Points)
			p, q := track.Points[i], track.Points[j]
			r := track.Road[i]
			for _, offset := range []float64{-r.Left - 20, 0, r.Right + 20} {
				l := math.Hypot(r.RX, r.RZ)
				x := p.X + (q.X-p.X)*.37 + r.RX/l*offset
				y := p.Y + (q.Y-p.Y)*.37 + r.RZ/l*offset
				h := r.Height + (track.Road[j].Height-r.Height)*.37
				got := game.ClassicRoadAt(track, x, y, h, i)
				heading := math.Atan2(q.Y-p.Y, q.X-p.X)
				probes = append(probes, map[string]any{"track": track.ID, "x": x, "y": y, "height": h, "hint": i, "heading": heading, "contact": got, "surface": game.ClassicSurfaceFor(got, heading)})
			}
		}
		for i := 0; i < 24 && i < len(track.Barriers); i++ {
			b := track.Barriers[i]
			s := game.ClassicMotionState{X: (b.A.X + b.B.X) / 20, Z: (b.A.Y + b.B.Y) / 20, VX: 7, VZ: 2, Angle: float64(i) * .23}
			collisions = append(collisions, map[string]any{"track": track.ID, "motion": s, "vehicle": "p911", "height": b.Bottom, "expected": game.CollideClassicRoad(track, s, "p911", b.Bottom)})
		}
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".local", "racing-nfs-backend")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(dir, "road-parity-input.json")
	b, _ := json.Marshal(map[string]any{"tracks": tracks, "probes": probes, "collisions": collisions})
	if err := os.WriteFile(fixture, b, 0644); err != nil {
		t.Fatal(err)
	}
	runner := filepath.Join(dir, "road-parity-runner.mjs")
	if err := os.WriteFile(runner, []byte(classicRoadParityRunner), 0644); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(dir, "road-parity-report.json")
	cmd := exec.Command("node", "--experimental-strip-types", runner, fixture, filepath.Join(root, "games/racing/web/classic-road.ts"), filepath.Join(root, "games/racing/assets/public/racing/v3/tracks"), report)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("production/client road verification failed: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}

func TestClassicSixCarDynamicBudgetDuringDriving(t *testing.T) {
	mod, ctx, m := begin(t, 6, 1, "france")
	bots := []int{0, 1, 2, 3, 4, 5}
	largest, events := 0, 0
	for i := 0; i < 220; i++ {
		m = tick(t, mod, &ctx, m, bots)
		v := classicView(t, mod, m, 0)
		payload := raw(v)
		largest = max(largest, len(payload))
		events = max(events, len(v["events"].([]any)))
		if len(payload) > 8*1024 {
			t.Fatalf("six-car moving projection exceeds frozen 8KiB budget at step %d: %d bytes", i, len(payload))
		}
	}
	classicTrace(t, map[string]any{"passed": true, "scope": "production six-car timer and drive projections, all dynamic fields retained", "maximumBytes": largest, "maximumEvents": events})
}

func TestClassicCollisionLoadKeepsAllEventsWithinBudget(t *testing.T) {
	mod := game.Module()
	ctx := gameapi.Context{Players: 6, Now: time.Unix(100000, 0)}
	m, err := mod.Start(ctx, raw(map[string]any{"track": "uk", "laps": 1, "vehicles": []string{"ldia", "ldia", "ldia", "ldia", "ldia", "ldia"}}))
	if err != nil {
		t.Fatal(err)
	}
	largest, maxEvents, maxHorns := 0, 0, 0
	resets := 0
	collisions := map[int]bool{}
	for step := 0; step < 500; step++ {
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		v := view(t, mod, m, 0)
		for seat := 0; seat < 6; seat++ {
			ctx.Actor = seat
			car := v.Cars[seat]
			if step >= 300 && step%55 == 0 && v.Phase != "countdown" && car.Speed < 54.99 && ctx.Now.UnixMilli() >= car.ResetUntil && (car.LastReset == 0 || ctx.Now.UnixMilli()-car.LastReset >= 5000) {
				m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "reset", "epoch": v.Epoch}))
				if err != nil {
					t.Fatal(err)
				}
				resets++
			}
			steer := .75
			if seat%2 == 1 {
				steer = -steer
			}
			phase := int64(0)
			if v.ServerAt > v.StartAt {
				phase = (v.ServerAt - v.StartAt) / 100 % 70
			}
			reverse := phase >= 45
			throttle, pressure := 1., 0.
			if reverse {
				throttle = 0
				pressure = 1
				steer = 0
			} else if v.Cars[seat].Gear < 0 {
				steer = 0
			}
			m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": v.Cars[seat].Seq + 1, "steer": steer, "throttle": throttle, "brake": pressure > 0, "brakeAmount": pressure, "drift": false, "horn": true}))
			if err != nil {
				t.Fatal(err)
			}
		}
		ctx.Actor = -1
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
		wire := classicView(t, mod, m, 0)
		b := raw(wire)
		horns := 0
		for _, car := range wire["cars"].([]any) {
			if car.(map[string]any)["horn"] == true {
				horns++
			}
		}
		maxHorns = max(maxHorns, horns)
		events := wire["events"].([]any)
		for _, event := range events {
			e := event.(map[string]any)
			if e["type"] == "collision" {
				collisions[int(e["id"].(float64))] = true
			}
		}
		largest = max(largest, len(b))
		maxEvents = max(maxEvents, len(events))
		if len(b) > 8*1024 {
			t.Fatalf("collision projection exceeds 8KiB at step %d: %d", step, len(b))
		}
		var stored struct {
			Events []any `json:"events"`
		}
		if err := json.Unmarshal(m.Data, &stored); err != nil {
			t.Fatal(err)
		}
		if len(stored.Events) != len(events) {
			t.Fatal("dynamic budget achieved by discarding collision history")
		}
	}
	if maxEvents < 24 {
		t.Fatalf("collision load did not cover the complete retained event ring: %d", maxEvents)
	}
	if maxHorns != 6 {
		t.Fatalf("collision budget did not include six simultaneous horn flags: %d", maxHorns)
	}
	if len(collisions) < 8 || resets < 6 {
		t.Fatalf("adversarial manual control did not exercise collision/reset load: collisions=%d resets=%d", len(collisions), resets)
	}
	classicTrace(t, map[string]any{"passed": true, "scope": "production six manual cars with held horns, drive/collision/authorized low-speed reset load; no event/state injection; complete 24-event history retained; separate natural AI matrix forbids every reset", "maximumBytes": largest, "maximumEvents": maxEvents, "maximumSimultaneousHorns": maxHorns, "collisions": len(collisions), "legalResets": resets, "final": classicView(t, mod, m, 0)})
}

func TestClassicMixedDrivingAssistanceRestoreAndReclaim(t *testing.T) {
	mod := game.Module()
	ctx := gameapi.Context{Players: 3, Now: time.Unix(100000, 0)}
	m, err := mod.Start(ctx, raw(map[string]any{"track": "france", "laps": 1, "vehicles": []string{"p911", "fm50", "ldia"}}))
	if err != nil {
		t.Fatal(err)
	}
	initial := view(t, mod, m, 0)
	for step := 0; step < 65; step++ {
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		v := view(t, mod, m, 0)
		ctx.Actor = 0
		m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": v.Cars[0].Seq + 1, "steer": 0, "throttle": 1, "brake": false, "drift": false}))
		if err != nil {
			t.Fatal(err)
		}
		for _, seat := range []int{1, 2} {
			ctx.Actor = seat
			a, e := mod.BotAction(ctx, m)
			if e != nil {
				t.Fatal(e)
			}
			if len(a) > 0 {
				m, err = mod.Apply(ctx, m, a)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		ctx.Actor = -1
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
	}
	manual := view(t, mod, m, 0)
	if manual.Cars[0].Progress <= 0 || manual.Cars[0].RPM <= 0 {
		t.Fatal("manual seat did not drive beside explicit computers")
	}
	for step := 0; step < 80; step++ {
		m = tick(t, mod, &ctx, m, []int{0, 1, 2})
	}
	before := view(t, mod, m, 0)
	ctx.Actor = 0
	m, err = mod.Reclaim(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	after := view(t, mod, m, 0)
	if before.Epoch != after.Epoch || before.StartAt != after.StartAt || before.MaxRaceAt != after.MaxRaceAt || before.Cars[1].Progress != after.Cars[1].Progress || before.Cars[2].Seq != after.Cars[2].Seq {
		t.Fatal("manual reclaim reset another seat or the race")
	}
	if !after.Cars[0].Protected || after.Cars[0].Motion.Throttle != 0 || after.Cars[0].Motion.Brake != 1 {
		t.Fatal("reclaim protection was not publicly distinguishable")
	}
	for step := 0; step < 25; step++ {
		m = tick(t, mod, &ctx, m, []int{1, 2})
	}
	restored, e := mod.Restore(ctx, m)
	if e != nil || !bytes.Equal(restored.Data, m.Data) {
		t.Fatalf("mixed live motion restoration changed the race: %v", e)
	}
	m = restored
	steps := 0
	for ; m.Result == nil && steps < 12040; steps++ {
		m = tick(t, mod, &ctx, m, []int{0, 1, 2})
	}
	final := view(t, mod, m, 0)
	defer func() {
		classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "production manual drive with computers, explicit assistance, manual reclaim, input-loss protection, restore and natural finish; no state/position/result injection", "initial": initial, "manual": manual, "reclaimed": after, "final": final, "result": m.Result})
	}()
	if m.Result == nil || final.ServerAt >= final.MaxRaceAt {
		t.Fatal("mixed race used absolute timeout")
	}
	for _, car := range final.Cars {
		if car.FinishAt == 0 || car.Lap != 1 || car.LastReset != 0 {
			t.Fatalf("mixed vehicle did not naturally finish: seat=%d vehicle=%s lap=%d checkpoint=%d", car.Seat, car.Vehicle, car.Lap, car.Checkpoint)
		}
	}
}

const classicRoadParityRunner = `
import assert from 'node:assert/strict';
import {readFile,writeFile} from 'node:fs/promises';
import {pathToFileURL} from 'node:url';
import path from 'node:path';
const [fixturePath,modulePath,sourceDir,reportPath]=process.argv.slice(2);
const api=await import(pathToFileURL(modulePath).href);
const input=JSON.parse(await readFile(fixturePath,'utf8'));
const tracks=new Map();let checked=0,maxError=0;
function compare(a,b,label){
 if(typeof a==='number'&&typeof b==='number'){const error=Math.abs(a-b);maxError=Math.max(maxError,error);assert.ok(error<1e-6,label+': '+a+' vs '+b);checked++;return;}
 if(Array.isArray(a)){assert.equal(a.length,b.length,label+' length');for(let i=0;i<a.length;i++)compare(a[i],b[i],label+'['+i+']');return;}
 if(a&&typeof a==='object'){for(const key of Object.keys(a))compare(a[key],b[key],label+'.'+key);return;}
 assert.equal(a,b,label);
}
for(const expected of input.tracks){const raw=JSON.parse(await readFile(path.join(sourceDir,expected.id+'.route.json'),'utf8'));const actual=api.classicTrackFromRoute(raw);compare(expected,actual,expected.id);tracks.set(expected.id,actual);}
for(const p of input.probes){const contact=api.classicRoadAt(tracks.get(p.track),p.x,p.y,p.height,p.hint);compare(p.contact,contact,'contact '+p.track);compare(p.surface,api.classicRoadSurface(contact,p.heading),'surface '+p.track);}
for(const p of input.collisions)compare(p.expected,api.collideClassicRoad(tracks.get(p.track),p.motion,p.vehicle,p.height),'collision '+p.track);
await writeFile(reportPath,JSON.stringify({passed:true,tracks:tracks.size,probes:input.probes.length,collisions:input.collisions.length,numericFields:checked,maxError},null,2)+'\n');
console.log('Verified '+tracks.size+' native routes, '+input.probes.length+' road/surface probes and '+input.collisions.length+' production/client collision probes.');
`

func TestClassicHornUsesOrderedDriveAndProtection(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "france")
	for i := 0; i < 33; i++ {
		m = tick(t, mod, &ctx, m, nil)
	}
	horn := func(seat int) bool {
		v := classicView(t, mod, m, seat)
		c := v["cars"].([]any)[seat].(map[string]any)
		return c["horn"] == true
	}
	drive := func(value any) {
		t.Helper()
		v := view(t, mod, m, 0)
		ctx.Actor = 0
		action := map[string]any{"type": "drive", "epoch": v.Epoch, "seq": v.Cars[0].Seq + 1, "steer": 0, "throttle": 0, "brake": false, "drift": false}
		if value != nil {
			action["horn"] = value
		}
		var err error
		m, err = mod.Apply(ctx, m, raw(action))
		if err != nil {
			t.Fatal(err)
		}
	}
	drive(true)
	if horn(0) {
		t.Fatal("future horn input was projected before its authoritative motion frame")
	}
	m = tick(t, mod, &ctx, m, nil)
	if !horn(0) || horn(1) {
		t.Fatal("held horn was not limited to the authenticated driver")
	}
	ctx.Now = ctx.Now.Add(10 * time.Millisecond)
	drive(false)
	if !horn(0) {
		t.Fatal("horn release was projected before the applied input timeline")
	}
	ctx.Now = ctx.Now.Add(60 * time.Millisecond)
	drive(true)
	stored := classicJSON(t, m.Data)
	cars := stored["cars"].([]any)
	pending := cars[0].(map[string]any)["pending"].([]any)
	if len(pending) != 2 || pending[0].(map[string]any)["horn"] == true || pending[1].(map[string]any)["horn"] != true {
		t.Fatal("queued inputs lost their individual horn levels")
	}
	restored, err := mod.Restore(ctx, m)
	if err != nil || !bytes.Equal(restored.Data, m.Data) {
		t.Fatalf("queued horn restore changed input timing: %v", err)
	}
	m = restored
	ctx.Now = time.UnixMilli(view(t, mod, m, 0).ServerAt + 100)
	ctx.Actor = -1
	m, err = mod.OnTimer(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if !horn(0) {
		t.Fatal("last held horn input did not apply")
	}
	for i := 0; i < 9; i++ {
		m = tick(t, mod, &ctx, m, nil)
	}
	if horn(0) {
		t.Fatal("expired horn was left sounding")
	}
	drive(true)
	m = tick(t, mod, &ctx, m, nil)
	if !horn(0) {
		t.Fatal("fresh drive could not sound the horn after protection")
	}
	ctx.Actor = 0
	m, err = mod.Reclaim(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if horn(0) {
		t.Fatal("manual reclaim retained a held horn")
	}
	drive(true)
	m = tick(t, mod, &ctx, m, nil)
	if !horn(0) {
		t.Fatal("fresh horn did not resume")
	}
	v := view(t, mod, m, 0)
	ctx.Actor = 0
	m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "reset", "epoch": v.Epoch}))
	if err != nil {
		t.Fatal(err)
	}
	if horn(0) {
		t.Fatal("legal reset retained a held horn")
	}
	drive(nil)
	m = tick(t, mod, &ctx, m, nil)
	if horn(0) {
		t.Fatal("legacy drive payload without horn was not compatible")
	}
	ctx.Actor = -1
	v = view(t, mod, m, 0)
	before := bytes.Clone(m.Data)
	_, err = mod.Apply(ctx, m, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": v.Cars[0].Seq + 1, "horn": true}))
	if err == nil || !bytes.Equal(before, m.Data) {
		t.Fatal("spectator changed a horn")
	}
	ctx.Actor = 0
	_, err = mod.Apply(ctx, m, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": v.Cars[0].Seq + 1, "horn": true, "seat": 1}))
	if err == nil {
		t.Fatal("cross-seat horn action was admitted")
	}
	_, err = mod.Apply(ctx, m, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": v.Cars[0].Seq + 1, "horn": 1}))
	if err == nil {
		t.Fatal("nonboolean horn input was admitted")
	}
	classicTrace(t, map[string]any{"passed": true, "scope": "production ordered drive horn press/release/pending/restore/expiry/reclaim/reset/old payload/authentication", "view": classicView(t, mod, m, 0)})
}

func TestClassicHornCannotChangeMotionRankingOrClock(t *testing.T) {
	mod, ctx, a := begin(t, 2, 1, "france")
	b, err := mod.Start(ctx, raw(map[string]any{"track": "france", "laps": 1}))
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 85; step++ {
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		for _, pair := range []struct {
			match *gameapi.Match
			horn  bool
		}{{&a, true}, {&b, false}} {
			v := view(t, mod, *pair.match, 0)
			ctx.Actor = 0
			*pair.match, err = mod.Apply(ctx, *pair.match, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": v.Cars[0].Seq + 1, "steer": .02, "throttle": 1, "brake": false, "drift": false, "horn": pair.horn}))
			if err != nil {
				t.Fatal(err)
			}
			ctx.Actor = -1
			*pair.match, err = mod.OnTimer(ctx, *pair.match)
			if err != nil {
				t.Fatal(err)
			}
		}
		va, vb := view(t, mod, a, 0), view(t, mod, b, 0)
		ca, cb := va.Cars[0], vb.Cars[0]
		if ca.X != cb.X || ca.Y != cb.Y || ca.VX != cb.VX || ca.VY != cb.VY || ca.Angle != cb.Angle || ca.RPM != cb.RPM || ca.Gear != cb.Gear || ca.Progress != cb.Progress || ca.Rank != cb.Rank || ca.Checkpoint != cb.Checkpoint || va.ServerAt != vb.ServerAt || va.StartAt != vb.StartAt || va.MaxRaceAt != vb.MaxRaceAt || va.FinishDeadline != vb.FinishDeadline {
			t.Fatal("horn changed authoritative driving, ranking or a deadline")
		}
	}
	classicTrace(t, map[string]any{"passed": true, "scope": "parallel production drive timelines with/without held horn have exactly equal motion/ranking/clock", "hornView": view(t, mod, a, 0), "silentView": view(t, mod, b, 0)})
}

func TestClassicHornStopsForUnfinishedCarAtRaceDeadline(t *testing.T) {
	for _, deadline := range []string{"race-limit", "finish-deadline"} {
		t.Run(deadline, func(t *testing.T) {
			mod, ctx, m := begin(t, 2, 1, "france")
			var err error
			if deadline == "race-limit" {
				// Exercise the real twenty-minute limit without changing a
				// snapshot, deadline or physical integration step.
				ctx.Now = time.UnixMilli(view(t, mod, m, 0).MaxRaceAt - 200)
				m, err = mod.OnTimer(ctx, m)
				if err != nil {
					t.Fatal(err)
				}
			}
			var held map[string]any
			for step := 0; m.Result == nil && step < 12040; step++ {
				v := view(t, mod, m, 0)
				ctx.Actor = 1
				m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": v.Cars[1].Seq + 1, "throttle": 0, "brake": true, "horn": true}))
				if err != nil {
					t.Fatal(err)
				}
				var computers []int
				if deadline == "finish-deadline" {
					computers = []int{0}
				}
				m = tick(t, mod, &ctx, m, computers)
				if m.Result == nil {
					held = classicView(t, mod, m, 1)
				}
			}
			final := classicView(t, mod, m, 1)
			defer func() {
				classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "production timers, held manual horn and ordinary driving; no state/deadline/result injection", "deadline": deadline, "beforeFinish": held, "final": final, "result": m.Result})
			}()
			if m.Result == nil || held == nil || held["cars"].([]any)[1].(map[string]any)["horn"] != true {
				t.Fatal("deadline case did not exercise an active unfinished-car horn")
			}
			car := final["cars"].([]any)[1].(map[string]any)
			if car["finishAt"].(float64) != 0 || final["phase"] != "finished" {
				t.Fatal("deadline case did not end with an unfinished car")
			}
			if deadline == "race-limit" && final["serverAt"] != final["maxRaceAt"] || deadline == "finish-deadline" && (final["finishDeadline"].(float64) == 0 || final["serverAt"].(float64) < final["finishDeadline"].(float64)) {
				t.Fatal("deadline case finished for the wrong reason")
			}
			if car["horn"] == true {
				t.Fatal("finished race left an unfinished car sounding forever")
			}
		})
	}
}
