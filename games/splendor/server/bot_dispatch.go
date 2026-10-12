package splendor

import (
	"errors"
	"sort"
)

func (s *State) BotAction(player int) (Action, error) {
	if s.Finished {
		return Action{}, errors.New("game finished")
	}
	if player < 0 || player >= len(s.Splendor.Players) || player != s.Turn || s.Splendor.Players[player].Eliminated {
		return Action{}, errors.New("inactive bot seat")
	}
	return s.gemBot(player)
}

type botChoice struct {
	action Action
	score  int
}

func (s *State) botLegal(player int, choices []botChoice) (Action, error) {
	sort.SliceStable(choices, func(i, j int) bool { return choices[i].score > choices[j].score })
	for _, c := range choices {
		trial := clone(*s)
		if trial.Apply(player, c.action) == nil {
			return c.action, nil
		}
	}
	return Action{}, errors.New("no legal bot action")
}

func gemMissing(p GemPlayer, c Card, tokens []int) int {
	if !gemCanAcquire(p, c) {
		return 20
	}
	if c.Orient == GemOrientSacrifice {
		count := 0
		for _, owned := range p.Cards {
			if owned.Color == c.SacrificeColor {
				count++
			}
		}
		return max(0, 2-count) * 5
	}
	need := 0
	value := 1
	if p.hasPost(GemPostDoubleGold) {
		value = 2
	}
	for i, cost := range c.Cost {
		need += (max(0, cost-p.Bonus[i]-tokens[i]) + value - 1) / value
	}
	virtual := 0
	for _, owned := range p.Cards {
		if owned.Orient == GemOrientGold {
			virtual += 2
		}
	}
	return max(0, need-tokens[5]-virtual)
}
func (s *State) gemBot(player int) (Action, error) {
	g := s.Splendor
	p := g.Players[player]
	if a, handled, err := s.gemExpansionBot(player); handled {
		return a, err
	}
	cards := append([]Card{}, p.Reserved...)
	for _, row := range g.Market {
		for _, c := range row {
			if c.ID > 0 && g.gemCardAccessible(c.ID, player) {
				cards = append(cards, c)
			}
		}
	}
	// A nearby affordable card gives the bot a consistent collection/discard goal.
	target := Card{Cost: make([]int, 5)}
	best := int(^uint(0) >> 1)
	for _, c := range cards {
		cost := 0
		for i, n := range c.Cost {
			cost += max(0, n-p.Bonus[i])
		}
		rank := gemMissing(p, c, p.Tokens)*12 + cost - c.Points*2
		if cost > 10 {
			rank += 100
		}
		if rank < best {
			best = rank
			target = c
		}
	}
	if s.Phase == "discard" {
		held := append([]int{}, p.Tokens...)
		give := make([]int, 6)
		for sum(held) > 10 {
			color, priority := -1, -999
			for i, n := range held {
				if n == 0 {
					continue
				}
				score := n * 2
				if i == 5 {
					score -= 100
				} else {
					score -= max(0, target.Cost[i]-p.Bonus[i]) * 5
				}
				if score > priority {
					color, priority = i, score
				}
			}
			held[color]--
			give[color]++
		}
		return s.botLegal(player, []botChoice{{Action{Type: "discard", Tokens: give}, 0}})
	}
	if s.Phase == "noble" {
		choices := []botChoice{}
		for _, n := range g.Nobles {
			if eligible(&p, n) {
				choices = append(choices, botChoice{Action{Type: "noble", Noble: n.ID}, 0})
			}
		}
		return s.botLegal(player, choices)
	}
	choices := []botChoice{}
	for _, c := range cards {
		if a, ok := gemBestPurchase(p, c); ok {
			score := 10000 + c.Points*100 + sum(c.Cost)
			if c.Color >= 0 && c.Color < 5 {
				score -= p.Bonus[c.Color] * 3
			}
			choices = append(choices, botChoice{a, score})
		}
	}
	// Enumerate the small set of legal take patterns, preferring useful colors.
	available := 0
	for i := 0; i < 5; i++ {
		if g.Bank[i] > 0 {
			available++
		}
	}
	for mask := 1; mask < 32; mask++ {
		take := make([]int, 6)
		for i := 0; i < 5; i++ {
			if mask&(1<<i) != 0 {
				take[i] = 1
			}
		}
		if sum(take) == min(3, available) {
			valid := true
			for i, n := range take {
				if n > g.Bank[i] {
					valid = false
				}
			}
			if valid {
				choices = append(choices, botChoice{Action{Type: "take", Tokens: take}, gemTakeScore(p, target, take)})
			}
		}
	}
	for i := 0; i < 5; i++ {
		if g.Bank[i] >= 4 {
			take := make([]int, 6)
			take[i] = 2
			choices = append(choices, botChoice{Action{Type: "take", Tokens: take}, gemTakeScore(p, target, take)})
		}
	}
	if len(p.Reserved) < 3 {
		for _, row := range g.Market {
			for _, c := range row {
				score := -50 - gemMissing(p, c, p.Tokens)*5
				if g.Bank[5] > 0 {
					score += 80
				}
				if c.ID == target.ID {
					score += 30
				}
				choices = append(choices, botChoice{Action{Type: "reserve", Card: c.ID}, score})
			}
		}
		for i, deck := range g.Decks {
			if len(deck) > 0 {
				a := Action{Type: "reserve", Tier: i%3 + 1}
				if i >= 3 {
					a.Choice = "orient"
				}
				choices = append(choices, botChoice{a, -500})
			}
		}
	}
	choices = append(choices, botChoice{Action{Type: "pass"}, -1000})
	return s.botLegal(player, choices)
}
func gemTakeScore(p GemPlayer, target Card, take []int) int {
	held := append([]int{}, p.Tokens...)
	useful := 0
	for i, n := range take {
		held[i] += n
		if i < 5 {
			useful += min(n, max(0, target.Cost[i]-p.Bonus[i]-p.Tokens[i]))
		}
	}
	return 100 + useful*40 - gemMissing(p, target, held)*15 - max(0, sum(held)-10)*20
}
