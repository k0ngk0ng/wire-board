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
		threat := 100
		for seat, hand := range s.Hands {
			if seat%2 != p%2 && len(hand) > 0 {
				threat = min(threat, len(hand))
			}
		}
		chosen := chooseBotMove(s.Hands[p], c.Moves, s.LastPlay != nil, threat, s.Level)
		a.Type = "play"
		a.Cards = chosen.Cards
		a.Interpretation = chosen.Key
	default:
		return nil, nil
	}
	return json.Marshal(a)
}

// Bounded work: score the existing legal move list, without recursively
// enumerating future deals. Only own cards and public remaining counts enter.
func chooseBotMove(hand []int, moves []Move, responding bool, threat, level int) Move {
	held := map[int]int{}
	for _, id := range hand {
		held[natural(id)]++
	}
	royalSize := 2
	rocket := held[16] == royalSize && held[17] == royalSize
	chosen, best := moves[0], -1<<30
	for _, m := range moves {
		if len(m.Cards) == len(hand) {
			return m
		}
		used := map[int]int{}
		for _, id := range m.Cards {
			used[natural(id)]++
		}
		damage := 0
		for r, n := range used {
			if n >= held[r] {
				continue
			}
			if r < 16 && held[r] >= 4 {
				damage += 500
			} else if held[r] >= 2 {
				damage += 80
			}
		}
		if rocket && (used[16] > 0 || used[17] > 0) && used[16]+used[17] < royalSize*2 {
			damage += 600
		}
		score := len(m.Cards)*60 - m.High - damage
		if responding {
			score = -m.High - damage
		}
		if category(m.Combination) > 0 {
			score -= 350 + len(m.Cards)
		} else if threat <= 2 {
			if len(m.Cards) <= threat {
				score += m.High * 12
			}
			if !responding && len(m.Cards) > threat {
				score += 250
			}
		}
		for _, id := range m.Cards {
			if wild(id, level) {
				score -= 30
			}
		}
		if score > best {
			chosen, best = m, score
		}
	}
	return chosen
}
