package game

import (
	"fmt"
	"slices"
	"testing"
)

func TestCatanExplorerFishingNaturalMatches(t *testing.T) {
	for _, cfg := range []struct {
		n              int
		scenario       string
		cities, events bool
	}{
		{2, "land-ho", false, false},
		{3, "explorers-and-pirates", false, true},
		{3, "fish-for-catan", true, false},
		{6, "explorers-and-pirates", true, true},
	} {
		t.Run(fmt.Sprintf("%d/%s/cities%t/events%t", cfg.n, cfg.scenario, cfg.cities, cfg.events), func(t *testing.T) {
			s := explorerFishingGame(t, cfg.n, cfg.scenario, cfg.cities, true, cfg.events)
			actions := map[string]int{}
			for step := 0; step < 12000 && !s.Finished; step++ {
				actor := s.CatanPendingActor()
				if actor < 0 {
					actor = s.Turn
				}
				if s.Phase == "catan_discard" {
					for p, due := range s.Catan.DiscardDue {
						if due > 0 {
							actor = p
							break
						}
					}
				}
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				if step%97 == 0 {
					// A bot may inspect its own fish, never the blind pile.
					other := clone(*s)
					slices.Reverse(other.Catan.Fishing.Tokens.DrawPile)
					b, e := other.BotAction(actor)
					if e != nil || fmt.Sprint(a) != fmt.Sprint(b) {
						t.Fatal("fish bot depends on blind pile", a, b, e)
					}
				}
				if err = s.Apply(actor, a); err != nil {
					t.Fatalf("step %d phase %s action %+v: %v", step, s.Phase, a, err)
				}
				actions[a.Type]++
				if step%97 == 0 {
					explorerFishingRestore(t, s)
				}
			}
			t.Log("round", s.Round, "actions", actions)
			if !s.Finished || len(s.Winners) != 1 {
				t.Fatal("natural match did not finish", s.Phase, s.Catan.TurnSerial)
			}
			if s.Catan.Players[s.Winners[0]].Score < s.Catan.victoryTargetFor(s.Winners[0]) {
				t.Fatal("wrong winner threshold")
			}
			explorerFishingRestore(t, s)
		})
	}
}
