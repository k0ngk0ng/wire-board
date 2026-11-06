package pvz_test

import (
	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
	"testing"
)

func TestPVZFullPlantsSubmergedAndUndergroundTargetMasks(t *testing.T) {
	for _, kind := range []string{"cabbagepult", "kernelpult", "melonpult", "wintermelon"} {
		t.Run(kind+"-diving-target", func(t *testing.T) {
			s := fullPlantWorld(t, "pool")
			fullPlantFixture(t, s, kind, 2, 1)
			id := fullPlantZombieFixture(t, s, "snorkel", 2, 6500)
			z := fullPlantZombieByID(s, id)
			z.Diving = true
			z.State = "diving"
			z.Flags["waterEntered"] = 1
			hp := z.HP
			fullPlantStep(t, s, 100)
			z = fullPlantZombieByID(s, id)
			if z != nil && z.HP >= hp {
				t.Fatal("lobber failed to target submerged snorkel")
			}
			fullPlantRecord(t, s, "lobber source mask permits submerged target")
		})
	}
	t.Run("cattail-excludes-diving", func(t *testing.T) {
		s := fullPlantWorld(t, "pool")
		id := fullPlantFixture(t, s, "cattail", 2, 1)
		zid := fullPlantZombieFixture(t, s, "snorkel", 3, 6500)
		z := fullPlantZombieByID(s, zid)
		z.Diving = true
		z.State = "diving"
		z.Flags["waterEntered"] = 1
		fullPlantStep(t, s, 100)
		for _, e := range s.Events {
			if e.Type == "shoot" && e.Target == id {
				t.Fatal("cattail used lobber submerged mask")
			}
		}
		fullPlantRecord(t, s, "homing needle preserves exclusion of submerged target")
	})
	t.Run("melon-submerged-splash", func(t *testing.T) {
		s := fullPlantWorld(t, "pool")
		fullPlantFixture(t, s, "melonpult", 1, 1)
		primary := fullPlantZombieFixture(t, s, "gargantuar", 1, 6500)
		fullPlantZombieByID(s, primary).Flags["frozenUntil"] = s.Tick + 1000
		id := fullPlantZombieFixture(t, s, "snorkel", 2, 6500)
		z := fullPlantZombieByID(s, id)
		z.Diving = true
		z.State = "diving"
		z.Flags["waterEntered"] = 1
		z.Flags["frozenUntil"] = s.Tick + 1000
		hp := z.HP
		fullPlantStep(t, s, 100)
		z = fullPlantZombieByID(s, id)
		if z == nil || z.HP >= hp {
			t.Fatal("melon splash excluded submerged adjacent target")
		}
		fullPlantRecord(t, s, "submerged secondary receives melon splash")
	})
	t.Run("squash-locks-diving", func(t *testing.T) {
		s := fullPlantWorld(t, "pool")
		pid := fullPlantFixture(t, s, "squash", 2, 4)
		id := fullPlantZombieFixture(t, s, "snorkel", 2, 5100)
		z := fullPlantZombieByID(s, id)
		z.Diving = true
		z.State = "diving"
		z.Flags["waterEntered"] = 1
		fullPlantStep(t, s, 1)
		p := fullPlantByID(s, pid)
		if p == nil || p.State != "jumping" || p.TargetID != id {
			t.Fatal("squash did not actively select diving target")
		}
		fullPlantStep(t, s, 25)
		if fullPlantZombieByID(s, id) != nil {
			t.Fatal("squash failed submerged impact")
		}
		fullPlantRecord(t, s, "squash mask includes submerged target")
	})
	t.Run("potato-triggered-by-underground-digger", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		fullPlantFixture(t, s, "potatomine", 2, 4)
		id := fullPlantZombieFixture(t, s, "digger", 2, 4500)
		z := fullPlantZombieByID(s, id)
		z.Digging = true
		z.State = "digging"
		fullPlantStep(t, s, 2)
		if fullPlantZombieByID(s, id) != nil {
			t.Fatal("underground digger failed to trigger armed mine")
		}
		fullPlantRecord(t, s, "potato mask includes underground without a separate surface trigger")
	})
}

func TestPVZFullPlantsLadderMagnetAndChomperCategories(t *testing.T) {
	t.Run("magnet-removes-ladder-shield", func(t *testing.T) {
		s := fullPlantWorld(t, "night")
		fullPlantFixture(t, s, "magnetshroom", 2, 4)
		fullPlantFixture(t, s, "peashooter", 2, 1)
		id := fullPlantZombieFixture(t, s, "ladder", 2, 6500)
		z := fullPlantZombieByID(s, id)
		z.Flags["frozenUntil"] = s.Tick + 1000
		fullPlantStep(t, s, 1)
		z = fullPlantZombieByID(s, id)
		if z == nil || z.Flags["magnetized"] != 1 || z.Armor != 0 {
			t.Fatal("magnetized ladder retained invisible shield")
		}
		hp := z.HP
		fullPlantStep(t, s, 100)
		z = fullPlantZombieByID(s, id)
		if z == nil || z.HP >= hp || z.Flags["ladderPlaced"] != 0 {
			t.Fatal("front pea did not reach unshielded body")
		}
		fullPlantRecord(t, s, "ladder tool and shield removed together")
	})
	for _, state := range []string{"bouncing", "frozen", "disarmed"} {
		t.Run("chomper-pogo-"+state, func(t *testing.T) {
			s := fullPlantWorld(t, "day")
			pid := fullPlantFixture(t, s, "chomper", 2, 4)
			id := fullPlantZombieFixture(t, s, "pogo", 2, 5600)
			z := fullPlantZombieByID(s, id)
			z.State = "bouncing"
			z.Flags["pogo"] = 1
			if state == "frozen" {
				z.Flags["frozenUntil"] = s.Tick + 200
			}
			if state == "disarmed" {
				z.Flags["pogo"] = 0
				z.Flags["magnetized"] = 1
				z.State = "walking"
			}
			fullPlantStep(t, s, 20)
			z = fullPlantZombieByID(s, id)
			if state == "bouncing" {
				if z == nil || fullPlantByID(s, pid).TargetID == id {
					t.Fatal("chomper locked/swallowed active pogo")
				}
			} else if z != nil {
				t.Fatal("immobilized or disarmed pogo was not swallowable")
			}
			fullPlantRecord(t, s, "pogo selection depends on actual bouncing/immobilized state")
		})
	}
	for _, kind := range []string{"zomboni", "catapult", "bobsled"} {
		t.Run("chomper-vehicle-"+kind, func(t *testing.T) {
			s := fullPlantWorld(t, "day")
			pid := fullPlantFixture(t, s, "chomper", 2, 4)
			id := fullPlantZombieFixture(t, s, kind, 2, 5650)
			z := fullPlantZombieByID(s, id)
			z.Flags["ammo"] = 0
			z.Flags["shootAt"] = s.Tick + 200
			fullPlantStep(t, s, 20)
			if fullPlantZombieByID(s, id) != nil || fullPlantByID(s, pid) == nil || fullPlantByID(s, pid).State != "digesting" {
				t.Fatal("chomper substituted vehicle type exclusion for original swallowing rule")
			}
			fullPlantRecord(t, s, "non-giant vehicle follows swallow eligibility")
		})
	}
}

func TestPVZFullPlantsDenseSplashAndCannonWindup(t *testing.T) {
	t.Run("melon-560-budget", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		ids := []int{}
		for range 11 {
			id := fullPlantZombieFixture(t, s, "gargantuar", 2, 6500)
			fullPlantZombieByID(s, id).Flags["frozenUntil"] = s.Tick + 1000
			ids = append(ids, id)
		}
		s.NextID++
		s.Projectiles = append(s.Projectiles, engine.Projectile{ID: s.NextID, Owner: 0, Row: 2, X: 6000, Y: 2500, TargetX: 6500, TargetY: 2500, Kind: "melon", Damage: 80, Radius: 1250, LandAt: s.Tick + 1, BornAt: s.Tick})
		fullPlantStep(t, s, 1)
		for i, id := range ids {
			z := fullPlantZombieByID(s, id)
			want := 26
			if i == 0 {
				want = 80
			}
			if z == nil || z.MaxHP-z.HP != want {
				t.Fatalf("dense melon damage at %d=%v want %d", i, z, want)
			}
		}
		fullPlantRecord(t, s, "eleven clustered targets retain 26 secondary below native560 budget")
	})
	t.Run("cannon-attack-clock", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		id := fullPlantFixture(t, s, "cobcannon", 2, 1)
		at := s.Tick
		if e := s.UsePlantAbility(0, id, 2, 6); e != nil {
			t.Fatal(e)
		}
		if fullPlantByID(s, id).Flags["attackAt"] != at {
			t.Fatal("legal cannon shot omitted attack windup tick")
		}
		fullPlantRecord(t, s, "same-state cannon shot restarts attack animation clock")
	})
}

func TestPVZFullPlantsBossProjectileMeanSpeed(t *testing.T) {
	for _, tc := range []struct {
		kind  string
		units int
	}{{"bossfire", 462}, {"bossice", 446}} {
		t.Run(tc.kind, func(t *testing.T) {
			s := fullPlantWorld(t, "day")
			s.NextID++
			s.Projectiles = append(s.Projectiles, engine.Projectile{ID: s.NextID, Owner: 1, Row: 1, X: 8000, Y: 1500, VX: -125, Kind: tc.kind, Damage: 100000, Payload: "hostile", BornAt: s.Tick})
			fullPlantStep(t, s, 20)
			if len(s.Projectiles) != 1 || 8000-s.Projectiles[0].X != tc.units {
				t.Fatalf("boss ball mean speed incorrect: %+v", s.Projectiles)
			}
			fullPlantRecord(t, s, "source-ground period mean with retained sub-unit timing")
		})
	}
}

func TestPVZFullPlantsNativeTargetRankingAndPuffLifetime(t *testing.T) {
	t.Run("cattail-two-dimensional-nearest", func(t *testing.T) {
		s := fullPlantWorld(t, "pool")
		fullPlantFixture(t, s, "cattail", 2, 4)
		fullPlantZombieFixture(t, s, "zombie", 0, 5000)
		near := fullPlantZombieFixture(t, s, "duckytube", 2, 5800)
		fullPlantStep(t, s, 1)
		if len(s.Projectiles) != 1 || s.Projectiles[0].TargetID != near {
			t.Fatal("cattail ranked cross-row target only by x distance")
		}
		fullPlantRecord(t, s, "cattail ranking uses two-dimensional distance")
	})
	t.Run("magnet-euclidean-plus-row", func(t *testing.T) {
		s := fullPlantWorld(t, "night")
		fullPlantFixture(t, s, "magnetshroom", 2, 4)
		far := fullPlantZombieFixture(t, s, "buckethead", 0, 5000)
		near := fullPlantZombieFixture(t, s, "buckethead", 1, 7200)
		fullPlantStep(t, s, 1)
		if fullPlantZombieByID(s, far).Armor != 1100 || fullPlantZombieByID(s, near).Armor != 0 {
			t.Fatal("magnet chose Manhattan/iteration order instead of native score")
		}
		fullPlantRecord(t, s, "metal candidate rank includes native row penalty")
	})
	t.Run("magnet-nearest-ground-ladder", func(t *testing.T) {
		s := fullPlantWorld(t, "night")
		fullPlantFixture(t, s, "magnetshroom", 2, 4)
		far := fullPlantFixture(t, s, "wallnut", 0, 2)
		near := fullPlantFixture(t, s, "wallnut", 2, 5)
		s.Ladders = []engine.Ladder{{Row: 0, Col: 2, PlantID: far, Owner: 1}, {Row: 2, Col: 5, PlantID: near, Owner: 1}}
		fullPlantStep(t, s, 1)
		if len(s.Ladders) != 1 || s.Ladders[0].PlantID != far {
			t.Fatal("magnet removed first placed ladder rather than closest")
		}
		fullPlantRecord(t, s, "placed ladder nearest score and stable ties")
	})
	t.Run("puff-expiry-after-75-updates", func(t *testing.T) {
		s := fullPlantWorld(t, "night")
		fullPlantFixture(t, s, "puffshroom", 1, 1)
		fullPlantZombieFixture(t, s, "buckethead", 1, 4300)
		var id int
		var born int64
		for range 20 {
			fullPlantStep(t, s, 1)
			if len(s.Projectiles) > 0 {
				id = s.Projectiles[0].ID
				born = s.Projectiles[0].BornAt
				break
			}
		}
		if id == 0 {
			t.Fatal("missing puff projectile")
		}
		s.Zombies = []engine.Zombie{}
		for s.Tick < born+14 {
			fullPlantStep(t, s, 1)
		}
		alive := false
		for _, q := range s.Projectiles {
			alive = alive || q.ID == id
		}
		if !alive {
			t.Fatal("short puff expired before 75 original updates")
		}
		fullPlantStep(t, s, 1)
		for _, q := range s.Projectiles {
			if q.ID == id {
				t.Fatal("short puff outlived original 75-update timer")
			}
		}
		fullPlantRecord(t, s, "75-update projectile lifetime includes its creation step")
	})
}

func TestPVZFullPlantsFireSplashResistance(t *testing.T) {
	t.Run("resistant-primary-no-splash", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		door := fullPlantZombieFixture(t, s, "screendoor", 2, 6500)
		other := fullPlantZombieFixture(t, s, "zombie", 2, 6700)
		for _, id := range []int{door, other} {
			z := fullPlantZombieByID(s, id)
			z.Flags["frozenUntil"] = s.Tick + 1000
			z.SlowUntil = s.Tick + 500
		}
		s.NextID++
		s.Projectiles = append(s.Projectiles, engine.Projectile{ID: s.NextID, Owner: 0, Row: 2, X: 6300, Y: 2500, VX: 208, Kind: "firepea", Damage: 40, Radius: 1250, BornAt: s.Tick})
		fullPlantStep(t, s, 1)
		d, o := fullPlantZombieByID(s, door), fullPlantZombieByID(s, other)
		if d == nil || d.Armor != 1060 || o == nil || o.HP != 270 {
			t.Fatal("fire resistant direct hit incorrectly splashed neighbors")
		}
		if d.SlowUntil <= s.Tick || d.Flags["frozenUntil"] <= s.Tick {
			t.Fatal("resisted direct fire incorrectly removed cold")
		}
		fullPlantRecord(t, s, "fireball resistant primary uses ordinary single-target damage")
	})
	t.Run("resistant-secondary-excluded", func(t *testing.T) {
		s := fullPlantWorld(t, "day")
		normal := fullPlantZombieFixture(t, s, "zombie", 2, 6500)
		door := fullPlantZombieFixture(t, s, "screendoor", 2, 6750)
		cone := fullPlantZombieFixture(t, s, "conehead", 2, 6900)
		car := fullPlantZombieFixture(t, s, "zomboni", 2, 7100)
		for _, id := range []int{normal, door, cone, car} {
			fullPlantZombieByID(s, id).Flags["frozenUntil"] = s.Tick + 1000
		}
		s.NextID++
		s.Projectiles = append(s.Projectiles, engine.Projectile{ID: s.NextID, Owner: 0, Row: 2, X: 6300, Y: 2500, VX: 208, Kind: "firepea", Damage: 40, Radius: 1250, BornAt: s.Tick})
		fullPlantStep(t, s, 1)
		if fullPlantZombieByID(s, normal).HP != 230 || fullPlantZombieByID(s, door).Armor != 1100 || fullPlantZombieByID(s, car).HP != 1350 || fullPlantZombieByID(s, cone).Armor != 357 {
			t.Fatal("fire splash did not distinguish resistant vehicles/shields from plastic armor")
		}
		fullPlantRecord(t, s, "secondary splash excludes metal shields and vehicles, retains plastic armor")
	})
}
