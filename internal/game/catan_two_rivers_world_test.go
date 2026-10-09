package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func twoRiverWorldGame(t *testing.T, prepared, events bool) *State {
	t.Helper()
	var world *CatanRiversWorldMap
	if prepared {
		world = preparedRiverWorldFixtureContact(t, 4, true)
	}
	s, err := newCatanTwoRiversWorld(world)
	if err != nil {
		t.Fatal(err)
	}
	if events {
		if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
			t.Fatal(err)
		}
	}
	return s
}
func TestCatanTwoRiversWorldNatural(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("prepared%t/events%t", prepared, events), func(t *testing.T) {
				s := twoRiverWorldGame(t, prepared, events)
				m := s.Catan.RiversWorldMap()
				neutral, production := false, false
				for step := 0; step < 16000 && !s.Finished; step++ {
					neutral = neutral || s.Phase == "catan_two_build"
					production = production || len(s.Catan.Two.Rolls) == 2
					twoSeaStep(t, s)
					if step%73 == 0 {
						twoSeaRestore(t, s)
						riversSeaRestore(t, s)
						riverConserved(t, s)
					}
				}
				if !s.Finished || !neutral || !production {
					t.Fatal("incomplete", s.Round, s.Phase, neutral, production)
				}
				if !reflect.DeepEqual(m, s.Catan.RiversWorldMap()) {
					t.Fatal("map changed")
				}
				if err := s.validateTwoRiversWorld(); err != nil {
					t.Fatal(err)
				}
				twoSeaRestore(t, s)
				riversSeaRestore(t, s)
				t.Log("round", s.Round, "winners", s.Winners)
			})
		}
	}
}
func TestCatanTwoRiversWorldIsolation(t *testing.T) {
	s := twoRiverWorldGame(t, false, false)
	g := s.Catan
	if len(g.Players) != 2 || len(g.Seafarers.Seats) != 2 || len(g.Rivers.Gold) != 2 || g.Paired != nil || len(g.Tiles) != 42 || g.Rivers.Bank != 100 {
		t.Fatal("live seats/inventory")
	}
	for step := 0; step < 200 && s.Phase != "catan_roll"; step++ {
		twoSeaStep(t, s)
	}
	if s.Phase != "catan_roll" {
		t.Fatal("setup stuck")
	}
	g = s.Catan
	coins, tokens := slices.Clone(g.Rivers.Gold), slices.Clone(g.Two.Tokens)
	bank := g.Rivers.Bank
	for _, owner := range catanTwoNeutralOwners {
		if err := s.catanRiverReward(owner, 3); err != nil {
			t.Fatal(err)
		}
	}
	if !slices.Equal(coins, g.Rivers.Gold) || !slices.Equal(tokens, g.Two.Tokens) || bank != g.Rivers.Bank {
		t.Fatal("neutral reward leaked")
	}
	for _, kind := range []string{"bridge", "ship"} {
		c := clone(*s)
		b := c.Catan
		if kind == "bridge" {
			e := b.Edges[b.Rivers.Map.Bridges[0]]
			b.Vertices[e.A].Owner, b.Vertices[e.A].Level = -2, 1
		}
		choices := b.twoNeutralChoices(kind)
		if len(choices) == 0 {
			t.Fatal("missing neutral choices", kind)
		}
		before := slices.Clone(b.Rivers.Gold)
		if err := b.placeTwoNeutral(kind, choices[0]); err != nil {
			t.Fatal(err)
		}
		edge := b.Edges[choices[0].Edge]
		if kind == "bridge" && !edge.Bridge || kind == "ship" && !edge.Ship {
			t.Fatal("wrong piece", kind, edge)
		}
		if !slices.Equal(before, b.Rivers.Gold) {
			t.Fatal("neutral build gold")
		}
	}
	for _, version := range []string{"", "unknown"} {
		c := clone(*s)
		c.Catan.Two.RiversSea = version
		if c.validateTwoRiversWorld() == nil || c.validateCatanTwo() == nil {
			t.Fatal("invalid version accepted", version)
		}
	}
	c := clone(*s)
	c.Catan.Rivers = nil
	if c.validateCatanTwo() == nil {
		t.Fatal("orphan marker")
	}
	c = clone(*s)
	c.Catan.Rivers.SeaLayout = "extended"
	if c.Catan.validateRivers() == nil {
		t.Fatal("wrong map recipe")
	}
}

func TestCatanTwoRiversWorldPaidRoutes(t *testing.T) {
	for _, kind := range []string{"bridge", "ship"} {
		t.Run(kind, func(t *testing.T) {
			s := twoRiverWorldGame(t, false, false)
			for s.Phase == "catan_world_ports" {
				twoSeaStep(t, s)
			}
			g := s.Catan
			g.SetupStep = g.SetupLimit()
			g.Two.Rolls = []int{6, 8}
			s.Phase = "catan_turn"
			p := s.Turn
			target := -1
			for _, e := range g.Edges {
				if kind == "bridge" && !slices.Contains(g.Rivers.Map.Bridges, e.ID) || kind == "ship" && (!g.riverEdge(e.ID) || !g.edgeTerrain(e.ID, true)) {
					continue
				}
				for _, v := range []int{e.A, e.B} {
					if !g.canSettlement(p, v, true) {
						continue
					}
					g.Vertices[v].Owner, g.Vertices[v].Level = p, 1
					if kind == "bridge" && g.canBridge(p, e.ID) || kind == "ship" && g.canShip(p, e.ID) {
						target = e.ID
						break
					}
					g.Vertices[v].Owner, g.Vertices[v].Level = -1, 0
				}
				if target >= 0 {
					break
				}
			}
			if target < 0 {
				t.Fatal("no paid river route")
			}
			// Guarantee a legal neutral bridge for testing the actual pending response,
			// preserving the two recorded starting villages.
			if kind == "bridge" {
				for _, id := range g.Rivers.Map.Bridges {
					if id == target {
						continue
					}
					for _, v := range []int{g.Edges[id].A, g.Edges[id].B} {
						if g.canSettlement(-2, v, true) {
							g.Vertices[v].Owner, g.Vertices[v].Level = -2, 1
							break
						}
					}
					cs := g.twoNeutralChoices("bridge")
					if len(cs) > 0 && slices.Contains(g.Rivers.Map.Bridges, cs[0].Edge) {
						break
					}
				}
			}
			helperGrant(s, p, []int{2, 3, 2, 0, 0})
			before := g.Rivers.Gold[p]
			helperApply(t, s, p, Action{Type: "catan_" + kind, Edge: target})
			g = s.Catan
			reward := 1
			if kind == "bridge" {
				reward = 3
			}
			if g.Rivers.Gold[p] != before+reward || s.Phase != "catan_two_build" || g.Two.Pending.Kind != kind {
				t.Fatal("paid route continuation", s.Phase, g.Two.Pending)
			}
			cs := g.twoNeutralChoices(kind)
			if len(cs) == 0 {
				t.Fatal("no neutral route")
			}
			choice := cs[0]
			if kind == "bridge" && !slices.Contains(g.Rivers.Map.Bridges, choice.Edge) {
				t.Fatal("bridge unexpectedly fell back")
			}
			coins, tokens, bank := slices.Clone(g.Rivers.Gold), slices.Clone(g.Two.Tokens), slices.Clone(g.Bank)
			helperApply(t, s, p, Action{Type: "catan_two_build", Target: -choice.Owner - 2, Vertex: -1, Edge: choice.Edge})
			g = s.Catan
			if s.Phase != "catan_turn" || g.Edges[choice.Edge].Owner != choice.Owner || !slices.Equal(coins, g.Rivers.Gold) || !slices.Equal(tokens, g.Two.Tokens) || !slices.Equal(bank, g.Bank) {
				t.Fatal("neutral route continuation/ledger")
			}
			twoSeaRestore(t, s)
			riversSeaRestore(t, s)
			riverConserved(t, s)
		})
	}
}

func TestCatanTwoRiversWorldGoldProduction(t *testing.T) {
	s := twoRiverWorldGame(t, true, false)
	for s.Phase == "catan_world_ports" {
		twoSeaStep(t, s)
	}
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_roll"
	p := s.Turn
	id := slices.IndexFunc(g.Tiles, func(tile CatanTile) bool { return tile.Resource == CatanGold })
	if id < 0 {
		t.Fatal("fixture has no gold")
	}
	tile := g.Tiles[id]
	vertex := -1
	for _, v := range tile.Vertices {
		if g.canSettlement(p, v, true) {
			vertex = v
			break
		}
	}
	if vertex < 0 {
		t.Fatal("no gold settlement")
	}
	g.Vertices[vertex].Owner, g.Vertices[vertex].Level = p, 2
	coins, tokens := slices.Clone(g.Rivers.Gold), slices.Clone(g.Two.Tokens)
	a := tile.Number / 2
	b := tile.Number - a
	if err := s.catanTwoRoll(a, b); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_gold" || g.GoldPending == nil {
		t.Fatal("no gold response")
	}
	for s.Catan.GoldPending != nil {
		twoSeaRestore(t, s)
		riversSeaRestore(t, s)
		twoSeaStep(t, s)
	}
	g = s.Catan
	if s.Phase != "catan_roll" || len(g.Two.Rolls) != 1 || !slices.Equal(coins, g.Rivers.Gold) || !slices.Equal(tokens, g.Two.Tokens) {
		t.Fatal("first production/ledger")
	}
	if err := s.catanTwoRoll(3, 4); err != nil {
		t.Fatal(err)
	}
	for s.Phase != "catan_turn" {
		twoSeaStep(t, s)
	}
	if len(s.Catan.Two.Rolls) != 2 {
		t.Fatal("second production missing")
	}
	twoSeaRestore(t, s)
	riversSeaRestore(t, s)
	riverConserved(t, s)
}
