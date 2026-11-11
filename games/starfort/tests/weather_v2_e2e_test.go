package starfort_test

// Failure inventory and numeric choices were recorded first in WEATHER_V2.md.
// These fixtures use the public module, timer, serialized match and HTTP host.
import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func v2Patch(s starfort.State, patch map[string]any) starfort.State {
	var raw map[string]any
	_ = json.Unmarshal(rulesJSON(s), &raw)
	raw["weatherRules"] = 2
	for k, v := range patch {
		raw[k] = v
	}
	_ = json.Unmarshal(rulesJSON(raw), &s)
	return s
}
func v2Boundary(t *testing.T, kind, branch string, enemies ...string) *rulesTable {
	fixtureBranch := branch
	if fixtureBranch == "" {
		fixtureBranch = map[string]string{"arrow": "heart", "cannon": "power", "frost": "speed", "arcane": "power"}[kind]
	}
	b := weatherBoundary(t, kind, fixtureBranch, 4, enemies...)
	s := v2Patch(cadenceState(b), nil)
	if branch == "" {
		s.Towers[0].Level = 1
		s.Towers[0].Branch = ""
	}
	cadenceSave(b, s)
	return b
}
func v2Enemy(b *rulesTable, i int) map[string]any {
	return specializationRaw(b)["enemies"].([]any)[i].(map[string]any)
}
func v2Field(v map[string]any, key string) float64 { x, _ := v[key].(float64); return x }
func TestStarfortWeatherV2Version(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{}, starfort.Module())
	if b.s.Schema != 9 || specializationRawView(b)["weatherRules"] != float64(2) {
		t.Fatal("new games require schema 9/weatherRules 2")
	}
	old := weatherBoundary(t, "arrow", "volley", 4, "slime")
	if old.s.Schema != 8 {
		t.Fatal("old weather save changed")
	}
	before := string(old.s.Data)
	old.view(0)
	old.validate()
	if string(old.s.Data) != before {
		t.Fatal("historical save migrated")
	}
	bad := b.s
	bad.Schema = 8
	if _, e := b.m.View(bad, 0); e == nil {
		t.Fatal("schema mismatch accepted")
	}
}
func TestStarfortWeatherV2TowerStats(t *testing.T) {
	for _, weather := range []string{"fog", "starlight", "frost"} {
		for _, kind := range []string{"arrow", "cannon", "frost", "arcane"} {
			t.Run(weather+"/"+kind, func(t *testing.T) {
				b := v2Boundary(t, kind, "", "slime")
				s := cadenceState(b)
				s.World.Event.ID = "calm"
				cadenceSave(b, s)
				base := b.view(0).Towers[0]
				s.World.Event.ID = weather
				cadenceSave(b, s)
				got := b.view(0).Towers[0]
				radius := base.EffectiveRange
				if weather == "fog" && kind != "arrow" {
					radius *= .9
				}
				weatherNear(t, "weather range", got.EffectiveRange, radius)
				rate := 1.
				if weather == "starlight" && kind != "arcane" {
					rate = .9
				}
				interval := map[string]float64{"arrow": 600, "cannon": 1600, "frost": 1000, "arcane": 1200}[kind]
				if got.EffectiveInterval != max(200, int(interval/rate)) {
					t.Fatal("weather attack speed", got.EffectiveInterval)
				}
			})
		}
	}
}
func TestStarfortWeatherV2Status(t *testing.T) {
	for _, weather := range []string{"fog", "starlight"} {
		t.Run(weather, func(t *testing.T) {
			b := v2Boundary(t, "frost", "speed", "healer")
			s := cadenceState(b)
			s.World.Event.ID = weather
			s.SkillRNG = 5
			s.Towers = nil
			s.Enemies[0].Speed = 40
			before := s.Enemies[0].Progress
			cadenceSave(b, s)
			b.timer(time.Second)
			e := v2Enemy(b, 0)
			key := "confusedUntil"
			if weather == "starlight" {
				key = "sleepingUntil"
			}
			until := v2Field(e, key)
			if until != 16 || v2Field(e, "weatherReadyAt") != 76 {
				t.Fatal("3s status/15s immunity from trigger", e)
			}
			if weather == "fog" && cadenceState(b).Enemies[0].Progress >= before {
				t.Fatal("fog does not reverse")
			}
			if weather == "starlight" {
				weatherNear(t, "sleep stops movement", cadenceState(b).Enemies[0].Progress, before)
			}
			s = cadenceState(b)
			s.SkillRNG = 5
			cadenceSave(b, s)
			b.timer(time.Second)
			if v2Field(v2Enemy(b, 0), key) != until {
				t.Fatal("same enemy weather reapplied")
			}
			for i := 0; i < 3; i++ {
				b.timer(time.Second)
			}
			if cadenceState(b).Enemies[0].Progress <= before && weather == "starlight" {
				t.Fatal("sleep did not end")
			}
			b.validate()
		})
	}
}
func TestStarfortWeatherV2SleepDamageAndStatus(t *testing.T) {
	for _, kind := range []string{"arrow", "cannon", "frost", "arcane"} {
		t.Run(kind, func(t *testing.T) {
			b := v2Boundary(t, kind, "", "slime")
			s := cadenceState(b)
			var e map[string]any
			_ = json.Unmarshal(rulesJSON(s.Enemies[0]), &e)
			e["sleepingUntil"] = 20
			e["weatherReadyAt"] = 80
			e["shield"] = 100
			s = v2Patch(s, map[string]any{"enemies": []any{e}})
			cadenceSave(b, s)
			b.timer(time.Second)
			after := cadenceState(b).Enemies[0]
			weatherNear(t, "sleep HP immunity", after.HP, 1000000)
			weatherNear(t, "sleep shield immunity", after.Shield, 100)
			if kind == "frost" && after.SlowUntil == 0 {
				t.Fatal("sleep blocks negative status")
			}
		})
	}
}
func TestStarfortWeatherV2Miss(t *testing.T) {
	for _, kind := range []string{"arrow", "cannon", "frost", "arcane"} {
		t.Run(kind, func(t *testing.T) {
			b := v2Boundary(t, kind, "", "slime")
			s := cadenceState(b)
			s.World.Event.ID = "frost"
			s.SkillRNG = 5
			cadenceSave(b, s)
			b.timer(time.Second)
			after := cadenceState(b)
			if kind == "frost" {
				if after.Enemies[0].HP >= 1000000 {
					t.Fatal("frost tower must never miss")
				}
				return
			}
			weatherNear(t, "miss no damage", after.Enemies[0].HP, 1000000)
			found := false
			for _, e := range after.Events {
				found = found || e.Type == "miss"
			}
			if !found {
				t.Fatal("missing authoritative miss cue")
			}
			after.SkillRNG = 5
			after.Towers[0].Cooldown = 900
			cadenceSave(b, after)
			b.timer(time.Second)
			if cadenceState(b).Enemies[0].HP >= 1000000 {
				t.Fatal("same-tower 10s miss immunity lost")
			}
		})
	}
}
func TestStarfortWeatherV2FrostMovement(t *testing.T) {
	b := v2Boundary(t, "arrow", "", "slime")
	s := cadenceState(b)
	s.Towers = nil
	s.World.Event.ID = "frost"
	s.Enemies[0].Speed = 40
	before := s.Enemies[0].Progress
	cadenceSave(b, s)
	b.timer(time.Second)
	weatherNear(t, "cold speed x0.9", cadenceState(b).Enemies[0].Progress-before, 36)
}
func TestStarfortWeatherV2BossGuards(t *testing.T) {
	b := v2Boundary(t, "arrow", "", "boss")
	s := cadenceState(b)
	s.Towers[0].Cooldown = 10000
	s.Enemies[0].HP = 500000
	s.Enemies[0].Phase = 1
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	if len(s.Enemies) != 5 {
		t.Fatal("boss must summon four guards", len(s.Enemies))
	}
	for i := 1; i < 5; i++ {
		if !boolField(v2Enemy(b, i), "bossGuard") {
			t.Fatal("guard identity missing")
		}
	}
	s.Towers[0].Cooldown = 900
	s.Towers[0].Priority = "strong"
	s.SkillRNG = 1
	after := s.EventSerial
	cadenceSave(b, s)
	b.timer(time.Second)
	for _, e := range cadenceState(b).Events {
		if e.ID > after && e.Type == "shot" && e.TargetID != 2 {
			t.Fatal("first living in-range guard must precede boss", e.TargetID)
		}
	}
	if len(cadenceState(b).Enemies) != 5 {
		t.Fatal("boss summoned twice")
	}
}
func boolField(v map[string]any, key string) bool { x, _ := v[key].(bool); return x }
func TestStarfortWeatherV2Leak(t *testing.T) {
	for _, tc := range []struct {
		kind     string
		fraction float64
		want     int
	}{{"boss", .01, 7}, {"boss", .5, 9}, {"boss", 1, 10}, {"elite", .01, 4}, {"elite", 1, 6}} {
		t.Run(fmt.Sprintf("%s/%g", tc.kind, tc.fraction), func(t *testing.T) {
			b := v2Boundary(t, "arrow", "", tc.kind)
			s := cadenceState(b)
			s.Towers = nil
			e := &s.Enemies[0]
			e.HP = e.MaxHP * tc.fraction
			e.Phase = 2
			e.Speed = 100
			e.Progress = fixturePathLength(s.World.Map.Routes[0].Points) - 1
			e.X, e.Y = fixturePathPosition(s.World.Map.Routes[0].Points, e.Progress)
			before := s.BaseHP
			cadenceSave(b, s)
			b.timer(time.Second)
			if cadenceState(b).BaseHP != before-tc.want {
				t.Fatal("health-proportional leak", cadenceState(b).BaseHP)
			}
		})
	}
	b := v2Boundary(t, "arrow", "", "shield", "shield", "shield", "shield")
	s := cadenceState(b)
	s.Towers = nil
	s.World.BossWards = 1
	var enemies []any
	for _, e := range s.Enemies {
		e.Progress = fixturePathLength(s.World.Map.Routes[0].Points) - 1
		e.X, e.Y = fixturePathPosition(s.World.Map.Routes[0].Points, e.Progress)
		e.Speed = 100
		var raw map[string]any
		_ = json.Unmarshal(rulesJSON(e), &raw)
		raw["bossGuard"] = true
		enemies = append(enemies, raw)
	}
	s = v2Patch(s, map[string]any{"enemies": enemies})
	before := s.BaseHP
	cadenceSave(b, s)
	b.timer(time.Second)
	if cadenceState(b).BaseHP != before-2 || cadenceState(b).World.BossWards != 1 {
		t.Fatal("guard leaks need half-count without consuming boss ward")
	}
}
func v2MedicalModule() gameapi.Module {
	m := starfort.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		v, e := start(ctx, raw)
		if e != nil {
			return v, e
		}
		var s starfort.State
		_ = json.Unmarshal(v.Data, &s)
		s.Wave = 21
		s.CompletedWaves = 20
		s.World.PreparedWave = 21
		s.World.TaskOffersWave = 21
		s.World.Objectives = nil
		s.World.Event.ID = "calm"
		s.BaseHP = 20
		s.NextAt = ctx.Now.Add(5 * time.Minute).UnixMilli()
		var w map[string]any
		_ = json.Unmarshal(rulesJSON(s.World), &w)
		w["medicalPacks"] = 1
		s = v2Patch(s, map[string]any{"world": w})
		return s.Envelope()
	}
	return m
}
func TestStarfortWeatherV2MedicalHTTP(t *testing.T) {
	tb := newHost(t, 2, v2MedicalModule())
	tb.start(nil)
	a := tb.people[0]
	tb.spectator.act(map[string]any{"type": "use_medical"}, 400)
	for _, p := range tb.people {
		p.act(map[string]any{"type": "pause"}, 200)
	}
	a.act(map[string]any{"type": "use_medical"}, 400)
	tb.restart()
	for _, p := range tb.people {
		p.act(map[string]any{"type": "resume"}, 200)
	}
	before := gold(a.room(), 0)
	a.act(map[string]any{"type": "use_medical"}, 200)
	if data(a.room())["baseHP"] != float64(23) || gold(a.room(), 0) != before {
		t.Fatal("pack heal/cost")
	}
	tb.people[1].act(map[string]any{"type": "use_medical"}, 400)
	tb.restart()
	if data(a.room())["baseHP"] != float64(23) {
		t.Fatal("medical use not persisted")
	}
	tb.record("shared pack: observer/pause rejected, SQLite restore, one free 3HP heal at wave 20, duplicate rejected")
}
func TestStarfortWeatherV2ConvoyAndPositions(t *testing.T) {
	for _, full := range []bool{false, true} {
		t.Run(fmt.Sprint(full), func(t *testing.T) {
			b := weatherTaskFixture(t, "convoy")
			s := v2Patch(cadenceState(b), nil)
			s.Wave = 20
			s.CompletedWaves = 19
			s.Phase = "battle"
			s.NextAt = 0
			s.WaveInfo.Total = 1
			s.WaveInfo.Spawned = 1
			s.World.Event.ID = "calm"
			o := &s.World.Objectives[0]
			o.EndWave = 20
			o.Progress = fixturePathLength(s.World.Map.Routes[0].Points) - 1
			o.X, o.Y = fixturePathPosition(s.World.Map.Routes[0].Points, o.Progress)
			if !full {
				s.BaseHP = 20
			}
			cadenceSave(b, s)
			b.timer(time.Second)
			if !full && cadenceState(b).BaseHP != 26 {
				t.Fatal("convoy wave-based heal")
			}
			if full {
				w := specializationRaw(b)["world"].(map[string]any)
				if v2Field(w, "medicalPacks") != 1 {
					t.Fatal("full core must award medical pack")
				}
			}
		})
	}
	b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, starfort.Module())
	seen := map[string]bool{}
	for wave := 1; wave <= 120; wave++ {
		s := cadenceState(b)
		if e := fixtureClearedWave(b.context(0), &s, wave, true); e != nil {
			t.Fatal(e)
		}
		cadenceSave(b, s)
		for _, o := range s.World.Objectives {
			if o.Kind != "mine" && o.Kind != "beacon" {
				continue
			}
			var path []starfort.Point
			for _, r := range s.World.Map.Routes {
				if r.ID == o.Route {
					path = r.Points
				}
			}
			fraction := o.Progress / fixturePathLength(path)
			lo := .4
			if o.Kind == "beacon" {
				lo = .5
			}
			if fraction < lo-1e-6 || fraction > lo+.1+1e-6 || math.IsNaN(fraction) {
				t.Fatal("new task location", o.Kind, fraction)
			}
			seen[o.Kind] = true
		}
	}
	if len(seen) != 2 {
		t.Fatal("task sample missing", seen)
	}
}
