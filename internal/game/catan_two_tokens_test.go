package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func twoTokenHand(s *State, player int, hand []int) {
	for color, count := range hand {
		catanGive(s.Catan, player, color, count-s.Catan.Players[player].Resources[color])
	}
}

func TestCatanTwoTokensSetupRewardsAndConservation(t *testing.T) {
	seen := map[int]bool{}
	for sample := 0; sample < 48; sample++ {
		s, err := newCatanTwoCore()
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(s.Catan.Two.Tokens, []int{5, 5}) || s.Catan.Two.Bank != 10 {
			t.Fatal("initial stock")
		}
		for s.Catan.setup() {
			p := s.Turn
			a, err := s.BotAction(p)
			if err != nil {
				t.Fatal(err)
			}
			reward := 0
			if a.Type == "catan_settlement" {
				// Exercise inland, desert, coast and combined rewards through
				// actual setup Apply instead of merely testing the calculator.
				for _, v := range s.Catan.Vertices {
					if s.Catan.canSettlement(p, v.ID, true) {
						r := s.Catan.twoSettlementTokens(p, v.ID)
						if !seen[r] {
							a.Vertex = v.ID
							break
						}
					}
				}
				reward = s.Catan.twoSettlementTokens(p, a.Vertex)
				seen[reward] = true
			}
			before := slices.Clone(s.Catan.Two.Tokens)
			bank := s.Catan.Two.Bank
			helperApply(t, s, p, a)
			if s.Catan.Two.Tokens[p] != before[p]+reward || s.Catan.Two.Tokens[1-p] != before[1-p] || s.Catan.Two.Bank != bank-reward {
				t.Fatal("wrong setup recipient")
			}
			twoCoreRestore(t, s)
		}
	}
	for reward := range 4 {
		if !seen[reward] {
			t.Fatal("reward not exercised", reward)
		}
	}
}

func TestCatanTwoTokensForcedTradePrivacyReturnAndRestore(t *testing.T) {
	for _, preRoll := range []bool{true, false} {
		for _, opponentCount := range []int{1, 2, 5} {
			t.Run(fmt.Sprintf("preroll=%v/cards=%d", preRoll, opponentCount), func(t *testing.T) {
				s := twoCoreFixture(t)
				p := s.Turn
				if !preRoll {
					twoCoreActionPhase(t, s)
				}
				resume := s.Phase
				own := 0
				if opponentCount == 1 {
					own = 1
				}
				twoTokenHand(s, p, []int{own, 0, 0, 0, 0})
				twoTokenHand(s, 1-p, []int{0, 0, opponentCount, 0, 0})
				beforeTokens, bank := s.Catan.Two.Tokens[p], s.Catan.Two.Bank
				s.Log = nil
				helperReject(t, s, 1-p, Action{Type: "catan_two_trade"})
				helperReject(t, s, p, Action{Type: "catan_two_trade", Take: []int{0, 0, 2, 0, 0}})
				helperApply(t, s, p, Action{Type: "catan_two_trade"})
				q := s.Catan.Two
				if s.Phase != "catan_two_trade" || s.CatanPendingActor() != p || !q.Spent || q.Tokens[p] != beforeTokens-1 || q.Bank != bank+1 || sum(q.Trade.Drawn) != min(2, opponentCount) {
					t.Fatal("draw or payment")
				}
				for _, viewer := range []int{-1, p, 1 - p} {
					v := s.View(viewer)["catan"].(map[string]any)
					two := v["two"].(map[string]any)
					_, drawn := two["trade"].(map[string]any)["drawn"]
					if drawn != (viewer == p) || two["canAct"] != (viewer == p) {
						t.Fatal("private draw leaked or actor missing")
					}
					for i, raw := range v["players"].([]any) {
						_, hand := raw.(map[string]any)["resources"]
						if hand != (i == viewer) {
							t.Fatal("hand leak")
						}
					}
				}
				for _, color := range CatanResources {
					if strings.Contains(strings.Join(s.Log, " "), color) {
						t.Fatal("private color in trade log")
					}
				}
				twoCoreRestore(t, s)
				for _, a := range []Action{{Type: "catan_two_trade"}, {Type: "catan_end"}, {Type: "catan_roll"}, {Type: "catan_two_robber"}, {Type: "catan_two_return", Give: []int{2, 0, 0, 0, 0}}, {Type: "catan_two_return", Give: []int{0, 0, 1, 0, 0}}} {
					helperReject(t, s, p, a)
				}
				helperReject(t, s, 1-p, Action{Type: "catan_two_return", Give: []int{0, 0, 2, 0, 0}})
				if err := s.EliminateCatan(p); err == nil {
					t.Fatal("pending trader can be removed")
				}
				give := []int{own, 0, min(2, opponentCount), 0, 0}
				helperApply(t, s, p, Action{Type: "catan_two_return", Give: give})
				if s.Phase != resume || s.Catan.Two.Trade != nil || sum(s.Catan.Players[p].Resources) != 0 || sum(s.Catan.Players[1-p].Resources) != opponentCount+own {
					t.Fatal("return of just-drawn cards failed")
				}
				helperReject(t, s, p, Action{Type: "catan_two_trade"})
				helperReject(t, s, p, Action{Type: "catan_two_return", Give: give})
				twoCoreRestore(t, s)
			})
		}
	}
}

func TestCatanTwoTokensEmptyHandsAndPublicPrice(t *testing.T) {
	s := twoCoreFixture(t)
	p := s.Turn
	for _, cards := range [][2]int{{0, 0}, {3, 0}, {0, 1}} {
		twoTokenHand(s, p, []int{cards[0], 0, 0, 0, 0})
		twoTokenHand(s, 1-p, []int{0, cards[1], 0, 0, 0})
		helperReject(t, s, p, Action{Type: "catan_two_trade"})
	}
	twoTokenHand(s, 1-p, []int{0, 2, 0, 0, 0})
	catanCard(s.Catan, p, 4)
	s.catanScores()
	if s.Catan.Players[p].Score != 3 || s.Catan.twoTokenCost(p) != 1 {
		t.Fatal("hidden VP affects token cost")
	}
	// Upgrade an existing settlement directly in this price fixture.
	for i, v := range s.Catan.Vertices {
		if v.Owner == p && v.Level == 1 {
			s.Catan.Vertices[i].Level = 2
			break
		}
	}
	s.catanScores()
	if s.Catan.twoTokenCost(p) != 2 || s.Catan.twoTokenCost(1-p) != 1 {
		t.Fatal("public score price")
	}
	before := s.Catan.Two.Tokens[p]
	helperApply(t, s, p, Action{Type: "catan_two_trade"})
	if s.Catan.Two.Tokens[p] != before-2 {
		t.Fatal("wrong paid price")
	}
	s.AutoCatanPending()
	if s.Catan.Two.Trade != nil || s.Phase != "catan_roll" {
		t.Fatal("auto return failed")
	}
}

func TestCatanTwoTokensKnightArmyAndIndependentDevelopment(t *testing.T) {
	s := twoCoreFixture(t)
	p := s.Turn
	g := s.Catan
	// Each fixture knight is a real card moved from the deck into the existing
	// played-card ledger. Exchanging one must not duplicate it in that ledger.
	for i, n := range []int{3, 3} {
		for range n {
			catanCard(g, i, 0)
			g.Players[i].Dev[0]--
			g.DevDiscard = append(g.DevDiscard, 0)
			g.Players[i].Knights++
		}
	}
	g.ArmyOwner = p
	s.catanScores()
	before := len(g.DevDiscard)
	tokens := g.Two.Tokens[p]
	helperApply(t, s, p, Action{Type: "catan_two_knight"})
	g = s.Catan
	if g.ArmyOwner != 1-p || g.Players[p].Knights != 2 || g.Two.Tokens[p] != tokens+2 || g.PlayedDev || len(g.DevDiscard) != before || !g.Two.KnightExchanged || g.Two.Spent {
		t.Fatal("knight exchange/army/card stock")
	}
	helperReject(t, s, p, Action{Type: "catan_two_knight"})
	catanCard(g, p, 0)
	helperApply(t, s, p, Action{Type: "catan_dev", Card: 0})
	a, err := s.BotAction(p)
	if err != nil {
		t.Fatal(err)
	}
	helperApply(t, s, p, a)
	if s.Phase != "catan_roll" || !s.Catan.PlayedDev {
		t.Fatal("exchange consumed development action")
	}
	// Move the robber away from the desert, then pay with no theft.
	g = s.Catan
	for _, tile := range g.Tiles {
		if tile.Resource != CatanDesert {
			g.Robber = tile.ID
			break
		}
	}
	hands := [][]int{slices.Clone(g.Players[0].Resources), slices.Clone(g.Players[1].Resources)}
	helperApply(t, s, p, Action{Type: "catan_two_robber"})
	if s.Catan.Robber != s.Catan.twoDesert() || !s.Catan.PlayedDev {
		t.Fatal("robber relocation")
	}
	for i := range 2 {
		if !slices.Equal(hands[i], s.Catan.Players[i].Resources) {
			t.Fatal("retreat stole resources")
		}
	}
	helperReject(t, s, p, Action{Type: "catan_two_robber"})
	twoCoreActionPhase(t, s)
	helperApply(t, s, p, Action{Type: "catan_end"})
	if s.Catan.Two.Spent || s.Catan.Two.KnightExchanged {
		t.Fatal("next turn limits not reset")
	}
	twoCoreRestore(t, s)
}

func TestCatanTwoTokensBotPrivateInformationIndependent(t *testing.T) {
	s := twoCoreFixture(t)
	p := s.Turn
	twoTokenHand(s, p, []int{4, 0, 0, 0, 0})
	twoTokenHand(s, 1-p, []int{0, 4, 0, 0, 0})
	s.Catan.Robber = s.Catan.twoDesert()
	a, err := s.BotAction(p)
	if err != nil || a.Type != "catan_two_trade" {
		t.Fatal("bot did not use surplus trade", a, err)
	}
	other := clone(*s)
	twoTokenHand(&other, 1-p, []int{0, 0, 0, 0, 4})
	slices.Reverse(other.Catan.DevDeck)
	b, err := other.BotAction(p)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("bot inspected opponent colors or deck")
	}
	helperApply(t, s, p, a)
	twoCoreRestore(t, s)
	a, err = s.BotAction(p)
	if err != nil {
		t.Fatal(err)
	}
	other = clone(*s)
	other.Catan.Players[1-p].Resources = []int{2, 1, 7, 3, 6}
	slices.Reverse(other.Catan.DevDeck)
	b, err = other.BotAction(p)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("return bot read opponent hand")
	}
	s.AutoCatanPending()
	if s.CatanPendingActor() != -1 || s.Phase != "catan_roll" {
		t.Fatal("timeout stuck")
	}
}

func TestCatanTwoTokensSupplyAndCorruptionAtomic(t *testing.T) {
	s := twoCoreFixture(t)
	p := s.Turn
	s.Catan.Two.Tokens = []int{10, 10}
	s.Catan.Two.Bank = 0
	s.Catan.Players[p].Knights = 1
	helperReject(t, s, p, Action{Type: "catan_two_knight"})
	// This is only the documented protective guard, not accepted shortage rules.
	for _, bad := range []func(*State){
		func(s *State) { s.Catan.Two.Bank++ },
		func(s *State) { s.Catan.Two.Tokens = nil },
		func(s *State) { s.Catan.Two.Tokens = []int{-1, 21} },
		func(s *State) { s.Phase = "catan_two_trade" },
		func(s *State) { s.Catan.Two.Trade = &CatanTwoTrade{Resume: "catan_turn", Drawn: []int{2, 0, 0, 0, 0}} },
	} {
		x := clone(*s)
		bad(&x)
		helperReject(t, &x, p, Action{Type: "catan_roll"})
	}
	// Persisted private choice validation must reject impossible resumptions,
	// double pending responses and resources not present in the actor's hand.
	s = twoCoreFixture(t)
	p = s.Turn
	twoTokenHand(s, p, []int{0, 0, 0, 0, 0})
	twoTokenHand(s, 1-p, []int{0, 0, 2, 0, 0})
	helperApply(t, s, p, Action{Type: "catan_two_trade"})
	for _, bad := range []func(*State){
		func(s *State) { s.Catan.Two.Trade.Resume = "catan_roads" },
		func(s *State) { s.Catan.Two.Trade.Resume = "catan_turn" },
		func(s *State) { s.Catan.Two.Trade.Drawn = []int{2, 0, 0, 0, 0} },
		func(s *State) { s.Catan.Two.Pending = &CatanTwoPending{Kind: "road", Resume: "catan_roll"} },
		func(s *State) { s.Catan.Two.Spent = false },
	} {
		x := clone(*s)
		bad(&x)
		helperReject(t, &x, p, Action{Type: "catan_two_return", Give: []int{0, 0, 2, 0, 0}})
	}
	b, _ := json.Marshal(s)
	twoCoreRestore(t, s)
	after, _ := json.Marshal(s)
	if string(b) != string(after) {
		t.Fatal("restore redrew")
	}
}

func TestCatanTwoTokensTimingAndInsufficientFunds(t *testing.T) {
	s := twoCoreFixture(t)
	p := s.Turn
	// No-op retreat neither spends nor grants an extra action.
	helperReject(t, s, p, Action{Type: "catan_two_robber"})
	twoTokenHand(s, 1-p, []int{0, 2, 0, 0, 0})
	s.Catan.Two.Bank += s.Catan.Two.Tokens[p]
	s.Catan.Two.Tokens[p] = 0
	helperReject(t, s, p, Action{Type: "catan_two_trade"})
	if err := twoCoreRoll(t, s, 1, 1); err != nil {
		t.Fatal(err)
	}
	// Actions between the two production phases remain gated until the
	// official timing is verified. No pending choice can bypass this gate.
	s.Catan.Two.Bank--
	s.Catan.Two.Tokens[p]++
	helperReject(t, s, p, Action{Type: "catan_two_trade"})
	helperReject(t, s, p, Action{Type: "catan_two_knight"})
	if err := twoCoreRoll(t, s, 6, 6); err != nil {
		t.Fatal(err)
	}
	twoTokenHand(s, 1-p, []int{0, 2, 0, 0, 0})
	helperApply(t, s, p, Action{Type: "catan_two_trade"})
	s.AutoCatanPending()
	if s.Phase != "catan_turn" {
		t.Fatal("action phase not restored")
	}
	ordinary, err := NewCatan(3, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	helperReject(t, ordinary, ordinary.Turn, Action{Type: "catan_two_trade"})
}

func TestCatanTwoTokensUnverifiedSupplyRollsBackSettlement(t *testing.T) {
	s, err := newCatanTwoCore()
	if err != nil {
		t.Fatal(err)
	}
	s.Catan.Two.Bank = 0
	s.Catan.Two.Tokens = []int{10, 10}
	for _, v := range s.Catan.Vertices {
		if s.Catan.canSettlement(s.Turn, v.ID, true) && s.Catan.twoSettlementTokens(s.Turn, v.ID) > 0 {
			// The protective rejection happens after catanSetup has modified
			// the clone. Nothing, including building/phase/log, may escape.
			helperReject(t, s, s.Turn, Action{Type: "catan_settlement", Vertex: v.ID})
			return
		}
	}
	t.Fatal("no legal reward-bearing settlement")
}
