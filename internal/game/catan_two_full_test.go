package game

import (
	"encoding/json"
	"fmt"
	"testing"
)

func twoFullActor(s *State) int {
	if actor := s.CatanPendingActor(); actor >= 0 {
		return actor
	}
	if s.Phase == "catan_discard" {
		for player, due := range s.Catan.DiscardDue {
			if due > 0 {
				return player
			}
		}
	}
	return s.Turn
}

func assertTwoFullState(t *testing.T, s *State) {
	t.Helper()
	catanCheck(t, s)
	if err := s.validateCatanTwo(); err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	for p, seat := range g.Players {
		score := seat.Dev[4]
		for _, v := range g.Vertices {
			if v.Owner == p {
				score += v.Level
			}
		}
		if g.LongestOwner == p {
			score += 2
		}
		if g.ArmyOwner == p {
			score += 2
		}
		if score != seat.Score || seat.Eliminated {
			t.Fatal("natural game score/elimination", p, score, seat.Score)
		}
	}
	if s.Finished && (len(s.Winners) != 1 || s.Winners[0] != s.Turn || g.Players[s.Turn].Score < 10) {
		t.Fatal("invalid natural victory")
	}
}

func TestCatanTwoCompleteEngineGames(t *testing.T) {
	coverage := map[string]int{}
	for sample := range 4 {
		t.Run(fmt.Sprint(sample), func(t *testing.T) {
			s, err := newCatanTwoCore()
			if err != nil {
				t.Fatal(err)
			}
			steps, restores, automatic := 0, 0, 0
			seenPhase := map[string]bool{}
			for ; steps < 5000 && !s.Finished; steps++ {
				actor := twoFullActor(s)
				if !seenPhase[s.Phase] || steps%43 == 0 {
					twoCoreRestore(t, s)
					restores++
					seenPhase[s.Phase] = true
				}
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal(steps, s.Phase, err)
				}
				coverage[a.Type]++
				if s.CatanPendingActor() >= 0 && steps%3 == 0 {
					before, _ := json.Marshal(s)
					s.AutoCatanPending()
					after, _ := json.Marshal(s)
					if string(before) == string(after) {
						t.Fatal("automatic pending stalled", steps, s.Phase)
					}
					automatic++
				} else if err = s.Apply(actor, a); err != nil {
					t.Fatalf("step=%d phase=%s action=%+v tokens=%+v: %v", steps, s.Phase, a, s.Catan.Two, err)
				}
				assertTwoFullState(t, s)
				if !s.Finished && steps%17 == 0 {
					for _, viewer := range []int{-1, 0, 1} {
						v := s.View(viewer)["catan"].(map[string]any)
						if v["devDeck"] != nil || v["devDiscard"] != nil {
							t.Fatal("private deck")
						}
						for p, raw := range v["players"].([]any) {
							seat := raw.(map[string]any)
							if p != viewer && (seat["resources"] != nil || seat["dev"] != nil) {
								t.Fatal("private hand")
							}
						}
						if q := s.Catan.Two.Trade; q != nil && viewer != s.Turn && v["two"].(map[string]any)["trade"].(map[string]any)["drawn"] != nil {
							t.Fatal("private forced draw")
						}
					}
				}
			}
			if !s.Finished {
				t.Fatal("natural game did not finish", steps, s.Phase, s.Catan.Players)
			}
			twoCoreRestore(t, s)
			helperReject(t, s, s.Turn, Action{Type: "catan_end"})
			t.Logf("steps=%d round=%d restores=%d automatic=%d", steps, s.Round, restores, automatic)
		})
	}
	for _, action := range []string{"catan_roll", "catan_two_build", "catan_two_trade", "catan_two_return", "catan_two_robber", "catan_city", "catan_settlement"} {
		if coverage[action] == 0 {
			t.Fatal("full-game path not exercised", action)
		}
	}
	t.Logf("coverage=%v", coverage)
}

func TestCatanTwoConstructorAndVersionGate(t *testing.T) {
	for _, n := range []int{0, 1, 3, 4, 5, 6} {
		if _, err := NewCatanTwo(n, CatanOptions{}); err == nil {
			t.Fatal("unsupported count", n)
		}
	}
	for _, o := range []CatanOptions{{Helpers: true}, {FiveSix: true}, {AllHelpers: true}, {Rules: "unknown"}} {
		if _, err := NewCatanTwo(2, o); err == nil {
			t.Fatal("unsupported combination", o)
		}
	}
	s, err := NewCatanTwo(2, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Turn != s.Catan.StartPlayer || s.Catan.Two.Rules != CatanTwoRules {
		t.Fatal("initial player/version")
	}
	s.Catan.Two.Rules = "stale"
	helperReject(t, s, s.Turn, Action{Type: "catan_settlement", Vertex: 0})
	if _, err := NewCatan(2, CatanOptions{}); err == nil {
		t.Fatal("unfinished public variant exposed")
	}
}
