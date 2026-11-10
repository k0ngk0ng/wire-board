package starfort_test

// Additional module-boundary acceptance for the failure inventory in WEATHER.md.
// Explicit milestone fixtures; no artificial resources are used by campaigns.
import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
)

func TestStarfortWeatherBeaconAndMineRewards(t *testing.T) {
	for prior := 0; prior < 4; prior++ {
		t.Run(fmt.Sprint(prior), func(t *testing.T) {
			b := weatherTaskFixture(t, "beacon")
			s := cadenceState(b)
			s.World.BeaconSuccesses = prior
			s.TowerSerial = 1
			s.Towers = []starfort.Tower{{ID: 1, Pad: 1, Owner: 0, Kind: "arrow", Level: 1, Spent: 90, Priority: "first"}}
			s.World.Objectives[0].X = s.World.Map.Pads[1].X
			s.World.Objectives[0].Y = s.World.Map.Pads[1].Y
			s.World.Event.ID = "calm"
			cadenceSave(b, s)
			weatherNear(t, "active beacon attack aura", b.view(0).Towers[0].EffectiveDamage, 22*1.2)
			s.Phase = "battle"
			s.NextAt = 0
			s.WaveInfo.Total = 1
			s.WaveInfo.Spawned = 1
			cadenceSave(b, s)
			b.timer(time.Second)
			s = cadenceState(b)
			percent := float64(prior/2+2) / 100
			if prior%2 == 0 {
				weatherNear(t, "alternating reach", s.World.BeaconReach, percent)
				weatherNear(t, "no power on reach turn", s.World.BeaconPower, 0)
			} else {
				weatherNear(t, "alternating power", s.World.BeaconPower, percent)
				weatherNear(t, "no reach on power turn", s.World.BeaconReach, 0)
			}
			if s.Players[0].Gold != 343 || s.Players[1].Gold != 443 {
				t.Fatal("beacon rewards paid once", s.Players)
			}
			b.validate()
		})
	}
	b := weatherTaskFixture(t, "mine")
	s := cadenceState(b)
	for len(s.World.Map.Pads) < 40 {
		i := len(s.World.Map.Pads)
		s.World.Map.Pads = append(s.World.Map.Pads, starfort.Pad{ID: i, X: float64(50 + (i%8)*100), Y: float64(50 + (i/8)*100), Custom: true, Terrain: "plain", Sector: 1})
	}
	s.TowerSerial = 1
	s.Towers = []starfort.Tower{{ID: 1, Pad: 1, Owner: 1, Kind: "arrow", Level: 4, Branch: "heart", Spent: 400, Priority: "support"}}
	s.Phase = "battle"
	s.NextAt = 0
	s.WaveInfo.Total = 1
	s.WaveInfo.Spawned = 1
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	if s.World.PadReward == nil || s.World.PadReward.Kind != "move" {
		t.Fatal("40-pad mine must award movement")
	}
	before := s.Towers[0]
	b.action(1, map[string]any{"type": "pad_reward", "towerId": 1, "x": 850, "y": 100}, false)
	b.action(0, map[string]any{"type": "pad_reward", "towerId": 1, "x": -1, "y": 100}, false)
	b.action(0, map[string]any{"type": "pad_reward", "towerId": 1, "x": 850, "y": 100}, true)
	s = cadenceState(b)
	if len(s.World.Map.Pads) != 40 || !reflect.DeepEqual(s.Towers[0], before) || s.World.Map.Pads[before.Pad].X != 850 || s.World.Map.Pads[before.Pad].Y != 100 {
		t.Fatal("move changed tower or pad count")
	}
	b.action(0, map[string]any{"type": "pad_reward", "towerId": 1, "x": 850, "y": 200}, false)
}

func TestStarfortWeatherTaskRouteAndHealth(t *testing.T) {
	b := weatherTaskFixture(t, "mine")
	s := cadenceState(b)
	if err := fixtureWorldStage(b.context(0), &s, 2); err != nil {
		t.Fatal(err)
	}
	r := s.World.Map.Routes[0]
	x, y := fixturePathPosition(r.Points, 300)
	s.World.Objectives = []starfort.Objective{{ID: "lane", Kind: "mine", Label: "分路边界", HP: 12, MaxHP: 12, Route: r.ID, Progress: 300, X: x, Y: y, Sector: 1, Owner: 0, StartWave: s.Wave, EndWave: s.Wave, Status: "active"}}
	s.Phase = "battle"
	s.NextAt = 0
	s.WaveInfo.Total = 2
	s.WaveInfo.Spawned = 2
	s.EnemySerial = 1
	s.Enemies = []starfort.Enemy{{ID: 1, Kind: "slime", HP: 100, MaxHP: 100, Speed: .01, Leak: 1, Route: s.World.Map.Routes[1].ID, Progress: 300, X: x, Y: y}}
	cadenceSave(b, s)
	b.timer(time.Second)
	weatherNear(t, "other route cannot attack overlapping task", cadenceState(b).World.Objectives[0].HP, 12)
	// Compare otherwise identical spawns on the accepted lane and another lane.
	s = cadenceState(b)
	s.Enemies = nil
	s.WaveInfo = starfort.WaveInfo{Rule: "march", Total: 2}
	s.SpawnClock = 0
	s.World.Event.ID = "calm"
	cadenceSave(b, s)
	b.timer(time.Second)
	b.timer(time.Second)
	s = cadenceState(b)
	if len(s.Enemies) != 2 {
		t.Fatal("spawn boundary")
	}
	weatherNear(t, "accepted lane health x1.1", s.Enemies[0].MaxHP/s.Enemies[1].MaxHP, 1.1)
}

func TestStarfortWeatherCraterAndFreezeProtection(t *testing.T) {
	b := weatherBoundary(t, "cannon", "power", 4, "slime")
	s := cadenceState(b)
	s.CraterSerial = 1
	s.SkillRNG = 1
	e := &s.Enemies[0]
	e.Repairing = 1
	s.Craters = []starfort.Crater{{ID: 1, TowerID: s.Towers[0].ID, Owner: 0, Route: e.Route, Progress: e.Progress, X: e.X, Y: e.Y, Radius: 28, Total: 5, Remaining: 5, Workers: []int{1}}}
	cadenceSave(b, s)
	damage := b.view(0).Towers[0].EffectiveDamage
	b.timer(time.Second)
	weatherNear(t, "power cannon versus filling target", 1000000-cadenceState(b).Enemies[0].HP, damage*1.5)
	for _, kind := range []string{"slime", "elite", "boss"} {
		b := weatherBoundary(t, "cannon", "speed", 4, kind)
		s := cadenceState(b)
		s.Towers[0].Cooldown = 10000
		s.CraterSerial = 1
		e := &s.Enemies[0]
		e.Repairing = 1
		s.Craters = []starfort.Crater{{ID: 1, TowerID: s.Towers[0].ID, Owner: 0, Route: e.Route, Progress: e.Progress, X: e.X, Y: e.Y, Radius: 28, Total: 5, Remaining: 5, Workers: []int{1}}}
		cadenceSave(b, s)
		b.timer(time.Second)
		want := 4.
		if kind != "slime" {
			want = 3
		}
		weatherNear(t, "heavy half-time work", cadenceState(b).Craters[0].Remaining, want)
	}
	b = weatherBoundary(t, "frost", "freeze", 4, "slime")
	s = cadenceState(b)
	s.World.Event.ID = "frost"
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	until := s.Enemies[0].FrozenUntil
	// Second attack is explicitly made ready before eight seconds, exercising
	// the independent per-enemy guard rather than the tower's normal cooldown.
	s.Towers[0].PassiveReadyAt = 0
	s.Towers[0].Cooldown = 900
	s.SkillRNG = 5
	cadenceSave(b, s)
	b.timer(time.Second)
	if cadenceState(b).Enemies[0].FrozenUntil != until {
		t.Fatal("same-tower freeze guard ignored")
	}
}

func TestStarfortWeatherMerchantGateAndWard(t *testing.T) {
	b := newModuleRulesTable(t, 2, map[string]any{"mode": "endless"}, weatherShopModule())
	b.action(0, map[string]any{"type": "shop", "choice": "shield"}, false)
	b.timer(time.Second)
	s := cadenceState(b)
	for s.World.Choice != nil {
		choice := s.World.Choice.Options[0].ID
		for i := range s.Players {
			b.action(i, map[string]any{"type": "route_vote", "choice": choice}, true)
		}
		s = cadenceState(b)
	}
	if s.World.BoonChoice != nil {
		choice := s.World.BoonChoice.Options[0].ID
		for i := range s.Players {
			b.action(i, map[string]any{"type": "boon_vote", "choice": choice}, true)
		}
	}
	b.action(0, map[string]any{"type": "shop", "choice": "shield"}, true)
	s = cadenceState(b)
	s.Phase = "battle"
	s.NextAt = 0
	s.World.Shop = nil
	s.World.Objectives = nil
	s.Towers = nil
	s.World.Event.ID = "calm"
	r := s.World.Map.Routes[0]
	d := fixturePathLength(r.Points) - .1
	x, y := fixturePathPosition(r.Points, d)
	s.EnemySerial = 2
	s.WaveInfo.Total = 2
	s.WaveInfo.Spawned = 2
	s.Enemies = []starfort.Enemy{{ID: 1, Kind: "elite", HP: 100, MaxHP: 100, Speed: 100, Leak: 4, Route: r.ID, Progress: d, X: x, Y: y, Phase: 2}, {ID: 2, Kind: "boss", HP: 100, MaxHP: 100, Speed: 100, Leak: 10, Route: r.ID, Progress: d, X: x, Y: y, Phase: 2}}
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	if s.BaseHP != 14 || s.World.BossWards != 0 {
		t.Fatal("ward must protect only the boss, once", s.BaseHP, s.World.BossWards)
	}
	b.validate()
	b.action(0, map[string]any{"type": "shop", "choice": "shield"}, false)
}

func TestStarfortWeatherFinalMerchantShop(t *testing.T) {
	b := newModuleRulesTable(t, 2, map[string]any{}, weatherShopModule())
	s := cadenceState(b)
	s.Options.Mode = "campaign"
	s.Options.Waves = 10
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	if b.s.Result != nil || s.Phase != "prepare" || s.World.Shop == nil || s.CompletedWaves != 10 {
		t.Fatal("final-wave shop lost")
	}
	b.action(0, map[string]any{"type": "shop", "choice": "repair", "amount": 1}, true)
	b.action(0, map[string]any{"type": "close_shop"}, true)
	if b.s.Result == nil || b.view(0).Phase != "won" {
		t.Fatal("final store exit did not settle win")
	}
}

func TestStarfortWeatherNaturalModes(t *testing.T) {
	for _, difficulty := range []string{"easy", "hard"} {
		t.Run(difficulty, func(t *testing.T) {
			b := newModuleRulesTable(t, 2, map[string]any{"waves": 10, "difficulty": difficulty}, starfort.Module())
			for sec := 0; sec < 16000 && b.s.Result == nil; sec++ {
				b.botPass()
				if b.s.Result == nil {
					b.timer(time.Second)
				}
			}
			if b.view(0).Phase != "won" {
				t.Fatal("natural mode", difficulty, b.view(0).CompletedWaves)
			}
		})
	}
	t.Run("endless", func(t *testing.T) {
		b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, starfort.Module())
		for sec := 0; sec < 22000 && b.s.Result == nil && b.view(0).CompletedWaves < 22; sec++ {
			b.botPass()
			if b.s.Result == nil {
				b.timer(time.Second)
			}
		}
		if b.s.Result != nil || b.view(0).CompletedWaves < 22 {
			t.Fatal("natural endless", b.view(0).CompletedWaves)
		}
	})
}

func TestStarfortWeatherFogHeart(t *testing.T) {
	b := weatherBoundary(t, "arrow", "heart", 4, "slime")
	s := cadenceState(b)
	s.World.Event.ID = "fog"
	s.Enemies[0].HP = 100000
	cadenceSave(b, s)
	damage := b.view(0).Towers[0].EffectiveDamage
	b.timer(time.Second)
	weatherNear(t, "fog enhances base and remaining-health damage", 100000-cadenceState(b).Enemies[0].HP, damage+(100000-damage)*.11*1.05)
}

func TestStarfortWeatherPadsAndMixedAutoplay(t *testing.T) {
	t.Run("even-road-pads", func(t *testing.T) {
		b := newModuleRulesTable(t, 4, map[string]any{"mode": "endless"}, starfort.Module())
		s := cadenceState(b)
		for stage := 1; stage <= 4; stage++ {
			old := append([]starfort.Pad{}, s.World.Map.Pads...)
			if err := fixtureWorldStage(b.context(0), &s, stage); err != nil {
				t.Fatal(err)
			}
			if stage > 1 && !reflect.DeepEqual(old, s.World.Map.Pads[:len(old)]) {
				t.Fatal("expansion moved existing pads")
			}
			route := s.World.Map.Routes[stage-1]
			count, high := 0, 0
			for _, pad := range s.World.Map.Pads {
				if pad.Route != route.ID {
					continue
				}
				count++
				if pad.Terrain == "highground" {
					high++
				}
				progress := fixturePathLength(route.Points) * float64(count) / 7
				x, y := fixturePathPosition(route.Points, progress)
				nx, ny := fixturePathPosition(route.Points, progress+1)
				tangent := math.Hypot(nx-x, ny-y)
				along := ((pad.X-x)*(nx-x) + (pad.Y-y)*(ny-y)) / tangent
				// A pad remains in its own evenly spaced road slot. Crossings
				// require lateral/longitudinal clearance from existing roads;
				// half a slot is the geometric boundary, independent of scale.
				if math.Abs(along) > fixturePathLength(route.Points)/14 || math.Hypot(pad.X-x, pad.Y-y) > 175 {
					t.Fatal("pad not near evenly spaced road progress", stage, count, pad, "along", along)
				}
			}
			if count != 6 || high != 1 {
				t.Fatal("six pads and one highground per road", stage, count, high)
			}
		}
		cadenceSave(b, s)
		b.validate()
	})
	t.Run("explicit-autoplay-and-takeover-http", func(t *testing.T) {
		tb := newHost(t, 2, starfort.Module())
		tb.start(nil)
		a, p := tb.people[0], tb.people[1]
		a.act(map[string]any{"type": "build", "pad": 0, "tower": "arrow"}, 200)
		before := worldData(t, a.room())["event"]
		p.command("autoplay", map[string]any{"enabled": true}, 200)
		wait(t, "new-rule bot invests alongside human", func() bool { return len(data(a.room())["towers"].([]any)) > 1 })
		p.command("autoplay", map[string]any{"enabled": false}, 200)
		if p.room()["seats"].([]any)[1].(map[string]any)["autoPlay"] == true || !reflect.DeepEqual(before, worldData(t, a.room())["event"]) {
			t.Fatal("takeover reset rules or weather")
		}
		tb.record("schema-8 manual/explicit-autoplay teammate builds legally, manual takeover preserves weather and match")
	})
}

func TestStarfortWeatherMeteorBoundsAndRestore(t *testing.T) {
	b := weatherBoundary(t, "arcane", "magic", 4, "slime", "slime", "boss")
	s := cadenceState(b)
	s.Enemies[1].Progress -= 100
	s.Enemies[1].X -= 100
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	if s.Enemies[1].HP != 1000000 || s.Enemies[1].SilencedUntil != 0 {
		t.Fatal("meteor escaped primary-target radius")
	}
	hp := s.Enemies[2].HP
	s.Towers[0].PassiveReadyAt = 0
	s.Towers[0].Cooldown = 900
	s.SkillRNG = 5
	cadenceSave(b, s)
	b.validate()
	b.timer(time.Second)
	s = cadenceState(b)
	weatherNear(t, "meteor per-enemy guard survives serialization", s.Enemies[2].HP, hp)
	for _, e := range specializationRawView(b)["enemies"].([]any) {
		v := e.(map[string]any)
		for _, key := range []string{"freezeGuards", "heartGuards", "meteorGuards", "craterGuards", "knockbackGuards"} {
			if _, ok := v[key]; ok {
				t.Fatal("saved-only guard leaked on wire", key)
			}
		}
	}
}

func TestStarfortWeatherEscortMustArrive(t *testing.T) {
	for _, kind := range []string{"convoy", "merchant"} {
		t.Run(kind, func(t *testing.T) {
			var b *rulesTable
			if kind == "merchant" {
				b = newModuleRulesTable(t, 2, map[string]any{"mode": "endless"}, weatherShopModule())
			} else {
				b = weatherTaskFixture(t, kind)
			}
			s := cadenceState(b)
			s.Phase = "battle"
			s.NextAt = 0
			s.Towers = nil
			s.BaseHP = 20
			s.WaveInfo.Total = 1
			s.WaveInfo.Spawned = 1
			s.Enemies = nil
			o := &s.World.Objectives[0]
			o.Progress = 300
			o.X, o.Y = fixturePathPosition(s.World.Map.Routes[0].Points, 300)
			wave, gold := s.Wave, s.Players[0].Gold
			cadenceSave(b, s)
			b.timer(time.Second)
			s = cadenceState(b)
			speed := 40.
			if kind == "merchant" {
				speed = 20
			}
			weatherNear(t, "escort speed", s.World.Objectives[0].Progress, 300+speed)
			if s.Wave != wave || s.Phase != "battle" || s.World.Objectives[0].Status != "active" || s.World.Shop != nil || s.Players[0].Gold != gold {
				t.Fatal("wave or reward settled before escort arrived")
			}
			b.validate()
			b.action(0, map[string]any{"type": "shop", "choice": "shield"}, false)
			s = cadenceState(b)
			o = &s.World.Objectives[0]
			o.Progress = fixturePathLength(s.World.Map.Routes[0].Points) - 1
			o.X, o.Y = fixturePathPosition(s.World.Map.Routes[0].Points, o.Progress)
			cadenceSave(b, s)
			b.timer(time.Second)
			s = cadenceState(b)
			if s.CompletedWaves != wave || s.Phase != "prepare" {
				t.Fatal("arrival did not clear the wave")
			}
			if kind == "convoy" && s.BaseHP != 24 {
				t.Fatal("convoy did not heal after arrival")
			}
			if kind == "merchant" && s.World.Shop == nil {
				t.Fatal("arrived merchant did not open post-wave store")
			}
		})
	}
}
