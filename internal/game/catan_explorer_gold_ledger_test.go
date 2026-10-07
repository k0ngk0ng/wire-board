package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerGoldLedgerProductionAndReusedPayments(t *testing.T) {
	for bank := 0; bank <= 8; bank++ {
		t.Run(fmt.Sprint(bank), func(t *testing.T) {
			g, f, c, e := explorerEconomyFixture(t, "pirate-lairs")
			e.Gold[3] += e.GoldBank - bank
			e.GoldBank = bank
			before := slices.Clone(e.Gold)
			result, err := e.resolveProduction(g, f, c, 0, 1, [2]int{2, 4})
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(result.Gold, []int{0, 0, 3, 1}) || e.GoldIssued != max(0, 4-bank) || e.GoldBank != max(0, bank-4) {
				t.Fatal("wrong ledger shortfall or full payouts", result, e)
			}
			for p := range before {
				if e.Gold[p] != before[p]+result.Gold[p] {
					t.Fatal("recipient did not get full reward")
				}
			}
			explorerEconomyRestore(t, g, f, c, e)
			issued := e.GoldIssued
			// Paying coins back replenishes the ledger bank. It must be reused
			// for later rewards instead of issuing credits on every transaction.
			if err := e.bankTrade(g, f, c, 0, 1, -1, 0); err != nil {
				t.Fatal(err)
			}
			g.Players[0].Resources[0]++
			g.Bank[0]-- // exactly three wood for sale
			if err := e.bankTrade(g, f, c, 0, 1, 0, -1); err != nil {
				t.Fatal(err)
			}
			if e.GoldIssued != issued || e.Turn.Bought != 1 {
				t.Fatal("payment did not recycle gold, or sale altered purchase cap")
			}
			explorerEconomyRestore(t, g, f, c, e)
		})
	}
}

func TestCatanExplorerCityGoldFieldsPerBuilding(t *testing.T) {
	// Isolated hexes are explicit production-rule fixtures, not a naturally
	// conquered lair. The full mission controller separately gates liberation.
	for _, kind := range []string{"settlement", "harbor", "city"} {
		for _, number := range []int{8, 6} {
			t.Run(fmt.Sprintf("%s/%d", kind, number), func(t *testing.T) {
				g, f, c, e := explorerEconomyFixture(t, "pirate-lairs")
				(&State{Catan: g}).enableCitiesKnights()
				v := g.Tiles[2].Vertices[0]
				g.Vertices[v].Level = 2
				g.Vertices[v].Harbor = kind == "harbor"
				if kind == "settlement" {
					g.Vertices[v].Level = 1
				}
				g.Tiles[2].Number = number
				e.Turn.Phase, e.Turn.Dice = "city", [2]int{2, 4}
				result, err := e.resolveProduction(g, f, c, 0, 1, [2]int{2, 4})
				if err != nil {
					t.Fatal(err)
				}
				want := 1 // no cards => one additional gold even when gold is produced
				if number == 6 {
					want += 2
				}
				if result.Gold[2] != want || sum(result.Resources[2]) != 0 {
					t.Fatal("gold field city/harbor must yield two gold, no commodity", result)
				}
				explorerEconomyRestore(t, g, f, c, e)
			})
		}
	}
}

func TestCatanExplorerGoldLedgerSaveGuards(t *testing.T) {
	for _, damage := range []func(*catanExplorerEconomy){
		func(e *catanExplorerEconomy) { e.GoldIssued = -1 },
		func(e *catanExplorerEconomy) { e.GoldIssued = catanExplorerGoldLedgerLimit + 1 },
		func(e *catanExplorerEconomy) { e.GoldIssued++ },
	} {
		g, f, c, e := explorerEconomyFixture(t, "land-ho")
		damage(e)
		if e.validate(g, f, c) == nil {
			t.Fatal("invalid ledger accepted")
		}
	}
	g, f, c, e := explorerEconomyFixture(t, "land-ho")
	e.GoldIssued = catanExplorerGoldLedgerLimit
	e.Gold[3] += e.GoldBank + e.GoldIssued
	e.GoldBank = 0
	before := clone(*e)
	if e.ensureGold(1) == nil || !reflect.DeepEqual(*e, before) {
		t.Fatal("overflow mutated ledger")
	}
	explorerEconomyReject(t, g, f, c, e, func() error { _, err := e.resolveProduction(g, f, c, 0, 1, [2]int{2, 4}); return err })
}

func TestCatanExplorerGoldLedgerSharedLairRewards(t *testing.T) {
	for bank := 0; bank < 4; bank++ {
		q := newExplorerLairsFixture(t, 2)
		q.start(t, 0, 1)
		q.must(t, "land", 0, []int{2, 3})
		q.end(t)
		q.start(t, 1, 2)
		q.must(t, "land", 3, []int{13})
		q.end(t)
		q.E.Gold[0] += q.E.GoldBank - bank
		q.E.GoldBank = bank
		before := slices.Clone(q.E.Gold)
		q.must(t, "begin", 3, nil)
		if q.E.Gold[0] != before[0]+2 || q.E.Gold[1] != before[1]+2 || q.E.GoldIssued != 4-bank || q.E.GoldBank != 0 {
			t.Fatal("coin shortage favored an earlier lair participant")
		}
		q.restore(t)
		q.reject(t, "begin", 3, nil)
	}
}

func TestCatanExplorerGoldLedgerCommodityFarmChoices(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := explorerCityGoldFixture(t, n)
		e := s.Catan.Explorer.Economy
		e.Gold[1] += e.GoldBank
		e.GoldBank = 0
		a := Action{Type: "catan_explorer_spice_gold", Card: 6, Prompt: int(s.Catan.TurnSerial)}
		if !slices.ContainsFunc(s.catanExplorerSpiceChoices(0), func(v Action) bool { return v.Type == a.Type && v.Card == a.Card }) {
			t.Fatal("empty coins hid commodity farm action")
		}
		before := s.Catan.Players[0].Resources[6]
		for i := 1; i <= 2; i++ {
			if err := s.catanExplorerCityAction(0, a); err != nil {
				t.Fatal(err)
			}
			explorerCityRestore(t, s)
			if s.Catan.Explorer.Economy.GoldIssued != i || s.Catan.Players[0].Resources[6] != before-i {
				t.Fatal("commodity sale did not issue and pay exactly once")
			}
		}
		explorerCityActionReject(t, s, 0, a)
	}
}

func TestCatanExplorerGoldLedgerLargePlayerTrade(t *testing.T) {
	s := explorerCityTradeFixture(t, 3)
	e := s.Catan.Explorer.Economy
	// A conserved late-game balance beyond the old shared river trade cap.
	e.GoldIssued = 200
	e.Gold[0] += 200
	before := slices.Clone(e.Gold)
	a := explorerCityTradeOffer(s)
	a.GoldGive = 180
	explorerCityTradeApply(t, s, 0, a)
	id := s.Catan.Trade.ID
	explorerCityTradeApply(t, s, 1, Action{Type: "catan_trade_accept", Prompt: a.Prompt, Offer: id})
	explorerCityTradeApply(t, s, 0, Action{Type: "catan_trade_complete", Prompt: a.Prompt, Offer: id, Target: 1})
	if s.Catan.Explorer.Economy.Gold[0] != before[0]-180 || s.Catan.Explorer.Economy.Gold[1] != before[1]+180 || s.Catan.Explorer.Economy.GoldIssued != 200 {
		t.Fatal("large ledger trade lost gold or issued more")
	}
}
