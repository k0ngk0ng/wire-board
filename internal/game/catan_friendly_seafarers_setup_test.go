package game

import "testing"

func TestCatanFriendlySeaConfigurationRecipes(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, info := range CatanSeafarersScenarios(n) {

			for _, layout := range info.Layouts {
				var world *CatanNewWorldMap
				if info.ID == "new_world" {
					var err error
					world, err = GenerateCatanNewWorldMap(n)
					if err != nil {
						t.Fatal(err)
					}
				}
				s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: info.ID, Layout: layout}, world)
				if err != nil {
					t.Fatal(err)
				}
				target := s.Catan.victoryTarget()
				err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true})
				allowed := CatanFriendlySeafarersSupported(n, info.ID)
				if (err == nil) != allowed || (s.Catan.FriendlyRobber != nil) != allowed || s.Catan.victoryTarget() != target {
					t.Fatal(n, info.ID, layout, err)
				}
			}
		}
	}
}
