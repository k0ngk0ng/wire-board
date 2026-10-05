package game

import "errors"

func (s *State) catanTradeProgressChoiceBot(player int) (Action, error) {
	g := s.Catan
	q := g.CitiesKnights.Pending
	if q.Kind == "guild_dues" {
		// This one hand has explicitly been revealed to this player by the card.
		hand := append([]int{}, g.Players[q.Target].Resources...)
		take := make([]int, len(g.Bank))
		for range min(2, sum(hand)) {
			best, value := -1, -1<<30
			for c, n := range hand {
				if n == 0 {
					continue
				}
				score := 20 - 3*(g.Players[player].Resources[c]+take[c])
				if c >= 5 {
					score += 4
				}
				if score > value {
					best, value = c, score
				}
			}
			take[best]++
			hand[best]--
		}
		return Action{Type: "catan_guild_dues", Take: take}, nil
	}
	if q.Kind == "commercial_harbor" {
		best := -1
		for c := 5; c < 8; c++ {
			if g.Players[player].Resources[c] > 0 && (best < 0 || g.Players[player].Resources[c] > g.Players[player].Resources[best]) {
				best = c
			}
		}
		if best >= 0 {
			return Action{Type: "catan_commercial_harbor", Color: best}, nil
		}
	}
	return Action{}, errors.New("no legal trade progress response")
}

// Estimate monopoly types from PUBLIC terrain/buildings and hand counts, never
// the composition of an opponent's cards. Revealed Guild Dues choices are separate.
func (g *Catan) monopolyEstimate(player, color int) int {
	value := 0
	for _, t := range g.Tiles {
		terrain := color
		if color >= 5 {
			terrain = []int{0, 2, 4}[color-5]
		}
		if t.Resource != terrain || t.Number == 0 {
			continue
		}
		for _, id := range t.Vertices {
			v := g.Vertices[id]
			if v.Owner < 0 || v.Owner == player || v.Level == 0 || g.Players[v.Owner].Eliminated || sum(g.Players[v.Owner].Resources) == 0 {
				continue
			}
			if color >= 5 && v.Level != 2 {
				continue
			}
			value += 6 - absCatan(7-t.Number)
		}
	}
	return value
}
func (g *Catan) tradeProgressBotChoices(player int) []botChoice {
	k := g.CitiesKnights
	if k == nil {
		return nil
	}
	choices := []botChoice{}
	hand := g.Players[player].Resources
	// Offers depend only on this hand and other seats' publicly visible counts.
	resource := -1
	for c, n := range hand[:5] {
		if n > 0 && (resource < 0 || n > hand[resource]) {
			resource = c
		}
	}
	if powers := k.TradePowers; powers != nil && powers.Player == player && resource >= 0 {
		for id, remaining := range powers.Harbors {
			for _, target := range remaining {
				if !g.Players[target].Eliminated && sum(g.Players[target].Resources) > 0 {
					choices = append(choices, botChoice{Action{Type: "catan_commercial_offer", Card: id, Target: target, Color: resource}, 720})
				}
			}
		}
	}
	seen := map[int]bool{}
	for _, card := range k.Players[player].Progress {
		if seen[card] {
			continue
		}
		seen[card] = true
		a := Action{Type: "catan_progress", Card: card}
		switch card {
		case 10:
			if resource >= 0 {
				for p, seat := range g.Players {
					if p != player && !seat.Eliminated && sum(seat.Resources) > 0 {
						choices = append(choices, botChoice{a, 700})
						break
					}
				}
			}
		case 11:
			best := -1
			for _, p := range g.guildDuesTargets(player) {
				if sum(g.Players[p].Resources) > 0 && (best < 0 || sum(g.Players[p].Resources) > sum(g.Players[best].Resources)) {
					best = p
				}
			}
			if best >= 0 {
				a.Target = best
				choices = append(choices, botChoice{a, 810})
			}
		case 12:
			for _, tile := range g.merchantTiles(player) {
				score := 780
				if k.Merchant != nil && k.Merchant.Owner == player {
					score = 0
				}
				c := g.Tiles[tile].Resource
				if c < 5 && g.rates(player)[c] > 2 && hand[c] >= 2 {
					score += 60 + hand[c]
				}
				if score > 0 {
					a.Tile = tile
					choices = append(choices, botChoice{a, score})
				}
			}
		case 13:
			for c, n := range hand {
				if n >= 2 && g.rates(player)[c] > 2 {
					a.Color = c
					choices = append(choices, botChoice{a, 670 + n})
				}
			}
		case 14, 15:
			low, high := 0, 5
			if card == 15 {
				low, high = 5, 8
			}
			best, value := -1, -1<<30
			for c := low; c < high; c++ {
				score := g.monopolyEstimate(player, c)*4 - hand[c]
				if score > value {
					best, value = c, score
				}
			}
			count := 0
			for p, seat := range g.Players {
				if p != player && !seat.Eliminated {
					count += sum(seat.Resources)
				}
			}
			if count > 0 {
				a.Color = best
				choices = append(choices, botChoice{a, 680 + max(0, value)})
			}
		}
	}
	return choices
}
