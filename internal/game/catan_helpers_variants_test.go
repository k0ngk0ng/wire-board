package game

import "testing"

func TestCatanHelpersVariantsConfigurationEveryMapAndCount(t *testing.T) {
	for n := 3; n <= 6; n++ {
		for _, scene := range []string{"", "shores", "islands", "fog", "desert", "tribe", "cloth", "pirate_islands", "wonders", "new_world"} {
			for _, all := range []bool{false, true} {
				options := CatanOptions{FiveSix: n > 4, Helpers: true, AllHelpers: all}
				var s *State
				var err error
				if scene == "" {
					s, err = NewCatanFriendlyRobber(n, options, CatanBaseConfiguration{})
				} else {
					s, err = NewCatanFriendlySeafarers(n, options, CatanSeafarersSetup{Scenario: scene})
				}
				if err != nil {
					t.Fatal(n, scene, all, err)
				}
				if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
					t.Fatal(err)
				}
				if s.Catan.FriendlyRobber == nil || s.Catan.Harbors == nil || !s.Catan.Options.Helpers || s.Catan.Options.AllHelpers != all {
					t.Fatal("combined configuration lost")
				}
			}
		}
	}
}

func TestCatanHelpersFriendlyProtectedDesertAndNoOutsideSubstitution(t *testing.T) {
	s := helperTestGame(t, 10)
	s.enableCatanFriendlyRobber()
	g := s.Catan
	desert := -1
	for _, tile := range g.Tiles {
		if tile.Resource == CatanDesert {
			desert = tile.ID
		} else if tile.Resource < 5 {
			g.Robber = tile.ID
		}
	}
	v := g.Tiles[desert].Vertices[0]
	g.Vertices[v].Owner, g.Vertices[v].Level = 1, 1
	g.Players[1].Score = 2
	helperGrant(s, 1, []int{1, 0, 0, 0, 0})
	color := g.Tiles[g.Robber].Resource
	helperApply(t, s, 0, Action{Type: "catan_helper", Choice: "desert", Tile: desert})
	if s.Catan.Robber != desert || s.Catan.Players[0].Resources[color] != 1 || s.Catan.Players[1].Resources[0] != 1 || s.Phase != "catan_helper" {
		t.Fatal("specific helper effect changed or stole")
	}
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
	if s.Phase != "catan_turn" {
		t.Fatal("did not resume")
	}
	// A sea retreat is not a substitute for Digur's missing printed desert.
	s = helperTestGame(t, 10)
	g = s.Catan
	g.Seafarers = &CatanSeafarers{Scenario: "shores"}
	s.enableCatanFriendlyRobber()
	for i := range g.Tiles {
		if g.Tiles[i].Resource == CatanDesert {
			g.Tiles[i].Resource = 0 // wood
		}
	}
	g.Robber = 0
	helperReject(t, s, 0, Action{Type: "catan_helper"})
	if _, ok := g.digurBotAction(0); ok {
		t.Fatal("bot invents retreat")
	}
	g.Robber = -1
	for _, id := range []int{10, 11} {
		g.Players[0].Helper.ID = id
		helperReject(t, s, 0, Action{Type: "catan_helper", Color: 0})
	}
}

func TestCatanHelpersHarborBuildRecalculatesAwardAndProtection(t *testing.T) {
	s := helperTestGame(t, 8)
	s.enableCatanHarbors()
	s.enableCatanFriendlyRobber()
	g := s.Catan
	a, b := g.Edges[g.Ports[0].Edge].A, g.Edges[g.Ports[1].Edge].A
	g.Vertices[a].Owner, g.Vertices[a].Level = 0, 1
	g.Vertices[b].Owner, g.Vertices[b].Level = 0, 1
	g.Players[0].Knights = 1
	g.DevDiscard = []int{0}
	s.catanScores()
	if !g.friendlyProtected(0) || g.Harbors.Owner != -1 {
		t.Fatal("initial public protection")
	}
	helperGrant(s, 0, []int{0, 0, 0, 1, 2})
	helperApply(t, s, 0, Action{Type: "catan_city", Vertex: a, Skill: "helper"})
	g = s.Catan
	if g.Harbors.Owner != 0 || g.harborPoints()[0] != 3 || g.Players[0].Score != 5 || g.friendlyProtected(0) {
		t.Fatal("helper building did not update public award/protection")
	}
	if g.Players[0].Knights != 0 || len(g.HelperExile) != 1 || g.victoryTarget() != 11 {
		t.Fatal("cost or target changed")
	}
}

func TestCatanHelpersFriendlyDoesNotProtectAgainstRyan(t *testing.T) {
	s := helperTestGame(t, 7)
	s.enableCatanFriendlyRobber()
	g := s.Catan
	g.Players[0].Score, g.Players[1].Score = 1, 2
	helperGrant(s, 1, []int{0, 0, 1, 0, 0})
	helperApply(t, s, 0, Action{Type: "catan_helper", Target: 1})
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Color: 2})
	if s.Catan.Players[0].Resources[2] != 1 || s.Catan.Players[1].Resources[2] != 0 {
		t.Fatal("friendly protection incorrectly blocked helper")
	}
}
