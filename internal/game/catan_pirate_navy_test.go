package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func navyFixture(t *testing.T, n, player int) *State {
	t.Helper()
	s := pirateIslandsMapGame(t, n)
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Turn = player
	s.Phase = "catan_turn"
	g.Seafarers.Pirate = -1
	fleetGive(g, player, []int{15, 0, 15, 0, 0})
	return s
}
func navyExtend(t *testing.T, s *State) int {
	t.Helper()
	g := s.Catan
	player := s.Turn
	f := g.pirateIslands().Fortresses[player]
	for _, e := range g.Edges {
		_, route, ok := g.pirateShipPlan(player, e.ID)
		if ok && len(route) == len(f.Route)+1 && g.canShip(player, e.ID) {
			helperApply(t, s, player, Action{Type: "catan_ship", Edge: e.ID})
			return e.ID
		}
	}
	t.Fatal("no shortest extension", player, f.Route)
	return -1
}
func TestCatanPirateNavyReachesEveryPrintedFortress(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for player := 0; player < n; player++ {
			t.Run(fmt.Sprintf("%d/%d", n, player), func(t *testing.T) {
				s := navyFixture(t, n, player)
				for count := 0; count < 15; count++ {
					g := s.Catan
					f := g.pirateIslands().Fortresses[player]
					vs, ok := g.pirateRouteVertices(player)
					if !ok {
						t.Fatal("invalid navy prefix")
					}
					if vs[len(vs)-1] == f.Vertex {
						if !slices.Contains(vs, f.Beachhead) {
							t.Fatal("bypassed beachhead")
						}
						fleetSupply(t, g)
						saved := clone(*s)
						if !reflect.DeepEqual(*s, saved) {
							t.Fatal("navy lost on restart")
						}
						return
					}
					navyExtend(t, s)
				}
				t.Fatal("fleet cannot reach fortress with fifteen ships")
			})
		}
	}
}
func TestCatanPirateNavyRejectsOffshoreBranchAndDetour(t *testing.T) {
	s := navyFixture(t, 4, 1)
	for i := 0; i < 3; i++ {
		navyExtend(t, s)
	}
	g := s.Catan
	vs, _ := g.pirateRouteVertices(1)
	rejected := 0
	for _, v := range vs[:len(vs)-1] {
		for _, id := range g.touching(v) {
			if g.Edges[id].Owner < 0 && g.edgeTerrain(id, true) && !g.pirateHomeCoast(id) {
				helperReject(t, s, 1, Action{Type: "catan_ship", Edge: id})
				rejected++
			}
		}
	}
	if rejected == 0 {
		t.Fatal("fixture did not cover any offshore branch")
	}
	for _, v := range g.Vertices {
		if g.pirateHomeVertex(v.ID) || v.ID == g.pirateIslands().Fortresses[1].Beachhead {
			continue
		}
		if g.canSettlement(1, v.ID, true) {
			t.Fatal("allowed arbitrary pirate-island settlement")
		}
	}
}
func TestCatanPirateNavyAuxiliaryCoastAndAnotherDeparture(t *testing.T) {
	s := navyFixture(t, 4, 1)
	g := s.Catan
	original := append([]int{}, g.pirateIslands().Fortresses[1].Route...)
	auxiliary := -1
	for _, e := range g.Edges {
		_, route, ok := g.pirateShipPlan(1, e.ID)
		if ok && g.canShip(1, e.ID) && g.pirateHomeCoast(e.ID) && reflect.DeepEqual(route, original) {
			auxiliary = e.ID
			break
		}
	}
	if auxiliary < 0 {
		t.Fatal("no auxiliary coast fixture")
	}
	helperApply(t, s, 1, Action{Type: "catan_ship", Edge: auxiliary})
	if !reflect.DeepEqual(s.Catan.pirateIslands().Fortresses[1].Route, original) {
		t.Fatal("coast branch replaced navy")
	}
	pirateGiveDev(t, s.Catan, 1, 0)
	helperApply(t, s, 1, Action{Type: "catan_dev", Card: 0})
	// An alternative owned coastal building may start the westbound line before
	// the existing line has any offshore ship or warship.
	found := false
	for _, v := range s.Catan.Vertices {
		if !s.Catan.pirateHomeVertex(v.ID) || v.Level > 0 {
			continue
		}
		copy := clone(*s)
		g = copy.Catan
		g.Vertices[v.ID].Owner = 1
		g.Vertices[v.ID].Level = 1
		for _, id := range g.touching(v.ID) {
			root, route, ok := g.pirateShipPlan(1, id)
			if ok && root == v.ID && len(route) > 0 && !g.pirateHomeCoast(id) && g.canShip(1, id) {
				helperApply(t, &copy, 1, Action{Type: "catan_ship", Edge: id})
				if copy.Catan.pirateIslands().Fortresses[1].Root != v.ID {
					t.Fatal("departure choice lost")
				}
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("no alternative departure accepted")
	}
}
func TestCatanPirateNavyShipMoveDoesNotMutatePlanningState(t *testing.T) {
	s := navyFixture(t, 4, 1)
	for i := 0; i < 2; i++ {
		navyExtend(t, s)
	}
	g := s.Catan
	f := g.pirateIslands().Fortresses[1]
	tail := f.Route[len(f.Route)-1]
	g.Seafarers.BuiltShips = nil
	g.Edges[tail].Warship = true
	before := clone(*s)
	destinations := g.shipDestinations(1, tail)
	if !reflect.DeepEqual(*s, before) {
		t.Fatal("move preview mutated live route")
	}
	if len(destinations) == 0 {
		t.Fatal("missing legal tail destination")
	}
	helperApply(t, s, 1, Action{Type: "catan_move_ship", Edge: tail, Target: destinations[0]})
	g = s.Catan
	if g.Edges[tail].Owner >= 0 || g.Edges[tail].Warship || !g.Edges[destinations[0]].Warship || !g.Seafarers.MovedShip {
		t.Fatal("warship identity / move lock")
	}
	if _, ok := g.pirateRouteVertices(1); !ok {
		t.Fatal("route broken by legal move")
	}
}
