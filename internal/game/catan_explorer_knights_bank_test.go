package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerCityBankCardRatesAndConservation(t *testing.T) {
	for _, config := range []struct {
		n         int
		secondary bool
	}{{3, false}, {6, false}, {6, true}} {
		for give := range 8 {
			for take := range 8 {
				if give == take {
					continue
				}
				t.Run(fmt.Sprintf("%d/%t/%d-%d", config.n, config.secondary, give, take), func(t *testing.T) {
					s := explorerTradeProgressFixture(t, config.n, config.secondary)
					actor, cost := s.Turn, 3
					if give >= 5 {
						cost = 4
					}
					explorerDevelopmentGrant(t, s, actor, give, cost-1)
					a := Action{Type: "catan_explorer_bank", Color: give, Target: take, Prompt: int(s.Catan.TurnSerial)}
					explorerCityActionReject(t, s, actor, a)
					explorerDevelopmentGrant(t, s, actor, give, 1)
					before := clone(*s.Catan)
					explorerKnightAct(t, s, a)
					if s.Catan.Players[actor].Resources[give] != 0 || s.Catan.Players[actor].Resources[take] != 1 || s.Catan.Bank[give] != before.Bank[give]+cost || s.Catan.Bank[take] != before.Bank[take]-1 || !reflect.DeepEqual(s.Catan.Explorer.Economy, before.Explorer.Economy) || !reflect.DeepEqual(s.Catan.Explorer.Cargo, before.Explorer.Cargo) {
						t.Fatal("wrong combined bank price, inventory or action state")
					}
				})
			}
		}
	}
}

func TestCatanExplorerCityBankFleetAndCommerce(t *testing.T) {
	for _, config := range []struct {
		n         int
		secondary bool
	}{{3, false}, {6, false}, {6, true}} {
		t.Run(fmt.Sprintf("%d/%t", config.n, config.secondary), func(t *testing.T) {
			s := explorerTradeProgressFixture(t, config.n, config.secondary)
			p := s.Turn
			ckProgressGive(t, s, p, 13, 13)
			for _, color := range []int{-1, 8} {
				explorerCityActionReject(t, s, p, Action{Type: "catan_progress", Card: 13, Color: color, Prompt: int(s.Catan.TurnSerial)})
			}
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 13, Color: 0})
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 13, Color: 5})
			explorerDevelopmentGrant(t, s, p, 0, 4)
			explorerDevelopmentGrant(t, s, p, 5, 4)
			for range 2 {
				explorerKnightAct(t, s, Action{Type: "catan_explorer_bank", Color: 0, Target: 6})
				explorerKnightAct(t, s, Action{Type: "catan_explorer_bank", Color: 5, Target: 4})
			}
			if s.Catan.Players[p].Resources[6] != 2 || s.Catan.Players[p].Resources[4] != 2 || s.Catan.explorerCityRates((p + 1) % config.n)[5] != 4 {
				t.Fatal("fleets failed repeated trades or affected opponent")
			}
			explorerCityFinishAction(t, s)
			if s.Catan.CitiesKnights.TradePowers != nil || s.Catan.explorerCityRates(p)[0] != 3 || s.Catan.explorerCityRates(p)[5] != 4 {
				t.Fatal("fleet survived action handoff")
			}
			if s.Phase == "catan_roll" {
				if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
					t.Fatal(err)
				}
			}
			p = s.Turn
			// Actually buy the three commerce improvements before using 2:1.
			for level := 1; level <= 3; level++ {
				explorerDevelopmentGrant(t, s, p, 6, level)
				explorerKnightAct(t, s, Action{Type: "catan_improvement", Color: CatanCommerce})
			}
			for color := 5; color < 8; color++ {
				explorerDevelopmentGrant(t, s, p, color, 2)
				before := s.Catan.Players[p].Resources[color]
				explorerKnightAct(t, s, Action{Type: "catan_explorer_bank", Color: color, Target: 0})
				if s.Catan.Players[p].Resources[color] != before-2 {
					t.Fatal("commerce level three did not discount commodity")
				}
			}
		})
	}
}

func explorerMerchantTile(t *testing.T, s *State, p int) int {
	t.Helper()
	for _, id := range s.Catan.merchantTiles(p) {
		if s.Catan.Tiles[id].Resource < 5 {
			return id
		}
	}
	t.Fatal("missing merchant resource tile")
	return -1
}

func TestCatanExplorerCityMerchantRateTransferAndVictory(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerTradeProgressFixture(t, n, false)
			ckProgressGive(t, s, 0, 12, 12)
			tile := explorerMerchantTile(t, s, 0)
			color := s.Catan.Tiles[tile].Resource
			start := s.Catan.Players[0].Score
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 12, Tile: tile})
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 12, Tile: tile})
			if s.Catan.Players[0].Score != start+1 || s.Catan.explorerCityRates(0)[color] != 2 || s.Catan.explorerCityRates(0)[5] != 4 {
				t.Fatal("merchant point or resource-only discount wrong")
			}
			explorerDevelopmentGrant(t, s, 0, color, 2)
			explorerKnightAct(t, s, Action{Type: "catan_explorer_bank", Color: color, Target: 5})
			explorerCityFinishAction(t, s)
			if s.Phase == "catan_roll" {
				if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
					t.Fatal(err)
				}
			}
			p := s.Turn
			tile = explorerMerchantTile(t, s, p)
			ckProgressGive(t, s, p, 12)
			before := s.Catan.Players[p].Score
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 12, Tile: tile})
			if s.Catan.Players[0].Score != start || s.Catan.Players[p].Score != before+1 || s.Catan.explorerCityRates(0)[color] != 3 {
				t.Fatal("merchant transfer kept old point/discount")
			}
			ckProgressGive(t, s, p, 12)
			for _, v := range []int{-1, len(s.Catan.Tiles), s.Catan.Explorer.Board.Hidden[0].Tile, s.Catan.Explorer.Board.FrameSea} {
				explorerCityActionReject(t, s, p, Action{Type: "catan_progress", Card: 12, Tile: v, Prompt: int(s.Catan.TurnSerial)})
			}
			// Controlled defender points place the next owner one point below
			// the scenario target; Merchant must immediately finish even paired.
			s = explorerTradeProgressFixture(t, n, n == 6)
			p = s.Turn
			s.Catan.CitiesKnights.Players[p].DefenderPoints = s.Catan.Explorer.Board.Target - 5
			s.catanScores()
			ckProgressGive(t, s, p, 12)
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 12, Tile: explorerMerchantTile(t, s, p)})
			if !s.Finished || !slices.Equal(s.Winners, []int{p}) || s.Catan.Players[p].Score != s.Catan.Explorer.Board.Target {
				t.Fatal("merchant did not trigger combo-target victory")
			}
		})
	}
}

func TestCatanExplorerCityBankGoldLimitsShortagesAndGuards(t *testing.T) {
	s := explorerTradeProgressFixture(t, 6, false)
	x := s.Catan.Explorer
	x.Economy.GoldBank -= 6 - x.Economy.Gold[0]
	x.Economy.Gold[0] = 6
	for _, bad := range []Action{{Color: -2, Target: 0}, {Color: 8, Target: 0}, {Color: 0, Target: 8}, {Color: -1, Target: -1}, {Color: 0, Target: 0}, {Color: -1, Target: 5}, {Color: 5, Target: -1}, {Color: 0, Target: 1, Choice: "free"}, {Color: 0, Target: 1, Give: []int{1}}} {
		bad.Type = "catan_explorer_bank"
		bad.Prompt = 1
		explorerCityActionReject(t, s, 0, bad)
	}
	explorerCityActionReject(t, s, 1, Action{Type: "catan_explorer_bank", Color: -1, Target: 0, Prompt: 1})
	explorerCityActionReject(t, s, 0, Action{Type: "catan_explorer_bank", Color: -1, Target: 0, Prompt: 0})
	for range 2 {
		explorerKnightAct(t, s, Action{Type: "catan_explorer_bank", Color: -1, Target: 0})
	}
	explorerCityActionReject(t, s, 0, Action{Type: "catan_explorer_bank", Color: -1, Target: 0, Prompt: 1})
	if s.Catan.Explorer.Economy.Gold[0] != 2 || s.Catan.Explorer.Economy.Turn.Bought != 2 {
		t.Fatal("wrong gold purchase cost/limit")
	}
	// Selling resources for gold neither consumes nor resets the buy allowance.
	explorerDevelopmentGrant(t, s, 0, 1, 3)
	explorerKnightAct(t, s, Action{Type: "catan_explorer_bank", Color: 1, Target: -1})
	if s.Catan.Explorer.Economy.Turn.Bought != 2 || s.Catan.Explorer.Economy.Gold[0] != 3 {
		t.Fatal("selling reset purchase count")
	}
	explorerCityFinishAction(t, s)
	p := s.Turn
	if p != 3 || s.Catan.Explorer.Economy.Turn.Bought != 0 {
		t.Fatal("paired allowance not independent")
	}
	x = s.Catan.Explorer
	x.Economy.GoldBank -= 4 - x.Economy.Gold[p]
	x.Economy.Gold[p] = 4
	for range 2 {
		explorerKnightAct(t, s, Action{Type: "catan_explorer_bank", Color: -1, Target: 1})
	}
	for _, kind := range []string{"resource", "gold"} {
		q := explorerTradeProgressFixture(t, 3, false)
		explorerDevelopmentGrant(t, q, 0, 0, 3)
		a := Action{Type: "catan_explorer_bank", Color: 0, Target: 1, Prompt: 1}
		if kind == "resource" {
			explorerDevelopmentGrant(t, q, 1, 1, q.Catan.Bank[1])
		} else {
			e := q.Catan.Explorer.Economy
			e.Gold[1] += e.GoldBank
			e.GoldBank = 0
			a.Target = -1
		}
		explorerCityActionReject(t, q, 0, a)
	}
}

func TestCatanExplorerCityBankPrivilegeStackingAndCorruptSave(t *testing.T) {
	s := explorerTradeProgressFixture(t, 3, false)
	tile := explorerMerchantTile(t, s, 0)
	color := s.Catan.Tiles[tile].Resource
	ckProgressGive(t, s, 0, 12, 13, 13, 10)
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 12, Tile: tile})
	for range 2 {
		explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 13, Color: color})
	}
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 10})
	if !slices.Equal(s.Catan.CitiesKnights.TradePowers.Fleets, []int{color}) || len(s.Catan.CitiesKnights.TradePowers.Harbors) != 1 {
		t.Fatal("repeated fleet duplicated discount or overwrote harbor")
	}
	explorerDevelopmentGrant(t, s, 0, color, 1)
	explorerCityActionReject(t, s, 0, Action{Type: "catan_explorer_bank", Color: color, Target: 5, Prompt: 1})
	explorerDevelopmentGrant(t, s, 0, color, 1)
	explorerKnightAct(t, s, Action{Type: "catan_explorer_bank", Color: color, Target: 5})
	if s.Catan.Players[0].Resources[color] != 0 || s.Catan.Players[0].Resources[5] != 1 {
		t.Fatal("discounts stacked below two cards")
	}
	// Gold uses the separate printed three-resource exchange, not a card rate.
	explorerDevelopmentGrant(t, s, 0, color, 2)
	explorerCityActionReject(t, s, 0, Action{Type: "catan_explorer_bank", Color: color, Target: -1, Prompt: 1})
	explorerDevelopmentGrant(t, s, 0, color, 1)
	explorerKnightAct(t, s, Action{Type: "catan_explorer_bank", Color: color, Target: -1})
	for _, kind := range []string{"merchant-owner", "merchant-outside", "merchant-sea", "merchant-foreign", "fleet-negative", "fleet-outside", "fleet-duplicate", "fleet-excess", "fleet-owner"} {
		t.Run(kind, func(t *testing.T) {
			q := clone(*s)
			k := q.Catan.CitiesKnights
			switch kind {
			case "merchant-owner":
				k.Merchant.Owner = -1
			case "merchant-outside":
				k.Merchant.Tile = len(q.Catan.Tiles)
			case "merchant-sea":
				k.Merchant.Tile = q.Catan.Explorer.Board.FrameSea
			case "merchant-foreign":
				found := false
				for _, id := range q.Catan.merchantTiles(1) {
					if !slices.Contains(q.Catan.merchantTiles(0), id) {
						k.Merchant.Tile = id
						found = true
						break
					}
				}
				if !found {
					t.Fatal("fixture lacks foreign merchant tile")
				}
			case "fleet-negative":
				k.TradePowers.Fleets = []int{-1}
			case "fleet-outside":
				k.TradePowers.Fleets = []int{8}
			case "fleet-duplicate":
				k.TradePowers.Fleets = []int{color, color}
			case "fleet-excess":
				k.TradePowers.Fleets = []int{0, 1, 2}
			case "fleet-owner":
				k.TradePowers.Player = 1
			}
			explorerCityActionReject(t, &q, 0, Action{Type: "catan_explorer_bank", Color: color, Target: 5, Prompt: 1})
		})
	}
}
