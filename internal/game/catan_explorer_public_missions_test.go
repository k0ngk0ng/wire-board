package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerPublicMissionRecipeAndIsolation(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		for _, scenario := range []string{"pirate-lairs", "fish-for-catan", "explorers-and-pirates"} {
			t.Run(fmt.Sprintf("%s/%d", scenario, n), func(t *testing.T) {
				s, err := NewCatanExplorerMission(n, scenario)
				if err != nil {
					t.Fatal(err)
				}
				want := []int{3, 4, 5, 9, 10, 11}
				if n > 4 {
					want = append(want, 6, 8)
				}
				l := s.Catan.Explorer.Lairs
				if l.NumberRecipe != CatanExplorerLairRecipe || !catanExplorerSameInventory(l.Inventory, want) || !slices.Contains(s.Log, catanExplorerLairRecipeNotice) {
					t.Fatal("wrong recipe")
				}
				restored := clone(*s)
				if !reflect.DeepEqual(*s, restored) || restored.validateCatanExplorer() != nil {
					t.Fatal("restore lost recipe")
				}
				// Explicit untagged historical inventories remain compatible; the
				// labelled new recipe cannot silently change its known multiset.
				restored.Catan.Explorer.Lairs.NumberRecipe = ""
				if restored.validateCatanExplorer() != nil {
					t.Fatal("legacy inventory rejected")
				}
				for _, mutate := range []func(*State){
					func(x *State) { x.Catan.Explorer.Lairs.NumberRecipe = "future-unknown" },
					func(x *State) {
						q := x.Catan.Explorer.Lairs
						q.Inventory[0] = 2
						for i, v := range q.Deck {
							if v == 3 {
								q.Deck[i] = 2
								break
							}
						}
					},
				} {
					trial := clone(*s)
					mutate(&trial)
					before := clone(trial)
					if trial.Apply(trial.Turn, Action{Type: "catan_explorer_setup"}) == nil || !reflect.DeepEqual(before, trial) {
						t.Fatal("invalid recipe accepted/mutated")
					}
				}
				view, err := json.Marshal(l.publicView(s.Catan.Explorer.Board))
				if err != nil {
					t.Fatal(err)
				}
				var public map[string]any
				if err = json.Unmarshal(view, &public); err != nil {
					t.Fatal(err)
				}
				if public["numberRecipe"] != CatanExplorerLairRecipe || public["deck"] != nil || public["inventory"] != nil {
					t.Fatal("private recipe assignment leaked")
				}
			})
		}
	}
	if _, err := NewCatanExplorerMission(3, "land-ho"); err == nil {
		t.Fatal("wrong constructor accepted initial scenario")
	}
	for _, n := range []int{2, 3, 4} {
		s, err := NewCatanExplorerLandHo(n)
		if err != nil || s.Catan.Explorer.Lairs != nil {
			t.Fatal("initial scenario changed", err)
		}
	}
	for _, n := range []int{2, 3, 4, 5, 6} {
		s, err := NewCatanExplorerSpices(n)
		if err != nil || s.Catan.Explorer.Lairs != nil {
			t.Fatal("spice scenario changed", err)
		}
	}
}
