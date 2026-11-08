package starfort_test

// Failure cases were recorded in REBALANCE.md before implementation. These
// tests cross production Module, serialization and real HTTP/SQLite boundaries.
import (
	"encoding/json"
	"fmt"
	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"testing"
	"time"
)

func TestStarfortRebalancePadRewardHTTP(t *testing.T) {
	m := starfort.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		v, e := start(ctx, raw)
		if e != nil {
			return v, e
		}
		var s starfort.State
		if e = json.Unmarshal(v.Data, &s); e != nil {
			return v, e
		}
		s.World.Objectives = []starfort.Objective{}
		s.World.PadReward = &starfort.PadReward{Owner: 0, Kind: "add"}
		return s.Envelope()
	}
	tb := newHost(t, 2, m)
	tb.start(nil)
	a := tb.people[0]
	tb.people[1].act(map[string]any{"type": "pad_reward", "x": 100, "y": 80}, 400)
	tb.spectator.act(map[string]any{"type": "pad_reward", "x": 100, "y": 80}, 400)
	a.act(map[string]any{"type": "pad_reward", "x": -1, "y": 80}, 400)
	for _, p := range tb.people {
		p.act(map[string]any{"type": "pause"}, 200)
	}
	tb.restart()
	for _, p := range tb.people {
		p.act(map[string]any{"type": "resume"}, 200)
	}
	a.act(map[string]any{"type": "pad_reward", "x": 100, "y": 80}, 200)
	a.act(map[string]any{"type": "pad_reward", "x": 150, "y": 80}, 400)
	if len(data(a.room())["map"].(map[string]any)["pads"].([]any)) != 7 {
		t.Fatal("claim did not persist exactly once")
	}
	tb.record("owned placement survived SQLite restart; spectator, other payer, invalid coordinate and duplicate claim rejected")
}

func TestStarfortRebalanceMixedAutoplayHTTP(t *testing.T) {
	tb := newHost(t, 2, starfort.Module())
	tb.start(nil)
	a := tb.people[0]
	peer := tb.people[1]
	a.act(map[string]any{"type": "build", "pad": 0, "tower": "arrow"}, 200)
	before := worldData(t, a.room())["objectives"].([]any)[0].(map[string]any)["id"]
	peer.command("autoplay", map[string]any{"enabled": true}, 200)
	wait(t, "new-rule teammate autoplay invests", func() bool { return len(data(a.room())["towers"].([]any)) > 1 })
	peer.command("autoplay", map[string]any{"enabled": false}, 200)
	if peer.room()["seats"].([]any)[1].(map[string]any)["autoPlay"] == true {
		t.Fatal("manual takeover failed")
	}
	if worldData(t, a.room())["objectives"].([]any)[0].(map[string]any)["id"] != before {
		t.Fatal("takeover rerolled task")
	}
	tb.record("manual player and explicitly enabled teammate autoplay share six pads; manual takeover preserves task and map")
}

// Explicit combat boundary fixture, driven by production timer and saved Match.
func rebalanceBoundary(t *testing.T, kind, branch string, enemies ...string) *rulesTable {
	b := specializationCase(t, kind, branch, enemies...)
	s := cadenceState(b)
	s.BalanceRules = 1
	cadenceSave(b, s)
	return b
}

func TestStarfortRebalanceControlBoundaries(t *testing.T) {
	t.Run("heavy-knockback-guard-per-tower", func(t *testing.T) {
		b := rebalanceBoundary(t, "arrow", "volley", "slime", "elite", "boss")
		s := cadenceState(b)
		s.SkillRNG = 10
		cadenceSave(b, s)
		b.timer(time.Second)
		after := cadenceState(b)
		heavyHits := 0
		for _, e := range after.Events {
			if e.Type == "knockback" && (e.TargetID == 2 || e.TargetID == 3) {
				heavyHits++
				if e.Amount != 20 {
					t.Fatal("heavy distance", e.Amount)
				}
			}
		}
		if heavyHits != 2 {
			t.Fatal("heavy knockback missing")
		}
		after.SkillRNG = 10
		after.Towers[0].Cooldown = 0
		cadenceSave(b, after)
		b.timer(time.Second)
		hits := 0
		for _, e := range cadenceState(b).Events {
			if e.Type == "knockback" && (e.TargetID == 2 || e.TargetID == 3) {
				hits++
			}
		}
		if hits != heavyHits {
			t.Fatal("same tower ignored eight-second guard")
		}
		s = cadenceState(b)
		s.TowerSerial = 2
		s.Towers[0].ID = 2
		s.SkillRNG = 10
		s.Towers[0].Cooldown = 0
		cadenceSave(b, s)
		b.timer(time.Second)
		hits = 0
		for _, e := range cadenceState(b).Events {
			if e.Type == "knockback" && e.TowerID == 2 && (e.TargetID == 2 || e.TargetID == 3) {
				hits++
			}
		}
		if hits != 2 {
			t.Fatal("different tower shared guard")
		}
	})
	t.Run("heavy-crater-quarter-time", func(t *testing.T) {
		b := rebalanceBoundary(t, "cannon", "speed", "slime", "elite", "boss", "slime")
		b.timer(time.Second)
		s := cadenceState(b)
		if len(s.Craters) != 1 || s.Craters[0].IgnoreHeavy {
			t.Fatal("new crater missing")
		}
		stopped := map[int]bool{}
		for _, e := range s.Events {
			if e.Type == "repair_start" {
				stopped[e.TargetID] = true
			}
		}
		if !stopped[2] || !stopped[3] {
			t.Fatal("heavy units bypass crater")
		}
		if s.Craters[0].Remaining != 0 || s.Craters[0].FilledMS-s.Craters[0].CreatedMS > 1000 {
			t.Fatal("quarter-time repair rate", s.Craters[0].Remaining)
		}
	})
	t.Run("freeze-and-meteor-timeline-state", func(t *testing.T) {
		b := rebalanceBoundary(t, "frost", "freeze", "slime", "elite", "boss")
		b.timer(time.Second)
		s := cadenceState(b)
		for _, e := range s.Enemies {
			if e.RecoveryUntil-e.FrozenUntil != 50 {
				t.Fatal("missing ten-second recovery")
			}
		}
		if s.Towers[0].PassiveReadyAt != 10100 {
			t.Fatal("freeze cooldown", s.Towers[0].PassiveReadyAt)
		}
		b = rebalanceBoundary(t, "arcane", "magic", "healer", "banner", "boss")
		b.timer(time.Second)
		s = cadenceState(b)
		chains, meteors := 0, 0
		for _, e := range s.Events {
			if e.Type == "chain" {
				chains++
			}
			if e.Type == "meteor" {
				meteors++
			}
		}
		if chains != 2 || meteors != 1 || s.Towers[0].ChainReadyAt == 0 {
			t.Fatal("magic chain and meteor must coexist", chains, meteors)
		}
	})
}

func TestStarfortRebalanceIceDurationExact(t *testing.T) {
	// Floating-point ceil must not turn the promised 2.4 seconds at level 14
	// into 2.6 seconds. Verify serialized hit targets through the module timer.
	for _, level := range []int{2, 3, 10, 14, 15, 100} {
		t.Run(fmt.Sprint(level), func(t *testing.T) {
			b := rebalanceBoundary(t, "frost", "speed", "slime")
			s := cadenceState(b)
			s.Towers[0].Level = level
			if level < 4 {
				s.Towers[0].Branch = ""
			}
			cadenceSave(b, s)
			b.timer(time.Second)
			for _, e := range cadenceState(b).Events {
				if e.Type == "hit" && e.Target != nil {
					if e.Target.SlowUntil-e.Tick != int64(5+level/2) {
						t.Fatal("slow duration rounded up by an extra physics step", level, e.Target.SlowUntil-e.Tick)
					}
				}
			}
		})
	}
}

func TestStarfortRebalanceTasksAndConvoy(t *testing.T) {
	t.Run("route-health-boost", func(t *testing.T) {
		b := newModuleRulesTable(t, 2, map[string]any{"mode": "endless"}, starfort.Module())
		s := cadenceState(b)
		if err := fixtureWorldStage(b.context(0), &s, 2); err != nil {
			t.Fatal(err)
		}
		s.World.Choice = nil
		s.World.BoonChoice = nil
		s.World.Objectives = []starfort.Objective{{ID: "risk", Kind: "mine", Label: "边界夹具", HP: 10, MaxHP: 10, Cost: 100, Reward: 120, Status: "available", Route: s.World.Map.Routes[0].ID, Sector: 1, X: 600, Y: 300}}
		s.WaveInfo = starfort.WaveInfo{Title: "路线加血边界夹具", Rule: "march", Total: 2}
		s.Players[0].Gold = 300
		cadenceSave(b, s)
		b.action(0, map[string]any{"type": "objective", "objectiveId": "risk"}, true)
		b.action(0, map[string]any{"type": "ready"}, true)
		b.action(1, map[string]any{"type": "ready"}, true)
		b.timer(time.Second)
		b.timer(time.Second)
		s = cadenceState(b)
		if len(s.Enemies) != 2 {
			t.Fatal("spawn fixture", len(s.Enemies))
		}
		if math.Abs(s.Enemies[0].MaxHP/s.Enemies[1].MaxHP-1.2) > 1e-7 {
			t.Fatal("boost leaked onto another road")
		}
	})
	for _, kind := range []string{"mine", "beacon", "convoy"} {
		t.Run(kind, func(t *testing.T) {
			b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, starfort.Module())
			s := cadenceState(b)
			hp, cost, reward := 10., 100, 120
			if kind == "beacon" {
				hp, reward = 4, 0
			}
			if kind == "convoy" {
				hp, cost, reward = 6, 300, 360
			}
			route := s.World.Map.Routes[0]
			s.World.Objectives = []starfort.Objective{{ID: "task-fixture", Kind: kind, Label: "明确标记的任务边界夹具", HP: hp, MaxHP: hp, Cost: cost, Reward: reward, Status: "available", Route: route.ID, Sector: 1, X: route.Points[0].X, Y: route.Points[0].Y}}
			if kind != "convoy" {
				s.World.Objectives[0].X = 600
				s.World.Objectives[0].Y = 300
			}
			cadenceSave(b, s)
			b.action(0, map[string]any{"type": "objective", "objectiveId": "task-fixture"}, true)
			b.action(0, map[string]any{"type": "ready"}, true)
			if kind == "convoy" {
				b.timer(time.Second)
				s = cadenceState(b)
				if s.WaveInfo.Spawned != 0 || s.World.Objectives[0].Progress != 20 {
					t.Fatal("convoy head start or speed", s.World.Objectives[0].Progress, s.WaveInfo.Spawned)
				}
				s.Enemies = nil
				s.WaveInfo.Total = 1
				s.WaveInfo.Spawned = 1
				cadenceSave(b, s)
				b.timer(time.Second)
				if cadenceState(b).Phase != "battle" {
					t.Fatal("wave ended before convoy arrived")
				}
				s = cadenceState(b)
				s.World.Objectives[0].Progress = fixturePathLength(route.Points) - 10
				s.BaseHP = 20
				cadenceSave(b, s)
			} else {
				s = cadenceState(b)
				s.Enemies = nil
				s.WaveInfo.Total = 1
				s.WaveInfo.Spawned = 1
				cadenceSave(b, s)
			}
			before := cadenceState(b).Players[0].Gold
			b.timer(time.Second)
			s = cadenceState(b)
			if s.World.TaskAttempts[kind] != 1 {
				t.Fatal("attempt count")
			}
			if kind == "beacon" {
				if s.World.BeaconSuccesses != 1 {
					t.Fatal("beacon reward")
				}
			} else if s.Players[0].Gold-before != reward+43 {
				t.Fatal("reward not exactly once", s.Players[0].Gold-before)
			}
			if kind == "convoy" && s.BaseHP != 24 {
				t.Fatal("convoy repair")
			}
			if kind == "mine" {
				if s.World.PadReward == nil {
					t.Fatal("missing placement claim")
				}
				b.action(0, map[string]any{"type": "ready"}, false)
				b.action(0, map[string]any{"type": "pad_reward", "x": -1, "y": 300}, false)
				b.action(0, map[string]any{"type": "pad_reward", "x": 100, "y": 80}, true)
				b.action(0, map[string]any{"type": "pad_reward", "x": 150, "y": 80}, false)
			}
		})
	}
}

func TestStarfortRebalanceTaskGrowthAndInactiveRoads(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, starfort.Module())
	seen := map[string]bool{}
	for wave := 1; wave <= 40; wave++ {
		s := cadenceState(b)
		// Preload prior paid attempts to exercise escalating costs without test
		// coins or pretending these milestone transitions are natural combat.
		s.World.TaskAttempts = map[string]int{"mine": 4, "beacon": 3, "convoy": 2}
		if err := fixtureClearedWave(b.context(0), &s, wave, true); err != nil {
			t.Fatal(err)
		}
		cadenceSave(b, s)
		if len(s.World.Objectives) != 1 {
			t.Fatal("one offer per wave")
		}
		o := s.World.Objectives[0]
		seen[o.Kind] = true
		active := false
		var route starfort.Route
		for _, r := range s.World.Map.Routes {
			if r.ID == o.Route {
				active = r.Active
				route = r
			}
		}
		if !active {
			t.Fatal("task offered on an inactive road")
		}
		n := s.World.TaskAttempts[o.Kind]
		baseHP, baseCost, growth := 10., 100., 1.2
		low, high := .7, .8
		if o.Kind == "beacon" {
			baseHP, growth, low, high = 4, 1.5, .5, .6
		}
		if o.Kind == "convoy" {
			baseHP, baseCost, growth, low, high = 6, 300, 2, 0, 0
		}
		if o.Cost != int(math.Round(baseCost*math.Pow(growth, float64(n)))) || math.Abs(o.MaxHP-baseHP*math.Pow(1.1, float64(n))) > 1e-7 {
			t.Fatal("task growth", o.Kind, o.Cost, o.MaxHP)
		}
		fraction := o.Progress / fixturePathLength(route.Points)
		if fraction < low-1e-8 || fraction > high+1e-8 {
			t.Fatal("task road position", o.Kind, fraction)
		}
		before := string(b.s.Data)
		b.view(0)
		if string(b.s.Data) != before {
			t.Fatal("view rerolled offer")
		}
	}
	if len(seen) != 3 {
		t.Fatal("random offers omitted a task type", seen)
	}
}

func TestStarfortRebalanceStartAndRestore(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			b := newModuleRulesTable(t, n, map[string]any{"waves": 20}, starfort.Module())
			raw := specializationRaw(b)
			if raw["balanceRules"] != float64(1) || b.s.Schema != 6 {
				t.Fatal("new balance rules not enabled", b.s.Schema)
			}
			s := cadenceState(b)
			if len(s.World.Map.Pads) != 6 {
				t.Fatal("initial road must have six pads", len(s.World.Map.Pads))
			}
			high := 0
			for _, p := range s.World.Map.Pads {
				if p.Terrain == "highground" {
					high++
				}
			}
			if high != 1 {
				t.Fatal("initial road must have one highground", high)
			}
			if len(s.World.Objectives) != 1 || s.World.Objectives[0].Status != "available" {
				t.Fatal("missing per-wave random offer")
			}
			o := s.World.Objectives[0]
			before := s.Players[0].Gold
			b.action(0, map[string]any{"type": "objective", "objectiveId": o.ID}, true)
			b.action(0, map[string]any{"type": "objective", "objectiveId": o.ID}, false)
			s = cadenceState(b)
			if s.Players[0].Gold != before-o.Cost || s.World.Objectives[0].Status != "active" {
				t.Fatal("task fee/state")
			}
			b.validate()
			for _, wave := range []int{6, 11, 16} {
				old := append([]starfort.Pad{}, s.World.Map.Pads...)
				if err := fixtureWorldStage(b.context(0), &s, 1+(wave-1)/5); err != nil {
					t.Fatal(err)
				}
				if len(s.World.Map.Pads) != 6*len(s.World.Map.Routes) {
					t.Fatal("expanded route pad count", wave, len(s.World.Map.Pads))
				}
				for i, p := range old {
					if s.World.Map.Pads[i] != p {
						t.Fatal("existing pad moved")
					}
				}
				cadenceSave(b, s)
			}
		})
	}
}

func TestStarfortRebalanceTaskHTTP(t *testing.T) {
	tb := newHost(t, 2, starfort.Module())
	tb.start(nil)
	a := tb.people[0]
	initial := data(a.room())
	if initial["balanceRules"] != float64(1) {
		t.Fatal("missing new rules in HTTP view")
	}
	offers := worldData(t, a.room())["objectives"].([]any)
	if len(offers) != 1 {
		t.Fatal("expected one random task")
	}
	o := offers[0].(map[string]any)
	before := gold(a.room(), 0)
	a.act(map[string]any{"type": "objective", "objectiveId": o["id"]}, 200)
	tb.people[1].act(map[string]any{"type": "objective", "objectiveId": o["id"]}, 400)
	for _, p := range tb.people {
		p.act(map[string]any{"type": "pause"}, 200)
	}
	tb.record("new per-wave offer accepted once; fee charged once; paused before restart")
	tb.restart()
	if gold(a.room(), 0) != before-o["cost"].(float64) {
		t.Fatal("restart changed fee")
	}
	after := worldData(t, a.room())["objectives"].([]any)[0].(map[string]any)
	if after["id"] != o["id"] || after["status"] != "active" {
		t.Fatal("restart rerolled accepted task")
	}
}

func TestStarfortRebalanceNaturalCampaign(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			b := newModuleRulesTable(t, n, map[string]any{"waves": 20}, starfort.Module())
			for sec := 0; sec < 20000 && b.s.Result == nil; sec++ {
				b.botPass()
				if b.s.Result == nil {
					b.timer(time.Second)
				}
			}
			if b.view(0).Phase != "won" {
				t.Fatal("natural campaign failed", b.view(0).CompletedWaves)
			}
			var s starfort.State
			if err := json.Unmarshal(b.s.Data, &s); err != nil {
				t.Fatal(err)
			}
			if len(s.World.Map.Pads) > 40 {
				t.Fatal("pad cap exceeded")
			}
		})
	}
}

func TestStarfortRebalanceNaturalModes(t *testing.T) {
	for _, difficulty := range []string{"easy", "hard"} {
		t.Run(difficulty, func(t *testing.T) {
			b := newModuleRulesTable(t, 2, map[string]any{"waves": 10, "difficulty": difficulty}, starfort.Module())
			for sec := 0; sec < 12000 && b.s.Result == nil; sec++ {
				b.botPass()
				if b.s.Result == nil {
					b.timer(time.Second)
				}
			}
			if b.view(0).Phase != "won" {
				t.Fatal("natural difficulty campaign failed", b.view(0).CompletedWaves)
			}
		})
	}
	t.Run("endless", func(t *testing.T) {
		b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, starfort.Module())
		for sec := 0; sec < 20000 && b.s.Result == nil && b.view(0).CompletedWaves < 22; sec++ {
			b.botPass()
			if b.s.Result == nil {
				b.timer(time.Second)
			}
		}
		if b.view(0).CompletedWaves < 22 || b.s.Result != nil {
			t.Fatal("endless did not continue beyond campaign limit", b.view(0).CompletedWaves)
		}
	})
}
