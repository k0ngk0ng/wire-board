package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanSeaNumberRecipeRestoreAndBaseIsolation(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, knights := range []bool{false, true} {
			for _, scenario := range []string{"shores", "islands"} {
				t.Run(fmt.Sprintf("%s/%d/knights=%t", scenario, n, knights), func(t *testing.T) {
					create := NewCatanSeafarers
					if knights {
						create = NewCatanCitiesKnightsSeafarers
					}
					s, err := create(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: scenario}, nil)
					if err != nil {
						t.Fatal(err)
					}
					expected := n > 4 && scenario == "shores"
					if (s.Catan.Seafarers.NumberRecipe == CatanExtendedNumberRecipe) != expected || slices.Contains(s.Log, catanSeaNumberNotice) != expected {
						t.Fatal("wrong map recipe or disclosure", s.Log)
					}
					restored := clone(*s)
					if !reflect.DeepEqual(*s, restored) || restored.validateCatanSeaNumberRecipe() != nil {
						t.Fatal("recipe restore")
					}
					// Existing saves already store their actual tile numbers and must not reroll.
					restored.Catan.Seafarers.NumberRecipe = ""
					before := clone(restored)
					if restored.validateCatanSeaNumberRecipe() != nil || !reflect.DeepEqual(before, restored) {
						t.Fatal("legacy save changed")
					}
					actor := ckActor(s)
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					s.Catan.Seafarers.NumberRecipe = "unknown"
					before = clone(*s)
					if s.Apply(actor, a) == nil || !reflect.DeepEqual(*s, before) {
						t.Fatal("invalid recipe was not atomic")
					}
					if !expected {
						s.Catan.Seafarers.NumberRecipe = CatanExtendedNumberRecipe
						if s.validateCatanSeaNumberRecipe() == nil {
							t.Fatal("tag accepted on other map")
						}
					}
					base, err := NewCatan(n, CatanOptions{FiveSix: n > 4})
					if err != nil || base.Catan.Seafarers != nil || slices.Contains(base.Log, catanSeaNumberNotice) {
						t.Fatal("base gained sea rules")
					}
				})
			}
		}
	}
}
