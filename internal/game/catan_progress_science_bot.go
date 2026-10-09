package game

import "slices"

func (g *Catan) smithingOptions(player int) [][]int {
	out := [][]int{}
	if g.attackKnights() {
		c := g.Attack.City
		for _, knight := range c.Knights {
			if !g.attackCityCanPromote(player, knight) {
				continue
			}
			out = append(out, []int{knight.Edge})
			next := clone(*g)
			i := next.Attack.City.at(knight.Edge)
			next.Attack.City.Knights[i].Strength++
			next.Attack.City.Knights[i].PromotedAt = next.CitiesKnights.ActionSerial
			for _, second := range next.Attack.City.Knights {
				if next.attackCityCanPromote(player, second) {
					out = append(out, []int{knight.Edge, second.Edge})
				}
			}
		}
		return out
	}
	if g.CitiesKnights == nil {
		return out
	}
	for i := range g.CitiesKnights.Knights {
		n := &g.CitiesKnights.Knights[i]
		if n.Owner != player || !g.knightCanPromote(n) {
			continue
		}
		out = append(out, []int{n.Vertex})
		next := clone(*g)
		promoted := next.knightAt(n.Vertex)
		promoted.Strength++
		promoted.PromotedAt = next.CitiesKnights.ActionSerial
		for j := range next.CitiesKnights.Knights {
			second := &next.CitiesKnights.Knights[j]
			if second.Owner == player && next.knightCanPromote(second) {
				out = append(out, []int{n.Vertex, second.Vertex})
			}
		}
	}
	return out
}
func (g *Catan) scienceTileValue(player, tile int) int {
	value := 0
	for _, v := range g.Tiles[tile].Vertices {
		building := g.Vertices[v]
		if building.Level == 0 || building.Owner < 0 || g.Players[building.Owner].Eliminated {
			continue
		}
		if building.Owner == player {
			value += building.Level * 4
		} else {
			value -= building.Level
		}
	}
	return value
}
func (g *Catan) alchemyBotDice(player int) []int {
	best := []int{1, 1}
	score := -1 << 30
	for red := 1; red <= 6; red++ {
		for yellow := 1; yellow <= 6; yellow++ {
			total := red + yellow
			if g.transportKnights() && len(g.Players) <= 4 && !g.riversTransport() && !g.caravansTransport() && !g.transportSea() && (total == 2 || total == 12) {
				continue
			}
			value := 0
			for _, t := range g.Tiles {
				if t.Number == total && t.ID != g.Robber && t.Resource < 5 {
					value += g.scienceTileValue(player, t.ID) * 10
				}
			}
			if total == 7 {
				value -= 20
				if sum(g.Players[player].Resources) > g.catanDiscardLimit(player) {
					value -= 100
				}
			}
			for _, level := range g.CitiesKnights.Players[player].Improvements {
				if level > 0 && red <= level+1 {
					value += 3
				}
			}
			if value > score {
				best = []int{red, yellow}
				score = value
			}
		}
	}
	return best
}
func (g *Catan) progressBotBuildCost(player int, a Action) []int {
	cost := make([]int, len(g.Bank))
	switch a.Card {
	case 1:
		cost[5+a.Color] = g.CitiesKnights.Players[player].Improvements[a.Color]
	case 5:
		cost[3], cost[4] = 1, 2
	}
	return cost
}
func (g *Catan) scienceBotChoices(player int) []botChoice {
	k := g.CitiesKnights
	if k == nil {
		return nil
	}
	choices := []botChoice{}
	seen := map[int]bool{}
	for _, card := range k.Players[player].Progress {
		if seen[card] {
			continue
		}
		seen[card] = true
		a := Action{Type: "catan_progress", Card: card}
		switch card {
		case 1:
			for _, build := range g.cityEconomyBotChoices(player) {
				if build.action.Type == "catan_improvement" {
					a.Color = build.action.Color
					choices = append(choices, botChoice{a, build.score + 120})
				}
			}
		case 2:
			if g.catanDiscardLimit(player) < 13 {
				for _, v := range g.Vertices {
					if v.Owner == player && g.cityAt(v.ID) && !slices.Contains(k.Walls, v.ID) {
						a.Vertex = v.ID
						choices = append(choices, botChoice{a, 710})
					}
				}
			}
		case 3:
			sites := g.inventionNumbers()
			best := 0
			for i, left := range sites {
				for _, right := range sites[i+1:] {
					leftOdds := 6 - absCatan(7-left.Number)
					rightOdds := 6 - absCatan(7-right.Number)
					gain := (rightOdds - leftOdds) * (g.scienceTileValue(player, left.Tile) - g.scienceTileValue(player, right.Tile))
					if gain > best {
						best = gain
						a.Tile = left.Tile
						a.Target = right.Tile
						a.Tokens = []int{left.Slot, right.Slot}
					}
				}
			}
			if best > 0 {
				choices = append(choices, botChoice{a, 650 + best})
			}
		case 4, 6:
			color := 3
			if card == 6 {
				color = 4
			}
			if gain := g.progressResourceGain(player, color); gain > 0 {
				choices = append(choices, botChoice{a, 650 + gain*10})
			}
		case 5:
			for _, v := range g.Vertices {
				if g.canCityUpgrade(player, v.ID) {
					a.Vertex = v.ID
					choices = append(choices, botChoice{a, 730})
				}
			}
		case 7:
			if g.hasFreeRouteAction(player) {
				choices = append(choices, botChoice{a, 700})
			}
		case 8:
			for _, targets := range g.smithingOptions(player) {
				a.Targets = targets
				choices = append(choices, botChoice{a, 750 + len(targets)*15})
			}
		}
	}
	return choices
}
