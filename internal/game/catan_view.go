package game

func (s *State) catanView(view map[string]any, player int) {
	g := s.Catan
	v := view["catan"].(map[string]any)
	delete(v, "devDeck")
	delete(v, "devDiscard")
	v["devRemaining"] = len(g.DevDeck)
	for i, raw := range v["players"].([]any) {
		p := raw.(map[string]any)
		actual := g.Players[i]
		p["resourceCount"] = sum(actual.Resources)
		p["devCount"] = sum(actual.Dev)
		p["publicScore"] = actual.Score - actual.Dev[4]
		p["rates"] = g.rates(i)
		roads, settlements, cities := g.pieces(i)
		p["roadsLeft"] = 15 - roads
		p["settlementsLeft"] = 5 - settlements
		p["citiesLeft"] = 4 - cities
		if i != player && !s.Finished {
			delete(p, "resources")
			delete(p, "dev")
			delete(p, "newDev")
			p["score"] = actual.Score - actual.Dev[4]
		}
	}
	// Legal locations are computed using only public map and the viewer's identity.
	legal := map[string][]int{"settlements": {}, "cities": {}, "roads": {}}
	if player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated && !s.Finished && player == s.Turn {
		roads, settlements, cities := g.pieces(player)
		for _, v := range g.Vertices {
			if settlements < 5 && (s.Phase == "catan_setup_settlement" || s.Phase == "catan_turn") && g.canSettlement(player, v.ID, g.setup()) {
				legal["settlements"] = append(legal["settlements"], v.ID)
			}
			if s.Phase == "catan_turn" && cities < 4 && v.Level == 1 && v.Owner == player {
				legal["cities"] = append(legal["cities"], v.ID)
			}
		}
		for _, e := range g.Edges {
			if roads >= 15 {
				continue
			}
			valid := false
			if s.Phase == "catan_setup_road" {
				valid = e.Owner < 0 && (e.A == g.SetupVertex || e.B == g.SetupVertex)
			} else if s.Phase == "catan_turn" || s.Phase == "catan_roads" {
				valid = g.canRoad(player, e.ID)
			}
			if valid {
				legal["roads"] = append(legal["roads"], e.ID)
			}
		}
	}
	v["legal"] = legal
}
