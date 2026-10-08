package game

import "slices"

func (s *State) catanExplorerHelperResponseChoices(player int) []Action {
	g := s.Catan
	out := []Action{}
	q := g.HelperPending
	if q == nil || q.Player != player {
		return out
	}
	add := func(a Action) { a.Type, a.Prompt = "catan_helper_choice", int(g.TurnSerial); out = append(out, a) }
	switch q.Kind {
	case "exchange":
		if !g.Players[player].Helper.Moon {
			add(Action{Choice: "flip"})
		}
		for _, id := range g.HelperDisplay {
			add(Action{Choice: "exchange", Card: id})
		}
	case "resource", "leader":
		hand := g.Bank[:5]
		if q.Kind == "leader" {
			hand = g.Players[q.Target].Resources[:5]
		}
		for r, n := range hand {
			if n > 0 {
				add(Action{Color: r})
			}
		}
		if q.Optional || q.Kind == "leader" && sum(hand) == 0 {
			add(Action{Choice: "skip"})
		}
	}
	return out
}

// Candidate construction uses only the acting hand and public geometry. Do
// not trial an action that inspects opponents' resources to decide visibility.
func (s *State) catanExplorerHelperChoices(player int) []Action {
	out := []Action{}
	if !s.explorerHelperReady(player) {
		return out
	}
	g, x := s.Catan, s.Catan.Explorer
	id, hand := g.Players[player].Helper.ID, g.Players[player].Resources[:5]
	add := func(a Action) { a.Prompt = int(g.TurnSerial); out = append(out, a) }
	probeBuild := func(a Action) {
		// Build primitives cannot choose/reveal a hidden tile; new ships are
		// placed beside an existing harbor, where all touching hexes are known.
		next := clone(*s)
		if next.catanExplorerHelperBuild(player, a) == nil {
			add(a)
		}
	}
	switch id {
	case 1:
		for want := range 5 {
			for give, count := range hand {
				if count == 0 || give == want {
					continue
				}
				for target, p := range g.Players {
					if target != player && !p.Eliminated && sum(p.Resources) > 0 {
						add(Action{Type: "catan_helper", Color: want, Targets: []int{target}, Cards: []int{give}})
					}
				}
			}
		}
	case 2:
		for _, cost := range helperCostChoices([]int{1, 1, 0, 0, 0}, hand) {
			for _, edge := range g.Edges {
				if _, err := x.Cargo.roadPrice(g, x.Fleet, player, g.TurnSerial, edge.ID, true); err == nil {
					add(Action{Type: "catan_road", Skill: "helper", Tokens: cost, Edge: edge.ID})
				}
			}
		}
	case 4:
		for _, from := range g.Edges {
			if !g.helperEndRoad(player, from.ID) {
				continue
			}
			base := *g
			base.Edges = slices.Clone(g.Edges)
			base.Edges[from.ID].Owner = -1
			for _, to := range g.Edges {
				if to.ID == from.ID {
					continue
				}
				if _, err := x.Cargo.roadPrice(&base, x.Fleet, player, g.TurnSerial, to.ID, true); err == nil {
					add(Action{Type: "catan_helper", Edge: from.ID, Target: to.ID})
				}
			}
		}
	case 6:
		for _, cost := range helperCostChoices([]int{1, 0, 1, 0, 0}, hand) {
			for _, edge := range g.Edges {
				if !catanExplorerSeaEdge(g, edge.ID) || !(g.Vertices[edge.A].Owner == player && catanExplorerHarborAt(g, edge.A) || g.Vertices[edge.B].Owner == player && catanExplorerHarborAt(g, edge.B)) {
					continue
				}
				for ship := player * 3; ship < (player+1)*3; ship++ {
					probeBuild(Action{Type: "catan_explorer_ship", Skill: "helper", Tokens: cost, Slot: ship, Edge: edge.ID})
				}
			}
		}
	case 7:
		for p, target := range g.Players {
			if p != player && !target.Eliminated && target.Score > g.Players[player].Score && sum(target.Resources) > 0 {
				add(Action{Type: "catan_helper", Target: p})
			}
		}
	case 8:
		for crew := player * 11; crew < (player+1)*11; crew++ {
			if !s.explorerHelperBuilderUnit(player, crew) {
				continue
			}
			for _, v := range g.Vertices {
				if catanExplorerBotSite(g, player, v.ID, true) && catanHas(hand, []int{1, 1, 0, 0, 0}) {
					probeBuild(Action{Type: "catan_settlement", Skill: "helper", Card: crew, Vertex: v.ID})
				}
				if v.Owner == player && v.Level == 1 && catanHas(hand, []int{0, 0, 0, 1, 2}) {
					probeBuild(Action{Type: "catan_explorer_harbor", Skill: "helper", Card: crew, Vertex: v.ID})
					if g.CitiesKnights != nil {
						probeBuild(Action{Type: "catan_city", Skill: "helper", Card: crew, Vertex: v.ID})
					}
				}
			}
		}
	case 9, 12:
		for give, count := range hand {
			need := 2
			if id == 12 {
				need = 1
			}
			if count < need {
				continue
			}
			for take, available := range g.Bank[:5] {
				if take == give || available == 0 {
					continue
				}
				a := Action{Type: "catan_helper", Color: give, Target: take}
				if id == 9 {
					a.Give, a.Take = make([]int, 5), make([]int, 5)
					a.Give[give], a.Take[take] = 2, 1
				}
				add(a)
			}
		}
	case 10:
		add(Action{Type: "catan_helper"})
	case 11:
		for r, n := range g.Bank[:5] {
			if n > 0 {
				add(Action{Type: "catan_helper", Color: r})
			}
		}
	}
	return out
}

func (s *State) catanExplorerHelperBot(player int) (Action, bool) {
	g := s.Catan
	choices := s.catanExplorerHelperChoices(player)
	if len(choices) == 0 {
		return Action{}, false
	}
	hand := g.Players[player].Resources
	best, value := -1, -100000
	for i, a := range choices {
		score := 0
		switch g.Players[player].Helper.ID {
		case 1:
			score = hand[a.Cards[0]] - hand[a.Color]
		case 2:
			if a.Edge != catanExplorerBotRoad(g, player) {
				continue
			}
			score = 30
		case 4:
			// Do not randomly undo useful routes to consume the helper.
			continue
		case 6:
			if g.Explorer.Fleet.Positions[a.Slot] >= 0 {
				continue
			}
			score = 10
		case 7:
			score = 30
		case 8:
			score = 50
		case 9:
			give, take := 0, 0
			for r := range 5 {
				if a.Give[r] > 0 {
					give = r
				}
				if a.Take[r] > 0 {
					take = r
				}
			}
			score = hand[give] - hand[take] - 1
		case 10:
			score = 10
		case 11:
			score = 20 - hand[a.Color]
		case 12:
			score = hand[a.Color] - hand[a.Target]
		}
		if score > 0 && score > value {
			best, value = i, score
		}
	}
	if best < 0 {
		return Action{}, false
	}
	return choices[best], true
}
