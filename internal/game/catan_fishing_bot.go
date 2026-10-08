package game

import "slices"

func (g *Catan) fishLakeOdds(tile int) int {
	if g.Fishing != nil {
		for _, lake := range g.Fishing.Map.Lakes {
			if lake.Tile == tile {
				odds := 0
				for _, number := range lake.Numbers {
					odds += 6 - absCatan(7-number)
				}
				return odds
			}
		}
	}
	return 0
}

func (g *Catan) fishVertexValue(vertex int) int {
	if g.Fishing == nil {
		return 0
	}
	odds := 0
	for _, lake := range g.Fishing.Map.Lakes {
		if slices.Contains(g.Tiles[lake.Tile].Vertices, vertex) {
			odds += g.fishLakeOdds(lake.Tile)
		}
	}
	for _, ground := range g.Fishing.Map.Grounds {
		if slices.Contains(ground.Vertices[:], vertex) {
			odds += 6 - absCatan(7-ground.Number)
		}
	}
	// A token averages about two fish, while a chosen bank resource costs four.
	return odds * 5
}

func (s *State) catanFishBotChoices(player int, builds []botChoice, road int) []botChoice {
	choices := []botChoice{}
	if !s.catanFishActionReady(player) {
		return choices
	}
	g := s.Catan
	for _, target := range g.fishBootTargets(player) {
		points := g.Players[target].Score - g.hiddenVictoryPoints(target)
		choices = append(choices, botChoice{Action{Type: "catan_fish_boot", Target: target}, 1500 + points})
	}
	add := func(a Action, score int) {
		if ids := g.fishPayment(player, g.fishActionCost(player, a.Type)); ids != nil {
			a.Tokens = ids
			choices = append(choices, botChoice{a, score})
		}
	}
	if g.Robber >= 0 {
		for _, vertex := range g.Tiles[g.Robber].Vertices {
			if g.Vertices[vertex].Owner == player && g.Vertices[vertex].Level > 0 {
				add(Action{Type: "catan_fish_robber"}, 750)
				break
			}
		}
	}
	if g.fishCanRemovePirate(player) {
		blocked := false
		for _, ground := range g.Fishing.Map.Grounds {
			if ground.SeaTile == nil || *ground.SeaTile != g.Seafarers.Pirate {
				continue
			}
			for _, id := range ground.Vertices {
				v := g.Vertices[id]
				blocked = blocked || v.Owner == player && v.Level > 0
			}
		}
		for _, edge := range g.Edges {
			blocked = blocked || edge.Owner == player && edge.Ship && g.pirateBlocks(edge.ID)
		}
		if blocked {
			add(Action{Type: "catan_fish_pirate"}, 760)
		}
	}
	if s.Phase == "catan_roll" {
		return choices // Remove a blocking robber before production; spend the rest after it.
	}
	for _, target := range g.cardTheftTargets(player) {
		points := g.Players[target].Score - g.hiddenVictoryPoints(target)
		add(Action{Type: "catan_fish_steal", Target: target}, 100+points*6)
	}
	if g.fishDevelopmentReady() {
		add(Action{Type: "catan_fish_dev"}, 220)
	}
	if k := g.CitiesKnights; k != nil {
		for track, deck := range k.ProgressDecks {
			if len(deck) > 0 {
				add(Action{Type: "catan_fish_progress", Color: track}, 230-track*5)
			}
		}
	}
	if g.fishingRivers() {
		for _, edge := range g.Rivers.Map.Bridges {
			if g.canBridge(player, edge) {
				add(Action{Type: "catan_fish_bridge", Edge: edge}, 240)
			}
		}
	}
	if road >= 0 {
		add(Action{Type: "catan_fish_road", Edge: road}, 210)
	}
	// Reuse the existing public-route evaluation; do not inspect hidden
	// terrain, other players' fish or development-card order.
	for _, build := range builds {
		if build.action.Type == "catan_ship" {
			add(Action{Type: "catan_fish_ship", Edge: build.action.Edge}, build.score+10)
		}
	}
	for color, count := range g.Bank[:5] {
		if count <= 0 {
			continue
		}
		score := 60 - g.Players[player].Resources[color]*10
		for _, build := range builds {
			cost := catanBotBuildCost(build.action)
			if len(cost) < 5 || cost[color] <= g.Players[player].Resources[color] {
				continue
			}
			missing := 0
			for c, amount := range cost[:5] {
				missing += max(0, amount-g.Players[player].Resources[c])
			}
			score = max(score, build.score/2-45*(missing-1))
		}
		add(Action{Type: "catan_fish_resource", Color: color}, score)
	}
	return choices
}
