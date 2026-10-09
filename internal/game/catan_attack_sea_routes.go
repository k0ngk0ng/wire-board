package game

// Seafarers + Barbarian Attack, 2025, General: new/modified routes
// must remain connected to an unconquered own building. Conquered land
// prohibits roads but not ships. Route modes join only at own buildings.
func (g *Catan) attackSeaRouteAnchored(player, edge int, ship bool) bool {
	if g.Attack == nil || g.Seafarers == nil {
		return true
	}
	type node struct {
		vertex int
		ship   bool
	}
	e := g.Edges[edge]
	queue := []node{{e.A, ship}, {e.B, ship}}
	seen := map[node]bool{}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		if seen[v] {
			continue
		}
		seen[v] = true
		if g.opponentPiece(player, v.vertex) {
			continue
		}
		b := g.Vertices[v.vertex]
		if b.Level > 0 {
			if b.Owner != player {
				continue
			}
			if !g.Attack.conqueredBuilding(g, v.vertex) {
				return true
			}
		}
		for _, id := range g.touching(v.vertex) {
			r := g.Edges[id]
			if r.Owner != player || r.Ship != v.ship && (b.Level == 0 || b.Owner != player) {
				continue
			}
			to := r.A
			if to == v.vertex {
				to = r.B
			}
			queue = append(queue, node{to, r.Ship})
		}
	}
	return false
}
