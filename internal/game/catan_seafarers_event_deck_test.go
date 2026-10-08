package game

import (
	"fmt"
	"reflect"
	"testing"
)

func seaReferenceGame(t *testing.T, n int, scenario string, ready bool) *State {
	t.Helper()
	return seaReferenceGameHelpers(t, n, scenario, ready, false)
}

func seaReferenceGameHelpers(t *testing.T, n int, scenario string, ready, helpers bool) *State {
	t.Helper()
	s, err := newCatanSeafarersReferenceEventsOptions(n, CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers && n%2 == 0}, CatanSeafarersSetup{Scenario: scenario})
	if err != nil {
		t.Fatal(err)
	}
	for ready && s.Catan.setup() {
		actor := ckActor(s)
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, actor, a)
	}
	return s
}

func seaEventConserved(t *testing.T, s *State) {
	t.Helper()
	g := s.Catan
	stock, cards := 19, 25
	if len(g.Players) > 4 {
		stock, cards = 24, 34
	}
	for color, bank := range g.Bank {
		total := bank
		if bank < 0 {
			t.Fatal("negative bank")
		}
		for _, p := range g.Players {
			if p.Resources[color] < 0 {
				t.Fatal("negative hand")
			}
			total += p.Resources[color]
		}
		if total != stock {
			t.Fatalf("resource %d stock=%d want=%d", color, total, stock)
		}
	}
	development := len(g.DevDeck) + len(g.DevDiscard) + len(g.HelperExile)
	if q := g.HelperPending; q != nil && q.Kind == "development" {
		development += len(q.Cards)
	}
	for seat, p := range g.Players {
		development += sum(p.Dev)
		roads, villages, cities := g.pieces(seat)
		if roads > 15 || villages > 5 || cities > 4 || g.shipCount(seat) > 15 {
			t.Fatal("piece supply exceeded")
		}
		for kind, count := range p.NewDev {
			if count < 0 || count > p.Dev[kind] {
				t.Fatal("invalid new development count")
			}
		}
	}
	if development != cards {
		t.Fatalf("development stock=%d want=%d", development, cards)
	}
	for _, e := range g.Edges {
		if g.Vertices[e.A].Level > 0 && g.Vertices[e.B].Level > 0 {
			t.Fatal("adjacent settlements")
		}
	}
}

func TestCatanSeafarersEventDeckAllFaces(t *testing.T) {
	testSeaEventFaces(t, false)
}

func TestCatanSeafarersHelperEventDeckAllFaces(t *testing.T) {
	testSeaEventFaces(t, true)
}

func testSeaEventFaces(t *testing.T, helpers bool) {
	t.Helper()
	for _, scenario := range []string{"shores", "islands", "fog", "desert"} {
		for _, n := range []int{3, 6} {
			for kind := range catanCardEventNames {
				t.Run(fmt.Sprintf("%s/%d/%s", scenario, n, kind), func(t *testing.T) {
					s := seaReferenceGameHelpers(t, n, scenario, true, helpers)
					pirate := s.Catan.Seafarers.Pirate
					if kind == "robber_flees" {
						for _, tile := range s.Catan.Tiles {
							if tile.Resource < CatanDesert {
								s.Catan.Robber = tile.ID
								break
							}
						}
					}
					referenceEventTop(t, s, kind)
					helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
					s = resolveTwoReferenceProduction(t, s)
					if s.Catan.RollID != 1 || s.Catan.RevealedEvent.Kind != kind || !s.Catan.RevealedEvent.ProductionStarted {
						t.Fatal("event did not produce once")
					}
					if kind == "robber_flees" {
						if len(s.Catan.fleeDeserts()) == 0 && s.Catan.Robber != -1 || s.Catan.Robber >= 0 && s.Catan.Tiles[s.Catan.Robber].Resource != CatanDesert || s.Catan.Seafarers.Pirate != pirate {
							t.Fatal("flee must use a revealed desert or frame, without moving pirate")
						}
					}
					for _, edge := range s.Catan.Edges {
						if edge.Ship && edge.Damaged {
							t.Fatal("earthquake damaged a ship")
						}
					}
					seaEventConserved(t, s)
					s = referenceEventRestore(t, s)
					if n == 6 {
						helperApply(t, s, s.Turn, Action{Type: "catan_end"})
						if !s.Catan.Paired.Second || s.Catan.RollID != 1 || s.Phase != "catan_turn" {
							t.Fatal("paired handoff redrew or lost action phase")
						}
						helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
					}
				})
			}
		}
	}
}

func TestCatanSeafarersEventDeckNaturalMatches(t *testing.T) {
	testSeaEventMatches(t, false)
}

func TestCatanSeafarersHelperEventDeckNaturalMatches(t *testing.T) {
	testSeaEventMatches(t, true)
}

func testSeaEventMatches(t *testing.T, helpers bool) {
	t.Helper()
	for _, scenario := range []string{"shores", "islands", "fog", "desert"} {
		for _, n := range []int{3, 4, 5, 6} {
			t.Run(fmt.Sprintf("%s/%d", scenario, n), func(t *testing.T) {
				s := seaReferenceGameHelpers(t, n, scenario, false, helpers)
				for step := 0; !s.Finished && step < 12000; step++ {
					actor := ckActor(s)
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatalf("step=%d phase=%s: %v", step, s.Phase, err)
					}
					if a.Type == "catan_roll" {
						other := clone(*s)
						pile := other.Catan.EventDeck.Deck.DrawPile
						pile[0], pile[1] = pile[1], pile[0]
						b, err := other.BotAction(actor)
						if err != nil || !reflect.DeepEqual(a, b) || !reflect.DeepEqual(s.View(-1), other.View(-1)) {
							t.Fatal("private card order affected bot or spectator")
						}
					}
					before := s.Catan.RollID
					if err := s.Apply(actor, a); err != nil {
						t.Fatalf("step=%d phase=%s action=%+v: %v", step, s.Phase, a, err)
					}
					if a.Type != "catan_roll" && s.Catan.RollID != before {
						t.Fatal("nonproduction action drew a card")
					}
					seaEventConserved(t, s)
					if step%31 == 0 || s.CatanPendingActor() >= 0 {
						s = referenceEventRestore(t, s)
					}
				}
				if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < s.Catan.Seafarers.VictoryPoints {
					t.Fatal("natural game failed to reach scenario victory")
				}
				referenceEventRestore(t, s)
			})
		}
	}
}

func TestCatanSeafarersEventDeckRejectsCorrupt(t *testing.T) {
	seed := seaReferenceGame(t, 3, "fog", true)
	for name, mutate := range map[string]func(*Catan){
		"rules":    func(g *Catan) { g.Seafarers.Rules = "unknown" },
		"layout":   func(g *Catan) { g.Seafarers.Layout = "" },
		"fog":      func(g *Catan) { g.Seafarers.Fog = nil },
		"scenario": func(g *Catan) { g.Seafarers.Scenario = "shores" },
		"helpers":  func(g *Catan) { g.Options.Helpers = true },
	} {
		t.Run(name, func(t *testing.T) {
			bad := clone(*seed)
			mutate(bad.Catan)
			if bad.validateCatanEventSession() == nil {
				t.Fatal("corrupt session accepted")
			}
			helperReject(t, &bad, bad.Turn, Action{Type: "catan_roll"})
		})
	}
}

func TestCatanSeafarersEventDeckGoldAfterRewardsAndEpidemic(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, kind := range []string{"plentiful_year", "epidemic"} {
			t.Run(fmt.Sprintf("%d/%s", n, kind), func(t *testing.T) {
				s := seaReferenceGame(t, n, "shores", true)
				id := referenceEventTop(t, s, kind)
				g := s.Catan
				// Isolated midgame fixture on the actual map: one city beside
				// gold. Preserve bank/hand/deck quantities, not a natural match.
				for v := range g.Vertices {
					g.Vertices[v].Owner, g.Vertices[v].Level = -1, 0
				}
				found := false
				for i, tile := range g.Tiles {
					if tile.Resource == CatanGold {
						g.Tiles[i].Number = catanEventReferenceFaces[id].Production
						g.Vertices[tile.Vertices[0]].Owner, g.Vertices[tile.Vertices[0]].Level = s.Turn, 2
						found = true
						break
					}
				}
				if !found {
					t.Fatal("map has no gold")
				}
				s.catanScores()
				helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
				for s.Catan.CardEvent != nil {
					if s.Catan.GoldPending != nil {
						t.Fatal("gold production ran before event rewards finished")
					}
					actor := ckActor(s)
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					helperApply(t, s, actor, a)
				}
				want := 2
				if kind == "epidemic" {
					want = 1
				}
				q := s.Catan.GoldPending
				if s.Phase != "catan_gold" || q == nil || len(q.Claims) != 1 || q.Claims[0].Player != s.Turn || q.Claims[0].Count != want || s.Catan.RollID != 1 {
					t.Fatalf("wrong gold continuation: phase=%s pending=%+v", s.Phase, q)
				}
				s = referenceEventRestore(t, s)
				before := sum(s.Catan.Players[s.Turn].Resources)
				helperReject(t, s, (s.Turn+1)%n, Action{Type: "catan_gold", Take: []int{want, 0, 0, 0, 0}})
				helperApply(t, s, s.Turn, Action{Type: "catan_gold", Take: []int{want, 0, 0, 0, 0}})
				if sum(s.Catan.Players[s.Turn].Resources)-before != want || s.Phase != "catan_turn" || s.Catan.RollID != 1 || s.Catan.GoldPending != nil {
					t.Fatal("gold did not resume original action exactly once")
				}
				helperReject(t, s, s.Turn, Action{Type: "catan_gold", Take: []int{want, 0, 0, 0, 0}})
				seaEventConserved(t, s)
				referenceEventRestore(t, s)
			})
		}
	}
}
