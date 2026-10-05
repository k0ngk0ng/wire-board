package game

import (
	"reflect"
	"slices"
	"testing"
)

func TestCatanCardRobberAttacksOneSevenAndNextRoll(t *testing.T) {
	s := cardEarthquakeFixture(t)
	rejectCardEvent(t, s, "robber_attacks", 6, 0, 0)
	for p := range 3 {
		give := make([]int, 5)
		give[p] = 8
		helperGrant(s, p, give)
	}
	beginCardEvent(t, s, "robber_attacks", 7, 0, 0)
	if s.Phase != "catan_discard" || s.Catan.CardEvent != nil || !slices.Equal(s.Catan.DiscardDue, []int{4, 4, 4}) {
		t.Fatal("attack did not use ordinary seven discard")
	}
	saved := clone(*s)
	s = &saved
	s.AutoCatanPending()
	if s.Phase != "catan_robber" || sum(s.Catan.DiscardDue) != 0 {
		t.Fatal("seven did not continue to robber")
	}
	// The one production hex touches all players: choose one victim explicitly.
	helperApply(t, s, 1, Action{Type: "catan_robber", Tile: 0})
	if s.Phase != "catan_steal" {
		t.Fatal("expected victim choice")
	}
	helperApply(t, s, 1, Action{Type: "catan_steal", Target: 0})
	if s.Phase != "catan_turn" || sum(s.Catan.Players[0].Resources) != 3 || sum(s.Catan.Players[1].Resources) != 5 || sum(s.Catan.Players[2].Resources) != 4 || s.Catan.RollID != 1 {
		t.Fatal("seven resolved more than once or gave production")
	}
	catanCheck(t, s)
}

func TestCatanCardRobberAttacksCityWallsAndSleepingRobber(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 1, 2})
	s.Phase = "catan_roll"
	g := s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.CitiesKnights.Walls = []int{0}
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 8, 0, 0})
	helperGrant(s, 1, []int{0, 0, 0, 0, 0, 0, 8, 0})
	beginCardEvent(t, s, "robber_attacks", 7, 6, 0)
	if !slices.Equal(s.Catan.DiscardDue, []int{0, 4, 0}) || s.Catan.CitiesKnights.Event != nil {
		t.Fatal("event seven ignored wall or commodity discard rules")
	}
	s.AutoCatanPending()
	if s.Phase != "catan_turn" || s.Catan.Robber != -1 || sum(s.Catan.Players[0].Resources) != 8 || sum(s.Catan.Players[1].Resources) != 4 {
		t.Fatal("sleeping robber moved or discard repeated")
	}
	ckSupply(t, s.Catan)
}

func TestCatanCardRobberAttacksPillageBeforeWallDiscardLimit(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 1, 2})
	s.Phase = "catan_roll"
	g := s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.CitiesKnights.Walls = []int{0}
	g.CitiesKnights.RobberStart = -1
	g.CitiesKnights.BarbarianPosition = 6
	helperGrant(s, 0, []int{8, 0, 0, 0, 0})
	beginCardEvent(t, s, "robber_attacks", 7, 2, 3)
	if s.Phase != "catan_pillage" || sum(s.Catan.DiscardDue) != 0 {
		t.Fatal("seven discard ran before barbarian attack")
	}
	helperApply(t, s, 0, Action{Type: "catan_pillage", Vertex: 0})
	if s.Phase != "catan_discard" || s.Catan.DiscardDue[0] != 4 || len(s.Catan.CitiesKnights.Walls) != 0 {
		t.Fatal("seven used a wall destroyed by this attack")
	}
	s.AutoCatanPending()
	if s.Phase != "catan_robber" || s.Catan.CitiesKnights.Invasions != 1 {
		t.Fatal("first invasion did not enable this seven's robber")
	}
	helperApply(t, s, 0, Action{Type: "catan_robber", Tile: 0})
	if s.Phase != "catan_turn" || sum(s.Catan.Players[0].Resources) != 4 {
		t.Fatal("seven or production repeated after invasion")
	}
	ckSupply(t, s.Catan)
}

func TestCatanCardRobberFleesMultipleDesertsNoStealAndPirateUnchanged(t *testing.T) {
	s := cardEarthquakeFixture(t)
	g := s.Catan
	g.Tiles = append(g.Tiles, CatanTile{ID: 1, Resource: CatanDesert, Vertices: []int{0}}, CatanTile{ID: 2, Resource: CatanDesert, Vertices: []int{3}}, CatanTile{ID: 3, Resource: CatanSea})
	g.Seafarers = &CatanSeafarers{Pirate: 3}
	g.Robber, g.Victims = 0, []int{0}
	helperGrant(s, 0, []int{0, 0, 3, 0, 0})
	beginCardEvent(t, s, "robber_flees", 4, 0, 0)
	if s.CatanPendingActor() != 1 || s.Catan.Robber != 0 {
		t.Fatal("multi-desert choice missing")
	}
	for viewer := -1; viewer < 3; viewer++ {
		legal := s.View(viewer)["catan"].(map[string]any)["legal"].(map[string][]int)
		if (len(legal["fleeDeserts"]) == 2) != (viewer == 1) {
			t.Fatal("desert choices shown to wrong player")
		}
	}
	for _, tile := range []int{-1, 0, 3, 4} {
		helperReject(t, s, 1, Action{Type: "catan_robber_flees", Tile: tile})
	}
	helperReject(t, s, 0, Action{Type: "catan_robber_flees", Tile: 1})
	helperReject(t, s, 1, Action{Type: "catan_robber", Tile: 1})
	saved := clone(*s)
	s = &saved
	helperApply(t, s, 1, Action{Type: "catan_robber_flees", Tile: 1})
	if s.Phase != "catan_turn" || s.Catan.Robber != 1 || s.Catan.Seafarers.Pirate != 3 || len(s.Catan.Victims) != 0 || sum(s.Catan.Players[0].Resources) != 3 || sum(s.Catan.Players[1].Resources) != 0 {
		t.Fatal("flee stole a card or changed pirate")
	}
	// Unlike ordinary activation, remaining on the current desert is valid.
	s.Phase = "catan_roll"
	beginCardEvent(t, s, "robber_flees", 4, 0, 0)
	s.AutoCatanPending()
	if s.Catan.Robber != 1 || s.Phase != "catan_turn" {
		t.Fatal("already-desert retreat stalled")
	}
	catanCheck(t, s)
}

func TestCatanCardRobberFleesNoDesertThenSevenOrKnight(t *testing.T) {
	for _, knight := range []bool{false, true} {
		s := cardEarthquakeFixture(t)
		s.Catan.Robber = 0
		beginCardEvent(t, s, "robber_flees", 4, 0, 0)
		if s.Catan.Robber != -1 || s.Phase != "catan_turn" {
			t.Fatal("no-desert flee did not move offboard")
		}
		if knight {
			catanCard(s.Catan, 1, 0)
			helperApply(t, s, 1, Action{Type: "catan_dev", Card: 0})
		} else {
			s.Phase = "catan_roll"
			beginCardEvent(t, s, "robber_attacks", 7, 0, 0)
		}
		if s.Phase != "catan_robber" || !s.Catan.robberAllowed(0) {
			t.Fatal("offboard robber could not be reactivated")
		}
		helperApply(t, s, 1, Action{Type: "catan_robber", Tile: 0})
		if s.Catan.Robber != 0 || s.Phase != "catan_turn" {
			t.Fatal("robber did not return to board")
		}
		catanCheck(t, s)
	}
}

func TestCatanCardRobberFleesBeforeProductionAndFirstInvasion(t *testing.T) {
	s := cardEarthquakeFixture(t)
	s.Catan.Robber = 0
	s.Catan.Tiles[0].Number = 4
	s.Catan.Tiles = append(s.Catan.Tiles, CatanTile{ID: 1, Resource: CatanDesert})
	beginCardEvent(t, s, "robber_flees", 4, 0, 0)
	if s.Catan.Robber != 1 || s.Phase != "catan_turn" {
		t.Fatal("single desert should not wait for an unnecessary choice")
	}
	for _, p := range s.Catan.Players {
		if p.Resources[0] != 1 {
			t.Fatal("flee was not resolved before production")
		}
	}
	s = ckEvent(t)
	start := s.Catan.CitiesKnights.RobberStart
	beginCardEvent(t, s, "robber_flees", 4, 6, 0)
	if s.Phase != "catan_turn" || s.Catan.Robber != -1 || s.Catan.CitiesKnights.RobberStart != start {
		t.Fatal("flee activated robber before first invasion")
	}
	s.Phase = "catan_roll"
	s.Catan.CitiesKnights.BarbarianPosition = 6
	beginCardEvent(t, s, "robber_flees", 4, 6, 3)
	for step := 0; step < 12 && s.CatanPendingActor() >= 0; step++ {
		s.AutoCatanPending()
	}
	if s.CatanPendingActor() >= 0 || s.Catan.Robber != start || s.Catan.CitiesKnights.Invasions != 1 {
		t.Fatal("flee changed normal first-invasion entry")
	}
	ckProgressStock(t, s.Catan)
}

func TestCatanCardRobberFleesDoesNotRevealFog(t *testing.T) {
	s, _ := fogTestGame(t, CatanDesert)
	s.Phase = "catan_roll"
	before := clone(*s.Catan.Seafarers.Fog)
	beginCardEvent(t, s, "robber_flees", 4, 0, 0)
	if s.Catan.Robber != -1 || !reflect.DeepEqual(before, *s.Catan.Seafarers.Fog) || s.Catan.Tiles[2].Resource != CatanFog {
		t.Fatal("flee inspected or revealed hidden desert")
	}
}
