package game

import "slices"

func (s *State) catanView(view map[string]any, player int) {
	g := s.Catan
	v := view["catan"].(map[string]any)
	s.catanCityView(v, player)
	if g.Explorer != nil {
		s.catanExplorerView(v, player)
		return
	}
	s.catanAttackView(v, player)
	if g.Transport != nil {
		s.catanTransportView(v, player)
	}
	if q := g.Two; q != nil {
		public := v["two"].(map[string]any)
		public["canAct"] = !s.Finished && (q.Pending != nil || q.Trade != nil) && player == s.Turn
		public["actor"] = s.CatanPendingActor()
		if q.Trade != nil && player != s.Turn {
			delete(public["trade"].(map[string]any), "drawn")
		}
		if player >= 0 && player < len(g.Players) {
			public["cost"] = g.twoTokenCost(player)
			public["tokenWindow"] = s.catanTwoTokenWindow(player)
			if s.catanTwoTokenWindow(player) {
				public["retreatTiles"] = g.twoRetreatTiles()
				if g.Transport != nil {
					public["retreatCost"] = 1
					public["retreatEdges"] = g.twoTransportRetreatEdges()
				}
			}
		}
		if q.Pending != nil && !s.Finished {
			public["choices"] = g.twoNeutralChoices(q.Pending.Kind)
		}
		public["neutralRoadLengths"] = []int{g.roadLength(-2), g.roadLength(-3)}
	}
	if c := g.Caravans; c != nil {
		public := v["caravans"].(map[string]any)
		public["remaining"] = c.Map.Supply - len(c.Wagons)
		public["actor"] = s.CatanPendingActor()
		public["canAct"] = !s.Finished && player >= 0 && player == s.CatanPendingActor()
		if c.Pending != nil && !s.Finished {
			public["choices"] = c.responseChoices(g)
		}
	}
	s.catanFishingView(v, player)
	if g.Rivers != nil {
		richest, poor := g.riverWealth()
		v["rivers"].(map[string]any)["richest"] = richest
		v["rivers"].(map[string]any)["poor"] = poor
	}
	v["setupLimit"] = g.SetupLimit()
	v["victoryTarget"] = g.victoryTarget()
	delete(v, "revealedEvent")
	if revealed := g.revealedEventView(); revealed != nil {
		v["revealedEvent"] = *revealed
	}
	if q := g.CardEvent; q != nil {
		public := v["cardEvent"].(map[string]any)
		delete(public, "gifts")
		if q.Kind == "conflict" {
			public["canSkip"] = q.Optional && s.Phase == "catan_card_event" && !s.Finished && s.CatanPendingActor() == player
		}
		for _, gift := range q.Gifts {
			if gift.From == player {
				public["ownGift"] = gift
			}
		}
	}
	if g.FriendlyRobber != nil {
		protected := []int{}
		for i := range g.Players {
			if g.friendlyProtected(i) {
				protected = append(protected, i)
			}
		}
		v["friendlyRobber"].(map[string]any)["protectedPlayers"] = protected
	}
	if g.Harbors != nil {
		v["harbors"].(map[string]any)["points"] = g.harborPoints()
	}
	if w := g.newWorld(); w != nil {
		public := map[string]any{"index": w.Index, "total": len(w.Ports), "remaining": len(w.Ports) - w.Index}
		if w.Index < len(w.Ports) {
			public["current"] = w.Ports[w.Index]
		}
		v["seafarers"].(map[string]any)["newWorld"] = public
	}
	if w := g.wonders(); w != nil {
		v["wonderRules"] = append([]CatanWonderRule{}, catanWonderRules[:len(w.Cards)]...)
		claims, builds := []int{}, []int{}
		if player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated && !s.Finished && player == s.Turn && s.Phase == "catan_turn" && g.HelperPending == nil && g.GoldPending == nil {
			for _, card := range w.Cards {
				if g.wonderClaimable(player, card.ID) {
					claims = append(claims, card.ID)
				}
				if card.Owner == player && card.Level < 4 && catanHas(g.Players[player].Resources, catanWonderRules[card.ID].Cost[:]) {
					builds = append(builds, card.ID)
				}
			}
		}
		v["wonderClaims"], v["wonderBuilds"] = claims, builds
	}
	if g.Seafarers != nil && g.Seafarers.Fog != nil {
		sea := v["seafarers"].(map[string]any)
		sea["fog"] = map[string]any{"remaining": len(g.Seafarers.Fog.Terrain), "startTiles": append([]int{}, g.Seafarers.Fog.StartTiles...)}
	}
	if t := g.tribe(); t != nil {
		tribe := v["seafarers"].(map[string]any)["tribe"].(map[string]any)
		// Old saves may have no placed ports yet. Keep the public collection
		// iterable, just like other map arrays.
		if g.Ports == nil {
			v["ports"] = []CatanPort{}
		}
		cards := []map[string]int{}
		for _, card := range t.Development {
			cards = append(cards, map[string]int{"edge": card.Edge})
		}
		tribe["development"] = cards
	}
	delete(v, "devDeck")
	delete(v, "devDiscard")
	v["devRemaining"] = len(g.DevDeck)
	if g.Attack != nil {
		v["devRemaining"] = len(g.Attack.Deck)
	}
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
		p["publicScore"] = actual.Score - g.hiddenVictoryPoints(i)
		p["rates"] = g.rates(i)
		roads, _, _ := g.pieces(i)
		p["roadsLeft"] = 15 - roads
		if g.Rivers != nil {
			p["bridgesLeft"] = 3 - g.bridgeCount(i)
		}
		if g.Seafarers != nil {
			p["shipsLeft"] = 15 - g.shipCount(i)
		}
		p["settlementsLeft"] = g.settlementPiecesLeft(i)
		p["citiesLeft"] = g.cityPiecesLeft(i)
		if actual.Helper != nil {
			p["helperReady"] = g.helperReady(i, actual.Helper.ID)
		}
		if i != player && !s.Finished {
			delete(p, "resources")
			delete(p, "dev")
			delete(p, "newDev")
			p["score"] = actual.Score - g.hiddenVictoryPoints(i)
		}
	}
	// Legal choices use public state and the viewer's own hand only.
	legal := map[string][]int{"settlements": {}, "cities": {}, "roads": {}, "robber": {}, "repairRoads": {}, "earthquakeRoads": {}, "eventResources": {}, "fleeDeserts": {}, "eventGifts": {}, "eventTargets": {}}
	if q := g.CardEvent; q != nil && s.Phase == "catan_card_event" && !s.Finished && s.CatanPendingActor() == player {
		switch q.Kind {
		case "conflict", "trade_advantage":
			legal["eventTargets"] = g.cardTheftTargets(player)
		case "earthquake":
			legal["earthquakeRoads"] = g.earthquakeRoads(player)
		case "plentiful_year", "calm_seas", "tournament":
			for color, count := range g.Bank[:5] {
				if count > 0 {
					legal["eventResources"] = append(legal["eventResources"], color)
				}
			}
		case "robber_flees":
			legal["fleeDeserts"] = g.fleeDeserts()
		case "good_neighbors", "helpful_neighbor":
			if q.Kind == "helpful_neighbor" {
				legal["eventTargets"] = append([]int{}, q.Targets...)
			}
			legal["eventGifts"] = []int{}
			for color, count := range g.Players[player].Resources {
				if count > 0 {
					legal["eventGifts"] = append(legal["eventGifts"], color)
				}
			}
		}
	}
	if k := g.CitiesKnights; k != nil {
		legal["pillage"] = []int{}
		if !s.Finished && s.CatanPendingActor() == player && k.Pending != nil && k.Pending.Kind == "pillage" {
			legal["pillage"] = g.pillageSites(player)
		}
		moves := map[int][]int{}
		for _, key := range []string{"knightRecruit", "knightActivate", "knightPromote", "knightChase", "knightChasePirate", "knightRetreat"} {
			legal[key] = []int{}
		}
		if !s.Finished && player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated {
			if q := k.Pending; q != nil && q.Kind == "knight_retreat" && q.Knight != nil && s.CatanPendingActor() == player {
				legal["knightRetreat"] = g.knightDestinations(*q.Knight, true)
			}
			if player == s.Turn && s.Phase == "catan_turn" && k.Pending == nil {
				for _, v := range g.Vertices {
					if g.knightRecruitable(player, v.ID) {
						legal["knightRecruit"] = append(legal["knightRecruit"], v.ID)
					}
				}
				for i := range k.Knights {
					n := &k.Knights[i]
					if n.Owner != player {
						continue
					}
					if !n.Active {
						legal["knightActivate"] = append(legal["knightActivate"], n.Vertex)
					}
					if g.knightCanPromote(n) {
						legal["knightPromote"] = append(legal["knightPromote"], n.Vertex)
					}
					if g.knightCanAct(n) {
						moves[n.Vertex] = g.knightDestinations(*n, false)
					}
					if g.knightCanChasePirate(n) {
						legal["knightChasePirate"] = append(legal["knightChasePirate"], n.Vertex)
					}
					if g.knightCanChase(n) {
						legal["knightChase"] = append(legal["knightChase"], n.Vertex)
					}
				}
			}
		}
		v["knightMoves"] = moves
	}
	if t := g.tribe(); t != nil {
		legal["ports"] = []int{}
		if t.Pending != nil && t.Pending.Player == player && !s.Finished {
			legal["ports"] = g.tribePortEdges(player)
		}
	}
	if g.Seafarers != nil {
		legal["ships"] = []int{}
		legal["pirate"] = []int{}
	}
	if player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated && !s.Finished && player == s.Turn {
		if s.Phase == "catan_rivers_start" && g.Rivers != nil {
			legal["robber"] = slices.Clone(g.Rivers.Map.Swamps)
		}
		if s.Phase == "catan_turn" && g.Rivers != nil {
			legal["bridges"] = []int{}
			for _, id := range g.Rivers.Map.Bridges {
				if g.canBridge(player, id) {
					legal["bridges"] = append(legal["bridges"], id)
				}
			}
		}
		if s.Phase == "catan_world_fish" {
			legal["fishGrounds"] = []int{}
			for _, coast := range g.worldFishCoasts() {
				legal["fishGrounds"] = append(legal["fishGrounds"], coast.Vertices[1])
			}
		}
		if s.Phase == "catan_world_ports" {
			legal["ports"] = g.worldPortEdges()
		}
		if s.Phase == "catan_wonders_start" {
			legal["robber"] = g.wonderStartTiles()
		}
		if s.Phase == "catan_cloth_start" {
			legal["robber"] = g.clothStartTiles()
		}
		if s.Phase == "catan_robber" {
			for _, tile := range g.Tiles {
				if g.robberAllowed(tile.ID) {
					legal["robber"] = append(legal["robber"], tile.ID)
				}
			}
		}
		roads, _, _ := g.pieces(player)
		settlementsLeft := g.settlementPiecesLeft(player)
		for _, v := range g.Vertices {
			if settlementsLeft > 0 && (s.Phase == "catan_setup_settlement" || s.Phase == "catan_turn") && g.canSettlement(player, v.ID, g.setup()) {
				legal["settlements"] = append(legal["settlements"], v.ID)
			}
			if (s.Phase == "catan_turn" && g.canCityUpgrade(player, v.ID)) || (s.Phase == "catan_setup_city" && g.canSettlement(player, v.ID, true)) {
				legal["cities"] = append(legal["cities"], v.ID)
			}
		}
		for _, e := range g.Edges {
			if (s.Phase == "catan_turn" || (s.Phase == "catan_roads" && g.FreeRoads > 0)) && g.roadRepairable(player, e.ID) {
				legal["repairRoads"] = append(legal["repairRoads"], e.ID)
			}
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
		if s.Phase == "catan_robber" && g.pirateAllowed(player) {
			for _, t := range g.Tiles {
				if g.pirateDestinationAllowed(player, t.ID) {
					legal["pirate"] = append(legal["pirate"], t.ID)
				}
			}
			if g.pirateDestinationAllowed(player, -1) {
				legal["pirate"] = append(legal["pirate"], -1)
			}
		}
		v["shipMoves"] = moves
	}
	v["legal"] = legal
}
