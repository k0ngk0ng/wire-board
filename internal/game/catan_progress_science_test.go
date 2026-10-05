package game

import (
	"reflect"
	"slices"
	"testing"
)

func TestCatanProgressSciencePhaseGuardsAndAlchemy(t *testing.T) {
	s := ckEvent(t)
	ckProgressGive(t, s, 0, 0, 1, 10)
	helperReject(t, s, 1, Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 5}})
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 1})
	for _, dice := range [][]int{nil, {1}, {0, 6}, {1, 7}, {1, 2, 3}} {
		helperReject(t, s, 0, Action{Type: "catan_progress", Card: 0, Tokens: dice})
	}
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 5}, Choice: "skip"})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 0, Tokens: []int{2, 4}, Color: 999}) // Color cannot choose the event die.
	if !reflect.DeepEqual(s.Catan.Dice, []int{2, 4}) || s.Catan.RollID != 1 || s.Catan.CitiesKnights.EventDie < 0 || s.Catan.CitiesKnights.EventDie > 5 || s.Phase != "catan_turn" {
		t.Fatal("alchemy did not select two production dice and resolve an independent event")
	}
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 5}})
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 10})
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 9})
	ckProgressStock(t, s.Catan)
}
func TestCatanProgressScienceCraneCostCityAndMetropolis(t *testing.T) {
	s := ckEmptyTurn(t)
	ckProgressGive(t, s, 0, 1, 1)
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 1, Color: 0})
	s.Catan.Vertices[0].Owner, s.Catan.Vertices[0].Level = 0, 2
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 1, Color: 0})
	if s.Catan.CitiesKnights.Players[0].Improvements[0] != 1 || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("first level was not free")
	}
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 1, Color: 0})
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 1, 0, 0})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 1, Color: 0})
	if s.Catan.CitiesKnights.Players[0].Improvements[0] != 2 || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("Crane did not discount exactly one commodity")
	}
	ckProgressStock(t, s.Catan)
	s = ckEmptyTurn(t)
	s.Catan.Vertices[0].Owner, s.Catan.Vertices[0].Level = 0, 2
	s.Catan.CitiesKnights.Players[0].Improvements[0] = 3
	ckProgressGive(t, s, 0, 1)
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 3, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_improvement", Color: 0, Card: 1, Skill: "progress"})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 1, Color: 0})
	if s.Phase != "catan_metropolis" || s.Catan.Players[0].Resources[5] != 0 {
		t.Fatal("discounted metropolis purchase")
	}
	restored := clone(*s)
	s = &restored
	s.AutoCatanPending()
	if s.Catan.cityMetropolisOwner(0) != 0 || s.Catan.Players[0].Score != 4 {
		t.Fatal("Crane metropolis continuation")
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanProgressScienceEngineeringAndMedicine(t *testing.T) {
	s := ckEmptyTurn(t)
	s.Catan.Vertices[0].Owner, s.Catan.Vertices[0].Level = 0, 2
	ckProgressGive(t, s, 0, 2)
	helperReject(t, s, 0, Action{Type: "catan_wall", Vertex: 0, Card: 2, Skill: "progress"})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 2, Vertex: 0})
	if !reflect.DeepEqual(s.Catan.CitiesKnights.Walls, []int{0}) || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("free wall")
	}
	ckProgressStock(t, s.Catan)
	s = ckEmptyTurn(t)
	for v := range 4 {
		s.Catan.Vertices[v].Owner, s.Catan.Vertices[v].Level = 0, 2
	}
	s.Catan.CitiesKnights.Walls = []int{0, 1, 2}
	ckProgressGive(t, s, 0, 2)
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 2, Vertex: 3})
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 2, Vertex: 0})
	s = ckEmptyTurn(t)
	s.Catan.Vertices[0].Owner, s.Catan.Vertices[0].Level = 0, 1
	ckProgressGive(t, s, 0, 5)
	helperGrant(s, 0, []int{0, 0, 0, 1, 2, 0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_city", Vertex: 0, Card: 5, Skill: "medicine"})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 5, Vertex: 0})
	if s.Catan.Vertices[0].Level != 2 || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("Medicine must cost exactly one grain and two ore")
	}
	ckProgressStock(t, s.Catan)
	s = ckEmptyTurn(t)
	for v := range 9 {
		s.Catan.Vertices[v].Owner = 0
		s.Catan.Vertices[v].Level = 1
		if v > 5 {
			s.Catan.Vertices[v].Level = 2
		}
	}
	s.Catan.CitiesKnights.FallenCities = []int{0}
	ckProgressGive(t, s, 0, 5)
	helperGrant(s, 0, []int{0, 0, 0, 1, 2, 0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 5, Vertex: 1})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 5, Vertex: 0})
	if len(s.Catan.CitiesKnights.FallenCities) != 0 || s.Catan.cityPiecesLeft(0) != 0 {
		t.Fatal("Medicine bypassed fallen city or physical city limit")
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanProgressScienceInventionNumberRestrictionsAndRobber(t *testing.T) {
	s := ckEmptyTurn(t)
	ckProgressGive(t, s, 0, 3)
	s.Catan.Tiles[0].Number, s.Catan.Tiles[1].Number = 3, 5
	s.Catan.Robber = 0
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 3, Tile: 0, Target: 0})
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 3, Tile: -1, Target: 1})
	for _, number := range []int{0, 2, 6, 8, 12} {
		s.Catan.Tiles[2].Number = number
		helperReject(t, s, 0, Action{Type: "catan_progress", Card: 3, Tile: 0, Target: 2})
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 3, Tile: 0, Target: 1})
	if s.Catan.Tiles[0].Number != 5 || s.Catan.Tiles[1].Number != 3 || s.Catan.Robber != 0 {
		t.Fatal("number swap moved robber or wrong tokens")
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanProgressScienceIrrigationMiningPerHexAndShortage(t *testing.T) {
	for _, card := range []int{4, 6} {
		color := 3
		if card == 6 {
			color = 4
		}
		for _, stock := range []int{19, 1, 0} {
			s := ckEmptyTurn(t)
			g := s.Catan
			g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
			g.Vertices[1].Owner, g.Vertices[1].Level = 0, 2
			g.Vertices[2].Owner, g.Vertices[2].Level = 0, 1
			g.Tiles = []CatanTile{{ID: 0, Resource: color, Number: 6, Vertices: []int{0, 1}}, {ID: 1, Resource: color, Number: 5, Vertices: []int{2}}}
			g.Robber = 0
			g.Players[2].Resources[color] = 19 - stock
			g.Bank[color] = stock
			ckProgressGive(t, s, 0, card)
			helperApply(t, s, 0, Action{Type: "catan_progress", Card: card})
			if s.Catan.Players[0].Resources[color] != min(4, stock) || len(s.Catan.CitiesKnights.Players[0].Progress) != 0 || s.Catan.CitiesKnights.ProgressDecks[0][0] != card {
				t.Fatal("card counted buildings instead of hexes, blocked non-production grant, or failed to return after zero benefit")
			}
			ckProgressStock(t, s.Catan)
		}
	}
}
func TestCatanProgressScienceSmithingStockOrderLocksAndAtomicity(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0, 0, 0, 0})
	k := s.Catan.CitiesKnights
	k.Players[0].Improvements[2] = 3
	k.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 1, Active: true}, {Owner: 0, Vertex: 2, Strength: 2}, {Owner: 0, Vertex: 4, Strength: 2}, {Owner: 1, Vertex: 6, Strength: 1}}
	ckProgressGive(t, s, 0, 8, 8)
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 8, Targets: []int{2, 6}}) // First promotion must roll back too.
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 8, Targets: []int{2, 2}})
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 8, Targets: []int{0, 2}}) // Strong pieces are all occupied.
	helperReject(t, s, 0, Action{Type: "catan_knight_promote", Vertex: 2, Card: 8, Choice: "free"})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 8, Targets: []int{2, 0}})
	if s.Catan.knightAt(2).Strength != 3 || s.Catan.knightAt(0).Strength != 2 || !s.Catan.knightAt(0).Active || s.Catan.knightAt(2).Active || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("Smithing cost/stock/activation")
	}
	restored := clone(*s)
	s = &restored
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 8, Targets: []int{0}})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 8, Targets: []int{4}})
	ckKnightStock(t, s.Catan)
	ckProgressStock(t, s.Catan)
	s = ckKnightGraph(t, []int{0, 0})
	s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 2}}
	ckProgressGive(t, s, 0, 8)
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 8, Targets: []int{0}})
}
func TestCatanProgressScienceRoadsRestoreAndMultipleCards(t *testing.T) {
	s := ckKnightGraph(t, []int{-1, -1, -1})
	s.Catan.Vertices[0].Owner, s.Catan.Vertices[0].Level = 0, 1
	s.Catan.PlayedDev = true
	ckProgressGive(t, s, 0, 7, 4)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 7})
	helperReject(t, s, 0, Action{Type: "catan_end"})
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 4})
	helperApply(t, s, 0, Action{Type: "catan_road", Edge: 0})
	if s.Catan.FreeRoads != 1 || s.Phase != "catan_roads" {
		t.Fatal("first free road continuation")
	}
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 0, Action{Type: "catan_road", Edge: 1})
	if s.Catan.FreeRoads != 0 || s.Phase != "catan_turn" || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("free roads cost/return phase")
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 4})
	ckProgressStock(t, s.Catan)
	s = ckKnightGraph(t, []int{1, 1, 1})
	ckProgressGive(t, s, 0, 7)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 7})
	if s.Phase != "catan_turn" {
		t.Fatal("no free route stalled")
	}
	owners := make([]int, 15)
	owners[14] = -1
	s = ckKnightGraph(t, owners)
	ckProgressGive(t, s, 0, 7)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 7})
	helperApply(t, s, 0, Action{Type: "catan_road", Edge: 14})
	if s.Phase != "catan_turn" || s.Catan.FreeRoads != 0 {
		t.Fatal("last physical road must finish partial effect")
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanProgressScienceBotsViewsAndHiddenIndependence(t *testing.T) {
	s := ckEvent(t)
	g := s.Catan
	g.Tiles[0].Number, g.Tiles[0].Resource, g.Tiles[0].Vertices = 8, 0, []int{0}
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	ckProgressGive(t, s, 0, 0)
	a, err := s.BotAction(0)
	if err != nil || a.Type != "catan_progress" || a.Card != 0 || sum(a.Tokens) != 8 {
		t.Fatal("Alchemy bot failed to use public production", a, err)
	}
	other := clone(*s)
	slices.Reverse(other.Catan.CitiesKnights.ProgressDecks[0])
	other.Catan.Players[1].Resources = []int{9, 8, 7, 6, 5, 4, 3, 2}
	b, err := other.BotAction(0)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("Alchemy bot used hidden state")
	}
	for _, viewer := range []int{-1, 0, 1} {
		v := s.View(viewer)["catan"].(map[string]any)
		if (len(v["progressPlayable"].([]int)) > 0) != (viewer == 0) {
			t.Fatal("playable progress exposed for wrong viewer")
		}
	}
	helperApply(t, s, 0, a)
	ckProgressStock(t, s.Catan)
	s = ckKnightGraph(t, []int{0, 0, 0})
	s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 1}, {Owner: 0, Vertex: 2, Strength: 1}}
	ckProgressGive(t, s, 0, 8)
	a, err = s.BotAction(0)
	if err != nil || a.Card != 8 || len(a.Targets) != 2 {
		t.Fatal("Smithing bot", a, err)
	}
	helperApply(t, s, 0, a)
	ckProgressStock(t, s.Catan)
}
