package game

import "slices"

// Extend mission priorities with public city/knight opportunities. Keep every
// resource AND commodity in the reserve when planning a bank exchange.
func (s *State) catanExplorerCityBotPlans(player int) []catanExplorerBotPlan {
	g := s.Catan
	if g.CitiesKnights == nil {
		return nil
	}
	choices := append(g.cityEconomyBotChoices(player), g.knightBotChoices(player)...)
	for _, v := range g.Vertices {
		if g.canCityUpgrade(player, v.ID) && g.Explorer.Cargo.landVertex(g, player, v.ID) {
			choices = append(choices, botChoice{Action{Type: "catan_city", Vertex: v.ID}, 600})
		}
	}
	plans := []catanExplorerBotPlan{}
	for _, choice := range choices {
		a := choice.action
		a.Prompt = int(g.TurnSerial)
		cost := make([]int, len(g.Bank))
		switch a.Type {
		case "catan_city":
			cost[3], cost[4] = 2, 3
		case "catan_wall":
			cost[1] = 2
		case "catan_improvement":
			cost[5+a.Color] = g.CitiesKnights.Players[player].Improvements[a.Color] + 1
		case "catan_knight_recruit", "catan_knight_promote":
			cost[2], cost[4] = 1, 1
		case "catan_knight_activate":
			cost[3] = 1
		case "catan_knight_move":
		default:
			// Chasing the E&P pirate with a knight needs a confirmed rule.
			continue
		}
		plans = append(plans, catanExplorerBotPlan{a, cost, choice.score / 4})
	}
	return plans
}

func (s *State) catanExplorerCityProgressBot(player int) (Action, bool) {
	g, x := s.Catan, s.Catan.Explorer
	hand := g.CitiesKnights.Players[player].Progress
	choices := append(g.scienceBotChoices(player), g.tradeProgressBotChoices(player)...)
	choices = append(choices, g.politicsBotChoices(player)...)
	// Base Road Building/Taxation use different board geometry. Replace those
	// candidates before checking legality, never simulate random exploration.
	choices = slices.DeleteFunc(choices, func(c botChoice) bool {
		return c.action.Type == "catan_progress" && (c.action.Card == 7 || c.action.Card == 21)
	})
	if slices.Contains(hand, 7) && len(s.catanExplorerFreeRoadSites(player)) > 0 {
		choices = append(choices, botChoice{Action{Type: "catan_progress", Card: 7}, 700})
	}
	if slices.Contains(hand, 5) {
		for _, v := range g.Vertices {
			if v.Owner == player && v.Level == 1 && catanExplorerCoast(g, v.ID) && !slices.Contains(g.CitiesKnights.FallenCities, v.ID) {
				choices = append(choices, botChoice{Action{Type: "catan_progress", Card: 5, Choice: "harbor", Vertex: v.ID}, 720})
			}
		}
	}
	if slices.Contains(hand, 21) && g.CitiesKnights.Invasions > 0 {
		for ship, edge := range x.Fleet.Positions {
			owner := ship / 3
			if edge >= 0 && owner != player && (sum(g.Players[owner].Resources) > 0 || x.Economy.Gold[owner] > 0) {
				choices = append(choices, botChoice{Action{Type: "catan_progress", Card: 21}, 760})
				break
			}
		}
	}
	for i := range choices {
		choices[i].action.Prompt = int(g.TurnSerial)
	}
	// These are owned, deterministic action-phase cards/offers. A trial checks
	// legality only; it never scores an outcome, rolls dice, sails or steals.
	a, err := s.botLegal(player, choices)
	return a, err == nil
}
