package game

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestSplendorModernCatalogPublisherExamplesAndValidation(t *testing.T) {
	c, err := readSplendorModernCatalog()
	if err != nil {
		t.Fatal(err)
	}
	// These rows can be read on the retained publisher photo sun-photo04.jpg.
	// Three additional rows (1014, 1019, 1023) are visible in the official
	// Sun Never Sets rulebook page 2 setup illustration, PDF object 75.
	// Five more (1011, 1012, 1021, 1026, 1029) are readable in Asmodee's
	// SCSPL04en-image3_2000.jpg and image4_2000.jpg product photographs.
	// Exact source hashes and visible fields are recorded in the research log.
	// This checks those examples only, not all 30 component faces.
	for _, want := range []Card{
		{ID: 1002, Tier: 1, Color: -1, Orient: GemOrientGold, Cost: []int{0, 0, 3, 0, 0}},
		{ID: 1003, Tier: 1, Color: -1, Orient: GemOrientGold, Cost: []int{3, 0, 0, 0, 0}},
		{ID: 1004, Tier: 1, Color: -1, Orient: GemOrientGold, Cost: []int{0, 0, 0, 0, 3}},
		{ID: 1008, Tier: 1, Color: -1, Orient: GemOrientCopy, Cost: []int{3, 0, 0, 2, 0}},
		{ID: 1009, Tier: 1, Color: -1, Orient: GemOrientCopy, Cost: []int{0, 0, 2, 0, 3}},
		{ID: 1010, Tier: 1, Color: -1, Orient: GemOrientCopy, Cost: []int{0, 2, 0, 3, 0}},
		{ID: 1011, Tier: 2, Points: 1, Color: 1, BonusCount: 2, Orient: GemOrientDouble, Cost: []int{3, 0, 0, 0, 4}},
		{ID: 1012, Tier: 2, Points: 1, Color: 2, BonusCount: 2, Orient: GemOrientDouble, Cost: []int{0, 0, 0, 4, 3}},
		{ID: 1014, Tier: 2, Points: 1, Color: 4, BonusCount: 2, Orient: GemOrientDouble, Cost: []int{0, 3, 4, 0, 0}},
		{ID: 1015, Tier: 2, Points: 1, Color: 3, BonusCount: 2, Orient: GemOrientDouble, Cost: []int{4, 0, 3, 0, 0}},
		{ID: 1017, Tier: 2, Points: 1, Color: -1, Orient: GemOrientCopyCascade, Cost: []int{0, 0, 4, 3, 1}},
		{ID: 1019, Tier: 2, Points: 1, Color: -1, Orient: GemOrientCopyCascade, Cost: []int{3, 1, 0, 0, 4}},
		{ID: 1021, Tier: 3, Points: 1, Color: 1, Orient: GemOrientCascade, Cost: []int{3, 0, 6, 0, 1}},
		{ID: 1022, Tier: 3, Points: 1, Color: 2, Orient: GemOrientCascade, Cost: []int{6, 0, 0, 1, 3}},
		{ID: 1023, Tier: 3, Points: 1, Color: 0, Orient: GemOrientCascade, Cost: []int{0, 1, 0, 3, 6}},
		{ID: 1026, Tier: 3, Points: 3, Color: 1, Orient: GemOrientSacrifice, SacrificeColor: 0, Cost: []int{0, 0, 0, 0, 0}},
		{ID: 1027, Tier: 3, Points: 3, Color: 2, Orient: GemOrientSacrifice, SacrificeColor: 4, Cost: []int{0, 0, 0, 0, 0}},
		{ID: 1028, Tier: 3, Points: 3, Color: 0, Orient: GemOrientSacrifice, SacrificeColor: 3, Cost: []int{0, 0, 0, 0, 0}},
		{ID: 1029, Tier: 3, Points: 3, Color: 4, Orient: GemOrientSacrifice, SacrificeColor: 1, Cost: []int{0, 0, 0, 0, 0}},
		{ID: 1030, Tier: 3, Points: 3, Color: 3, Orient: GemOrientSacrifice, SacrificeColor: 2, Cost: []int{0, 0, 0, 0, 0}},
	} {
		if got := c.Orient[want.ID-1001]; !reflect.DeepEqual(got, want) {
			t.Fatal("publisher example mismatch", want.ID, got, want)
		}
	}
	// The official Silk Road example explicitly says 14 prestige, 4 white,
	// and 4 of another color. It does not verify this tile's opposite face.
	if face := c.Cities[8]; face.Points != 14 || face.Cost != [5]int{0, 4, 0, 0, 0} || face.Any != 4 {
		t.Fatal("official city example mismatch")
	}
	// Publisher silk-photo09.jpg shows the entire Seoul face: 15 points,
	// five physical cards of one color, with no fixed-color requirements.
	if face := c.Cities[10]; face.Points != 15 || face.Cost != [5]int{} || face.Any != 5 {
		t.Fatal("publisher Seoul face mismatch")
	}
	for _, change := range []func(*splendorModernCatalog){
		func(c *splendorModernCatalog) { c.Rules = "cities-2017" },
		func(c *splendorModernCatalog) { c.Orient[0].ID = c.Orient[1].ID },
		func(c *splendorModernCatalog) { c.Orient[10].Points = 0 },
		func(c *splendorModernCatalog) { c.Orient[10].Color = -1 },
		func(c *splendorModernCatalog) { c.Orient[20].Tier = 2 },
		func(c *splendorModernCatalog) { c.Cities[1].Side = 0 },
		func(c *splendorModernCatalog) { c.Cities[1].Name = "不同的实体城市" },
	} {
		bad := clone(c)
		change(&bad)
		if bad.validate() == nil {
			t.Fatal("invalid catalog accepted")
		}
	}
}

func TestSplendorModernCatalogSetupPairsIndependentDecksAndPublicGate(t *testing.T) {
	faces := map[[2]int]bool{}
	for n := 2; n <= 4; n++ {
		for mask := 0; mask < 16; mask++ {
			o := SplendorOptions{TradingPosts: mask&1 != 0, Strongholds: mask&2 != 0, Cities: mask&4 != 0, Orient: mask&8 != 0}
			s, e := newSplendorModernCatalogState(n, o)
			if e != nil {
				t.Fatal(e)
			}
			g := s.Splendor
			rows := 3
			if o.Orient {
				rows = 6
			}
			if len(g.Decks) != rows || len(g.Market) != rows {
				t.Fatal("wrong independent deck count")
			}
			for row := range rows {
				market, want := 4, []int{40, 30, 20}[row%3]
				if row >= 3 {
					market, want = 2, 10
				}
				if len(g.Market[row]) != market || len(g.Market[row])+len(g.Decks[row]) != want {
					t.Fatal("wrong card distribution", row)
				}
				for _, card := range append(slices.Clone(g.Market[row]), g.Decks[row]...) {
					if card.gemDeck() != row {
						t.Fatal("mixed card tiers")
					}
				}
			}
			if o.Cities {
				if len(g.Nobles) != 0 || len(g.Cities) != 3 {
					t.Fatal("cities must replace nobles")
				}
				tiles := map[int]bool{}
				for _, city := range g.Cities {
					if tiles[city.Tile] {
						t.Fatal("both faces of same tile were selected")
					}
					tiles[city.Tile] = true
					faces[[2]int{city.Tile, city.Side}] = true
				}
			} else if len(g.Nobles) != n+1 || len(g.Cities) != 0 {
				t.Fatal("ordinary noble setup changed")
			}
			if o.Cities || o.Orient {
				if _, e := NewSplendor(n, o); e == nil {
					t.Fatal("unverified catalog leaked into public constructor")
				}
				if g.Catalog != "2025-secondary-v1" {
					t.Fatal("missing provenance")
				}
			}
			restored := clone(*s)
			for p := -1; p < n; p++ {
				if !reflect.DeepEqual(s.View(p), restored.View(p)) {
					t.Fatal("setup view restore")
				}
			}
		}
	}
	// Additional samples for all physical sides, independently of configuration.
	for i := 0; i < 100 && len(faces) < 14; i++ {
		s, e := newSplendorModernCatalogState(3, SplendorOptions{Cities: true})
		if e != nil {
			t.Fatal(e)
		}
		for _, city := range s.Splendor.Cities {
			faces[[2]int{city.Tile, city.Side}] = true
		}
	}
	if len(faces) != 14 {
		t.Fatal("a city face was never selected", faces)
	}
}

func assertSplendorCatalogInventory(t *testing.T, s *State, initial []int, want int) {
	t.Helper()
	g := s.Splendor
	counts := map[int]int{}
	add := func(cards []Card) {
		for _, c := range cards {
			if c.ID > 0 {
				counts[c.ID]++
			}
		}
	}
	for _, row := range g.Market {
		add(row)
	}
	for _, deck := range g.Decks {
		add(deck)
	}
	add(g.Exiled)
	add(g.ReserveChoice)
	tokens := slices.Clone(g.Bank)
	for _, p := range g.Players {
		add(p.Cards)
		add(p.Reserved)
		bonus := make([]int, 5)
		score := 3 * len(p.Nobles)
		for _, card := range p.Cards {
			score += card.Points
			if card.gemBonus() > 0 {
				bonus[card.Color] += card.gemBonus()
			}
		}
		if p.hasPost(GemPostPrestige) {
			score += len(p.TradingPosts)
		}
		if !slices.Equal(bonus, p.Bonus) || score != p.Score {
			t.Fatal("card bonuses or score drift")
		}
		for i, n := range p.Tokens {
			if n < 0 {
				t.Fatal("negative player tokens")
			}
			tokens[i] += n
		}
	}
	if len(counts) != want || !slices.Equal(tokens, initial) {
		t.Fatal("component conservation", len(counts), tokens)
	}
	for id, count := range counts {
		if count != 1 {
			t.Fatal("duplicate card", id)
		}
	}
	for _, n := range g.Bank {
		if n < 0 {
			t.Fatal("negative bank")
		}
	}
}

func TestSplendorModernCatalogCombinationGames(t *testing.T) {
	for mask := 4; mask < 16; mask++ {
		for n := 2; n <= 4; n++ {
			t.Run(fmt.Sprintf("mask=%d/n=%d", mask, n), func(t *testing.T) {
				s, e := newSplendorModernCatalogState(n, SplendorOptions{TradingPosts: mask&1 != 0, Strongholds: mask&2 != 0, Cities: mask&4 != 0, Orient: mask&8 != 0})
				if e != nil {
					t.Fatal(e)
				}
				initial := slices.Clone(s.Splendor.Bank)
				want := 90
				if s.Splendor.Options.Orient {
					want = 120
				}
				steps := 0
				for ; !s.Finished && steps < 2500; steps++ {
					a, e := s.BotAction(s.Turn)
					if e != nil {
						t.Fatal(steps, s.Phase, e)
					}
					if e = s.Apply(s.Turn, a); e != nil {
						t.Fatal(steps, s.Phase, a, e)
					}
					assertSplendorCatalogInventory(t, s, initial, want)
					if steps%19 == 0 {
						restored := clone(*s)
						s = &restored
						assertSplendorCatalogInventory(t, s, initial, want)
					}
				}
				if !s.Finished || len(s.Winners) == 0 {
					t.Fatal("catalog game did not finish", s.Phase)
				}
				if s.Splendor.Options.Cities {
					for _, p := range s.Winners {
						if len(s.Splendor.gemCitiesFor(s.Splendor.Players[p])) == 0 {
							t.Fatal("winner lacks a city condition")
						}
					}
				}
				t.Logf("%d actions, %d cards conserved, winners %v", steps, want, s.Winners)
			})
		}
	}
}

// Persisted HTTP fixtures must remain genuine untouched starts with the exact
// candidate component facts. Their shuffled order is intentionally fixed.
func TestSplendorModernHTTPFixtureIntegrity(t *testing.T) {
	catalog, err := readSplendorModernCatalog()
	if err != nil {
		t.Fatal(err)
	}
	known := map[int]Card{}
	for _, c := range append(Cards(), catalog.Orient...) {
		known[c.ID] = c
	}
	for n := 2; n <= 4; n++ {
		raw, err := os.ReadFile(fmt.Sprintf("../server/testdata/splendor_modern_%d.json", n))
		if err != nil {
			t.Fatal(err)
		}
		var s State
		if err = json.Unmarshal(raw, &s); err != nil {
			t.Fatal(err)
		}
		g := s.Splendor
		if s.Finished || s.Phase != "turn" || s.Round != 1 || len(s.Log) != 0 || len(s.Winners) != 0 || g.StartPlayer != s.Turn || s.Turn < 0 || s.Turn >= n || len(g.Players) != n || len(g.Effects) != 0 || len(g.Refills) != 0 || len(g.Exiled) != 0 || len(g.ReserveChoice) != 0 || len(g.Strongholds) != 0 || g.LastRound {
			t.Fatal("fixture is not a normal initial state", n)
		}
		if g.Options != (SplendorOptions{Rules: SplendorExpansionRules, TradingPosts: true, Strongholds: true, Cities: true, Orient: true}) || g.Catalog != "2025-secondary-v1" {
			t.Fatal("fixture rules")
		}
		bank := []int{n + 2, n + 2, n + 2, n + 2, n + 2, 5}
		if n == 4 {
			bank = []int{7, 7, 7, 7, 7, 5}
		}
		if !slices.Equal(g.Bank, bank) {
			t.Fatal("fixture bank", n, g.Bank)
		}
		for _, p := range g.Players {
			if p.Score != 0 || p.Eliminated || len(p.Cards) != 0 || len(p.Reserved) != 0 || len(p.Nobles) != 0 || len(p.TradingPosts) != 0 || sum(p.Tokens) != 0 || sum(p.Bonus) != 0 {
				t.Fatal("fixture grants player progress")
			}
		}
		if len(g.Market) != 6 || len(g.Decks) != 6 || len(g.Cities) != 3 || len(g.Nobles) != 0 {
			t.Fatal("fixture market/cities")
		}
		for row, market := range g.Market {
			count := 4
			want := []int{40, 30, 20}[row%3]
			if row >= 3 {
				count = 2
				want = 10
			}
			if len(market) != count || len(market)+len(g.Decks[row]) != want {
				t.Fatal("fixture deck size")
			}
			for _, c := range append(slices.Clone(market), g.Decks[row]...) {
				if c.gemDeck() != row || !reflect.DeepEqual(c, known[c.ID]) {
					t.Fatal("fixture invented/altered card", c)
				}
			}
		}
		tiles := map[int]bool{}
		for _, city := range g.Cities {
			if tiles[city.Tile] || !slices.Contains(catalog.Cities, city) {
				t.Fatal("fixture city")
			}
			tiles[city.Tile] = true
		}
		assertSplendorCatalogInventory(t, &s, bank, 120)
	}
}

func TestSplendorModernCityPhysicalUnboxingExamples(t *testing.T) {
	catalog, err := readSplendorModernCatalog()
	if err != nil {
		t.Fatal(err)
	}
	// These seven faces' conditions are readable in The Brosey Game Company's
	// physical 2025 Silk Road unboxing, not copied from the secondary data file.
	// Video ID, confirmed frame times and retained crops are recorded in
	// docs/research/splendor-city-video-evidence.json. Local tile/side numbers
	// identify candidate rows only; this does not verify their reverse pairing.
	for _, want := range []GemCity{
		{Tile: 5, Side: 1, Name: "撒马尔罕", Points: 14, Cost: [5]int{0, 0, 0, 4, 0}, Any: 4},
		{Tile: 4, Side: 1, Name: "德里", Points: 14, Cost: [5]int{2, 2, 2, 2, 2}},
		{Tile: 6, Side: 1, Name: "首尔", Points: 13, Cost: [5]int{}, Any: 6},
		{Tile: 3, Side: 0, Name: "廷巴克图", Points: 13, Cost: [5]int{0, 3, 4, 0, 0}},
		{Tile: 7, Side: 1, Name: "克拉科夫", Points: 17, Cost: [5]int{}},
		{Tile: 1, Side: 0, Name: "马德里", Points: 12, Cost: [5]int{3, 3, 0, 3, 3}},
		// Illustration partly covered by Amboise, but all printed conditions
		// on the underlying Madrid tile are visible. No reverse pairing claim.
		{Tile: 1, Side: 1, Name: "马德里", Points: 12, Cost: [5]int{3, 0, 3, 3, 3}},
	} {
		got := catalog.Cities[(want.Tile-1)*2+want.Side]
		if got != want {
			t.Fatal("physical city face mismatch", got, want)
		}
	}
}

func TestSplendorModernOrientPhysicalVideoExamples(t *testing.T) {
	catalog, err := readSplendorModernCatalog()
	if err != nil {
		t.Fatal(err)
	}
	// Independently read from The Brosey Game Company's physical 2025
	// Sun Never Sets unboxing and Game4LifeBG's physical rules demonstration.
	// The sources, presented frame times and retained crops
	// are recorded in docs/research/splendor-orient-video-evidence.json.
	// Obscured cards in the fan are deliberately excluded.
	for _, want := range []Card{
		{ID: 1006, Tier: 1, Color: -1, Orient: GemOrientCopy, Cost: []int{0, 3, 0, 0, 2}},
		{ID: 1007, Tier: 1, Color: -1, Orient: GemOrientCopy, Cost: []int{2, 0, 3, 0, 0}},
		{ID: 1018, Tier: 2, Points: 1, Color: -1, Orient: GemOrientCopyCascade, Cost: []int{4, 3, 1, 0, 0}},
		{ID: 1024, Tier: 3, Points: 1, Color: 4, Orient: GemOrientCascade, Cost: []int{0, 3, 1, 6, 0}},
	} {
		if got := catalog.Orient[want.ID-1001]; !reflect.DeepEqual(got, want) {
			t.Fatal("physical Orient card mismatch", want.ID, got, want)
		}
	}
}

// These three additional faces are independently readable in the 2025 Never
// Bored Gaming review, at the exact frames recorded in the city evidence file.
// This verifies the face facts, not physical reverse-side pairing.
func TestSplendorModernCityPhysicalReviewExamples(t *testing.T) {
	catalog, err := readSplendorModernCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []GemCity{
		{Tile: 2, Side: 0, Name: "昂布瓦兹", Points: 13, Cost: [5]int{3, 0, 0, 4, 0}},
		{Tile: 4, Side: 0, Name: "德里", Points: 13, Cost: [5]int{4, 0, 3, 0, 0}},
		{Tile: 7, Side: 0, Name: "克拉科夫", Points: 16, Cost: [5]int{1, 1, 1, 1, 1}},
	} {
		if got := catalog.Cities[(want.Tile-1)*2+want.Side]; got != want {
			t.Fatal("physical review city face mismatch", got, want)
		}
	}
}
