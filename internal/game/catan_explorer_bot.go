package game

import (
	"errors"
	"slices"
)

// Geometry-only queries used by the private bot. They inspect visible tiles,
// roads and buildings, never hidden terrain/number stacks or opponents' hands.
func catanExplorerBotSite(g *Catan, player, vertex int, roadRequired bool) bool {
	if !catanExplorerLandVertex(g, vertex) || g.Explorer != nil && !g.Explorer.Cargo.landVertex(g, player, vertex) || g.Vertices[vertex].Level != 0 || g.knightAt(vertex) != nil {
		return false
	}
	connected := false
	if g.settlementPiecesLeft(player) <= 0 {
		return false
	}
	for _, edge := range g.Edges {
		other := -1
		if edge.A == vertex {
			other = edge.B
		} else if edge.B == vertex {
			other = edge.A
		}
		if other < 0 {
			continue
		}
		if g.Vertices[other].Level > 0 {
			return false
		}
		connected = connected || edge.Owner == player
	}
	return !roadRequired || connected
}

func catanExplorerBotRoad(g *Catan, player int) int {
	n := len(g.Vertices)
	dist, previous := make([]int, n), make([]int, n)
	used := make([]bool, n)
	for i := range dist {
		dist[i], previous[i] = 10000, -1
	}
	for _, v := range g.Vertices {
		if v.Owner == player && v.Level > 0 {
			dist[v.ID] = 0
		}
	}
	count := 0
	for _, e := range g.Edges {
		if e.Owner == player {
			count++
			for _, v := range []int{e.A, e.B} {
				if !g.opponentPiece(player, v) {
					dist[v] = 0
				}
			}
		}
	}
	if count >= 15 {
		return -1
	}
	for step := 0; step < n; step++ {
		at := -1
		for v := range dist {
			if !used[v] && (at < 0 || dist[v] < dist[at]) {
				at = v
			}
		}
		if at < 0 || dist[at] >= 10000 {
			break
		}
		used[at] = true
		for _, e := range g.Edges {
			if !catanExplorerLandEdge(g, e.ID) || g.Explorer != nil && !g.Explorer.Cargo.landEdge(g, player, e.ID) || e.Owner != -1 && e.Owner != player {
				continue
			}
			to := -1
			if e.A == at {
				to = e.B
			} else if e.B == at {
				to = e.A
			}
			if to < 0 {
				continue
			}
			if g.opponentPiece(player, to) {
				continue
			}
			cost := 1
			if e.Owner == player {
				cost = 0
			}
			if dist[at]+cost < dist[to] {
				dist[to], previous[to] = dist[at]+cost, e.ID
			}
		}
	}
	goal, best := -1, -100000
	for v, d := range dist {
		if d < 1 || d > 3 || !catanExplorerBotSite(g, player, v, false) {
			continue
		}
		value := g.vertexValue(player, v) - 80*d
		if value > best {
			best, goal = value, v
		}
	}
	if goal < 0 {
		return -1
	}
	first := -1
	for previous[goal] >= 0 {
		e := g.Edges[previous[goal]]
		if e.Owner == -1 {
			first = e.ID
		}
		if e.A == goal {
			goal = e.B
		} else {
			goal = e.A
		}
	}
	return first
}

func catanExplorerBotVoyage(g *Catan, player, ship int) []int {
	x := g.Explorer
	from := x.Fleet.Positions[ship]
	queue, seen := [][]int{{from}}, map[int]bool{from: true}
	var bestPath []int
	best := -100000
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		last := path[len(path)-1]
		edge := g.Edges[last]
		fog := false
		for _, tile := range g.Tiles {
			if tile.Resource == CatanFog && catanExplorerTouches(g, last, tile.ID) {
				fog = true
				break
			}
		}
		others := 0
		for id, position := range x.Fleet.Positions {
			if id != ship && position == last {
				others++
			}
		}
		if len(path) > 1 && others < 2 {
			value := -100000
			if fog {
				value = 500 - 15*len(path)
			}
			for _, vertex := range []int{edge.A, edge.B} {
				if catanExplorerBotSite(g, player, vertex, false) {
					value = max(value, 1000+g.vertexValue(player, vertex)-15*len(path))
				}
			}
			if value > best {
				best, bestPath = value, slices.Clone(path[1:])
			}
		}
		if fog || len(path) > 14 {
			continue
		}
		for _, to := range g.Edges {
			if !seen[to.ID] && catanExplorerSeaEdge(g, to.ID) && catanExplorerAdjacentEdges(edge, to) {
				seen[to.ID] = true
				queue = append(queue, append(slices.Clone(path), to.ID))
			}
		}
	}
	if len(bestPath) == 0 {
		return nil
	}
	limit := min(len(bestPath), x.Fleet.Turn.Ships[ship].Remaining)
	// Passing occupied edges is allowed, but stop before the point budget runs
	// out on an edge holding two other ships. Quote also guards actual topology.
	pirateOwner, pirateTile := -1, -1
	if x.Pirate != nil {
		pirateOwner, pirateTile = x.Pirate.Owner, x.Pirate.Tile
	}
	for limit > 0 {
		if quote, err := x.Fleet.quote(g, player, g.TurnSerial, ship, bestPath[:limit], pirateOwner, pirateTile); err == nil && quote.Gold <= x.Economy.Gold[player] {
			return bestPath[:limit]
		}
		limit--
	}
	return nil
}

func (s *State) catanExplorerBot(player int) (Action, error) {
	g := s.Catan
	x := g.Explorer
	if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || s.Finished {
		return Action{}, errors.New("inactive explorer seat")
	}
	a := Action{Prompt: int(g.TurnSerial)}
	if g.Fishing != nil && g.Fishing.Pending != nil {
		choice, err := s.catanFishBot(player)
		choice.Prompt = a.Prompt
		return choice, err
	}
	if k := g.CitiesKnights; k != nil && k.Pending != nil {
		choice, err := s.catanCityChoiceBot(player)
		choice.Prompt = a.Prompt
		return choice, err
	}
	if s.Phase == "catan_discard" && g.DiscardDue[player] > 0 {
		a.Type = "catan_discard"
		a.Tokens = make([]int, len(g.Bank))
		hand := slices.Clone(g.Players[player].Resources)
		for i := 0; i < g.DiscardDue[player]; i++ {
			most := 0
			for r := range hand {
				if hand[r] > hand[most] {
					most = r
				}
			}
			hand[most]--
			a.Tokens[most]++
		}
		return a, nil
	}
	if player != s.Turn {
		if s.Phase == "catan_turn" && g.Trade != nil {
			t := g.Trade
			a.Offer = t.ID
			a.Type = "catan_trade_reject"
			if catanHas(g.Players[player].Resources, t.Take) && g.hasTradeGold(player, t.GoldTake) && sum(t.Give)+t.GoldGive >= sum(t.Take)+t.GoldTake {
				a.Type = "catan_trade_accept"
			}
			return a, nil
		}
		return a, errors.New("not explorer turn")
	}
	if actions, handled := s.catanExplorerSpecialChoices(player); handled {
		if len(actions) == 0 {
			return a, errors.New("no explorer response")
		}
		if x.Setup != nil && x.Setup.CitiesKnights {
			if target, ok := x.Setup.botOpening(g, x.Board, x.Fleet, 4096); ok {
				for _, choice := range actions {
					if choice.Target == target {
						return choice, nil
					}
				}
			}
		}
		return actions[0], nil
	}
	if choice, ok := s.catanExplorerFishingBot(player); ok {
		return choice, nil
	}
	switch s.Phase {
	case "catan_roll":
		if k := g.CitiesKnights; k != nil && slices.Contains(k.Players[player].Progress, 0) {
			a.Type, a.Card, a.Tokens = "catan_progress", 0, g.alchemyBotDice(player)
			return a, nil
		}
		a.Type = "catan_roll"
		return a, nil
	case "catan_roads":
		if g.CitiesKnights != nil {
			a.Type = "catan_skip_roads"
			if sites := s.catanExplorerFreeRoadSites(player); len(sites) > 0 {
				a.Type, a.Edge = "catan_road", sites[0]
			}
			return a, nil
		}
	case "catan_turn":
		if g.CitiesKnights != nil {
			if choice, ok := s.catanExplorerCityProgressBot(player); ok {
				return choice, nil
			}
		}
		plans := s.catanExplorerMissionPlans(player)
		plans = append(plans, s.catanExplorerCityBotPlans(player)...)
		harbors := 0
		for _, v := range g.Vertices {
			if v.Owner == player && catanExplorerHarborAt(g, v.ID) {
				harbors++
			}
		}
		for _, v := range g.Vertices {
			if harbors < 4 && v.Owner == player && v.Level == 1 && catanExplorerCoast(g, v.ID) && (g.CitiesKnights == nil || !slices.Contains(g.CitiesKnights.FallenCities, v.ID)) {
				plans = append(plans, catanExplorerBotPlan{Action{Type: "catan_explorer_harbor", Vertex: v.ID, Prompt: a.Prompt}, []int{0, 0, 0, 2, 2}, 110})
			}
			if catanExplorerBotSite(g, player, v.ID, true) {
				plans = append(plans, catanExplorerBotPlan{Action{Type: "catan_settlement", Vertex: v.ID, Prompt: a.Prompt}, []int{1, 1, 1, 1, 0}, 130 + g.vertexValue(player, v.ID)/10})
			}
		}
		if road := catanExplorerBotRoad(g, player); road >= 0 {
			plans = append(plans, catanExplorerBotPlan{Action{Type: "catan_road", Edge: road, Prompt: a.Prompt}, []int{1, 1, 0, 0, 0}, 60})
		}
		best := -1
		value := -100000
		for i, p := range plans {
			// Mission costs have five resources; city costs also reserve commodities.
			cost := make([]int, len(g.Bank))
			copy(cost, p.cost)
			plans[i].cost = cost
			score := p.value
			for r, n := range p.cost {
				score -= 20 * max(0, n-g.Players[player].Resources[r])
			}
			if catanHas(g.Players[player].Resources, p.cost) {
				score += 1000
			}
			if score > value {
				value, best = score, i
			}
		}
		if best >= 0 {
			p := plans[best]
			hand := g.Players[player].Resources
			if catanHas(hand, p.cost) {
				return p.action, nil
			}
			rates := make([]int, len(g.Bank))
			for r := range rates {
				rates[r] = 3
			}
			if g.CitiesKnights != nil {
				rates = g.explorerCityRates(player)
			}
			for missing, n := range p.cost {
				if hand[missing] >= n || g.Bank[missing] == 0 {
					continue
				}
				if missing < 5 && x.Economy.Gold[player] >= 2 && x.Economy.Turn.Bought < 2 {
					a.Type, a.Color, a.Target = "catan_explorer_bank", -1, missing
					return a, nil
				}
				for surplus, count := range hand {
					if surplus != missing && count-p.cost[surplus] >= rates[surplus] {
						a.Type, a.Color, a.Target = "catan_explorer_bank", surplus, missing
						return a, nil
					}
				}
			}
			if action, ok := s.catanExplorerSpiceBotGold(player, p.cost); ok {
				return action, nil
			}
		}
		a.Type = "catan_explorer_begin_move"
		return a, nil
	case "catan_explorer_move":
		if action, ok := s.catanExplorerMissionCargo(player); ok {
			return action, nil
		}
		for ship, edge := range x.Fleet.Positions {
			if ship/3 != player || edge < 0 {
				continue
			}
			units := x.Cargo.contents(catanExplorerCargoLocation{"ship", ship})
			if len(units) != 1 || units[0]%11 >= 2 {
				continue
			}
			for _, v := range []int{g.Edges[edge].A, g.Edges[edge].B} {
				if catanExplorerBotSite(g, player, v, false) {
					a.Type, a.Slot, a.Vertex = "catan_explorer_settle", ship, v
					return a, nil
				}
			}
		}
		for ship, edge := range x.Fleet.Positions {
			if ship/3 != player || edge < 0 || x.Fleet.Turn.Ships[ship].Closed || x.Fleet.Turn.Ships[ship].Remaining == 0 {
				continue
			}
			units := x.Cargo.contents(catanExplorerCargoLocation{"ship", ship})
			if x.Lairs == nil && x.Spice == nil && (len(units) != 1 || units[0]%11 >= 2) {
				continue
			}
			var path []int
			if x.Lairs != nil || x.Spice != nil {
				path = catanExplorerMissionVoyage(g, player, ship)
			} else {
				path = catanExplorerBotVoyage(g, player, ship)
			}
			if len(path) > 0 {
				a.Type, a.Slot, a.Targets = "catan_explorer_sail", ship, path
				return a, nil
			}
		}
		a.Type = "catan_end"
		return a, nil
	}
	return a, errors.New("no explorer action in this phase")
}
