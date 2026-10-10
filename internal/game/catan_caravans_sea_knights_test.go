package game

import (
	"fmt"
	"testing"
)

func caravansSeaSetups() []string {
	return []string{"shores", "islands", "desert", "tribe", "new_world"}
}

func TestCatanCaravansSeaKnightsMatrix(t *testing.T) {
	for _, scenario := range caravansSeaSetups() {
		for n := 2; n <= 6; n++ {
			t.Run(fmt.Sprintf("%s/%d", scenario, n), func(t *testing.T) {
				s, err := NewCatanCaravansSeaCitiesKnights(n, scenario, nil)
				if err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				if !g.caravansSeaKnights() || len(g.Players) != n {
					t.Fatal("missing knight markers")
				}
				if g.Seafarers.Pirate != -1 {
					t.Fatal("pirate stayed on the board")
				}
				if err = s.validateCaravans(); err != nil {
					t.Fatal(err)
				}
				if err = s.validateCatanTwo(); err != nil {
					t.Fatal(err)
				}
				b := clone(*s)
				if err = b.validateCaravans(); err != nil {
					t.Fatal("restored state rejected", err)
				}
			})
		}
	}
}

func TestCatanCaravansSeaKnightsNatural(t *testing.T) {
	for _, tc := range []struct {
		scenario string
		n        int
	}{
		{"shores", 3},
		{"shores", 2},
		{"islands", 4},
		{"desert", 5},
		{"tribe", 6},
		{"new_world", 2},
	} {
		t.Run(fmt.Sprintf("%s/%d", tc.scenario, tc.n), func(t *testing.T) {
			s, err := NewCatanCaravansSeaCitiesKnights(tc.n, tc.scenario, nil)
			if err != nil {
				t.Fatal(err)
			}
			for step := 0; step < 20000 && !s.Finished; step++ {
				p := ckActor(s)
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				if err = s.Apply(p, a); err != nil {
					t.Fatal(step, s.Phase, a, err)
				}
				if step%137 == 0 {
					b := clone(*s)
					s = &b
					if err = s.validateCaravans(); err != nil {
						t.Fatal(step, err)
					}
				}
			}
			if !s.Finished {
				t.Fatalf("unfinished round %d phase %s", s.Round, s.Phase)
			}
			t.Log("round", s.Round)
		})
	}
}

func TestCatanCaravansSeaKnightsRejectsForeignState(t *testing.T) {
	s, err := NewCatanCaravansSeaCitiesKnights(3, "shores", nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Catan){
		"missing sea knights":   func(g *Catan) { g.Caravans.SeaKnights = "" },
		"unknown sea knights":   func(g *Catan) { g.Caravans.SeaKnights = "bad" },
		"missing caravans mark": func(g *Catan) { g.Caravans.Knights = "" },
		"pirate on board":       func(g *Catan) { g.Seafarers.Pirate = 0 },
		"early robber":          func(g *Catan) { g.Robber = g.Caravans.Map.WateringHoles[0] },
	} {
		t.Run(name, func(t *testing.T) {
			bad := clone(*s)
			mutate(bad.Catan)
			if err := bad.validateCaravans(); err == nil {
				t.Fatal("bad state accepted")
			}
		})
	}
	plain, err := NewCatanCaravansShoresSeafarers(3)
	if err != nil {
		t.Fatal(err)
	}
	if plain.Catan.CitiesKnights != nil || plain.Catan.caravansSeaKnights() {
		t.Fatal("plain caravan sea gained knight state")
	}
	if err := plain.validateCaravans(); err != nil {
		t.Fatal("plain caravan sea rejected", err)
	}
}
