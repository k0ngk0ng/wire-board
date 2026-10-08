package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func twoVariantsGame(t *testing.T, scenario string, variants int, events bool) *State {
	t.Helper()
	makeGame := NewCatanTwo
	if scenario == "fishing" {
		makeGame = NewCatanTwoFishing
	}
	if scenario == "cities-knights" {
		makeGame = NewCatanTwoCitiesKnights
	}
	s, err := makeGame(2, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: variants&1 != 0}); err != nil {
		t.Fatal(err)
	}
	if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: variants&2 != 0}); err != nil {
		t.Fatal(err)
	}
	if events {
		if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestCatanTwoVariantsRecipesAndRestore(t *testing.T) {
	for _, scenario := range []string{"", "fishing", "cities-knights"} {
		for variants := range 4 {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/events=%v", scenario, variants, events), func(t *testing.T) {
					s := twoVariantsGame(t, scenario, variants, events)
					twoCoreRestore(t, s)
					for s.Catan.setup() {
						a, err := s.BotAction(s.Turn)
						if err != nil {
							t.Fatal(err)
						}
						helperApply(t, s, s.Turn, a)
					}
					twoCoreRestore(t, s)
					g := s.Catan
					want := 10
					if scenario == "cities-knights" {
						want = 13
					}
					if variants&2 != 0 {
						want++
					}
					if g.victoryTarget() != want || (g.Two.Variants != "") != (variants != 0) {
						t.Fatal("wrong target or supplement")
					}
					if variants&1 != 0 && ((g.FriendlyRobber.Knights != "") != (scenario == "cities-knights") || (g.FriendlyRobber.Fallback != "") != (scenario == "fishing")) {
						t.Fatal("missing companion recipe")
					}
					if g.harborGainValue(-2, 1) != 0 {
						t.Fatal("neutral gained harbor heuristic")
					}
				})
			}
		}
	}
	for _, build := range []func(int, CatanOptions) (*State, error){NewCatanTwoRivers, NewCatanTwoCaravans} {
		s, err := build(2, CatanOptions{})
		if err != nil {
			t.Fatal(err)
		}
		before, _ := json.Marshal(s)
		if s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}) == nil || s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}) == nil {
			t.Fatal("unverified crossing opened")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("rejected setup mutated game")
		}
	}
}

func TestCatanTwoVariantsNeutralAndProtectedSeven(t *testing.T) {
	s := twoVariantsGame(t, "", 3, false)
	for s.Catan.setup() {
		a, e := s.BotAction(s.Turn)
		if e != nil {
			t.Fatal(e)
		}
		helperApply(t, s, s.Turn, a)
	}
	g := s.Catan
	for p := range 2 {
		if !g.friendlyProtected(p) {
			t.Fatal("two-point player unprotected")
		}
	}
	for _, owner := range []int{-2, -3} {
		if g.friendlyProtected(owner) {
			t.Fatal("neutral protected")
		}
	}
	// Explicit neutral-only tile fixture; neutral buildings never protect land.
	fixture := clone(*s)
	for i := range fixture.Catan.Vertices {
		fixture.Catan.Vertices[i].Owner, fixture.Catan.Vertices[i].Level = -1, 0
	}
	v := fixture.Catan.Tiles[0].Vertices[0]
	fixture.Catan.Vertices[v].Owner, fixture.Catan.Vertices[v].Level = -2, 1
	if fixture.Catan.friendlyRobberBlocks(0) {
		t.Fatal("neutral blocks robber destination")
	}
	// Force the first seven through the actual two-production resolver.
	if err := twoCoreRoll(t, s, 3, 4); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_robber" {
		t.Fatal("first seven skipped robber", s.Phase)
	}
	for _, tile := range s.Catan.Tiles {
		if tile.Resource != CatanDesert && s.Catan.friendlyRobberBlocks(tile.ID) {
			helperReject(t, s, s.Turn, Action{Type: "catan_robber", Tile: tile.ID})
		}
	}
	twoCoreRestore(t, s)
	a, err := s.BotAction(s.Turn)
	if err != nil {
		t.Fatal(err)
	}
	helperApply(t, s, s.Turn, a)
	if s.Phase != "catan_roll" || len(s.Catan.Two.Rolls) != 1 {
		t.Fatal("did not resume second production")
	}
	if err = twoCoreRoll(t, s, 1, 1); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_turn" || len(s.Catan.Two.Rolls) != 2 {
		t.Fatal("did not finish production")
	}
	// A token retreat is its own action: a protected settlement at the desert
	// does not prevent it, and it never steals cards.
	g = s.Catan
	desert := g.twoDesert()
	g.Robber = (desert + 1) % len(g.Tiles)
	vertex := -1
	for _, id := range g.Tiles[desert].Vertices {
		if g.Vertices[id].Level == 0 {
			vertex = id
			break
		}
	}
	if vertex < 0 {
		t.Fatal("no empty desert site for protection fixture")
	}
	g.Vertices[vertex].Owner, g.Vertices[vertex].Level = 1-s.Turn, 1
	before := slices.Clone(g.Players[1-s.Turn].Resources)
	if !slices.Contains(g.twoRetreatTiles(), desert) {
		t.Fatal("protected desert blocked token retreat")
	}
	helperApply(t, s, s.Turn, Action{Type: "catan_two_robber", Tile: desert})
	if s.Catan.Robber != desert || !slices.Equal(s.Catan.Players[1-s.Turn].Resources, before) {
		t.Fatal("retreat stole cards")
	}
}

func TestCatanTwoVariantsHarborOwnershipAndValidation(t *testing.T) {
	s := twoVariantsGame(t, "", 3, false)
	g := s.Catan
	// Directed public-board scoring fixture, not a legal construction history.
	harborBuildings(g, []int{3, 2})
	for _, port := range g.Ports[6:] {
		v := g.Edges[port.Edge].A
		g.Vertices[v].Owner, g.Vertices[v].Level = -2, 1
	}
	s.catanScores()
	if g.Harbors.Owner != 0 || !slices.Equal(g.harborPoints(), []int{3, 2}) || g.Players[0].Score != 5 || g.friendlyProtected(0) || !g.friendlyProtected(1) {
		t.Fatal("wrong award or protection")
	}
	port := g.Edges[g.Ports[0].Edge].A
	g.Vertices[port].Level = 1
	s.catanScores()
	if g.Harbors.Owner != -1 || !g.friendlyProtected(0) {
		t.Fatal("lost city did not remove award/restore protection")
	}
	seed := twoVariantsGame(t, "", 3, false)
	for name, mutate := range map[string]func(*Catan){
		"marker":         func(g *Catan) { g.Two.Variants = "unknown" },
		"missing":        func(g *Catan) { g.Two.Variants = "" },
		"harbors":        func(g *Catan) { g.Harbors.Rules = "unknown" },
		"neutral-holder": func(g *Catan) { g.Harbors.Owner = -2 },
		"robber":         func(g *Catan) { g.FriendlyRobber.Rules = "unknown" },
		"fallback":       func(g *Catan) { g.FriendlyRobber.Fallback = "unknown" },
	} {
		t.Run(name, func(t *testing.T) {
			next := clone(*seed)
			mutate(next.Catan)
			if next.validateCatanTwo() == nil {
				t.Fatal("corrupt recipe accepted")
			}
		})
	}
	base := twoVariantsGame(t, "", 0, false)
	base.Catan.Two.Variants = CatanTwoVariantsRules
	if base.validateCatanTwo() == nil {
		t.Fatal("unused supplement accepted")
	}
}
