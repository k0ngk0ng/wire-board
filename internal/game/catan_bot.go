package game

import (
	"errors"
	"slices"
)

// Catan AI evaluates public production odds and its own hand, never the next
// development card or another player's resource composition.
func (g *Catan) vertexValue(p, v int) int {
	values := []int{0, 0, 0, 0, 0}
	goldValue := 0
	for _, t := range g.Tiles {
		if t.Number <= 0 {
			continue
		}
		if t.Resource == CatanGold {
			for _, id := range t.Vertices {
				if id == v {
					goldValue += (6 - absCatan(7-t.Number)) * 14
				}
			}
		}
		if t.Resource >= 5 {
			continue
		}
		for _, id := range t.Vertices {
			if id == v {
				values[t.Resource] += 6 - absCatan(7-t.Number)
			}
		}
	}
	existing := make([]int, 5)
	for _, u := range g.Vertices {
		if u.Owner != p {
			continue
		}
		for _, t := range g.Tiles {
			if t.Number <= 0 {
				continue
			}
			if t.Resource >= 5 {
				continue
			}
			for _, id := range t.Vertices {
				if id == u.ID {
					existing[t.Resource] += 6 - absCatan(7-t.Number)
				}
			}
		}
	}
	score := goldValue + g.wonderVertexValue(p, v)
	if g.Seafarers != nil && !g.setup() && p < len(g.Seafarers.Seats) {
		island := g.islandAt(v)
		if island >= 0 && !slices.Contains(g.Seafarers.Seats[p].SettledIslands, island) {
			score += g.Seafarers.IslandBonus * 80
		}
	}
	for c, n := range values {
		score += n * 10
		if n > 0 && existing[c] == 0 {
			score += 20
		}
		if c == 3 || c == 4 {
			score += n * 3
		}
	}
	return score
}
func absCatan(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
func (s *State) catanBot(player int) (Action, error) {
	g := s.Catan
	if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || s.Finished {
		return Action{}, errors.New("inactive bot seat")
	}
	p := g.Players[player]
	if fleet := g.pirateIslands(); fleet != nil && fleet.Raid != nil {
		return s.catanFleetRewardBot(player)
	}
	if t := g.tribe(); t != nil && t.Pending != nil {
		return s.catanTribePortBot(player)
	}
	if g.HelperPending != nil {
		return s.catanHelperPendingBot(player)
	}
	if g.GoldPending != nil {
		return s.catanGoldBot(player)
	}
	if s.Phase == "catan_discard" && g.DiscardDue[player] > 0 {
		hand := append([]int{}, p.Resources...)
		give := make([]int, 5)
		for range g.DiscardDue[player] {
			best := 0
			for c := range hand {
				if hand[c] > hand[best] {
					best = c
				}
			}
			give[best]++
			hand[best]--
		}
		return Action{Type: "catan_discard", Tokens: give}, nil
	}
	if g.Trade != nil && player != s.Turn && g.Trade.Responses[player] == 0 {
		a := Action{Type: "catan_trade_reject", Offer: g.Trade.ID}
		if catanHas(p.Resources, g.Trade.Take) && sum(g.Trade.Give) >= sum(g.Trade.Take) {
			a.Type = "catan_trade_accept"
		}
		return a, nil
	}
	if player != s.Turn {
		return Action{}, errors.New("inactive bot seat")
	}
	bestVertex := func(setup bool) int {
		best, score := -1, -1
		for _, v := range g.Vertices {
			if g.canSettlement(player, v.ID, setup) {
				n := g.vertexValue(player, v.ID)
				if n > score {
					best, score = v.ID, n
				}
			}
		}
		return best
	}
	switch s.Phase {
	case "catan_world_ports":
		return s.catanWorldPortBot(player)
	case "catan_cloth_start", "catan_wonders_start":
		return Action{Type: s.Phase, Tile: g.Robber}, nil
	case "catan_setup_settlement":
		return Action{Type: "catan_settlement", Vertex: bestVertex(true)}, nil
	case "catan_setup_road":
		best, score := -1, -1
		for _, e := range g.Edges {
			if g.setupRoute(player, e.ID, false) {
				next := e.A
				if next == g.SetupVertex {
					next = e.B
				}
				n := g.vertexValue(player, next)
				if n > score {
					best, score = e.ID, n
				}
			}
		}
		if g.Seafarers != nil {
			choices := []botChoice{}
			if best >= 0 {
				choices = append(choices, botChoice{Action{Type: "catan_road", Edge: best}, score})
			}
			for _, e := range g.Edges {
				if g.setupRoute(player, e.ID, true) {
					choices = append(choices, botChoice{Action{Type: "catan_ship", Edge: e.ID}, max(g.vertexValue(player, e.A), g.vertexValue(player, e.B))})
				}
			}
			return s.botLegal(player, choices)
		}
		return Action{Type: "catan_road", Edge: best}, nil
	case "catan_roll":
		if g.pirateIslands() != nil && !g.PlayedDev && g.pirateNextWarship(player) >= 0 {
			for _, kind := range []int{0, 4} {
				if p.Dev[kind] > p.NewDev[kind] {
					return Action{Type: "catan_dev", Card: kind}, nil
				}
			}
		}
		if a, ok := g.digurBotAction(player); ok {
			return a, nil
		}
		return Action{Type: "catan_roll"}, nil
	case "catan_robber":
		choices := g.pirateBotChoices(player)
		best, score := -1, -999
		for _, t := range g.Tiles {
			if !g.robberAllowed(t.ID) {
				continue
			}
			value := 0
			for _, id := range t.Vertices {
				v := g.Vertices[id]
				if v.Level == 0 || v.Owner < 0 {
					continue
				}
				n := v.Level * (6 - absCatan(7-t.Number))
				if v.Owner == player {
					value -= n * 4
				} else if !g.Players[v.Owner].Eliminated {
					value += n
				}
			}
			if value > score {
				best, score = t.ID, value
			}
		}
		if best >= 0 {
			choices = append(choices, botChoice{Action{Type: "catan_robber", Tile: best}, score})
		}
		return s.botLegal(player, choices)
	case "catan_cloth_steal":
		return s.catanClothStealBot(player)
	case "catan_steal":
		best := g.Victims[0]
		for _, i := range g.Victims {
			if sum(g.Players[i].Resources) > sum(g.Players[best].Resources) {
				best = i
			}
		}
		return Action{Type: "catan_steal", Target: best}, nil
	}
	roads, settlements, cities := g.pieces(player)
	// Choose a reachable vacant intersection by distance from the current network.
	road, roadScore := -1, -99999
	for _, e := range g.Edges {
		if g.Seafarers != nil || !g.canRoad(player, e.ID) {
			continue
		}
		score := -9999
		// Public graph BFS, treating our roads as free and opponents as blocked.
		for _, goal := range g.Vertices {
			if !g.canSettlement(player, goal.ID, true) {
				continue
			}
			dist := make([]int, len(g.Vertices))
			for i := range dist {
				dist[i] = 999
			}
			dist[e.A] = 0
			dist[e.B] = 0
			for pass := 0; pass < len(g.Vertices); pass++ {
				changed := false
				for _, edge := range g.Edges {
					if edge.Owner >= 0 && edge.Owner != player {
						continue
					}
					cost := 1
					if edge.Owner == player {
						cost = 0
					}
					for _, pair := range [][2]int{{edge.A, edge.B}, {edge.B, edge.A}} {
						a, b := pair[0], pair[1]
						if g.Vertices[a].Level > 0 && g.Vertices[a].Owner != player {
							continue
						}
						if dist[a]+cost < dist[b] {
							dist[b] = dist[a] + cost
							changed = true
						}
					}
				}
				if !changed {
					break
				}
			}
			score = max(score, g.vertexValue(player, goal.ID)-dist[goal.ID]*95)
		}
		if score > roadScore {
			road, roadScore = e.ID, score
		}
	}
	if s.Phase == "catan_roads" {
		if g.Seafarers != nil {
			choices := g.seaBuildChoices(player)
			choices = append(choices, botChoice{Action{Type: "catan_skip_roads"}, -99999})
			return s.botLegal(player, choices)
		}
		if road >= 0 && roads < 15 {
			return Action{Type: "catan_road", Edge: road}, nil
		}
		return Action{Type: "catan_skip_roads"}, nil
	}
	if s.Phase != "catan_turn" {
		return Action{}, errors.New("no bot action in phase")
	}
	choices := []botChoice{}
	choices = append(choices, g.wonderBotChoices(player)...)
	if g.Seafarers != nil {
		choices = append(choices, g.seaBuildChoices(player)...)
		choices = append(choices, g.seaMoveChoices(player)...)
	}
	bestCity, cityScore := -1, -1
	for _, v := range g.Vertices {
		if v.Owner == player && v.Level == 1 {
			n := g.vertexValue(player, v.ID)
			if n > cityScore {
				bestCity, cityScore = v.ID, n
			}
		}
	}
	if bestCity >= 0 && cities < 4 {
		choices = append(choices, botChoice{Action{Type: "catan_city", Vertex: bestCity}, 600})
	}
	if v := bestVertex(false); v >= 0 && settlements < 5 {
		choices = append(choices, botChoice{Action{Type: "catan_settlement", Vertex: v}, 620})
	}
	if road >= 0 && roads < 15 && (settlements < 5 || g.LongestOwner != player) {
		choices = append(choices, botChoice{Action{Type: "catan_road", Edge: road}, 200})
	}
	if len(g.DevDeck) > 0 {
		priority := 100
		if g.pirateIslands() != nil && g.pirateNextWarship(player) >= 0 && sum(p.Dev) < 2 {
			priority = 540
		}
		choices = append(choices, botChoice{Action{Type: "catan_buy_dev"}, priority})
	}
	choices = append(choices, s.catanHelperBotChoices(player, choices, road)...)
	// Trade only toward an immediately useful build, with a strictly smaller deficit.
	targets := append([]botChoice{}, choices...)
	for _, target := range targets {
		cost := catanBotBuildCost(target.action)
		for want, n := range cost {
			if n <= p.Resources[want] || g.Bank[want] == 0 {
				continue
			}
			for give, rate := range g.rates(player) {
				if give == want || p.Resources[give]-cost[give] < rate {
					continue
				}
				a := Action{Type: "catan_bank", Give: make([]int, 5), Take: make([]int, 5)}
				a.Give[give] = rate
				a.Take[want] = 1
				choices = append(choices, botChoice{a, 50 + target.score/20})
			}
		}
	}
	if !g.PlayedDev {
		maxKind := 3
		if g.pirateIslands() != nil {
			maxKind = 4
		}
		for kind := 0; kind <= maxKind; kind++ {
			if p.Dev[kind]-p.NewDev[kind] <= 0 {
				continue
			}
			a := Action{Type: "catan_dev", Card: kind}
			if kind == 2 {
				a.Take = make([]int, 5)
				bank := append([]int{}, g.Bank...)
				for range min(2, sum(bank)) {
					best, priority := -1, -999
					for c, n := range bank {
						if n == 0 {
							continue
						}
						score := 6 - p.Resources[c]
						if c == 3 || c == 4 {
							score += 2
						}
						if score > priority {
							best, priority = c, score
						}
					}
					a.Take[best]++
					bank[best]--
				}
			}
			if kind == 3 { // Decide from visible production potential, not hidden hands.
				a.Color = 3
				production := make([]int, 5)
				for _, t := range g.Tiles {
					if t.Resource >= 5 {
						continue
					}
					for _, v := range t.Vertices {
						if g.Vertices[v].Owner >= 0 && g.Vertices[v].Owner != player {
							production[t.Resource] += g.Vertices[v].Level * (6 - absCatan(7-t.Number))
						}
					}
				}
				for c := range production {
					if production[c] > production[a.Color] {
						a.Color = c
					}
				}
			}
			choices = append(choices, botChoice{a, 800})
		}
	}
	choices = append(choices, botChoice{Action{Type: "catan_end"}, -1000})
	return s.botLegal(player, choices)
}
