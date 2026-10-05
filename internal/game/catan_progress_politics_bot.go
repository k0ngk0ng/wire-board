package game

import "errors"

func catanProgressValue(card int) int {
	switch card {
	case 0, 1, 5, 7, 8, 11, 12, 16, 18, 19, 22:
		return 80
	case 4, 6, 10, 13, 14, 15, 20, 24:
		return 60
	default:
		return 40
	}
}
func (s *State) catanPoliticsChoiceBot(player int) (Action, error) {
	g := s.Catan
	k := g.CitiesKnights
	q := k.Pending
	a := Action{Type: "catan_" + q.Kind}
	switch q.Kind {
	case "diplomacy":
		best, value := -1, -1<<30
		for _, edge := range g.diplomacyPlacements(player) {
			next := clone(*g)
			next.Edges[edge].Owner = player
			e := g.Edges[edge]
			score := next.roadLength(player)*100 + g.vertexValue(player, e.A) + g.vertexValue(player, e.B)
			if score > value {
				best, value = edge, score
			}
		}
		if best < 0 {
			a.Choice = "skip"
		} else {
			a.Edge = best
		}
		return a, nil
	case "espionage":
		hand := k.Players[q.Target].Progress
		if len(hand) == 0 {
			a.Choice = "skip"
			return a, nil
		}
		a.Card = hand[0]
		for _, card := range hand {
			if catanProgressValue(card) > catanProgressValue(a.Card) {
				a.Card = card
			}
		}
		return a, nil
	case "sabotage", "wedding":
		hand := append([]int{}, g.Players[player].Resources...)
		due := min(2, sum(hand))
		if q.Kind == "sabotage" {
			due = sum(hand) / 2
		}
		a.Give = make([]int, len(g.Bank))
		for range due {
			best := -1
			for c, n := range hand {
				if n > 0 && (best < 0 || n > hand[best]) {
					best = c
				}
			}
			a.Give[best]++
			hand[best]--
		}
		return a, nil
	case "treason_remove":
		best, value := -1, 1<<30
		for _, n := range k.Knights {
			if n.Owner != player {
				continue
			}
			score := n.Strength * 100
			if n.Active {
				score += 40
			}
			if score < value {
				best, value = n.Vertex, score
			}
		}
		if best >= 0 {
			a.Vertex = best
			return a, nil
		}
	case "treason_place":
		if q.Knight == nil {
			break
		}
		sites := g.treasonPlacements(player, q.Knight.Strength)
		if len(sites) == 0 {
			a.Choice = "skip"
			return a, nil
		}
		a.Vertex = sites[0]
		for _, v := range sites {
			if g.vertexValue(player, v) > g.vertexValue(player, a.Vertex) {
				a.Vertex = v
			}
		}
		for strength := q.Knight.Strength; strength >= 1; strength-- {
			if g.knightCount(player, strength) < 2 {
				a.Color = strength
				return a, nil
			}
		}
	}
	return Action{}, errors.New("no legal politics response")
}
func (g *Catan) politicsBotChoices(player int) []botChoice {
	k := g.CitiesKnights
	if k == nil {
		return nil
	}
	choices := []botChoice{}
	seen := map[int]bool{}
	for _, card := range k.Players[player].Progress {
		if seen[card] {
			continue
		}
		seen[card] = true
		a := Action{Type: "catan_progress", Card: card}
		switch card {
		case 16:
			for _, id := range g.diplomacyRoads() {
				owner := g.Edges[id].Owner
				if owner == player {
					continue
				}
				a.Edge = id
				choices = append(choices, botChoice{a, 710})
			}
		case 17:
			inactive := 0
			for _, n := range k.Knights {
				if n.Owner == player && !n.Active {
					inactive += n.Strength
				}
			}
			if inactive > 0 {
				choices = append(choices, botChoice{a, 760 + inactive*10})
			}
		case 18:
			best := -1
			for p := range g.Players {
				if g.politicsOpponent(player, p) && len(k.Players[p].Progress) > 0 && (best < 0 || len(k.Players[p].Progress) > len(k.Players[best].Progress)) {
					best = p
				}
			}
			if best >= 0 {
				a.Target = best
				choices = append(choices, botChoice{a, 810})
			}
		case 19:
			for _, v := range g.intrigueTargets(player) {
				n := g.knightAt(v)
				score := 700 + n.Strength*10
				if len(g.knightDestinations(*n, true)) == 0 {
					score += 80
				}
				a.Vertex = v
				choices = append(choices, botChoice{a, score})
			}
		case 20, 24:
			own := g.Players[player].Score - g.hiddenVictoryPoints(player)
			value := 0
			for p, seat := range g.Players {
				if !g.politicsOpponent(player, p) {
					continue
				}
				score := seat.Score - g.hiddenVictoryPoints(p)
				if card == 20 && score >= own {
					value += sum(seat.Resources) / 2
				}
				if card == 24 && score > own {
					value += min(2, sum(seat.Resources))
				}
			}
			if value > 0 {
				choices = append(choices, botChoice{a, 720 + value*10})
			}
		case 21:
			if k.Invasions == 0 {
				continue
			}
			for _, t := range g.Tiles {
				if !g.robberAllowed(t.ID) {
					continue
				}
				value := 0
				victims := map[int]bool{}
				for _, id := range t.Vertices {
					v := g.Vertices[id]
					if v.Level == 0 || v.Owner < 0 || g.Players[v.Owner].Eliminated {
						continue
					}
					if v.Owner == player {
						value -= 50 * v.Level
						continue
					}
					if !victims[v.Owner] && sum(g.Players[v.Owner].Resources) > 0 {
						value += 60
						victims[v.Owner] = true
					}
					value += v.Level * (6 - absCatan(7-t.Number))
				}
				if value > 0 {
					a.Tile = t.ID
					choices = append(choices, botChoice{a, 700 + value})
				}
			}
		case 22:
			for p := range g.Players {
				if !g.politicsOpponent(player, p) {
					continue
				}
				weakest := 4
				for _, n := range k.Knights {
					if n.Owner == p {
						weakest = min(weakest, n.Strength)
					}
				}
				if weakest < 4 {
					a.Target = p
					choices = append(choices, botChoice{a, 780 + weakest*10})
				}
			}
		}
	}
	return choices
}
