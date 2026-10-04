package game

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCatanPirateIslandsBotsComplete(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, helpers := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/helpers=%v", n, helpers), func(t *testing.T) {
				s, err := NewCatan(n, CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers})
				if err != nil {
					t.Fatal(err)
				}
				build := (*Catan).makeSeafarersPirateIslandsFour
				if n > 4 {
					build = (*Catan).makeSeafarersPirateIslandsSix
				}
				if err = build(s.Catan); err != nil {
					t.Fatal(err)
				}
				totalDev := len(s.Catan.DevDeck)
				battles := 0
				for steps := 0; steps < 12000 && !s.Finished; steps++ {
					player := s.Turn
					if actor := s.CatanPendingActor(); actor >= 0 {
						player = actor
					} else if s.Phase == "catan_discard" {
						for i, due := range s.Catan.DiscardDue {
							if due > 0 {
								player = i
								break
							}
						}
					}
					a, err := s.BotAction(player)
					if err != nil {
						t.Fatal(steps, s.Phase, player, err)
					}
					helperApply(t, s, player, a)
					g := s.Catan
					fleetSupply(t, g)
					dev := len(g.DevDeck) + len(g.DevDiscard) + len(g.HelperExile)
					if g.HelperPending != nil {
						dev += len(g.HelperPending.Cards)
					}
					for i, p := range g.Players {
						dev += sum(p.Dev)
						roads, settlements, cities := g.pieces(i)
						if roads > 15 || settlements > 5 || cities > 4 || g.shipCount(i) > 15 {
							t.Fatal("piece inventory", i, roads, settlements, cities, g.shipCount(i))
						}
						if _, ok := g.pirateRouteVertices(i); !ok {
							t.Fatal("broken navy", i)
						}
						for kind, count := range p.NewDev {
							if count < 0 || count > p.Dev[kind] {
								t.Fatal("invalid new development")
							}
						}
					}
					if dev != totalDev {
						t.Fatal("development conservation", dev, totalDev)
					}
					if g.LongestOwner != -1 || g.ArmyOwner != -1 {
						t.Fatal("awards must be absent")
					}
					for _, e := range g.Edges {
						if e.Warship && (!e.Ship || e.Owner < 0) {
							t.Fatal("orphan warship")
						}
						if g.Vertices[e.A].Level > 0 && g.Vertices[e.B].Level > 0 {
							t.Fatal("adjacent buildings")
						}
					}
					if b := g.pirateIslands().Battle; b != nil {
						battles = b.ID
					}
					if steps%31 == 0 {
						saved := clone(*s)
						if !reflect.DeepEqual(*s, saved) {
							t.Fatal("save changed state")
						}
						s = &saved
					}
				}
				if !s.Finished || len(s.Winners) != 1 || battles < 3 {
					t.Fatalf("bots did not finish: round=%d phase=%s battles=%d scores=%v routes=%v", s.Round, s.Phase, battles, s.Catan.Players, s.Catan.pirateIslands().Fortresses)
				}
				winner := s.Winners[0]
				if s.Catan.Players[winner].Score < 10 || s.Catan.pirateIslands().Fortresses[winner].Strength != 0 {
					t.Fatal("invalid winner")
				}
				t.Logf("round=%d battles=%d winner=%d", s.Round, battles, winner)
			})
		}
	}
}
