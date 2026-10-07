package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func assertRiverLedgerRestore(t *testing.T, s *State) {
	t.Helper()
	riverConserved(t, s)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	riverConserved(t, &restored)
}

func TestCatanRiversGoldLedgerConstruction(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, kind := range []string{"catan_bridge", "catan_road", "catan_settlement"} {
			for bank := 0; bank <= 3; bank++ {
				t.Run(fmt.Sprintf("%d/%s/%d", n, kind, bank), func(t *testing.T) {
					s := riversFixture(t, n)
					g := s.Catan
					riverGold(g, 1, g.Rivers.Bank-bank)
					a := Action{Type: kind}
					reward := 1
					switch kind {
					case "catan_bridge":
						a.Edge = g.Rivers.Map.Bridges[0]
						g.Vertices[g.Edges[a.Edge].A].Owner, g.Vertices[g.Edges[a.Edge].A].Level = 0, 1
						reward = 3
					case "catan_road":
						a.Edge = -1
						for _, e := range g.Edges {
							if g.riverEdge(e.ID) && !slices.Contains(g.Rivers.Map.Bridges, e.ID) {
								a.Edge = e.ID
								g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 0, 1
								break
							}
						}
						if a.Edge < 0 {
							t.Fatal("missing riverbank")
						}
					case "catan_settlement":
						a.Vertex = -1
						for _, v := range g.Vertices {
							if g.riverVertex(v.ID) && g.canSettlement(0, v.ID, true) {
								for _, id := range g.touching(v.ID) {
									if !slices.Contains(g.Rivers.Map.Bridges, id) {
										g.Edges[id].Owner = 0
										a.Vertex = v.ID
										break
									}
								}
								if a.Vertex >= 0 {
									break
								}
							}
						}
						if a.Vertex < 0 {
							t.Fatal("missing river settlement")
						}
					}
					for c, cost := range catanPrices[kind] {
						catanGive(g, 0, c, cost)
					}
					if err := s.Apply(0, a); err != nil {
						t.Fatal(err)
					}
					r := s.Catan.Rivers
					if r.Bank != max(0, bank-reward) || r.GoldIssued != max(0, reward-bank) || r.Gold[0] != reward {
						t.Fatal("incorrect reward/shortfall", r)
					}
					riverReject(t, s, 0, a)
					assertRiverLedgerRestore(t, s)
				})
			}
		}
	}
}

func TestCatanRiversGoldLedgerSaleRecyclingAndGuards(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := riversFixture(t, n)
			g := s.Catan
			riverGold(g, 1, g.Rivers.Bank)
			catanGive(g, 0, 0, 12)
			for i := 0; i < 2; i++ {
				if err := s.Apply(0, Action{Type: "catan_coin_sell", Color: 0}); err != nil {
					t.Fatal(err)
				}
			}
			if s.Catan.Rivers.GoldIssued != 2 {
				t.Fatal("sale credits missing")
			}
			if err := s.Apply(0, Action{Type: "catan_coin_buy", Color: 1}); err != nil {
				t.Fatal(err)
			}
			if err := s.Apply(0, Action{Type: "catan_coin_sell", Color: 0}); err != nil {
				t.Fatal(err)
			}
			r := s.Catan.Rivers
			if r.GoldIssued != 2 || r.Bank != 1 || r.Gold[0] != 1 || r.Bought != 1 {
				t.Fatal("coins not recycled", r)
			}
			assertRiverLedgerRestore(t, s)
			for _, damage := range []func(*CatanRivers){
				func(r *CatanRivers) { r.GoldIssued = -1 },
				func(r *CatanRivers) { r.GoldIssued++ },
				func(r *CatanRivers) { r.GoldIssued = catanGoldLedgerLimit + 1 },
				func(r *CatanRivers) { r.Gold[0] = int(^uint(0) >> 1) },
				func(r *CatanRivers) { r.Bank = -1 },
			} {
				next := clone(*s)
				damage(next.Catan.Rivers)
				if next.Catan.validateRivers() == nil {
					t.Fatal("corrupt ledger accepted")
				}
			}
			next := clone(*s)
			next.Catan.SetupStep = 0
			if next.Catan.validateRivers() == nil {
				t.Fatal("setup cannot contain issued credits")
			}
			// Exhaust the safe numeric limit, not the physical box: reject atomically.
			g = s.Catan
			r.Bank = 0
			r.Gold[1]++
			r.Gold[1] += catanGoldLedgerLimit - r.GoldIssued
			r.GoldIssued = catanGoldLedgerLimit
			catanGive(g, 0, 0, 4)
			riverReject(t, s, 0, Action{Type: "catan_coin_sell", Color: 0})
			assertRiverLedgerRestore(t, s)
		})
	}
	// Missing goldIssued remains compatible with physical-supply old saves.
	legacy := riversFixture(t, 3)
	assertRiverLedgerRestore(t, legacy)
}

func TestCatanRiversGoldLedgerLargeTradeAndBot(t *testing.T) {
	s := riversFixture(t, 3)
	g := s.Catan
	riverGold(g, 1, g.Rivers.Bank)
	catanGive(g, 0, 0, 4)
	choices := g.riverBotChoices(0)
	if !slices.ContainsFunc(choices, func(c botChoice) bool { return c.action.Type == "catan_coin_sell" }) {
		t.Fatal("bot cannot sell with empty gold bank")
	}
	g.Rivers.GoldIssued = 200
	g.Rivers.Gold[0] = 200
	catanGive(g, 1, 1, 1)
	if !g.hasTradeGold(0, 180) || g.hasTradeGold(0, 201) {
		t.Fatal("trade balance bounds")
	}
	if err := s.Apply(0, Action{Type: "catan_trade_offer", Give: make([]int, 5), Take: []int{0, 1, 0, 0, 0}, GoldGive: 180}); err != nil {
		t.Fatal(err)
	}
	id := s.Catan.Trade.ID
	if err := s.Apply(1, Action{Type: "catan_trade_accept", Offer: id}); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(0, Action{Type: "catan_trade_complete", Offer: id, Target: 1}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Rivers.Gold[0] != 20 || s.Catan.Rivers.Gold[1] != 280 || s.Catan.Rivers.GoldIssued != 200 {
		t.Fatal("trade ledger mismatch")
	}
	assertRiverLedgerRestore(t, s)
}

func TestCatanTwoRiversGoldLedgerNeutralIsolation(t *testing.T) {
	s := twoRiverFixture(t)
	g, p := s.Catan, s.Turn
	other := 1 - p
	g.Rivers.Gold[other] += g.Rivers.Bank
	g.Rivers.Bank = 0
	tokens := slices.Clone(g.Two.Tokens)
	before := slices.Clone(g.Rivers.Gold)
	if err := s.catanRiverReward(p, 3); err != nil {
		t.Fatal(err)
	}
	for _, neutral := range catanTwoNeutralOwners {
		if err := s.catanRiverReward(neutral, 3); err != nil {
			t.Fatal(err)
		}
	}
	if g.Rivers.GoldIssued != 3 || g.Rivers.Gold[p] != before[p]+3 || g.Rivers.Gold[other] != before[other] || !slices.Equal(tokens, g.Two.Tokens) {
		t.Fatal("neutral rewards/token supply changed")
	}
	twoRiverCheck(t, s)
}
