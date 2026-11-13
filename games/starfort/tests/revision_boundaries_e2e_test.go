package starfort_test

// Additional public-boundary coverage from REVISION.md: deadline edges, every
// leak kind, contact ledgers, late reward claims and independent proc rolls.
import (
	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"math"
	"testing"
	"time"
)

func TestStarfortRevisionInvulnerabilityLeaks(t *testing.T) {
	for _, kind := range []string{"slime", "elite", "boss", "guard"} {
		t.Run(kind, func(t *testing.T) {
			b := revBoundary(t, "arrow", "heart", 4, "slime")
			s := cadenceState(b)
			s.Towers = nil
			e := &s.Enemies[0]
			e.Kind = kind
			if kind == "guard" {
				e.Kind = "shield"
				e.BossGuard = true
			}
			e.Progress = fixturePathLength(s.World.Map.Routes[0].Points) - .01
			e.X, e.Y = fixturePathPosition(s.World.Map.Routes[0].Points, e.Progress)
			e.Speed = 10
			s.InvulnerabilityPacks = 1
			cadenceSave(b, s)
			b.action(0, map[string]any{"type": "use_invulnerability"}, true)
			b.timer(time.Second)
			if cadenceState(b).BaseHP != 30 {
				t.Fatal("leak ignored immunity", kind)
			}
		})
	}
	for _, until := range []int64{0, 1, 2} {
		b := revBoundary(t, "arrow", "heart", 4, "slime")
		s := cadenceState(b)
		s.Towers = nil
		s.InvulnerableUntil = until
		e := &s.Enemies[0]
		e.Progress = fixturePathLength(s.World.Map.Routes[0].Points) - .01
		e.X, e.Y = fixturePathPosition(s.World.Map.Routes[0].Points, e.Progress)
		e.Speed = 10
		cadenceSave(b, s)
		b.timer(time.Second)
		want := 30
		if until <= 1 {
			want = 29
		}
		if cadenceState(b).BaseHP != want {
			t.Fatal("deadline", until)
		}
	}
	b := newModuleRulesTable(t, 1, map[string]any{}, starfort.Module())
	s := cadenceState(b)
	s.World.Objectives = nil
	s.InvulnerabilityPacks = 1
	cadenceSave(b, s)
	b.action(0, map[string]any{"type": "use_invulnerability"}, true)
	b.action(0, map[string]any{"type": "pause"}, true)
	before := cadenceState(b).InvulnerableUntil
	b.now = b.now.Add(time.Hour)
	b.action(0, map[string]any{"type": "resume"}, true)
	if cadenceState(b).Tick != 0 || cadenceState(b).InvulnerableUntil != before {
		t.Fatal("pause consumed simulation time")
	}
}

func TestStarfortRevisionProcDamageAndVisuals(t *testing.T) {
	b := revBoundary(t, "arrow", "volley", 4, "slime", "slime", "slime", "slime", "slime")
	damage := b.view(0).Towers[0].EffectiveDamage
	b.timer(time.Second)
	s := cadenceState(b)
	extra := 0
	for _, e := range s.Enemies {
		loss := 1000000 - e.HP
		if math.Abs(loss-damage*.22) < 1e-6 {
			extra++
		}
	}
	if extra != 3 {
		t.Fatal("three extra victims", extra)
	}
	rain := 0
	for _, e := range s.Events {
		if e.Type == "arrow_rain" {
			rain++
			if e.VisualMS != 2000 || len(e.StatusTargets) != 3 {
				t.Fatal("rain contract", e)
			}
		}
	}
	if rain != 1 {
		t.Fatal("rain event missing")
	}
	// First saved roll 10 misses the 6% volley but succeeds the independent 5%
	// push on its next roll (search only test RNG, never force production result).
	for seed := uint64(1); seed < 10000; seed++ {
		x := seed
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		a := x % 100
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		if a >= 6 && x%100 < 5 {
			b = revBoundary(t, "arrow", "volley", 4, "slime")
			s = cadenceState(b)
			s.SkillRNG = seed
			cadenceSave(b, s)
			b.timer(time.Second)
			found := false
			for _, e := range cadenceState(b).Events {
				if e.Type == "knockback" {
					found = true
					weatherNear(t, "independent push", e.Amount, 50)
				}
			}
			if !found {
				t.Fatal("push tied to volley")
			}
			break
		}
	}
	b = revBoundary(t, "arcane", "magic", 4, "slime", "elite", "boss")
	damage = b.view(0).Towers[0].EffectiveDamage
	b.timer(time.Second)
	s = cadenceState(b)
	for _, e := range s.Enemies {
		if e.Kind == "elite" || e.Kind == "boss" {
			weatherNear(t, "heavy meteor", 1000000-e.HP, damage*(1.32+func() float64 {
				for _, v := range s.Events {
					if v.Type == "shot" && v.TargetID == e.ID {
						return 1
					}
				}
				return 0
			}()))
			if len(e.MeteorGuards) != 1 || e.MeteorGuards[0].Until != 30 {
				t.Fatal("five second protection", e)
			}
		}
	}
	found := false
	for _, e := range s.Events {
		if e.Type == "meteor" {
			found = true
			if e.VisualMS != 3000 || e.SilencedUntil != 20 {
				t.Fatal("meteor contract", e)
			}
		}
	}
	if !found {
		t.Fatal("missing meteor")
	}
}

func TestStarfortRevisionWetContactAndRewardPersistence(t *testing.T) {
	b := revBoundary(t, "arrow", "heart", 4, "slime")
	s := cadenceState(b)
	s.Towers = nil
	e := s.Enemies[0]
	s.Geology = &starfort.Geology{Wave: s.Wave, Kind: "flood", Triggered: true, RoutePeaks: map[string]float64{}, Zones: []starfort.GeoZone{{ID: 1, Kind: "flood", X: e.X - 100, Y: e.Y - 50, Width: 200, Height: 100, CreatedMS: 0, ExpiresMS: 20000, Sweep: true}}}
	s.Tick = 25
	s.ElapsedMS = 5000
	cadenceSave(b, s)
	b.timer(time.Second)
	first := cadenceState(b).Enemies[0]
	b.timer(time.Second)
	second := cadenceState(b).Enemies[0]
	weatherNear(t, "one flood hit", first.HP, 900000)
	weatherNear(t, "no second damage", second.HP, first.HP)
	if second.WetUntil != cadenceState(b).Tick+25 || second.WetUntil <= first.WetUntil {
		t.Fatal("wet refresh")
	}
	s = cadenceState(b)
	s.Phase = "prepare"
	s.Enemies = nil
	s.World.PadRewards = []starfort.PadReward{{Owner: 0, Kind: "add", Count: 2}, {Owner: 0, Kind: "highground", Count: 1}}
	s.NextAt = b.now.Add(time.Second).UnixMilli()
	cadenceSave(b, s)
	b.timer(time.Second)
	if len(cadenceState(b).World.PadRewards) != 2 || b.view(0).Phase != "battle" {
		t.Fatal("timeout auto claimed reward")
	}
	b.action(0, map[string]any{"type": "pad_reward", "choice": "highground", "pad": 1}, true)
	s = cadenceState(b)
	if len(s.World.PadRewards) != 1 || s.World.PadRewards[0].Count != 2 {
		t.Fatal("other reward overwritten")
	}
	b.validate()
}

func TestStarfortRevisionMagePowerTargets(t *testing.T) {
	for _, tc := range []struct {
		kind                  string
		armor, shield, factor float64
	}{
		{"slime", 0, 0, 1}, {"armored", .5, 0, 1.2 * 1.5}, {"shield", 0, 800000, 1.2}, {"shield", .5, 800000, 1.2 * 1.5}, {"elite", 0, 0, 1.4}, {"boss", .5, 0, 1.2 * 1.4},
	} {
		b := revBoundary(t, "arcane", "power", 4, tc.kind)
		s := cadenceState(b)
		s.Enemies[0].Armor = tc.armor
		s.Enemies[0].Shield = tc.shield
		cadenceSave(b, s)
		damage := b.view(0).Towers[0].EffectiveDamage
		b.timer(time.Second)
		after := cadenceState(b).Enemies[0]
		weatherNear(t, "mage target multiplier "+tc.kind, 1000000-after.HP, damage*tc.factor)
		weatherNear(t, "mage retained shield", after.Shield, tc.shield)
	}
}

func TestStarfortRevisionMineRewardsAccumulate(t *testing.T) {
	b := newModuleRulesTable(t, 2, map[string]any{}, starfort.Module())
	s := cadenceState(b)
	s.World.MineSuccesses = 1
	s.World.PadRewards = []starfort.PadReward{{Owner: 0, Kind: "add", Count: 2}}
	for i := range s.World.Map.Pads {
		s.World.Map.Pads[i].Terrain = "highground"
	}
	for owner := 0; owner < 2; owner++ {
		r := s.World.Map.Routes[0]
		d := fixturePathLength(r.Points) * .45
		x, y := fixturePathPosition(r.Points, d)
		s.World.Objectives = []starfort.Objective{{ID: "mine-completion", Kind: "mine", Label: "矿场", Status: "active", Owner: owner, HP: 10, MaxHP: 10, Route: r.ID, Progress: d, X: x, Y: y, Sector: 1, StartWave: s.Wave, EndWave: s.Wave, Cost: 80, Reward: 64}}
		s.Phase = "battle"
		s.NextAt = 0
		s.Enemies = nil
		s.WaveInfo = starfort.WaveInfo{Total: 1, Spawned: 1}
		cadenceSave(b, s)
		b.timer(time.Second)
		s = cadenceState(b)
	}
	if len(s.World.PadRewards) != 3 || s.World.PadRewards[0].Count != 2 || s.World.PadRewards[1].Kind != "highground" || s.World.PadRewards[2].Owner != 1 {
		t.Fatal("deferred rewards lost", s.World.PadRewards)
	}
}

func TestStarfortRevisionRetainedRewardsAndLeaks(t *testing.T) {
	for _, tc := range []struct {
		peak  float64
		bonus int
	}{{.5, 10}, {.50001, 5}, {.7, 5}, {.70001, 0}} {
		b := revBoundary(t, "arrow", "heart", 4, "slime")
		s := cadenceState(b)
		s.Enemies = nil
		s.World.Objectives = nil
		s.Geology = &starfort.Geology{Wave: s.Wave, Kind: "none", RoutePeaks: map[string]float64{s.World.Map.Routes[0].ID: tc.peak}}
		gold := s.Players[0].Gold
		cadenceSave(b, s)
		b.timer(time.Second)
		if cadenceState(b).Players[0].Gold-gold != 43+tc.bonus {
			t.Fatal("exclusive historical-distance bonus", tc)
		}
	}
	for _, tc := range []struct {
		kind     string
		fraction float64
		damage   int
	}{{"elite", .01, 4}, {"elite", .5, 5}, {"elite", 1, 6}, {"boss", .01, 7}, {"boss", .5, 9}, {"boss", 1, 10}} {
		b := revBoundary(t, "arrow", "heart", 4, tc.kind)
		s := cadenceState(b)
		s.Towers = nil
		e := &s.Enemies[0]
		e.HP = e.MaxHP * tc.fraction
		e.Phase = 2
		e.AbilityTick = 10000
		e.Progress = fixturePathLength(s.World.Map.Routes[0].Points) - .01
		e.X, e.Y = fixturePathPosition(s.World.Map.Routes[0].Points, e.Progress)
		e.Speed = 10
		cadenceSave(b, s)
		b.timer(time.Second)
		if cadenceState(b).BaseHP != 30-tc.damage {
			t.Fatal("health-fraction leak", tc, cadenceState(b).BaseHP)
		}
	}
}
