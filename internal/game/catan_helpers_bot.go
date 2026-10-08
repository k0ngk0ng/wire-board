package game

import "errors"

func (s *State) catanHelperPendingBot(player int) (Action, error) {
	g := s.Catan
	q := g.HelperPending
	if q == nil || q.Player != player {
		return Action{}, errors.New("waiting for another helper owner")
	}
	p := g.Players[player]
	a := Action{Type: "catan_helper_choice"}
	switch q.Kind {
	case "exchange":
		if !p.Helper.Moon && (p.Helper.ID == 3 || p.Helper.ID == 5 || p.Helper.ID == 6 || p.Helper.ID == 11) {
			a.Choice = "flip"
			return a, nil
		}
		if len(g.HelperDisplay) == 0 && !p.Helper.Moon {
			a.Choice = "flip"
			return a, nil
		}
		score := -1
		for _, id := range g.HelperDisplay {
			value := map[int]int{1: 30, 2: 40, 3: 80, 4: 10, 5: 70, 6: 60, 7: 35, 8: 25, 9: 50, 10: 45, 11: 75, 12: 20}[id]
			if value > score {
				score = value
				a.Card = id
			}
		}
		a.Choice = "exchange"
	case "resource", "leader":
		hand := g.Bank
		if q.Kind == "leader" {
			hand = g.Players[q.Target].Resources
		}
		best, score := -1, -999
		for i, n := range hand[:5] {
			if n == 0 {
				continue
			}
			value := 10 - p.Resources[i]*3
			if i == 3 || i == 4 {
				value += 2
			}
			if value > score {
				best = i
				score = value
			}
		}
		if best < 0 && q.Optional {
			a.Choice = "skip"
		} else {
			a.Color = best
		}
	case "progress":
		best := -1
		for _, card := range q.Cards {
			score := g.cityHelperProgressValue(player, card)
			if score > best {
				best = score
				a.Card = card
			}
		}
	case "development":
		best := -1
		for _, card := range q.Cards {
			score := []int{4, 3, 2, 1, 5}[card]
			if score > best {
				best = score
				a.Card = card
			}
		}
	default:
		return Action{}, errors.New("unknown helper prompt")
	}
	return a, nil
}

func helperCostChoices(base, held []int) [][]int {
	result := [][]int{}
	if catanHas(held, base) {
		result = append(result, append([]int{}, base...))
	}
	for old, n := range base {
		if n == 0 {
			continue
		}
		for color := 0; color < 5; color++ {
			if color == old {
				continue
			}
			cost := append([]int{}, base...)
			cost[old]--
			cost[color]++
			if catanHas(held, cost) {
				result = append(result, cost)
			}
		}
	}
	return result
}

func (s *State) catanHelperBotChoices(player int, builds []botChoice, road int) []botChoice {
	g := s.Catan
	p := g.Players[player]
	h := p.Helper
	if h == nil || !g.helperReady(player, h.ID) {
		return nil
	}
	choices := []botChoice{}
	add := func(a Action, score int) { choices = append(choices, botChoice{a, score}) }
	for _, build := range builds {
		a := build.action
		if h.ID == 2 && a.Type == "catan_road" || h.ID == 6 && a.Type == "catan_buy_dev" {
			for _, cost := range helperCostChoices(catanPrices[a.Type], p.Resources) {
				a.Skill = "helper"
				a.Tokens = cost
				add(a, build.score+10)
			}
		}
		if h.ID == 8 && p.Knights > 0 && (a.Type == "catan_settlement" || a.Type == "catan_city") {
			a.Skill = "helper"
			add(a, build.score+10)
		}
		if h.ID == 9 {
			cost := catanBotBuildCost(a)
			for want, n := range cost {
				if n <= p.Resources[want] || g.Bank[want] == 0 {
					continue
				}
				for give := range p.Resources[:min(5, len(cost))] {
					if give == want || p.Resources[give]-cost[give] < 2 {
						continue
					}
					trade := Action{Type: "catan_helper", Give: make([]int, 5), Take: make([]int, 5)}
					trade.Give[give] = 2
					trade.Take[want] = 1
					add(trade, 150+build.score/20)
				}
			}
		}
	}
	if g.cityHelpers() {
		if h.ID == 6 {
			for track, deck := range g.CitiesKnights.ProgressDecks {
				if len(deck) > 0 {
					for _, cost := range helperCostChoices(catanPrices["catan_buy_dev"], p.Resources) {
						add(Action{Type: "catan_helper", Choice: "progress_buy", Color: track, Tokens: cost}, 280)
					}
				}
			}
		}
		if h.ID == 8 {
			for _, build := range builds {
				if build.action.Type != "catan_city" && build.action.Type != "catan_settlement" {
					continue
				}
				for _, n := range g.CitiesKnights.Knights {
					if n.Owner == player {
						a := build.action
						a.Skill = "helper"
						a.Target = n.Vertex
						add(a, build.score+10-n.Strength*3)
					}
				}
			}
		}
		if h.ID == 12 {
			for _, card := range g.CitiesKnights.Players[player].Progress {
				add(Action{Type: "catan_helper", Card: card}, 50-g.cityHelperProgressValue(player, card))
			}
		}
	}
	switch h.ID {
	case 1:
		give, want := 0, 0
		for i := range p.Resources[:5] {
			if p.Resources[i] > p.Resources[give] {
				give = i
			}
			if p.Resources[i] < p.Resources[want] {
				want = i
			}
		}
		if p.Resources[give] > 0 && give != want {
			// Select by public hand size only, never by opponents' composition.
			for i, target := range g.Players {
				if i != player && !target.Eliminated && sum(target.Resources) > 0 {
					add(Action{Type: "catan_helper", Color: want, Targets: []int{i}, Cards: []int{give}}, 180)
				}
			}
		}
	case 4:
		if road >= 0 {
			for _, e := range g.Edges {
				if e.ID != road && g.helperEndRoad(player, e.ID) {
					add(Action{Type: "catan_helper", Edge: e.ID, Target: road}, 190)
				}
			}
		}
	case 7:
		for i, target := range g.Players {
			if i != player && !target.Eliminated && target.Score-g.hiddenVictoryPoints(i) > p.Score-g.hiddenVictoryPoints(player) && sum(target.Resources) > 0 {
				add(Action{Type: "catan_helper", Target: i}, 700)
			}
		}
	case 10:
		if a, ok := g.digurBotAction(player); ok {
			add(a, 700)
		}
	case 11:
		for color := 0; color < 5; color++ {
			add(Action{Type: "catan_helper", Color: color}, 700-p.Resources[color]*3)
		}
	case 12:
		for kind, n := range p.Dev {
			if n > 0 && kind != 4 && (kind == 3 || kind == 0 && g.ArmyOwner == player) {
				add(Action{Type: "catan_helper", Card: kind}, 20)
			}
		}
	}
	return choices
}

func (g *Catan) digurBotAction(player int) (Action, bool) {
	a := Action{Type: "catan_helper"}
	if !g.helperReady(player, 10) || g.Robber < 0 || g.Robber >= len(g.Tiles) || g.Tiles[g.Robber].Resource == CatanDesert {
		return a, false
	}
	desert := false
	for _, t := range g.Tiles {
		desert = desert || (t.Resource == CatanDesert && g.clothLand(t.ID))
	}
	if !desert && !g.fishingHelpers() {
		return a, false
	}
	if g.Tiles[g.Robber].Resource == CatanGold || g.fishingHelpers() && g.Tiles[g.Robber].Resource == catanLake {
		best := -1
		for color, n := range g.Bank[:5] {
			if n > 0 && (best < 0 || g.Players[player].Resources[color] < g.Players[player].Resources[best]) {
				best = color
			}
		}
		if best >= 0 {
			a.Color = best
		}
	}
	return a, true
}

func (g *Catan) cityHelperProgressValue(player, card int) int {
	if catanProgressRules[card].Victory {
		return 100
	}
	switch card {
	case 3, 7, 8, 16:
		return 50
	case 5:
		if g.cityPiecesLeft(player) > 0 {
			return 45
		}
	}
	return 20
}
