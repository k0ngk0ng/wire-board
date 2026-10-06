package game

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"
)

func TestCatanAttackNaturalMatches(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := newAttackState(t, n, true)
			phases := map[string]int{}
			battles, ends := 0, 0
			for step := 0; !s.Finished && step < 4000; step++ {
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
				phases[s.Phase]++
				a, err := s.BotAction(actor)
				if err == nil {
					err = s.Apply(actor, a)
				}
				if err != nil {
					if out := os.Getenv("ATTACK_QA_TRACE"); out != "" {
						data, _ := json.MarshalIndent(s, "", "  ")
						_ = os.WriteFile(out+fmt.Sprintf("/failed-%d.json", n), data, 0600)
					}
					t.Fatalf("step=%d round=%d phase=%s actor=%d action=%+v error=%v", step, s.Round, s.Phase, actor, a, err)
				}
				if err = s.validateCatanAttack(); err != nil {
					t.Fatal(step, err)
				}
				g, attack := s.Catan, s.Catan.Attack
				if attack.EndSequence != ends {
					ends = attack.EndSequence
					battles += len(attack.End.Battles)
				}
				for player, piece := range g.Players {
					score := attack.Prisoners[player] / 2
					if g.LongestOwner == player {
						score += 2
					}
					for _, v := range g.Vertices {
						if v.Owner != player || v.Level == 0 {
							continue
						}
						conquered := true
						for _, tile := range g.Tiles {
							if slices.Contains(tile.Vertices, v.ID) && attack.Barbarians[tile.ID] != 3 {
								conquered = false
							}
						}
						if !conquered {
							score += v.Level
						}
					}
					if piece.Score != score {
						t.Fatalf("stale score at step %d seat %d: got %d want %d", step, player, piece.Score, score)
					}
				}
				if step%31 == 0 {
					assertAttackRestored(t, s)
				}
			}
			if !s.Finished {
				t.Fatalf("no natural finish after 4000 actions: round=%d phases=%v", s.Round, phases)
			}
			if len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 12 {
				t.Fatal("not a genuine points victory")
			}
			t.Logf("round=%d winner=%d score=%d battles=%d landings=%d phases=%v", s.Round, s.Winners[0], s.Catan.Players[s.Winners[0]].Score, battles, s.Catan.Attack.Sequence, phases)
		})
	}
}
