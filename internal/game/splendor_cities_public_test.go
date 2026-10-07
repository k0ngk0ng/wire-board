package game

import (
	"reflect"
	"slices"
	"testing"
)

func TestSplendorPublicCitiesSetupAndNormalization(t *testing.T) {
	catalog, err := readSplendorModernCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for n := 2; n <= 4; n++ {
		for mask := 0; mask < 8; mask++ {
			options := SplendorOptions{Cities: true, ExtraNobles: true, Orient: mask&1 != 0, TradingPosts: mask&2 != 0, Strongholds: mask&4 != 0}
			normalized, err := NormalizeSplendorOptions(options)
			if err != nil || normalized.ExtraNobles || normalized.Rules != SplendorExpansionRules {
				t.Fatal(normalized, err)
			}
			state, err := NewSplendor(n, options)
			if err != nil {
				t.Fatal(err)
			}
			g := state.Splendor
			if g.Options != normalized || g.Catalog != "2025-cities-bga-v1" || len(g.Nobles) != 0 || len(g.Cities) != 3 {
				t.Fatal("incorrect city setup", g)
			}
			seen := map[int]bool{}
			for _, city := range g.Cities {
				if seen[city.Tile] || !slices.Contains(catalog.Cities, city) {
					t.Fatal("duplicate tile or unknown face", city)
				}
				seen[city.Tile] = true
			}
			rows := 3
			if options.Orient {
				rows = 6
			}
			if len(g.Decks) != rows || len(g.Market) != rows || (g.Strongholds != nil) != options.Strongholds {
				t.Fatal("lost combination")
			}
			restored := clone(*state)
			for viewer := -1; viewer < n; viewer++ {
				if !reflect.DeepEqual(state.View(viewer), restored.View(viewer)) {
					t.Fatal("city setup changed after restore")
				}
			}
		}
	}
	if _, err := NewSplendor(3, SplendorOptions{Cities: true, Rules: "cities-2017"}); err == nil {
		t.Fatal("wrong edition accepted")
	}
}
