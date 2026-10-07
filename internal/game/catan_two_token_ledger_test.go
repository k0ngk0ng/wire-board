package game

import (
	"fmt"
	"slices"
	"testing"
)

func twoLedgerKnight(s *State, p int) {
	catanCard(s.Catan, p, 0)
	s.Catan.Players[p].Dev[0]--
	s.Catan.DevDiscard = append(s.Catan.DevDiscard, 0)
	s.Catan.Players[p].Knights++
}

func TestCatanTwoTokenLedgerKnightRecyclingAndViews(t *testing.T) {
	for bank := 0; bank <= 3; bank++ {
		t.Run(fmt.Sprint(bank), func(t *testing.T) {
			s := twoCoreFixture(t)
			p, g := s.Turn, s.Catan
			q := g.Two
			q.Bank, q.Tokens[p], q.Tokens[1-p] = bank, 0, 20-bank
			twoLedgerKnight(s, p)
			if a, ok := s.catanTwoOptionalBot(p); !ok || a.Type != "catan_two_knight" {
				t.Fatal("bot failed to exchange with depleted supply")
			}
			for _, viewer := range []int{-1, p, 1 - p} {
				v := s.View(viewer)["catan"].(map[string]any)["two"].(map[string]any)
				if v["tokenRule"] != "ledger" || v["canExchangeKnight"] != (viewer == p) {
					t.Fatal("rule or action permissions missing", v)
				}
			}
			cards := len(g.DevDiscard)
			helperApply(t, s, p, Action{Type: "catan_two_knight"})
			q = s.Catan.Two
			issued := max(0, 2-bank)
			if q.TokensIssued != issued || q.Bank != max(0, bank-2) || q.Tokens[p] != 2 || !q.KnightExchanged || s.Catan.Players[p].Knights != 0 || len(s.Catan.DevDiscard) != cards {
				t.Fatal("knight exchange changed reward or physical cards")
			}
			helperReject(t, s, p, Action{Type: "catan_two_knight"})
			twoCoreRestore(t, s)
			twoTokenHand(s, p, []int{0, 0, 0, 0, 0})
			twoTokenHand(s, 1-p, []int{2, 0, 0, 0, 0})
			cost := s.Catan.twoTokenCost(p)
			helperApply(t, s, p, Action{Type: "catan_two_trade"})
			if !s.Catan.Two.Spent || s.Catan.Two.Tokens[p] != 2-cost {
				t.Fatal("ledger altered score-dependent cost")
			}
			s.AutoCatanPending()
			if s.Phase != "catan_roll" || s.Catan.Two.Trade != nil {
				t.Fatal("forced trade did not resume")
			}
			bankBefore := s.Catan.Two.Bank
			if err := s.catanTwoEarn(p, 1); err != nil {
				t.Fatal(err)
			}
			q = s.Catan.Two
			if q.TokensIssued != issued || q.Bank != bankBefore-1 {
				t.Fatal("returned token not reused")
			}
			twoCoreRestore(t, s)
		})
	}
}

func TestCatanTwoTokenLedgerRewardsAndCorruption(t *testing.T) {
	for count := 0; count <= 3; count++ {
		for bank := 0; bank <= 3; bank++ {
			s := twoCoreFixture(t)
			p, q := s.Turn, s.Catan.Two
			q.Bank, q.Tokens[p], q.Tokens[1-p] = bank, 0, 20-bank
			if err := s.catanTwoEarn(p, count); err != nil {
				t.Fatal(err)
			}
			if q.TokensIssued != max(0, count-bank) || q.Tokens[p] != count || q.Bank != max(0, bank-count) {
				t.Fatal("partial reward", count, bank)
			}
			twoCoreRestore(t, s)
		}
	}
	s := twoCoreFixture(t)
	p, q := s.Turn, s.Catan.Two
	q.Bank, q.Tokens[p], q.Tokens[1-p], q.TokensIssued = 0, 0, 20+catanTwoTokenLedgerLimit, catanTwoTokenLedgerLimit
	twoLedgerKnight(s, p)
	helperReject(t, s, p, Action{Type: "catan_two_knight"})
	if s.catanTwoCanExchangeKnight(p) {
		t.Fatal("unsafe issuance advertised")
	}
	twoCoreRestore(t, s)
	for _, bad := range []func(*CatanTwo){
		func(q *CatanTwo) { q.TokensIssued = -1 },
		func(q *CatanTwo) { q.TokensIssued++ },
		func(q *CatanTwo) { q.Tokens[0] = int(^uint(0) >> 1) },
		func(q *CatanTwo) { q.Bank = -1 },
	} {
		trial := clone(*s)
		bad(trial.Catan.Two)
		if trial.validateCatanTwoTokens() == nil {
			t.Fatal("corrupt token ledger accepted")
		}
	}
	// Legacy snapshots omit tokensIssued and retain ordinary physical accounting.
	legacy := twoCoreFixture(t)
	twoCoreRestore(t, legacy)
}

func TestCatanTwoTokenLedgerPaidSettlementAndNeutral(t *testing.T) {
	for _, scenario := range []string{"base", "rivers", "caravans"} {
		t.Run(scenario, func(t *testing.T) {
			var s *State
			switch scenario {
			case "base":
				s = twoCoreFixture(t)
				twoCoreActionPhase(t, s)
			case "rivers":
				s = twoRiverFixture(t)
			case "caravans":
				s = twoCaravanFixture(t)
			}
			g, p := s.Catan, s.Turn
			vertex := -1
			for _, v := range g.Vertices {
				if g.canSettlement(p, v.ID, true) && g.twoSettlementTokens(p, v.ID) > 0 {
					for _, id := range g.touching(v.ID) {
						if g.Edges[id].Owner == -1 && (g.Rivers == nil || !slices.Contains(g.Rivers.Map.Bridges, id)) {
							g.Edges[id].Owner = p
							vertex = v.ID
							break
						}
					}
				}
				if vertex >= 0 {
					break
				}
			}
			if vertex < 0 {
				t.Fatal("no eligible paid settlement")
			}
			q := g.Two
			q.Bank, q.Tokens[p], q.Tokens[1-p] = 0, 0, 20
			if g.Rivers != nil {
				g.Rivers.Gold[1-p] += g.Rivers.Bank
				g.Rivers.Bank = 0
			}
			reward := g.twoSettlementTokens(p, vertex)
			twoTokenHand(s, p, []int{1, 1, 1, 1, 0})
			helperApply(t, s, p, Action{Type: "catan_settlement", Vertex: vertex})
			if s.Catan.Two.TokensIssued != reward || s.Catan.Two.Tokens[p] != reward {
				t.Fatal("settlement reward incorrect")
			}
			tokens := slices.Clone(s.Catan.Two.Tokens)
			if s.Catan.Two.Pending == nil {
				t.Fatal("expected neutral build")
			}
			twoCoreRestore(t, s)
			a, err := s.BotAction(p)
			if err != nil {
				t.Fatal(err)
			}
			helperApply(t, s, p, a)
			if s.Phase != "catan_turn" || s.Catan.Two.Pending != nil || !slices.Equal(tokens, s.Catan.Two.Tokens) || s.Catan.Two.TokensIssued != reward {
				t.Fatal("neutral response repeated reward")
			}
			if s.Catan.Rivers != nil {
				riverConserved(t, s)
			}
			twoCoreRestore(t, s)
		})
	}
}
