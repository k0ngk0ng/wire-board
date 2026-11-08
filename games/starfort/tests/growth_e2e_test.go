package starfort_test

// Failure inventory and numeric interpretation were written first in GROWTH.md.
// These are public Module/serialized-Match and real HTTP/SQLite tests. Fixed
// combat positions and RNG seeds are labelled boundary fixtures, not campaigns.
import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func growthState(s starfort.State) starfort.State {
	var fields map[string]any
	_ = json.Unmarshal(rulesJSON(s), &fields)
	fields["growthRules"] = 1
	_ = json.Unmarshal(rulesJSON(fields), &s)
	return s
}

func growthBoundary(t *testing.T, kind, branch string, level int, enemies ...string) *rulesTable {
	t.Helper()
	b := rebalanceBoundary(t, kind, branch, enemies...)
	s := growthState(cadenceState(b))
	s.Towers[0].Level = level
	s.Towers[0].Cooldown = 900
	s.SkillRNG = 5 // First roll is 5%; the second is 91%, so no knockback.
	cadenceSave(b, s)
	return b
}

func growthPassive(b *rulesTable) map[string]any {
	return specializationRawView(b)["towers"].([]any)[0].(map[string]any)["passive"].(map[string]any)
}

func specializationRawView(b *rulesTable) map[string]any {
	b.t.Helper()
	raw, err := b.m.View(b.s, 0)
	if err != nil {
		b.t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(rulesJSON(raw), &value); err != nil {
		b.t.Fatal(err)
	}
	return value
}

func TestStarfortGrowthStartAndLegacy(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{}, starfort.Module())
	if specializationRaw(b)["growthRules"] != float64(1) || b.s.Schema != 7 {
		t.Fatal("new growth rules require schema 7", b.s.Schema)
	}
	// Existing schema-6 matches retain their old behavior and serialization.
	old := rebalanceBoundary(t, "cannon", "speed", "slime")
	old.timer(time.Second)
	s := cadenceState(old)
	if old.s.Schema != 6 || s.Craters[0].Total != 6 || s.Towers[0].PassiveReadyAt != 10100 {
		t.Fatal("old balance version changed")
	}
	b.validate()
}

func TestStarfortGrowthCannon(t *testing.T) {
	for _, tc := range []struct{ level, chance, radius, percent int }{
		{4, 12, 102, 22}, {5, 14, 102, 22}, {6, 14, 104, 22}, {7, 14, 104, 24}, {100, 76, 166, 86},
	} {
		t.Run(fmt.Sprint(tc.level), func(t *testing.T) {
			b := growthBoundary(t, "cannon", "power", tc.level, "slime", "elite", "boss")
			p := growthPassive(b)
			if p["triggerChance"] != float64(tc.chance) {
				t.Fatal("probability", p)
			}
			b.timer(time.Second)
			s := cadenceState(b)
			found := false
			for _, e := range s.Events {
				if e.Type == "blast" {
					found = true
					if e.Amount != float64(tc.radius) {
						t.Fatal("radius", e.Amount)
					}
				}
			}
			if !found {
				t.Fatal("missing blast")
			}
			// All three enemies overlap at this explicit boundary. Both ordinary
			// and heavy units receive basic splash plus the scaled extra blast.
			damage := b.view(0).Towers[0].EffectiveDamage
			for _, e := range s.Enemies {
				if math.Abs((1000000-e.HP)-damage*(1+float64(tc.percent)/100)) > 1e-6 {
					t.Fatal("scaled splash damage", e.Kind, 1000000-e.HP, damage)
				}
			}
		})
	}
	for _, tc := range []struct{ level, chance, cd, work int }{
		{4, 12, 19, 5}, {19, 19, 11, 5}, {20, 19, 11, 6}, {21, 20, 11, 6}, {39, 29, 2, 6}, {40, 29, 2, 7}, {1000, 100, 0, 55},
	} {
		t.Run("crater-"+fmt.Sprint(tc.level), func(t *testing.T) {
			b := growthBoundary(t, "cannon", "speed", tc.level, "slime")
			p := growthPassive(b)
			cd := max(200, tc.cd*1000)
			if p["triggerChance"] != float64(tc.chance) || p["cooldownMS"] != float64(cd) {
				t.Fatal("crater view", p)
			}
			b.timer(time.Second)
			s := cadenceState(b)
			if len(s.Craters) != 1 || s.Craters[0].Total != float64(tc.work) || s.Towers[0].PassiveReadyAt != 900+int64(cd) {
				t.Fatal("crater growth", s.Craters, s.Towers[0].PassiveReadyAt)
			}
			c := b.view(0).Catalog
			var interval int
			for _, v := range c {
				if v.Kind == "cannon" {
					interval = v.Interval
				}
			}
			want := max(200, int(float64(interval)/(1+.04*float64(tc.level-1))))
			if b.view(0).Towers[0].EffectiveInterval != want {
				t.Fatal("crater attack rate is not x1.0")
			}
		})
	}
}

func TestStarfortGrowthVolley(t *testing.T) {
	for _, tc := range []struct{ level, chance, radius, percent, targets int }{
		{4, 14, 80, 52, 3}, {9, 18, 80, 58, 3}, {10, 18, 90, 58, 3}, {11, 20, 90, 58, 3},
		{19, 28, 90, 66, 3}, {20, 28, 90, 66, 4}, {21, 28, 90, 68, 4}, {30, 36, 100, 76, 4}, {40, 46, 100, 84, 5},
	} {
		t.Run(fmt.Sprint(tc.level), func(t *testing.T) {
			b := growthBoundary(t, "arrow", "volley", tc.level, "slime", "slime", "slime", "slime", "slime", "slime", "slime")
			p := growthPassive(b)
			if p["triggerChance"] != float64(tc.chance) {
				t.Fatal("volley probability", p)
			}
			if !strings.Contains(p["description"].(string), fmt.Sprint(tc.radius)) {
				t.Fatal("radius missing from public description", p)
			}
			b.timer(time.Second)
			s := cadenceState(b)
			hit := 0
			damage := b.view(0).Towers[0].EffectiveDamage
			for _, e := range s.Enemies {
				if e.ID == 1 {
					if math.Abs(1000000-e.HP-damage) > 1e-6 {
						t.Fatal("primary hit twice")
					}
					continue
				}
				if e.HP < 1000000 {
					hit++
					if math.Abs(1000000-e.HP-damage*float64(tc.percent)/100) > 1e-6 {
						t.Fatal("volley damage", e.ID, e.HP)
					}
				}
			}
			if hit != tc.targets {
				t.Fatal("extra target cap", hit, tc.targets)
			}
		})
	}
	for _, kind := range []string{"slime", "elite", "boss"} {
		t.Run("repeat-knockback-"+kind, func(t *testing.T) {
			b := growthBoundary(t, "arrow", "volley", 4, kind, "slime")
			s := cadenceState(b)
			s.SkillRNG = 1 // Arrow AoE fails (61), independent knockback succeeds (5).
			cadenceSave(b, s)
			b.timer(time.Second)
			s = cadenceState(b)
			want := 30.
			if kind != "slime" {
				want = 15
			}
			count := 0
			for _, e := range s.Events {
				if e.Type == "knockback" {
					count++
					if e.TargetID != 1 || e.Amount != want {
						t.Fatal("only primary is pushed", e)
					}
				}
			}
			if count != 1 {
				t.Fatal("knockback missing")
			}
			s.SkillRNG = 1
			s.Enemies = s.Enemies[:1]
			s.Towers[0].Cooldown = 900
			cadenceSave(b, s)
			b.timer(time.Second)
			count = 0
			for _, e := range cadenceState(b).Events {
				if e.Type == "knockback" {
					count++
					if count == 2 && e.Amount != want/2 {
						t.Fatal("eight-second same-tower reduction", e.Amount)
					}
				}
			}
			if count != 2 {
				t.Fatal("repeat should be reduced, not immune")
			}
		})
	}
}

func TestStarfortGrowthArcaneHeavyShield(t *testing.T) {
	for _, kind := range []string{"shield", "elite", "boss"} {
		b := growthBoundary(t, "arcane", "power", 20, kind)
		s := cadenceState(b)
		s.Enemies[0].Shield = 1000000
		cadenceSave(b, s)
		b.timer(time.Second)
		s = cadenceState(b)
		extra := 3.1
		if kind != "shield" {
			extra = 1.1
		}
		want := b.view(0).Towers[0].EffectiveDamage * 1.5 * extra
		if math.Abs(1000000-s.Enemies[0].Shield-want) > 1e-6 {
			t.Fatal("shield scaling", kind, 1000000-s.Enemies[0].Shield, want)
		}
	}
}

func TestStarfortGrowthCraterGuardRestore(t *testing.T) {
	b := growthBoundary(t, "cannon", "speed", 4, "slime")
	s := cadenceState(b)
	r := s.World.Map.Routes[0]
	s.Towers[0].Cooldown = 10000
	s.CraterSerial = 2
	s.Craters = nil
	for i, progress := range []float64{100, 200} {
		x, y := fixturePathPosition(r.Points, progress)
		s.Craters = append(s.Craters, starfort.Crater{ID: i + 1, TowerID: s.Towers[0].ID, Owner: 0, Route: r.ID, Progress: progress, X: x, Y: y, Radius: 28, Total: 5, Remaining: 5, Workers: []int{}})
	}
	s.Enemies[0].Progress = 99
	s.Enemies[0].Speed = 10
	s.Enemies[0].X, s.Enemies[0].Y = fixturePathPosition(r.Points, 99)
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	if s.Enemies[0].Repairing != 1 {
		t.Fatal("first crater did not stop enemy")
	}
	b.validate()
	s = cadenceState(b)
	s.Craters[0].Remaining = 0
	s.Craters[0].FilledMS = s.ElapsedMS
	s.Enemies[0].Repairing = 0
	s.Enemies[0].Progress = 199
	s.Enemies[0].X, s.Enemies[0].Y = fixturePathPosition(r.Points, 199)
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	if s.Enemies[0].Repairing != 0 {
		t.Fatal("same tower stopped the enemy again within four seconds")
	}
	// Another tower has an independent guard, including across serialization.
	s.TowerSerial++
	s.Craters[1].TowerID = s.TowerSerial
	s.Enemies[0].Progress = 199
	s.Enemies[0].X, s.Enemies[0].Y = fixturePathPosition(r.Points, 199)
	cadenceSave(b, s)
	b.timer(time.Second)
	if cadenceState(b).Enemies[0].Repairing != 2 {
		t.Fatal("another tower incorrectly shared immunity")
	}
}

func TestStarfortGrowthTaskRefundAndCosts(t *testing.T) {
	for _, kind := range []string{"mine", "beacon", "convoy"} {
		t.Run(kind, func(t *testing.T) {
			b := newModuleRulesTable(t, 2, map[string]any{"mode": "endless"}, starfort.Module())
			s := cadenceState(b)
			r := s.World.Map.Routes[0]
			x, y := fixturePathPosition(r.Points, 100)
			s.World.Objectives = []starfort.Objective{{ID: "refund", Kind: kind, Label: "Refund boundary fixture", HP: 1, MaxHP: 1, Cost: 105, Status: "available", Route: r.ID, Sector: 1, X: x, Y: y, Progress: 100}}
			cadenceSave(b, s)
			b.action(1, map[string]any{"type": "objective", "objectiveId": "refund"}, true)
			s = cadenceState(b)
			s.Phase = "battle"
			s.NextAt = 0
			s.World.Objectives[0].HP = .1
			s.EnemySerial = 1
			s.WaveInfo.Total = 1
			s.WaveInfo.Spawned = 1
			s.Enemies = []starfort.Enemy{{ID: 1, Kind: "slime", HP: 100, MaxHP: 100, Speed: 1, Leak: 1, Route: r.ID, Progress: 100, X: x, Y: y}}
			cadenceSave(b, s)
			b.timer(time.Second)
			s = cadenceState(b)
			if s.Players[0].Gold != 300 || s.Players[1].Gold != 216 || s.World.Objectives[0].Status != "failed" {
				t.Fatal("refund to payer exactly once", s.Players, s.World.Objectives)
			}
			b.validate()
			if cadenceState(b).Players[1].Gold != 216 {
				t.Fatal("restore refunded twice")
			}
		})
	}
	b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, starfort.Module())
	found := false
	for wave := 1; wave <= 30; wave++ {
		s := cadenceState(b)
		s.World.TaskAttempts = map[string]int{"convoy": 3}
		if err := fixtureClearedWave(b.context(0), &s, wave, true); err != nil {
			t.Fatal(err)
		}
		cadenceSave(b, s)
		for _, o := range s.World.Objectives {
			if o.Kind == "convoy" {
				found = true
				if o.Cost != 518 {
					t.Fatal("convoy must grow by 20%", o.Cost)
				}
			}
		}
	}
	if !found {
		t.Fatal("convoy never offered")
	}
}

func growthRepairModule() gameapi.Module {
	m := starfort.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		v, e := start(ctx, raw)
		if e != nil {
			return v, e
		}
		var s starfort.State
		_ = json.Unmarshal(v.Data, &s)
		s.BaseHP = 20
		s.World.Objectives = nil
		return s.Envelope()
	}
	return m
}

func TestStarfortGrowthRepairHTTP(t *testing.T) {
	tb := newHost(t, 2, growthRepairModule())
	tb.start(nil)
	a, p := tb.people[0], tb.people[1]
	tb.spectator.act(map[string]any{"type": "repair_base", "amount": 1}, 400)
	for _, amount := range []int{-1, 0, 4} {
		a.act(map[string]any{"type": "repair_base", "amount": amount}, 400)
	}
	a.act(map[string]any{"type": "repair_base", "amount": 2}, 200)
	if gold(a.room(), 0) != 100 || data(a.room())["baseHP"] != float64(22) {
		t.Fatal("repair amount/cost")
	}
	for _, c := range tb.people {
		c.act(map[string]any{"type": "pause"}, 200)
	}
	tb.restart()
	a.act(map[string]any{"type": "repair_base", "amount": 1}, 400)
	for _, c := range tb.people {
		c.act(map[string]any{"type": "resume"}, 200)
	}
	p.act(map[string]any{"type": "repair_base", "amount": 2}, 400)
	p.act(map[string]any{"type": "repair_base", "amount": 1}, 200)
	a.act(map[string]any{"type": "repair_base", "amount": 1}, 400)
	if data(a.room())["baseHP"] != float64(23) || gold(a.room(), 1) != 200 {
		t.Fatal("shared wave limit")
	}
	tb.record("repair charges payer, respects shared 10% wave limit, rejects spectator/invalid/paused/over-limit requests, survives SQLite restart")
}

func TestStarfortGrowthRefundHTTP(t *testing.T) {
	tb := newHost(t, 2, scenarioModule())
	tb.start(map[string]any{"scenario": "rebalance-growth-refund"})
	a := tb.people[0]
	wait(t, "real timer settles failed task", func() bool {
		return worldData(t, a.room())["objectives"].([]any)[0].(map[string]any)["status"] == "failed"
	})
	if gold(a.room(), 0) != 216 || gold(a.room(), 1) != 300 {
		t.Fatal("refund went to the wrong player")
	}
	tb.restart()
	before := data(a.room())["tick"].(float64)
	wait(t, "restored battle timer advances", func() bool { return data(a.room())["tick"].(float64) > before })
	if gold(a.room(), 0) != 216 || gold(a.room(), 1) != 300 {
		t.Fatal("refund repeated after SQLite restore")
	}
	tb.record("failed task refunds payer exactly once; real battle timer and SQLite restart do not replay refund")
}
