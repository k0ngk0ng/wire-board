package game

import (
	"fmt"
	"strings"
	"testing"
)

func TestCatanFishingCaravansSeaMatrix(t *testing.T) {
	for _, scenario := range caravansSeaSetups() {
		for n := 2; n <= 6; n++ {
			for _, knights := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/knights%t", scenario, n, knights), func(t *testing.T) {
					s, err := NewCatanFishingCaravansSea(n, scenario, nil, knights)
					if err != nil {
						t.Fatal(err)
					}
					g := s.Catan
					if !g.fishingCaravansSea() || len(g.Fishing.Map.Lakes) != 0 {
						t.Fatal("missing fishing markers")
					}
					if knights != (g.Fishing.SeaKnights == CatanFishingSeaKnightsRules) {
						t.Fatal("knight marker mismatch")
					}
					expected := 6
					if n > 4 {
						expected = 8
					}
					if len(g.Fishing.Map.Grounds) != expected {
						t.Fatalf("grounds %d", len(g.Fishing.Map.Grounds))
					}
					if err = s.validateCaravans(); err != nil {
						t.Fatal(err)
					}
					if err = g.validateFishing(); err != nil {
						t.Fatal(err)
					}
					if err = s.validateCatanTwo(); err != nil {
						t.Fatal(err)
					}
					b := clone(*s)
					if err = b.Catan.validateFishing(); err != nil {
						t.Fatal("restored state rejected", err)
					}
					starts := map[int]bool{}
					for _, start := range g.Caravans.Map.Starts {
						starts[start.Edge] = true
					}
					for _, ground := range g.Fishing.Map.Grounds {
						for _, edge := range ground.Edges {
							if !g.edgeTerrain(edge, true) || !g.edgeTerrain(edge, false) || starts[edge] {
								t.Fatal("inland or blocked fishing ground")
							}
						}
					}
				})
			}
		}
	}
}

func TestCatanFishingCaravansSeaNatural(t *testing.T) {
	total := 0
	for _, tc := range []struct {
		scenario string
		n        int
		knights  bool
	}{
		{"shores", 3, false},
		{"shores", 2, true},
		{"islands", 4, true},
		{"desert", 5, false},
		{"tribe", 6, true},
		{"new_world", 2, false},
	} {
		t.Run(fmt.Sprintf("%s/%d/knights%t", tc.scenario, tc.n, tc.knights), func(t *testing.T) {
			s, err := NewCatanFishingCaravansSea(tc.n, tc.scenario, nil, tc.knights)
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

func TestCatanFishingCaravansSeaRejectsForeignState(t *testing.T) {
	s, err := NewCatanFishingCaravansSea(3, "desert", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Catan){
		"missing recipe":  func(g *Catan) { g.Fishing.Map.SeaRecipe = "" },
		"unknown recipe":  func(g *Catan) { g.Fishing.Map.SeaRecipe = "bad" },
		"lake present":    func(g *Catan) { g.Fishing.Map.Lakes = []catanFishingLake{{}} },
		"missing caravan": func(g *Catan) { g.Fishing.Caravans = "" },
		"knight marker":   func(g *Catan) { g.Fishing.SeaKnights = CatanFishingSeaKnightsRules },
		"on a wagon start": func(g *Catan) {
			g.Fishing.Map.Grounds[0].Edges[0] = g.Caravans.Map.Starts[0].Edge
		},
		"bad number": func(g *Catan) { g.Fishing.Map.Grounds[0].Number = 7 },
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
