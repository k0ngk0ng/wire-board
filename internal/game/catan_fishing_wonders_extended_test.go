package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanFishingWondersExtendedComponentsAndGates(t *testing.T) {
	for _, n := range []int{5, 6} {
		s := fishWondersGame(t, n, "fixed")
		g := s.Catan
		terrain, numbers, fish, dev := make([]int, 8), make([]int, 13), make([]int, 4), make([]int, 5)
		grounds := []int{}
		for _, tile := range g.Tiles {
			terrain[tile.Resource]++
			if tile.Number > 0 {
				numbers[tile.Number]++
			}
		}
		for _, id := range g.Fishing.Tokens.DrawPile {
			fish[catanFishValue(id)]++
		}
		for _, card := range g.DevDeck {
			dev[card]++
		}
		for _, ground := range g.Fishing.Map.Grounds {
			grounds = append(grounds, ground.Number)
		}
		slices.Sort(grounds)
		// Seafarers extension p.11, T&B extension p.5, combination p.2.
		if !slices.Equal(terrain, []int{7, 6, 7, 6, 6, 4, 24, 3}) ||
			!slices.Equal(numbers, []int{0, 0, 2, 3, 4, 4, 4, 0, 4, 4, 4, 4, 2}) ||
			!slices.Equal(fish, []int{1, 15, 15, 13}) ||
			!slices.Equal(dev, []int{20, 3, 3, 3, 5}) ||
			!slices.Equal(grounds, []int{4, 5, 5, 6, 8, 9, 9, 10}) {
			t.Fatal("extended printed components", terrain, numbers, fish, dev, grounds)
		}
		if len(g.Ports) != 11 || len(g.wonders().Cards) != 7 || len(g.wonders().Markers) != 9 || len(g.wonders().SetupBlocked) != 17 || len(g.wonderStartTiles()) != 4 {
			t.Fatal("ports, wonders, markers or start deserts")
		}
		for _, change := range []func(*Catan){
			func(g *Catan) { g.Paired = nil },
			func(g *Catan) { g.Options.FiveSix = false },
			func(g *Catan) { g.Seafarers.Variable = true },
			func(g *Catan) { g.Seafarers.Pirate = 1 },
			func(g *Catan) { g.Fishing.Map.Grounds = g.Fishing.Map.Grounds[:6] },
			func(g *Catan) { g.Fishing.Map.Grounds[1].Number = 4 },
			func(g *Catan) { g.CitiesKnights = &CatanCitiesKnights{} },
		} {
			copy := clone(*s)
			change(copy.Catan)
			if copy.Catan.validateFishing() == nil {
				t.Fatal("invalid extended combination accepted")
			}
		}
		for _, options := range []CatanOptions{{}, {FiveSix: true, AllHelpers: true}} {
			if _, err := NewCatanFishingSeafarers(n, options, CatanSeafarersSetup{Scenario: "wonders"}, nil); err == nil {
				t.Fatal("unsupported options accepted")
			}
		}
		if _, err := NewCatanFishingSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "wonders", Layout: "variable"}, nil); err == nil {
			t.Fatal("unsupported variable layout accepted")
		}
		fleetSupply(t, g)
	}
}

func TestCatanFishingWondersExtendedDuplicateProduction(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, number := range []int{5, 9} {
			for _, full := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/roll%d/full%v", n, number, full), func(t *testing.T) {
					s := fishWondersGame(t, n, "fixed")
					g := s.Catan
					s.Turn, s.Phase = n-1, "catan_roll"
					g.SetupStep, g.TurnSerial, g.RollID, g.Robber = g.SetupLimit(), 1, 1, -1
					g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = n-1, (n+2)%n, false
					// Explicit production fixture: distance-legal city/village on
					// the two matching grounds. No claim of a full-game transcript.
					player := 0
					for _, ground := range g.Fishing.Map.Grounds {
						if ground.Number != number {
							continue
						}
						placed := false
						for _, vertex := range ground.Vertices {
							if g.canSettlement(player, vertex, true) {
								g.Vertices[vertex].Owner, g.Vertices[vertex].Level = player, 2-player
								placed = true
								break
							}
						}
						if !placed {
							t.Fatal("no distance-legal production fixture")
						}
						player++
					}
					if player != 2 {
						t.Fatal("missing duplicate ground")
					}
					want := make([]int, n)
					want[0], want[1] = 2, 1
					due, err := g.Fishing.Map.production(g, number)
					if err != nil || !slices.Equal(due, want) {
						t.Fatal("duplicate-ground production", due, err)
					}
					if full {
						fishOwn(&g.Fishing.Tokens, 0, 0, 1, 2, 3, 4, 5, 6)
						fishOwn(&g.Fishing.Tokens, 1, 11, 12, 13, 14, 15, 16, 17)
					}
					fishTop(&g.Fishing.Tokens, 21, 22, 23)
					pile := len(g.Fishing.Tokens.DrawPile)
					if err := s.catanRollProduction(number); err != nil {
						t.Fatal(err)
					}
					if full {
						for p := range 2 {
							if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != p {
								t.Fatal("clockwise response order/cap")
							}
							copy := clone(*s)
							s = &copy
							helperReject(t, s, (p+1)%n, Action{Type: "catan_fish_keep"})
							helperApply(t, s, p, Action{Type: "catan_fish_replace", Card: s.Catan.Fishing.Tokens.Hands[p][0]})
						}
					}
					wantDraws := 3
					if full {
						wantDraws = 2
					}
					if pile-len(s.Catan.Fishing.Tokens.DrawPile) != wantDraws || s.Phase != "catan_turn" || s.Turn != n-1 {
						t.Fatal("wrong payout/continuation")
					}
					before := clone(*s)
					if err := s.catanRollProduction(number); err == nil || !reflect.DeepEqual(before, *s) {
						t.Fatal("repeated production")
					}
					tokens := clone(s.Catan.Fishing.Tokens)
					helperApply(t, s, s.Turn, Action{Type: "catan_end"})
					if s.Turn != (n+2)%n || !s.Catan.Paired.Second || s.Phase != "catan_turn" || !reflect.DeepEqual(tokens, s.Catan.Fishing.Tokens) || s.Catan.RollID != 1 {
						t.Fatal("secondary action produced fish")
					}
					helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
					fleetSupply(t, s.Catan)
				})
			}
		}
	}
}
