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
		chosen := legal[0]
		best := -1 << 30
		for _, move := range legal {
			score := len(move.Cards)*100 - move.High
			if move.Kind == "bomb" || move.Kind == "rocket" {
				score -= 1000
			}
			if s.LastPlay != nil {
				score = -move.High
				if move.Kind == "bomb" || move.Kind == "rocket" {
					score -= 200 + move.Count
				}
			}
			if score > best {
				chosen = move
				best = score
			}
		}
		a.Type = "play"
		a.Cards = chosen.Cards
	default:
		return nil, nil
	}
	return json.Marshal(a)
}
