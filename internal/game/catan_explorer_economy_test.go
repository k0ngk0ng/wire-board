package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Isolated producing hexes avoid random-map outcomes in resource/shortage
// checks. This is a rule fixture, not a legal full scenario or natural game.
func explorerEconomyFixture(t *testing.T, scenario string) (*Catan, *catanExplorerSailing, *catanExplorerCargo, *catanExplorerEconomy) {
	t.Helper()
	g := &Catan{Players: make([]CatanPlayer, 4), Bank: []int{19, 19, 19, 19, 19}, Robber: -1}
	for p := range g.Players {
		g.Players[p].Resources = make([]int, 5)
	}
	if err := g.makeScenarioMap([]CatanHexSpec{{Resource: 0, Number: 6}, {Q: 3, Resource: 3, Number: 6}, {Q: 6, Resource: CatanGold, Number: 6}, {Q: 9, Resource: 4, Number: 8}}); err != nil {
		t.Fatal(err)
	}
	for _, b := range []struct{ tile, corner, owner, level int }{{0, 0, 0, 1}, {0, 3, 1, 2}, {1, 0, 0, 1}, {2, 0, 2, 2}, {3, 0, 2, 1}} {
		v := g.Tiles[b.tile].Vertices[b.corner]
		g.Vertices[v].Owner, g.Vertices[v].Level = b.owner, b.level
	}
	f, _ := newCatanExplorerSailing(4)
	c, err := newCatanExplorerCargo(g, f, scenario)
	if err != nil {
		t.Fatal(err)
	}
	e, err := newCatanExplorerEconomy(g, f, c)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.beginProduction(g, f, c, 0, 1); err != nil {
		t.Fatal(err)
	}
	return g, f, c, e
}

func explorerEconomySnapshot(t *testing.T, g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy) string {
	t.Helper()
	b, err := json.Marshal([]any{g, f, c, e})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func explorerEconomyReject(t *testing.T, g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy, action func() error) {
	t.Helper()
	before := explorerEconomySnapshot(t, g, f, c, e)
	if err := action(); err == nil {
		t.Fatal("invalid economy action accepted")
	}
	if explorerEconomySnapshot(t, g, f, c, e) != before {
		t.Fatal("rejected economy action partially mutated state")
	}
}
func explorerEconomyRestore(t *testing.T, g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy) {
	t.Helper()
	before := explorerEconomySnapshot(t, g, f, c, e)
	for _, value := range []any{g, f, c, e} {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.validate(g, f, c); err != nil {
		t.Fatal(err)
	}
	if explorerEconomySnapshot(t, g, f, c, e) != before {
		t.Fatal("economy restore differs")
	}
}

func TestCatanExplorerEconomyPrintedInitialGoldAndResources(t *testing.T) {
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			g, _, f, c, err := newCatanExplorerLandHo(n)
			if err != nil {
				t.Fatal(err)
			}
			e, err := newCatanExplorerEconomy(g, f, c)
			if err != nil {
				t.Fatal(err)
			}
			if e.GoldBank != 148-2*n || e.Turn != nil {
				t.Fatal("official coin supply or pre-roll phase")
			}
			for _, gold := range e.Gold {
				if gold != 2 {
					t.Fatal("each player starts with 2 gold")
				}
			}
			explorerEconomyRestore(t, g, f, c, e)
			if err = e.beginProduction(g, f, c, n-1, 1); err != nil {
				t.Fatal(err)
			}
			// Printed settlements/harbors earn resources once each. Neutral
			// obstacles are not players and must never receive money or cards.
			if _, err = e.resolveProduction(g, f, c, n-1, 1, [2]int{3, 3}); err != nil {
				t.Fatal(err)
			}
			if len(e.Gold) != n || c.Turn.Player != n-1 || c.Turn.Phase != "action" {
				t.Fatal("printed scenario economy/turn")
			}
			explorerEconomyRestore(t, g, f, c, e)
		})
	}
}

func TestCatanExplorerEconomyProductionHarborsGoldAndCompensation(t *testing.T) {
	g, f, c, e := explorerEconomyFixture(t, "pirate-lairs")
	payout, err := e.resolveProduction(g, f, c, 0, 1, [2]int{2, 4})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]int{{1, 0, 0, 1, 0}, {1, 0, 0, 0, 0}, {0, 0, 0, 0, 0}, {0, 0, 0, 0, 0}}
	if !reflect.DeepEqual(payout.Resources, want) || !slices.Equal(payout.Gold, []int{0, 0, 3, 1}) {
		t.Fatal("harbor produces once; gold-only income still earns compensation", payout)
	}
	for p := range g.Players {
		if !slices.Equal(g.Players[p].Resources, want[p]) || e.Gold[p] != 2+payout.Gold[p] {
			t.Fatal("payout differs from actual hands")
		}
	}
	if !slices.Equal(g.Bank, []int{17, 19, 19, 18, 19}) || e.GoldBank != 136 {
		t.Fatal("production must transfer from supply")
	}
	// Results cannot mutate saved bank, hands, gold or pending responses.
	payout.Resources[0][0] = 99
	payout.Gold[0] = 99
	payout.Discard[0] = 99
	explorerEconomyRestore(t, g, f, c, e)
	explorerEconomyReject(t, g, f, c, e, func() error { _, err := e.resolveProduction(g, f, c, 0, 1, [2]int{3, 3}); return err })
	view := e.publicView()
	view.Gold[0] = 100
	if e.Gold[0] != 2 {
		t.Fatal("public gold slice aliases private state")
	}
	b, _ := json.Marshal(e.publicView())
	var fields map[string]any
	if err = json.Unmarshal(b, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 5 || fields["goldRule"] != "ledger" || fields["goldIssued"] != float64(0) || fields["resources"] != nil || fields["discard"] != nil {
		t.Fatal("economic view exposes hands or response internals")
	}
}

func TestCatanExplorerEconomyScarcityIndependentEnumeration(t *testing.T) {
	// Each player claims 0, 1, or 2 wood. All 81 claim vectors are checked
	// against each stock 0..8, independently of the production implementation.
	for mask := 0; mask < 81; mask++ {
		claims := make([]int, 4)
		value, total, recipients := mask, 0, 0
		for p := range claims {
			claims[p] = value % 3
			value /= 3
			total += claims[p]
			if claims[p] > 0 {
				recipients++
			}
		}
		for stock := 0; stock <= 8; stock++ {
			g, f, c, e := explorerEconomyFixture(t, "land-ho")
			for i := range g.Vertices {
				g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
			}
			for p, n := range claims {
				g.Tiles[p].Resource, g.Tiles[p].Number = 0, 6
				for k := 0; k < n; k++ {
					v := g.Tiles[p].Vertices[k*3]
					g.Vertices[v].Owner, g.Vertices[v].Level = p, 1+k
				}
			}
			g.Bank[0], g.Players[0].Resources[0] = stock, 19-stock
			payout, err := e.resolveProduction(g, f, c, 0, 1, [2]int{3, 3})
			if err != nil {
				t.Fatal(mask, stock, err)
			}
			paid := 0
			for p, claim := range claims {
				want := claim
				if total > stock {
					want = 0
					if recipients == 1 && claim > 0 {
						want = stock
					}
				}
				bonus := 0
				if want == 0 {
					bonus = 1
				}
				if payout.Resources[p][0] != want || payout.Gold[p] != bonus {
					t.Fatalf("claims %v stock %d player %d: %+v want %d/%d", claims, stock, p, payout, want, bonus)
				}
				paid += want
			}
			if g.Bank[0] != stock-paid {
				t.Fatal("scarce resource bank mismatch")
			}
			if err = e.validate(g, f, c); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCatanExplorerEconomySevenParallelDiscardsAndNoGoldBonus(t *testing.T) {
	for _, scenario := range []string{"land-ho", "pirate-lairs"} {
		t.Run(scenario, func(t *testing.T) {
			g, f, c, e := explorerEconomyFixture(t, scenario)
			for p, n := range []int{8, 9, 7, 0} {
				g.Players[p].Resources[p] = n
				g.Bank[p] -= n
			}
			e.Gold[3] += 90
			e.GoldBank -= 90 // Gold is neither in the seven-card threshold nor discarded.
			beforeGold, bank := slices.Clone(e.Gold), e.GoldBank
			payout, err := e.resolveProduction(g, f, c, 0, 1, [2]int{1, 6})
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(payout.Discard, []int{4, 4, 0, 0}) || sum(payout.Gold) != 0 || e.Turn.Phase != "discard" || c.Turn != nil {
				t.Fatal("seven cutoff/round-down/pause", payout)
			}
			explorerEconomyRestore(t, g, f, c, e)
			explorerEconomyReject(t, g, f, c, e, func() error { return e.discard(g, f, c, 2, 1, []int{0, 0, 3, 0, 0}) })
			explorerEconomyReject(t, g, f, c, e, func() error { return e.discard(g, f, c, 1, 2, []int{0, 4, 0, 0, 0}) })
			explorerEconomyReject(t, g, f, c, e, func() error { return e.discard(g, f, c, 1, 1, []int{4, 0, 0, 0, 0}) })
			explorerEconomyReject(t, g, f, c, e, func() error { return e.bankTrade(g, f, c, 0, 1, 0, 1) })
			if err = e.discard(g, f, c, 1, 1, []int{0, 4, 0, 0, 0}); err != nil {
				t.Fatal("noncurrent player can respond first", err)
			}
			if e.Turn.Phase != "discard" {
				t.Fatal("must wait for every required player")
			}
			explorerEconomyRestore(t, g, f, c, e)
			explorerEconomyReject(t, g, f, c, e, func() error { return e.discard(g, f, c, 1, 1, []int{0, 4, 0, 0, 0}) })
			if err = e.discard(g, f, c, 0, 1, []int{4, 0, 0, 0, 0}); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(e.Gold, beforeGold) || e.GoldBank != bank || g.Players[0].Resources[0] != 4 || g.Players[1].Resources[1] != 5 || g.Bank[0] != 15 || g.Bank[1] != 14 {
				t.Fatal("seven payouts/discards")
			}
			if scenario == "land-ho" {
				if e.Turn.Phase != "ready" || c.Turn.Phase != "action" {
					t.Fatal("Land Ho has neither robber nor pirate")
				}
			} else {
				if e.Turn.Phase != "pirate" || c.Turn != nil {
					t.Fatal("must await pirate controller")
				}
				explorerEconomyReject(t, g, f, c, e, func() error { return e.beginProduction(g, f, c, 1, 2) })
			}
			explorerEconomyRestore(t, g, f, c, e)
		})
	}
}

func TestCatanExplorerEconomyBankTradesAndPerTurnLimits(t *testing.T) {
	g, f, c, e := explorerEconomyFixture(t, "land-ho")
	for r := 0; r < 5; r++ {
		g.Players[0].Resources[r] = 9
		g.Bank[r] -= 9
	}
	e.Gold[0] += 6
	e.GoldBank -= 6
	if _, err := e.resolveProduction(g, f, c, 0, 1, [2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
	if err := e.bankTrade(g, f, c, 0, 1, 0, 1); err != nil {
		t.Fatal(err)
	}
	if g.Players[0].Resources[0] != 6 || g.Players[0].Resources[1] != 10 || g.Bank[0] != 13 || g.Bank[1] != 9 {
		t.Fatal("resource trade must be exactly 3:1")
	}
	oldGold := e.Gold[0]
	if err := e.bankTrade(g, f, c, 0, 1, 0, -1); err != nil {
		t.Fatal(err)
	}
	if g.Players[0].Resources[0] != 3 || e.Gold[0] != oldGold+1 || e.Turn.Bought != 0 {
		t.Fatal("resource to gold isn't a gold purchase")
	}
	for _, resource := range []int{2, 4} {
		if err := e.bankTrade(g, f, c, 0, 1, -1, resource); err != nil {
			t.Fatal(err)
		}
	}
	if e.Gold[0] != oldGold-3 || g.Players[0].Resources[2] != 10 || g.Players[0].Resources[4] != 10 || e.Turn.Bought != 2 {
		t.Fatal("each gold purchase costs 2")
	}
	explorerEconomyRestore(t, g, f, c, e)
	for _, args := range [][2]int{{-1, 3}, {0, 0}, {-1, -1}, {-2, 0}, {0, 5}} {
		explorerEconomyReject(t, g, f, c, e, func() error { return e.bankTrade(g, f, c, 0, 1, args[0], args[1]) })
	}
	explorerEconomyReject(t, g, f, c, e, func() error { return e.bankTrade(g, f, c, 1, 1, 0, 1) })
	explorerEconomyReject(t, g, f, c, e, func() error { return e.bankTrade(g, f, c, 0, 2, 0, 1) })
	if err := e.bankTrade(g, f, c, 0, 1, 0, -1); err != nil {
		t.Fatal("selling resources still allowed after two purchases", err)
	}
	if err := c.beginMovement(g, f, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	explorerEconomyReject(t, g, f, c, e, func() error { return e.bankTrade(g, f, c, 0, 1, 1, 0) })
	if err := c.endMovement(g, f, 0, 1); err != nil {
		t.Fatal(err)
	}
	explorerEconomyRestore(t, g, f, c, e)
	if err := e.beginProduction(g, f, c, 0, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := e.resolveProduction(g, f, c, 0, 2, [2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
	if e.Turn.Bought != 0 {
		t.Fatal("new turn clears only the per-turn purchase limit")
	}
	if err := e.bankTrade(g, f, c, 0, 2, -1, 1); err != nil {
		t.Fatal(err)
	}
	explorerEconomyRestore(t, g, f, c, e)
}

func TestCatanExplorerEconomyGuardsAndLedgerShortage(t *testing.T) {
	g, f, c, e := explorerEconomyFixture(t, "land-ho")
	for _, dice := range [][2]int{{0, 6}, {7, 1}, {-1, 4}} {
		explorerEconomyReject(t, g, f, c, e, func() error { _, err := e.resolveProduction(g, f, c, 0, 1, dice); return err })
	}
	explorerEconomyReject(t, g, f, c, e, func() error { _, err := e.resolveProduction(g, f, c, 1, 1, [2]int{3, 3}); return err })
	explorerEconomyReject(t, g, f, c, e, func() error { return e.beginProduction(g, f, c, 0, 2) })
	e.Gold[3] += e.GoldBank
	e.GoldBank = 0
	if _, err := e.resolveProduction(g, f, c, 0, 1, [2]int{3, 3}); err != nil {
		t.Fatal(err)
	}
	if e.GoldIssued != 4 || e.GoldBank != 0 {
		t.Fatal("empty coin supply must issue exactly the missing rewards")
	}
	for name, damage := range map[string]func(*Catan, *catanExplorerEconomy){
		"gold value":          func(g *Catan, e *catanExplorerEconomy) { e.Gold[0]++ },
		"negative gold":       func(g *Catan, e *catanExplorerEconomy) { e.Gold[0] = -1; e.GoldBank += 3 },
		"resources":           func(g *Catan, e *catanExplorerEconomy) { g.Bank[0]++ },
		"resource kind count": func(g *Catan, e *catanExplorerEconomy) { g.Players[0].Resources = g.Players[0].Resources[:4] },
		"phase":               func(g *Catan, e *catanExplorerEconomy) { e.Turn.Phase = "ready" },
		"future discard":      func(g *Catan, e *catanExplorerEconomy) { e.Turn.Discard[0] = 2 },
		"already rolled":      func(g *Catan, e *catanExplorerEconomy) { e.Turn.Dice = [2]int{3, 3} },
		"buy count":           func(g *Catan, e *catanExplorerEconomy) { e.Turn.Bought = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			g, f, c, e := explorerEconomyFixture(t, "land-ho")
			damage(g, e)
			if err := e.validate(g, f, c); err == nil {
				t.Fatal("corrupt economic state accepted")
			}
			explorerEconomyReject(t, g, f, c, e, func() error { _, err := e.resolveProduction(g, f, c, 0, 1, [2]int{3, 3}); return err })
		})
	}
}

func TestCatanExplorerEconomyBankShortagesAndNoDiscardSeven(t *testing.T) {
	for _, kind := range []string{"resource", "gold", "payer", "coins"} {
		t.Run(kind, func(t *testing.T) {
			g, f, c, e := explorerEconomyFixture(t, "land-ho")
			// No one has eight cards; seven directly opens action without gold
			// compensation, pirate movement, or a vacuous discard response.
			if _, err := e.resolveProduction(g, f, c, 0, 1, [2]int{4, 3}); err != nil {
				t.Fatal(err)
			}
			if c.Turn == nil || c.Turn.Phase != "action" || e.Turn.Phase != "ready" || !slices.Equal(e.Gold, []int{2, 2, 2, 2}) {
				t.Fatal("empty seven flow")
			}
			g.Players[0].Resources[0], g.Bank[0] = 3, 16
			give, receive := 0, 1
			switch kind {
			case "resource":
				g.Players[1].Resources[1], g.Bank[1] = 19, 0
				explorerEconomyReject(t, g, f, c, e, func() error { return e.bankTrade(g, f, c, 0, 1, -1, 1) })
			case "gold":
				e.Gold[3] += e.GoldBank
				e.GoldBank = 0
				receive = -1
			case "payer":
				g.Players[0].Resources[0], g.Bank[0] = 2, 17
			case "coins":
				e.Gold[0]--
				e.GoldBank++
				give = -1
			}
			if kind == "gold" {
				if err := e.bankTrade(g, f, c, 0, 1, give, receive); err != nil {
					t.Fatal(err)
				}
				if e.GoldIssued != 1 || e.GoldBank != 0 || e.Gold[0] != 3 || g.Players[0].Resources[0] != 0 {
					t.Fatal("ledger bank exchange failed")
				}
			} else {
				explorerEconomyReject(t, g, f, c, e, func() error { return e.bankTrade(g, f, c, 0, 1, give, receive) })
			}
			explorerEconomyRestore(t, g, f, c, e)
		})
	}
}
