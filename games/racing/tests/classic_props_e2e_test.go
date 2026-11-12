package racing_test

// Failure inventory precedes implementation in classic-props-failures.md.
// These use real production Module snapshots and timers. Historical-format and
// malformed-snapshot cases change optional metadata only, never car positions,
// progress, physical outcomes or deadlines.
import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/racing/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type classicPropProjection struct {
	Catalog string `json:"catalog"`
	Broken  []byte `json:"broken"`
}

func classicPropsView(t *testing.T, mod gameapi.Module, m gameapi.Match) classicPropProjection {
	t.Helper()
	v, err := mod.View(m, -1)
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Props *classicPropProjection `json:"props"`
	}
	if err := json.Unmarshal(raw(v), &data); err != nil {
		t.Fatal(err)
	}
	if data.Props == nil {
		t.Fatal("classic View has no authoritative prop catalog and bitset")
	}
	return *data.Props
}

func TestClassicPropStateAndLegacySnapshotCompatibility(t *testing.T) {
	counts := map[string]int{"france": 29, "coastal": 83, "uk": 108, "park": 45}
	for _, track := range classicTracks {
		t.Run(track, func(t *testing.T) {
			mod, ctx, m := begin(t, 6, 1, track)
			props := classicPropsView(t, mod, m)
			if len(props.Catalog) != 64 || len(props.Broken) != (counts[track]+7)/8 || !bytes.Equal(props.Broken, make([]byte, len(props.Broken))) {
				t.Fatal("new race does not freeze a complete intact prop state", props)
			}
			before := view(t, mod, m, 0)
			var old map[string]any
			if err := json.Unmarshal(m.Data, &old); err != nil {
				t.Fatal(err)
			}
			delete(old, "props")
			legacy := m
			legacy.Data = raw(old)
			restored, err := mod.Restore(ctx, legacy)
			if err != nil || !bytes.Equal(restored.Data, legacy.Data) {
				t.Fatalf("old v3 restore rewrote the race: %v", err)
			}
			after := view(t, mod, restored, 0)
			if !bytes.Equal(raw(before), raw(after)) || !bytes.Equal(raw(props), raw(classicPropsView(t, mod, restored))) {
				t.Fatal("missing prop metadata changed old v3 driving or intact-world defaults")
			}
			ctx.Now = ctx.Now.Add(100 * time.Millisecond)
			advanced, err := mod.OnTimer(ctx, restored)
			if err != nil {
				t.Fatal(err)
			}
			var persisted map[string]any
			if json.Unmarshal(advanced.Data, &persisted) != nil || persisted["props"] == nil || view(t, mod, advanced, 0).Epoch != before.Epoch {
				t.Fatal("first ordinary timer did not freeze the legacy prop version without resetting the race")
			}
			otherMod, _, other := begin(t, 2, 1, track)
			if !bytes.Equal(props.Broken, classicPropsView(t, otherMod, other).Broken) {
				t.Fatal("independent new race inherited another prop state")
			}
			classicTrace(t, map[string]any{"passed": true, "scope": "new six-car catalog state and byte-preserving historical v3 Restore", "count": counts[track], "props": props, "legacyView": classicView(t, mod, restored, 0)})
		})
	}
}

func TestClassicPropSnapshotRejectsWrongLengthTailAndGeometry(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "uk")
	valid := classicPropsView(t, mod, m)
	france, _, other := begin(t, 2, 1, "france")
	wrongGeometry := classicPropsView(t, france, other)
	tail := make([]byte, len(valid.Broken))
	tail[len(tail)-1] = 0x80
	for _, bad := range []classicPropProjection{
		{Catalog: valid.Catalog, Broken: nil},
		{Catalog: valid.Catalog, Broken: make([]byte, len(valid.Broken)-1)},
		{Catalog: valid.Catalog, Broken: make([]byte, len(valid.Broken)+1)},
		{Catalog: valid.Catalog, Broken: tail},
		wrongGeometry,
	} {
		var data map[string]any
		if err := json.Unmarshal(m.Data, &data); err != nil {
			t.Fatal(err)
		}
		data["props"] = bad
		invalid := m
		invalid.Data = raw(data)
		if _, err := mod.Restore(ctx, invalid); err == nil {
			t.Fatal("inconsistent prop state was accepted", bad)
		}
	}
}

func TestClassicPropStateRejectsUnknownAndMalformedSnapshotMetadata(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "uk")
	for _, bad := range []map[string]any{
		{"catalog": strings.Repeat("0", 64), "broken": make([]byte, 14)},
		{"catalog": "missing", "broken": make([]byte, 14)},
		{"broken": []byte{1}},
		{"catalog": strings.Repeat("0", 64), "broken": "not valid base64"},
	} {
		var stored map[string]any
		if err := json.Unmarshal(m.Data, &stored); err != nil {
			t.Fatal(err)
		}
		stored["props"] = bad
		broken := m
		broken.Data = raw(stored)
		if _, err := mod.Restore(ctx, broken); err == nil {
			t.Fatalf("unknown or malformed prop state was silently accepted: %v", bad)
		}
	}
	// Legacy physics remains independent of any classic prop catalogue.
	legacyMod := game.Module()
	legacy, err := legacyMod.Start(ctx, raw(map[string]any{"track": "metro", "laps": 1}))
	if err != nil {
		t.Fatal(err)
	}
	v := classicView(t, legacyMod, legacy, 0)
	if v["props"] != nil {
		t.Fatal("new classic props leaked into physics=0")
	}
}

func classicPropBroken(p classicPropProjection, index int) bool {
	return index >= 0 && index/8 < len(p.Broken) && p.Broken[index/8]&(1<<uint(index%8)) != 0
}

func classicPropDrive(t *testing.T, mod gameapi.Module, ctx *gameapi.Context, m gameapi.Match, seat int, target game.Point, speed float64) gameapi.Match {
	t.Helper()
	v := view(t, mod, m, seat)
	c := v.Cars[seat]
	p := game.ClassicVehicle(c.Vehicle)
	dx, dy := (target.X-c.X)/10, (target.Y-c.Y)/10
	difference := math.Atan2(math.Sin(math.Atan2(dy, dx)-c.Angle), math.Cos(math.Atan2(dy, dx)-c.Angle))
	delta := math.Atan2(2*p.Wheelbase*math.Sin(difference), math.Max(4, math.Hypot(dx, dy)))
	steer := math.Max(-1, math.Min(1, delta/math.Atan(p.Wheelbase/p.TurningRadius)))
	actual := c.Speed / 10
	throttle := math.Max(0, math.Min(1, .18+(speed-actual)/4))
	pressure := math.Max(0, math.Min(1, (actual-speed-.5)/3))
	if pressure > 0 {
		throttle = 0
	}
	ctx.Actor = seat
	next, err := mod.Apply(*ctx, m, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": c.Seq + 1, "steer": steer, "throttle": throttle, "brake": pressure > 0, "brakeAmount": pressure, "drift": false, "horn": true}))
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func TestClassicPropRealContactAndPersistentWorld(t *testing.T) {
	// Frozen original France normal object index 1, s1/c0/o1. This target
	// point guides ordinary inputs; no car/snapshot coordinates are written.
	target := game.Point{X: 3204.8711373, Y: 1313.243409}
	for _, speed := range []float64{4, 13} {
		t.Run(map[bool]string{true: "low", false: "high"}[speed < 8], func(t *testing.T) {
			mod, ctx, m := begin(t, 2, 1, "france")
			initial := classicPropsView(t, mod, m)
			trajectory := []any{}
			for step := 0; step < 900 && !classicPropBroken(classicPropsView(t, mod, m), 1); step++ {
				ctx.Now = ctx.Now.Add(100 * time.Millisecond)
				m = classicPropDrive(t, mod, &ctx, m, 0, target, speed)
				ctx.Actor = -1
				var err error
				m, err = mod.OnTimer(ctx, m)
				if err != nil {
					t.Fatal(err)
				}
				if step%5 == 0 {
					trajectory = append(trajectory, classicView(t, mod, m, 0))
				}
			}
			broken := classicPropsView(t, mod, m)
			v := classicView(t, mod, m, 0)
			defer func() {
				classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "ordinary authenticated steering/throttle/brake toward a frozen original normal object; no position/event/result injection", "target": target, "requestedSpeedMS": speed, "initialProps": initial, "props": broken, "view": v, "trajectory": trajectory})
			}()
			if !classicPropBroken(broken, 1) {
				t.Fatal("ordinary drive could not break the original France prop", v["cars"])
			}
			var hit map[string]any
			for _, rawEvent := range v["events"].([]any) {
				e := rawEvent.(map[string]any)
				if e["type"] == "prop" && e["prop"] == float64(1) {
					if hit != nil {
						t.Fatal("one object emitted multiple first-hit events")
					}
					hit = e
				}
			}
			if hit == nil || (hit["high"] == true) != (speed >= 8) {
				t.Fatal("authoritative first-hit animation band missing or wrong", hit)
			}
			if heading, ok := hit["dir"].(float64); ok && (math.IsNaN(heading) || math.Abs(heading) > math.Pi+.001) {
				t.Fatal("invalid impact direction", hit)
			}
			if c := view(t, mod, m, 0).Cars[0]; c.Speed < 10 {
				t.Fatal("breakable prop still applied the static-wall stop", c)
			}
			restored, err := mod.Restore(ctx, m)
			if err != nil || !bytes.Equal(restored.Data, m.Data) || !bytes.Equal(raw(broken), raw(classicPropsView(t, mod, restored))) {
				t.Fatal("restore lost or rewrote persistent broken state", err)
			}
			ctx.Actor = 0
			m, err = mod.Reclaim(ctx, restored)
			if err != nil || !bytes.Equal(raw(broken), raw(classicPropsView(t, mod, m))) {
				t.Fatal("reclaim reset the broken world", err)
			}
			other, _, fresh := begin(t, 2, 1, "france")
			if classicPropBroken(classicPropsView(t, other, fresh), 1) {
				t.Fatal("broken state leaked through shared static track data")
			}
		})
	}
}

func TestClassicPropSixCarDrivingPacketBudget(t *testing.T) {
	mod := game.Module()
	ctx := gameapi.Context{Players: 6, Now: time.Unix(1794410000, 0)}
	m, err := mod.Start(ctx, raw(map[string]any{"track": "france", "laps": 1}))
	if err != nil {
		t.Fatal(err)
	}
	target := game.Point{X: 3204.8711373, Y: 1313.243409}
	largest, maxEvents, propEvents := 0, 0, map[int]bool{}
	for step := 0; step < 850; step++ {
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		v := view(t, mod, m, 0)
		for seat := range v.Cars {
			if step < 150 {
				m = classicPropDrive(t, mod, &ctx, m, seat, target, 13)
				continue
			}
			ctx.Actor = seat
			if step%55 == 0 && v.Phase != "countdown" && v.Cars[seat].Speed < 54.99 && ctx.Now.UnixMilli() >= v.Cars[seat].ResetUntil && (v.Cars[seat].LastReset == 0 || ctx.Now.UnixMilli()-v.Cars[seat].LastReset >= 5000) {
				var err error
				m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "reset", "epoch": v.Epoch}))
				if err != nil {
					t.Fatal(err)
				}
			}
			steer, throttle, pressure := .75, 1., 0.
			if seat%2 == 1 {
				steer = -steer
			}
			if step%70 >= 45 {
				steer, throttle, pressure = 0, 0, 1
			}
			var err error
			m, err = mod.Apply(ctx, m, raw(map[string]any{"type": "drive", "epoch": v.Epoch, "seq": v.Cars[seat].Seq + 1, "steer": steer, "throttle": throttle, "brake": pressure > 0, "brakeAmount": pressure, "horn": true}))
			if err != nil {
				t.Fatal(err)
			}
		}
		ctx.Actor = -1
		var err error
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
		wire := classicView(t, mod, m, 0)
		largest = max(largest, len(raw(wire)))
		events := wire["events"].([]any)
		maxEvents = max(maxEvents, len(events))
		for _, event := range events {
			e := event.(map[string]any)
			if e["type"] == "prop" {
				propEvents[int(e["id"].(float64))] = true
			}
		}
		if len(raw(wire)) > 8192 {
			t.Fatal("six cars with persistent props and full events exceeded 8KiB", len(raw(wire)))
		}
	}
	if maxEvents != 24 || len(propEvents) == 0 || !classicPropBroken(classicPropsView(t, mod, m), 1) {
		t.Fatal("packet workload did not include real prop contact and retained 24-event ring", maxEvents, propEvents)
	}
	classicTrace(t, map[string]any{"passed": true, "scope": "six manual production cars, native-object first contact, held horns, collisions and authorized low-speed resets; no state/event injection", "maximumBytes": largest, "maximumEvents": maxEvents, "distinctPropEvents": len(propEvents), "final": classicView(t, mod, m, 0)})
}

func TestClassicPropPredictionProductionTrace(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "france")
	target := game.Point{X: 3204.8711373, Y: 1313.243409}
	rows := []any{}
	brokenFrames := 0
	for step := 0; step < 900 && brokenFrames < 10; step++ {
		before := view(t, mod, m, 0)
		props := classicPropsView(t, mod, m)
		m = classicPropDrive(t, mod, &ctx, m, 0, target, 13)
		input := view(t, mod, m, 0).Cars[0]
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		ctx.Actor = -1
		var err error
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
		if before.Phase == "racing" {
			rows = append(rows, map[string]any{"before": before.Cars[0], "after": view(t, mod, m, 0).Cars[0], "input": map[string]any{"steer": input.Steer, "throttle": input.Throttle, "brake": input.Brake, "brakeAmount": input.BrakeAmount, "drift": input.Drift}, "propsBefore": props, "propsAfter": classicPropsView(t, mod, m), "ms": 100, "at": ctx.Now.UnixMilli()})
		}
		if classicPropBroken(classicPropsView(t, mod, m), 1) {
			brokenFrames++
		}
	}
	if brokenFrames != 10 {
		t.Fatal("prediction reference did not cross a real intact-to-broken transition")
	}
	out := filepath.Join("..", "..", "..", ".local/racing-nfs-backend/props-prediction")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "production ordinary-input trajectory before/through/after an original prop; no state/position/event injection", "rows": rows}, "", "  ")
	if err := os.WriteFile(filepath.Join(out, "production.json"), payload, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestClassicPropZeroIndexNormalDrive(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "uk")
	var track game.Track
	for _, candidate := range game.Tracks() {
		if candidate.ID == "uk" {
			track = candidate
		}
	}
	target := game.Point{X: 10545.2099597, Y: 9424.3591291}
	trajectory := []any{}
	for step := 0; step < 1500 && !classicPropBroken(classicPropsView(t, mod, m), 0); step++ {
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		car := view(t, mod, m, 0).Cars[0]
		index := car.RouteIndex
		goal := target
		if (25-index+len(track.Points))%len(track.Points) > 8 {
			goal = track.Points[(index+6)%len(track.Points)]
		}
		m = classicPropDrive(t, mod, &ctx, m, 0, goal, 13)
		ctx.Actor = -1
		var err error
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
		if step%10 == 0 {
			trajectory = append(trajectory, classicView(t, mod, m, 0))
		}
	}
	v := classicView(t, mod, m, 0)
	defer func() {
		classicTrace(t, map[string]any{"passed": !t.Failed(), "scope": "ordinary drive along original UK VROAD then toward normal prop0; no position/event injection", "view": v, "trajectory": trajectory})
	}()
	if !classicPropBroken(classicPropsView(t, mod, m), 0) {
		t.Fatal("ordinary UK drive failed to contact normal prop0")
	}
	for _, event := range v["events"].([]any) {
		e := event.(map[string]any)
		if e["type"] == "prop" && e["prop"] == float64(0) {
			return
		}
	}
	t.Fatal("first prop index zero was omitted from its authoritative event")
}
