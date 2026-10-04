package game

import "slices"

// Price a route by the reachable public settlement sites, without reading
// opponents' hands or unrevealed terrain. Roads and ships cannot swap mode at
// an empty intersection; a building we already own permits that transition.
func (g *Catan) seaRouteValue(player, edge int, ship bool) int {
	const unreachable = 100000
	dist := make([][2]int, len(g.Vertices))
	for i := range dist {
		dist[i] = [2]int{unreachable, unreachable}
	}
	mode := 0
	if ship {
		mode = 1
	}
	e := g.Edges[edge]
	dist[e.A][mode] = 0
	dist[e.B][mode] = 0
	for pass := 0; pass < len(g.Vertices)*2; pass++ {
		changed := false
		for _, route := range g.Edges {
			if route.Owner >= 0 && route.Owner != player {
				continue
			}
			for m := 0; m < 2; m++ {
				isShip := m == 1
				if !g.edgeTerrain(route.ID, isShip) || (isShip && g.pirateBlocks(route.ID)) || (route.Owner == player && route.Ship != isShip) {
					continue
				}
				cost := 1
				if route.Owner == player || route.ID == edge {
					cost = 0
				}
				for _, ends := range [][2]int{{route.A, route.B}, {route.B, route.A}} {
					a, b := ends[0], ends[1]
					if g.Vertices[a].Owner >= 0 && g.Vertices[a].Owner != player {
						continue
					}
					from := dist[a][m]
					if g.Vertices[a].Owner == player && g.Vertices[a].Level > 0 {
						from = min(from, dist[a][1-m])
					}
					if from+cost < dist[b][m] {
						dist[b][m] = from + cost
						changed = true
					}
				}
			}
		}
		if !changed {
			break
		}
	}
	score := -1000
	if c := g.cloth(); c != nil {
		for _, village := range c.Villages {
			if village.Stock <= 0 || slices.Contains(village.Traders, player) {
				continue
			}
			if distance := dist[village.Vertex][1]; distance < unreachable {
				score = max(score, 350+(6-absCatan(7-village.Number))*15+min(village.Stock, 3)*60-distance*95)
			}
		}
	}
	for _, v := range g.Vertices {
		if !g.canSettlement(player, v.ID, true) {
			continue
		}
		distance := min(dist[v.ID][0], dist[v.ID][1])
		if distance < unreachable {
			score = max(score, g.vertexValue(player, v.ID)-distance*95)
		}
	}
	if g.Seafarers != nil && g.Seafarers.Fog != nil {
		for _, t := range g.Tiles {
			if t.Resource != CatanFog {
				continue
			}
			for _, v := range t.Vertices {
				distance := min(dist[v][0], dist[v][1])
				if distance < unreachable {
					score = max(score, 200-distance*95)
				}
			}
		}
	}
	if t := g.tribe(); t != nil {
		rewards := map[int]int{}
		for _, id := range t.Tokens {
			rewards[id] += 300
		}
		for _, card := range t.Development {
			rewards[card.Edge] += 240
		}
		for _, port := range t.Ports {
			rewards[port.Edge] += 200
		}
		for id, value := range rewards {
			target := g.Edges[id]
			if target.Owner >= 0 || g.pirateBlocks(id) {
				continue
			}
			distance := min(dist[target.A][1], dist[target.B][1])
			if id != edge {
				distance++
			}
			if distance < unreachable {
				score = max(score, value-distance*95)
			}
		}
	}
	return score
}
func (g *Catan) seaBuildChoices(player int) []botChoice {
	result := []botChoice{}
	roads, _, _ := g.pieces(player)
	ships := g.shipCount(player)
	for _, e := range g.Edges {
		if roads < 15 && g.canRoad(player, e.ID) {
			result = append(result, botChoice{Action{Type: "catan_road", Edge: e.ID}, 200 + g.seaRouteValue(player, e.ID, false)/4})
		}
		if ships < 15 && g.canShip(player, e.ID) {
			if p := g.pirateIslands(); p != nil {
				root, route, ok := g.pirateShipPlan(player, e.ID)
				f := p.Fortresses[player]
				if ok && (len(route) > len(f.Route) || root != f.Root) {
					result = append(result, botChoice{Action{Type: "catan_ship", Edge: e.ID}, 450})
				} else {
					// A free Road Building placement must still include auxiliary
					// coastal ships when no expedition extension remains possible.
					result = append(result, botChoice{Action{Type: "catan_ship", Edge: e.ID}, -1500})
				}
				continue
			}
			result = append(result, botChoice{Action{Type: "catan_ship", Edge: e.ID}, 200 + g.seaRouteValue(player, e.ID, true)/4})
		}
	}
	return result
}
func (g *Catan) seaMoveChoices(player int) []botChoice {
	choices := []botChoice{}
	if g.Seafarers == nil || g.Seafarers.MovedShip || g.pirateIslands() != nil {
		return choices
	}
	for _, from := range g.Edges {
		destinations := g.shipDestinations(player, from.ID)
		if len(destinations) == 0 {
			continue
		}
		before := g.seaRouteValue(player, from.ID, true)
		temp := *g
		temp.Edges = append([]CatanEdge{}, g.Edges...)
		temp.Edges[from.ID].Owner = -1
		temp.Edges[from.ID].Ship = false
		for _, to := range destinations {
			improvement := temp.seaRouteValue(player, to, true) - before
			if improvement > 0 {
				choices = append(choices, botChoice{Action{Type: "catan_move_ship", Edge: from.ID, Target: to}, 250 + improvement/4})
			}
		}
	}
	return choices
}
func (g *Catan) pirateBotChoices(player int) []botChoice {
	choices := []botChoice{}
	if !g.pirateAllowed(player) {
		return choices
	}
	for _, tile := range g.Tiles {
		if tile.Resource != CatanSea || tile.ID == g.Seafarers.Pirate {
			continue
		}
		value := 0
		for _, edge := range g.Edges {
			if !edge.Ship || edge.Owner < 0 {
				continue
			}
			touches := false
			for _, id := range g.edgeTiles(edge.ID) {
				touches = touches || id == tile.ID
			}
			if !touches {
				continue
			}
			if edge.Owner == player {
				value -= 12
			} else if !g.Players[edge.Owner].Eliminated {
				value += 5
			}
		}
		choices = append(choices, botChoice{Action{Type: "catan_pirate", Tile: tile.ID}, value})
	}
	if g.Seafarers.Pirate >= 0 {
		choices = append(choices, botChoice{Action{Type: "catan_pirate", Tile: -1}, -1})
	}
	return choices
}
