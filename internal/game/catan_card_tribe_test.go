package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func cardTribeSetup(t *testing.T, n int, layout string) *State {
	t.Helper()
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "tribe", Layout: layout}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; s.Catan.setup() && step < 4*n; step++ {
		a, err := s.BotAction(s.Turn)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, s.Turn, a)
	}
	if s.Phase != "catan_roll" {
		t.Fatal("setup did not reach roll", s.Phase)
	}
	return s
}

func cardTribeInventory(t *testing.T, g *Catan) {
	t.Helper()
	fleetSupply(t, g)
	cards := len(g.DevDeck) + len(g.DevDiscard) + len(g.tribe().Development)
	for i, p := range g.Players {
		cards += sum(p.Dev)
		roads, villages, cities := g.pieces(i)
		if roads > 15 || villages > 5 || cities > 4 || g.shipCount(i) > 15 {
			t.Fatal("piece inventory")
		}
		for c, n := range p.NewDev {
			if n < 0 || n > p.Dev[c] {
				t.Fatal("new development age")
			}
		}
	}
	want := 25
	if len(g.Players) > 4 {
		want = 34
	}
	if cards != want {
		t.Fatal("development cards including tribe rewards", cards, want)
	}
}

func TestCatanCardTribeFleeOfficialMapsAndOrdinaryRobberLimits(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		layouts := []string{"fixed"}
		if n <= 4 {
			layouts = append(layouts, "variable")
		}
		for _, layout := range layouts {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				for _, automatic := range []bool{false, true} {
					s := cardTribeSetup(t, n, layout)
					g := s.Catan
					deserts := g.fleeDeserts()
					if len(deserts) < 2 {
						t.Fatal("official tribe needs multiple deserts")
					}
					original := clone(*g.tribe())
					pirate := g.Seafarers.Pirate
					actor := s.Turn
					// Start on an actual numbered tile, so this tests the event exception.
					for _, tile := range g.Tiles {
						if tile.Number > 0 {
							g.Robber = tile.ID
							break
						}
					}
					beginCardEvent(t, s, "robber_flees", 4, 0, 0)
					if s.CatanPendingActor() != actor || s.Turn != actor {
						t.Fatal("wrong flee chooser")
					}
					for viewer := -1; viewer < n; viewer++ {
						v := s.View(viewer)["catan"].(map[string]any)
						legal := v["legal"].(map[string][]int)
						want := []int{}
						if viewer == actor {
							want = deserts
						}
						if !slices.Equal(legal["fleeDeserts"], want) || len(legal["robber"]) != 0 {
							t.Fatal("event locations mixed with ordinary robber choices", viewer, legal)
						}
						for _, card := range v["seafarers"].(map[string]any)["tribe"].(map[string]any)["development"].([]map[string]int) {
							if len(card) != 1 {
								t.Fatal("hidden tribe reward identity exposed", card)
							}
							if _, ok := card["edge"]; !ok {
								t.Fatal("public card location missing")
							}
						}
					}
					helperReject(t, s, (actor+1)%n, Action{Type: "catan_robber_flees", Tile: deserts[0]})
					helperReject(t, s, actor, Action{Type: "catan_robber_flees", Tile: g.Robber})
					helperReject(t, s, actor, Action{Type: "catan_robber", Tile: deserts[0]})
					saved := clone(*s)
					s = &saved
					if automatic {
						s.AutoCatanPending()
					} else {
						helperApply(t, s, actor, Action{Type: "catan_robber_flees", Tile: deserts[len(deserts)-1]})
					}
					g = s.Catan
					if !slices.Contains(deserts, g.Robber) || len(g.Victims) != 0 || g.Seafarers.Pirate != pirate || !reflect.DeepEqual(original, *g.tribe()) {
						t.Fatal("flee stole, moved pirate or altered tribe rewards")
					}
					if s.Phase != "catan_turn" || g.CardEvent != nil || g.RevealedEvent.Production != 4 || !g.RevealedEvent.ProductionStarted {
						t.Fatal("event did not produce and finish")
					}
					cardTribeInventory(t, s.Catan)
					// A later seven must again use numbered hexes, not the event's deserts.
					for p := range g.Players {
						catanMove(g.Players[p].Resources, g.Bank, append([]int{}, g.Players[p].Resources...))
					}
					s.Phase = "catan_roll"
					beginCardEvent(t, s, "robber_attacks", 7, 0, 0)
					g = s.Catan
					if s.Phase != "catan_robber" {
						t.Fatal("seven did not activate robber", s.Phase)
					}
					for _, tile := range deserts {
						if g.robberAllowed(tile) {
							t.Fatal("flee relaxed ordinary tribe restriction")
						}
						helperReject(t, s, actor, Action{Type: "catan_robber", Tile: tile})
					}
					target := -1
					for _, tile := range s.Catan.Tiles {
						if s.Catan.robberAllowed(tile.ID) {
							target = tile.ID
							break
						}
					}
					if target < 0 {
						t.Fatal("no numbered destination")
					}
					helperApply(t, s, actor, Action{Type: "catan_robber", Tile: target})
					if s.Catan.Robber != target || s.Catan.Tiles[target].Number == 0 {
						t.Fatal("ordinary robber left numbered terrain")
					}
					cardTribeInventory(t, s.Catan)
				}
			})
		}
	}
}

func TestCatanCardTribeFleeStaysOnDesertWithoutTheft(t *testing.T) {
	s := cardTribeSetup(t, 3, "fixed")
	desert := s.Catan.Robber
	if s.Catan.Tiles[desert].Resource != CatanDesert {
		t.Fatal("official start must be desert")
	}
	before := clone(s.Catan.Players)
	// Use a number which has no occupied hex; preserving hands proves no theft.
	for i := range s.Catan.Tiles {
		if s.Catan.Tiles[i].Number == 4 {
			s.Catan.Tiles[i].Number = 3
		}
	}
	beginCardEvent(t, s, "robber_flees", 4, 0, 0)
	helperApply(t, s, s.Turn, Action{Type: "catan_robber_flees", Tile: desert})
	if s.Catan.Robber != desert || !reflect.DeepEqual(before, s.Catan.Players) {
		t.Fatal("staying on starting desert changed hands")
	}
}
