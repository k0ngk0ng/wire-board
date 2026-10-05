package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Player 0's ship line is 0--1--2--3--4--5--6, anchored at building 0.
// Villages occupy 4 and 6; player 1 can retreat from 2 through 7 back to 3.
// Player 2 has a branch 3--8 for Treason placement tests.
func ckClothKnightGraph(t *testing.T) *State {
	t.Helper()
	s := ckKnightGraph(t, []int{0, 0, 0, 0, 0, 0})
	g := s.Catan
	g.SetupStep = 9
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 1
	for i := range g.Edges {
		g.Edges[i].Ship = true
	}
	g.Vertices = append(g.Vertices, CatanVertex{ID: 7, Owner: -1}, CatanVertex{ID: 8, Owner: -1})
	g.Edges = append(g.Edges,
		CatanEdge{ID: 6, A: 2, B: 7, Owner: 1, Ship: true},
		CatanEdge{ID: 7, A: 7, B: 3, Owner: 1, Ship: true},
		CatanEdge{ID: 8, A: 3, B: 8, Owner: 2, Ship: true})
	g.Seafarers = &CatanSeafarers{Scenario: "cloth", Pirate: -1, VictoryPoints: 16, Cloth: &CatanClothState{
		Stock: 10, Held: make([]int, 3), HomeTiles: []int{0}, EmptyLimit: 5,
		Villages: []CatanClothVillage{{Vertex: 4, Number: 6, Stock: 5}, {Vertex: 6, Number: 8, Stock: 5}},
	}}
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 1, Vertex: 2, Strength: 1, Active: true}}
	return s
}

func TestCatanClothKnightsBlockNewTradeButKeepEstablishedRelations(t *testing.T) {
	s := ckClothKnightGraph(t)
	g := s.Catan
	total := clothTotal(g)
	for _, active := range []bool{false, true} {
		g.CitiesKnights.Knights[0].Active = active
		s.catanClothTrade(0)
		if g.cloth().Held[0] != 0 {
			t.Fatal("opponent knight did not block new trade")
		}
	}
	s.Turn = 1
	helperApply(t, s, 1, Action{Type: "catan_knight_move", Vertex: 2, Target: 7})
	g = s.Catan
	if g.cloth().Held[0] != 2 || g.Players[0].Score != 2 || clothTotal(g) != total {
		t.Fatal("moving enemy knight did not open both villages and update score")
	}
	// The first village is an occupied destination, but routes may pass it.
	if !reflect.DeepEqual(g.cloth().Villages[1].Traders, []int{0}) {
		t.Fatal("village incorrectly blocked the next village")
	}
	g.CitiesKnights.Knights[0].Vertex = 2
	s.catanClothKnightRoutes()
	if err := s.catanProduceCloth(6); err != nil {
		t.Fatal(err)
	}
	if g.cloth().Held[0] != 3 || clothTotal(g) != total {
		t.Fatal("existing trade was revoked or connection reward repeated")
	}
	for _, edge := range g.Edges[:6] {
		if g.movableShip(0, edge.ID) || slices.Contains(g.diplomacyRoads(), edge.ID) {
			t.Fatal("opponent interruption opened a closed village route", edge.ID)
		}
	}
	// Own knights allow passage, but never substitute for an own building.
	s = ckClothKnightGraph(t)
	g = s.Catan
	g.CitiesKnights.Knights[0].Owner = 0
	s.catanClothTrade(0)
	if g.cloth().Held[0] != 2 {
		t.Fatal("own knight blocked trade")
	}
	s = ckClothKnightGraph(t)
	g = s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = -1, 0
	g.CitiesKnights.Knights[0].Owner = 0
	s.catanClothTrade(0)
	if g.cloth().Held[0] != 0 {
		t.Fatal("knight used as building anchor")
	}
}

func TestCatanClothVillageCannotHoldKnightButAllowsPassage(t *testing.T) {
	s := ckClothKnightGraph(t)
	g := s.Catan
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 1, Strength: 1, Active: true}}
	helperGrant(s, 0, []int{0, 0, 1, 0, 1, 0, 0, 0})
	for _, v := range []int{4, 6} {
		if g.knightPlaceable(0, v) || slices.Contains(g.knightDestinations(g.CitiesKnights.Knights[0], false), v) || slices.Contains(g.treasonPlacements(0, 1), v) {
			t.Fatal("village offered as knight destination", v)
		}
		helperReject(t, s, 0, Action{Type: "catan_knight_recruit", Vertex: v})
		helperReject(t, s, 0, Action{Type: "catan_knight_move", Vertex: 1, Target: v})
	}
	if !slices.Contains(g.knightDestinations(g.CitiesKnights.Knights[0], false), 5) {
		t.Fatal("neutral village blocked travel through an own route")
	}
	helperApply(t, s, 0, Action{Type: "catan_knight_move", Vertex: 1, Target: 5})
	ckKnightStock(t, s.Catan)
}

func TestCatanClothTradeWaitsForIntrigueRetreat(t *testing.T) {
	for _, target := range []int{3, 7} {
		t.Run(fmt.Sprint(target), func(t *testing.T) {
			s := ckClothKnightGraph(t)
			ckProgressGive(t, s, 0, 19)
			total := clothTotal(s.Catan)
			helperApply(t, s, 0, Action{Type: "catan_progress", Card: 19, Vertex: 2})
			if s.Phase != "catan_knight_retreat" || s.Catan.cloth().Held[0] != 0 {
				t.Fatal("temporary route gap awarded cloth")
			}
			saved := clone(*s)
			s = &saved
			helperReject(t, s, 0, Action{Type: "catan_knight_retreat", Vertex: target})
			helperApply(t, s, 1, Action{Type: "catan_knight_retreat", Vertex: target})
			want := 0
			if target == 7 {
				want = 2
			}
			if s.Phase != "catan_turn" || s.Catan.cloth().Held[0] != want || clothTotal(s.Catan) != total {
				t.Fatal("trade did not use final retreat position", s.Catan.cloth())
			}
			ckProgressStock(t, s.Catan)
			ckKnightStock(t, s.Catan)
		})
	}
}

func TestCatanClothTradeWaitsForTreasonPlacement(t *testing.T) {
	for _, skip := range []bool{false, true} {
		t.Run(fmt.Sprint(skip), func(t *testing.T) {
			s := ckClothKnightGraph(t)
			s.Turn = 2
			ckProgressGive(t, s, 2, 22)
			total := clothTotal(s.Catan)
			helperApply(t, s, 2, Action{Type: "catan_progress", Card: 22, Target: 1})
			helperApply(t, s, 1, Action{Type: "catan_treason_remove", Vertex: 2})
			if s.Phase != "catan_treason_place" || s.Catan.cloth().Held[0] != 0 {
				t.Fatal("Treason gap awarded cloth before replacement")
			}
			saved := clone(*s)
			s = &saved
			a := Action{Type: "catan_treason_place", Vertex: 3, Color: 1}
			want := 0
			if skip {
				a.Choice = "skip"
				want = 2
			}
			helperApply(t, s, 2, a)
			if s.Phase != "catan_turn" || s.Catan.cloth().Held[0] != want || clothTotal(s.Catan) != total {
				t.Fatal("trade did not use final Treason placement")
			}
			ckProgressStock(t, s.Catan)
			ckKnightStock(t, s.Catan)
		})
	}
}

func TestCatanClothTradeRefreshSkipsEliminatedAndPreservesOwnTurnVictory(t *testing.T) {
	s := ckClothKnightGraph(t)
	g := s.Catan
	g.cloth().Held[0] = 29 // isolated victory boundary fixture
	s.Turn = 1
	helperApply(t, s, 1, Action{Type: "catan_knight_move", Vertex: 2, Target: 7})
	if s.Finished || s.Catan.Players[0].Score != 16 {
		t.Fatal("non-active player must wait for own turn")
	}
	s.Turn = 0
	s.catanVictory()
	if !s.Finished || !reflect.DeepEqual(s.Winners, []int{0}) {
		t.Fatal("new cloth not included in own-turn victory")
	}
	s = ckClothKnightGraph(t)
	s.Turn = 1
	if err := s.EliminateCatan(1); err != nil {
		t.Fatal(err)
	}
	if s.Catan.cloth().Held[0] != 2 {
		t.Fatal("removed player's knight still blocked trade")
	}
	s = ckClothKnightGraph(t)
	s.Catan.Players[0].Eliminated = true
	s.Turn = 1
	helperApply(t, s, 1, Action{Type: "catan_knight_move", Vertex: 2, Target: 7})
	if s.Catan.cloth().Held[0] != 0 {
		t.Fatal("eliminated player established new trade")
	}
}
