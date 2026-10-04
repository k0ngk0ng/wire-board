package game

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCatanWondersBotsComplete(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, helpers := range []bool{false, true} {
			for _, variable := range []bool{false, true} {
				if n > 4 && variable {
					continue
				}
				t.Run(fmt.Sprintf("%d/helpers=%v/variable=%v", n, helpers, variable), func(t *testing.T) {
					s, err := NewCatanWonders(n, CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers})
					if err != nil {
						t.Fatal(err)
					}
					if variable {
						if err = s.randomizeCatanSeafarersMap(); err != nil {
							t.Fatal(err)
						}
					}
					initialDev := len(s.Catan.DevDeck)
					claims, builds := 0, 0
					for steps := 0; steps < 10000 && !s.Finished; steps++ {
						actor := s.Turn
						if pending := s.CatanPendingActor(); pending >= 0 {
							actor = pending
						} else if s.Phase == "catan_discard" {
							for i, due := range s.Catan.DiscardDue {
								if due > 0 {
									actor = i
									break
								}
							}
						}
						a, err := s.BotAction(actor)
						if err != nil {
							t.Fatal(steps, s.Phase, actor, err)
						}
						if a.Type == "catan_wonder_claim" {
							claims++
						}
						if a.Type == "catan_wonder_build" {
							builds++
						}
						helperApply(t, s, actor, a)
						g := s.Catan
						fleetSupply(t, g)
						dev := len(g.DevDeck) + len(g.DevDiscard) + len(g.HelperExile)
						if g.HelperPending != nil {
							dev += len(g.HelperPending.Cards)
						}
						for player, p := range g.Players {
							dev += sum(p.Dev)
							roads, settlements, cities := g.pieces(player)
							if roads > 15 || settlements > 5 || cities > 4 || g.shipCount(player) > 15 {
								t.Fatal("piece inventory", player)
							}
						}
						if dev != initialDev {
							t.Fatal("development inventory", dev, initialDev)
						}
						owners := map[int]bool{}
						for _, card := range g.wonders().Cards {
							if card.Level < 0 || card.Level > 4 || (card.Owner < 0 && card.Level > 0) || (card.Owner >= 0 && owners[card.Owner]) {
								t.Fatal("wonder exclusivity or level", card)
							}
							if card.Owner >= 0 {
								owners[card.Owner] = true
							}
						}
						for _, edge := range g.Edges {
							if g.Vertices[edge.A].Level > 0 && g.Vertices[edge.B].Level > 0 {
								t.Fatal("adjacent buildings")
							}
						}
						if steps%29 == 0 {
							saved := clone(*s)
							if !reflect.DeepEqual(*s, saved) {
								t.Fatal("save roundtrip")
							}
							s = &saved
						}
					}
					if !s.Finished || len(s.Winners) != 1 || claims == 0 || builds == 0 || !s.Catan.wonderVictory(s.Winners[0]) {
						t.Fatal("no valid wonder victory", s.Round, s.Phase, s.Catan.wonders().Cards)
					}
					t.Logf("rounds=%d claims=%d levels=%d winner=%d", s.Round, claims, builds, s.Winners[0])
				})
			}
		}
	}
}

func TestCatanWondersVariableLayout(t *testing.T) {
	for _, n := range []int{3, 4} {
		for repeat := 0; repeat < 60; repeat++ {
			s := wondersGame(t, n, true)
			before := clone(*s)
			if err := s.randomizeCatanSeafarersMap(); err != nil {
				t.Fatal(err)
			}
			g := s.Catan
			if !g.Seafarers.Variable || !reflect.DeepEqual(g.wonders(), before.Catan.wonders()) || g.Robber != before.Catan.Robber {
				t.Fatal("variable layout changed markers or robber")
			}
			oldTerrain, oldNumbers := make([]int, 8), make([]int, 13)
			newTerrain, newNumbers := make([]int, 8), make([]int, 13)
			for i, tile := range g.Tiles {
				old := before.Catan.Tiles[i]
				oldTerrain[old.Resource]++
				oldNumbers[old.Number]++
				newTerrain[tile.Resource]++
				newNumbers[tile.Number]++
				if old.Resource >= CatanDesert || g.Seafarers.Islands[i] != g.Seafarers.StartIslands[0] {
					if !reflect.DeepEqual(tile, old) {
						t.Fatal("shuffled fixed sea, desert or islet", i)
					}
				}
			}
			if !reflect.DeepEqual(oldTerrain, newTerrain) || !reflect.DeepEqual(oldNumbers, newNumbers) {
				t.Fatal("variable inventory changed")
			}
			for _, e := range g.Edges {
				ids := g.edgeTiles(e.ID)
				if len(ids) != 2 {
					continue
				}
				a, b := g.Tiles[ids[0]], g.Tiles[ids[1]]
				if (a.Resource == CatanDesert && (b.Number == 6 || b.Number == 8)) || (b.Resource == CatanDesert && (a.Number == 6 || a.Number == 8)) {
					t.Fatal("red next to desert")
				}
			}
		}
	}
	for _, n := range []int{5, 6} {
		s := wondersGame(t, n, true)
		before := clone(*s)
		if err := s.randomizeCatanSeafarersMap(); err == nil || !reflect.DeepEqual(*s, before) {
			t.Fatal("unprinted five/six variable recipe accepted or mutated state")
		}
	}
}
