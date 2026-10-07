package game

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// A controlled mission midgame: reveal the two gold farms, assign crew and
// sacks, and conserve all eight card inventories. No claim of natural play.
func explorerCityGoldFixture(t *testing.T, n int) *State {
	t.Helper()
	s := explorerCityScenarioFixture(t, n, "spices-for-catan")
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	g, x := s.Catan, s.Catan.Explorer
	x.Spice = &catanExplorerSpice{Deliveries: []catanExplorerSpiceDelivery{}}
	x.Fish = &catanExplorerFish{Deliveries: []catanExplorerFishDelivery{}}
	crew := 2
	for _, hidden := range slices.Clone(x.Board.Hidden) {
		if hidden.Farm != "gold" {
			continue
		}
		if _, err := x.Board.reveal(g, hidden.Tile); err != nil {
			t.Fatal(err)
		}
		if err := x.Cargo.discoverSpice(g, x.Board, x.Fleet, hidden.Tile); err != nil {
			t.Fatal(err)
		}
		// Player zero and the six-player secondary actor each have both farms.
		for _, p := range []int{0, n - 3} {
			if p == 0 && n == 3 && x.Cargo.farmFriend(0, hidden.Tile) {
				continue
			}
			harbor := -1
			for _, v := range g.Vertices {
				if v.Owner == p && v.Harbor {
					harbor = v.ID
					break
				}
			}
			explorerSpiceClaimFixture(t, s, p, hidden.Tile, crew, catanExplorerCargoLocation{"harbor", harbor})
		}
		crew++
	}
	for p := range g.Players {
		for r := range g.Bank {
			delta := 2 - g.Players[p].Resources[r]
			g.Players[p].Resources[r] += delta
			g.Bank[r] -= delta
		}
	}
	if err := x.Spice.validate(g, x.Board, x.Fleet, x.Cargo, x.Economy); err != nil {
		t.Fatal(err)
	}
	explorerCityRestore(t, s)
	return s
}

func TestCatanExplorerCityFastGoldCommodities(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, card := range []int{0, 5, 6, 7} {
			t.Run(fmt.Sprintf("%d/card%d", n, card), func(t *testing.T) {
				s := explorerCityGoldFixture(t, n)
				a := Action{Type: "catan_explorer_spice_gold", Card: card, Prompt: int(s.Catan.TurnSerial)}
				found := false
				for _, choice := range s.catanExplorerSpiceChoices(0) {
					found = found || choice.Type == a.Type && choice.Card == card
				}
				if !found || len(s.catanExplorerSpiceChoices(-1)) != 0 || len(s.catanExplorerSpiceChoices(1)) != 0 {
					t.Fatal("wrong owner-only gold preview")
				}
				initial := clone(*s)
				explorerCityActionReject(t, s, 1, a)
				for used := 1; used <= 2; used++ {
					if err := s.catanExplorerCityAction(0, a); err != nil {
						t.Fatal(err)
					}
					g, x := s.Catan, s.Catan.Explorer
					if g.Players[0].Resources[card] != initial.Catan.Players[0].Resources[card]-used || g.Bank[card] != initial.Catan.Bank[card]+used || x.Economy.Gold[0] != initial.Catan.Explorer.Economy.Gold[0]+used || x.Economy.GoldBank != initial.Catan.Explorer.Economy.GoldBank-used || x.Spice.GoldUse.Count != used {
						t.Fatal("incorrect gold exchange or conservation")
					}
					if !strings.Contains(s.Log[len(s.Log)-1], catanCardName(card)+"×1") {
						t.Fatal("log lacks actual card")
					}
					explorerCityRestore(t, s)
				}
				explorerCityActionReject(t, s, 0, a)
				if len(s.catanExplorerSpiceChoices(0)) != 0 {
					t.Fatal("spent farm remains available")
				}
				for _, bad := range []int{-1, 8} {
					trial := clone(initial)
					a.Card = bad
					explorerCityActionReject(t, &trial, 0, a)
				}
			})
		}
	}
}

func explorerCityFinishAction(t *testing.T, s *State) {
	t.Helper()
	g, x := s.Catan, s.Catan.Explorer
	if err := x.Cargo.beginMovement(g, x.Fleet, s.Turn, g.TurnSerial, 0); err != nil {
		t.Fatal(err)
	}
	if err := x.Cargo.endMovement(g, x.Fleet, s.Turn, g.TurnSerial); err != nil {
		t.Fatal(err)
	}
	if err := s.catanExplorerAdvanceTurn(); err != nil {
		t.Fatal(err)
	}
	s.Phase = s.catanExplorerPhase()
	explorerCityRestore(t, s)
}

func TestCatanExplorerCityFastGoldPairedResetAndGuards(t *testing.T) {
	s := explorerCityGoldFixture(t, 6)
	a := Action{Type: "catan_explorer_spice_gold", Card: 5, Prompt: int(s.Catan.TurnSerial)}
	if err := s.catanExplorerCityAction(0, a); err != nil {
		t.Fatal(err)
	}
	explorerCityFinishAction(t, s)
	if s.Turn != 3 || !s.Catan.Explorer.Economy.Turn.NoProduction {
		t.Fatal("missing secondary actor")
	}
	explorerCityActionReject(t, s, 3, a)
	a.Prompt = int(s.Catan.TurnSerial)
	for range 2 {
		if err := s.catanExplorerCityAction(3, a); err != nil {
			t.Fatal(err)
		}
	}
	explorerCityActionReject(t, s, 3, a)
	if s.Catan.Explorer.Spice.GoldUse.Player != 3 || s.Catan.Explorer.Spice.GoldUse.Count != 2 {
		t.Fatal("secondary allowance not independent")
	}
	explorerCityRestore(t, s)
	// Rotate through real ordinary and paired phase handoffs to player zero.
	for range 12 {
		explorerCityFinishAction(t, s)
		if s.Phase == "catan_roll" {
			if err := s.catanExplorerCityRoll(1, 1, 0); err != nil {
				t.Fatal(err)
			}
		}
		if s.Turn == 0 {
			break
		}
	}
	if s.Turn != 0 {
		t.Fatal("did not return to farm owner")
	}
	a.Prompt = int(s.Catan.TurnSerial)
	if err := s.catanExplorerCityAction(0, a); err != nil {
		t.Fatal("next action allowance did not reset", err)
	}
	explorerCityRestore(t, s)
	for _, reason := range []string{"ledger_overflow", "unowned_farm", "movement", "commodity_missing"} {
		t.Run(reason, func(t *testing.T) {
			trial := explorerCityGoldFixture(t, 3)
			g, x := trial.Catan, trial.Catan.Explorer
			a := Action{Type: "catan_explorer_spice_gold", Card: 6, Prompt: int(g.TurnSerial)}
			switch reason {
			case "ledger_overflow":
				x.Economy.GoldIssued = catanExplorerGoldLedgerLimit
				x.Economy.Gold[1] += x.Economy.GoldIssued
				x.Economy.Gold[1] += x.Economy.GoldBank
				x.Economy.GoldBank = 0
			case "unowned_farm":
				for i, loc := range x.Cargo.Units {
					if i/11 == 0 && loc.Kind == "farm" {
						x.Cargo.Units[i] = catanExplorerCargoLocation{"supply", -1}
					}
				}
				for i := range x.Cargo.Spice {
					if x.Cargo.Spice[i].Owner == 0 {
						x.Cargo.Spice[i].Owner = -1
						x.Cargo.Spice[i].At = catanExplorerCargoLocation{"farm", x.Cargo.Spice[i].Origin}
					}
				}
			case "movement":
				if err := x.Cargo.beginMovement(g, x.Fleet, 0, g.TurnSerial, 0); err != nil {
					t.Fatal(err)
				}
				trial.Phase = "catan_explorer_move"
			case "commodity_missing":
				g.Bank[6] += g.Players[0].Resources[6]
				g.Players[0].Resources[6] = 0
			}
			if err := x.Spice.validate(g, x.Board, x.Fleet, x.Cargo, x.Economy); err != nil {
				t.Fatal("invalid guard fixture", err)
			}
			explorerCityActionReject(t, trial, 0, a)
		})
	}
}
