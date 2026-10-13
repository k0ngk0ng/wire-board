package guandan

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	p := ctx.Actor
	if !s.canAct(p) {
		return nil, nil
	}
	c := s.controls(p)
	a := Action{Window: s.Window}
	switch s.Phase {
	case "tribute":
		a.Type = "tribute"
		a.Card = c.Choices[0]
	case "return":
		a.Type = "return"
		a.Card = c.Choices[0]
		for _, id := range c.Choices {
			if strength(natural(id), s.Level) < strength(natural(a.Card), s.Level) {
				a.Card = id
			}
		}
	case "round_end":
		a.Type = "ready"
	case "play":
		for _, m := range c.Moves {
			if len(m.Cards) == len(s.Hands[p]) {
				a.Type = "play"
				a.Cards = m.Cards
				a.Interpretation = m.Key
				return json.Marshal(a)
			}
		}
		if len(c.Moves) == 0 || (s.LastPlay != nil && s.LastPlay.Seat%2 == p%2) {
			a.Type = "pass"
			break
		}
		best := -1 << 30
		chosen := c.Moves[0]
		for _, m := range c.Moves {
			score := len(m.Cards)*100 - m.High - len(m.Wild)*25
			if category(m.Combination) > 0 {
				score -= 650 + category(m.Combination)
			}
			if s.LastPlay != nil {
				score = -m.High - len(m.Wild)*25
				if category(m.Combination) > 0 {
					score -= 150 + category(m.Combination)
				}
			}
			if score > best {
				best = score
				chosen = m
			}
		}
		a.Type = "play"
		a.Cards = chosen.Cards
		a.Interpretation = chosen.Key
	default:
		return nil, nil
	}
	return json.Marshal(a)
}
