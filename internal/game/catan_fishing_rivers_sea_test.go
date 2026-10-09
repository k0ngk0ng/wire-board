package game

import (
	"fmt"
	"strings"
	"testing"
)

func riversSeaFishingSetups() []CatanRiversSeafarersSetup {
	return []CatanRiversSeafarersSetup{
		{Scenario: "shores"},
		{Scenario: "fog"},
		{Scenario: "desert", Layout: "rivers-across"},
		{Scenario: "desert", Layout: "desert-belt"},
		{Scenario: "tribe"},
		{Scenario: "new_world"},
	}
}

// Every printed river-sea map has to admit the fishing nesting for the same
// player counts as the plain combination, with and without city knights.
func TestCatanFishingRiversSeaMatrix(t *testing.T) {
	for _, setup := range riversSeaFishingSetups() {
		for n := 2; n <= 6; n++ {
			for _, knights := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/%d/knights%t", setup.Scenario, setup.Layout, n, knights), func(t *testing.T) {
					s, err := NewCatanFishingRiversSea(n, setup, nil, knights)
					if err != nil {
						t.Fatal(err)
					}
					g := s.Catan
					if !g.fishingRiversSea() || len(g.Fishing.Map.Lakes) != 0 {
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
					if err = g.validateRivers(); err != nil {
						t.Fatal(err)
					}
					if err = g.validateFishing(); err != nil {
						t.Fatal(err)
					}
					b := clone(*s)
					if err = b.Catan.validateFishing(); err != nil {
						t.Fatal("restored state rejected", err)
					}
					// Lake-free maps must never place a lake or leak ground edges.
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
}

// Spot-check natural games across the map families instead of repeating the
// full 60-configuration matrix, which is already covered by the HTTP tests.
func TestCatanFishingRiversSeaNatural(t *testing.T) {
	total := 0
	for _, tc := range []struct {
		setup   CatanRiversSeafarersSetup
		n       int
		knights bool
	}{
		{CatanRiversSeafarersSetup{Scenario: "shores"}, 3, false},
		{CatanRiversSeafarersSetup{Scenario: "shores"}, 2, true},
		{CatanRiversSeafarersSetup{Scenario: "fog"}, 4, true},
		{CatanRiversSeafarersSetup{Scenario: "desert", Layout: "rivers-across"}, 5, false},
		{CatanRiversSeafarersSetup{Scenario: "desert", Layout: "desert-belt"}, 6, true},
		{CatanRiversSeafarersSetup{Scenario: "tribe"}, 3, true},
		{CatanRiversSeafarersSetup{Scenario: "new_world"}, 6, false},
	} {
		t.Run(fmt.Sprintf("%s/%s/%d/knights%t", tc.setup.Scenario, tc.setup.Layout, tc.n, tc.knights), func(t *testing.T) {
			s, err := NewCatanFishingRiversSea(tc.n, tc.setup, nil, tc.knights)
			if err != nil {
				t.Fatal(err)
			}
			fishActions := 0
			for step := 0; step < 20000 && !s.Finished; step++ {
				p := ckActor(s)
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				if strings.HasPrefix(a.Type, "catan_fish_") {
					fishActions++
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
			total += fishActions
			t.Log("round", s.Round, "fish", fishActions)
		})
	}
	// A single short game may legitimately ignore fish; the set must not.
	if total == 0 {
		t.Fatal("no fish action across the natural games")
	}
}

func TestCatanFishingRiversSeaRejectsForeignState(t *testing.T) {
	s, err := NewCatanFishingRiversSea(3, CatanRiversSeafarersSetup{Scenario: "shores"}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Catan){
		"missing recipe": func(g *Catan) { g.Fishing.Map.SeaRecipe = "" },
		"unknown recipe": func(g *Catan) { g.Fishing.Map.SeaRecipe = "bad" },
		"lake present":   func(g *Catan) { g.Fishing.Map.Lakes = []catanFishingLake{{}} },
		"missing rivers": func(g *Catan) { g.Fishing.Rivers = "" },
		"knight marker":  func(g *Catan) { g.Fishing.SeaKnights = CatanFishingSeaKnightsRules },
		"duplicate edge": func(g *Catan) {
			g.Fishing.Map.Grounds[1].Edges[0] = g.Fishing.Map.Grounds[0].Edges[0]
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
