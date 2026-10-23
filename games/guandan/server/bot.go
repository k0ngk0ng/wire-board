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
		if !s.shouldSpendBomb(p, chosen) {
			a.Type = "pass"
			break
		}
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
				if responding && threat > 2 {
					damage += 2500
				}
			} else if held[r] >= 2 {
				damage += 80
			}
		}
		if rocket && (used[16] > 0 || used[17] > 0) && used[16]+used[17] < royalSize*2 {
			damage += 600
			if responding && threat > 2 {
				damage += 15000
			}
		}
		score := len(m.Cards)*60 - m.High - damage
		if responding {
			score = -m.High - damage
		}
		if category(m.Combination) > 0 {
			score -= 350 + category(m.Combination)*30
			if !responding {
				score = -100 - category(m.Combination)*3 - m.High - damage
			}
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

// Compare the continuation unlocked by a bomb with the cost of losing
// control. No "wait until N cards" gate: efficient long groups and a feeding
// opportunity can justify a mid-game bomb; scattered hands and bomb wars do not.
func (s *State) shouldSpendBomb(p int, move Move) bool {
	if s.LastPlay == nil || !(category(move.Combination) > 0) || len(move.Cards) == len(s.Hands[p]) {
		return true
	}
	used := map[int]bool{}
	for _, id := range move.Cards {
		used[id] = true
	}
	left := []int{}
	for _, id := range s.Hands[p] {
		if !used[id] {
			left = append(left, id)
		}
	}
	budget := -1
	plan := s.analyzeHand(left, budget)
	if plan.turns == 1 {
		return true
	}
	ownerLeft := len(s.Hands[s.LastPlay.Seat])
	for seat, hand := range s.Hands {
		if !(seat%2 != p%2) || len(hand) == 0 {
			continue
		}
		// A last-card opponent matters when they own the lead or can finish in
		// the current ordinary shape, not merely because their counter is small.
		if seat == s.LastPlay.Seat && len(hand) <= 2 || s.LastPlay.Kind == "single" && len(hand) == 1 || s.LastPlay.Kind == "pair" && len(hand) == 2 {
			return true
		}
	}
	benefit := max(0, 4-plan.turns) * 2
	if plan.largest >= 5 {
		benefit += 2
	}
	if plan.control {
		benefit += 2
	}
	if s.LastPlay.High >= 13 {
		benefit++
	}
	if ownerLeft <= 5 {
		benefit += 3
	} else if ownerLeft <= 8 {
		benefit++
	}
	for seat, hand := range s.Hands {
		if seat == p || !(seat%2 == p%2) {
			continue
		}
		if len(hand) == 1 && plan.feedOne || len(hand) == 2 && plan.feedPair {
			benefit += 3
			break
		}
	}
	if s.unbeatableBomb(p, move) {
		benefit++
	}
	cost := 2 + plan.singles
	if category(s.LastPlay.Combination) > 0 {
		cost += 3
	}
	if move.Kind == "rocket" {
		cost++
	}
	if plan.turns >= 5 {
		cost += 3
	}
	return benefit-cost >= 3
}

// A conservative public-card bound: unknown cards are pooled across all other
// seats. This is a small control bonus, never a claim about a specific hand.
func (s *State) unbeatableBomb(p int, m Move) bool {
	if m.Kind == "rocket" {
		return true
	}
	var face [54]int
	for i := range face {
		face[i] = 2
	}
	for _, id := range s.Hands[p] {
		face[id%54]--
	}
	for _, id := range s.Played {
		face[id%54]--
	}
	if face[52] >= 2 && face[53] >= 2 {
		return false
	}
	wilds := max(0, face[26+s.Level-2])
	ownClass := category(m.Combination)
	for r := 2; r <= 14; r++ {
		n := 0
		for suit := 0; suit < 4; suit++ {
			if suit == 2 && r == s.Level {
				continue
			}
			n += max(0, face[suit*13+r-2])
		}
		n += wilds
		if n >= 4 {
			c := category(Combination{Kind: "bomb", Count: n})
			if c > ownClass || c == ownClass && strength(r, s.Level) > m.High {
				return false
			}
		}
	}
	if ownClass <= 20 {
		for suit := 0; suit < 4; suit++ {
			for start := 1; start <= 10; start++ {
				if ownClass == 20 && start+4 <= m.High {
					continue
				}
				missing := 0
				for _, r := range consecutive(start, 5) {
					if suit == 2 && r == s.Level || face[suit*13+r-2] <= 0 {
						missing++
					}
				}
				if missing <= wilds {
					return false
				}
			}
		}
	}
	return true
}
