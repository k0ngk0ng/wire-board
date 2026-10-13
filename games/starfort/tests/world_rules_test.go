package starfort_test

// Failures enumerated before implementation: generated layouts never differ;
// expansions move existing towers or pads, reuse IDs, erase upgrades, or occur
// mid-wave; a late vote mutates a resolved choice; tied/unanswered votes stall;
// timer launches without preview/build time; pause loses votes; observers act;
// schema-1 saves teleport live enemies; event/route rotations never happen;
// old saves fail on reconnect; endless maps/entities grow without bounds;
// terrain/boon statistics are cosmetic; objectives charge twice or fail silently;
// natural 1/2/4-player campaigns and 40-wave endless cannot use new strategies.
// The tests drive public Module callbacks and retain reproducible action traces.

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type worldPoint struct{ X, Y float64 }
type worldPad struct {
	ID      int
	X, Y    float64
	Terrain string
	Sector  int
}
type worldMap struct {
	Width, Height int
	Base          worldPoint
	Path          []worldPoint
	Pads          []worldPad
	Routes        []struct {
		ID     string
		Points []worldPoint
		Active bool
	}
	Regions  []json.RawMessage
	Features []struct {
		ID, Kind            string
		X, Y, Width, Height float64
		Sector              int
	}
}
type worldChoiceView struct {
	Kind     string
	Stage    int
	Deadline int64
	Votes    []string
	Options  []struct {
		ID, Label, Description string
		Map                    worldMap
	}
}
type worldView struct {
	rulesView
	Map   worldMap `json:"map"`
	World struct {
		Seed       int64
		Stage      int
		Map        worldMap
		Choice     *worldChoiceView
		BoonChoice *worldChoiceView
		Branches   []string
		Event      struct{ ID string }
		Boons      map[string]int
		Objectives []struct {
			ID, Kind, Status      string
			Cost, Reward, EndWave int
		}
	} `json:"world"`
}

func worldRead(t *testing.T, b *rulesTable) worldView {
	t.Helper()
	x, e := b.m.View(b.s, 0)
	if e != nil {
		t.Fatal(e)
	}
	var v worldView
	if e = json.Unmarshal(rulesJSON(x), &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func worldArtifact(t *testing.T, b *rulesTable, data any) {
	t.Helper()
	t.Cleanup(func() {
		dir := filepath.Join("../../../.local/starfort-world/rules", t.Name())
		if e := os.MkdirAll(dir, 0755); e != nil {
			t.Error(e)
			return
		}
		if e := os.WriteFile(filepath.Join(dir, "summary.json"), rulesJSON(map[string]any{"passed": !t.Failed(), "details": data, "trace": b.dir + "/replay.jsonl", "command": ". scripts/env.sh && go test ./games/starfort/tests -run '^" + t.Name() + "$' -count=1 -v"}), 0644); e != nil {
			t.Error(e)
		}
	})
}
func worldUntil(t *testing.T, b *rulesTable, wave int) {
	t.Helper()
	for sec := 0; sec < 14000 && b.view(0).Wave < wave && b.s.Result == nil; sec++ {
		b.botPass()
		b.timer(time.Second)
	}
	if b.view(0).Wave < wave {
		t.Fatalf("failed to reach wave %d: %+v", wave, b.view(0))
	}
}
func worldGeometry(t *testing.T, m worldMap) {
	t.Helper()
	for _, pad := range m.Pads {

		nearest := math.Inf(1)
		for _, route := range m.Routes {
			for i := 1; i < len(route.Points); i++ {
				a, b := route.Points[i-1], route.Points[i]
				dx, dy := b.X-a.X, b.Y-a.Y
				ratio := 0.
				if dx*dx+dy*dy > 0 {
					ratio = math.Max(0, math.Min(1, ((pad.X-a.X)*dx+(pad.Y-a.Y)*dy)/(dx*dx+dy*dy)))
				}
				nearest = math.Min(nearest, math.Hypot(pad.X-a.X-ratio*dx, pad.Y-a.Y-ratio*dy))
			}
		}
		if nearest < 36 || nearest > 230 {
			t.Fatalf("pad %d sector%d unusable distance %.1f", pad.ID, pad.Sector, nearest)
		}
	}
	for _, f := range m.Features {
		if f.X < 0 || f.Y < 0 || f.X > float64(m.Width) || f.Y > float64(m.Height) {
			t.Fatalf("feature %s outofbounds (%f,%f)", f.ID, f.X, f.Y)
		}
	}

	for _, route := range m.Routes {
		for i, p := range route.Points {
			if p.X > float64(m.Width) || p.Y > float64(m.Height) || (i > 0 && (p.X < 0 || p.Y < 0)) {
				t.Fatalf("route %s vertex%d outside map: %+v", route.ID, i, p)
			}
		}
	}
	for _, river := range m.Features {
		if river.Kind != "river" {
			continue
		}
		for _, route := range m.Routes {
			for i := 1; i < len(route.Points); i++ {
				a, b := route.Points[i-1], route.Points[i]
				if a.X == b.X || river.X < math.Min(a.X, b.X) || river.X > math.Max(a.X, b.X) {
					continue
				}
				y := a.Y + (b.Y-a.Y)*(river.X-a.X)/(b.X-a.X)
				if y < river.Y-river.Height/2 || y > river.Y+river.Height/2 {
					continue
				}
				bridge := false
				for _, f := range m.Features {
					if f.Kind == "bridge" && math.Hypot(f.X-river.X, f.Y-y) < 2 {
						bridge = true
					}
				}
				if !bridge {
					t.Fatalf("route %s crosses %s unbridged at %.1f %.1f", route.ID, river.ID, river.X, y)
				}
			}
		}
	}

}
func TestStarfortWorldNaturalCampaigns(t *testing.T) {
	for _, n := range []int{1, 2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			b := newRulesTable(t, n, map[string]any{"waves": 20})
			stages := map[int]bool{}
			fired := map[int]bool{}
			events := map[string]bool{}
			original := worldRead(t, b).Map.Pads
			previous := map[int]rulesTower{}
			for sec := 0; sec < 16000 && b.s.Result == nil; sec++ {
				v := worldRead(t, b)
				if !stages[v.World.Stage] {
					worldGeometry(t, v.Map)
				}
				stages[v.World.Stage] = true
				events[v.World.Event.ID] = true
				for _, raw := range v.Events {
					var e struct {
						Type string
						X, Y float64
					}
					_ = json.Unmarshal(raw, &e)
					if e.Type == "shot" {
						for _, pad := range v.Map.Pads {
							if pad.X == e.X && pad.Y == e.Y {
								fired[pad.Sector] = true
							}
						}
					}
				}
				if len(v.Map.Pads) < len(original) || !reflect.DeepEqual(v.Map.Pads[:len(original)], original) {
					t.Fatal("expansion moved original pads")
				}
				for _, tw := range v.Towers {
					if old, ok := previous[tw.ID]; ok && (old.Pad != tw.Pad || old.Owner != tw.Owner || tw.Level < old.Level) {
						t.Fatal("expansion reset tower")
					}
					previous[tw.ID] = tw
				}
				if v.World.Stage < 1 || v.World.Stage > 4 || len(v.Map.Regions) > 4 || len(v.Map.Pads) > 40 {
					t.Fatal("unbounded map")
				}
				b.botPass()
				b.timer(time.Second)
			}
			v := worldRead(t, b)
			worldArtifact(t, b, map[string]any{"view": v, "stages": stages, "events": events, "firedSectors": fired})
			if v.Phase != "won" || v.CompletedWaves != 20 {
				t.Fatalf("campaign %d failed wave %d hp %d", n, v.Wave, v.BaseHP)
			}
			if len(stages) != 4 || len(events) < 4 {
				t.Fatalf("missing growth/events: %v %v", stages, events)
			}
			covered := map[int]bool{}
			for _, tower := range v.Towers {
				covered[v.Map.Pads[tower.Pad].Sector] = true
			}
			if len(fired) < 4 {
				t.Fatalf("expanded defenses never fired: %v", fired)
			}
			if len(covered) < 4 {
				t.Fatalf("bots never defended expanded sectors: %v", covered)
			}
			if v.Map.Width*v.Map.Height < 960*640*3 {
				t.Fatal("final map did not triple")
			}
		})
	}
}
func TestStarfortWorldEndlessForty(t *testing.T) {
	b := newRulesTable(t, 2, map[string]any{"mode": "endless"})
	routeSets := map[string]bool{}
	for sec := 0; sec < 30000 && b.view(0).CompletedWaves < 40 && b.s.Result == nil; sec++ {
		v := worldRead(t, b)
		if v.Wave > 20 {
			ids := []string{}
			for _, r := range v.Map.Routes {
				if r.Active {
					ids = append(ids, r.ID)
				}
			}
			routeSets[string(rulesJSON(ids))] = true
		}
		b.botPass()
		b.timer(time.Second)
	}
	v := worldRead(t, b)
	worldArtifact(t, b, map[string]any{"view": v, "routeSets": routeSets})
	if v.CompletedWaves < 40 || b.s.Result != nil {
		t.Fatalf("endless stopped at wave %d hp %d", v.Wave, v.BaseHP)
	}
	if v.World.Stage != 4 || len(v.Map.Pads) > 40 || len(v.Map.Routes) > 8 || len(routeSets) < 2 {
		t.Fatalf("unbounded or static endless: stages=%d pads=%d routes=%d rotation=%v", v.World.Stage, len(v.Map.Pads), len(v.Map.Routes), routeSets)
	}
}
func TestStarfortWorldVotingAndMigration(t *testing.T) {
	b := newRulesTable(t, 2, map[string]any{"waves": 20})
	worldUntil(t, b, 6)
	v := worldRead(t, b)
	if v.World.Choice == nil || v.World.BoonChoice == nil {
		t.Fatal("wave six must preview route and boon")
	}
	before := b.s
	choice := v.World.Choice
	if len(choice.Options) != 2 || reflect.DeepEqual(choice.Options[0].Map, choice.Options[1].Map) {
		t.Fatal("branch maps identical")
	}
	b.action(-1, map[string]any{"type": "route_vote", "choice": choice.Options[0].ID}, false)
	b.action(0, map[string]any{"type": "ready"}, false)
	b.action(0, map[string]any{"type": "route_vote", "choice": "unknown"}, false)
	b.action(0, map[string]any{"type": "route_vote", "choice": choice.Options[0].ID}, true)
	b.action(1, map[string]any{"type": "route_vote", "choice": choice.Options[1].ID}, true)
	v = worldRead(t, b)
	if v.World.Choice != nil || v.World.Stage != 2 || v.World.Branches[0] != choice.Options[0].ID {
		t.Fatal("tie did not resolve deterministically")
	}
	b.action(1, map[string]any{"type": "route_vote", "choice": choice.Options[1].ID}, false)
	firstMap := v.Map
	b.s = before
	for p := 0; p < 2; p++ {
		b.action(p, map[string]any{"type": "route_vote", "choice": choice.Options[1].ID}, true)
	}
	if reflect.DeepEqual(firstMap, worldRead(t, b).Map) {
		t.Fatal("votes do not affect routes")
	}
	b.timer(time.Minute)
	v = worldRead(t, b)
	if v.Phase != "prepare" || v.World.BoonChoice != nil || v.NextAt <= b.now.UnixMilli() {
		t.Fatal("timeout skipped construction grace")
	}
	worldArtifact(t, b, v)
	// Public migration boundary: original schema-one layout and live enemy are
	// frozen while existing wave runs, then normal wave-boundary preview resumes.
	old := newRulesTable(t, 1, map[string]any{})
	old.allReady()
	old.timer(time.Second)
	var data map[string]json.RawMessage
	_ = json.Unmarshal(old.s.Data, &data)
	delete(data, "world")
	var enemies []map[string]any
	_ = json.Unmarshal(data["enemies"], &enemies)
	for _, e := range enemies {
		delete(e, "route")
		delete(e, "phase")
		delete(e, "abilityTick")
	}
	data["enemies"] = rulesJSON(enemies)
	old.s.Schema = 1
	old.s.Data = rulesJSON(data)
	saved := old.s
	view, err := old.m.View(saved, 0)
	if err != nil {
		t.Fatal(err)
	}
	var migrated worldView
	_ = json.Unmarshal(rulesJSON(view), &migrated)
	if len(migrated.Map.Routes) != 1 || migrated.World.Stage != 1 {
		t.Fatal("legacy midwave layout expanded")
	}
	for i := range migrated.Enemies {
		var e map[string]any
		_ = json.Unmarshal(migrated.Enemies[i], &e)
		if e["x"] != enemies[i]["x"] || e["y"] != enemies[i]["y"] {
			t.Fatal("migration teleported enemy")
		}
	}
	old.timer(time.Second)
	if old.s.Schema != 2 {
		t.Fatal("migration not persisted")
	}
}
func TestStarfortWorldSeedAndTargetAuthority(t *testing.T) {
	b := newRulesTable(t, 2, map[string]any{})
	v := worldRead(t, b)
	if v.World.Seed == 0 || len(v.Map.Routes) != 1 {
		t.Fatal("missing seeded map")
	}
	ctx := b.context(-1)
	ctx.Now = ctx.Now.Add(17 * time.Second)
	other, err := b.m.Start(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	saved := b.s
	b.s = other
	different := worldRead(t, b)
	b.s = saved
	if reflect.DeepEqual(v.Map.Path, different.Map.Path) {
		t.Fatal("different match seed repeats initial layout")
	}
	b.action(0, map[string]any{"type": "build", "pad": 1, "tower": "arrow"}, true)
	id := b.view(0).Towers[0].ID
	b.action(1, map[string]any{"type": "target", "towerId": id, "priority": "strong"}, false)
	b.action(0, map[string]any{"type": "target", "towerId": id, "priority": "unknown"}, false)
	for _, priority := range []string{"strong", "support", "first"} {
		b.action(0, map[string]any{"type": "target", "towerId": id, "priority": priority}, true)
	}
	worldArtifact(t, b, worldRead(t, b))
}

// Branch/seed failures: a bend leaves the map, towers sit on roads or cannot
// cover them, and a later road crosses a previously promised building pad.
func TestStarfortWorldGeometrySeeds(t *testing.T) {
	seeds := []int64{1531756199}
	for seed := int64(1); seed <= 64; seed++ {
		seeds = append(seeds, seed)
	}
	for _, seed := range seeds {
		for branchBits := 0; branchBits < 8; branchBits++ {
			t.Run(fmt.Sprintf("%d-%d", seed, branchBits), func(t *testing.T) {
				b := newRulesTable(t, 1, map[string]any{"waves": 20})
				ctx := b.context(-1)
				ctx.Now = time.Unix(0, seed)
				initial, err := b.m.Start(ctx, rulesJSON(map[string]any{"waves": 20}))
				if err != nil {
					t.Fatal(err)
				}
				b.s = initial
				worldGeometry(t, worldRead(t, b).Map)
				for stage := 2; stage <= 4; stage++ {
					var raw map[string]any
					_ = json.Unmarshal(b.s.Data, &raw)
					world := raw["world"].(map[string]any)
					world["seed"] = seed
					raw["phase"] = "battle"
					raw["wave"] = (stage - 1) * 5
					raw["enemies"] = []any{}
					info := raw["waveInfo"].(map[string]any)
					info["spawned"] = info["total"]
					b.s.Data = rulesJSON(raw)
					b.s.SystemDeadline = b.now.Add(time.Second).UnixMilli()
					b.timer(time.Second)
					v := worldRead(t, b)
					if v.World.Choice == nil {
						t.Fatal("missing stagechoice")
					}
					for _, o := range v.World.Choice.Options {
						worldGeometry(t, o.Map)
						for _, r := range o.Map.Routes {
							for _, p := range r.Points {
								if p.X < -40 || p.X > float64(o.Map.Width)+40 || p.Y < -40 || p.Y > float64(o.Map.Height)+40 {
									t.Fatalf("route %s escaped map: %+v", r.ID, p)
								}
							}
						}
					}
					option := v.World.Choice.Options[(branchBits>>(stage-2))&1].ID
					b.action(0, map[string]any{"type": "route_vote", "choice": option}, true)
					v = worldRead(t, b)
					if v.World.BoonChoice != nil {
						b.action(0, map[string]any{"type": "boon_vote", "choice": "power"}, true)
					}
				}
				worldArtifact(t, b, worldRead(t, b))
			})
		}
	}
}

// Malformed saves must be rejected before exposing controls or simulating them:
// objective health/progress/route corruption, duplicate IDs, broken preview
// maps, invalid region/feature coordinates, and unbounded boon counters.
func TestStarfortWorldRejectsCorruptSnapshots(t *testing.T) {
	b := newRulesTable(t, 1, map[string]any{"waves": 20})
	worldUntil(t, b, 6)
	v := worldRead(t, b)
	b.action(0, map[string]any{"type": "route_vote", "choice": v.World.Choice.Options[0].ID}, true)
	base := b.s
	cases := map[string]func(map[string]any){
		"negative objective health":   func(w map[string]any) { w["objectives"].([]any)[0].(map[string]any)["hp"] = -1 },
		"objective over maximum":      func(w map[string]any) { w["objectives"].([]any)[0].(map[string]any)["hp"] = 999999 },
		"objective missing route":     func(w map[string]any) { w["objectives"].([]any)[0].(map[string]any)["route"] = "missing" },
		"objective negative progress": func(w map[string]any) { w["objectives"].([]any)[0].(map[string]any)["progress"] = -1 },
		"objective offboard":          func(w map[string]any) { w["objectives"].([]any)[0].(map[string]any)["y"] = 99999 },
		"duplicate objective":         func(w map[string]any) { o := w["objectives"].([]any); w["objectives"] = append(o, o[0]) },
		"boon overflow":               func(w map[string]any) { w["boons"].(map[string]any)["power"] = 1e12 },
		"feature offboard": func(w map[string]any) {
			w["map"].(map[string]any)["features"].([]any)[0].(map[string]any)["x"] = -99999
		},
		"region offboard": func(w map[string]any) {
			w["map"].(map[string]any)["regions"].([]any)[0].(map[string]any)["width"] = 99999
		},
		"invalid feature kind": func(w map[string]any) {
			w["map"].(map[string]any)["features"].([]any)[0].(map[string]any)["kind"] = "arbitrary"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var raw map[string]any
			_ = json.Unmarshal(base.Data, &raw)
			mutate(raw["world"].(map[string]any))
			bad := base
			bad.Data = rulesJSON(raw)
			if _, err := b.m.View(bad, 0); err == nil {
				t.Fatal("corrupt save exposed")
			}
			if _, err := b.m.Apply(b.context(0), bad, rulesJSON(map[string]any{"type": "wait", "epoch": v.Epoch})); err == nil {
				t.Fatal("corrupt save mutated")
			}
		})
	}
	worldArtifact(t, b, map[string]any{"rejected": len(cases)})
}

// Migration failure cases, listed before the migration-only fix: a v1 battle
// suddenly gains seed-selected fog or frost; computed tower stats or actual
// movement change while reconnecting; calm migration leaks into later waves.
// These old-format fixtures represent an in-progress wave, then use the public
// timer and a persisted schema-2 roundtrip to verify the entire boundary.
func TestStarfortWorldLegacyWeatherBoundary(t *testing.T) {
	for _, tc := range []struct {
		name      string
		seed      int64
		nextEvent string
	}{{"fog", 3, "frost"}, {"frost", 4, "surge"}} {
		t.Run(tc.name, func(t *testing.T) {
			b := newRulesTable(t, 1, map[string]any{"waves": 20})
			b.action(0, map[string]any{"type": "build", "pad": 12, "tower": "arrow"}, true)
			b.action(0, map[string]any{"type": "upgrade", "towerId": 1}, true)
			expectedTower := b.view(0).Towers[0]
			var old map[string]any
			_ = json.Unmarshal(b.s.Data, &old)
			delete(old, "world")
			old["phase"] = "battle"
			old["wave"] = 3
			old["completedWaves"] = 2
			serverAt := int64(2147483647)*1000 + tc.seed
			b.now = time.UnixMilli(serverAt)
			old["serverAt"] = serverAt
			old["nextAt"] = 0
			old["waveInfo"].(map[string]any)["spawned"] = old["waveInfo"].(map[string]any)["total"]
			old["enemySerial"] = 1
			old["enemies"] = []any{map[string]any{"id": 1, "kind": "slime", "x": 250, "y": 240, "progress": 400, "hp": 100000, "maxHP": 100000, "shield": 0, "speed": 10, "slowUntil": 0, "armor": 0, "reward": 8, "leak": 1}}
			for _, tower := range old["towers"].([]any) {
				delete(tower.(map[string]any), "priority")
			}
			b.s.Schema = 1
			b.s.Data = rulesJSON(old)
			b.s.SystemDeadline = serverAt + 1000
			b.record(map[string]any{"op": "legacy_weather_fixture", "wouldSelect": tc.name, "match": b.s})
			migrated := worldRead(t, b)
			if migrated.World.Event.ID != "calm" || migrated.World.Seed != tc.seed {
				t.Fatalf("legacy current wave unexpectedly gains %s: %+v", tc.name, migrated.World.Event)
			}
			for sec := 0; sec < 2; sec++ {
				b.timer(time.Second)
				var restored = b.s
				_ = json.Unmarshal(rulesJSON(b.s), &restored)
				b.s = restored
				got := worldRead(t, b)
				if got.Wave != 3 || got.World.Event.ID != "calm" || b.s.Schema != 2 {
					t.Fatal("legacy weather changed during current battle or reload")
				}
				tower := got.Towers[0]
				if tower.EffectiveRange != expectedTower.EffectiveRange || tower.EffectiveDamage != expectedTower.EffectiveDamage || tower.EffectiveInterval != expectedTower.EffectiveInterval {
					t.Fatalf("legacy tower stats changed: %+v vs %+v", tower, expectedTower)
				}
				var enemy combatEnemy
				_ = json.Unmarshal(got.Enemies[0], &enemy)
				if math.Abs(enemy.Progress-float64(400+(sec+1)*10)) > 1e-9 || enemy.Speed != 10 {
					t.Fatalf("legacy movement changed: %+v", enemy)
				}
			}
			// Complete the real legacy road with one normal enemy leak; the next
			// preparation boundary must start ordinary seeded weather generation.
			var raw map[string]any
			_ = json.Unmarshal(b.s.Data, &raw)
			route := worldRead(t, b).Map.Routes[0]
			length := 0.
			for i := 1; i < len(route.Points); i++ {
				a, z := route.Points[i-1], route.Points[i]
				length += math.Hypot(z.X-a.X, z.Y-a.Y)
			}
			enemy := raw["enemies"].([]any)[0].(map[string]any)
			enemy["progress"] = length - 1
			enemy["x"] = 999
			enemy["y"] = 500
			b.s.Data = rulesJSON(raw)
			b.timer(time.Second)
			next := worldRead(t, b)
			if next.Phase != "prepare" || next.Wave != 4 || next.World.Event.ID != tc.nextEvent {
				t.Fatalf("next wave did not resume seeded weather: wave%d phase%s event%s", next.Wave, next.Phase, next.World.Event.ID)
			}
			worldArtifact(t, b, map[string]any{"legacySeed": tc.seed, "suppressed": tc.name, "currentWavePreserved": true, "nextWeather": next.World.Event.ID})
		})
	}
}
