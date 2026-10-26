package starfort_test

// Legacy layout regressions use captured v1.36.1 geometry; current central
// layout and skills are verified separately through production starts.

// Failure cases enumerated before combat implementation: enemies change route
// mid-wave; healing resurrects dead enemies, exceeds maximum health, or runs
// every tick; banner auras stack without bound or persist after their source
// dies; frost mutates stored speed; splitters reward their children repeatedly,
// recurse, or exceed the 64-entity cap; a boss summons every tick or teleports;
// engineers change a live road instead of telegraphing the next wave; support
// targeting ignores support enemies; ranged target selection ranks raw distance
// on unequal routes. Exact ability boundaries use serialized Module fixtures;
// natural-game E2E coverage lives in the world/rules suites. Every fixture keeps
// a replay, final snapshot and repeatable command through rulesTable.

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

type combatEnemy struct {
	ID                                       int    `json:"id"`
	Kind                                     string `json:"kind"`
	Route                                    string `json:"route"`
	Phase                                    int    `json:"phase"`
	AbilityTick                              int64  `json:"abilityTick"`
	X, Y, HP, MaxHP, Shield, Speed, Progress float64
}

type combatSnapshot struct {
	Enemies []combatEnemy `json:"enemies"`
	World   struct {
		ShortcutWave int `json:"shortcutWave"`
		Event        struct {
			ID string `json:"id"`
		} `json:"event"`
	} `json:"world"`
	Map struct {
		Pads []struct {
			ID   int
			X, Y float64
		} `json:"pads"`
		Routes []struct {
			ID     string                   `json:"id"`
			Points []struct{ X, Y float64 } `json:"points"`
		} `json:"routes"`
	} `json:"map"`
}

func combatPublic(b *rulesTable) combatSnapshot {
	b.t.Helper()
	v, err := b.m.View(b.s, 0)
	if err != nil {
		b.t.Fatal(err)
	}
	var out combatSnapshot
	if err = json.Unmarshal(rulesJSON(v), &out); err != nil {
		b.t.Fatal(err)
	}
	return out
}
func combatFixture(t *testing.T, enemies []map[string]any, tower bool) *rulesTable {
	t.Helper()
	b := newLegacyRulesTable(t, 1, map[string]any{})
	v := combatPublic(b)
	if tower {
		// The world has several initial road templates. Choose a real public
		// pad covering the combat fixture, rather than assuming v1 pad 1.
		type point struct{ x, y float64 }
		samples := []point{}
		for _, distance := range []float64{390, 450} {
			remaining := distance
			for j := 1; j < len(v.Map.Routes[0].Points); j++ {
				a, z := v.Map.Routes[0].Points[j-1], v.Map.Routes[0].Points[j]
				length := math.Hypot(z.X-a.X, z.Y-a.Y)
				if remaining <= length {
					samples = append(samples, point{a.X + (z.X-a.X)*remaining/length, a.Y + (z.Y-a.Y)*remaining/length})
					break
				}
				remaining -= length
			}
		}
		pad, best := -1, math.Inf(1)
		for _, p := range v.Map.Pads {
			distance := 0.
			for _, q := range samples {
				distance = math.Max(distance, math.Hypot(q.x-p.X, q.y-p.Y))
			}
			if distance < best {
				pad, best = p.ID, distance
			}
		}
		if pad < 0 || best > 180 {
			t.Fatalf("no real pad covers fixture: %d / %g", pad, best)
		}
		b.action(0, map[string]any{"type": "build", "pad": pad, "tower": "arrow"}, true)
	}
	b.allReady()
	if len(v.Map.Routes) == 0 {
		t.Fatal("world routes missing")
	}
	route := v.Map.Routes[0]
	var raw map[string]any
	if err := json.Unmarshal(b.s.Data, &raw); err != nil {
		t.Fatal(err)
	}
	for i, e := range enemies {
		progress, _ := e["progress"].(float64)
		x, y := route.Points[0].X, route.Points[0].Y
		remain := progress
		for j := 1; j < len(route.Points); j++ {
			a, z := route.Points[j-1], route.Points[j]
			length := math.Hypot(z.X-a.X, z.Y-a.Y)
			if remain <= length {
				x = a.X + (z.X-a.X)*remain/length
				y = a.Y + (z.Y-a.Y)*remain/length
				break
			}
			remain -= length
		}
		e["id"] = i + 1
		e["route"] = route.ID
		e["x"] = x
		e["y"] = y
		if _, ok := e["maxHP"]; !ok {
			e["maxHP"] = 100.
		}
		if _, ok := e["hp"]; !ok {
			e["hp"] = 100.
		}
		if _, ok := e["speed"]; !ok {
			e["speed"] = 10.
		}
		if _, ok := e["reward"]; !ok {
			e["reward"] = 8
		}
		e["leak"] = 1
	}
	raw["enemies"] = enemies
	raw["enemySerial"] = len(enemies)
	wave := raw["waveInfo"].(map[string]any)
	wave["spawned"] = wave["total"]
	b.s.Data = rulesJSON(raw)
	b.record(map[string]any{"op": "combat_fixture", "match": b.s})
	b.validate()
	return b
}
func combatFind(t *testing.T, v combatSnapshot, id int) combatEnemy {
	t.Helper()
	for _, e := range v.Enemies {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("enemy %d missing", id)
	return combatEnemy{}
}
func TestStarfortWorldCombatHealerCadence(t *testing.T) {
	b := combatFixture(t, []map[string]any{{"kind": "healer"}, {"kind": "slime", "hp": 50.}, {"kind": "slime", "hp": 99.}}, false)
	b.timer(time.Second)
	if got := combatFind(t, combatPublic(b), 2).HP; got != 50 {
		t.Fatalf("healed before 2-second cadence: %g", got)
	}
	b.timer(time.Second)
	v := combatPublic(b)
	if got := combatFind(t, v, 2).HP; math.Abs(got-53) > 1e-9 {
		t.Fatalf("3%% heal wrong: %g", got)
	}
	if got := combatFind(t, v, 3).HP; got != 100 {
		t.Fatalf("heal exceeds maxHP: %g", got)
	}
	b.timer(time.Second)
	if got := combatFind(t, combatPublic(b), 2).HP; got != 53 {
		t.Fatalf("healed again too early: %g", got)
	}
}
func TestStarfortWorldCombatBannerAndFrost(t *testing.T) {
	for _, event := range []string{"calm", "frost"} {
		t.Run(event, func(t *testing.T) {
			b := combatFixture(t, []map[string]any{{"kind": "banner"}, {"kind": "banner"}, {"kind": "slime"}}, false)
			var raw map[string]any
			_ = json.Unmarshal(b.s.Data, &raw)
			raw["world"].(map[string]any)["event"].(map[string]any)["id"] = event
			b.s.Data = rulesJSON(raw)
			b.timer(time.Second)
			e := combatFind(t, combatPublic(b), 3)
			want := 12.
			if event == "frost" {
				want *= .85
			}
			if math.Abs(e.Progress-want) > 1e-8 || e.Speed != 10 {
				t.Fatalf("aura/frost expected progress=%g and unchanged speed=10, got %+v", want, e)
			}
		})
	}
}
func TestStarfortWorldCombatBossAndEngineer(t *testing.T) {
	b := combatFixture(t, []map[string]any{{"kind": "boss", "hp": 49., "phase": 1}, {"kind": "engineer", "abilityTick": 0}}, false)
	b.timer(time.Second)
	v := combatPublic(b)
	if len(v.Enemies) != 4 || combatFind(t, v, 1).Phase != 2 {
		t.Fatalf("boss did not enter phase 2 and summon exactly two: %+v", v.Enemies)
	}
	route := combatFind(t, v, 1).Route
	for i := 0; i < 22; i++ {
		b.timer(time.Second)
	}
	v = combatPublic(b)
	if len(v.Enemies) != 4 {
		t.Fatalf("boss re-summoned: %d enemies", len(v.Enemies))
	}
	if v.World.ShortcutWave != 2 {
		t.Fatalf("engineer did not telegraph next-wave shortcut: %d", v.World.ShortcutWave)
	}
	for _, e := range v.Enemies {
		if e.Route != route {
			t.Fatal("live enemy changed its saved route")
		}
	}
}
func TestStarfortWorldCombatSplitter(t *testing.T) {
	b := combatFixture(t, []map[string]any{{"kind": "splitter", "hp": 1., "progress": 400.}, {"kind": "slime", "hp": 100000., "maxHP": 100000.}}, true)
	gold := b.view(0).Players[0].Gold
	b.timer(time.Second)
	v := combatPublic(b)
	if len(v.Enemies) != 3 {
		t.Fatalf("splitter must replace itself with 2 children: %+v", v.Enemies)
	}
	children := 0
	for _, e := range v.Enemies {
		if e.ID > 2 {
			children++
			if e.Kind != "swarm" || e.Progress < 400 {
				t.Fatalf("bad splitter child: %+v", e)
			}
		}
	}
	if children != 2 || b.view(0).Players[0].Gold-gold != 8 {
		t.Fatalf("wrong child count/reward: children=%d gold=%d", children, b.view(0).Players[0].Gold-gold)
	}
	for i := 0; i < 20; i++ {
		b.timer(time.Second)
	}
	if b.view(0).Players[0].Gold-gold != 8 {
		t.Fatal("splitter children created extra gold")
	}
}
func TestStarfortWorldCombatSummonCap(t *testing.T) {
	enemies := []map[string]any{{"kind": "splitter", "hp": 1., "progress": 400.}}
	for i := 0; i < 63; i++ {
		enemies = append(enemies, map[string]any{"kind": "slime", "hp": 100000., "maxHP": 100000.})
	}
	b := combatFixture(t, enemies, true)
	b.timer(time.Second)
	v := combatPublic(b)
	if len(v.Enemies) > 64 {
		t.Fatalf("summoning exceeded entity cap: %d", len(v.Enemies))
	}
	for _, e := range v.Enemies {
		if e.ID == 1 {
			t.Fatal("dead splitter retained")
		}
	}
}
func TestStarfortWorldCombatSupportPriority(t *testing.T) {
	b := combatFixture(t, []map[string]any{{"kind": "healer", "progress": 390.}, {"kind": "slime", "progress": 450.}}, true)
	b.action(0, map[string]any{"type": "target", "towerId": 1, "priority": "support"}, true)
	b.timer(time.Second)
	v := combatPublic(b)
	if combatFind(t, v, 1).HP >= 100 || combatFind(t, v, 2).HP != 100 {
		t.Fatalf("support priority attacked wrong enemy: %+v", v.Enemies)
	}
}

// Fortune failure cases, specified before wiring rewards: the selected boon
// is cosmetic; only kill or only wave rewards apply it; zero-reward summons
// become farmable; integer payouts overflow or disagree between teammates.
func TestStarfortWorldCombatFortuneRewards(t *testing.T) {
	b := combatFixture(t, []map[string]any{{"kind": "slime", "hp": 1., "progress": 400.}}, true)
	var raw map[string]any
	_ = json.Unmarshal(b.s.Data, &raw)
	raw["world"].(map[string]any)["boons"].(map[string]any)["fortune"] = 3
	b.s.Data = rulesJSON(raw)
	gold := b.view(0).Players[0].Gold
	b.timer(time.Second)
	if gain := b.view(0).Players[0].Gold - gold; gain != 10+55 {
		t.Fatalf("fortune should apply to both kill and wave: %d", gain)
	}
}

// Bot-targeting failure cases, before policy implementation: every tower keeps
// first despite support threats; bots repeatedly toggle targeting and starve
// construction; a bot changes another player's tower. Natural module gameplay
// must establish one arrow/arcane support interceptor in every opened sector.
func TestStarfortWorldCombatBotSupportTargets(t *testing.T) {
	b := newLegacyRulesTable(t, 2, map[string]any{"waves": 20})
	previous := map[int]string{}
	changes := 0
	covered := map[int]bool{}
	for sec := 0; sec < 16000 && b.s.Result == nil; sec++ {
		b.botPass()
		if b.s.Result == nil {
			b.timer(time.Second)
		}
		value, err := b.m.View(b.s, 0)
		if err != nil {
			t.Fatal(err)
		}
		var v struct {
			Map    struct{ Pads []struct{ Sector int } }
			Towers []struct {
				ID, Pad, Owner int
				Kind, Priority string
			}
		}
		if err = json.Unmarshal(rulesJSON(value), &v); err != nil {
			t.Fatal(err)
		}
		for _, tower := range v.Towers {
			if old, ok := previous[tower.ID]; ok && old != tower.Priority {
				changes++
				if old == "support" {
					t.Fatalf("bot churned established target policy for tower%d", tower.ID)
				}
			}
			previous[tower.ID] = tower.Priority
			if tower.Priority == "support" && (tower.Kind == "arrow" || tower.Kind == "arcane") {
				covered[v.Map.Pads[tower.Pad].Sector] = true
			}
		}
	}
	if len(covered) != 4 {
		t.Fatalf("bot support interceptors missing: sectors=%v changes=%d", covered, changes)
	}
	if changes > len(previous) {
		t.Fatal("target changes starved investments")
	}
	if b.view(0).Phase != "won" {
		t.Fatal("targeting policy prevented natural victory")
	}
}
