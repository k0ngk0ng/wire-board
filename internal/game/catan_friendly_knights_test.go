package game

import (
	"slices"
	"testing"
)

func TestCatanFriendlyKnightsAllConfigurations(t *testing.T) {
	for n := 3; n <= 6; n++ {
		for _, scene := range []string{"", "shores", "islands", "fog", "desert", "new_world", "wonders", "cloth"} {
			options := CatanOptions{FiveSix: n > 4}
			var s *State
			var err error
			if scene == "" {
				s, err = NewCatanCitiesKnights(n, options)
			} else {
				var world *CatanNewWorldMap
				if scene == "new_world" {
					world, err = GenerateCatanNewWorldMap(n)
					if err != nil {
						t.Fatal(err)
					}
				}
				s, err = NewCatanCitiesKnightsSeafarers(n, options, CatanSeafarersSetup{Scenario: scene}, world)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
				t.Fatal(n, scene, err)
			}
			if s.Catan.FriendlyRobber.Knights != CatanFriendlyKnightsRules {
				t.Fatal("missing source")
			}
			if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
				t.Fatal(err)
			}
			if len(s.Catan.taxationTiles()) != 0 {
				t.Fatal("taxation before first attack")
			}
		}
	}
}

func friendlyKnightsTaxationFixture(t *testing.T) *State {
	s := ckEmptyTurn(t)
	g := s.Catan
	g.Seafarers = &CatanSeafarers{Scenario: "shores", Pirate: -1}
	s.enableCatanFriendlyRobber()
	g.CitiesKnights.Invasions = 1
	g.Tiles = []CatanTile{
		{ID: 0, Resource: 0, Number: 5, Vertices: []int{0}},
		{ID: 1, Resource: CatanDesert, Vertices: []int{1, 2, 3}},
	}
	g.Vertices = []CatanVertex{{ID: 0, Owner: 1, Level: 1}, {ID: 1, Owner: 1, Level: 1}, {ID: 2, Owner: 2, Level: 2}, {ID: 3, Owner: 2, Level: 1}}
	g.Edges = nil
	g.Ports = nil
	g.Robber = 0
	s.catanScores()
	ckProgressGive(t, s, 0, 21)
	helperGrant(s, 1, []int{0, 0, 0, 0, 0, 2, 0, 0})
	helperGrant(s, 2, []int{0, 0, 0, 0, 0, 0, 2, 0})
	return s
}

func TestCatanFriendlyKnightsTaxationProtectedDesertAndOutside(t *testing.T) {
	for _, outside := range []bool{false, true} {
		s := friendlyKnightsTaxationFixture(t)
		if outside {
			s.Catan.Tiles[1].Resource = 1
		}
		want := 1
		if outside {
			want = -1
		}
		if !slices.Equal(s.Catan.taxationTiles(), []int{want}) {
			t.Fatal("wrong retreat hints", s.Catan.taxationTiles())
		}
		for _, viewer := range []int{-1, 0, 1} {
			v := s.View(viewer)["catan"].(map[string]any)
			_, shown := v["taxationTiles"]
			if shown != (viewer == 0) {
				t.Fatal("target hints exposed to another actor")
			}
		}
		restored := clone(*s)
		s = &restored
		helperApply(t, s, 0, Action{Type: "catan_progress", Card: 21, Tile: want})
		g := s.Catan
		if g.Robber != want || s.Phase != "catan_turn" || len(g.CitiesKnights.Players[0].Progress) != 0 {
			t.Fatal("card did not complete")
		}
		stolen := 1
		if outside {
			stolen = 0
		}
		if g.Players[1].Resources[5] != 2 || g.Players[2].Resources[6] != 2-stolen || g.Players[0].Resources[6] != stolen {
			t.Fatal("protected or duplicate theft")
		}
		ckSupply(t, g)
		ckProgressStock(t, g)
	}
	s := friendlyKnightsTaxationFixture(t)
	s.Catan.CitiesKnights.Invasions = 0
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 21, Tile: -1})
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 21, Tile: 1})
}

func TestCatanFriendlyKnightsCityLossRestoresProtection(t *testing.T) {
	s := ckEmptyTurn(t)
	g := s.Catan
	k := g.CitiesKnights
	s.enableCatanFriendlyRobber()
	g.Tiles = []CatanTile{{ID: 0, Resource: 0, Number: 6, Vertices: []int{0}}, {ID: 1, Resource: 1, Number: 5, Vertices: []int{1}}, {ID: 2, Resource: CatanDesert}}
	g.Vertices = []CatanVertex{{ID: 0, Owner: 1, Level: 2}, {ID: 1, Owner: 1, Level: 1}}
	g.Edges = nil
	g.Ports = nil
	k.RobberStart = 2
	k.BarbarianPosition = 6
	s.catanScores()
	if g.friendlyProtected(1) {
		t.Fatal("city owner starts protected")
	}
	s.Phase = "catan_roll"
	if err := s.catanCityRoll(1, 5, 3); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_pillage" {
		t.Fatal("missing city loss")
	}
	helperApply(t, s, 1, Action{Type: "catan_pillage", Vertex: 0})
	g = s.Catan
	if !g.friendlyProtected(1) || g.robberAllowed(0) || g.Players[1].Score != 2 || g.CitiesKnights.Invasions != 1 {
		t.Fatal("city loss did not update protection")
	}
	ckSupply(t, g)
}

func TestCatanFriendlyKnightsChaseTypeAndOutsideRestore(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0})
	g := s.Catan
	g.Seafarers = &CatanSeafarers{Scenario: "shores", Pirate: -1}
	s.enableCatanFriendlyRobber()
	g.Tiles = append(g.Tiles, CatanTile{ID: 1, Resource: 2, Number: 8, Vertices: []int{2, 3}})
	g.Vertices[2].Owner, g.Vertices[2].Level = 1, 1
	g.Players[1].Score = 2
	g.Robber = 0
	g.CitiesKnights.Invasions = 1
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 1, Active: true}}
	helperApply(t, s, 0, Action{Type: "catan_knight_chase", Vertex: 0})
	helperReject(t, s, 0, Action{Type: "catan_pirate", Tile: -1})
	helperReject(t, s, 0, Action{Type: "catan_robber", Tile: 1})
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 0, Action{Type: "catan_robber", Tile: -1})
	if s.Phase != "catan_turn" || s.Catan.CitiesKnights.Chase != "" || s.Catan.knightAt(0).Active {
		t.Fatal("chase failed to resume")
	}
}

func TestCatanFriendlyKnightsRejectUnknownMetadataAndKeepLegacy(t *testing.T) {
	for _, version := range []string{"future", CatanFriendlyKnightsRules} {
		s := friendlyKnightsTaxationFixture(t)
		s.Catan.FriendlyRobber.Knights = version
		if version == CatanFriendlyKnightsRules {
			s.Catan.CitiesKnights = nil
		}
		helperReject(t, s, 0, Action{Type: "catan_progress", Card: 21, Tile: 1})
	}
	s := friendlyKnightsTaxationFixture(t)
	s.Catan.FriendlyRobber.Knights = ""
	restored := clone(*s)
	if restored.Catan.FriendlyRobber.Knights != "" {
		t.Fatal("legacy metadata changed")
	}
	base := catanGame(t, 3)
	if base.Catan.FriendlyRobber != nil || base.Catan.CitiesKnights != nil {
		t.Fatal("base configuration changed")
	}
}
