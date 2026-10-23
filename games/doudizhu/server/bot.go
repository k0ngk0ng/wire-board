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
		if !s.shouldSpendBomb(p, chosen) {
			a.Type = "pass"
			break
		}
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
		if m.Kind == "bomb" || m.Kind == "rocket" {
			score -= 350 + len(m.Cards)*80
			if m.Kind == "rocket" {
				score -= 10000
			}
			if !responding {
				score = -100 - len(m.Cards)*10 - m.High - damage
				if m.Kind == "rocket" {
					score -= 200
				}
			}
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

// Compare the continuation unlocked by a bomb with the cost of losing
// control. No "wait until N cards" gate: efficient long groups and a feeding
// opportunity can justify a mid-game bomb; scattered hands and bomb wars do not.
func (s *State) shouldSpendBomb(p int, move Move) bool {
	if s.LastPlay == nil || !(move.Kind == "bomb" || move.Kind == "rocket") || len(move.Cards) == len(s.Hands[p]) {
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
	if limit := s.bombLimit(p); limit >= 0 {
		budget = max(0, limit-s.bombsUsed(p)-1)
	}
	plan := s.analyzeHand(left, budget)
	if plan.turns == 1 {
		return true
	}
	ownerLeft := len(s.Hands[s.LastPlay.Seat])
	for seat, hand := range s.Hands {
		if !(seat != p && (p == s.Landlord || seat == s.Landlord)) || len(hand) == 0 {
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
		if seat == p || !(p != s.Landlord && seat != s.Landlord) {
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
	if s.LastPlay.Kind == "bomb" || s.LastPlay.Kind == "rocket" {
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

// The unseen pool is inferable from the deck, our hand and public discards.
// It deliberately pools all other seats rather than reading their identities.
func (s *State) unbeatableBomb(p int, m Move) bool {
	if m.Kind == "rocket" {
		return true
	}
	copies, royals := 4, 1
	if s.Players == 4 {
		copies, royals = 8, 2
	}
	unseen := map[int]int{}
	for r := 3; r <= 15; r++ {
		unseen[r] = copies
	}
	unseen[16], unseen[17] = royals, royals
	for _, id := range s.Hands[p] {
		unseen[rank(id)]--
	}
	for _, id := range s.Played {
		unseen[rank(id)]--
	}
	if unseen[16] >= royals && unseen[17] >= royals {
		return false
	}
	for r := 3; r <= 15; r++ {
		n := unseen[r]
		if n > len(m.Cards) || n == len(m.Cards) && r > m.High {
			return false
		}
	}
	return true
}
