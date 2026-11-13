package starfort_test

// Failure inventory and numeric interpretation precede implementation in WEATHER.md.
// Public Module, timer, serialized Match and HTTP/SQLite boundary checks.
// Fixed positions/RNG are explicit boundary fixtures, never natural campaigns.
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

func weatherState(s starfort.State, patch map[string]any) starfort.State {
	var v map[string]any
	_ = json.Unmarshal(rulesJSON(s), &v)
	v["weatherRules"] = 1
	for k, x := range patch {
		v[k] = x
	}
	_ = json.Unmarshal(rulesJSON(v), &s)
	return s
}
func weatherBoundary(t *testing.T, kind, branch string, level int, enemies ...string) *rulesTable {
	t.Helper()
	b := growthBoundary(t, kind, branch, level, enemies...)
	s := weatherState(cadenceState(b), nil)
	s.World.Event = starfort.WaveEvent{ID: "calm", Label: "无特殊天气", Wave: s.Wave}
	cadenceSave(b, s)
	return b
}
func weatherNear(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("%s: got %.8f want %.8f", name, got, want)
	}
}
func TestStarfortWeatherVersionAndForecast(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, starfort.Module())
	if b.s.Schema != 11 || specializationRaw(b)["weatherRules"] != float64(3) || specializationRawView(b)["weatherRules"] != float64(3) {
		t.Fatal("new games need schema 11 and public weatherRules")
	}
	seen := map[string]bool{}
	for wave := 1; wave <= 40; wave++ {
		s := cadenceState(b)
		if err := fixtureClearedWave(b.context(0), &s, wave, true); err != nil {
			t.Fatal(err)
		}
		cadenceSave(b, s)
		seen[s.World.Event.ID] = true
		if s.World.Event.ID == "surge" {
			t.Fatal("fifth weather survived")
		}
		for _, e := range s.World.Forecast {
			if e.ID == "surge" {
				t.Fatal("forecast uses fifth weather")
			}
		}
		before := string(b.s.Data)
		b.view(0)
		b.validate()
		if string(b.s.Data) != before {
			t.Fatal("read/restore changed weather or task")
		}
	}
	if len(seen) != 4 {
		t.Fatal("four weather types", seen)
	}
	old := growthBoundary(t, "cannon", "power", 4, "slime")
	if old.s.Schema != 7 || specializationRaw(old)["weatherRules"] != nil {
		t.Fatal("schema 7 must stay historical")
	}
	if !strings.Contains(growthPassive(old)["description"].(string), "×1.5") {
		t.Fatal("old cannon changed")
	}
}

func TestStarfortWeatherUpgradeCosts(t *testing.T) {
	for _, tc := range []struct {
		level  int
		factor float64
	}{{9, 1}, {10, 1.1}, {19, 1.1}, {20, 1.15}, {29, 1.15}, {30, 1.2}, {100, 1.55}} {
		t.Run(fmt.Sprint(tc.level), func(t *testing.T) {
			b := weatherBoundary(t, "cannon", "power", tc.level, "slime")
			s := cadenceState(b)
			s.Players[0].Gold = 100000
			cadenceSave(b, s)
			base := math.Round(140 * (.7 + .38*float64(tc.level-1)))
			want := int(math.Round(base * tc.factor))
			if b.view(0).Controls.Upgrades[0].Cost != want {
				t.Fatal("upgrade quote", b.view(0).Controls.Upgrades[0].Cost, want)
			}
			b.action(0, map[string]any{"type": "upgrade", "towerId": s.Towers[0].ID}, true)
			if cadenceState(b).Players[0].Gold != 100000-want {
				t.Fatal("quote and debit differ")
			}
		})
	}
}

func TestStarfortWeatherCombatGrowth(t *testing.T) {
	for _, tc := range []struct {
		kind, branch                            string
		level, chance, radius, percent, targets int
		rate, damage                            float64
	}{
		{"cannon", "power", 4, 12, 105, 22, 0, 1, 1.2}, {"cannon", "power", 7, 14, 110, 24, 0, 1, 1.2},
		{"arrow", "volley", 4, 12, 85, 33, 3, .7, 1}, {"arrow", "volley", 9, 16, 95, 36, 3, .7, 1}, {"arrow", "volley", 10, 16, 95, 36, 4, .7, 1}, {"arrow", "volley", 20, 22, 110, 45, 5, .7, 1},
	} {
		t.Run(tc.kind+fmt.Sprint(tc.level), func(t *testing.T) {
			b := weatherBoundary(t, tc.kind, tc.branch, tc.level, "slime", "slime", "slime", "slime", "slime", "slime", "slime")
			p := growthPassive(b)
			if p["triggerChance"] != float64(tc.chance) {
				t.Fatal("chance", p)
			}
			if !strings.Contains(p["description"].(string), fmt.Sprint(tc.radius)) {
				t.Fatal("radius description", p)
			}
			b.timer(time.Second)
			s := cadenceState(b)
			damage := b.view(0).Towers[0].EffectiveDamage
			base := 48.
			interval := 1600.
			if tc.kind == "arrow" {
				base = 22
				interval = 600
			}
			weatherNear(t, "damage multiplier", damage, base*(1+.65*float64(tc.level-1))*tc.damage)
			if b.view(0).Towers[0].EffectiveInterval != max(200, int(interval/((1+.04*float64(tc.level-1))*tc.rate))) {
				t.Fatal("rate")
			}
			hits := 0
			for _, e := range s.Enemies {
				want := 0.
				if tc.kind == "cannon" {
					want = damage * (1 + float64(tc.percent)/100)
				} else if e.ID == 1 {
					want = damage
				} else if e.HP < 1000000 {
					hits++
					want = damage * float64(tc.percent) / 100
				}
				weatherNear(t, "area damage", 1000000-e.HP, want)
			}
			if tc.kind == "arrow" && hits != tc.targets {
				t.Fatal("extra target cap", hits, tc.targets)
			}
		})
	}
	for _, tc := range []struct{ level, chance, cd, work int }{{4, 14, 19, 5}, {9, 18, 16, 5}, {10, 18, 16, 6}, {11, 20, 16, 6}, {20, 28, 12, 7}, {1000, 100, 0, 105}} {
		t.Run("crater"+fmt.Sprint(tc.level), func(t *testing.T) {
			b := weatherBoundary(t, "cannon", "speed", tc.level, "slime")
			p := growthPassive(b)
			cd := max(200, tc.cd*1000)
			if p["triggerChance"] != float64(tc.chance) || p["cooldownMS"] != float64(cd) {
				t.Fatal("crater quote", p)
			}
			b.timer(time.Second)
			s := cadenceState(b)
			if len(s.Craters) != 1 || s.Craters[0].Total != float64(tc.work) || s.Towers[0].PassiveReadyAt != 900+int64(cd) {
				t.Fatal("crater state", s.Craters)
			}
			weatherNear(t, "no crater damage penalty", b.view(0).Towers[0].EffectiveDamage, 48*(1+.65*float64(tc.level-1)))
		})
	}
}

func TestStarfortWeatherHeartAndGuards(t *testing.T) {
	for _, kind := range []string{"slime", "elite", "boss"} {
		t.Run(kind, func(t *testing.T) {
			b := weatherBoundary(t, "arrow", "heart", 4, kind)
			s := cadenceState(b)
			s.Enemies[0].Phase = 2
			s.Enemies[0].HP = 100000
			s.Enemies[0].MaxHP = 1000000
			s.SkillRNG = 5
			cadenceSave(b, s)
			p := growthPassive(b)
			if p["name"] != "一箭残废" || p["triggerChance"] != float64(12) {
				t.Fatal("heart replacement", p)
			}
			damage := b.view(0).Towers[0].EffectiveDamage
			b.timer(time.Second)
			s = cadenceState(b)
			weatherNear(t, "remaining HP damage", 100000-s.Enemies[0].HP, damage+(100000-damage)*.11)
			hp := s.Enemies[0].HP
			s.Towers[0].Cooldown = 900
			s.SkillRNG = 5
			cadenceSave(b, s)
			b.timer(time.Second)
			weatherNear(t, "five second immunity", hp-cadenceState(b).Enemies[0].HP, damage)
			b.validate()
		})
	}
	for _, kind := range []string{"slime", "elite", "boss"} {
		b := weatherBoundary(t, "arrow", "volley", 4, kind)
		s := cadenceState(b)
		s.SkillRNG = 1
		s.World.Event.ID = "fog"
		cadenceSave(b, s)
		b.timer(time.Second)
		s = cadenceState(b)
		count := 0
		for _, e := range s.Events {
			if e.Type == "knockback" {
				count++
				want := 50.
				if kind != "slime" {
					want = 25
				}
				weatherNear(t, "fog knockback", e.Amount, want)
			}
		}
		if count != 1 {
			t.Fatal("independent push missing")
		}
		s.SkillRNG = 1
		s.Towers[0].Cooldown = 900
		cadenceSave(b, s)
		b.timer(time.Second)
		for _, e := range cadenceState(b).Events {
			if e.Type == "knockback" {
				count--
			}
		}
		if count != 0 {
			t.Fatal("same tower eight second push immunity")
		}
	}
}

func TestStarfortWeatherFreezeAndMeteor(t *testing.T) {
	for _, weather := range []string{"calm", "frost"} {
		b := weatherBoundary(t, "frost", "freeze", 4, "slime", "elite", "boss")
		s := cadenceState(b)
		s.World.Event.ID = weather
		cadenceSave(b, s)
		b.timer(time.Second)
		s = cadenceState(b)
		duration, cd := int64(2000), int64(10000)
		if weather == "frost" {
			duration, cd = 4000, 8000
		}
		if s.Towers[0].PassiveReadyAt != 900+cd {
			t.Fatal("freeze weather cooldown")
		}
		for _, e := range s.Enemies {
			if e.FrozenUntil != 5+duration/200 || e.RecoveryUntil-e.FrozenUntil != 40 {
				t.Fatal("freeze duration/recovery", e)
			}
		}
	}
	for _, weather := range []string{"calm", "starlight"} {
		b := weatherBoundary(t, "arcane", "magic", 4, "healer", "elite", "boss")
		s := cadenceState(b)
		s.World.Event.ID = weather
		cadenceSave(b, s)
		p := growthPassive(b)
		chance := 15.
		if weather == "starlight" {
			chance = 25
		}
		if p["triggerChance"] != chance || p["cooldownMS"] != float64(10000) {
			t.Fatal("meteor quote", p)
		}
		damage := b.view(0).Towers[0].EffectiveDamage
		b.timer(time.Second)
		s = cadenceState(b)
		for _, e := range s.Events {
			if e.Type == "chain" {
				t.Fatal("legacy chain survived")
			}
		}
		for _, e := range s.Enemies {
			want := damage * .33
			if e.ID == 1 {
				want += damage
			}
			if e.Kind != "healer" {
				want += damage * .5
			}
			weatherNear(t, "meteor damage", 1000000-e.HP, want)
			if e.SilencedUntil != 20 {
				t.Fatal("silence")
			}
		}
	}
}

func TestStarfortWeatherArcaneArmor(t *testing.T) {
	for _, kind := range []string{"slime", "elite", "boss"} {
		for _, armor := range []float64{0, .5} {
			b := weatherBoundary(t, "arcane", "power", 4, kind)
			s := cadenceState(b)
			s.Enemies[0].Armor = armor
			cadenceSave(b, s)
			damage := b.view(0).Towers[0].EffectiveDamage
			b.timer(time.Second)
			factor := 1.
			if armor > 0 {
				factor = 1.1
				if kind == "slime" {
					factor *= 1.4
				} else {
					factor *= 1.3
				}
			}
			weatherNear(t, "armored damage", 1000000-cadenceState(b).Enemies[0].HP, damage*factor)
		}
	}
}

func TestStarfortWeatherTaskLottery(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, starfort.Module())
	seen := map[string]int{}
	offers := 0
	for seed := int64(1); seed <= 240; seed++ {
		s := cadenceState(b)
		s.GeologyRules = 0
		s.Geology = nil
		s.GeoRNG = 0
		s.WeatherRules = 1 // Archived schema-8 lottery remains 25%.
		s.World.TaskOffersWave = 0
		s.World.PreparedWave = 0
		s.World.BoonWave = 0
		s.World.Choice = nil
		s.World.BoonChoice = nil
		s.World.Seed = seed
		s.SkillRNG = uint64(seed)
		s.World.TaskAttempts = map[string]int{"mine": 2, "beacon": 2, "convoy": 2, "merchant": 2}
		s.World.Objectives = nil
		wave := 12
		if seed%2 == 0 {
			wave = 5
		}
		if err := fixtureClearedWave(b.context(0), &s, wave, true); err != nil {
			t.Fatal(err)
		}
		cadenceSave(b, s)
		if len(s.World.Objectives) > 1 {
			t.Fatal("more than one shared lottery")
		}
		for _, o := range s.World.Objectives {
			offers++
			seen[o.Kind]++
			if wave < 10 && o.Kind == "merchant" {
				t.Fatal("early merchant")
			}
			baseHP, baseCost, growth, lo, hi, mult := 12., 80., 1.2, .3, .5, .8
			switch o.Kind {
			case "beacon":
				baseHP, lo, hi, mult = 8, .4, .6, 1
			case "convoy":
				baseHP, baseCost, growth, lo, hi, mult = 5, 200, 1.15, .3, .4, 1.1
			case "merchant":
				baseHP, baseCost, lo, hi, mult = 4, 200, .5, .6, 1.3
			}
			cost := int(math.Round(baseCost * math.Pow(growth, 2)))
			if o.Cost != cost || o.Reward != int(math.Round(float64(cost)*mult)) {
				t.Fatal("task economics", o)
			}
			weatherNear(t, "task HP", o.MaxHP, baseHP*1.44)
			var length float64
			active := false
			for _, r := range s.World.Map.Routes {
				if r.ID == o.Route {
					length = fixturePathLength(r.Points)
					active = r.Active
				}
			}
			f := o.Progress / length
			if !active || f < lo-1e-8 || f > hi+1e-8 {
				t.Fatal("task location", o.Kind, f, active)
			}
		}
	}
	if offers < 35 || offers > 90 || len(seen) != 4 {
		t.Fatal("25% shared lottery distribution", offers, seen)
	}
}

func weatherTaskFixture(t *testing.T, kind string) *rulesTable {
	t.Helper()
	b := newModuleRulesTable(t, 2, map[string]any{"mode": "endless"}, starfort.Module())
	s := cadenceState(b)
	s.GeologyRules = 0
	s.Geology = nil
	s.GeoRNG = 0
	s.WeatherRules = 1 // Archived schema-8 fixture.
	r := s.World.Map.Routes[0]
	x, y := fixturePathPosition(r.Points, 300)
	s.World.Objectives = []starfort.Objective{{ID: "weather-task", Kind: kind, Label: "任务边界夹具", HP: 10, MaxHP: 10, Cost: 100, Reward: 100, Status: "available", Owner: -1, Route: r.ID, Sector: 1, Progress: 300, X: x, Y: y}}
	cadenceSave(b, s)
	b.action(0, map[string]any{"type": "objective", "objectiveId": "weather-task"}, true)
	return b
}
func TestStarfortWeatherTaskDamageRefund(t *testing.T) {
	for _, kind := range []string{"mine", "beacon", "convoy", "merchant"} {
		t.Run(kind, func(t *testing.T) {
			b := weatherTaskFixture(t, kind)
			s := cadenceState(b)
			s.Phase = "battle"
			s.NextAt = 0
			s.World.Objectives[0].HP = .5
			s.WaveInfo.Total = 2
			s.WaveInfo.Spawned = 2
			s.EnemySerial = 1
			r := s.World.Map.Routes[0]
			x, y := fixturePathPosition(r.Points, 298)
			speed := 40.
			if kind == "merchant" {
				speed = 20
			}
			s.Enemies = []starfort.Enemy{{ID: 1, Kind: "slime", HP: 100, MaxHP: 100, Speed: speed + 20, Leak: 1, Route: r.ID, Progress: 298, X: x, Y: y}}
			cadenceSave(b, s)
			b.timer(time.Second)
			s = cadenceState(b)
			refund := 10
			if kind == "beacon" {
				refund = 20
			}
			if kind == "convoy" {
				refund = 30
			}
			if kind == "merchant" {
				refund = 0
			}
			if s.World.Objectives[0].Status != "failed" || s.World.Objectives[0].Refunded != refund || s.Players[0].Gold != 200+refund || s.Players[1].Gold != 300 {
				t.Fatal("single payer refund", s.World.Objectives, s.Players)
			}
			b.validate()
			b.timer(time.Second)
			if cadenceState(b).Players[0].Gold != 200+refund {
				t.Fatal("duplicate refund")
			}
		})
	}
	b := weatherTaskFixture(t, "mine")
	s := cadenceState(b)
	s.Phase = "battle"
	s.NextAt = 0
	s.EnemySerial = 3
	s.WaveInfo.Total = 3
	s.WaveInfo.Spawned = 3
	r := s.World.Map.Routes[0]
	x, y := fixturePathPosition(r.Points, 300)
	for i, kind := range []string{"slime", "elite", "boss"} {
		s.Enemies = append(s.Enemies, starfort.Enemy{ID: i + 1, Kind: kind, HP: 100, MaxHP: 100, Speed: .01, Leak: 1, Route: r.ID, Progress: 300, X: x, Y: y})
	}
	cadenceSave(b, s)
	b.timer(time.Second)
	weatherNear(t, "one attack per ordinary enemy", cadenceState(b).World.Objectives[0].HP, 9)
	b.validate()
	b.timer(time.Second)
	weatherNear(t, "no repeat after restore", cadenceState(b).World.Objectives[0].HP, 9)
}

func TestStarfortWeatherLeak(t *testing.T) {
	for _, kind := range []string{"elite", "boss"} {
		for _, ratio := range []float64{.01, .25, .5, .99, 1} {
			b := weatherBoundary(t, "arrow", "volley", 4, kind)
			s := cadenceState(b)
			s.Towers = nil
			s.World.Objectives = nil
			s.Enemies = s.Enemies[:1]
			e := &s.Enemies[0]
			e.Phase = 2
			e.HP = 1000 * ratio
			e.MaxHP = 1000
			e.Shield = 100
			e.Progress = fixturePathLength(s.World.Map.Routes[0].Points) - .1
			e.Speed = 100
			e.X, e.Y = fixturePathPosition(s.World.Map.Routes[0].Points, e.Progress)
			s.WaveInfo.Total = 1
			s.WaveInfo.Spawned = 1
			cadenceSave(b, s)
			base := 4
			if kind == "boss" {
				base = 8
			}
			want := base + int(math.Round(2*ratio))
			b.timer(time.Second)
			if cadenceState(b).BaseHP != 30-want {
				t.Fatal("HP ratio leak", kind, ratio, cadenceState(b).BaseHP, want)
			}
		}
	}
}

func weatherShopModule() gameapi.Module {
	m := starfort.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		v, e := start(ctx, raw)
		if e != nil {
			return v, e
		}
		var s starfort.State
		_ = json.Unmarshal(v.Data, &s)
		s.GeologyRules = 0
		s.Geology = nil
		s.GeoRNG = 0
		s.WeatherRules = 1 // Archived schema-8 fixture.
		s.Wave = 10
		s.CompletedWaves = 9
		s.Options.Mode = "endless"
		s.Phase = "battle"
		s.NextAt = 0
		s.BaseHP = 20
		s.Players[0].Gold = 2000
		s.Players[1].Gold = 2000
		r := s.World.Map.Routes[0]
		d := fixturePathLength(r.Points) - 1
		x, y := fixturePathPosition(r.Points, d)
		s.World.Objectives = []starfort.Objective{{ID: "merchant-shop", Kind: "merchant", Label: "商店边界夹具", HP: 4, MaxHP: 4, Cost: 200, Reward: 260, Status: "active", Owner: 0, StartWave: 10, Route: r.ID, Sector: 1, Progress: d, X: x, Y: y}}
		s.World.TaskAttempts = map[string]int{"merchant": 1}
		s.WaveInfo.Total = 1
		s.WaveInfo.Spawned = 1
		s.TowerSerial = 2
		s.Towers = []starfort.Tower{{ID: 1, Pad: 0, Owner: 0, Kind: "arrow", Level: 1, Spent: 90, Priority: "first"}, {ID: 2, Pad: 1, Owner: 1, Kind: "cannon", Level: 1, Spent: 140, Priority: "first"}}
		return s.Envelope()
	}
	return m
}
func TestStarfortWeatherShopHTTP(t *testing.T) {
	tb := newHost(t, 2, weatherShopModule())
	tb.start(nil)
	a, p := tb.people[0], tb.people[1]
	wait(t, "merchant arrives and wave clears", func() bool { return data(a.room())["wave"] == float64(11) })
	for _, kind := range []string{"route", "boon"} {
		key := "choice"
		if kind == "boon" {
			key = "boonChoice"
		}
		for {
			choice, ok := worldData(t, a.room())[key].(map[string]any)
			if !ok {
				break
			}
			id := choice["options"].([]any)[0].(map[string]any)["id"]
			for _, c := range tb.people {
				c.act(map[string]any{"type": kind + "_vote", "choice": id}, 200)
			}
		}
	}
	before := gold(a.room(), 0)
	baseDamage := data(a.room())["towers"].([]any)[0].(map[string]any)["effectiveDamage"].(float64)
	a.act(map[string]any{"type": "shop", "choice": "damage", "towerId": 2}, 400)
	tb.spectator.act(map[string]any{"type": "shop", "choice": "repair", "amount": 1}, 400)
	a.act(map[string]any{"type": "shop", "choice": "damage", "towerId": 1}, 200)
	a.act(map[string]any{"type": "shop", "choice": "damage", "towerId": 1}, 400)
	p.act(map[string]any{"type": "shop", "choice": "damage", "towerId": 2}, 400)
	a.act(map[string]any{"type": "shop", "choice": "range", "towerId": 1}, 200)
	a.act(map[string]any{"type": "shop", "choice": "repair", "amount": 1}, 200)
	a.act(map[string]any{"type": "shop", "choice": "repair", "amount": 1}, 400)
	a.act(map[string]any{"type": "shop", "choice": "shield"}, 200)
	p.act(map[string]any{"type": "shop", "choice": "shield"}, 400)
	if gold(a.room(), 0) != before-1300 || data(a.room())["baseHP"] != float64(21) {
		t.Fatal("shop costs and repair")
	}
	for _, c := range tb.people {
		c.act(map[string]any{"type": "pause"}, 200)
	}
	tb.restart()
	a.act(map[string]any{"type": "shop", "choice": "range", "towerId": 1}, 400)
	for _, c := range tb.people {
		c.act(map[string]any{"type": "resume"}, 200)
	}
	p.act(map[string]any{"type": "shop", "choice": "repair", "amount": 1}, 400)
	towers := data(a.room())["towers"].([]any)
	weatherNear(t, "permanent shop damage", towers[0].(map[string]any)["effectiveDamage"].(float64), baseDamage*1.05)
	tb.record("merchant must arrive before shop; independent shared quotas, ownership, funds, observer and pause guards; all four purchases and SQLite restore")
}

func TestStarfortWeatherNaturalCampaign(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			b := newModuleRulesTable(t, n, map[string]any{"waves": 20}, starfort.Module())
			for sec := 0; sec < 22000 && b.s.Result == nil; sec++ {
				b.botPass()
				if b.s.Result == nil {
					b.timer(time.Second)
				}
			}
			if b.view(0).Phase != "won" {
				t.Fatal("natural new-rule campaign failed", b.view(0).CompletedWaves)
			}
		})
	}
}
