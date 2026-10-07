package game

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"
)

func explorerCityHTTPFixture(t *testing.T, n int, phase string) *State {
	t.Helper()
	raw, err := os.ReadFile(fmt.Sprintf("../server/testdata/catan_explorer_city_%s_%d.json", phase, n))
	if err != nil {
		t.Fatal(err)
	}
	var s State
	if err = json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	explorerCityStateRestore(t, &s)
	return &s
}

func TestCatanExplorerCityDepartureReturnsCityAndExplorerHoldings(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, phase := range []string{"roll", "action", "movement"} {
			t.Run(fmt.Sprintf("%d/%s", n, phase), func(t *testing.T) {
				fixture := phase
				if fixture == "movement" {
					fixture = "action"
				}
				s := explorerCityHTTPFixture(t, n, fixture)
				p := s.Turn
				if phase != "roll" {
					ckProgressGive(t, s, p, 12, 13)
					explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 12, Tile: s.Catan.merchantTiles(p)[0]})
					explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 13, Color: 5})
					explorerCityStateAct(t, s, p, explorerCityFlowChoice(t, s, "catan_wall"))
					explorerCityStateAct(t, s, p, explorerCityFlowChoice(t, s, "catan_knight_recruit"))
					explorerCityStateAct(t, s, p, explorerCityFlowChoice(t, s, "catan_knight_activate"))
				} else {
					ckProgressGive(t, s, p, 13)
				}
				if phase == "movement" {
					explorerCityStateAct(t, s, p, Action{Type: "catan_explorer_begin_move"})
				}
				before := clone(*s)
				if err := s.EliminateCatan((p + 1) % n); err == nil || !reflect.DeepEqual(*s, before) {
					t.Fatal("off-turn removal was not atomic")
				}
				if err := s.EliminateCatan(p); err != nil {
					t.Fatal(err)
				}
				explorerCityStateRestore(t, s)
				g, x, k := s.Catan, s.Catan.Explorer, s.Catan.CitiesKnights
				if !g.Players[p].Eliminated || sum(g.Players[p].Resources) != 0 || x.Economy.Gold[p] != 0 || len(k.Players[p].Progress) != 0 || k.Merchant != nil || k.TradePowers != nil {
					t.Fatal("retired player retained mobile holdings")
				}
				if slices.ContainsFunc(k.Knights, func(kn CatanKnight) bool { return kn.Owner == p }) || x.ActionID != before.Catan.Explorer.ActionID+1 || x.Motion != nil {
					t.Fatal("knights or stale animation retained")
				}
				for card, amount := range before.Catan.Players[p].Resources {
					if g.Bank[card] != before.Catan.Bank[card]+amount {
						t.Fatal("card not returned", card)
					}
				}
				if !reflect.DeepEqual(g.Vertices, before.Catan.Vertices) || !reflect.DeepEqual(g.Edges, before.Catan.Edges) || !reflect.DeepEqual(k.Walls, before.Catan.CitiesKnights.Walls) {
					t.Fatal("permanent pieces removed")
				}
				last := k.ProgressEvents[len(k.ProgressEvents)-1]
				if last.Kind != "return" || last.Card != nil || last.Count != len(before.Catan.CitiesKnights.Players[p].Progress) {
					t.Fatal("return animation missing or exposed cards")
				}
				wantSkip := 0
				if phase == "roll" {
					wantSkip = 1
				}
				if x.SkippedRolls != wantSkip || g.RollID != before.Catan.RollID {
					t.Fatal("removal fabricated production")
				}
				start := g.TurnSerial
				for steps := 0; s.Catan.TurnSerial < start+4 && !s.Finished; steps++ {
					if steps > 250 {
						t.Fatal("post-removal game stalled")
					}
					actor := s.CatanPendingActor()
					if actor < 0 {
						actor = s.Turn
					}
					if s.Phase == "catan_discard" {
						for i, due := range s.Catan.DiscardDue {
							if due > 0 {
								actor = i
								break
							}
						}
					}
					if actor == p {
						t.Fatal("retired player asked to act")
					}
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					explorerCityStateAct(t, s, actor, a)
				}
				if sum(s.Catan.Players[p].Resources) != 0 || s.Catan.Explorer.Economy.Gold[p] != 0 || len(s.Catan.CitiesKnights.Players[p].Progress) != 0 {
					t.Fatal("retired player got new production/cards")
				}
				corrupt := clone(*s)
				ckProgressGive(t, &corrupt, p, 5)
				if corrupt.validateCatanExplorer() == nil {
					t.Fatal("restoration accepted retired private hand")
				}
			})
		}
	}
}

func TestCatanExplorerCityDepartureLastSurvivorAndPendingRejection(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerCityHTTPFixture(t, n, "roll")
			removed := 0
			for !s.Finished {
				if removed >= n-1 {
					t.Fatal("did not finish with one survivor")
				}
				if err := s.EliminateCatan(s.Turn); err != nil {
					t.Fatal("removal", removed, s.Phase, err)
				}
				explorerCityStateRestore(t, s)
				removed++
			}
			if removed != n-1 || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Eliminated {
				t.Fatal("wrong last survivor")
			}
			before := clone(*s)
			if err := s.EliminateCatan(s.Turn); err == nil || !reflect.DeepEqual(before, *s) {
				t.Fatal("last player removed")
			}
		})
	}
	s := explorerCityHTTPFixture(t, 3, "action")
	p := s.Turn
	explorerCityStateAct(t, s, p, Action{Type: "catan_progress", Card: 10})
	explorerCityStateAct(t, s, p, Action{Type: "catan_commercial_offer", Card: 0, Target: (p + 1) % 3, Color: 0})
	before := clone(*s)
	for seat := range s.Catan.Players {
		if err := s.EliminateCatan(seat); err == nil || !reflect.DeepEqual(before, *s) {
			t.Fatal("pending player removed", seat)
		}
	}
}
