package game

import (
	"reflect"
	"slices"
	"strings"
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
			if g.Options != normalized || g.Catalog != SplendorCityCatalogue || len(g.Nobles) != 0 || len(g.Cities) != 3 {
				t.Fatal("incorrect city setup", g)
			}
			if !strings.Contains(strings.Join(state.Log, "\n"), "本站城市分组") {
				t.Fatal("missing city grouping disclosure")
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

func TestSplendorCityCatalogueLegacyContinuationAndBaseIsolation(t *testing.T) {
	for _, legacy := range []string{"", "2025-secondary-v1", "2025-cities-bga-v1"} {
		s, err := NewSplendor(2, SplendorOptions{Cities: true})
		if err != nil {
			t.Fatal(err)
		}
		// Explicit old-save metadata fixture; component conditions are unchanged.
		s.Splendor.Catalog = legacy
		s.Log = nil
		before := slices.Clone(s.Splendor.Cities)
		restored := clone(*s)
		a, err := restored.BotAction(restored.Turn)
		if err != nil {
			t.Fatal(err)
		}
		if err = restored.Apply(restored.Turn, a); err != nil {
			t.Fatal(err)
		}
		if restored.Splendor.Catalog != legacy || !slices.Equal(restored.Splendor.Cities, before) || strings.Contains(strings.Join(restored.Log, "\n"), "本站城市分组") {
			t.Fatal("legacy save was relabelled or regrouped")
		}
	}
	for _, options := range []SplendorOptions{{}, {Orient: true}, {TradingPosts: true}, {Strongholds: true}} {
		s, err := NewSplendor(2, options)
		if err != nil || s.Splendor.Catalog == SplendorCityCatalogue || len(s.Splendor.Cities) != 0 || strings.Contains(strings.Join(s.Log, "\n"), "本站城市分组") {
			t.Fatal("city recipe leaked into non-city game", err)
		}
	}
}
