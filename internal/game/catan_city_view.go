package game

// Apply city hand/deck privacy before any expansion-specific early return.
func (s *State) catanCityView(v map[string]any, player int) {
	g := s.Catan
	if k := g.CitiesKnights; k != nil {
		public := v["citiesKnights"].(map[string]any)
		delete(public, "progressDecks")
		// The saved production queue is server-only. Clients receive the
		// current response through pending and the public result through eventDie.
		delete(public, "event")
		public["progressRemaining"] = []int{len(k.ProgressDecks[0]), len(k.ProgressDecks[1]), len(k.ProgressDecks[2])}
		v["progressRules"] = CatanProgressRules()
		playable := []int{}
		if !s.Finished && player == s.Turn && player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated && k.Pending == nil && k.Event == nil && !g.setup() {
			seen := map[int]bool{}
			for _, card := range k.Players[player].Progress {
				if !seen[card] && ((card == 0 && s.Phase == "catan_roll" && (!g.twoKnights() || len(g.Two.Rolls) == 0)) || (card > 0 && card <= 24 && !catanProgressRules[card].Victory && (card != 21 || k.Invasions > 0) && s.Phase == "catan_turn")) {
					playable = append(playable, card)
					seen[card] = true
				}
			}
			if s.Phase == "catan_turn" {
				v["inventionTiles"] = g.inventionTiles()
				v["inventionNumbers"] = g.inventionNumbers()
				v["taxationTiles"] = g.taxationTiles()
				v["smithingOptions"] = g.smithingOptions(player)
				v["merchantTiles"] = g.merchantTiles(player)
				v["guildDuesTargets"] = g.guildDuesTargets(player)
				v["intrigueTargets"] = g.intrigueTargets(player)
				v["diplomacyRoads"] = g.diplomacyRoadsFor(player)
			}
		}
		v["progressPlayable"] = playable
		if q := k.Pending; q != nil && q.Kind == "commercial_harbor" && player != q.Target && s.CatanPendingActor() != player {
			delete(public["pending"].(map[string]any), "color")
		}
		if q := k.Pending; q != nil && q.Kind == "guild_dues" && s.CatanPendingActor() == player {
			public["pending"].(map[string]any)["resources"] = append([]int{}, g.Players[q.Target].Resources...)
		}
		if q := k.Pending; q != nil && s.CatanPendingActor() == player {
			if q.Kind == "diplomacy" {
				v["diplomacyPlacements"] = g.diplomacyPlacements(player)
			}
			if q.Kind == "espionage" {
				public["pending"].(map[string]any)["progress"] = append([]int{}, k.Players[q.Target].Progress...)
			}
			if q.Kind == "treason_place" && q.Knight != nil {
				v["treasonPlacements"] = g.treasonPlacements(player, q.Knight.Strength)
			}
		}
		for i, raw := range public["players"].([]any) {
			seat := raw.(map[string]any)
			seat["progressCount"] = len(k.Players[i].Progress)
			if i != player && !s.Finished {
				delete(seat, "progress")
			}
		}
	}
}
