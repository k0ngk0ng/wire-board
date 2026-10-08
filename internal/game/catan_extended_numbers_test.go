package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExtendedNumberRecipeRestoreAndIsolation(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, scene := range []string{"rivers", "caravans"} {
			t.Run(fmt.Sprintf("%s/%d", scene, n), func(t *testing.T) {
				build := NewCatanRivers
				if scene == "caravans" {
					build = NewCatanCaravans
				}
				s, err := build(n, CatanOptions{FiveSix: n > 4})
				if err != nil {
					t.Fatal(err)
				}
				tag := func(s *State) *string {
					if s.Catan.Rivers != nil {
						return &s.Catan.Rivers.Map.NumberRecipe
					}
					return &s.Catan.Caravans.Map.NumberRecipe
				}
				if (*tag(s) == CatanExtendedNumberRecipe) != (n > 4) || slices.Contains(s.Log, catanExtendedNumberNotice) != (n > 4) {
					t.Fatal("wrong recipe tag or notice", s.Log)
				}
				restored := clone(*s)
				if !reflect.DeepEqual(*s, restored) {
					t.Fatal("recipe not restored")
				}
				// Old saves have the same board sequence without a recipe field.
				*tag(&restored) = ""
				if restored.Catan.validateRivers() != nil || restored.validateCaravans() != nil {
					t.Fatal("legacy board rejected")
				}
				if n > 4 {
					*tag(s) = "invented-version"
				} else {
					*tag(s) = CatanExtendedNumberRecipe
				}
				before := clone(*s)
				a, err := s.BotAction(s.Turn)
				if err != nil {
					t.Fatal(err)
				}
				if s.Apply(s.Turn, a) == nil || !reflect.DeepEqual(before, *s) {
					t.Fatal("wrong recipe accepted or changed state")
				}
			})
		}
	}
}
