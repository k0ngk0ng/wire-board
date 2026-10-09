package game

import "slices"

func (g *Catan) pirateHomeVertex(vertex int) bool {
	p := g.pirateIslands()
	if p == nil {
		return false
	}
	for _, id := range p.HomeTiles {
		if slices.Contains(g.Tiles[id].Vertices, vertex) {
			return true
		}
	}
	return false
}
func (g *Catan) pirateHomeCoast(edge int) bool {
	p := g.pirateIslands()
	if p == nil {
		return false
	}
	for _, id := range g.edgeTiles(edge) {
		if slices.Contains(p.HomeTiles, id) {
			return g.edgeTerrain(edge, true)
		}
	}
	return false
}

// Distances use the printed sea/coastal geometry, not an opponent's temporary
// occupancy: a detour must not become legal merely because it blocks a rival.
func (g *Catan) pirateSeaDistances(target int) []int {
	distance := make([]int, len(g.Vertices))
	for i := range distance {
		distance[i] = -1
	}
	distance[target] = 0
	queue := []int{target}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for _, id := range g.touching(v) {
			if !g.edgeTerrain(id, true) {
				continue
			}
			e := g.Edges[id]
			to := e.A
			if to == v {
				to = e.B
			}
			if distance[to] < 0 {
				distance[to] = distance[v] + 1
				queue = append(queue, to)
			}
		}
	}
	return distance
}
func (g *Catan) pirateRouteVertices(player int) ([]int, bool) {
	p := g.pirateIslands()
	if p == nil || player < 0 || player >= len(p.Fortresses) {
		return nil, false
	}
	f := p.Fortresses[player]
	if f.Root < 0 || f.Root >= len(g.Vertices) {
		return nil, false
	}
	result := []int{f.Root}
	v := f.Root
	for _, id := range f.Route {
		if id < 0 || id >= len(g.Edges) {
			return nil, false
		}
		e := g.Edges[id]
		if e.Owner != player || !e.Ship {
			return nil, false
		}
		to := e.A
		if to == v {
			to = e.B
		} else if e.B != v {
			return nil, false
		}
		if slices.Contains(result, to) {
			return nil, false
		}
		result = append(result, to)
		v = to
	}
	return result, true
}

// Plan the expedition prefix without mutation. Coastal auxiliary routes are
// allowed; all ships beyond the main coast must extend the one stored line.
func (g *Catan) pirateShipPlan(player, edge int) (int, []int, bool) {
	p := g.pirateIslands()
	if p == nil {
		return -1, nil, true
	}
	if g.attackPirates() && player >= 0 && player < len(p.Fortresses) && p.Fortresses[player].Strength == 0 {
		return -1, nil, false
	}
	if g.twoAttackSea() && player < 0 {
		return -1, nil, g.pirateHomeCoast(edge)
	}
	if player < 0 || player >= len(p.Fortresses) || edge < 0 || edge >= len(g.Edges) {
		return -1, nil, false
	}
	f := p.Fortresses[player]
	vertices, valid := g.pirateRouteVertices(player)
	if !valid {
		return -1, nil, false
	}
	end := vertices[len(vertices)-1]
	target := f.Beachhead
	if slices.Contains(vertices, target) {
		target = f.Vertex
	}
	if end != f.Vertex && !slices.Contains(f.Route, edge) && (g.Vertices[end].Level == 0 || g.Vertices[end].Owner == player) {
		e := g.Edges[edge]
		next := -1
		if e.A == end {
			next = e.B
		} else if e.B == end {
			next = e.A
		}
		if next >= 0 && !slices.Contains(vertices, next) {
			dist := g.pirateSeaDistances(target)
			if dist[end] > 0 && dist[next] == dist[end]-1 {
				return f.Root, append(append([]int{}, f.Route...), edge), true
			}
		}
	}
	if g.pirateHomeCoast(edge) {
		return f.Root, append([]int{}, f.Route...), true
	}
	// A player may depart from a different coastal building before committing
	// any offshore ship on the existing expedition.
	for _, id := range f.Route {
		if !g.pirateHomeCoast(id) {
			return -1, nil, false
		}
	}
	dist := g.pirateSeaDistances(f.Beachhead)
	for _, root := range g.Vertices {
		if root.Owner != player || root.Level == 0 || !g.pirateHomeVertex(root.ID) {
			continue
		}
		var search func(int, []int) ([]int, bool)
		search = func(v int, path []int) ([]int, bool) {
			if dist[v] <= 0 {
				return nil, false
			}
			for _, id := range g.touching(v) {
				e := g.Edges[id]
				if id != edge && (e.Owner != player || !e.Ship || !g.pirateHomeCoast(id)) {
					continue
				}
				to := e.A
				if to == v {
					to = e.B
				}
				if dist[to] != dist[v]-1 || (g.Vertices[to].Level > 0 && g.Vertices[to].Owner != player) {
					continue
				}
				route := append(append([]int{}, path...), id)
				if id == edge {
					return route, true
				}
				if route, ok := search(to, route); ok {
					return route, true
				}
			}
			return nil, false
		}
		if route, ok := search(root.ID, nil); ok {
			return root.ID, route, true
		}
	}
	return -1, nil, false
}
func (g *Catan) pirateCommitShip(player, edge int) bool {
	if g.pirateIslands() == nil || g.twoAttackSea() && player < 0 {
		return true
	}
	root, route, ok := g.pirateShipPlan(player, edge)
	if !ok {
		return false
	}
	f := &g.pirateIslands().Fortresses[player]
	f.Root, f.Route = root, route
	return true
}
func (g *Catan) pirateRemoveRouteTail(player, edge int) bool {
	p := g.pirateIslands()
	if p == nil {
		return true
	}
	f := &p.Fortresses[player]
	at := slices.Index(f.Route, edge)
	if at < 0 {
		return true
	}
	if at != len(f.Route)-1 {
		return false
	}
	f.Route = append([]int{}, f.Route[:at]...)
	return true
}
func (g *Catan) pirateSettlementAllowed(player, vertex int, hypothetical bool) bool {
	p := g.pirateIslands()
	if p == nil {
		return true
	}
	if g.pirateHomeVertex(vertex) {
		return true
	}
	if g.setup() || player < 0 || player >= len(p.Fortresses) || vertex != p.Fortresses[player].Beachhead {
		return false
	}
	if hypothetical {
		return true
	}
	vertices, ok := g.pirateRouteVertices(player)
	return ok && slices.Contains(vertices, vertex)
}
func (g *Catan) pirateNextWarship(player int) int {
	p := g.pirateIslands()
	if p == nil || player < 0 || player >= len(p.Fortresses) || g.attackPirates() && p.Fortresses[player].Strength == 0 {
		return -1
	}
	if _, valid := g.pirateRouteVertices(player); !valid {
		return -1
	}
	for _, id := range p.Fortresses[player].Route {
		if !g.Edges[id].Warship {
			return id
		}
	}
	return -1
}
