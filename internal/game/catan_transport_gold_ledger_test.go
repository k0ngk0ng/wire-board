package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func TestCatanTransportGoldLedgerArrival(t *testing.T) {
	for _, n := range []int{3, 4, 6} {
		for level := 0; level < 5; level++ {
			for bank := 0; bank <= 5; bank++ {
				t.Run(fmt.Sprintf("%d/%d/%d", n, level, bank), func(t *testing.T) {
					g, tr := transportCargoFixture(t, n)
					if err := tr.beginTurn(g, 0, 1); err != nil {
						t.Fatal(err)
					}
					for upgrade := 0; upgrade < level; upgrade++ {
						transportGive(g, 0, catanTransportUpgradeCost(upgrade))
						if err := tr.upgrade(g, 0); err != nil {
							t.Fatal(err)
						}
					}
					if err := tr.beginTravel(g, 0); err != nil {
						t.Fatal(err)
					}
					if err := tr.stop(g, 0, tr.Sequence); err != nil {
						t.Fatal(err)
					}
					transportVisitFixture(t, g, tr, 0, 0)
					if _, err := tr.resolveArrival(g, 0, tr.Sequence, false); err != nil {
						t.Fatal(err)
					}
					token, _ := tr.token(tr.Wagons[0].Cargo)
					site := -1
					for i := range tr.Map.Sites {
						if tr.Map.accepts(i, token.Cargo) {
							site = i
							break
						}
					}
					transportVisitFixture(t, g, tr, 0, site)
					tr.Gold[1] += tr.GoldBank - bank
					tr.GoldBank = bank
					g, tr = transportRoundTrip(t, g, tr)
					oldGold, oldPoints := tr.Gold[0], tr.extraPoints(0)
					res, err := tr.resolveArrival(g, 0, tr.Sequence, true)
					missing := max(0, level+1-bank)
					if err != nil || res.Delivered != token.ID || res.Gold != level+1 || tr.Gold[0] != oldGold+level+1 || tr.GoldIssued != missing || tr.GoldBank != max(0, bank-level-1) || tr.extraPoints(0) != oldPoints+1 {
						t.Fatal("delivery blocked or mispaid", res, err)
					}
					if tr.GoldBank+sum(tr.Gold) != tr.Map.Gold+missing {
						t.Fatal("ledger conservation")
					}
					g, tr = transportRoundTrip(t, g, tr)
					before := transportSnapshot(g, tr)
					if _, err = tr.resolveArrival(g, 0, tr.Sequence, true); err == nil || transportSnapshot(g, tr) != before {
						t.Fatal("delivery replay changed ledger")
					}
				})
			}
		}
	}
}

func TestCatanTransportGoldLedgerSaleReuseTradeAndMovement(t *testing.T) {
	s := transportState(t, true)
	s.Phase = "catan_turn"
	p := s.Turn
	g, tr := s.Catan, s.Catan.Transport
	other := (p + 1) % 3
	tr.Gold[other] += tr.GoldBank
	tr.GoldBank = 0
	transportGive(g, p, []int{8, 0, 0, 0, 0})
	if !slices.Contains(s.catanTransportChoices(p)["sell"].([]int), 0) {
		t.Fatal("empty bank hid earned gold")
	}
	if err := s.Apply(p, Action{Type: "catan_coin_sell", Color: 0}); err != nil {
		t.Fatal(err)
	}
	tr = s.Catan.Transport
	if tr.GoldIssued != 1 || tr.GoldBank != 0 || tr.Gold[p] != 6 {
		t.Fatal("sale did not issue only deficit")
	}
	if err := s.Apply(p, Action{Type: "catan_coin_buy", Color: 4}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(p, Action{Type: "catan_coin_sell", Color: 0}); err != nil {
		t.Fatal(err)
	}
	tr = s.Catan.Transport
	if tr.GoldIssued != 1 || tr.GoldBank != 1 {
		t.Fatal("returned coins not reused")
	}
	s = transportRestoreState(t, s)
	g, tr = s.Catan, s.Catan.Transport
	// Valid explicit high-balance fixture; normal game does not grant this windfall.
	tr.GoldIssued += 200
	tr.Gold[p] += 200
	if err := s.validateCatanTransport(); err != nil {
		t.Fatal(err)
	}
	catanMove(g.Players[other].Resources, g.Bank, slices.Clone(g.Players[other].Resources))
	transportGive(g, other, []int{1, 0, 0, 0, 0})
	beforeOwn, beforeOther := tr.Gold[p], tr.Gold[other]
	if err := s.Apply(p, Action{Type: "catan_trade_offer", Give: []int{0, 0, 0, 0, 0}, Take: []int{1, 0, 0, 0, 0}, GoldGive: 180}); err != nil {
		t.Fatal(err)
	}
	id := s.Catan.Trade.ID
	if err := s.Apply(other, Action{Type: "catan_trade_accept", Offer: id}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(p, Action{Type: "catan_trade_complete", Offer: id, Target: other}); err != nil {
		t.Fatal(err)
	}
	tr = s.Catan.Transport
	if tr.Gold[p] != beforeOwn-180 || tr.Gold[other] != beforeOther+180 {
		t.Fatal("ledger trade capped at physical supply")
	}
	if err := s.Apply(p, Action{Type: "catan_end"}); err != nil {
		t.Fatal(err)
	}
	if len(s.catanTransportChoices(p)["steps"].([]catanTransportStep)) == 0 {
		t.Fatal("large ledger blocked movement")
	}
	s = transportRestoreState(t, s)
	g = s.Catan
	if got := catanTransportPath(g, g.Transport.Map, g.Transport.Wagons[p].Position, -1, p, catanGoldLedgerLimit); got != nil {
		t.Fatal("unreachable path")
	}
}

func TestCatanTransportGoldLedgerCorruptionAndLegacy(t *testing.T) {
	s := transportState(t, true)
	raw, _ := json.Marshal(s)
	if string(raw) == "" {
		t.Fatal("missing save")
	}
	for _, mutate := range []func(*catanTransport){
		func(tr *catanTransport) { tr.GoldIssued = -1 },
		func(tr *catanTransport) { tr.GoldIssued = 1 },
		func(tr *catanTransport) { tr.GoldIssued = catanGoldLedgerLimit + 1; tr.Gold[0] += tr.GoldIssued },
		func(tr *catanTransport) { tr.GoldBank++ },
	} {
		var trial State
		if err := json.Unmarshal(raw, &trial); err != nil {
			t.Fatal(err)
		}
		mutate(trial.Catan.Transport)
		if trial.validateCatanTransport() == nil {
			t.Fatal("corrupt ledger accepted")
		}
	}
	var legacy State
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Catan.Transport.GoldIssued != 0 {
		t.Fatal("legacy save changed")
	}
	view := legacy.Catan.Transport.publicView()
	if view.GoldRule != "ledger" || view.GoldIssued != 0 {
		t.Fatal("missing public rule")
	}
}
