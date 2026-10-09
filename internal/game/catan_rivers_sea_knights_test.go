package game

import (
	"fmt"
	"slices"
	"testing"
)

// The knight nesting reuses the sea knight conversion on every printed
// river-sea map, including the labelled extended recipes.
func TestCatanRiversSeaKnightsNatural(t *testing.T) {
	setups := []CatanRiversSeafarersSetup{
		{Scenario: "shores"},
		{Scenario: "fog"},
		{Scenario: "desert", Layout: "rivers-across"},
		{Scenario: "desert", Layout: "desert-belt"},
		{Scenario: "tribe"},
		{Scenario: "new_world"},
	}
	for _, setup := range setups {
		for n := 2; n <= 6; n++ {
			t.Run(fmt.Sprintf("%s/%s/%d", setup.Scenario, setup.Layout, n), func(t *testing.T) {
				s, err := NewCatanRiversSeaCitiesKnights(n, setup, nil)
				if err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				if !g.riversSeaKnights() || g.CitiesKnights == nil || g.CitiesKnights.PirateStart < -1 {
					t.Fatal("missing knight markers")
				}
				if g.Seafarers.Pirate != -1 {
					t.Fatal("pirate stayed on the board")
				}
				if n > 4 && !g.Options.FiveSix {
					t.Fatal("missing paired turns")
				}
				if err = g.validateRivers(); err != nil {
					t.Fatal(err)
				}
				if err = s.validateCatanTwo(); err != nil {
					t.Fatal(err)
				}
				// Periodic clone-and-validate keeps the saved state honest.
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
						if err = s.Catan.validateRivers(); err != nil {
							t.Fatal(step, err)
						}
					}
				}
				if !s.Finished {
					t.Fatalf("unfinished round %d phase %s", s.Round, s.Phase)
				}
				best := 0
				for _, player := range s.Catan.Players {
					if player.Score > best {
						best = player.Score
					}
				}
				if best < s.Catan.victoryTarget() {
					t.Fatalf("finished at %d points, target %d", best, s.Catan.victoryTarget())
				}
			})
		}
	}
}

func TestCatanRiversSeaKnightsRejectsForeignState(t *testing.T) {
	s, err := NewCatanRiversSeaCitiesKnights(3, CatanRiversSeafarersSetup{Scenario: "shores"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Catan){
		"missing sea knights": func(g *Catan) { g.Rivers.SeaKnights = "" },
		"unknown sea knights": func(g *Catan) { g.Rivers.SeaKnights = "bad" },
		"missing river knights": func(g *Catan) {
			g.Rivers.Knights = ""
		},
		"pirate on board": func(g *Catan) { g.Seafarers.Pirate = 0 },
		"early robber": func(g *Catan) {
			g.Robber = slices.IndexFunc(g.Tiles, func(t CatanTile) bool { return t.Resource >= 0 && t.Resource < 5 })
		},
	} {
		t.Run(name, func(t *testing.T) {
			bad := clone(*s)
			mutate(bad.Catan)
			if err := bad.Catan.validateRivers(); err == nil {
				t.Fatal("bad state accepted")
			}
		})
	}
	// The plain recipe keeps its printed target and no knight state.
	plain, err := NewCatanRiversSeafarers(3, CatanRiversSeafarersSetup{Scenario: "shores"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := plain.Catan.validateRivers(); err != nil {
		t.Fatal("plain river sea rejected", err)
	}
	if plain.Catan.CitiesKnights != nil || plain.Catan.riversSeaKnights() {
		t.Fatal("plain river sea gained knight state")
	}
	if plain.Catan.Seafarers.VictoryPoints != 14 || plain.Catan.victoryTarget() != 14 {
		t.Fatalf("plain target changed: %d/%d", plain.Catan.Seafarers.VictoryPoints, plain.Catan.victoryTarget())
	}
}
