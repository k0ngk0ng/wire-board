package doudizhu

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Decisions consume only this actor's hand plus public roles, counts and plays.
// No opponent card identities are used for bidding, doubling, hints or play.
func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	p := ctx.Actor
	if !s.canAct(p) {
		return nil, nil
	}
	a := Action{Window: s.Window}
	own := s.Hands[p]
	switch s.Phase {
	case "landlord_choice":
		a.Type = "landlord"
		for i := 0; i < min(2, len(s.reportCandidates())); i++ {
			a.Reports = append(a.Reports, i)
		}
	case "bid":
		a.Type = "bid"
		power := 0
		groups := map[int]int{}
		for _, id := range own {
			r := rank(id)
			groups[r]++
			if r >= 15 {
				power++
			}
		}
		for r, c := range groups {
			if r < 16 && c >= 4 {
				power += 2
			}
		}
		bid := 0
		if power >= 8 {
			bid = 3
		} else if power >= 5 {
			bid = 2
		} else if power >= 2 {
			bid = 1
		}

		if bid > s.Bid {
			a.Bid = bid
		} else if s.Bid == 0 && s.BidSteps == s.Players-1 {
			a.Bid = 1
		}
	case "double", "redouble":
		a.Type = "double"
		power := 0
		for _, id := range own {
			if rank(id) >= 15 {
				power++
			}
		}
		a.Double = power >= 4
	case "round_end":
		a.Type = "ready"
	case "play":
		legal := s.controls(p).Moves
		if len(legal) == 0 {
			a.Type = "pass"
			break
		}
		for _, move := range legal {
			if len(move.Cards) == len(own) {
				a.Type = "play"
				a.Cards = move.Cards
				return json.Marshal(a)
			}
		}
		// A farmer does not overtake their partner merely to spend a card.
		if s.LastPlay != nil && p != s.Landlord && s.LastPlay.Seat != s.Landlord {
			a.Type = "pass"
			break
		}
		threat := 100
		for seat, hand := range s.Hands {
			if seat != p && (p == s.Landlord || seat == s.Landlord) && len(hand) > 0 {
				threat = min(threat, len(hand))
			}
		}
		chosen := chooseBotMove(own, legal, s.LastPlay != nil, threat, s.Players)
		a.Type = "play"
		a.Cards = chosen.Cards
	default:
		return nil, nil
	}
	return json.Marshal(a)
}

// Bounded work: score the existing legal move list, without recursively
// enumerating future deals. Only own cards and public remaining counts enter.
func chooseBotMove(hand []int, moves []Move, responding bool, threat, players int) Move {
	held := map[int]int{}
	for _, id := range hand {
		held[rank(id)]++
	}
	royalSize := 1
	if players == 4 {
		royalSize = 2
	}
	rocket := held[16] == royalSize && held[17] == royalSize
	chosen, best := moves[0], -1<<30
	for _, m := range moves {
		if len(m.Cards) == len(hand) {
			return m
		}
		used := map[int]int{}
		for _, id := range m.Cards {
			used[rank(id)]++
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
		if m.Kind == "bomb" || m.Kind == "rocket" {
			score -= 350 + len(m.Cards)
		} else if threat <= 2 {
			if len(m.Cards) <= threat {
				score += m.High * 12
			}
			if !responding && len(m.Cards) > threat {
				score += 250
			}
		}

		if score > best {
			chosen, best = m, score
		}
	}
	return chosen
}
