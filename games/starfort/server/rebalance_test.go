package starfort

// Isolation failure inventory: see REBALANCE.md. Probability seeds and combat
// states below are explicit boundary fixtures, not natural campaign evidence.
import (
	"math"
	"testing"
)

func rebalanceCase(t *testing.T, kind, branch string) *State {
	s := iceArrowCase(t, kind)
	s.BalanceRules = 1
	s.SpecializationRules = 1
	s.Towers[0].Branch = branch
	if branch != "" {
		s.Towers[0].Level = 4
	}
	return s
}
func TestRebalanceNoDefaultPassives(t *testing.T) {
	for _, kind := range []string{"arrow", "cannon", "frost", "arcane"} {
		s := rebalanceCase(t, kind, "")
		s.fireTower(0)
		s.fireTower(0)
		s.fireTower(0)
		if len(s.IceWalls) != 0 || len(s.Craters) != 0 {
			t.Fatal("obsolete passive", kind)
		}
		if s.passiveView(s.Towers[0]) != nil {
			t.Fatal("obsolete skill description", kind)
		}
		if kind == "arrow" && math.Abs(s.Enemies[0].HP-(10000-66)) > 1e-7 {
			t.Fatal("focus bonus remains")
		}
		if kind == "arcane" && s.Enemies[1].HP != 10000 {
			t.Fatal("default chain remains")
		}
	}
}
func TestRebalanceTowerStats(t *testing.T) {
	for _, tc := range []struct {
		kind, branch string
		damage, rate float64
	}{
		{"arrow", "volley", 1, .7}, {"arrow", "heart", 1, 1.7},
		{"cannon", "power", 1.5, 1}, {"cannon", "speed", .5, 1.5},
		{"frost", "freeze", 1, .5}, {"frost", "speed", 1, 1.5},
		{"arcane", "power", 1.2, 1}, {"arcane", "magic", 1, 1},
	} {
		s := rebalanceCase(t, tc.kind, tc.branch)
		tower := s.Towers[0]
		c, _ := spec(tc.kind)
		damage, _, interval := s.towerStats(tower)
		wantDamage := c.Damage * (1 + .65*3) * tc.damage
		wantInterval := max(200, int(float64(c.Interval)/(1.12*tc.rate)))
		if math.Abs(damage-wantDamage) > 1e-7 || interval != wantInterval {
			t.Fatalf("%s/%s: damage %g/%g interval %d/%d", tc.kind, tc.branch, damage, wantDamage, interval, wantInterval)
		}
	}
}
func TestRebalanceIceGrowthRecoveryAndShield(t *testing.T) {
	for _, level := range []int{1, 2, 3, 4, 10} {
		s := rebalanceCase(t, "frost", "")
		s.Towers[0].Level = level
		s.fireTower(0)
		wantSlow := .1 * (1 + .2*float64((level-1)/2))
		if math.Abs(s.iceSlowFactor(s.Enemies[0])-(1-wantSlow)) > 1e-7 {
			t.Fatal("ice slow growth", level)
		}
		ticks := int64(math.Ceil((1 + .2*float64(level/2)) * 1000 / StepMS))
		if s.Enemies[0].SlowUntil != s.Tick+ticks {
			t.Fatal("ice duration growth", level)
		}
	}
	s := rebalanceCase(t, "frost", "freeze")
	s.SkillRNG = 5
	s.fireTower(0)
	if s.Towers[0].PassiveReadyAt != 10200 {
		t.Fatal("freeze CD")
	}
	e := s.Enemies[0]
	s.Tick = e.FrozenUntil
	e.SlowUntil = 0
	if s.iceSlowFactor(e) != 0 {
		t.Fatal("recovery did not start at zero")
	}
	s.Tick += 25
	if math.Abs(s.iceSlowFactor(e)-.5) > 1e-7 {
		t.Fatal("recovery midpoint")
	}
	s.Tick += 25
	if s.iceSlowFactor(e) != 1 {
		t.Fatal("recovery never completed")
	}
	for _, kind := range []string{"shield", "elite", "boss"} {
		s := rebalanceCase(t, "arcane", "power")
		s.Enemies = s.Enemies[:1]
		s.Enemies[0].Kind = kind
		s.Enemies[0].Shield = 10000
		before := s.Enemies[0].Shield
		d, _, _ := s.towerStats(s.Towers[0])
		s.fireTower(0)
		factor := 1.5 * (1.2 + .1*3)
		if kind != "shield" {
			factor = 1.5 * (1.1 + .1*3)
		}
		if math.Abs(before-s.Enemies[0].Shield-d*factor) > 1e-7 {
			t.Fatal("shield scaling", kind)
		}
	}
}
func TestRebalanceTaskScalingAndPlacement(t *testing.T) {
	s := centralCase(t, 1)
	s.BalanceRules = 1
	s.SpecializationRules = 1
	s.World.Objectives = nil
	s.World.TaskAttempts = map[string]int{}
	s.World.TaskOffersWave = 0
	s.offerWaveTask()
	if len(s.World.Objectives) != 1 {
		t.Fatal("no offer")
	}
	o := s.World.Objectives[0]
	s.Players[0].Gold = 100000
	if err := s.acceptObjective(o.ID, 0); err != nil {
		t.Fatal(err)
	}
	if s.World.TaskAttempts[o.Kind] != 1 {
		t.Fatal("attempt counter")
	}
	// Mine success produces an owned, durable placement claim; hostile requests
	// cannot consume it, and the second success upgrades an existing pad.
	s.World.Objectives[0].Kind = "mine"
	s.World.Objectives[0].Reward = 120
	s.finishObjective(0, true, "")
	if s.World.PadReward == nil || s.World.PadReward.Kind != "add" {
		t.Fatal("missing pad reward")
	}
	before := len(s.World.Map.Pads)
	if err := s.claimPad(1, Point{400, 400}, 0); err == nil {
		t.Fatal("other player placed")
	}
	if err := s.claimPad(0, Point{math.NaN(), 400}, 0); err == nil {
		t.Fatal("nonfinite placement")
	}
	if err := s.claimPad(0, Point{400, 400}, 0); err != nil {
		t.Fatal(err)
	}
	if len(s.World.Map.Pads) != before+1 || s.World.PadReward != nil {
		t.Fatal("placement claim")
	}
	if err := s.claimPad(0, Point{500, 400}, 0); err == nil {
		t.Fatal("duplicate placement")
	}
	s.World.Objectives[0].Status = "active"
	s.finishObjective(0, true, "")
	if s.World.PadReward == nil || s.World.PadReward.Kind != "highground" {
		t.Fatal("alternating reward")
	}
}
