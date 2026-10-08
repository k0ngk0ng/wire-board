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
		choices["relocate"] = edges
	}
	if s.Phase != "catan_transport_move" || q == nil || q.Ended || q.Pending >= 0 {
		return choices
	}
	steps, drive := []catanTransportStep{}, []int{}
	for _, e := range g.Edges {
		if quote, err := q.quote(g, t.Map, t.Barbarians, t.Gold, e.ID); err == nil {
			steps = append(steps, quote)
		}
	}
	if q.Level > 0 {
		for piece, edge := range t.Barbarians {
			e := g.Edges[edge]
			if !q.Attempted[piece] && (e.A == q.Position || e.B == q.Position) {
				drive = append(drive, piece)
			}
		}
	}
	choices["steps"], choices["drive"] = steps, drive
	choices["canWheat"] = !q.WheatUsed && g.Players[player].Resources[3] > 0
	choices["canStop"] = true
	return choices
}
