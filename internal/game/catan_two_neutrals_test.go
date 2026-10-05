package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func twoNeutralFixture(t *testing.T) *State {
	t.Helper()
	s := &State{Kind: "catan", Round: 1}
	s.initCatan(2)
	if err := s.Catan.prepareTwoNeutrals(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCatanTwoNeutralPrintedSetupAndIsolation(t *testing.T) {
	for sample := 0; sample < 48; sample++ {
		s := &State{Kind: "catan", Round: 1}
		s.initCatan(2)
		before := clone(*s)
		g := s.Catan
		if err := g.prepareTwoNeutrals(); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct{ tile, corner, owner int }{{1, 1, -2}, {17, 4, -3}} {
			id := g.Tiles[tc.tile].Vertices[tc.corner]
			if v := g.Vertices[id]; v.Owner != tc.owner || v.Level != 1 || v.X != 340 {
				t.Fatal("printed centerline village", v)
			}
			if roads, villages, cities := g.pieces(tc.owner); roads != 0 || villages != 1 || cities != 0 {
				t.Fatal("neutral initial pieces")
			}
			// Normalize only the two intended changes, then compare everything.
			before.Catan.Vertices[id] = g.Vertices[id]
		}
		if !reflect.DeepEqual(*s, before) {
			t.Fatal("neutral setup changed map, players or supply")
		}
		b, _ := json.Marshal(s)
		var saved State
		if err := json.Unmarshal(b, &saved); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(saved.Catan.twoNeutralChoices("road"), g.twoNeutralChoices("road")) {
			t.Fatal("neutral choices changed on restore")
		}
		b, _ = json.Marshal(s)
		if g.prepareTwoNeutrals() == nil {
			t.Fatal("overwrote occupied setup")
		}
		after, _ := json.Marshal(s)
		if string(b) != string(after) {
			t.Fatal("rejected setup mutated state")
		}
	}
	ordinary, err := NewCatan(3, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if ordinary.Catan.prepareTwoNeutrals() == nil {
		t.Fatal("changed ordinary game")
	}
	if _, err = NewCatan(2, CatanOptions{}); err == nil {
		t.Fatal("unfinished two-player rules exposed")
	}
}

func TestCatanTwoNeutralConstructionFallbackAndBlocking(t *testing.T) {
	s := twoNeutralFixture(t)
	g := s.Catan
	if len(g.twoNeutralChoices("city")) != 0 {
		t.Fatal("neutral city")
	}
	initial := g.twoNeutralChoices("settlement")
	if len(initial) == 0 || initial[0].Vertex != -1 {
		t.Fatal("initial village must fall back to road")
	}
	if err := g.placeTwoNeutral("settlement", initial[0]); err != nil {
		t.Fatal(err)
	}
	occupied := initial[0].Edge
	for _, p := range []int{0, 1, -2, -3} {
		g.SetupVertex = g.Edges[occupied].A
		if g.canRoad(p, occupied) || g.setupRoute(p, occupied, false) {
			t.Fatal("occupied neutral road reused", p)
		}
	}
	for step := 0; step < 15; step++ {
		choices := g.twoNeutralChoices("settlement")
		if len(choices) > 0 && choices[0].Vertex >= 0 {
			before, _ := json.Marshal(g)
			road := g.twoNeutralChoices("road")[0]
			if g.placeTwoNeutral("settlement", road) == nil {
				t.Fatal("fell back while a village was legal")
			}
			after, _ := json.Marshal(g)
			if string(before) != string(after) {
				t.Fatal("invalid choice mutated board")
			}
			choice := choices[0]
			if err := g.placeTwoNeutral("settlement", choice); err != nil {
				t.Fatal(err)
			}
			for _, edge := range g.touching(choice.Vertex) {
				e := g.Edges[edge]
				other := e.A + e.B - choice.Vertex
				if g.canSettlement(0, other, true) || g.canSettlement(-3, other, true) {
					t.Fatal("neutral distance rule ignored")
				}
			}
			if !g.opponentPiece(0, choice.Vertex) || g.opponentPiece(choice.Owner, choice.Vertex) {
				t.Fatal("neutral blocker identity")
			}
			return
		}
		roads := g.twoNeutralChoices("road")
		if len(roads) == 0 {
			t.Fatal("network stopped before village")
		}
		if err := g.placeTwoNeutral("road", roads[0]); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("no connected neutral village found")
}

func TestCatanTwoNeutralInventoryAndRestore(t *testing.T) {
	s := twoNeutralFixture(t)
	g := s.Catan
	// Explicit stock-boundary fixture: five legal villages of each color.
	for _, owner := range catanTwoNeutralOwners {
		for _, v := range g.Vertices {
			_, count, _ := g.pieces(owner)
			if count == 5 {
				break
			}
			if g.canSettlement(owner, v.ID, true) {
				g.Vertices[v.ID].Owner, g.Vertices[v.ID].Level = owner, 1
			}
		}
		_, count, _ := g.pieces(owner)
		if count != 5 {
			t.Fatal("stock fixture")
		}
	}
	for step := 0; step < 31; step++ {
		choices := g.twoNeutralChoices("settlement")
		if len(choices) == 0 {
			break
		}
		for _, c := range choices {
			if c.Vertex >= 0 {
				t.Fatal("sixth neutral settlement")
			}
		}
		if err := g.placeTwoNeutral("settlement", choices[0]); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(s)
		var restored State
		if err := json.Unmarshal(b, &restored); err != nil {
			t.Fatal(err)
		}
		*s = restored
		g = s.Catan
	}
	for _, owner := range catanTwoNeutralOwners {
		r, v, c := g.pieces(owner)
		if r != 15 || v != 5 || c != 0 {
			t.Fatal("neutral stock", owner, r, v, c)
		}
	}
	if len(g.twoNeutralChoices("road")) != 0 || len(g.twoNeutralChoices("settlement")) != 0 {
		t.Fatal("empty plastic supply still buildable")
	}
	for _, seat := range g.Players {
		if sum(seat.Resources) != 0 || sum(seat.Dev) != 0 {
			t.Fatal("neutral construction changed hand")
		}
	}
	if !slices.Equal(g.Bank, []int{19, 19, 19, 19, 19}) {
		t.Fatal("neutral construction spent resources")
	}
}

func TestCatanTwoNeutralProductionAndRobberIsolation(t *testing.T) {
	s := twoNeutralFixture(t)
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_roll"
	// No real buildings: producing next to either neutral changes no resources.
	for total := 2; total <= 12; total++ {
		if total == 7 {
			continue
		}
		if err := s.catanRollProduction(total); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(g.Bank, []int{19, 19, 19, 19, 19}) {
			t.Fatal("neutral produced resources")
		}
	}
	for _, tile := range []int{1, 17} {
		g.Robber = -1
		g.ResumePhase = "catan_turn"
		s.Phase = "catan_robber"
		if err := s.catanMoveRobber(0, tile); err != nil {
			t.Fatal(err)
		}
		if len(g.Victims) != 0 || s.Phase != "catan_turn" {
			t.Fatal("neutral robber victim")
		}
	}
}

func TestCatanTwoNeutralRealSetupDoesNotCreateNeutralSeats(t *testing.T) {
	for sample := 0; sample < 24; sample++ {
		s := twoNeutralFixture(t)
		for s.Catan.setup() {
			a, err := s.BotAction(s.Turn)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Apply(s.Turn, a); err != nil {
				t.Fatal(err)
			}
		}
		g := s.Catan
		if len(g.Players) != 2 || g.SetupStep != 4 || s.Phase != "catan_roll" {
			t.Fatal("neutral became a turn/seat")
		}
		for p := range 2 {
			roads, villages, cities := g.pieces(p)
			if roads != 2 || villages != 2 || cities != 0 || g.Players[p].Score != 2 {
				t.Fatal("ordinary setup disturbed")
			}
		}
		for _, owner := range catanTwoNeutralOwners {
			roads, villages, cities := g.pieces(owner)
			if roads != 0 || villages != 1 || cities != 0 {
				t.Fatal("setup duplicated neutral pieces")
			}
		}
		for color, total := range g.Bank {
			for _, seat := range g.Players {
				total += seat.Resources[color]
			}
			if total != 19 {
				t.Fatal("setup resource conservation")
			}
		}
	}
}

func TestCatanTwoSettlementTradeTokenRewards(t *testing.T) {
	s := twoNeutralFixture(t)
	g := s.Catan
	for i := range g.Tiles {
		if g.Tiles[i].Resource == CatanDesert {
			g.Tiles[i].Resource = 0
		}
	}
	g.Tiles[0].Resource = CatanDesert
	// Top-left corner touches desert and two frame edges: exactly three.
	coast := g.Tiles[0].Vertices[4]
	if g.twoSettlementTokens(0, coast) != 3 || g.twoSettlementTokens(-2, coast) != 0 {
		t.Fatal("additive reward or neutral token grant")
	}
	interior := g.Tiles[0].Vertices[0]
	if g.twoSettlementTokens(1, interior) != 2 {
		t.Fatal("desert-only reward")
	}
	if g.twoSettlementTokens(1, g.Tiles[18].Vertices[1]) != 1 {
		t.Fatal("coast-only reward")
	}
	if g.twoSettlementTokens(0, g.Tiles[9].Vertices[0]) != 0 {
		t.Fatal("ordinary interior reward")
	}
}

func twoSixEdgePath(t *testing.T, g *Catan, owner int) []int {
	t.Helper()
	var walk func(int, []int, map[int]bool) []int
	walk = func(v int, edges []int, seen map[int]bool) []int {
		if len(edges) == 6 {
			return append([]int{}, edges...)
		}
		for _, id := range g.touching(v) {
			e := g.Edges[id]
			to := e.A + e.B - v
			if seen[to] || g.opponentPiece(owner, to) || !g.canRoad(owner, id) {
				continue
			}
			g.Edges[id].Owner = owner
			seen[to] = true
			if found := walk(to, append(edges, id), seen); found != nil {
				return found
			}
			g.Edges[id].Owner = -1
			delete(seen, to)
		}
		return nil
	}
	for _, v := range g.Vertices {
		if v.Owner == owner && v.Level > 0 {
			if found := walk(v.ID, nil, map[int]bool{v.ID: true}); found != nil {
				return found
			}
		}
	}
	t.Fatal("could not construct six-edge path", owner)
	return nil
}

func TestCatanTwoNeutralLongestRoute(t *testing.T) {
	s := twoNeutralFixture(t)
	g := s.Catan
	g.Vertices[g.Tiles[0].Vertices[4]].Owner = 0
	g.Vertices[g.Tiles[0].Vertices[4]].Level = 1
	neutral, real := twoSixEdgePath(t, g, -2), twoSixEdgePath(t, g, 0)
	g.Edges[neutral[5]].Owner = -1
	g.Edges[real[5]].Owner = -1
	if g.roadLength(-2) != 5 || g.roadLength(0) != 5 {
		t.Fatal("path length")
	}
	if g.twoLongestOwner(-1) != -1 || g.twoLongestOwner(-2) != -2 || g.twoLongestOwner(0) != 0 {
		t.Fatal("neutral/real tie retention")
	}
	g.Edges[neutral[5]].Owner = -2
	if g.twoLongestOwner(0) != -2 {
		t.Fatal("neutral did not take longest route")
	}
	a, b := g.Edges[neutral[2]], g.Edges[neutral[3]]
	cut := a.A
	if cut != b.A && cut != b.B {
		cut = a.B
	}
	g.Vertices[cut].Owner, g.Vertices[cut].Level = 1, 1
	if g.roadLength(-2) != 3 || g.twoLongestOwner(-2) != 0 {
		t.Fatal("enemy village did not split neutral route")
	}
}
