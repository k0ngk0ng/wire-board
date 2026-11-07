package pvz_test

// Full-unit behavioral scenarios were declared before zombies.go. Focused
// battle fixtures explicitly inject budget/positions; all purchases, state
// machines, collisions, damage and timers run through the real engine.
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
)

func fullZombieAction(t *testing.T, s *engine.State, p int, body map[string]any) {
	t.Helper()
	body["epoch"] = s.Epoch
	body["seq"] = s.Players[p].Seq + 1
	raw, _ := json.Marshal(body)
	var a engine.Action
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	if err := engine.Apply(s, p, a); err != nil {
		t.Fatalf("action %v: %v", body, err)
	}
}
func fullZombieFixture(t *testing.T, scene, kind string) *engine.State {
	t.Helper()
	var opts engine.Options
	raw, _ := json.Marshal(map[string]any{"mode": "duel", "scene": scene, "roundSeconds": 600, "prepSeconds": 10})
	_ = json.Unmarshal(raw, &opts)
	s, err := engine.New(opts, 2, "full-zombies-behavior")
	if err != nil {
		t.Fatal(err)
	}
	s.Players[0].Money = 1000000
	s.Players[1].Money = 1000000
	fullZombieAction(t, &s, 1, map[string]any{"type": "deck", "kinds": []string{kind}})
	if err = engine.Step(&s, 200); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		out := filepath.Join("..", "..", "..", ".local", "pvz", "full-zombies")
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "production engine Apply/Step; explicitly injected budget and starting positions; not a natural match claim", "command": ". scripts/env.sh && go test ./games/pvz/tests -run '^TestPVZFullZombie' -count=1", "final": s}, "", "  ")
		if err := os.WriteFile(filepath.Join(out, strings.ReplaceAll(t.Name(), "/", "-")+".json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	return &s
}
func fullZombieSpawn(t *testing.T, s *engine.State, kind string, row, col int) int {
	t.Helper()
	fullZombieAction(t, s, 1, map[string]any{"type": "spawn", "kind": kind, "row": row, "col": col})
	for i := len(s.Zombies) - 1; i >= 0; i-- {
		if s.Zombies[i].Kind == kind {
			return i
		}
	}
	t.Fatal("purchased zombie missing", kind)
	return -1
}
func fullZombiePlant(t *testing.T, s *engine.State, kind string, row, col int) int {
	t.Helper()
	c, ok := engine.PlantSpec(kind)
	if !ok {
		t.Fatal("plant missing", kind)
	}
	id := s.NextID + 1
	s.NextID = id
	s.Plants = append(s.Plants, engine.Plant{ID: id, Owner: 0, Kind: kind, Row: row, Col: col, HP: c.HP, MaxHP: c.HP, Layer: "body", BornAt: s.Tick, AttackAt: s.Tick + 100000, ReadyAt: s.Tick + 100000})
	return id
}
func fullZombieStep(t *testing.T, s *engine.State, n int) {
	t.Helper()
	if err := engine.Step(s, n); err != nil {
		t.Fatal(err)
	}
}
func fullZombieByID(s *engine.State, id int) *engine.Zombie {
	for i := range s.Zombies {
		if s.Zombies[i].ID == id {
			return &s.Zombies[i]
		}
	}
	return nil
}
func fullZombiePlantByID(s *engine.State, id int) *engine.Plant {
	for i := range s.Plants {
		if s.Plants[i].ID == id {
			return &s.Plants[i]
		}
	}
	return nil
}

func TestPVZFullZombieCatalogAndInitializationE2E(t *testing.T) {
	kinds := []string{"zombie", "flag", "conehead", "polevaulter", "buckethead", "newspaper", "screendoor", "football", "dancing", "backupdancer", "duckytube", "snorkel", "zomboni", "bobsled", "dolphin", "jackinthebox", "balloon", "digger", "pogo", "yeti", "bungee", "ladder", "catapult", "gargantuar", "imp", "zomboss", "peahead", "wallnuthead", "jalapenohead", "gatlinghead", "squashhead", "tallnuthead", "redeye"}
	if len(engine.GetCatalog().Zombies) != len(kinds) {
		t.Fatalf("want %d complete zombie cards; got %d", len(kinds), len(engine.GetCatalog().Zombies))
	}
	var profile struct {
		Zombies []struct {
			Kind   string `json:"kind"`
			Body   *int   `json:"bodyHP"`
			Helmet *int   `json:"helmetHP"`
			Shield *int   `json:"shieldHP"`
		} `json:"zombies"`
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs/research/pvz/evidence/full/zombie-health-profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &profile); err != nil {
		t.Fatal(err)
	}
	type health struct{ body, armor int }
	expected := map[string]health{}
	for _, p := range profile.Zombies {
		kind := p.Kind
		if kind == "normal" {
			kind = "zombie"
		}
		if kind == "pole" {
			kind = "polevaulter"
		}
		h := health{}
		if p.Body != nil {
			h.body = *p.Body
		}
		if p.Helmet != nil {
			h.armor += *p.Helmet
		}
		if p.Shield != nil {
			h.armor += *p.Shield
		}
		if kind == "bobsled" {
			h.armor = 300
		}
		if kind == "zomboss" {
			h.body = 40000
		}
		expected[kind] = h
	}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			scene, row := "day", 1
			if kind == "duckytube" || kind == "snorkel" || kind == "dolphin" {
				scene, row = "pool", 2
			}
			if kind == "zomboss" {
				scene = "roofnight"
			}
			s := fullZombieFixture(t, scene, kind)
			if kind == "bobsled" {
				s.Terrain = append(s.Terrain, engine.Terrain{ID: s.NextID + 1, Kind: "ice", Row: row, FromX: 0, ToX: 10000, Until: s.Tick + 1000})
				s.NextID++
			}
			at := fullZombieSpawn(t, s, kind, row, 0)
			z := s.Zombies[at]
			if h := expected[kind]; z.HP != h.body || z.Armor != h.armor {
				t.Fatalf("spawn health differs from canonical original Evidence: kind=%s body=%d armor=%d want=%+v", kind, z.HP, z.Armor, h)
			}
			if z.Flags == nil || z.Allegiance != "zombies" || z.Owner != 1 || z.HP <= 0 {
				t.Fatal("incomplete initialized zombie", z)
			}
			fullZombieStep(t, s, 1)
			clone := engine.Clone(*s)
			if len(clone.Zombies) > 0 {
				clone.Zombies[0].Flags["isolation"] = 1
				if s.Zombies[0].Flags["isolation"] != 0 {
					t.Fatal("Clone shares zombie flags")
				}
			}
		})
	}
}

func TestPVZFullZombieRawMovementAndJackCountersE2E(t *testing.T) {
	for _, q := range []struct {
		kind, scene string
		low, high   int
	}{{"balloon", "day", 287, 463}, {"snorkel", "pool", 250, 250}, {"pogo", "day", 562, 563}, {"digger", "day", 825, 850}, {"catapult", "day", 287, 463}, {"dolphin", "pool", 1112, 1138}} {
		t.Run(q.kind, func(t *testing.T) {
			s := fullZombieFixture(t, q.scene, q.kind)
			row := 1
			if q.scene == "pool" {
				row = 2
			}
			at := fullZombieSpawn(t, s, q.kind, row, 0)
			id := s.Zombies[at].ID
			s.Zombies[at].X = 7000
			if q.kind == "snorkel" {
				fullZombieStep(t, s, 30)
				fullZombieByID(s, id).X = 7000
			}
			if q.kind == "dolphin" {
				fullZombieStep(t, s, 60)
				fullZombieByID(s, id).X = 7000
			}
			cold := engine.Clone(*s)
			cold.Zombies[at].SlowUntil = cold.Tick + 100
			fullZombieStep(t, s, 20)
			fullZombieStep(t, &cold, 20)
			normal := 7000 - fullZombieByID(s, id).X
			chilled := 7000 - fullZombieByID(&cold, id).X
			if normal < q.low || normal > q.high {
				t.Fatalf("native direct velocity %s moved %d want[%d,%d]", q.kind, normal, q.low, q.high)
			}
			if q.kind == "digger" {
				if chilled != normal {
					t.Fatal("underground digger incorrectly chilled")
				}
			} else {
				ratio := float64(chilled) / float64(normal)
				if ratio < .39 || ratio > .41 {
					t.Fatalf("direct velX chill factor=%f want0.4", ratio)
				}
			}
		})
	}
	t.Run("jackbox-distance-over-velocity-counter", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "jackinthebox")
		at := fullZombieSpawn(t, s, "jackinthebox", 1, 0)
		z := s.Zombies[at]
		velocity, distance := z.Flags["velocityMilli"], z.Flags["fuseDistance"]
		if velocity < 660 || velocity > 680 || distance < 150 || distance > 749 {
			t.Fatal("missing original jackbox initialization inputs", z.Flags)
		}
		updates := 2 * (distance * 1000 / velocity)
		want := s.Tick + (updates*10+49)/50
		if z.Flags["explodeAt"] != want {
			t.Fatalf("jackbox counter %d != native2*truncate(distance/velX) %d", z.Flags["explodeAt"], want)
		}
	})
}

func TestPVZFullZombieColdEligibilityAndJackDisarmE2E(t *testing.T) {
	t.Run("canonical-chill-and-freeze-states", func(t *testing.T) {
		base := fullZombieFixture(t, "day", "zombie")
		at := fullZombieSpawn(t, base, "zombie", 1, 0)
		normal := base.Zombies[at]
		for _, q := range []struct {
			kind, state   string
			alt           int
			flags         map[string]int64
			side          string
			chill, freeze bool
		}{
			{"zombie", "walking", 0, nil, "zombies", true, true}, {"zombie", "walking", 0, nil, "plants", false, false},
			{"zomboni", "driving", 0, nil, "zombies", false, false}, {"bobsled", "sledding", 0, map[string]int64{"sled": 1}, "zombies", false, false},
			{"digger", "digging", 0, nil, "zombies", false, false}, {"digger", "emerging", 0, nil, "zombies", false, false},
			{"balloon", "flying", 1, nil, "zombies", true, false}, {"pogo", "bouncing", 0, map[string]int64{"pogo": 1}, "zombies", true, false},
			{"jackinthebox", "jackbox-popping", 0, nil, "zombies", true, false}, {"polevaulter", "vaulting", 1, nil, "zombies", true, false},
			{"bungee", "descending", 3000, nil, "zombies", true, false}, {"bungee", "grabbing", 0, nil, "zombies", true, true}, {"bungee", "bungee-grab", 0, nil, "zombies", true, false},
			{"zomboss", "boss-idle", 0, nil, "zombies", false, false}, {"zomboss", "boss-head-enter", 0, nil, "zombies", false, false}, {"zomboss", "boss-head-idle-before", 0, nil, "zombies", true, true},
		} {
			z := normal
			z.Kind = q.kind
			z.State = q.state
			z.Altitude = q.alt
			z.Flags = q.flags
			z.Allegiance = q.side
			z.Digging = q.state == "digging"
			if engine.CanChillZombie(z) != q.chill || engine.CanFreezeZombie(z) != q.freeze {
				t.Fatalf("wrong canonical cold eligibility %s/%s", q.kind, q.state)
			}
		}
	})
	t.Run("magnet-disarms-running-jackbox", func(t *testing.T) {
		s := fullZombieFixture(t, "night", "jackinthebox")
		fullZombiePlant(t, s, "magnetshroom", 1, 5)
		p := fullZombiePlant(t, s, "wallnut", 2, 5)
		at := fullZombieSpawn(t, s, "jackinthebox", 2, 0)
		id := s.Zombies[at].ID
		s.Zombies[at].X = 5500
		s.Zombies[at].Flags["explodeAt"] = s.Tick + 2
		fullZombieStep(t, s, 60)
		z := fullZombieByID(s, id)
		if z == nil || z.Flags["disarmed"] != 1 {
			t.Fatal("magnetized box still exploded", z)
		}
		if fullZombiePlantByID(s, p) == nil {
			t.Fatal("disarmed jackbox destroyed nearby wallnut")
		}
	})
}
func TestPVZFullZombieSummonsVehiclesAndWaterE2E(t *testing.T) {
	t.Run("dancing-owned-bounded-followers", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "dancing")
		at := fullZombieSpawn(t, s, "dancing", 2, 0)
		s.Zombies[at].X = 6500
		fullZombieStep(t, s, 180)
		count := 0
		for _, z := range s.Zombies {
			if z.Kind == "backupdancer" {
				count++
				if z.Owner != 1 || z.Allegiance != "zombies" {
					t.Fatal("summon lost owner", z)
				}
			}
		}
		if count != 4 {
			t.Fatalf("want four owned dancers; got %d", count)
		}
		fullZombieStep(t, s, 200)
		count = 0
		for _, z := range s.Zombies {
			if z.Kind == "backupdancer" {
				count++
			}
		}
		if count > 4 {
			t.Fatal("duplicate living followers", count)
		}
	})
	t.Run("zomboni-crush-and-ice", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "zomboni")
		id := fullZombiePlant(t, s, "wallnut", 1, 7)
		at := fullZombieSpawn(t, s, "zomboni", 1, 0)
		s.Zombies[at].X = 7800
		fullZombieStep(t, s, 80)
		if fullZombiePlantByID(s, id) != nil {
			t.Fatal("vehicle did not crush plant")
		}
		found := false
		for _, x := range s.Terrain {
			if x.Kind == "ice" && x.Row == 1 && x.Until > s.Tick {
				found = true
			}
		}
		if !found {
			t.Fatal("vehicle left no live ice track")
		}
	})
	t.Run("bobsled-dismount", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "bobsled")
		s.Terrain = append(s.Terrain, engine.Terrain{ID: s.NextID + 1, Kind: "ice", Row: 1, FromX: 7500, ToX: 10000, Until: s.Tick + 1000})
		s.NextID++
		at := fullZombieSpawn(t, s, "bobsled", 1, 0)
		s.Zombies[at].X = 7550
		fullZombieStep(t, s, 100)
		count := 0
		for _, z := range s.Zombies {
			if z.Kind == "bobsled" && z.State != "sledding" {
				count++
			}
		}
		if count < 4 {
			t.Fatalf("sled did not release four owned members: %d", count)
		}
	})
	t.Run("snorkel-dive-and-surface", func(t *testing.T) {
		s := fullZombieFixture(t, "pool", "snorkel")
		at := fullZombieSpawn(t, s, "snorkel", 2, 0)
		id := s.Zombies[at].ID
		s.Zombies[at].X = 6500
		fullZombieStep(t, s, 30)
		if z := fullZombieByID(s, id); z == nil || !z.Diving {
			t.Fatal("snorkel did not dive")
		}
		p := fullZombiePlant(t, s, "wallnut", 2, 5)
		fullZombieStep(t, s, 60)
		if z := fullZombieByID(s, id); z == nil || z.Diving {
			t.Fatal("snorkel did not surface to attack")
		}
		if fullZombiePlantByID(s, p).HP >= 4000 {
			t.Fatal("surfaced snorkel did not bite")
		}
	})
	t.Run("dolphin-single-vault-and-tallnut", func(t *testing.T) {
		for _, plant := range []string{"wallnut", "tallnut"} {
			t.Run(plant, func(t *testing.T) {
				s := fullZombieFixture(t, "pool", "dolphin")
				fullZombiePlant(t, s, plant, 2, 7)
				at := fullZombieSpawn(t, s, "dolphin", 2, 0)
				id := s.Zombies[at].ID
				fullZombieStep(t, s, 200)
				z := fullZombieByID(s, id)
				if z == nil {
					t.Fatal("missing dolphin")
				}
				if plant == "wallnut" && (!z.Vaulted || z.X >= 7500) {
					t.Fatal("dolphin failed to vault")
				}
				if plant == "tallnut" && z.X < 7500 {
					t.Fatal("dolphin crossed tallnut")
				}
			})
		}
	})
}

func TestPVZFullZombieNativeContactClocksE2E(t *testing.T) {
	t.Run("jackbox-110-update-pop-before-explosion", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "jackinthebox")
		id := fullZombiePlant(t, s, "wallnut", 2, 5)
		at := fullZombieSpawn(t, s, "jackinthebox", 2, 0)
		s.Zombies[at].X = 5500
		s.Zombies[at].Flags["explodeAt"] = s.Tick + 1
		fullZombieStep(t, s, 5)
		if fullZombiePlantByID(s, id) == nil {
			t.Fatal("jackbox skipped original 110-update opening animation")
		}
		fullZombieStep(t, s, 22)
		if fullZombiePlantByID(s, id) != nil {
			t.Fatal("jackbox failed to explode after opening animation")
		}
	})
	t.Run("giant-smash-contact-after-132-updates", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "gargantuar")
		id := fullZombiePlant(t, s, "tallnut", 2, 7)
		at := fullZombieSpawn(t, s, "gargantuar", 2, 0)
		s.Zombies[at].X = 7950
		fullZombieStep(t, s, 26)
		if fullZombiePlantByID(s, id) == nil {
			t.Fatal("giant damage preceded original .64 smash contact")
		}
		fullZombieStep(t, s, 4)
		if fullZombiePlantByID(s, id) != nil {
			t.Fatal("giant failed to apply smash contact")
		}
	})
}

func TestPVZFullZombieSpikyContactReachabilityE2E(t *testing.T) {
	t.Run("giant-naturally-targets-spikeweed", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "gargantuar")
		id := fullZombiePlant(t, s, "spikeweed", 1, 7)
		at := fullZombieSpawn(t, s, "gargantuar", 1, 0)
		s.Zombies[at].X = 7950
		fullZombieStep(t, s, 60)
		if fullZombiePlantByID(s, id) != nil {
			t.Fatal("giant skipped a naturally reached spikeweed")
		}
	})
	t.Run("spikerock-survives-eight-giant-swings", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "gargantuar")
		id := fullZombiePlant(t, s, "spikerock", 1, 7)
		at := fullZombieSpawn(t, s, "gargantuar", 1, 0)
		s.Zombies[at].X = 7950
		fullZombieStep(t, s, 345)
		p := fullZombiePlantByID(s, id)
		if p == nil || p.HP != 50 {
			t.Fatalf("eight natural swings should leave 50HP: %+v", p)
		}
		fullZombieStep(t, s, 45)
		if fullZombiePlantByID(s, id) != nil {
			t.Fatal("ninth natural giant swing failed to break spikerock")
		}
	})
	t.Run("vehicle-naturally-runs-over-spikes", func(t *testing.T) {
		for _, kind := range []string{"zomboni", "catapult"} {
			t.Run(kind, func(t *testing.T) {
				s := fullZombieFixture(t, "day", kind)
				fullZombiePlant(t, s, "spikeweed", 1, 7)
				at := fullZombieSpawn(t, s, kind, 1, 0)
				id := s.Zombies[at].ID
				s.Zombies[at].X = 7950
				fullZombieStep(t, s, 100)
				if z := fullZombieByID(s, id); z != nil && z.HP > 0 {
					t.Fatal("vehicle passed spikes with working tires", z)
				}
			})
		}
	})
}

func TestPVZFullZombieSpikerockVehicleDurabilityE2E(t *testing.T) {
	for _, kind := range []string{"zomboni", "catapult"} {
		for _, initialUses := range []int{0, 9} {
			t.Run(fmt.Sprintf("%s-uses-%d", kind, initialUses), func(t *testing.T) {
				s := fullZombieFixture(t, "day", kind)
				id := fullZombiePlant(t, s, "spikerock", 1, 7)
				fullZombiePlantByID(s, id).Uses = initialUses
				for hit := 1; hit <= 9; hit++ {
					for s.Tick < s.Players[1].Cooldowns[kind] || s.Tick < s.LaneCooldowns[1] {
						fullZombieStep(t, s, 1)
					}
					at := fullZombieSpawn(t, s, kind, 1, 0)
					vehicleID := s.Zombies[at].ID
					s.Zombies[at].X = 7950
					fullZombieStep(t, s, 1)
					if fullZombieByID(s, vehicleID) != nil {
						t.Fatal("vehicle survived natural tire collision")
					}
					p := fullZombiePlantByID(s, id)
					if hit == 9 {
						if p != nil {
							t.Fatal("spikerock survived ninth vehicle collision", p)
						}
						break
					}
					if p == nil || p.HP != 450-hit*50 || p.Uses != 9-hit {
						t.Fatalf("collision%d did not consume exactly one durability: %+v", hit, p)
					}
					fullZombieStep(t, s, 20)
					if p = fullZombiePlantByID(s, id); p == nil || p.HP != 450-hit*50 || p.Uses != 9-hit {
						t.Fatalf("dead vehicle consumed durability again: %+v", p)
					}
				}
			})
		}
	}
}

func fullZombieCobFixture(t *testing.T, s *engine.State, row, col int) (left, right, cob int) {
	t.Helper()
	left = fullZombiePlant(t, s, "flowerpot", row, col)
	fullZombiePlantByID(s, left).Layer = "support"
	right = fullZombiePlant(t, s, "flowerpot", row, col+1)
	fullZombiePlantByID(s, right).Layer = "support"
	cob = fullZombiePlant(t, s, "cobcannon", row, col)
	return
}
func TestPVZFullZombieCobFootprintE2E(t *testing.T) {
	t.Run("walking-from-right-bites-cannon-not-pot", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "zombie")
		_, right, cob := fullZombieCobFixture(t, s, 1, 3)
		at := fullZombieSpawn(t, s, "zombie", 1, 0)
		s.Zombies[at].X = 5000
		fullZombieStep(t, s, 20)
		body, pot := fullZombiePlantByID(s, cob), fullZombiePlantByID(s, right)
		if body == nil || body.HP >= body.MaxHP {
			t.Fatal("walking zombie did not contact cannon's right half", body)
		}
		if pot == nil || pot.HP != pot.MaxHP {
			t.Fatal("walking zombie bit cannon's right support", pot)
		}
	})
	t.Run("giant-and-vehicle-contact-right-body", func(t *testing.T) {
		for _, kind := range []string{"gargantuar", "zomboni"} {
			t.Run(kind, func(t *testing.T) {
				s := fullZombieFixture(t, "day", kind)
				left, right, cob := fullZombieCobFixture(t, s, 1, 3)
				at := fullZombieSpawn(t, s, kind, 1, 0)
				id := s.Zombies[at].ID
				s.Zombies[at].X = 5000
				fullZombieStep(t, s, 8)
				if kind == "gargantuar" {
					if z := fullZombieByID(s, id); z == nil || z.TargetID != cob {
						t.Fatal("giant targeted support instead of cannon's right footprint", z)
					}
				}
				fullZombieStep(t, s, 50)
				if fullZombiePlantByID(s, cob) != nil {
					t.Fatal("right-hand collision did not destroy full cannon")
				}
				if fullZombiePlantByID(s, right) != nil {
					t.Fatal("right-hand collision did not strike right cell")
				}
				if fullZombiePlantByID(s, left) == nil {
					t.Fatal("right-hand collision incorrectly destroyed left-only support")
				}
			})
		}
	})
	t.Run("bungee-old-right-kernel-id-after-upgrade", func(t *testing.T) {
		s := fullZombieFixture(t, "roofnight", "bungee")
		left := fullZombiePlant(t, s, "flowerpot", 1, 3)
		fullZombiePlantByID(s, left).Layer = "support"
		right := fullZombiePlant(t, s, "flowerpot", 1, 4)
		fullZombiePlantByID(s, right).Layer = "support"
		fullZombiePlant(t, s, "kernelpult", 1, 3)
		oldRight := fullZombiePlant(t, s, "kernelpult", 1, 4)
		at := fullZombieSpawn(t, s, "bungee", 1, 4)
		if s.Zombies[at].TargetID != oldRight {
			t.Fatal("fixture bungee did not select old right kernel")
		}
		fullZombieAction(t, s, 0, map[string]any{"type": "plant", "kind": "cobcannon", "row": 1, "col": 3})
		cob := 0
		for _, p := range s.Plants {
			if p.Kind == "cobcannon" {
				cob = p.ID
			}
		}
		if cob == 0 || fullZombiePlantByID(s, oldRight) != nil {
			t.Fatal("fixture upgrade did not invalidate old target")
		}
		fullZombieStep(t, s, 220)
		if fullZombiePlantByID(s, cob) == nil || fullZombiePlantByID(s, right) == nil || fullZombiePlantByID(s, left) == nil {
			t.Fatal("bungee retargeted a support through cannon's right half")
		}
	})
	t.Run("boss-stomp-right-cell-directly-hits-body", func(t *testing.T) {
		s := fullZombieFixture(t, "roofnight", "zomboss")
		left, _, cob := fullZombieCobFixture(t, s, 1, 4)
		at := fullZombieSpawn(t, s, "zomboss", 2, 0)
		for _, key := range []string{"headAt", "summonAt", "bungeeAt", "rvAt"} {
			s.Zombies[at].Flags[key] = s.Tick + 100000
		}
		s.Zombies[at].Flags["stompAt"] = s.Tick + 1
		fullZombieStep(t, s, 150)
		if fullZombiePlantByID(s, cob) != nil {
			t.Fatal("right-half stomp failed to destroy cannon")
		}
		if fullZombiePlantByID(s, left) == nil {
			t.Fatal("right-half stomp leaked into left support column")
		}
		hitBody := false
		for _, e := range s.Events {
			if e.Type == "plant-hit" && e.Target == cob {
				hitBody = true
			}
		}
		if !hitBody {
			t.Fatal("stomp only hit right support and dropped cannon indirectly")
		}
	})
}

func TestPVZFullZombieBossHeadAndStompClocksE2E(t *testing.T) {
	t.Run("native-head-enter-idle-spit-leave", func(t *testing.T) {
		s := fullZombieFixture(t, "roofnight", "zomboss")
		at := fullZombieSpawn(t, s, "zomboss", 2, 0)
		id := s.Zombies[at].ID
		for _, key := range []string{"summonAt", "bungeeAt", "stompAt", "rvAt"} {
			s.Zombies[at].Flags[key] = s.Tick + 100000
		}
		s.Zombies[at].Flags["headAt"] = s.Tick + 1
		fullZombieStep(t, s, 70)
		if z := fullZombieByID(s, id); z.State != "boss-head-enter" || z.Flags["headOpen"] != 0 {
			t.Fatal("head entry skipped source animation", z.State)
		}
		fullZombieStep(t, s, 100)
		if z := fullZombieByID(s, id); z.State != "boss-head-idle-before" || z.Flags["headOpen"] != 1 {
			t.Fatal("head has no vulnerable five-second idle", z.State)
		}
		fullZombieStep(t, s, 100)
		if z := fullZombieByID(s, id); z.State != "boss-head-spit" {
			t.Fatal("head did not enter native spit clip", z.State)
		}
		fullZombieStep(t, s, 40)
		found := false
		for _, p := range s.Projectiles {
			if p.Kind == "bossfire" || p.Kind == "bossice" {
				found = true
			}
		}
		if !found {
			t.Fatal("head did not emit elemental ball at .42 contact")
		}
		fullZombieStep(t, s, 100)
		if z := fullZombieByID(s, id); z.State != "boss-head-leave" {
			t.Fatal("head skipped post-spit idle and leave", z.State)
		}
		fullZombieStep(t, s, 70)
		if z := fullZombieByID(s, id); z.State != "boss-idle" || z.Flags["headOpen"] != 0 {
			t.Fatal("head did not return to protected body", z.State)
		}
	})
	t.Run("stomp-hits-two-rows-last-four-columns", func(t *testing.T) {
		s := fullZombieFixture(t, "roofnight", "zomboss")
		for row := 0; row < 5; row++ {
			for col := 0; col < 9; col++ {
				fullZombiePlant(t, s, "wallnut", row, col)
			}
		}
		at := fullZombieSpawn(t, s, "zomboss", 2, 0)
		for _, key := range []string{"summonAt", "bungeeAt", "headAt", "rvAt"} {
			s.Zombies[at].Flags[key] = s.Tick + 100000
		}
		s.Zombies[at].Flags["stompAt"] = s.Tick + 1
		fullZombieStep(t, s, 150)
		if len(s.Plants) != 45-8 {
			t.Fatalf("native stomp must remove 2x4 plants; remaining=%d", len(s.Plants))
		}
		for _, p := range s.Plants {
			if p.Col < 5 && p.HP < 4000 {
				t.Fatal("stomp hit a protected left column")
			}
		}
	})
}

func TestPVZFullZombieMountedAndBungeeDamageEligibilityE2E(t *testing.T) {
	t.Run("mounted-followers-shielded-then-dismount", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "bobsled")
		s.Terrain = append(s.Terrain, engine.Terrain{ID: s.NextID + 1, Kind: "ice", Row: 1, FromX: 0, ToX: 10000, Until: s.Tick + 1000})
		s.NextID++
		at := fullZombieSpawn(t, s, "bobsled", 1, 0)
		leader := s.Zombies[at].ID
		fullZombieStep(t, s, 1)
		follower := 0
		before := 0
		for _, z := range s.Zombies {
			if z.Kind == "bobsled" && z.Flags["leader"] == int64(leader) {
				follower = z.ID
				before = z.HP
				break
			}
		}
		if follower == 0 {
			t.Fatal("sled has no passenger")
		}
		s.DamageZombie(follower, 1800, "cherrybomb", 0)
		if z := fullZombieByID(s, follower); z == nil || z.HP != before {
			t.Fatal("mounted passenger incorrectly took blast damage", z)
		}
		s.DamageZombie(leader, 100000, "squash", 0)
		fullZombieStep(t, s, 30)
		z := fullZombieByID(s, follower)
		if z == nil || z.Flags["sled"] != 0 || z.State == "sledding" {
			t.Fatal("passenger never left a destroyed sled", z)
		}
		if z.SpeedMilli < 6756 || z.SpeedMilli > 10869 {
			t.Fatal("dismounted passenger did not restore ordinary ground speed", z.SpeedMilli)
		}
		s.DamageZombie(follower, 20, "pea", 0)
		if z.HP != before-20 {
			t.Fatal("dismounted passenger remained invulnerable")
		}
	})
	t.Run("descending-bungee-untargetable-bottom-vulnerable", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "bungee")
		fullZombiePlant(t, s, "wallnut", 1, 5)
		at := fullZombieSpawn(t, s, "bungee", 1, 5)
		id := s.Zombies[at].ID
		before := s.Zombies[at].HP
		s.DamageZombie(id, 1800, "cherrybomb", 0)
		if s.Zombies[at].HP != before {
			t.Fatal("descending bungee incorrectly took area damage")
		}
		fullZombieStep(t, s, 85)
		z := fullZombieByID(s, id)
		if z == nil || z.State != "grabbing" {
			t.Fatal("bungee did not arrive at bottom", z)
		}
		s.DamageZombie(id, 20, "pea", 0)
		if z.HP != before-20 {
			t.Fatal("bottom bungee remained untargetable")
		}
	})
}

func TestPVZFullZombieDancerHypnosisDetachesRelationshipsE2E(t *testing.T) {
	for _, targetLeader := range []bool{false, true} {
		t.Run(fmt.Sprint(targetLeader), func(t *testing.T) {
			s := fullZombieFixture(t, "day", "dancing")
			at := fullZombieSpawn(t, s, "dancing", 2, 0)
			leader := s.Zombies[at].ID
			s.Zombies[at].X = 8500
			fullZombieStep(t, s, 100)
			children := []int{}
			for _, z := range s.Zombies {
				if z.Kind == "backupdancer" && z.Flags["leader"] == int64(leader) {
					children = append(children, z.ID)
				}
			}
			if len(children) != 4 {
				t.Fatal("four original backup dancers missing")
			}
			target := children[0]
			if targetLeader {
				target = leader
			}
			z := fullZombieByID(s, target)
			z.Row = 2
			z.X = 4900
			fullZombiePlant(t, s, "hypnoshroom", 2, 4)
			fullZombieStep(t, s, 110)
			z = fullZombieByID(s, target)
			if z == nil || z.Allegiance != "plants" {
				t.Fatal("naturally bitten hypnoshroom did not persistently hypnotize target", z)
			}
			if targetLeader {
				for _, id := range children {
					if child := fullZombieByID(s, id); child != nil && child.Allegiance != "zombies" {
						t.Fatal("hypnotizing leader converted an existing backup", child)
					}
				}
			} else {
				if z.Flags["leader"] != 0 {
					t.Fatal("hypnotized backup kept original parent link")
				}
				if boss := fullZombieByID(s, leader); boss == nil || boss.Allegiance != "zombies" {
					t.Fatal("backup hypnosis changed the leader")
				}
			}
		})
	}
}

func TestPVZFullZombieHypnosisNativeVelocityE2E(t *testing.T) {
	for _, kind := range []string{"zombie", "polevaulter", "newspaper", "football", "backupdancer", "flag"} {
		t.Run(kind, func(t *testing.T) {
			s := fullZombieFixture(t, "day", kind)
			at := fullZombieSpawn(t, s, kind, 1, 0)
			id := s.Zombies[at].ID
			z := &s.Zombies[at]
			z.X = 4500
			z.Allegiance = "plants"
			z.State = "walking"
			if kind == "newspaper" {
				z.Armor = 0
			}
			// This explicit conversion boundary isolates the exact post-chew
			// callback while all subsequent motion uses the production Step path.
			s.OnZombieHypnotized(z)
			if z.Owner != 1 || z.SpeedMilli != 4993 || z.SpecialSpeedMilli != 4993 {
				t.Fatalf("conversion did not apply native .17 ground profile: %+v", z)
			}
			fullZombieStep(t, s, 20)
			z = fullZombieByID(s, id)
			if z == nil {
				t.Fatal("converted zombie disappeared")
			}
			if delta := z.X - 4500; delta < 99 || delta > 100 {
				t.Fatalf("converted %s still used original/special fast speed: delta=%d", kind, delta)
			}
		})
	}
	t.Run("retains-existing-cold-and-stun-deadlines", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "zombie")
		at := fullZombieSpawn(t, s, "zombie", 1, 0)
		id := s.Zombies[at].ID
		z := &s.Zombies[at]
		z.X = 4500
		z.Allegiance = "plants"
		z.State = "walking"
		z.SlowUntil = s.Tick + 100
		z.Flags["frozenUntil"] = s.Tick + 2
		z.Flags["butterUntil"] = s.Tick + 3
		slow, frozen, butter := z.SlowUntil, z.Flags["frozenUntil"], z.Flags["butterUntil"]
		s.OnZombieHypnotized(z)
		if z.SlowUntil != slow || z.Flags["frozenUntil"] != frozen || z.Flags["butterUntil"] != butter {
			t.Fatal("conversion cleared source-preserved cold/stun counters")
		}
		fullZombieStep(t, s, 20)
		z = fullZombieByID(s, id)
		if z == nil {
			t.Fatal("converted zombie disappeared")
		}
		if delta := z.X - 4500; delta < 44 || delta > 45 {
			t.Fatalf("preserved cold/stun did not affect native .17 profile: delta=%d", delta)
		}
	})
}
func TestPVZFullZombieToolsAndTargetingE2E(t *testing.T) {
	t.Run("jackbox-fuse-and-local-explosion", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "jackinthebox")
		near := fullZombiePlant(t, s, "wallnut", 2, 5)
		far := fullZombiePlant(t, s, "wallnut", 0, 0)
		at := fullZombieSpawn(t, s, "jackinthebox", 2, 0)
		s.Zombies[at].X = 5500
		s.Zombies[at].Flags["explodeAt"] = s.Tick + 2
		fullZombieStep(t, s, 30)
		if fullZombiePlantByID(s, near) != nil {
			t.Fatal("jackbox did not destroy nearby plant")
		}
		if fullZombiePlantByID(s, far) == nil {
			t.Fatal("jackbox destroyed distant plant")
		}
	})
	t.Run("balloon-flies-over-and-lands", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "balloon")
		fullZombiePlant(t, s, "tallnut", 1, 7)
		at := fullZombieSpawn(t, s, "balloon", 1, 0)
		id := s.Zombies[at].ID
		s.Zombies[at].X = 7900
		fullZombieStep(t, s, 120)
		z := fullZombieByID(s, id)
		if z == nil || z.Altitude == 0 || z.X >= 7500 {
			t.Fatal("balloon did not overfly plant")
		}
		z.Altitude = 0
		fullZombieStep(t, s, 1)
		if fullZombieByID(s, id).State == "flying" {
			t.Fatal("popped balloon still flying")
		}
	})
	t.Run("digger-emerges-and-reverses", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "digger")
		at := fullZombieSpawn(t, s, "digger", 1, 0)
		id := s.Zombies[at].ID
		s.Zombies[at].X = 350
		fullZombieStep(t, s, 100)
		z := fullZombieByID(s, id)
		if z == nil || z.Digging || z.Flags["direction"] != 1 {
			t.Fatal("digger did not emerge facing right")
		}
		x := z.X
		fullZombieStep(t, s, 20)
		if fullZombieByID(s, id).X <= x {
			t.Fatal("emerged digger did not reverse")
		}
		if delta := fullZombieByID(s, id).X - x; delta < 70 || delta > 71 {
			t.Fatalf("emerged digger did not use native .12 ground velocity: %d", delta)
		}
	})
	t.Run("pogo-blocked-and-magnetized", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "pogo")
		fullZombiePlant(t, s, "tallnut", 1, 7)
		at := fullZombieSpawn(t, s, "pogo", 1, 0)
		id := s.Zombies[at].ID
		s.Zombies[at].X = 7950
		fullZombieStep(t, s, 80)
		z := fullZombieByID(s, id)
		if z == nil || z.X < 7500 {
			t.Fatal("pogo crossed tallnut")
		}
		if z.Flags["pogo"] != 0 {
			t.Fatal("tallnut did not break the pogo stick")
		}
		z.Flags["magnetized"] = 1
		fullZombieStep(t, s, 1)
		if z := fullZombieByID(s, id); z.State == "vaulting" || z.Altitude > 0 {
			t.Fatal("magnetized pogo still bouncing")
		}
	})
	t.Run("yeti-retreat", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "yeti")
		at := fullZombieSpawn(t, s, "yeti", 1, 0)
		id := s.Zombies[at].ID
		s.Zombies[at].X = 4500
		s.Zombies[at].Flags["retreatAt"] = s.Tick + 1
		fullZombieStep(t, s, 20)
		z := fullZombieByID(s, id)
		if z == nil || z.State != "escaping" || z.X <= 4500 {
			t.Fatal("yeti did not retreat")
		}
	})
	t.Run("bungee-target-and-umbrella", func(t *testing.T) {
		for _, shielded := range []bool{false, true} {
			t.Run(fmt.Sprint(shielded), func(t *testing.T) {
				s := fullZombieFixture(t, "day", "bungee")
				id := fullZombiePlant(t, s, "sunflower", 1, 3)
				if shielded {
					fullZombiePlant(t, s, "umbrellaleaf", 2, 3)
				}
				at := fullZombieSpawn(t, s, "bungee", 1, 3)
				s.Zombies[at].X = 3500
				s.Zombies[at].TargetID = id
				fullZombieStep(t, s, 220)
				exists := fullZombiePlantByID(s, id) != nil
				if exists != shielded {
					t.Fatalf("umbrella=%v target survived=%v", shielded, exists)
				}
			})
		}
	})
	t.Run("ladder-placement-and-following-climb", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "ladder")
		id := fullZombiePlant(t, s, "tallnut", 1, 7)
		at := fullZombieSpawn(t, s, "ladder", 1, 0)
		s.Zombies[at].X = 7950
		fullZombieStep(t, s, 100)
		found := false
		for _, l := range s.Ladders {
			if l.Row == 1 && l.Col == 7 && l.PlantID == id {
				found = true
			}
		}
		if !found {
			t.Fatal("ladder not placed")
		}
		if z := s.Zombies[at]; z.X >= 7500 {
			t.Fatal("ladder zombie did not climb")
		}
	})
}
func TestPVZFullZombieHeavyAndPlantHeadsE2E(t *testing.T) {
	t.Run("giant-smash-and-owned-imp-once", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "gargantuar")
		id := fullZombiePlant(t, s, "tallnut", 2, 7)
		at := fullZombieSpawn(t, s, "gargantuar", 2, 0)
		gid := s.Zombies[at].ID
		s.Zombies[at].X = 7950
		s.Zombies[at].HP = s.Zombies[at].MaxHP/2 - 1
		fullZombieStep(t, s, 180)
		if fullZombiePlantByID(s, id) != nil {
			t.Fatal("giant failed to smash plant")
		}
		count := 0
		for _, z := range s.Zombies {
			if z.Kind == "imp" {
				count++
				if z.Owner != 1 {
					t.Fatal("thrown imp lost owner")
				}
			}
		}
		if count != 1 {
			t.Fatalf("want one thrown imp, got %d", count)
		}
		if fullZombieByID(s, gid) == nil {
			t.Fatal("giant unexpectedly missing")
		}
	})
	t.Run("catapult-and-plant-head-shots", func(t *testing.T) {
		for _, kind := range []string{"catapult", "peahead", "gatlinghead"} {
			t.Run(kind, func(t *testing.T) {
				s := fullZombieFixture(t, "day", kind)
				id := fullZombiePlant(t, s, "tallnut", 1, 0)
				at := fullZombieSpawn(t, s, kind, 1, 0)
				s.Zombies[at].X = 7000
				fullZombieStep(t, s, 400)
				p := fullZombiePlantByID(s, id)
				if p != nil && p.HP >= p.MaxHP {
					t.Fatal("ranged zombie produced no plant damage")
				}
				shot := false
				for _, e := range s.Events {
					if e.Type == "shoot" {
						shot = true
					}
				}
				if !shot {
					t.Fatal("ranged zombie emitted no ordered shot")
				}
			})
		}
	})
	t.Run("catapult-ammo-exhaustion-resumes-driving", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "catapult")
		fullZombiePlant(t, s, "tallnut", 1, 0)
		at := fullZombieSpawn(t, s, "catapult", 1, 0)
		id := s.Zombies[at].ID
		s.Zombies[at].X = 7000
		fullZombieStep(t, s, 1300)
		z := fullZombieByID(s, id)
		if z == nil || z.Flags["ammo"] != 0 || z.X >= 6900 {
			t.Fatal("empty catapult stayed parked", z)
		}
	})
	t.Run("jalapeno-row-and-squash-contact", func(t *testing.T) {
		for _, kind := range []string{"jalapenohead", "squashhead"} {
			t.Run(kind, func(t *testing.T) {
				s := fullZombieFixture(t, "day", kind)
				id := fullZombiePlant(t, s, "tallnut", 1, 5)
				far := fullZombiePlant(t, s, "wallnut", 3, 5)
				at := fullZombieSpawn(t, s, kind, 1, 0)
				s.Zombies[at].X = 5950
				s.Zombies[at].Flags["explodeAt"] = s.Tick + 1
				fullZombieStep(t, s, 160)
				if fullZombiePlantByID(s, id) != nil {
					t.Fatal("plant head special did not kill target")
				}
				if fullZombiePlantByID(s, far) == nil {
					t.Fatal("plant head attacked wrong row")
				}
			})
		}
	})
	t.Run("boss-cycles-real-attacks", func(t *testing.T) {
		s := fullZombieFixture(t, "roofnight", "zomboss")
		for row := 0; row < 5; row++ {
			for col := 0; col < 8; col++ {
				fullZombiePlant(t, s, "wallnut", row, col)
			}
		}
		at := fullZombieSpawn(t, s, "zomboss", 2, 0)
		id := s.Zombies[at].ID
		for _, flag := range []string{"summonAt", "headAt", "bungeeAt", "stompAt", "rvAt"} {
			s.Zombies[at].Flags[flag] = s.Tick + 1
		}
		seen := map[string]bool{}
		for n := 0; n < 2200; n++ {
			fullZombieStep(t, s, 1)
			for _, e := range s.Events {
				seen[e.Type] = true
			}
			if z := fullZombieByID(s, id); z != nil && strings.HasPrefix(z.State, "boss-head") {
				seen["boss-head"] = true
			}
			if s.Phase != "battle" {
				break
			}
		}
		if fullZombieByID(s, id) == nil {
			t.Fatal("boss unexpectedly missing")
		}
		if len(s.Zombies) < 2 {
			t.Fatal("boss never summoned owned units")
		}
		if len(s.Plants) >= 40 {
			t.Fatal("boss never attacked battlefield")
		}
		if !seen["boss-head"] && !seen["boss-ball"] {
			t.Fatal("boss has no head/elemental phase")
		}
	})
	t.Run("hypnotized-units-never-breach-or-hit-friendly", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "zombie")
		at := fullZombieSpawn(t, s, "zombie", 1, 0)
		s.Zombies[at].Allegiance = "plants"
		s.Zombies[at].X = 1000
		id := s.Zombies[at].ID
		fullZombieStep(t, s, 80)
		z := fullZombieByID(s, id)
		if z == nil || z.X <= 1000 || z.Owner != 1 {
			t.Fatal("hypnotized zombie failed to reverse with original owner")
		}
		if s.Mowers[1].State != "ready" {
			t.Fatal("hypnotized zombie triggered mower")
		}
	})
	t.Run("hypnotized-giant-imp-keeps-allegiance", func(t *testing.T) {
		s := fullZombieFixture(t, "day", "gargantuar")
		at := fullZombieSpawn(t, s, "gargantuar", 1, 0)
		s.Zombies[at].Allegiance = "plants"
		s.Zombies[at].X = 4500
		s.Zombies[at].HP = s.Zombies[at].MaxHP/2 - 1
		fullZombieStep(t, s, 80)
		found := false
		for _, z := range s.Zombies {
			if z.Kind == "imp" {
				found = true
				if z.Owner != 1 || z.Allegiance != "plants" || z.X <= 4500 {
					t.Fatal("hypnotized giant threw a hostile/backward imp", z)
				}
			}
		}
		if !found {
			t.Fatal("hypnotized giant did not use its imp ability")
		}
	})
}
