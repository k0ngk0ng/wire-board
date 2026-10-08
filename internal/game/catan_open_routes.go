package game

import "slices"

// Openness differs from longest-route connectivity: an enemy piece interrupts
// scoring but never opens a closed line. Official FAQ also allows an unanchored
// cycle, or either edge beside the sole anchor in a cycle, to be opened.
func (g *Catan) openRoute(player, edge int) bool {
	if edge < 0 || edge >= len(g.Edges) || g.Edges[edge].Owner != player {
		return false
	}
	route := g.Edges[edge]
	adjacency := make([][]int, len(g.Vertices))
	for _, e := range g.Edges {
		if e.Owner == player && e.Ship == route.Ship {
			adjacency[e.A] = append(adjacency[e.A], e.ID)
			adjacency[e.B] = append(adjacency[e.B], e.ID)
		}
	}
	anchors := make([]bool, len(g.Vertices))
	for _, v := range g.Vertices {
		anchors[v.ID] = v.Owner == player && v.Level > 0
		if n := g.knightAt(v.ID); n != nil && n.Owner == player {
			anchors[v.ID] = true
		}
		if route.Ship && g.clothShipAnchor(player, v.ID) {
			anchors[v.ID] = true
		}
	}
	otherEnd := func(id, v int) int {
		e := g.Edges[id]
		if e.A == v {
			return e.B
		}
		return e.A
	}
	// Reject every edge that lies on a simple route between distinct anchors.
	visited := make([]bool, len(g.Vertices))
	var closed func(int, int, bool) bool
	closed = func(v, start int, used bool) bool {
		if v != start && anchors[v] {
			return used
		}
		visited[v] = true
		defer func() { visited[v] = false }()
		for _, id := range adjacency[v] {
			next := otherEnd(id, v)
			if !visited[next] && closed(next, start, used || id == edge) {
				return true
			}
		}
		return false
	}
	for v, anchor := range anchors {
		if anchor && closed(v, v, false) {
			return false
		}
	}
	for _, v := range []int{route.A, route.B} {
		if !anchors[v] && len(adjacency[v]) == 1 {
			return true
		}
	}
	// A cycle is open only if the alternative path has no interior anchor.
	var cycle func(int) bool
	cycle = func(v int) bool {
		if v == route.B {
			return true
		}
		if v != route.A && anchors[v] {
			return false
		}
		visited[v] = true
		defer func() { visited[v] = false }()
		for _, id := range adjacency[v] {
			if id == edge {
				continue
			}
			next := otherEnd(id, v)
			if !visited[next] && cycle(next) {
				return true
			}
		}
		return false
	}
	return cycle(route.A)
}
func (g *Catan) diplomacyRoads() []int {
	out := []int{}
	for _, e := range g.Edges {
		if p := g.pirateIslands(); p != nil && e.Owner >= 0 && e.Ship {
			route := p.Fortresses[e.Owner].Route
			if at := slices.Index(route, e.ID); at >= 0 && at != len(route)-1 {
				continue
			}
		}
		if (g.twoNeutralKnightOwner(e.Owner) || e.Owner >= 0 && !g.Players[e.Owner].Eliminated) && !e.Damaged && !e.Bridge && g.openRoute(e.Owner, e.ID) && g.preservesKnightConnections(e.Owner, e.ID) {
			out = append(out, e.ID)
		}
	}
	return out
}
func (g *Catan) diplomacyPlacements(player int) []int {
	out := []int{}
	if x := g.Explorer; x != nil {
		for _, edge := range g.Edges {
			if _, err := x.Cargo.roadPrice(g, x.Fleet, player, g.TurnSerial, edge.ID, true); err == nil {
				out = append(out, edge.ID)
			}
		}
		return out
	}
	for _, e := range g.Edges {
		ship := g.CitiesKnights != nil && g.CitiesKnights.Pending != nil && g.CitiesKnights.Pending.Kind == "diplomacy" && g.CitiesKnights.Pending.Ship
		if (ship && g.canDiplomacyShip(player, e.ID)) || (!ship && g.canRoad(player, e.ID)) {
			out = append(out, e.ID)
		}
	}
	return out
}
