package game

import "slices"

// Enumerate every public building/knight option, not just the bot's preferred
// moves. The caller quotes the real payment on independently owned pieces.
func (s *State) catanExplorerCityBuildCandidates(player int) []Action {
	g, k := s.Catan, s.Catan.CitiesKnights
	out := []Action{}
	for track := range 3 {
		out = append(out, Action{Type: "catan_improvement", Color: track})
	}
	for _, v := range g.Vertices {
		if g.canCityUpgrade(player, v.ID) {
			out = append(out, Action{Type: "catan_city", Vertex: v.ID})
		}
		if v.Owner == player && g.cityAt(v.ID) && !slices.Contains(k.Walls, v.ID) {
			out = append(out, Action{Type: "catan_wall", Vertex: v.ID})
		}
		if g.knightRecruitable(player, v.ID) {
			out = append(out, Action{Type: "catan_knight_recruit", Vertex: v.ID})
		}
	}
	for i := range k.Knights {
		n := &k.Knights[i]
		if n.Owner != player {
			continue
		}
		if !n.Active {
			out = append(out, Action{Type: "catan_knight_activate", Vertex: n.Vertex})
		}
		if g.knightCanPromote(n) {
			out = append(out, Action{Type: "catan_knight_promote", Vertex: n.Vertex})
		}
		if g.knightCanAct(n) {
			for _, target := range g.knightDestinations(*n, false) {
				out = append(out, Action{Type: "catan_knight_move", Vertex: n.Vertex, Target: target})
			}
		}
	}
	return out
}

// Required replies are described without executing them: the last city event
// can draw a secret progress card or resume production. Bundles are selected by
// the player using the separately projected count, own hand or authorized hand.
func (s *State) catanExplorerCityResponseChoices(player int) []Action {
	g, k := s.Catan, s.Catan.CitiesKnights
	out := []Action{}
	q := k.Pending
	if s.Finished || q == nil || s.CatanPendingActor() != player {
		return out
	}
	add := func(a Action) { a.Prompt = int(g.TurnSerial); out = append(out, a) }
	vertices := []int{}
	switch q.Kind {
	case "metropolis":
		vertices = g.cityMetropolisSites(player)
	case "pillage":
		vertices = g.pillageSites(player)
	case "knight_retreat":
		if q.Knight != nil {
			vertices = g.knightDestinations(*q.Knight, true)
		}
	case "treason_remove":
		for _, n := range k.Knights {
			if n.Owner == player {
				vertices = append(vertices, n.Vertex)
			}
		}
	case "treason_place":
		if q.Knight != nil {
			for _, vertex := range g.treasonPlacements(player, q.Knight.Strength) {
				for strength := 1; strength <= q.Knight.Strength; strength++ {
					if g.knightCount(player, strength) < 2 {
						add(Action{Type: "catan_treason_place", Vertex: vertex, Color: strength})
					}
				}
			}
		}
		add(Action{Type: "catan_treason_place", Choice: "skip"})
	case "aqueduct":
		for color, count := range g.Bank[:5] {
			if count > 0 {
				add(Action{Type: "catan_aqueduct", Color: color})
			}
		}
		if len(out) == 0 {
			add(Action{Type: "catan_aqueduct", Choice: "skip"})
		}
	case "defender_reward":
		for track, deck := range k.ProgressDecks {
			if len(deck) > 0 {
				add(Action{Type: "catan_defender_reward", Color: track})
			}
		}
	case "commercial_harbor":
		for color := 5; color < len(g.Bank); color++ {
			if g.Players[player].Resources[color] > 0 {
				add(Action{Type: "catan_commercial_harbor", Color: color})
			}
		}
	case "diplomacy":
		for _, edge := range g.diplomacyPlacements(player) {
			add(Action{Type: "catan_diplomacy", Edge: edge})
		}
		add(Action{Type: "catan_diplomacy", Choice: "skip"})
	case "espionage":
		seen := map[int]bool{}
		for _, card := range k.Players[q.Target].Progress {
			if !seen[card] && !catanProgressRules[card].Victory {
				add(Action{Type: "catan_espionage", Card: card})
				seen[card] = true
			}
		}
		add(Action{Type: "catan_espionage", Choice: "skip"})
	}
	for _, vertex := range vertices {
		add(Action{Type: "catan_" + q.Kind, Vertex: vertex})
	}
	return out
}

func (s *State) catanExplorerCityView(v map[string]any, player int) {
	g, k := s.Catan, s.Catan.CitiesKnights
	x := v["explorer"].(map[string]any)
	actor := s.CatanPendingActor()
	if actor < 0 && s.Phase != "catan_discard" && !s.Finished {
		actor = s.Turn
	}
	x["actor"] = actor
	x["canRespond"] = false
	legal := v["legal"].(map[string][]int)
	for _, key := range []string{"cities", "settlements", "roads", "pillage", "metropolis", "walls", "knightRecruit", "knightActivate", "knightPromote", "knightRetreat", "knightChase", "knightChasePirate"} {
		legal[key] = []int{}
	}
	moves := map[int][]int{}
	v["knightMoves"] = moves
	v["medicineHarbors"] = []int{}
	if s.Finished || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated {
		return
	}
	if s.Phase == "catan_discard" {
		if g.DiscardDue[player] > 0 {
			x["canRespond"] = true
			x["response"] = map[string]any{"type": "catan_discard", "field": "tokens", "count": g.DiscardDue[player], "prompt": int(g.TurnSerial)}
		}
		return
	}
	if q := k.Pending; q != nil {
		if s.CatanPendingActor() != player {
			return
		}
		x["canRespond"] = true
		field, count := "", 0
		switch q.Kind {
		case "progress_discard":
			field, count = "cards", len(k.Players[player].Progress)-4
		case "guild_dues":
			field, count = "take", min(2, sum(g.Players[q.Target].Resources))
		case "wedding":
			field, count = "give", min(2, sum(g.Players[player].Resources))
		case "sabotage":
			field, count = "give", sum(g.Players[player].Resources)/2
		}
		if field != "" {
			x["response"] = map[string]any{"type": "catan_" + q.Kind, "field": field, "count": count, "prompt": int(g.TurnSerial)}
		}
	} else if player == s.Turn {
		x["canRespond"] = true
		if s.Phase == "catan_turn" {
			// Medicine uses city geometry even when the ordinary city cost is
			// unaffordable; the actual discounted payment remains authoritative.
			for _, vertex := range g.Vertices {
				if g.canCityUpgrade(player, vertex.ID) && g.Explorer.Cargo.landVertex(g, player, vertex.ID) {
					legal["cities"] = append(legal["cities"], vertex.ID)
				}
				if slices.Contains(k.Players[player].Progress, 5) && vertex.Owner == player && vertex.Level == 1 {
					trial := clone(*s)
					tg, tx := trial.Catan, trial.Catan.Explorer
					if tx.Cargo.upgradeSettlement(tg, tx.Fleet, player, tg.TurnSerial, vertex.ID, "harbor", true) == nil {
						v["medicineHarbors"] = append(v["medicineHarbors"].([]int), vertex.ID)
					}
				}
			}
		}
	}
	// Reuse the already computed compact actions, without running probes twice.
	for _, a := range x["choices"].([]map[string]any) {
		typeName := a["type"].(string)
		key := map[string]string{"catan_settlement": "settlements", "catan_road": "roads", "catan_wall": "walls", "catan_pillage": "pillage", "catan_metropolis": "metropolis", "catan_knight_recruit": "knightRecruit", "catan_knight_activate": "knightActivate", "catan_knight_promote": "knightPromote", "catan_knight_retreat": "knightRetreat"}[typeName]
		if key != "" {
			field := "vertex"
			if typeName == "catan_road" {
				field = "edge"
			}
			legal[key] = append(legal[key], a[field].(int))
		}
		if typeName == "catan_knight_move" {
			vertex := a["vertex"].(int)
			moves[vertex] = append(moves[vertex], a["target"].(int))
		}
	}
}
