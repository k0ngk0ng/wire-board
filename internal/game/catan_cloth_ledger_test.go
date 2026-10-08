package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Inventory boundary fixture, not a claim that every bank/exit combination
// occurs naturally. The separate replay proves the 5/6-player shortage legally.
func clothLedgerFixture(t *testing.T, stock int) *State {
	t.Helper()
	s, err := NewCatanSeafarers(6, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "cloth"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g, c := s.Catan, s.Catan.cloth()
	g.SetupStep, s.Phase = g.SetupLimit(), "catan_turn"
	c.Stock = stock
	for i := range c.Villages {
		if c.Villages[i].Number == 4 {
			c.Villages[i].Stock = 1
			c.Villages[i].Traders = []int{0, 1, 2}
		}
	}
	c.Held[5] = 70 - clothTotal(g)
	if err := g.validateClothSupply(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCatanClothLedgerExactDeficitAndRestore(t *testing.T) {
	for stock := 0; stock <= 5; stock++ {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("bank%d/reverse%t", stock, reverse), func(t *testing.T) {
				s := clothLedgerFixture(t, stock)
				c := s.Catan.cloth()
				if reverse {
					slices.Reverse(c.Villages)
				}
				before := slices.Clone(c.Held)
				if err := s.catanProduceCloth(4); err != nil {
					t.Fatal(err)
				}
				if c.Issued != max(0, 4-stock) || c.Stock != max(0, stock-4) || clothTotal(s.Catan) != 70+c.Issued {
					t.Fatal("wrong deficit", c)
				}
				for p := 0; p < 6; p++ {
					want := before[p]
					if p < 3 {
						want += 2
					}
					if c.Held[p] != want {
						t.Fatal("wrong payout", p, c.Held)
					}
				}
				if err := s.Catan.validateClothSupply(); err != nil {
					t.Fatal(err)
				}
				saved := clone(*s)
				if !reflect.DeepEqual(*s, saved) {
					t.Fatal("ledger not restored")
				}
				if err := saved.catanProduceCloth(4); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(c, saved.Catan.cloth()) {
					t.Fatal("empty villages produced or issued again")
				}
				view := saved.View(-1)["catan"].(map[string]any)["seafarers"].(map[string]any)["cloth"].(map[string]any)
				if c.Issued > 0 && view["issued"] != float64(c.Issued) {
					t.Fatal("missing public issuance")
				}
			})
		}
	}
}

func TestCatanClothLedgerDeduplicatesAndSkipsEliminated(t *testing.T) {
	s := clothLedgerFixture(t, 0)
	c := s.Catan.cloth()
	s.Catan.Players[2].Eliminated = true
	for i := range c.Villages {
		if c.Villages[i].Number == 4 {
			c.Villages[i].Traders = []int{0, 0, 1, 2}
		}
	}
	before := slices.Clone(c.Held)
	if err := s.catanProduceCloth(4); err != nil {
		t.Fatal(err)
	}
	if c.Issued != 2 || c.Held[0] != before[0]+2 || c.Held[1] != before[1]+2 || c.Held[2] != before[2] {
		t.Fatal("duplicate or eliminated trader paid", c)
	}
	if err := s.Catan.validateClothSupply(); err != nil {
		t.Fatal(err)
	}
}

func TestCatanClothLedgerRejectsCorruptSaveAtomically(t *testing.T) {
	for name, corrupt := range map[string]func(*CatanClothState){
		"negative issued":  func(c *CatanClothState) { c.Issued = -1 },
		"unbacked issue":   func(c *CatanClothState) { c.Issued = 1 },
		"overflow":         func(c *CatanClothState) { c.Issued = int(^uint(0) >> 1) },
		"missing cloth":    func(c *CatanClothState) { c.Held[5]-- },
		"negative bank":    func(c *CatanClothState) { c.Stock = -1 },
		"negative village": func(c *CatanClothState) { c.Villages[0].Stock = -1 },
		"invalid trader":   func(c *CatanClothState) { c.Villages[0].Traders = []int{6} },
		"missing seat":     func(c *CatanClothState) { c.Held = c.Held[:5] },
	} {
		t.Run(name, func(t *testing.T) {
			s := clothLedgerFixture(t, 2)
			corrupt(s.Catan.cloth())
			s.Phase = "catan_roll"
			before, _ := json.Marshal(s)
			if err := s.Apply(s.Turn, Action{Type: "catan_roll"}); err == nil {
				t.Fatal("invalid save accepted")
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("rejected action mutated save")
			}
		})
	}
	// The omitted JSON field in old physical-inventory saves defaults to zero.
	s := clothLedgerFixture(t, 2)
	restored := clone(*s)
	if restored.Catan.cloth().Issued != 0 || restored.Catan.validateClothSupply() != nil {
		t.Fatal("old inventory rejected")
	}
}

func TestCatanClothLedgerOpeningRuleForCombinations(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, kind := range []string{"ordinary", "helpers", "knights", "fishing"} {
			if kind == "fishing" && n > 4 {
				continue
			}
			var s *State
			var err error
			options := CatanOptions{FiveSix: n > 4, Helpers: kind == "helpers"}
			setup := CatanSeafarersSetup{Scenario: "cloth"}
			switch kind {
			case "knights":
				s, err = NewCatanCitiesKnightsSeafarers(n, options, setup, nil)
			case "fishing":
				s, err = NewCatanFishingSeafarers(n, options, setup, nil)
			default:
				s, err = NewCatanSeafarers(n, options, setup, nil)
			}
			if err != nil {
				t.Fatal(n, kind, err)
			}
			if !slices.Contains(s.Log, catanClothSupplyRule) {
				t.Fatal("missing house rule", n, kind, s.Log)
			}
		}
	}
}
