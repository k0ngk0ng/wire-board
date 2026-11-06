package pvz_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
)

func fullPlantWorld(t *testing.T, scene string) *engine.State {
	t.Helper()
	s, e := engine.New(engine.Options{Scene: scene, Mode: "duel", RoundSeconds: 600, PrepSeconds: 10}, 2, "full-plant-"+t.Name())
	if e != nil {
		t.Fatal(e)
	}
	if e = engine.Step(&s, 200); e != nil {
		t.Fatal(e)
	}
	s.Players[0].Money = 100000
	s.Players[1].Money = 100000
	return &s
}
func fullPlantFixture(t *testing.T, s *engine.State, kind string, row, col int) int {
	t.Helper()
	c, ok := engine.PlantSpec(kind)
	if !ok {
		t.Fatal("missing plant", kind)
	}
	s.NextID++
	p := engine.Plant{ID: s.NextID, Owner: 0, Kind: kind, Row: row, Col: col, HP: c.HP, MaxHP: c.HP, Layer: c.Layer, BornAt: s.Tick, ReadyAt: s.Tick + 1, ReadyMS: (s.Tick + 1) * engine.TickMS, AttackAt: s.Tick + 1, AttackMS: (s.Tick + 1) * engine.TickMS, State: "ready", Flags: map[string]int64{}}
	p.Asleep = c.Mushroom && !s.Map.Night
	if kind == "spikerock" {
		p.Uses = 9
	}
	if kind == "cobcannon" {
		p.ReadyAt = s.Tick
		p.ChargeMS = s.Tick * engine.TickMS
	}
	s.Plants = append(s.Plants, p)
	return p.ID
}
func fullPlantZombieFixture(t *testing.T, s *engine.State, kind string, row, x int) int {
	t.Helper()
	c, ok := engine.ZombieSpec(kind)
	if !ok {
		t.Fatal("missing zombie", kind)
	}
	s.NextID++
	z := engine.Zombie{ID: s.NextID, Owner: 1, Kind: kind, Row: row, X: x, HP: c.HP, MaxHP: c.HP, Armor: c.Armor, MaxArmor: c.Armor, State: "walking", BornAt: s.Tick, SpeedMilli: 8000, SpecialSpeedMilli: 19600, Allegiance: "zombies", Flags: map[string]int64{}}
	if kind == "balloon" {
		z.Altitude = 1
	}
	s.Zombies = append(s.Zombies, z)
	return z.ID
}
func fullPlantByID(s *engine.State, id int) *engine.Plant {
	for i := range s.Plants {
		if s.Plants[i].ID == id {
			return &s.Plants[i]
		}
	}
	return nil
}
func fullPlantZombieByID(s *engine.State, id int) *engine.Zombie {
	for i := range s.Zombies {
		if s.Zombies[i].ID == id {
			return &s.Zombies[i]
		}
	}
	return nil
}
func fullPlantRecord(t *testing.T, s *engine.State, label string) {
	t.Helper()
	out := filepath.Join("..", "..", "..", ".local", "pvz", "full-plants")
	if e := os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "authoritative full-world combat fixture; money/positions deliberately set to reach each special behavior; not a natural economy game", "label": label, "digest": engine.Digest(*s), "final": s}, "", "  ")
	name := filepath.Base(t.Name()) + ".json"
	if e := os.WriteFile(filepath.Join(out, name), raw, 0600); e != nil {
		t.Fatal(e)
	}
}
func fullPlantStep(t *testing.T, s *engine.State, n int) {
	t.Helper()
	if e := engine.Step(s, n); e != nil {
		t.Fatal(e)
	}
}

func TestPVZFullPlantsProjectileFamilies(t *testing.T) {
	for _, kind := range []string{"peashooter", "snowpea", "repeater", "gatlingpea", "puffshroom", "seashroom", "scaredyshroom", "cabbagepult", "kernelpult", "melonpult", "wintermelon"} {
		t.Run(kind, func(t *testing.T) {
			scene := "night"
			row := 1
			if kind == "seashroom" {
				scene = "fog"
				row = 2
			}
			s := fullPlantWorld(t, scene)
			fullPlantFixture(t, s, kind, row, 1)
			targetX := 7000
			if kind == "puffshroom" || kind == "seashroom" {
				targetX = 4000
			}
			z := fullPlantZombieFixture(t, s, "buckethead", row, targetX)
			hp := fullPlantZombieByID(s, z).HP + fullPlantZombieByID(s, z).Armor
			fullPlantStep(t, s, 200)
			after := fullPlantZombieByID(s, z)
			if after != nil && after.HP+after.Armor >= hp {
				t.Fatal("plant never hit forward target")
			}
			if kind == "snowpea" || kind == "wintermelon" {
				if after != nil && after.SlowUntil <= s.Tick {
					t.Fatal("cold shot omitted slow")
				}
			}
			fullPlantRecord(t, s, "forward fire, trajectory, armor and status")
		})
	}
}

func TestPVZFullPlantsMultidirectionalAndAir(t *testing.T) {
	t.Run("threepeater", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		fullPlantFixture(t, s, "threepeater", 2, 1)
		ids := []int{}
		for _, r := range []int{1, 2, 3} {
			ids = append(ids, fullPlantZombieFixture(t, s, "buckethead", r, 6000))
		}
		fullPlantStep(t, s, 80)
		for _, id := range ids {
			z := fullPlantZombieByID(s, id)
			if z == nil || z.Armor >= 1100 {
				t.Fatal("threepeater missed adjacent row")
			}
		}
		fullPlantRecord(t, s, "three simultaneous lanes")
	})
	t.Run("splitpea", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		fullPlantFixture(t, s, "splitpea", 2, 4)
		rear := fullPlantZombieFixture(t, s, "buckethead", 2, 2000)
		front := fullPlantZombieFixture(t, s, "buckethead", 2, 8000)
		fullPlantStep(t, s, 80)
		for _, id := range []int{rear, front} {
			z := fullPlantZombieByID(s, id)
			if z == nil || z.Armor >= 1100 {
				t.Fatal("split head did not fire both directions")
			}
		}
		fullPlantRecord(t, s, "front single and rear repeat")
	})
	t.Run("starfruit", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		fullPlantFixture(t, s, "starfruit", 2, 4)
		fullPlantZombieFixture(t, s, "buckethead", 2, 2500)
		fullPlantStep(t, s, 15)
		directions := map[string]bool{}
		for _, p := range s.Projectiles {
			if p.Kind == "star" {
				directions[fmt.Sprintf("%d,%d", p.VX, p.VY)] = true
			}
		}
		if len(directions) != 5 {
			t.Fatalf("starfruit emitted %d directions", len(directions))
		}
		fullPlantRecord(t, s, "five two-dimensional trajectories")
	})
	for _, kind := range []string{"cactus", "cattail"} {
		t.Run(kind, func(t *testing.T) {
			scene := "day"
			row := 2
			targetRow := 2
			if kind == "cattail" {
				scene = "pool"
				targetRow = 0
			}
			s := fullPlantWorld(t, scene)
			fullPlantFixture(t, s, kind, row, 2)
			id := fullPlantZombieFixture(t, s, "balloon", targetRow, 6000)
			fullPlantStep(t, s, 100)
			z := fullPlantZombieByID(s, id)
			if z != nil && z.Altitude > 0 {
				t.Fatal("air counter did not pop balloon")
			}
			fullPlantRecord(t, s, "balloon targeting and homing")
		})
	}
}

func TestPVZFullPlantsMushroomBehavior(t *testing.T) {
	t.Run("sleep-and-scare", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		fullPlantFixture(t, s, "puffshroom", 1, 3)
		fullPlantZombieFixture(t, s, "zombie", 1, 6000)
		fullPlantStep(t, s, 60)
		if len(s.Projectiles) > 0 {
			t.Fatal("sleeping mushroom shot")
		}
		s = fullPlantWorld(t, "night")
		id := fullPlantFixture(t, s, "scaredyshroom", 2, 3)
		fullPlantZombieFixture(t, s, "zombie", 1, 4000)
		fullPlantZombieFixture(t, s, "zombie", 2, 6000)
		fullPlantStep(t, s, 40)
		p := fullPlantByID(s, id)
		if p == nil || p.State != "scared" {
			t.Fatal("scaredy did not hide")
		}
		for _, e := range s.Events {
			if e.Type == "shoot" && e.Owner == 0 {
				t.Fatal("scared mushroom fired")
			}
		}
		fullPlantRecord(t, s, "sleep and adjacent-lane fear")
	})
	for _, kind := range []string{"fumeshroom", "gloomshroom"} {
		t.Run(kind, func(t *testing.T) {
			s := fullPlantWorld(t, "night")
			fullPlantFixture(t, s, kind, 2, 3)
			secondX := 5600
			if kind == "gloomshroom" {
				secondX = 4400
			}
			ids := []int{fullPlantZombieFixture(t, s, "screendoor", 2, 4800), fullPlantZombieFixture(t, s, "screendoor", 2, secondX)}
			if kind == "gloomshroom" {
				ids = append(ids, fullPlantZombieFixture(t, s, "zombie", 1, 4000))
			}
			fullPlantStep(t, s, 55)
			for _, id := range ids {
				z := fullPlantZombieByID(s, id)
				if z != nil && z.HP >= z.MaxHP {
					t.Fatal("piercing area did not reach body")
				}
			}
			fullPlantRecord(t, s, "piercing fume/gloom multi-target")
		})
	}
	t.Run("ice-and-doom", func(t *testing.T) {
		s := fullPlantWorld(t, "night")
		fullPlantFixture(t, s, "iceshroom", 1, 3)
		id := fullPlantZombieFixture(t, s, "buckethead", 4, 7000)
		fullPlantStep(t, s, 25)
		z := fullPlantZombieByID(s, id)
		if z == nil || z.Flags["frozenUntil"] <= s.Tick || z.SlowUntil <= s.Tick {
			t.Fatal("ice mushroom did not freeze whole board")
		}
		s = fullPlantWorld(t, "night")
		fullPlantFixture(t, s, "doomshroom", 2, 4)
		id = fullPlantZombieFixture(t, s, "buckethead", 2, 6000)
		fullPlantStep(t, s, 25)
		if fullPlantZombieByID(s, id) != nil || len(s.Craters) != 1 {
			t.Fatal("doom did not explode and leave crater")
		}
		fullPlantRecord(t, s, "whole-board ice and crater-producing explosion")
	})
	t.Run("gravebuster", func(t *testing.T) {
		s := fullPlantWorld(t, "night")
		s.Graves = []engine.Grave{{ID: s.NextID + 1, Row: 2, Col: 5, HP: 500}}
		s.NextID++
		id := fullPlantFixture(t, s, "gravebuster", 2, 5)
		fullPlantStep(t, s, 150)
		if len(s.Graves) != 0 || fullPlantByID(s, id) != nil {
			t.Fatal("gravebuster did not consume grave and itself")
		}
		fullPlantRecord(t, s, "grave consumption")
	})
}

func TestPVZFullPlantsEconomyAndMagnets(t *testing.T) {
	for _, kind := range []string{"sunflower", "twinsunflower", "sunshroom", "marigold"} {
		t.Run(kind, func(t *testing.T) {
			s := fullPlantWorld(t, "night")
			fullPlantFixture(t, s, kind, 1, 1)
			fullPlantStep(t, s, 2)
			total := 0
			for _, p := range s.Pickups {
				if p.Owner != 0 {
					t.Fatal("wrong pickup owner")
				}
				total += p.Amount
				if kind == "marigold" && p.Kind == "sun" {
					t.Fatal("marigold generated sunlight instead of coin")
				}
			}
			want := 25
			if kind == "twinsunflower" {
				want = 50
			}
			if kind == "sunshroom" {
				want = 15
			}
			if kind == "marigold" {
				want = 1
			}
			if total < want {
				t.Fatalf("production missing: %d want >=%d", total, want)
			}
			fullPlantRecord(t, s, "owned resource pickup")
		})
	}
	t.Run("magnet-metal-only", func(t *testing.T) {
		s := fullPlantWorld(t, "night")
		fullPlantFixture(t, s, "magnetshroom", 2, 4)
		metal := fullPlantZombieFixture(t, s, "buckethead", 2, 6500)
		plastic := fullPlantZombieFixture(t, s, "conehead", 2, 5000)
		fullPlantStep(t, s, 10)
		if fullPlantZombieByID(s, metal).Armor != 0 || fullPlantZombieByID(s, plastic).Armor != 370 {
			t.Fatal("magnet confused metal and plastic")
		}
		fullPlantRecord(t, s, "metal armor removal and cooldown")
	})
	t.Run("goldmagnet-owner", func(t *testing.T) {
		s := fullPlantWorld(t, "night")
		fullPlantFixture(t, s, "goldmagnet", 2, 4)
		before := s.Players[0].Money
		s.Pickups = []engine.Pickup{{ID: s.NextID + 1, Owner: 0, Row: 1, X: 6000, Y: 1500, Kind: "coin", Amount: 5, AutoAt: s.Tick + 600, ExpiresAt: s.Tick + 1200}, {ID: s.NextID + 2, Owner: 1, Row: 1, X: 6000, Y: 1500, Kind: "coin", Amount: 5, AutoAt: s.Tick + 600, ExpiresAt: s.Tick + 1200}}
		s.NextID += 2
		fullPlantStep(t, s, 10)
		if s.Players[0].Money != before+5 || len(s.Pickups) != 1 || s.Pickups[0].Owner != 1 {
			t.Fatal("goldmagnet failed owner-only collection")
		}
		fullPlantRecord(t, s, "coin collection retains wallet ownership")
	})
}

func TestPVZFullPlantsContactAndInstant(t *testing.T) {
	t.Run("chomper-digest-and-giant", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		id := fullPlantFixture(t, s, "chomper", 2, 4)
		victim := fullPlantZombieFixture(t, s, "buckethead", 2, 5200)
		fullPlantStep(t, s, 25)
		if fullPlantZombieByID(s, victim) != nil || fullPlantByID(s, id).State != "digesting" {
			t.Fatal("chomper failed swallow/digestion")
		}
		another := fullPlantZombieFixture(t, s, "zombie", 2, 5200)
		fullPlantStep(t, s, 30)
		if fullPlantZombieByID(s, another) == nil {
			t.Fatal("digesting chomper swallowed again")
		}
		s = fullPlantWorld(t, "day")
		fullPlantFixture(t, s, "chomper", 2, 4)
		giant := fullPlantZombieFixture(t, s, "gargantuar", 2, 5200)
		fullPlantStep(t, s, 25)
		z := fullPlantZombieByID(s, giant)
		if z == nil || z.HP != z.MaxHP-40 {
			t.Fatal("chomper should bite giant for 40, not swallow")
		}
		fullPlantRecord(t, s, "swallow versus heavy target")
	})
	for _, kind := range []string{"cherrybomb", "potatomine", "squash", "jalapeno", "tanglekelp"} {
		t.Run(kind, func(t *testing.T) {
			scene := "day"
			row := 2
			zk := "zombie"
			if kind == "tanglekelp" {
				scene = "pool"
				zk = "snorkel"
			}
			s := fullPlantWorld(t, scene)
			fullPlantFixture(t, s, kind, row, 4)
			x := 5100
			if kind == "potatomine" {
				x = 4500
			}
			id := fullPlantZombieFixture(t, s, zk, row, x)
			if zk == "snorkel" {
				fullPlantZombieByID(s, id).Diving = true
			}
			fullPlantStep(t, s, 70)
			if fullPlantZombieByID(s, id) != nil {
				t.Fatal("instant/contact plant failed to remove close target")
			}
			fullPlantRecord(t, s, "single-use trigger and damage")
		})
	}
	t.Run("hypno-and-garlic", func(t *testing.T) {
		s := fullPlantWorld(t, "night")
		id := fullPlantFixture(t, s, "hypnoshroom", 2, 4)
		fullPlantByID(s, id).HP = 4
		victim := fullPlantZombieFixture(t, s, "zombie", 2, 4900)
		fullPlantStep(t, s, 2)
		z := fullPlantZombieByID(s, victim)
		if z == nil || z.Allegiance != "plants" || z.Owner != 1 {
			t.Fatal("hypnosis did not convert allegiance while preserving owner")
		}
		s = fullPlantWorld(t, "day")
		fullPlantFixture(t, s, "garlic", 2, 4)
		victim = fullPlantZombieFixture(t, s, "zombie", 2, 4900)
		fullPlantStep(t, s, 2)
		if fullPlantZombieByID(s, victim).Row == 2 {
			t.Fatal("garlic did not redirect bitten zombie")
		}
		fullPlantRecord(t, s, "bite-triggered allegiance and lane change")
	})
}

func TestPVZFullPlantsGroundTorchAndFog(t *testing.T) {
	for _, kind := range []string{"spikeweed", "spikerock"} {
		t.Run(kind, func(t *testing.T) {
			s := fullPlantWorld(t, "day")
			fullPlantFixture(t, s, kind, 2, 4)
			id := fullPlantZombieFixture(t, s, "buckethead", 2, 4500)
			hp := fullPlantZombieByID(s, id).HP
			fullPlantStep(t, s, 20)
			z := fullPlantZombieByID(s, id)
			want := 20
			if kind == "spikerock" {
				want = 40
			}
			if z == nil || hp-z.HP < want {
				t.Fatalf("spike damage missing: %+v", z)
			}
			fullPlantRecord(t, s, "ground-only periodic piercing damage")
		})
	}
	t.Run("torchwood", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		fullPlantFixture(t, s, "peashooter", 2, 0)
		fullPlantFixture(t, s, "torchwood", 2, 2)
		id := fullPlantZombieFixture(t, s, "buckethead", 2, 4500)
		fullPlantStep(t, s, 55)
		z := fullPlantZombieByID(s, id)
		if z == nil || 1100-z.Armor < 80 {
			t.Fatal("torch did not double pea damage")
		}
		fullPlantRecord(t, s, "pea to fire conversion")
	})
	t.Run("blover-and-plantern", func(t *testing.T) {
		s := fullPlantWorld(t, "fog")
		fullPlantFixture(t, s, "plantern", 1, 5)
		fullPlantFixture(t, s, "blover", 1, 2)
		id := fullPlantZombieFixture(t, s, "balloon", 4, 8000)
		fullPlantStep(t, s, 30)
		if fullPlantZombieByID(s, id) != nil || s.FogClearedUntil <= s.Tick {
			t.Fatal("blover failed balloon/fog removal")
		}
		fullPlantRecord(t, s, "fog clearing and aerial removal")
	})
}

func TestPVZFullPlantsCobAndHostileProjectiles(t *testing.T) {
	t.Run("cob-authority-delay-reload", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		id := fullPlantFixture(t, s, "cobcannon", 2, 1)
		victim := fullPlantZombieFixture(t, s, "buckethead", 2, 7000)
		if e := s.UsePlantAbility(1, id, 2, 6); e == nil {
			t.Fatal("other side fired cannon")
		}
		if e := s.UsePlantAbility(0, id, 2, 6); e != nil {
			t.Fatal(e)
		}
		if fullPlantZombieByID(s, victim) == nil {
			t.Fatal("cannon hit before flight")
		}
		if e := s.UsePlantAbility(0, id, 2, 6); e == nil {
			t.Fatal("cannon ignored reload")
		}
		fullPlantStep(t, s, 120)
		if fullPlantZombieByID(s, victim) != nil {
			t.Fatal("cannon explosion failed")
		}
		fullPlantRecord(t, s, "owned active cannon, flight and reload")
	})
	t.Run("umbrella-and-friendly-owner", func(t *testing.T) {
		s := fullPlantWorld(t, "roof")
		id := fullPlantFixture(t, s, "sunflower", 2, 2)
		fullPlantFixture(t, s, "umbrellaleaf", 2, 3)
		s.NextID++
		s.Projectiles = append(s.Projectiles, engine.Projectile{ID: s.NextID, Owner: 1, Row: 2, X: 7000, Y: 2500, Kind: "basketball", Damage: 75, TargetX: 2500, TargetY: 2500, LandAt: s.Tick + 10, Payload: "hostile", BornAt: s.Tick})
		fullPlantStep(t, s, 12)
		if fullPlantByID(s, id).HP != 300 {
			t.Fatal("umbrella did not block basketball")
		}
		victim := fullPlantZombieFixture(t, s, "zombie", 1, 4000)
		s.NextID++
		s.Projectiles = append(s.Projectiles, engine.Projectile{ID: s.NextID, Owner: 1, Row: 1, X: 3500, Y: 1500, VX: 250, Kind: "zombiepea", Damage: 20, Payload: "friendly", BornAt: s.Tick})
		fullPlantStep(t, s, 5)
		z := fullPlantZombieByID(s, victim)
		if z == nil || z.HP >= 270 {
			t.Fatal("hypnotized shot used original owner's side")
		}
		fullPlantRecord(t, s, "umbrella defense and converted projectile allegiance")
	})
}

func TestPVZFullPlantsAerialAndLaneCounterBoundaries(t *testing.T) {
	t.Run("cactus-over-ground-screen", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		fullPlantFixture(t, s, "cactus", 2, 2)
		fullPlantZombieFixture(t, s, "buckethead", 2, 3500)
		id := fullPlantZombieFixture(t, s, "balloon", 2, 6000)
		fullPlantStep(t, s, 100)
		z := fullPlantZombieByID(s, id)
		if z != nil && z.Altitude > 0 {
			t.Fatal("ground screen intercepted high cactus needle")
		}
		fullPlantRecord(t, s, "cactus high needles bypass ground screen")
	})
	t.Run("blover-preserves-bungee", func(t *testing.T) {
		s := fullPlantWorld(t, "fog")
		fullPlantFixture(t, s, "blover", 1, 1)
		id := fullPlantZombieFixture(t, s, "bungee", 4, 8000)
		z := fullPlantZombieByID(s, id)
		z.Altitude = 600
		z.State = "descending"
		z.AbilityUntil = s.Tick + 100
		fullPlantStep(t, s, 5)
		if fullPlantZombieByID(s, id) == nil {
			t.Fatal("blover incorrectly killed descending bungee")
		}
		fullPlantRecord(t, s, "flying balloon counter excludes bungee descent")
	})
	t.Run("jalapeno-removes-own-row-ladders", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		body := fullPlantFixture(t, s, "wallnut", 2, 4)
		fullPlantFixture(t, s, "jalapeno", 2, 2)
		s.Ladders = []engine.Ladder{{Row: 2, Col: 4, PlantID: body, Owner: 1}, {Row: 3, Col: 4, PlantID: body, Owner: 1}}
		fullPlantStep(t, s, 5)
		for _, l := range s.Ladders {
			if l.Row == 2 {
				t.Fatal("burned row retained placed ladder")
			}
		}
		fullPlantRecord(t, s, "row burn removes attached ladders")
	})
}

func TestPVZFullPlantsBurstCountsAndSplash(t *testing.T) {
	for _, tc := range []struct {
		kind         string
		count, ticks int
	}{{"repeater", 2, 20}, {"gatlingpea", 4, 20}, {"gloomshroom", 4, 36}, {"cattail", 2, 20}} {
		t.Run(tc.kind+"-one-cycle", func(t *testing.T) {
			scene := "night"
			row := 2
			if tc.kind == "cattail" {
				scene = "fog"
			}
			s := fullPlantWorld(t, scene)
			id := fullPlantFixture(t, s, tc.kind, row, 2)
			zid := fullPlantZombieFixture(t, s, "gargantuar", row, 3700)
			z := fullPlantZombieByID(s, zid)
			z.Flags["frozenUntil"] = s.Tick + 1000
			fullPlantStep(t, s, tc.ticks)
			count := 0
			for _, e := range s.Events {
				if e.Type == "shoot" && e.Owner == 0 && e.Target == id {
					count++
				}
			}
			if count != tc.count {
				t.Fatalf("%s shot count=%d want=%d", tc.kind, count, tc.count)
			}
			fullPlantRecord(t, s, "single attack-cycle burst count")
		})
	}
	for _, kind := range []string{"melonpult", "wintermelon"} {
		t.Run(kind+"-splash", func(t *testing.T) {
			s := fullPlantWorld(t, "day")
			fullPlantFixture(t, s, kind, 2, 1)
			ids := []int{}
			for _, row := range []int{1, 2, 3} {
				id := fullPlantZombieFixture(t, s, "gargantuar", row, 6500)
				fullPlantZombieByID(s, id).Flags["frozenUntil"] = s.Tick + 1000
				ids = append(ids, id)
			}
			fullPlantStep(t, s, 100)
			for _, id := range ids {
				z := fullPlantZombieByID(s, id)
				if z == nil || z.MaxHP-z.HP < 26 {
					t.Fatal("melon splash missed nearby row")
				}
				if kind == "wintermelon" && z.SlowUntil <= s.Tick {
					t.Fatal("winter melon splash omitted cold")
				}
			}
			fullPlantRecord(t, s, "three-row splash and secondary cold")
		})
	}
}
func TestPVZFullPlantsButterGrowthAndVehicleDurability(t *testing.T) {
	t.Run("kernel-butter-and-kernel", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		id := fullPlantFixture(t, s, "kernelpult", 2, 1)
		zid := fullPlantZombieFixture(t, s, "gargantuar", 2, 6500)
		z := fullPlantZombieByID(s, zid)
		z.Flags["frozenUntil"] = s.Tick + 3000
		seen := map[string]bool{}
		butter := false
		for range 120 {
			fullPlantStep(t, s, 20)
			for _, e := range s.Events {
				if e.Type == "shoot" && e.Target == id {
					seen[e.Kind] = true
				}
			}
			z = fullPlantZombieByID(s, zid)
			if z != nil && z.Flags["butterUntil"] > s.Tick {
				remaining := z.Flags["butterUntil"] - s.Tick
				if remaining > 80 {
					t.Fatal("butter exceeds four-second timer")
				}
				butter = true
			}
			if seen["butter"] && seen["kernel"] && butter {
				break
			}
		}
		if !seen["butter"] || !seen["kernel"] || !butter {
			t.Fatalf("kernel random payload or butter stun missing: %+v", seen)
		}
		fullPlantRecord(t, s, "deterministic kernel/quarter-chance butter and timed stun")
	})
	t.Run("sunshroom-growth", func(t *testing.T) {
		s := fullPlantWorld(t, "night")
		id := fullPlantFixture(t, s, "sunshroom", 1, 1)
		fullPlantStep(t, s, 2400)
		p := fullPlantByID(s, id)
		if p == nil || p.State != "grown" {
			t.Fatal("sunshroom did not mature after 120 seconds")
		}
		large := false
		for range 101 {
			for _, v := range s.Pickups {
				if v.Owner == 0 && v.Kind == "sun" && v.Amount == 25 {
					large = true
				}
			}
			if large {
				break
			}
			fullPlantStep(t, s, 5)
		}
		if !large {
			t.Fatal("grown sunshroom did not produce full sun")
		}
		fullPlantRecord(t, s, "small to grown production cycle")
	})
	for _, kind := range []string{"spikeweed", "spikerock"} {
		t.Run(kind+"-vehicle", func(t *testing.T) {
			s := fullPlantWorld(t, "day")
			id := fullPlantFixture(t, s, kind, 1, 4)
			vehicle := fullPlantZombieFixture(t, s, "zomboni", 1, 4500)
			fullPlantStep(t, s, 20)
			if fullPlantZombieByID(s, vehicle) != nil {
				t.Fatal("spikes failed to burst vehicle tires with lethal damage")
			}
			p := fullPlantByID(s, id)
			if kind == "spikeweed" && p != nil {
				t.Fatal("ordinary spikes survived vehicle")
			}
			if kind == "spikerock" && (p == nil || p.Uses != 8 || p.HP != 400) {
				t.Fatalf("spikerock durability not consumed once: %+v", p)
			}
			fullPlantRecord(t, s, "vehicle damage and distinct ground-plant durability")
		})
	}
}
func TestPVZFullPlantsBossCounterAndReplayBoundaries(t *testing.T) {
	t.Run("ice-fire-balls", func(t *testing.T) {
		s := fullPlantWorld(t, "night")
		fullPlantFixture(t, s, "iceshroom", 1, 1)
		s.NextID++
		s.Projectiles = append(s.Projectiles, engine.Projectile{ID: s.NextID, Owner: 1, Row: 3, X: 8500, Y: 3500, VX: -125, Kind: "bossfire", Damage: 100000, Payload: "hostile", BornAt: s.Tick})
		fullPlantStep(t, s, 2)
		for _, v := range s.Projectiles {
			if v.Kind == "bossfire" {
				t.Fatal("ice mushroom did not counter boss fire")
			}
		}
		s = fullPlantWorld(t, "day")
		fullPlantFixture(t, s, "jalapeno", 2, 1)
		for _, row := range []int{2, 3} {
			s.NextID++
			s.Projectiles = append(s.Projectiles, engine.Projectile{ID: s.NextID, Owner: 1, Row: row, X: 8500, Y: row*1000 + 500, VX: -125, Kind: "bossice", Damage: 100000, Payload: "hostile", BornAt: s.Tick})
		}
		fullPlantStep(t, s, 2)
		for _, v := range s.Projectiles {
			if v.Kind == "bossice" && v.Row == 2 {
				t.Fatal("jalapeno did not counter same-row boss ice")
			}
		}
		fullPlantRecord(t, s, "boss ball elemental counter")
	})
	t.Run("flags-projectiles-clone-replay", func(t *testing.T) {
		s := fullPlantWorld(t, "night")
		fullPlantFixture(t, s, "gatlingpea", 1, 1)
		fullPlantFixture(t, s, "sunshroom", 2, 0)
		fullPlantFixture(t, s, "magnetshroom", 3, 2)
		fullPlantZombieFixture(t, s, "buckethead", 1, 7000)
		fullPlantZombieFixture(t, s, "football", 3, 6000)
		fullPlantStep(t, s, 30)
		copy := engine.Clone(*s)
		if len(copy.Plants) == 0 {
			t.Fatal("fixture unexpectedly empty")
		}
		copy.Plants[0].Flags["probe"] = 1
		if s.Plants[0].Flags["probe"] != 0 {
			t.Fatal("clone shared plant flags")
		}
		copy = engine.Clone(*s)
		fullPlantStep(t, s, 100)
		for range 100 {
			fullPlantStep(t, &copy, 1)
		}
		if engine.Digest(*s) != engine.Digest(copy) {
			t.Fatal("extended world replay diverged")
		}
		fullPlantRecord(t, s, "Flags and projectile counters survive deterministic replay")
	})
}

func TestPVZFullPlantsCrossRowProjectileOrigins(t *testing.T) {
	t.Run("cattail-launches-from-owner-row", func(t *testing.T) {
		s := fullPlantWorld(t, "pool")
		fullPlantFixture(t, s, "cattail", 2, 2)
		fullPlantZombieFixture(t, s, "balloon", 0, 6500)
		fullPlantStep(t, s, 1)
		if len(s.Projectiles) == 0 {
			t.Fatal("missing first homing needle")
		}
		q := s.Projectiles[0]
		if q.Y < 2230 || q.Y > 2770 {
			t.Fatalf("homing projectile appeared in target row: y=%d source=2500", q.Y)
		}
		firstY := q.Y
		fullPlantStep(t, s, 5)
		if len(s.Projectiles) == 0 || s.Projectiles[0].Y >= firstY {
			t.Fatal("homing needle did not move toward upper target row")
		}
		fullPlantRecord(t, s, "homing needle starts at owner then crosses rows")
	})
	t.Run("threepeater-gradual-neighbor-lanes", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		fullPlantFixture(t, s, "threepeater", 2, 1)
		fullPlantZombieFixture(t, s, "buckethead", 1, 7000)
		var first []engine.Projectile
		for range 12 {
			fullPlantStep(t, s, 1)
			if len(s.Projectiles) > 0 {
				first = append(first, s.Projectiles...)
				break
			}
		}
		if len(first) != 3 {
			t.Fatalf("threepeater emitted %d projectiles", len(first))
		}
		for _, q := range first {
			if q.Y < 2300 || q.Y > 2700 {
				t.Fatalf("threepeater teleported to neighbor row: y=%d source=2500", q.Y)
			}
		}
		fullPlantStep(t, s, 8)
		low, high := 2500, 2500
		for _, q := range s.Projectiles {
			if q.Y < low {
				low = q.Y
			}
			if q.Y > high {
				high = q.Y
			}
		}
		if low >= 2200 || high <= 2800 {
			t.Fatal("neighbor shots did not curve into their lanes")
		}
		fullPlantRecord(t, s, "threepeater neighbor lanes use gradual vertical motion")
	})
}
