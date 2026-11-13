package starfort_test

// WEATHER_V3.md records failure cases before implementation. These tests cross
// public Module, timers, serialized Match and real HTTP/SQLite boundaries.
// Fixed RNG and high-HP enemies are explicit boundary fixtures, not campaigns.
import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func v3Boundary(t *testing.T, kind, branch string, level int, enemies ...string) *rulesTable {
	b := weatherBoundary(t, kind, branch, level, enemies...)
	s := cadenceState(b)
	s.WeatherRules = 3
	cadenceSave(b, s)
	return b
}
func TestStarfortWeatherV3Version(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{}, preRevisionTestModule())
	if b.s.Schema != 11 || cadenceState(b).WeatherRules != 3 {
		t.Fatal("new games require schema 11/weatherRules 3")
	}
	for _, version := range []int{1, 2, 3} {
		old := weatherBoundary(t, "cannon", "power", 4, "slime")
		s := cadenceState(old)
		s.WeatherRules = version
		cadenceSave(old, s)
		if old.s.Schema != 7+version {
			t.Fatal("wrong schema", old.s.Schema)
		}
		before := string(old.s.Data)
		old.view(0)
		old.validate()
		if string(old.s.Data) != before {
			t.Fatal("view migrated save")
		}
		bad := old.s
		bad.Schema = 8 + (version % 3)
		if _, err := old.m.View(bad, 0); err == nil {
			t.Fatal("mismatched rule accepted")
		}
	}
}
func TestStarfortWeatherV3AreaGrowth(t *testing.T) {
	for _, tc := range []struct {
		kind, branch                            string
		level, chance, radius, percent, targets int
	}{
		{"cannon", "power", 4, 15, 110, 25, 0}, {"cannon", "power", 5, 20, 110, 25, 0}, {"cannon", "power", 7, 20, 120, 30, 0},
		{"arrow", "volley", 4, 15, 250, 35, 3}, {"arrow", "volley", 9, 25, 350, 40, 3}, {"arrow", "volley", 10, 25, 350, 40, 4}, {"arrow", "volley", 11, 25, 350, 45, 4},
	} {
		t.Run(tc.kind+fmt.Sprint(tc.level), func(t *testing.T) {
			b := v3Boundary(t, tc.kind, tc.branch, tc.level, "slime", "elite", "boss", "slime", "slime", "slime", "slime")
			p := growthPassive(b)
			if p["triggerChance"] != float64(tc.chance) || !strings.Contains(p["description"].(string), fmt.Sprint(tc.radius)) {
				t.Fatal("growth quote", p)
			}
			damage := b.view(0).Towers[0].EffectiveDamage
			b.timer(time.Second)
			s := cadenceState(b)
			extra := 0
			for _, e := range s.Enemies {
				want := damage * (1 + float64(tc.percent)/100)
				if tc.kind == "arrow" {
					want = 0
					if e.ID == 1 {
						want = damage
					} else if e.HP < 1000000 {
						extra++
						want = damage * float64(tc.percent) / 100
					}
				}
				weatherNear(t, "AoE damage", 1000000-e.HP, want)
			}
			if tc.kind == "arrow" && extra != tc.targets {
				t.Fatal("extra targets", extra)
			}
		})
	}
	for _, tc := range []struct{ level, chance, cd, work int }{{4, 20, 18, 5}, {5, 20, 18, 6}, {6, 20, 16, 6}, {9, 30, 14, 6}, {10, 30, 14, 7}, {15, 40, 10, 8}, {1000, 100, 0, 205}} {
		t.Run("crater"+fmt.Sprint(tc.level), func(t *testing.T) {
			b := v3Boundary(t, "cannon", "speed", tc.level, "slime")
			p := growthPassive(b)
			if p["triggerChance"] != float64(tc.chance) || p["cooldownMS"] != float64(max(200, tc.cd*1000)) {
				t.Fatal("crater quote", p)
			}
			b.timer(time.Second)
			s := cadenceState(b)
			if len(s.Craters) != 1 || s.Craters[0].Total != float64(tc.work) || s.Towers[0].PassiveReadyAt != 900+int64(max(200, tc.cd*1000)) {
				t.Fatal("crater state", s.Craters)
			}
		})
	}
}
func TestStarfortWeatherV3FreezeAndSlow(t *testing.T) {
	for _, tc := range []struct{ level, chance, duration, cd int }{{4, 20, 3, 8}, {5, 25, 3, 8}, {6, 25, 4, 8}, {7, 25, 4, 6}, {100, 100, 35, 0}} {
		for _, weather := range []string{"calm", "frost"} {
			t.Run(fmt.Sprint(tc.level)+weather, func(t *testing.T) {
				b := v3Boundary(t, "frost", "freeze", tc.level, "slime", "elite", "boss")
				s := cadenceState(b)
				s.World.Event.ID = weather
				for i := range s.Enemies {
					s.Enemies[i].WeatherCheckAt = 1000
				}
				cadenceSave(b, s)
				p := growthPassive(b)
				duration, cd := tc.duration*1000, tc.cd*1000
				if weather == "frost" {
					duration += 2000
					cd -= 2000
				}
				cd = max(200, cd)
				if p["triggerChance"] != float64(tc.chance) || p["cooldownMS"] != float64(cd) {
					t.Fatal("freeze quote", p)
				}
				b.timer(time.Second)
				s = cadenceState(b)
				for _, e := range s.Enemies {
					if e.FrozenUntil != 5+int64(duration/200) || e.RecoveryUntil-e.FrozenUntil != 40 {
						t.Fatal("freeze/recovery", e)
					}
				}
				if s.Towers[0].PassiveReadyAt != 900+int64(cd) {
					t.Fatal("freeze cooldown")
				}
			})
		}
	}
	for _, tc := range []struct {
		level    int
		factor   float64
		duration int64
	}{{4, .85, 10}, {5, .8, 10}, {6, .8, 13}, {9, .7, 15}} {
		b := v3Boundary(t, "frost", "speed", tc.level, "elite")
		b.timer(time.Second)
		e := cadenceState(b).Enemies[0]
		weatherNear(t, "slow strength", e.SlowFactor, tc.factor)
		if e.SlowUntil != 5+tc.duration {
			t.Fatal("half-second duration must round up to simulation step", e.SlowUntil)
		}
	}
}
func TestStarfortWeatherV3HeartAndMagic(t *testing.T) {
	for _, kind := range []string{"slime", "elite", "boss"} {
		b := v3Boundary(t, "arrow", "heart", 4, kind)
		s := cadenceState(b)
		s.Enemies[0].Phase = 2
		s.Enemies[0].HP = 100000
		s.World.Event.ID = "fog"
		for i := range s.Enemies {
			s.Enemies[i].WeatherCheckAt = 1000
		}
		cadenceSave(b, s)
		p := growthPassive(b)
		if p["triggerChance"] != float64(20) {
			t.Fatal("heart chance", p)
		}
		damage := b.view(0).Towers[0].EffectiveDamage
		b.timer(time.Second)
		weatherNear(t, "remaining HP cripple and fog", 100000-cadenceState(b).Enemies[0].HP, damage+(100000-damage)*.15*1.1)
		s = cadenceState(b)
		hp := s.Enemies[0].HP
		s.Towers[0].Cooldown = 900
		s.SkillRNG = 5
		cadenceSave(b, s)
		b.timer(time.Second)
		weatherNear(t, "heart protection", hp-cadenceState(b).Enemies[0].HP, damage)
	}
	for _, weather := range []string{"calm", "starlight"} {
		b := v3Boundary(t, "arcane", "magic", 4, "slime", "elite", "boss")
		s := cadenceState(b)
		s.World.Event.ID = weather
		for i := range s.Enemies {
			s.Enemies[i].WeatherCheckAt = 1000
		}
		cadenceSave(b, s)
		p := growthPassive(b)
		chance := 20.
		if weather == "starlight" {
			chance = 30
		}
		if p["triggerChance"] != chance || p["cooldownMS"] != float64(5000) || !strings.Contains(p["description"].(string), "240") {
			t.Fatal("meteor quote", p)
		}
		damage := b.view(0).Towers[0].EffectiveDamage
		b.timer(time.Second)
		s = cadenceState(b)
		if s.Towers[0].PassiveReadyAt != 5900 {
			t.Fatal("meteor cooldown")
		}
		for i, e := range s.Enemies {
			want := damage * .35
			if i == 0 {
				want += damage
			} else {
				want += damage
			}
			weatherNear(t, "meteor damage", 1000000-e.HP, want)
			if e.SilencedUntil != 20 || len(e.MeteorGuards) != 1 || e.MeteorGuards[0].Until != 45 {
				t.Fatal("meteor silence/protection", e)
			}
		}
	}
	for _, kind := range []string{"slime", "elite", "boss"} {
		b := v3Boundary(t, "arcane", "power", 4, kind)
		s := cadenceState(b)
		s.Enemies[0].Armor = .5
		s.World.Event.ID = "starlight"
		for i := range s.Enemies {
			s.Enemies[i].WeatherCheckAt = 1000
		}
		cadenceSave(b, s)
		damage := b.view(0).Towers[0].EffectiveDamage
		b.timer(time.Second)
		factor := 1.5
		if kind != "slime" {
			factor = 1.4
		}
		weatherNear(t, "arcane armor bonus", 1000000-cadenceState(b).Enemies[0].HP, damage*1.1*factor)
	}
}
func TestStarfortWeatherV3Sleep(t *testing.T) {
	b := v3Boundary(t, "frost", "speed", 4, "healer")
	s := cadenceState(b)
	s.Towers = nil
	s.World.Event.ID = "starlight"
	s.SkillRNG = 5
	s.Enemies[0].Speed = 40
	before := s.Enemies[0].Progress
	cadenceSave(b, s)
	b.timer(time.Second)
	e := cadenceState(b).Enemies[0]
	if e.SleepingUntil != 16 || e.WeatherReadyAt != 51 || e.Progress != before {
		t.Fatal("sleep duration, ten-second protection, movement", e)
	}
	for _, kind := range []string{"arrow", "cannon", "frost", "arcane"} {
		branch := map[string]string{"arrow": "heart", "cannon": "power", "frost": "speed", "arcane": "magic"}[kind]
		b := v3Boundary(t, kind, branch, 4, "slime")
		s := cadenceState(b)
		s.Enemies[0].SleepingUntil = 20
		s.Enemies[0].WeatherReadyAt = 80
		s.SkillRNG = 1
		cadenceSave(b, s)
		// Seed 1 fails both 15%/20% procs; only ordinary damage is halved.
		damage := b.view(0).Towers[0].EffectiveDamage
		b.timer(time.Second)
		e := cadenceState(b).Enemies[0]
		weatherNear(t, "sleep half damage "+kind, 1000000-e.HP, damage*.5)
		if kind == "frost" {
			weatherNear(t, "double slow", e.SlowFactor, .7)
			if e.SlowUntil != 25 {
				t.Fatal("double slow duration", e.SlowUntil)
			}
		}
	}
	for _, kind := range []string{"frost", "arcane"} {
		branch := "freeze"
		if kind == "arcane" {
			branch = "magic"
		}
		b := v3Boundary(t, kind, branch, 4, "elite")
		s := cadenceState(b)
		s.Enemies[0].SleepingUntil = 20
		s.Enemies[0].WeatherReadyAt = 80
		cadenceSave(b, s)
		b.timer(time.Second)
		e := cadenceState(b).Enemies[0]
		if kind == "frost" && (e.FrozenUntil != 35 || e.RecoveryUntil-e.FrozenUntil != 80) {
			t.Fatal("sleep doubled freeze/recovery", e)
		}
		if kind == "arcane" && e.SilencedUntil != 35 {
			t.Fatal("sleep doubled silence", e)
		}
	}
}
func TestStarfortWeatherV3Knockback(t *testing.T) {
	for _, kind := range []string{"slime", "elite", "boss"} {
		b := v3Boundary(t, "arrow", "volley", 4, kind)
		s := cadenceState(b)
		s.SkillRNG = 1
		s.World.Event.ID = "fog"
		for i := range s.Enemies {
			s.Enemies[i].WeatherCheckAt = 1000
		}
		cadenceSave(b, s)
		b.timer(time.Second)
		found := false
		for _, e := range cadenceState(b).Events {
			if e.Type == "knockback" {
				found = true
				want := 70.
				if kind != "slime" {
					want = 35
				}
				weatherNear(t, "fog knockback", e.Amount, want)
			}
		}
		if !found {
			t.Fatal("independent knockback missing")
		}
	}
}
func TestStarfortWeatherV3TaskLottery(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, preRevisionTestModule())
	seen := map[string]bool{}
	offered := 0
	for wave := 1; wave <= 200; wave++ {
		s := cadenceState(b)
		s.World.Objectives = nil
		if err := fixtureClearedWave(b.context(0), &s, wave, true); err != nil {
			t.Fatal(err)
		}
		cadenceSave(b, s)
		if len(s.World.Objectives) > 1 {
			t.Fatal("independent task rolls")
		}
		for _, o := range s.World.Objectives {
			offered++
			seen[o.Kind] = true
			if wave < 10 && o.Kind == "merchant" {
				t.Fatal("early merchant")
			}
		}
		before := string(b.s.Data)
		b.view(0)
		b.validate()
		if before != string(b.s.Data) {
			t.Fatal("read rerolled lottery")
		}
	}
	if offered < 55 || offered > 105 || len(seen) != 4 {
		t.Fatal("40% shared lottery", offered, seen)
	}
	for _, tc := range []struct {
		seed    uint64
		roll    int
		offered bool
	}{{100, 24, true}, {29, 25, true}, {115, 39, true}, {40, 40, false}} {
		x := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, preRevisionTestModule())
		s := cadenceState(x)
		s.SkillRNG = tc.seed
		s.World.Objectives = nil
		if err := fixtureClearedWave(x.context(0), &s, 1, true); err != nil {
			t.Fatal(err)
		}
		if (len(s.World.Objectives) == 1) != tc.offered {
			t.Fatal("shared 40% boundary", tc.roll, s.World.Objectives)
		}
	}
}
func v3HTTPModule(weather string) gameapi.Module {
	m := preRevisionTestModule()
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		b := v3BoundaryForHTTP(ctx, raw)
		labels := map[string]string{"calm": "无特殊天气", "fog": "薄雾", "frost": "寒潮", "starlight": "星辉"}
		b.World.Event = starfort.WaveEvent{ID: weather, Label: labels[weather], Wave: b.Wave}
		return b.Envelope()
	}
	return m
}
func v3BoundaryForHTTP(ctx gameapi.Context, raw json.RawMessage) starfort.State {
	v, err := preRevisionTestModule().Start(ctx, raw)
	if err != nil {
		panic(err)
	}
	var s starfort.State
	_ = json.Unmarshal(v.Data, &s)
	s.NextAt = ctx.Now.Add(5 * time.Minute).UnixMilli()
	s.World.Objectives = nil
	s.TowerSerial = 1
	s.Towers = []starfort.Tower{{ID: 1, Pad: 1, Owner: 0, Kind: "frost", Level: 4, Branch: "freeze", Spent: 400, Priority: "first", PassiveReadyAt: 5000}}
	return s
}
func TestStarfortWeatherV3HTTP(t *testing.T) {
	// A random initial cold wave legitimately changes level-four cooldown to
	// six seconds. Exercise every weather explicitly so restart expectations
	// never depend on the room's random seed.
	for _, tc := range []struct {
		weather  string
		cooldown float64
	}{
		{"calm", 8000}, {"fog", 8000}, {"frost", 6000}, {"starlight", 8000},
	} {
		t.Run(tc.weather, func(t *testing.T) {
			tb := newHost(t, 2, v3HTTPModule(tc.weather))
			tb.start(nil)
			a := tb.people[0]
			if data(a.room())["weatherRules"] != float64(3) {
				t.Fatal("HTTP new rule missing")
			}
			initial := data(a.room())["towers"].([]any)[0].(map[string]any)
			if initial["passive"].(map[string]any)["cooldownMS"] != tc.cooldown {
				t.Fatal("wrong weather cooldown before restart", tc, initial)
			}
			a.act(map[string]any{"type": "target", "towerId": 1, "priority": "strong"}, 200)
			tb.restart()
			d := data(a.room())
			tower := d["towers"].([]any)[0].(map[string]any)
			if d["weatherRules"] != float64(3) || tower["priority"] != "strong" || tower["passive"].(map[string]any)["cooldownMS"] != tc.cooldown || tower["passiveReadyAt"] != float64(5000) {
				t.Fatal("SQLite restore lost rules/cooldown", tc, tower)
			}
			tb.record("schema10 weather3 " + tc.weather + ": HTTP/SQLite restart preserves priority, weather-specific freeze cooldown and saved ready time")
		})
	}
}
