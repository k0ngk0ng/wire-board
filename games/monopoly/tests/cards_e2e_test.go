package monopoly_test

// Failure scenarios written before card implementation: a missing/duplicate
// card; client-controlled draws; leaked deck order; held jail cards duplicated;
// missing GO salary or double salary on backward/jail movement; nearest-station
// or utility rent modifier leaking into later moves; repairs counting a hotel as
// five houses; pending multi-player payments lost or replayed after restart;
// bankruptcy paying every creditor instead of the current one; an eliminated
// collector receiving later payments; endless final survivor inheritance fees.

import (
	"fmt"
	"testing"

	monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
)

func topCard(s *monopoly.State, deck, id int) {
	ordered := []int{id}
	for _, card := range s.Decks[deck] {
		if card != id {
			ordered = append(ordered, card)
		}
	}
	s.Decks[deck] = ordered
}
func cardFixture(s *monopoly.State, deck, id int) {
	s.Phase = "roll"
	s.Players[0].Position = 4
	if deck == 1 {
		s.Players[0].Position = 14
	}
	dice(s, 5)
	topCard(s, deck, id)
}

func TestMonopolyEveryCardHTTP(t *testing.T) {
	type expected struct {
		cash, position int
		phase          string
	}
	chance := []expected{{1500, 7, "end"}, {1500, 7, "end"}, {1485, 7, "end"}, {1400, 7, "end"}, {1300, 4, "end"}, {1500, 12, "purchase"}, {1550, 7, "end"}, {1500, 15, "purchase"}, {1700, 0, "end"}, {1700, 5, "purchase"}, {1500, 39, "purchase"}, {1500, 24, "purchase"}, {1650, 7, "end"}, {1500, 15, "purchase"}, {1500, 11, "purchase"}, {1500, 10, "end"}}
	chest := []expected{{1500, 17, "end"}, {1510, 17, "end"}, {1550, 17, "end"}, {1600, 17, "end"}, {1520, 17, "end"}, {1600, 17, "end"}, {1600, 17, "end"}, {1525, 17, "end"}, {1400, 17, "end"}, {1700, 17, "end"}, {1450, 17, "end"}, {1450, 17, "end"}, {1520, 17, "end"}, {1700, 0, "end"}, {1500, 17, "end"}, {1500, 10, "end"}}
	for deck, cases := range [][]expected{chance, chest} {
		for id, want := range cases {
			t.Run(fmt.Sprintf("deck%d-card%d", deck, id), func(t *testing.T) {
				tb := newTable(t, 3, func(s *monopoly.State) { cardFixture(s, deck, id) })
				a := tb.people[0]
				a.act(map[string]any{"type": "roll"}, 200)
				r := a.room()
				if money(r, 0) != want.cash || player(r, 0)["position"] != float64(want.position) || data(r)["phase"] != want.phase {
					t.Fatal("card effect mismatch", r)
				}
				last := data(r)["lastCard"].(map[string]any)
				if last["id"] != float64(id) || last["player"] != float64(0) {
					t.Fatal("drawn card not projected", last)
				}
				if id == 0 && len(player(r, 0)["jailCards"].([]any)) != 1 {
					t.Fatal("jail card not retained")
				}
				if id == 15 && player(r, 0)["jailed"] != true {
					t.Fatal("jail card movement not applied")
				}
				tb.record("draw resolved")
			})
		}
	}
}

func TestMonopolyCardChainAndSpecialRentHTTP(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		setup                     func(*monopoly.State)
		cash, ownerCash, position int
		phase                     string
	}{
		{"forward_go_twice", func(s *monopoly.State) { cardFixture(s, 1, 13); s.Players[0].Position = 39 }, 1900, 1500, 0, "end"},
		{"jail_skips_go_salary", func(s *monopoly.State) { cardFixture(s, 0, 15); s.Players[0].Position = 33 }, 1500, 1500, 10, "end"},
		{"back_to_chest_then_go", func(s *monopoly.State) { cardFixture(s, 0, 4); s.Players[0].Position = 33; topCard(s, 1, 13) }, 1700, 1500, 0, "end"},
		{"nearest_station_double_rent", func(s *monopoly.State) {
			cardFixture(s, 0, 7)
			s.Properties[15].Owner = 1
			s.Properties[5].Owner = 1
			s.Properties[5].Mortgaged = true
		}, 1400, 1600, 15, "end"},
		{"nearest_utility_tenfold", func(s *monopoly.State) { cardFixture(s, 0, 5); s.Properties[12].Owner = 1 }, 1420, 1580, 12, "end"},
		{"nearest_station_wrap_go", func(s *monopoly.State) { cardFixture(s, 0, 13); s.Players[0].Position = 33; s.Properties[5].Owner = 1 }, 1650, 1550, 5, "end"},
		{"mortgaged_station_no_rent", func(s *monopoly.State) {
			cardFixture(s, 0, 7)
			s.Properties[15].Owner = 1
			s.Properties[15].Mortgaged = true
		}, 1500, 1500, 15, "end"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb := newTable(t, 3, tc.setup)
			tb.people[0].act(map[string]any{"type": "roll"}, 200)
			r := tb.people[0].room()
			if money(r, 0) != tc.cash || money(r, 1) != tc.ownerCash || player(r, 0)["position"] != float64(tc.position) || data(r)["phase"] != tc.phase {
				t.Fatal("chained card resolution", r)
			}
			tb.restart()
			tb.record("card chain persisted")
		})
	}
}

func TestMonopolyCardRepairsHTTP(t *testing.T) {
	for _, deck := range []int{0, 1} {
		t.Run(fmt.Sprint(deck), func(t *testing.T) {
			tb := newTable(t, 3, func(s *monopoly.State) {
				id := 1
				if deck == 1 {
					id = 14
				}
				cardFixture(s, deck, id)
				group(s, 0, []int{1, 3}, []int{5, 5})
				group(s, 0, []int{6, 8, 9}, []int{1, 1, 1})
				group(s, 1, []int{11, 13, 14}, []int{3, 3, 3})
			})
			tb.people[0].act(map[string]any{"type": "roll"}, 200)
			want := 1225
			if deck == 1 {
				want = 1150
			}
			if money(tb.people[0].room(), 0) != want {
				t.Fatal("repairs counted hotels or other player's buildings incorrectly")
			}
			tb.record("repair charge settled")
		})
	}
}

func TestMonopolyCardPayEachBankruptcyHTTP(t *testing.T) {
	tb := newTable(t, 4, func(s *monopoly.State) {
		cardFixture(s, 0, 3)
		s.Players[0].Cash = 60
		s.Properties[5].Owner = 0
		s.Properties[5].Mortgaged = true
	})
	a, c := tb.people[0], tb.people[2]
	a.act(map[string]any{"type": "roll"}, 200)
	r := a.room()
	if data(r)["phase"] != "debt" || money(r, 0) != 10 || money(r, 1) != 1550 {
		t.Fatal("payments not in clockwise order", r)
	}
	tb.restart()
	a.act(map[string]any{"type": "bankrupt"}, 200)
	if property(c.room(), 5)["owner"] != float64(2) {
		t.Fatal("estate assigned to wrong creditor")
	}
	c.act(map[string]any{"type": "accept_mortgage", "choice": "keep"}, 200)
	r = c.room()
	if money(r, 1) != 1550 || money(r, 2) != 1500 || money(r, 3) != 1500 || data(r)["phase"] != "roll" || data(r)["turn"] != float64(1) {
		t.Fatal("bankruptcy did not resume or cancel remaining debtor obligations", r)
	}
	tb.record("pay-each card bankruptcy resumed")
}

func TestMonopolyCardCollectAfterPayerBankruptcyHTTP(t *testing.T) {
	tb := newTable(t, 4, func(s *monopoly.State) { cardFixture(s, 1, 12); s.Players[1].Cash = 0 })
	a, b := tb.people[0], tb.people[1]
	a.act(map[string]any{"type": "roll"}, 200)
	tb.restart()
	b.act(map[string]any{"type": "bankrupt"}, 200)
	r := a.room()
	if money(r, 0) != 1520 || money(r, 2) != 1490 || money(r, 3) != 1490 || data(r)["phase"] != "end" {
		t.Fatal("remaining collection was lost or replayed", r)
	}
	tb.record("collect-each continued after payer bankruptcy")
}

func TestMonopolyCardCollectorBankruptcyHTTP(t *testing.T) {
	tb := newTable(t, 4, func(s *monopoly.State) {
		cardFixture(s, 1, 12)
		s.Players[0].Cash = 0
		s.Players[1].Cash = 0
		s.Properties[5].Owner = 1
		s.Properties[5].Mortgaged = true
	})
	a, b, c, d := tb.people[0], tb.people[1], tb.people[2], tb.people[3]
	a.act(map[string]any{"type": "roll"}, 200)
	b.act(map[string]any{"type": "bankrupt"}, 200)
	a.act(map[string]any{"type": "accept_mortgage", "choice": "keep"}, 200)
	if data(a.room())["phase"] != "debt" {
		t.Fatal("inherited mortgage fee not due")
	}
	tb.restart()
	a.act(map[string]any{"type": "bankrupt"}, 200)
	c.act(map[string]any{"type": "auction_pass"}, 200)
	d.act(map[string]any{"type": "auction_pass"}, 200)
	r := c.room()
	if money(r, 2) != 1500 || money(r, 3) != 1500 || data(r)["turn"] != float64(2) || data(r)["phase"] != "roll" {
		t.Fatal("eliminated collector continued collecting", r)
	}
	tb.record("collector bankruptcy cancels remaining collection")
}

// Site ordering: transfer the final estate, then declare the last survivor.
// Do not ask a sole winner to process fees or auctions after victory is decided.
func TestMonopolyLastSurvivorEstateHTTP(t *testing.T) {
	tb := newTable(t, 2, func(s *monopoly.State) {
		s.Phase = "debt"
		s.Players[0].Cash = 0
		s.Players[1].Cash = 0
		s.Properties[5].Owner = 0
		s.Properties[5].Mortgaged = true
		s.Debt = &monopoly.Debt{Debtor: 0, Creditor: 1, Amount: 100, Reason: "租金"}
	})
	tb.people[0].act(map[string]any{"type": "bankrupt"}, 200)
	r := tb.people[1].room()
	if r["status"] != "finished" || property(r, 5)["owner"] != float64(1) {
		t.Fatal("sole survivor trapped in inherited fees", r)
	}
	tb.restart()
	tb.record("final estate transferred before victory")
}
