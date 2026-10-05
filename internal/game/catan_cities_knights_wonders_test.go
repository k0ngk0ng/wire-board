package game

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestCatanCitiesKnightsWondersSetup(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		layouts := []string{"fixed"}
		if n < 5 {
			layouts = append(layouts, "variable")
		}
		for _, layout := range layouts {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				s, err := NewCatanCitiesKnightsSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "wonders", Layout: layout}, nil)
				if err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				if s.Phase != "catan_wonders_start" || g.Robber != -1 || g.Seafarers.Pirate != -1 || g.Seafarers.VictoryPoints != 12 {
					t.Fatal("incorrect combined wonder prelude")
				}
				starts := g.wonderStartTiles()
				chosen := starts[len(starts)-1]
				helperReject(t, s, (s.Turn+1)%n, Action{Type: "catan_wonders_start", Tile: chosen})
				helperApply(t, s, s.Turn, Action{Type: "catan_wonders_start", Tile: chosen})
				if s.Catan.Robber != -1 || s.Catan.CitiesKnights.RobberStart != chosen {
					t.Fatal("origin choice woke robber or was lost")
				}
				saved := clone(*s)
				s = &saved
				for step := 0; s.Catan.setup() && step < 100; step++ {
					actor := ckActor(s)
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					helperApply(t, s, actor, a)
				}
				g = s.Catan
				if g.setup() || s.Phase != "catan_roll" {
					t.Fatal("incomplete setup", s.Phase)
				}
				for p := range g.Players {
					_, v, c := g.pieces(p)
					if v != 1 || c != 1 || sum(g.Players[p].Resources[5:]) != 0 {
						t.Fatal("wrong opening buildings/resources")
					}
				}
				for _, v := range g.Vertices {
					if v.Level > 0 {
						for _, blocked := range g.wonders().SetupBlocked {
							if v.ID == blocked {
								t.Fatal("forbidden opening vertex")
							}
						}
					}
				}
				ckProgressStock(t, g)
				ckKnightStock(t, g)
				if !reflect.DeepEqual(*s, clone(*s)) {
					t.Fatal("setup persistence")
				}
				s.catanFinishBarbarians()
				if g.Robber != chosen || g.Seafarers.Pirate != -1 {
					t.Fatal("first attack lost selected origin or introduced pirate")
				}
				for _, line := range s.Log {
					if strings.Contains(line, "海盗进入") {
						t.Fatal("nonexistent pirate log")
					}
				}
				// Even after invasion there are no pirate destinations or knight chases.
				s.Phase = "catan_robber"
				view := s.View(s.Turn)["catan"].(map[string]any)
				legal := view["legal"].(map[string][]int)
				if len(legal["pirate"]) > 0 || len(legal["knightChasePirate"]) > 0 {
					t.Fatal("pirate enabled")
				}
				helperReject(t, s, s.Turn, Action{Type: "catan_pirate", Tile: chosen})
			})
		}
	}
}

func TestCatanCitiesKnightsWondersVictoryPaths(t *testing.T) {
	for _, test := range []struct {
		name                string
		score, level, rival int
		win                 bool
	}{
		{"ten_is_not_enough", 10, 3, 2, false}, {"eleven_is_not_enough", 11, 3, 2, false},
		{"twelve_leading", 12, 3, 2, true}, {"thirteen_tied", 13, 2, 2, false},
		{"unclaimed_level", 15, 0, 0, false}, {"four_without_points", 3, 4, 3, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := ckSea(t, 3, "wonders")
			g := s.Catan
			g.SetupStep = 6
			s.Phase = "catan_turn"
			p := s.Turn
			g.Players[p].Score = test.score
			g.wonders().Cards[0] = CatanWonder{ID: 0, Owner: p, Level: test.level}
			g.wonders().Cards[1] = CatanWonder{ID: 1, Owner: (p + 1) % 3, Level: test.rival}
			s.catanVictory()
			if s.Finished != test.win {
				t.Fatal("wrong combined wonder victory", test)
			}
		})
	}
	// Standalone and legacy snapshots retain their ten-point alternative.
	s, err := NewCatanWonders(3, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.SetupStep = 6
	g.Players[s.Turn].Score = 10
	g.wonders().Cards[0] = CatanWonder{ID: 0, Owner: s.Turn, Level: 1}
	s.catanVictory()
	if !s.Finished {
		t.Fatal("standalone changed")
	}
}

func TestCatanCitiesKnightsWondersBotsComplete(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		layouts := []string{"fixed"}
		if n < 5 {
			layouts = append(layouts, "variable")
		}
		for _, layout := range layouts {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				s, err := NewCatanCitiesKnightsSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "wonders", Layout: layout}, nil)
				if err != nil {
					t.Fatal(err)
				}
				builds, claims, steps := 0, 0, 0
				for ; steps < 10000 && !s.Finished; steps++ {
					p := ckActor(s)
					a, e := s.BotAction(p)
					if e != nil {
						t.Fatal(steps, s.Phase, e)
					}
					if e = s.Apply(p, a); e != nil {
						t.Fatal(steps, s.Phase, a, e)
					}
					if a.Type == "catan_wonder_claim" {
						claims++
					}
					if a.Type == "catan_wonder_build" {
						builds++
					}
					g := s.Catan
					ckProgressStock(t, g)
					ckKnightStock(t, g)
					for p := range g.Players {
						if g.shipCount(p) > 15 || g.cityPiecesLeft(p) < 0 || g.settlementPiecesLeft(p) < 0 {
							t.Fatal("piece inventory")
						}
					}
					if g.Seafarers.Pirate != -1 || g.ArmyOwner != -1 || len(g.DevDeck) != 0 {
						t.Fatal("wrong combined components")
					}
					if steps%53 == 0 {
						saved := clone(*s)
						if !reflect.DeepEqual(*s, saved) {
							t.Fatal("persistence")
						}
						s = &saved
					}
				}
				if !s.Finished || len(s.Winners) != 1 || !s.Catan.wonderVictory(s.Winners[0]) || builds == 0 || claims == 0 {
					t.Fatal("incomplete wonder game", steps, s.Phase)
				}
				t.Logf("steps=%d claims=%d builds=%d invasions=%d score=%d level=%d", steps, claims, builds, s.Catan.CitiesKnights.Invasions, s.Catan.Players[s.Winners[0]].Score, s.Catan.wonders().Cards[s.Catan.wonderOwned(s.Winners[0])].Level)
			})
		}
	}
}
