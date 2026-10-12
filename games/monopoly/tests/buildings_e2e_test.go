package monopoly_test

// Failure scenarios written before implementation: incomplete/mortgaged groups,
// uneven building/selling, overspending, building outside allowed windows,
// overdrawn physical inventory, hotels failing to return houses, a last-building
// race awarded by packet arrival, auction targets owned by another player,
// restart losing the suspended phase/clock, hotel demolition without enough
// replacement houses, and liquidation prematurely ending a debt workflow.

import (
	"fmt"
	"reflect"
	"testing"

	monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
)

func group(s *monopoly.State, owner int, ids, levels []int) {
	for j, id := range ids {
		s.Properties[id].Owner = owner
		s.Properties[id].Buildings = levels[j]
		if levels[j] == 5 {
			s.Hotels--
		} else {
			s.Houses -= levels[j]
		}
	}
}
func level(r map[string]any, id int) int { return int(property(r, id)["buildings"].(float64)) }

func TestMonopolyEvenBuildingsHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) { group(s, 0, []int{6, 8, 9}, []int{0, 0, 0}) })
	a := tb.people[0]
	a.act(map[string]any{"type": "build", "property": 6}, 200)
	before := a.room()
	a.act(map[string]any{"type": "build", "property": 6}, 400)
	if !reflect.DeepEqual(before, a.room()) {
		t.Fatal("rejected uneven build mutated state")
	}
	a.act(map[string]any{"type": "build", "property": 8}, 200)
	a.act(map[string]any{"type": "sell_building", "property": 9}, 400)
	a.act(map[string]any{"type": "build", "property": 9}, 200)
	a.act(map[string]any{"type": "build", "property": 6}, 200)
	a.act(map[string]any{"type": "sell_building", "property": 8}, 400)
	a.act(map[string]any{"type": "mortgage", "property": 9}, 400)
	a.act(map[string]any{"type": "sell_building", "property": 6}, 200)
	r := a.room()
	if money(r, 0) != 1325 || level(r, 6) != 1 || data(r)["houses"] != float64(29) {
		t.Fatal("building economics", r)
	}
	tb.restart()
	a.act(map[string]any{"type": "sell_group", "property": 8}, 200)
	r = a.room()
	if money(r, 0) != 1400 || data(r)["houses"] != float64(32) {
		t.Fatal("group liquidation inventory/refund", r)
	}
	a.act(map[string]any{"type": "mortgage", "property": 9}, 200)
	a.act(map[string]any{"type": "build", "property": 6}, 400)
	tb.record("even build/sale and mortgage restrictions")
}

func TestMonopolyHotelInventoryHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) { group(s, 0, []int{6, 8, 9}, []int{4, 4, 4}) })
	a := tb.people[0]
	a.act(map[string]any{"type": "build", "property": 6}, 200)
	r := a.room()
	if level(r, 6) != 5 || data(r)["houses"] != float64(24) || data(r)["hotels"] != float64(11) || money(r, 0) != 1450 {
		t.Fatal("hotel upgrade inventory", r)
	}
	a.act(map[string]any{"type": "build", "property": 6}, 400)
	a.act(map[string]any{"type": "sell_building", "property": 8}, 400)
	a.act(map[string]any{"type": "sell_building", "property": 6}, 200)
	r = a.room()
	if level(r, 6) != 4 || data(r)["houses"] != float64(20) || data(r)["hotels"] != float64(12) || money(r, 0) != 1475 {
		t.Fatal("hotel downgrade inventory", r)
	}
	tb.record("hotel round trip")
}

func TestMonopolyOffTurnBuildHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Phase = "roll"
		group(s, 1, []int{1, 3}, []int{0, 0})
		s.Players[1].Jailed = true
		s.Players[1].Position = 10
	})
	b := tb.people[1]
	before := b.room()["turnDeadline"]
	b.act(map[string]any{"type": "build", "property": 1}, 200)
	r := b.room()
	if r["turnDeadline"] != before || data(r)["turn"] != float64(0) || level(r, 1) != 1 {
		t.Fatal("off-turn build stole or extended turn", r)
	}
	tb.people[2].act(map[string]any{"type": "build", "property": 3}, 400)
	tb.record("jailed off-turn owner can build")
}

func TestMonopolyBuildingShortageAuctionHTTP(t *testing.T) {
	for _, hotel := range []bool{false, true} {
		t.Run(fmt.Sprint(hotel), func(t *testing.T) {
			tb := newTable(t, 3, func(s *monopoly.State) {
				if !hotel {
					group(s, 0, []int{1, 3}, []int{0, 0})
					group(s, 1, []int{6, 8, 9}, []int{0, 0, 0})
					group(s, 2, []int{11, 13, 14}, []int{4, 4, 4})
					group(s, 2, []int{16, 18, 19}, []int{4, 4, 4})
					group(s, 2, []int{21, 23, 24}, []int{3, 2, 2})
				} else {
					group(s, 0, []int{1, 3}, []int{4, 4})
					group(s, 1, []int{6, 8, 9}, []int{4, 4, 4})
					group(s, 2, []int{11, 13, 14}, []int{5, 5, 5})
					group(s, 2, []int{16, 18, 19}, []int{5, 5, 5})
					group(s, 2, []int{21, 23, 24}, []int{5, 5, 5})
					group(s, 2, []int{31, 32, 34}, []int{5, 5, 4})
				}
			})
			a, b := tb.people[0], tb.people[1]
			a.act(map[string]any{"type": "build", "property": 1}, 200)
			r := a.room()
			if data(r)["phase"] != "auction" || len(r["game"].(map[string]any)["attention"].([]any)) != 0 {
				t.Fatal("scarcity did not open optional auction", r)
			}
			b.act(map[string]any{"type": "bid", "property": 1, "amount": 20}, 400)
			b.act(map[string]any{"type": "bid", "property": 6, "amount": 20}, 200)
			a.act(map[string]any{"type": "bid", "property": 1, "amount": 30}, 200)
			tb.restart()
			tb.waitPhase("end")
			r = a.room()
			want := 1
			if hotel {
				want = 5
			}
			if level(r, 1) != want || money(r, 0) != 1470 || money(r, 1) != 1500 || data(r)["turn"] != float64(0) {
				t.Fatal("building auction wrong winner/cost/resume", r)
			}
			stock := "houses"
			if hotel {
				stock = "hotels"
			}
			if data(r)[stock] != float64(0) {
				t.Fatal("scarce stock not consumed", r)
			}
			b.act(map[string]any{"type": "build", "property": 6}, 400)
			tb.record("last building awarded via bidding")
		})
	}
}

func TestMonopolyHotelLiquidationAndDebtHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		group(s, 0, []int{1, 3}, []int{5, 5})
		group(s, 1, []int{6, 8, 9}, []int{4, 4, 4})
		group(s, 1, []int{11, 13, 14}, []int{4, 4, 4})
		group(s, 2, []int{16, 18, 19}, []int{3, 3, 2})
		s.Players[0].Cash = 10
		s.Phase = "debt"
		s.Debt = &monopoly.Debt{Debtor: 0, Creditor: 2, Amount: 200, Reason: "租金"}
	})
	a := tb.people[0]
	a.act(map[string]any{"type": "sell_building", "property": 1}, 400)
	a.act(map[string]any{"type": "bankrupt"}, 400)
	a.act(map[string]any{"type": "build", "property": 1}, 400)
	tb.restart()
	a.act(map[string]any{"type": "sell_group", "property": 1}, 200)
	r := a.room()
	if money(r, 0) != 60 || money(r, 2) != 1700 || data(r)["hotels"] != float64(12) || data(r)["houses"] != float64(0) || level(r, 1) != 0 || level(r, 3) != 0 || data(r)["phase"] != "end" {
		t.Fatal("whole-group liquidation lost debt/inventory", r)
	}
	tb.record("liquidated hotels with no replacement houses")
}

func TestMonopolyBuildingBidFundingAndReservationHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		group(s, 0, []int{1, 3}, []int{0, 0})
		group(s, 1, []int{6, 8, 9}, []int{0, 0, 0})
		group(s, 2, []int{11, 13, 14}, []int{4, 4, 4})
		group(s, 2, []int{16, 18, 19}, []int{4, 4, 4})
		group(s, 2, []int{21, 23, 24}, []int{3, 2, 2})
		s.Players[1].Cash = 0
		s.Properties[5].Owner = 1
	})
	a, b := tb.people[0], tb.people[1]
	a.act(map[string]any{"type": "build", "property": 1}, 200)
	b.act(map[string]any{"type": "mortgage", "property": 5}, 200)
	b.act(map[string]any{"type": "bid", "property": 6, "amount": 70}, 200)
	before := b.room()
	b.act(map[string]any{"type": "mortgage", "property": 8}, 400)
	if !reflect.DeepEqual(before, b.room()) {
		t.Fatal("leading bid invalidated by group mortgage")
	}
	propose(b, 0, bundle(40, 5), bundle(0), 400)
	tb.waitPhase("end")
	r := b.room()
	if money(r, 1) != 30 || level(r, 6) != 1 {
		t.Fatal("fundraising or bid reserve lost", r)
	}
	tb.record("legal fundraising preserved building bid")
}

func TestMonopolyBuildingAuctionEligibilityHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		group(s, 0, []int{1, 3}, []int{0, 0})
		group(s, 1, []int{6, 8, 9}, []int{0, 0, 0})
		group(s, 2, []int{11, 13, 14}, []int{4, 4, 4})
		group(s, 2, []int{16, 18, 19}, []int{4, 4, 4})
		group(s, 2, []int{21, 23, 24}, []int{3, 2, 2})
		s.Players[1].Cash = 0
	})
	tb.people[0].act(map[string]any{"type": "build", "property": 1}, 200)
	for _, p := range data(tb.people[0].room())["auctionActors"].([]any) {
		if p == float64(1) {
			t.Fatal("mortgaging the only buildable color set cannot fund a legal building bid")
		}
	}
	tb.record("unfundable construction bidder omitted")
}
