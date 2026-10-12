package catan

// Only the main island admits new, freely chosen settlements in Pirate
// Islands. Offshore settlements are the player's designated beachhead and
// recaptured fortress, already reached by its ship line. There is no Longest
// Route award, so offshore roads add no construction or scoring opportunity.
// This is a bot policy, not a restriction on legal human actions.
func (g *Catan) pirateBotRoadUseful(edge int) bool {
	if g.pirateIslands() == nil || g.CitiesKnights != nil || g.Attack != nil {
		// Those modules give routes uses beyond settlements and route awards.
		return true
	}
	e := g.Edges[edge]
	return g.pirateHomeVertex(e.A) && g.pirateHomeVertex(e.B)
}

// Do not voluntarily start Road Building if completing its mandatory choices
// would force the bot to place an unhelpful offshore road. Simulate both legal
// placements, including stock limits and the expedition's no-branch rule.
func (s *State) pirateBotSafeRoadBuilding(player int) bool {
	if s.Catan.CitiesKnights != nil || s.Catan.Attack != nil {
		return true
	}
	start := clone(*s)
	if start.Apply(player, Action{Type: "catan_dev", Card: 1}) != nil {
		return false
	}
	return start.Phase != "catan_roads" || len(start.pirateBotSafeFreeRoutes(player)) > 0
}

// Preserve the safe continuation when the bot actually executes the card, not
// just when it decides whether to play it. Each successful placement consumes
// a free route, so this lookahead is bounded by the card's two placements.
func (s *State) pirateBotSafeFreeRoutes(player int) []botChoice {
	if s.Catan.CitiesKnights != nil || s.Catan.Attack != nil {
		return s.Catan.seaBuildChoices(player)
	}
	result := []botChoice{}
	for _, choice := range s.Catan.seaBuildChoices(player) {
		if choice.action.Type == "catan_road" && !s.Catan.pirateBotRoadUseful(choice.action.Edge) {
			continue
		}
		next := clone(*s)
		if next.Apply(player, choice.action) != nil {
			continue
		}
		if next.Phase != "catan_roads" || len(next.pirateBotSafeFreeRoutes(player)) > 0 {
			result = append(result, choice)
		}
	}
	return result
}
