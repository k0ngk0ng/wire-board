package game

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCatanCitiesKnightsBotsComplete(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprintf("%d", n), func(t *testing.T) {
			s := ckGame(t, n)
			used := map[int]int{}
			steps := 0
			for ; steps < 8000 && !s.Finished; steps++ {
				actor := s.Turn
				if pending := s.CatanPendingActor(); pending >= 0 {
					actor = pending
				} else if s.Phase == "catan_discard" {
					for p, due := range s.Catan.DiscardDue {
						if due > 0 {
							actor = p
							break
						}
					}
				}
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal("bot action", steps, s.Phase, actor, err)
				}
				if err = s.Apply(actor, a); err != nil {
					t.Fatal("bot produced illegal action", steps, s.Phase, actor, a, err)
				}
				if a.Type == "catan_progress" {
					used[a.Card]++
				}
				g := s.Catan
				ckProgressStock(t, g)
				ckKnightStock(t, g)
				if g.ArmyOwner != -1 || len(g.DevDeck) != 0 {
					t.Fatal("base development/army accidentally enabled")
				}
				for p := range g.Players {
					roads, _, _ := g.pieces(p)
					if roads > 15 || g.settlementPiecesLeft(p) < 0 || g.cityPiecesLeft(p) < 0 || g.catanDiscardLimit(p) > 13 {
						t.Fatal("physical inventory")
					}
				}
				for _, e := range g.Edges {
					if g.Vertices[e.A].Level > 0 && g.Vertices[e.B].Level > 0 {
						t.Fatal("building distance")
					}
				}
				if steps%37 == 0 {
					saved := clone(*s)
					if !reflect.DeepEqual(*s, saved) {
						t.Fatal("persistence changed live game")
					}
					s = &saved
				}
			}
			if !s.Finished || len(s.Winners) != 1 {
				t.Fatalf("did not finish: steps=%d round=%d phase=%s used=%v players=%+v", steps, s.Round, s.Phase, used, s.Catan.Players)
			}
			if s.Catan.Players[s.Winners[0]].Score < 13 || s.Catan.CitiesKnights.Invasions == 0 || len(used) == 0 {
				t.Fatal("missing expansion play or wrong victory")
			}
			t.Logf("steps=%d round=%d invasions=%d progress=%v", steps, s.Round, s.Catan.CitiesKnights.Invasions, used)
		})
	}
}
