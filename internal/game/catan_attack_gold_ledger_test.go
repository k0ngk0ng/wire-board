package game

import (
	"fmt"
	"slices"
	"testing"
)

func TestCatanAttackGoldLedgerFullBattleCompensation(t *testing.T) {
	for _, n := range []int{3, 6} {
		for bank := 0; bank <= 9; bank++ {
			t.Run(fmt.Sprintf("%d/%d", n, bank), func(t *testing.T) {
				s := newAttackState(t, n, false)
				a, actor := s.Catan.Attack, s.Turn
				tile := a.Map.Coast[3]
				clear(a.Barbarians)
				a.Barbarians[tile] = 1
				// Three owners contest a single captive. Losing owners and killed
				// knights must all receive the full recorded compensation.
				attackBattleKnights(s, tile, actor, (actor+1)%n, (actor+2)%n)
				attackCoins(s, (actor+1)%n, a.GoldBank-bank)
				before := slices.Clone(a.Gold)
				if err := s.catanAttackResolveEnd(nil, attackDice(t, 6, 4, 2, 1)); err != nil {
					t.Fatal(err)
				}
				a = s.Catan.Attack
				if len(a.End.Battles) != 1 {
					t.Fatal("battle missing")
				}
				b := a.End.Battles[0]
				if sum(b.Gold) < 6 || sum(b.Prisoners) != 1 || a.Barbarians[tile] != 0 {
					t.Fatal("battle fixture did not exercise shared compensation")
				}
				for p, reward := range b.Gold {
					if a.Gold[p] != before[p]+reward {
						t.Fatal("partial compensation favored an earlier player")
					}
				}
				if a.GoldIssued != max(0, sum(b.Gold)-bank) || a.GoldBank != max(0, bank-sum(b.Gold)) {
					t.Fatal("wrong ledger issue amount")
				}
				assertAttackRestored(t, s)
			})
		}
	}
}

func TestCatanAttackGoldLedgerCardsAndRecycledPayments(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, card := range []string{"capture", "knighthood", "swift_knight", "treason"} {
			t.Run(fmt.Sprintf("%d/%s", n, card), func(t *testing.T) {
				s := newAttackState(t, n, false)
				p, a := s.Turn, s.Catan.Attack
				attackCoins(s, (p+1)%n, a.GoldBank)
				buyAttackCard(t, s, card)
				choice, err := s.BotAction(p)
				if err != nil {
					t.Fatal(err)
				}
				if err := s.Apply(p, choice); err != nil {
					t.Fatal(err)
				}
				a = s.Catan.Attack
				want := 0
				if card == "treason" {
					want = 2
				}
				if a.GoldIssued != want || a.Gold[p] != want || a.GoldBank != 0 {
					t.Fatal("empty physical supply blocked/changed card reward")
				}
				assertAttackRestored(t, s)
				if card != "treason" {
					return
				}
				// Buy returns two coins, then a resource sale reuses one of them.
				if err := s.Apply(p, Action{Type: "catan_coin_buy", Color: 0}); err != nil {
					t.Fatal(err)
				}
				attackHand(s, p, []int{4, 0, 0, 0, 0})
				if err := s.Apply(p, Action{Type: "catan_coin_sell", Color: 0}); err != nil {
					t.Fatal(err)
				}
				a = s.Catan.Attack
				if a.GoldIssued != 2 || a.GoldBank != 1 || a.Gold[p] != 1 || a.Bought != 1 {
					t.Fatal("returned coins were not reused or purchase cap changed")
				}
				assertAttackRestored(t, s)
			})
		}
	}
}

func TestCatanAttackGoldLedgerBankSaleAndSaveGuards(t *testing.T) {
	s := newAttackState(t, 3, false)
	p, a := s.Turn, s.Catan.Attack
	attackCoins(s, (p+1)%3, a.GoldBank)
	attackHand(s, p, []int{4, 0, 0, 0, 0})
	if err := s.Apply(p, Action{Type: "catan_coin_sell", Color: 0}); err != nil {
		t.Fatal(err)
	}
	a = s.Catan.Attack
	if a.GoldIssued != 1 || a.Gold[p] != 1 || a.GoldBank != 0 {
		t.Fatal("bank sale failed to pay")
	}
	assertAttackRestored(t, s)
	for _, damage := range []func(*catanAttack){
		func(a *catanAttack) { a.GoldIssued = -1 },
		func(a *catanAttack) { a.GoldIssued++ },
		func(a *catanAttack) { a.GoldIssued = catanGoldLedgerLimit + 1 },
		func(a *catanAttack) { a.Gold[p] = catanGoldLedgerLimit + a.Map.Gold + 1 },
	} {
		next := clone(*s)
		damage(next.Catan.Attack)
		if next.validateCatanAttack() == nil {
			t.Fatal("invalid ledger accepted")
		}
	}
	// Above the old 152-coin shared trade cap; still bounded by actual holdings.
	a.GoldIssued += 200
	a.Gold[p] += 200
	if !s.Catan.hasTradeGold(p, 180) || s.Catan.hasTradeGold(p, 202) {
		t.Fatal("ledger trade balance bound")
	}
	other := (p + 1) % 3
	s.Catan.Bank[1]--
	s.Catan.Players[other].Resources[1]++
	before := slices.Clone(a.Gold)
	if err := s.Apply(p, Action{Type: "catan_trade_offer", Give: []int{0, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0}, GoldGive: 180}); err != nil {
		t.Fatal(err)
	}
	id := s.Catan.Trade.ID
	if err := s.Apply(other, Action{Type: "catan_trade_accept", Offer: id}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(p, Action{Type: "catan_trade_complete", Offer: id, Target: other}); err != nil {
		t.Fatal(err)
	}
	a = s.Catan.Attack
	if a.Gold[p] != before[p]-180 || a.Gold[other] != before[other]+180 || a.GoldIssued != 201 {
		t.Fatal("large trade did not conserve ledger gold")
	}
	assertAttackRestored(t, s)
}
