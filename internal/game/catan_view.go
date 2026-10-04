package game

func (s *State) catanView(view map[string]any, player int) {
	g := s.Catan
	v := view["catan"].(map[string]any)
	if g.Seafarers != nil && g.Seafarers.Fog != nil {
		sea := v["seafarers"].(map[string]any)
		sea["fog"] = map[string]any{"remaining": len(g.Seafarers.Fog.Terrain), "startTiles": append([]int{}, g.Seafarers.Fog.StartTiles...)}
	}
	delete(v, "devDeck")
	delete(v, "devDiscard")
	v["devRemaining"] = len(g.DevDeck)
	if g.Options.Helpers {
		v["helperRules"] = CatanHelpers()
		if player >= 0 && player < len(g.Players) && g.helperReady(player, 4) && player == s.Turn && s.Phase == "catan_turn" {
			moves := map[int][]int{}
			for _, from := range g.Edges {
				if !g.helperEndRoad(player, from.ID) {
					continue
				}
				temp := *g
				temp.Edges = append([]CatanEdge{}, g.Edges...)
				temp.Edges[from.ID].Owner = -1
				for _, to := range temp.Edges {
					if to.ID != from.ID && temp.canRoad(player, to.ID) {
						moves[from.ID] = append(moves[from.ID], to.ID)
					}
				}
			}
			v["helperRoadMoves"] = moves
		}
		if q := g.HelperPending; q != nil {
			pending := v["helperPending"].(map[string]any)
			if q.Player != player {
				delete(pending, "cards")
			}
			if q.Kind == "leader" && q.Player == player {
				pending["resources"] = append([]int{}, g.Players[q.Target].Resources...)
			}
		}
	}
	for i, raw := range v["players"].([]any) {
		p := raw.(map[string]any)
		actual := g.Players[i]
		p["resourceCount"] = sum(actual.Resources)
		p["devCount"] = sum(actual.Dev)
		p["publicScore"] = actual.Score - actual.Dev[4]
		p["rates"] = g.rates(i)
		roads, settlements, cities := g.pieces(i)
		p["roadsLeft"] = 15 - roads
		if g.Seafarers != nil {
			p["shipsLeft"] = 15 - g.shipCount(i)
		}
		p["settlementsLeft"] = 5 - settlements
		p["citiesLeft"] = 4 - cities
		if actual.Helper != nil {
			p["helperReady"] = g.helperReady(i, actual.Helper.ID)
		}
		if i != player && !s.Finished {
			delete(p, "resources")
			delete(p, "dev")
			delete(p, "newDev")
			p["score"] = actual.Score - actual.Dev[4]
		}
	}
	// Legal locations are computed using only public map and the viewer's identity.
	legal := map[string][]int{"settlements": {}, "cities": {}, "roads": {}}
	if g.Seafarers != nil {
		legal["ships"] = []int{}
		legal["pirate"] = []int{}
	}
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
				valid = g.setupRoute(player, e.ID, false)
			} else if s.Phase == "catan_turn" || s.Phase == "catan_roads" {
				valid = g.canRoad(player, e.ID)
			}
			if valid {
				legal["roads"] = append(legal["roads"], e.ID)
			}
		}
	}
	if g.Seafarers != nil && player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated && !s.Finished && player == s.Turn {
		moves := map[int][]int{}
		for _, e := range g.Edges {
			if g.shipCount(player) < 15 && ((s.Phase == "catan_setup_road" && g.setupRoute(player, e.ID, true)) || ((s.Phase == "catan_turn" || s.Phase == "catan_roads") && g.canShip(player, e.ID))) {
				legal["ships"] = append(legal["ships"], e.ID)
			}
			if s.Phase == "catan_turn" {
				if destinations := g.shipDestinations(player, e.ID); len(destinations) > 0 {
					moves[e.ID] = destinations
				}
			}
		}
		if s.Phase == "catan_robber" {
			for _, t := range g.Tiles {
				if t.Resource == CatanSea && t.ID != g.Seafarers.Pirate {
					legal["pirate"] = append(legal["pirate"], t.ID)
				}
			}
			if g.Seafarers.Pirate >= 0 {
				legal["pirate"] = append(legal["pirate"], -1)
			}
		}
		v["shipMoves"] = moves
	}
	v["legal"] = legal
}
