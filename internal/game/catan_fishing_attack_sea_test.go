package game

import (
	"fmt"
	"strings"
	"testing"
)

func attackSeaSetups() []string {
	return []string{"shores", "desert", "tribe", "wonders", "pirates"}
}

func TestCatanFishingAttackSeaMatrix(t *testing.T) {
	for _, scenario := range attackSeaSetups() {
		for n := 2; n <= 6; n++ {
			t.Run(fmt.Sprintf("%s/%d", scenario, n), func(t *testing.T) {
				s, err := NewCatanFishingAttackSea(n, scenario)
				if err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				if !g.fishingAttackSea() || len(g.Fishing.Map.Lakes) != 0 {
					t.Fatal("missing fishing markers")
				}
				expected := 6
				if n > 4 {
					expected = 8
				}
				if len(g.Fishing.Map.Grounds) != expected {
					t.Fatalf("grounds %d", len(g.Fishing.Map.Grounds))
				}
				if err = g.validateFishing(); err != nil {
					t.Fatal(err)
				}
				if err = s.validateCatanAttack(); err != nil {
					t.Fatal(err)
				}
				b := clone(*s)
				if err = b.Catan.validateFishing(); err != nil {
					t.Fatal("restored state rejected", err)
				}
				for _, ground := range g.Fishing.Map.Grounds {
					for _, edge := range ground.Edges {
						if !g.edgeTerrain(edge, true) || !g.edgeTerrain(edge, false) {
							t.Fatal("inland fishing ground")
						}
					}
				}
			})
		}
	}
}

func TestCatanFishingAttackSeaNatural(t *testing.T) {
	total := 0
	for _, tc := range []struct {
		scenario string
		n        int
	}{
		{"shores", 3},
		{"desert", 2},
		{"tribe", 5},
		{"wonders", 4},
		{"pirates", 6},
	} {
		t.Run(fmt.Sprintf("%s/%d", tc.scenario, tc.n), func(t *testing.T) {
			s, err := NewCatanFishingAttackSea(tc.n, tc.scenario)
			if err != nil {
				t.Fatal(err)
			}
			fish := 0
			for step := 0; step < 20000 && !s.Finished; step++ {
				p := ckActor(s)
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				if strings.HasPrefix(a.Type, "catan_fish_") {
					fish++
				}
				if err = s.Apply(p, a); err != nil {
					t.Fatal(step, s.Phase, a, err)
				}
				if step%137 == 0 {
					b := clone(*s)
					s = &b
					if err = s.Catan.validateFishing(); err != nil {
						t.Fatal(step, err)
					}
				}
			}
			if !s.Finished {
				t.Fatalf("unfinished round %d phase %s", s.Round, s.Phase)
			}
			total += fish
			t.Log("round", s.Round, "fish", fish)
		})
	}
	if total == 0 {
		t.Fatal("no fish action across the natural games")
	}
}

func TestCatanFishingAttackSeaRejectsForeignState(t *testing.T) {
	s, err := NewCatanFishingAttackSea(3, "shores")
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Catan){
		"missing recipe": func(g *Catan) { g.Fishing.Map.SeaRecipe = "" },
		"unknown recipe": func(g *Catan) { g.Fishing.Map.SeaRecipe = "bad" },
		"lake present":   func(g *Catan) { g.Fishing.Map.Lakes = []catanFishingLake{{}} },
		"missing attack": func(g *Catan) { g.Fishing.Attack = "" },
		"duplicate edge": func(g *Catan) { g.Fishing.Map.Grounds[1].Edges[0] = g.Fishing.Map.Grounds[0].Edges[0] },
		"bad number":     func(g *Catan) { g.Fishing.Map.Grounds[0].Number = 7 },
	} {
		t.Run(name, func(t *testing.T) {
			bad := clone(*s)
			mutate(bad.Catan)
			if err := bad.Catan.validateFishing(); err == nil {
				t.Fatal("bad state accepted")
			}
		})
	}
}

// Supply tiles (wonders deserts) hold many more barbarians than players, and
// every defeated barbarian has to become a prisoner or the ledger breaks.
func TestCatanAttackSurplusPrisonersStayAccounted(t *testing.T) {
	for _, players := range [][]int{{0, 1}, {0, 1, 2}, {0, 1, 2, 3}} {
		strength := make([]int, len(players))
		for i := range strength {
			strength[i] = 1
		}
		for barbarians := len(players); barbarians <= 12; barbarians++ {
			battle := catanAttackBattle{Barbarians: barbarians, Prisoners: make([]int, len(players)), Gold: make([]int, len(players))}
			rolls := 0
			die := func() int { rolls++; return 1 + rolls%6 }
			if err := battle.distribute(strength, die); err != nil {
				t.Fatal(barbarians, err)
			}
			total := 0
			for _, n := range battle.Prisoners {
				total += n
			}
			if total != barbarians {
				t.Fatalf("%d players, %d barbarians: %d prisoners", len(players), barbarians, total)
			}
		}
	}
}
