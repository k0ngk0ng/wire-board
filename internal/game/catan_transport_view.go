package game

import "slices"

// Only the acting player receives resource-dependent choices. Movement quotes
// use the same rule function as Apply; the browser must not guess tolls or MPs.
func (s *State) catanTransportChoices(player int) map[string]any {
	g := s.Catan
	t := g.Transport
	choices := map[string]any{}
	if player < 0 || player != s.Turn || s.Finished || g.Players[player].Eliminated {
		return choices
	}
	if s.Phase == "catan_turn" {
		cost := catanTransportUpgradeCost(t.Wagons[player].Level)
		choices["upgradeCost"] = cost
		choices["canUpgrade"] = cost != nil && catanHas(g.Players[player].Resources, cost)
		buy, sell := []int{}, []int{}
		rates := g.rates(player)
		_, goldErr := catanGoldShortfall(t.GoldBank, t.GoldIssued, 1)
		for c := 0; c < len(g.Bank); c++ {
			if c < 5 && t.Bought < 2 && t.Gold[player] >= 2 && g.Bank[c] > 0 {
				buy = append(buy, c)
			}
			if goldErr == nil && g.Players[player].Resources[c] >= rates[c] {
				sell = append(sell, c)
			}
		}
		choices["buy"], choices["sell"], choices["rates"] = buy, sell, rates
		if g.transportKnights() {
			chases := []map[string]any{}
			for _, knight := range g.CitiesKnights.Knights {
				if knight.Owner == player {
					if pieces := g.transportKnightBarbarians(&knight); len(pieces) > 0 {
						chases = append(chases, map[string]any{"vertex": knight.Vertex, "barbarians": pieces})
					}
				}
			}
			choices["knightChases"] = chases
			targets := []int{}
			if len(chases) > 0 {
				for _, e := range g.Edges {
					if !slices.Contains(t.Barbarians[:], e.ID) {
						targets = append(targets, e.ID)
					}
				}
			}
			choices["knightTargets"] = targets
		}
	}
	q := t.Travel
	if s.Phase == "catan_transport_barbarian" || s.Phase == "catan_transport_move" && q != nil && q.Pending >= 0 {
		edges := []int{}
		for _, e := range g.Edges {
			if !slices.Contains(t.Barbarians[:], e.ID) {
				edges = append(edges, e.ID)
			}
		}
		if g.attackTransport() && q != nil {
			options := []map[string]any{}
			for _, tile := range g.attackBattleTiles() {
				allowed := []int{}
				for _, edge := range g.AttackTransport.Pieces.edges(g, tile, q.Pending) {
					p := clone(g.AttackTransport.Pieces)
					if p.relocate(g, g.attackTransportBoard(), q.Pending, tile, edge) == nil {
						allowed = append(allowed, edge)
					}
				}
				if len(allowed) > 0 {
					options = append(options, map[string]any{"tile": tile, "edges": allowed})
				}
			}
			choices["relocateHexes"] = options
		} else {
			choices["relocate"] = edges
		}
	}
	if s.Phase != "catan_transport_move" || q == nil || q.Ended || q.Pending >= 0 {
		return choices
	}
	steps, drive := []catanTransportStep{}, []int{}
	for _, e := range g.Edges {
		if quote, err := t.quoteTravel(g, *q, e.ID); err == nil {
			steps = append(steps, quote)
		}
	}
	if q.Level > 0 {
		edges := t.Barbarians[:]
		attempted := q.Attempted[:]
		if g.attackTransport() {
			edges = g.AttackTransport.Pieces.blockingEdges()
			attempted = g.AttackTransport.Attempted
		}
		for piece, edge := range edges {
			if edge < 0 {
				continue
			}
			e := g.Edges[edge]
			if !attempted[piece] && (e.A == q.Position || e.B == q.Position) {
				drive = append(drive, piece)
			}
		}
	}
	choices["steps"], choices["drive"] = steps, drive
	choices["canWheat"] = !q.WheatUsed && g.Players[player].Resources[3] > 0
	if g.fishingTransport() {
		cost := g.fishActionCost(player, "catan_transport_fish")
		choices["fishCost"] = cost
		choices["canFish"] = !q.WheatUsed && g.fishPayment(player, cost) != nil
		v, _ := g.Fishing.Tokens.view(player)
		choices["fishTokens"] = v.Players[player].Tokens
	}
	choices["canStop"] = true
	return choices
}
