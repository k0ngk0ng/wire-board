package game

import (
	"fmt"
	"slices"
	"testing"
)

// Enumerate a village's component histories using the printed rules. Joins
// consume one local token, production pays each trader, and an empty village
// never draws again. Three immutable incident ship edges cap its traders at
// three. This explores join timing, including 1 join -> production -> 2 joins,
// which attains the two-token common-supply bound.
func TestCatanClothVillageCommonSupplyBound(t *testing.T) {
	type inventory struct{ local, traders, spent int }
	seen := map[inventory]bool{}
	queue := []inventory{{local: 5}}
	maxSpent := 0
	for len(queue) > 0 {
		x := queue[0]
		queue = queue[1:]
		if seen[x] {
			continue
		}
		seen[x] = true
		if x.spent > 2 || x.local > 0 && x.spent != 0 {
			t.Fatal("one village exceeded the bound", x)
		}
		maxSpent = max(maxSpent, x.spent)
		if x.traders < 3 {
			queue = append(queue, inventory{max(0, x.local-1), x.traders + 1, x.spent})
		}
		// Eliminations only lower the eligible number; they do not release a
		// ship or produce a new joining bonus. All possible active subsets fit.
		for active := 1; active <= x.traders && x.local > 0; active++ {
			queue = append(queue, inventory{max(0, x.local-active), x.traders, x.spent + max(0, active-x.local)})
		}
	}
	if maxSpent != 2 {
		t.Fatal("did not cover the worst ordering")
	}
	t.Logf("checked %d distinct village inventories", len(seen))
}

func TestCatanClothSmallMapProductionFitsRemainingSupply(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"fixed", "variable"} {
			for _, variant := range []string{"ordinary", "knights", "fishing"} {
				t.Run(fmt.Sprintf("%d/%s/%s", n, layout, variant), func(t *testing.T) {
					setup := CatanSeafarersSetup{Scenario: "cloth", Layout: layout}
					var s *State
					var err error
					switch variant {
					case "ordinary":
						s, err = NewCatanSeafarers(n, CatanOptions{}, setup, nil)
					case "knights":
						s, err = NewCatanCitiesKnightsSeafarers(n, CatanOptions{}, setup, nil)
					case "fishing":
						s, err = NewCatanFishingSeafarers(n, CatanOptions{}, setup, nil)
					}
					if err != nil {
						t.Fatal(err)
					}
					g, c := s.Catan, s.Catan.cloth()
					if c.Stock != 10 || c.EmptyLimit != 5 || len(c.Villages) != 8 {
						t.Fatal("small-board supply/ending assumptions changed")
					}
					numbers := map[int]bool{}
					for _, v := range c.Villages {
						if len(g.touching(v.Vertex)) != 3 || numbers[v.Number] || v.Stock != 5 {
							t.Fatal("village degree, distinct production numbers or local supply changed")
						}
						numbers[v.Number] = true
					}
					// A worst-case accounting envelope, not a claimed natural game:
					// four depleted villages each needed two common tokens. A fifth
					// can require at most two, and no other village shares its number.
					g.SetupStep, s.Phase = g.SetupLimit(), "catan_turn"
					c.Stock = 2
					for i := 0; i < 4; i++ {
						c.Villages[i].Stock = 0
					}
					c.Villages[4].Stock = 1
					c.Villages[4].Traders = []int{0, 1, 2}
					for i := 0; i < 32; i++ {
						c.Held[i%n]++
					}
					if clothTotal(g) != 50 || s.catanClothEnd() {
						t.Fatal("bad four-empty boundary")
					}
					if err := s.catanProduceCloth(c.Villages[4].Number); err != nil {
						t.Fatal("legal bound exceeds common supply", err)
					}
					if c.Stock != 0 || c.Villages[4].Stock != 0 || clothTotal(g) != 50 || !s.catanClothEnd() {
						t.Fatal("last possible draw/end transition")
					}
				})
			}
		}
	}
}

// The 5/6 board DOES have repeated village numbers, so the small-board proof
// cannot be generalized: one production may exhaust two villages at once.
func TestCatanClothExtendedMapHasSimultaneousVillageProduction(t *testing.T) {
	for _, n := range []int{5, 6} {
		s := ckClothFixture(t, n, "fixed")
		counts := map[int]int{}
		for _, v := range s.Catan.cloth().Villages {
			if len(s.Catan.touching(v.Vertex)) != 3 {
				t.Fatal("village degree changed")
			}
			counts[v.Number]++
		}
		for number := 2; number <= 12; number++ {
			want := 1
			if slices.Contains([]int{4, 5, 9, 10}, number) {
				want = 2
			} else if number == 3 || number == 7 || number == 11 {
				want = 0
			}
			if counts[number] != want {
				t.Fatal("printed village-number multiplicity changed", number, counts[number], want)
			}
		}
	}
}
