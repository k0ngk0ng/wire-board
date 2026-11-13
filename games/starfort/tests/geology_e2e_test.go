package starfort_test

// Failure inventory and interpretation were frozen in GEOLOGY.md before code.
// Public Module/timer/serialized Match and real HTTP/SQLite, not private helpers.
import (
	"encoding/json"
	"fmt"
	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"strings"
	"testing"
	"time"
)

func geoRaw(s starfort.State) map[string]any {
	var v map[string]any
	_ = json.Unmarshal(rulesJSON(s), &v)
	return v
}
func geoPatch(s starfort.State, patch map[string]any) starfort.State {
	v := geoRaw(s)
	v["geologyRules"] = 1
	for k, x := range patch {
		v[k] = x
	}
	var result starfort.State
	_ = json.Unmarshal(rulesJSON(v), &result)
	return result
}
func geoBoundary(t *testing.T) *rulesTable {
	b := v3Boundary(t, "arrow", "heart", 4, "slime", "elite", "boss")
	s := cadenceState(b)
	s.Towers = nil
	s.TowerSerial = 0
	for i := range s.Enemies {
		s.Enemies[i].Speed = 40
		s.Enemies[i].WeatherCheckAt = 10000
		s.Enemies[i].HP = 1000
		s.Enemies[i].MaxHP = 1000
		s.Enemies[i].Shield = 0
		s.Enemies[i].Phase = 2
		s.Enemies[i].Progress = fixturePathLength(s.World.Map.Routes[0].Points)*.3 + float64(i)*8
		s.Enemies[i].X, s.Enemies[i].Y = fixturePathPosition(s.World.Map.Routes[0].Points, s.Enemies[i].Progress)
	}
	s = geoPatch(s, map[string]any{"geology": map[string]any{"wave": s.Wave, "startedMS": 0, "kind": "none", "triggerMS": 1000, "routePeaks": map[string]float64{}}})
	cadenceSave(b, s)
	return b
}
func geoConfig(b *rulesTable, kind string, zones []map[string]any) {
	s := cadenceState(b)
	s = geoPatch(s, map[string]any{"geology": map[string]any{"wave": s.Wave, "startedMS": 0, "kind": kind, "triggerMS": 200, "triggered": true, "zones": zones, "routePeaks": map[string]float64{}}})
	cadenceSave(b, s)
}
func geoZone(b *rulesTable, kind string) map[string]any {
	e := cadenceState(b).Enemies[0]
	return map[string]any{"id": 1, "kind": kind, "x": e.X, "y": e.Y, "radius": 150, "width": 400, "height": 200, "createdMS": 0, "expiresMS": 20000, "route": e.Route, "progress": e.Progress, "speed": 40}
}
func TestStarfortGeologyVersion(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{}, starfort.Module())
	if b.s.Schema != 11 || specializationRawView(b)["geologyRules"] != float64(1) {
		t.Fatal("new games require schema11 and public geology rule")
	}
	s := cadenceState(b)
	v := geoRaw(s)
	delete(v, "geologyRules")
	delete(v, "geology")
	_ = json.Unmarshal(rulesJSON(v), &s)
	// Unmarshal into a fresh state so absent fields cannot survive.
	s = starfort.State{}
	_ = json.Unmarshal(rulesJSON(v), &s)
	cadenceSave(b, s)
	if b.s.Schema != 10 {
		t.Fatal("old weather3 save migrated")
	}
	before := string(b.s.Data)
	b.view(0)
	b.validate()
	if before != string(b.s.Data) {
		t.Fatal("read changed archive")
	}
	bad := b.s
	bad.Schema = 11
	if _, err := b.m.View(bad, 0); err == nil {
		t.Fatal("schema mismatch accepted")
	}
}
func TestStarfortGeologyRewards(t *testing.T) {
	for _, tc := range []struct {
		peak  float64
		bonus int
	}{{.5, 10}, {.50001, 5}, {.7, 5}, {.70001, 0}, {1, 0}} {
		t.Run(fmt.Sprint(tc.peak), func(t *testing.T) {
			b := geoBoundary(t)
			s := cadenceState(b)
			s.Enemies = nil
			s.World.Objectives = nil
			s = geoPatch(s, map[string]any{"geology": map[string]any{"wave": s.Wave, "startedMS": 0, "kind": "none", "triggerMS": 1000, "routePeaks": map[string]float64{s.World.Map.Routes[0].ID: tc.peak}}})
			before := s.Players[0].Gold
			wave := s.Wave
			cadenceSave(b, s)
			b.timer(time.Second)
			after := cadenceState(b)
			want := 40 + wave*3 + tc.bonus*wave
			if after.Players[0].Gold-before != want {
				t.Fatal("exclusive reward", after.Players[0].Gold-before, want)
			}
			gold := after.Players[0].Gold
			b.timer(time.Second)
			if cadenceState(b).Players[0].Gold != gold {
				t.Fatal("duplicate reward")
			}
		})
	}
	b := geoBoundary(t)
	s := cadenceState(b)
	e := &s.Enemies[0]
	e.Progress = fixturePathLength(s.World.Map.Routes[0].Points) * .71
	e.X, e.Y = fixturePathPosition(s.World.Map.Routes[0].Points, e.Progress)
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	g := geoRaw(s)["geology"].(map[string]any)
	if g["routePeaks"].(map[string]any)[e.Route].(float64) < .71 {
		t.Fatal("history not tracked")
	}
}
func TestStarfortGeologyEarthquake(t *testing.T) {
	b := geoBoundary(t)
	zone := geoZone(b, "earthquake")
	geoConfig(b, "earthquake", []map[string]any{zone})
	before := cadenceState(b).Enemies[0].Progress
	b.timer(time.Second)
	weatherNear(t, "quake half movement", cadenceState(b).Enemies[0].Progress-before, 20)
	// Trigger quake over real pads: road-local half disabled, scheduled recovery and floor.
	s := cadenceState(b)
	for i := range s.Enemies {
		s.Enemies[i].HP = 1000000
		s.Enemies[i].MaxHP = 1000000
		s.Enemies[i].Speed = .001
	}
	s.TowerSerial = len(s.World.Map.Pads)
	s.Towers = nil
	for i, p := range s.World.Map.Pads {
		s.Towers = append(s.Towers, starfort.Tower{ID: i + 1, Pad: p.ID, Kind: "arrow", Level: 1, Owner: 0, Spent: 90, Priority: "first", Cooldown: 10000})
	}
	s = geoPatch(s, map[string]any{"geology": map[string]any{"wave": s.Wave, "startedMS": s.ElapsedMS, "kind": "earthquake", "triggerMS": s.ElapsedMS + 200, "routePeaks": map[string]float64{}}})
	cadenceSave(b, s)
	b.timer(time.Second)
	g := geoRaw(cadenceState(b))["geology"].(map[string]any)
	if !g["triggered"].(bool) {
		t.Fatal("quake not triggered")
	}
	for i := 0; i < 16; i++ {
		b.timer(time.Second)
	}
	g = geoRaw(cadenceState(b))["geology"].(map[string]any)
	if g["recoveries"] != float64(3) {
		t.Fatal("three recovery deadlines", g)
	}
}
func TestStarfortGeologyFlood(t *testing.T) {
	b := geoBoundary(t)
	z := geoZone(b, "flood")
	ss := cadenceState(b)
	ss.Tick = 20
	ss.ElapsedMS = 4000
	cadenceSave(b, ss)
	z["y"] = cadenceState(b).Enemies[0].Y - 20
	geoConfig(b, "flood", []map[string]any{z})
	b.timer(time.Second)
	for i, e := range cadenceState(b).Enemies {
		want := 900.
		if i > 0 {
			want = 950
		}
		weatherNear(t, "max HP flood/half heavy", e.HP, want)
	}
	b.timer(time.Second)
	if cadenceState(b).Enemies[0].HP != 900 {
		t.Fatal("same flood hit twice")
	}
}
func TestStarfortGeologyTyphoon(t *testing.T) {
	b := geoBoundary(t)
	z := geoZone(b, "typhoon")
	z["radius"] = 75
	geoConfig(b, "typhoon", []map[string]any{z})
	before := cadenceState(b).Enemies[0].Progress
	b.timer(time.Second)
	s := cadenceState(b)
	weatherNear(t, "wind stops movement", s.Enemies[0].Progress, before)
	weatherNear(t, "current HP wind", s.Enemies[0].HP, 900)
	b.timer(time.Second)
	weatherNear(t, "one wind one HP hit", cadenceState(b).Enemies[0].HP, 900)
}
func TestStarfortGeologyFreezeDirection(t *testing.T) {
	b := v3Boundary(t, "frost", "freeze", 100, "slime", "elite", "boss")
	s := cadenceState(b)
	s = geoPatch(s, nil)
	s.Enemies[0].ConfusedUntil = 100
	s.Enemies[1].SleepingUntil = 100
	s.Enemies[2].Speed = 40
	for i := range s.Enemies {
		s.Enemies[i].WeatherCheckAt = 10000
	}
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	if s.Enemies[0].FrozenUntil != 0 || s.Enemies[1].FrozenUntil != 0 || s.Enemies[2].FrozenUntil == 0 {
		t.Fatal("freeze must only target forward motion", s.Enemies)
	}
}
func TestStarfortGeologyLotteryAndGeometry(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, starfort.Module())
	counts := map[string]int{}
	for wave := 1; wave <= 300; wave++ {
		s := cadenceState(b)
		s.Wave = wave
		s.CompletedWaves = wave - 1
		s.Phase = "prepare"
		s.World.Objectives = nil
		s.World.Choice = nil
		s.World.BoonChoice = nil
		cadenceSave(b, s)
		b.action(0, map[string]any{"type": "ready"}, true)
		s = cadenceState(b)
		g, ok := geoRaw(s)["geology"].(map[string]any)
		if !ok {
			t.Fatal("wave plan missing")
		}
		kind := g["kind"].(string)
		counts[kind]++
		before := string(b.s.Data)
		b.view(0)
		b.validate()
		if string(b.s.Data) != before {
			t.Fatal("view rerolled")
		}
		zones, _ := g["zones"].([]any)
		if kind == "earthquake" || kind == "flood" {
			if len(zones) < 1 || len(zones) > map[string]int{"earthquake": 4, "flood": 5}[kind] {
				t.Fatal("zone count", kind, zones)
			}
		}

		// The shared 30% selection must become visible even when each spawn is
		// killed immediately. A wave cannot clear before its final spawn.
		interval := 800
		if s.WaveInfo.Rule == "swarm" {
			interval = 400
		}
		earliestClear := float64(max(1000, (s.WaveInfo.Total-1)*interval))
		if len(zones) > 0 {
			first := zones[0].(map[string]any)["createdMS"].(float64) - float64(s.ElapsedMS)
			if first > earliestClear {
				t.Fatal("selected event scheduled after earliest clear", kind, first, earliestClear)
			}
		}
		if kind == "flood" && len(zones) > 1 {
			direction := zones[0].(map[string]any)["fromBottom"]
			for _, z := range zones[1:] {
				if z.(map[string]any)["fromBottom"] != direction {
					t.Fatal("flood event changed ingress direction")
				}
			}
		}
		for i, raw := range zones {
			z := raw.(map[string]any)
			created := z["createdMS"].(float64) - float64(s.ElapsedMS)
			if created < 1000 || created > 30000 || (kind == "earthquake" && created > 10000) {
				t.Fatal("spawn schedule", z)
			}
			for j := 0; j < i; j++ {
				q := zones[j].(map[string]any)
				if kind == "earthquake" && math.Hypot(z["x"].(float64)-q["x"].(float64), z["y"].(float64)-q["y"].(float64)) < z["radius"].(float64)+q["radius"].(float64) {
					t.Fatal("overlapping quake")
				}
				if kind == "flood" && z["x"].(float64) < q["x"].(float64)+q["width"].(float64) && z["x"].(float64)+z["width"].(float64) > q["x"].(float64) {
					t.Fatal("overlapping flood sweep")
				}
			}
			if kind == "flood" {
				base := s.World.Map.Base
				dx := math.Max(z["x"].(float64), math.Min(base.X, z["x"].(float64)+z["width"].(float64))) - base.X
				dy := math.Max(z["y"].(float64), math.Min(base.Y, z["y"].(float64)+z["height"].(float64))) - base.Y
				if math.Hypot(dx, dy) < 60 {
					t.Fatal("flood at core")
				}
			}
			if kind == "typhoon" {
				ratio := z["progress"].(float64) / fixturePathLength(s.World.Map.Routes[0].Points)
				if ratio < .1 || ratio > .4 || len(zones) > 16 {
					t.Fatal("wind geometry", ratio)
				}
			}
		}
		s.Phase = "prepare"

		cadenceSave(b, s)
	}
	if counts["none"] < 180 || counts["none"] > 240 || len(counts) != 4 {
		t.Fatal("shared 30% three-kind lottery", counts)
	}
	t.Log(counts)
}
func TestStarfortGeologyEscortAndHTTP(t *testing.T) {
	m := starfort.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		v, err := start(ctx, raw)
		if err != nil {
			return v, err
		}
		var s starfort.State
		_ = json.Unmarshal(v.Data, &s)
		s.NextAt = ctx.Now.Add(5 * time.Minute).UnixMilli()
		return s.Envelope()
	}
	tb := newHost(t, 2, m)
	tb.start(nil)
	a := tb.people[0]
	if data(a.room())["geologyRules"] != float64(1) {
		t.Fatal("new HTTP rule missing")
	}
	before := data(a.room())["world"]
	tb.restart()
	if string(rulesJSON(before)) != string(rulesJSON(data(a.room())["world"])) {
		t.Fatal("restart changed plan")
	}
	tb.record("schema11 new rule and world preserved across SQLite restart")
	b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, starfort.Module())
	seen := map[string]bool{}
	for wave := 1; wave < 200; wave++ {
		s := cadenceState(b)
		s.World.Objectives = nil
		if err := fixtureClearedWave(b.context(0), &s, wave, true); err != nil {
			t.Fatal(err)
		}
		for _, o := range s.World.Objectives {
			if o.Kind == "convoy" {
				length := 0.
				for _, r := range s.World.Map.Routes {
					if r.ID == o.Route {
						length = fixturePathLength(r.Points)
					}
				}
				ratio := o.Progress / length
				if ratio < .4 || ratio > .5 {
					t.Fatal("convoy spawn", ratio)
				}
				seen[o.Kind] = true
			}
			if o.Kind == "merchant" {
				seen[o.Kind] = true
			}
		}
		cadenceSave(b, s)
	}
	if len(seen) != 2 {
		t.Fatal("escort fixtures missing")
	}

}

func TestStarfortGeologyFloodTowerRecoveryAndEscortDamage(t *testing.T) {
	b := geoBoundary(t)
	s := cadenceState(b)
	s.Tick = 20
	s.ElapsedMS = 4000
	s.TowerSerial = 1
	s.Towers = []starfort.Tower{{ID: 1, Pad: 0, Kind: "arrow", Level: 1, Owner: 0, Spent: 90, Priority: "first", Cooldown: 10000}}
	e := s.Enemies[0]
	s.World.Objectives = []starfort.Objective{{ID: "escort", Kind: "merchant", Label: "商人夹具", Route: e.Route, Progress: e.Progress, X: e.X, Y: e.Y, HP: 10, MaxHP: 10, Status: "active", Owner: 0, Sector: 1, StartWave: s.Wave, Cost: 200}}
	cadenceSave(b, s)
	base := b.view(0).Towers[0].EffectiveDamage
	pad := s.World.Map.Pads[0]
	z := geoZone(b, "flood")
	z["x"] = pad.X - 150
	z["y"] = pad.Y - 100
	z["width"] = 400
	z["height"] = 300
	geoConfig(b, "flood", []map[string]any{z})
	b.timer(time.Second)
	damage := b.view(0).Towers[0].EffectiveDamage
	if damage >= base || damage < base*.8 {
		t.Fatal("flood weakening", damage, base)
	}
	for i := 0; i < 10; i++ {
		b.timer(time.Second)
	}
	weatherNear(t, "linear recovery completes", b.view(0).Towers[0].EffectiveDamage, base)
	// Separate escort centered in flood, no enemy contact; merchant travels 15/s.
	b = geoBoundary(t)
	s = cadenceState(b)
	s.Tick = 20
	s.ElapsedMS = 4000
	e = s.Enemies[0]
	s.Enemies = nil
	s.World.Objectives = []starfort.Objective{{ID: "escort", Kind: "merchant", Label: "商人夹具", Route: e.Route, Progress: e.Progress, X: e.X, Y: e.Y, HP: 10, MaxHP: 10, Status: "active", Owner: 0, Sector: 1, StartWave: s.Wave, Cost: 200}}
	cadenceSave(b, s)
	z = map[string]any{"id": 1, "kind": "flood", "x": e.X - 100, "y": e.Y - 50, "width": 200, "height": 100, "createdMS": 0, "expiresMS": 20000}
	geoConfig(b, "flood", []map[string]any{z})
	b.timer(time.Second)
	o := cadenceState(b).World.Objectives[0]
	weatherNear(t, "escort max HP damage", o.HP, 9)
	weatherNear(t, "merchant15", o.Progress-e.Progress, 15)
}
func TestStarfortGeologyInvalidSnapshots(t *testing.T) {
	b := geoBoundary(t)
	base := geoRaw(cadenceState(b))
	for _, patch := range []map[string]any{{"geologyRules": 2}, {"geology": map[string]any{"wave": 1, "kind": "lava"}}, {"geology": map[string]any{"wave": 1, "kind": "typhoon", "zones": []any{map[string]any{"id": 1, "kind": "typhoon", "radius": 10000}}}}} {
		v := map[string]any{}
		_ = json.Unmarshal(rulesJSON(base), &v)
		for k, x := range patch {
			v[k] = x
		}
		m := b.s
		m.Data = rulesJSON(v)
		if _, err := b.m.View(m, 0); err == nil {
			t.Fatal("invalid geology accepted", patch)
		}
	}
}
func TestStarfortGeologyNaturalCampaigns(t *testing.T) {
	for _, tc := range []struct {
		n, waves   int
		difficulty string
	}{{1, 10, "normal"}, {2, 20, "normal"}, {4, 10, "hard"}} {
		t.Run(fmt.Sprintf("%dp-%dw-%s", tc.n, tc.waves, tc.difficulty), func(t *testing.T) {
			b := newModuleRulesTable(t, tc.n, map[string]any{"waves": tc.waves, "difficulty": tc.difficulty}, starfort.Module())
			for i := 0; i < 18000 && b.s.Result == nil; i++ {
				b.botPass()
				b.timer(time.Second)
			}
			if b.s.Result == nil || b.view(0).Phase != "won" {
				t.Fatal("natural new-rule campaign failed", b.view(0).Phase, b.view(0).Wave)
			}
			b.record(map[string]any{"natural": true, "result": b.s.Result})
		})
	}
}
func TestStarfortGeologyEarthquakeDamageAndDisable(t *testing.T) {
	b := v3Boundary(t, "arrow", "heart", 4, "slime")
	s := cadenceState(b)
	s = geoPatch(s, nil)
	s.Enemies[0].WeatherCheckAt = 10000
	pad := s.World.Map.Pads[s.Towers[0].Pad]
	e := s.Enemies[0]
	base := b.view(0).Towers[0].EffectiveDamage
	z := map[string]any{"id": 1, "kind": "earthquake", "x": e.X, "y": e.Y, "radius": 75, "createdMS": 0}
	s = geoPatch(s, map[string]any{"geology": map[string]any{"wave": s.Wave, "kind": "earthquake", "triggered": true, "triggerMS": 0, "zones": []any{z}, "routePeaks": map[string]float64{}}})
	s.Towers[0].Branch = ""
	s.Towers[0].Level = 1
	s.SkillRNG = 5
	cadenceSave(b, s)
	base = b.view(0).Towers[0].EffectiveDamage
	b.timer(time.Second)
	weatherNear(t, "quake extra tower damage", 1000000-cadenceState(b).Enemies[0].HP, base*1.2)
	// Quake triggers over one real tower. With one affected tower the 10% floor retains it.
	s = cadenceState(b)
	s.Towers[0].Cooldown = 0
	s.Enemies[0].Speed = .001
	z = map[string]any{"id": 1, "kind": "earthquake", "x": pad.X, "y": pad.Y, "radius": 75, "createdMS": s.ElapsedMS + 200}
	s = geoPatch(s, map[string]any{"geology": map[string]any{"wave": s.Wave, "startedMS": s.ElapsedMS, "kind": "earthquake", "triggerMS": s.ElapsedMS + 200, "zones": []any{z}, "routePeaks": map[string]float64{}}})
	cadenceSave(b, s)
	b.timer(time.Second)
	hp := cadenceState(b).Enemies[0].HP
	for i := 0; i < 16; i++ {
		b.timer(time.Second)
	}
	if cadenceState(b).Enemies[0].HP != hp {
		t.Fatal("floor disabled tower fired")
	}
	if !strings.Contains(string(rulesJSON(specializationRawView(b))), "disabled") {
		t.Fatal("disabled missing in projection")
	}
}

// Additional public-boundary checks cover independent per-wind protection,
// simultaneous route budgets, and persisted mid-wave state.
func TestStarfortGeologyTyphoonBudgetAndProtection(t *testing.T) {
	b := geoBoundary(t)
	s := cadenceState(b)
	r := s.World.Map.Routes[0]
	length := fixturePathLength(r.Points)
	d := length * .2
	x, y := fixturePathPosition(r.Points, d)
	var zones []map[string]any
	for i := 0; i < 16; i++ {
		zones = append(zones, map[string]any{"id": i + 1, "kind": "typhoon", "route": r.ID, "progress": d, "x": x, "y": y, "radius": 75, "speed": 40, "createdMS": 200})
	}
	geoConfig(b, "typhoon", zones)
	b.timer(time.Second)
	g := geoRaw(cadenceState(b))["geology"].(map[string]any)
	active, skipped := 0, 0
	for _, raw := range g["zones"].([]any) {
		z := raw.(map[string]any)
		if z["activated"] == true {
			active++
		}
		if z["skipped"] == true {
			skipped++
		}
	}
	if active != 3 || skipped != 13 {
		t.Fatal("single route budget", active, skipped)
	}
	// Use a legitimate current-map pad within the largest wind circle.
	b = newModuleRulesTable(t, 1, map[string]any{}, starfort.Module())
	s = cadenceState(b)
	r = s.World.Map.Routes[0]
	best := math.Inf(1)
	pad, progress := 0, 0.
	for _, p := range s.World.Map.Pads {
		for dd := 0.; dd < fixturePathLength(r.Points); dd++ {
			xx, yy := fixturePathPosition(r.Points, dd)
			dist := math.Hypot(p.X-xx, p.Y-yy)
			if dist < best {
				best, pad, progress = dist, p.ID, dd
			}
		}
	}
	if best > 74 {
		t.Fatal("no legal pad in typhoon range", best)
	}
	x, y = fixturePathPosition(r.Points, progress)
	s.Phase = "battle"
	s.NextAt = 0
	s.World.Event.ID = "calm"
	s.World.Objectives = nil
	s.WaveInfo = starfort.WaveInfo{Total: 1, Spawned: 1}
	s.EnemySerial = 1
	s.Enemies = []starfort.Enemy{{ID: 1, Kind: "slime", Route: r.ID, Progress: progress, X: x, Y: y, HP: 1000000, MaxHP: 1000000, Speed: .001, Leak: 1}}
	s.TowerSerial = 1
	s.Towers = []starfort.Tower{{ID: 1, Pad: pad, Kind: "arrow", Level: 1, Owner: 0, Spent: 90, Priority: "first"}}
	s = geoPatch(s, map[string]any{"geoRng": 5, "geology": map[string]any{"wave": s.Wave, "kind": "typhoon", "triggered": true, "zones": []any{map[string]any{"id": 1, "kind": "typhoon", "route": r.ID, "progress": progress, "x": x, "y": y, "radius": 75, "speed": 40, "createdMS": 0, "activated": true}}, "routePeaks": map[string]float64{}}})
	cadenceSave(b, s)
	for i := 0; i < 4; i++ {
		b.timer(time.Second)
	}
	s = cadenceState(b)
	last := int64(-1000)
	misses := 0
	for _, e := range s.Events {
		if e.Type == "miss" && e.Skill == "typhoon" {
			if e.Tick-last < 25 {
				t.Fatal("same wind bypassed protection")
			}
			last = e.Tick
			misses++
		}
	}
	if misses == 0 {
		t.Fatal("wind miss not exercised")
	}
	before := string(b.s.Data)
	b.view(0)
	b.validate()
	if before != string(b.s.Data) {
		t.Fatal("view lost guards")
	}
}

func TestStarfortGeologyMidWaveHTTP(t *testing.T) {
	m := starfort.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		v, err := start(ctx, raw)
		if err != nil {
			return v, err
		}
		var s starfort.State
		_ = json.Unmarshal(v.Data, &s)
		s.Phase = "paused"
		s.NextAt = 0
		s.Tick = 25
		s.ElapsedMS = 5000
		s.TowerSerial = 1
		s.Towers = []starfort.Tower{{ID: 1, Pad: 0, Kind: "arrow", Level: 1, Owner: 0, Spent: 90, Priority: "first"}}
		r := s.World.Map.Routes[0]
		s = geoPatch(s, map[string]any{"geoRng": 123, "geology": map[string]any{"wave": s.Wave, "kind": "earthquake", "triggerMS": 1000, "triggered": true, "recoveries": 0, "disabled": []any{map[string]any{"towerId": 1, "route": r.ID, "floor": 1}}, "zones": []any{map[string]any{"id": 1, "kind": "earthquake", "x": 200, "y": 200, "radius": 100, "createdMS": 1000}}, "routePeaks": map[string]float64{r.ID: .61}}})
		return s.Envelope()
	}
	tb := newHost(t, 2, m)
	tb.start(nil)
	a := tb.people[0]
	before := data(a.room())["geology"]
	tb.restart()
	after := data(a.room())
	if string(rulesJSON(before)) != string(rulesJSON(after["geology"])) || after["phase"] != "paused" {
		t.Fatal("mid-wave persisted geology changed")
	}
	tb.record("SQLite restart preserves active quake, disabled tower, recovery deadline and historical peak")
}
