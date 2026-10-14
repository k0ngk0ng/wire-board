package shengji

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"slices"
	"sort"
)

// Suggestions and autoplay only read this player's hand and public state.
// In particular, they never call resolveThrow against the other hands.
func (s *State) moves(p int) []Move {
	h := append([]int{}, s.Hands[p]...)
	r := s.rules()
	sortHand(h, r)
	slices.Reverse(h) // Search cheap cards first, independently of visual order.
	out := []Move{}
	seen := map[string]bool{}
	add := func(ids []int) {
		if len(ids) == 0 || !owns(h, ids) {
			return
		}
		key := cardKey(ids)
		if seen[key] {
			return
		}
		label := "跟牌"
		if s.Lead != nil {
			if validateFollow(h, ids, *s.Lead, r) != nil {
				return
			}
			if p == s.ChallengeSeat && !beatsTrick(ids, s.Trick[0].Cards, *s.Lead, r) {
				return
			}
		} else {
			shape, e := leadShape(ids, r)
			if e != nil {
				return
			}
			label = shapeName(shape)
		}
		seen[key] = true
		out = append(out, Move{append([]int{}, ids...), label})
	}
	if s.Lead == nil {
		for _, id := range h {
			add([]int{id})
		}
		for suit := 0; suit <= 4; suit++ {
			nodes := pairsIn(ofSuit(h, suit, r), r)
			mask := uint64(1)<<len(nodes) - 1
			for n := 1; n <= len(nodes); n++ {
				cs := chains(nodes, mask, n)
				if len(cs) == 0 {
					break
				}
				for _, m := range cs {
					add(maskIDs(nodes, m))
				}
			}
		}
		if r.Decks == 1 {
			by := map[int][]int{}
			for _, id := range h {
				by[rank(id)] = append(by[rank(id)], id)
			}
			for _, ids := range by {
				if len(ids) == 4 {
					add(ids)
				}
			}
		}
		// Map iteration above must not affect deterministic bot choices.
		sort.SliceStable(out, func(i, j int) bool {
			if len(out[i].Cards) != len(out[j].Cards) {
				return len(out[i].Cards) > len(out[j].Cards)
			}
			return cardKey(out[i].Cards) < cardKey(out[j].Cards)
		})
		return out
	}
	lead := *s.Lead
	n := lead.Count
	if lead.Four {
		chosen := []int{}
		for suit := 0; suit < 4; suit++ {
			for _, id := range h {
				if printedSuit(id) == suit {
					chosen = append(chosen, id)
					break
				}
			}
		}
		for _, id := range h {
			if len(chosen) == n {
				break
			}
			if !slices.Contains(chosen, id) {
				chosen = append(chosen, id)
			}
		}
		add(chosen)
		by := map[int][]int{}
		for _, id := range h {
			by[rank(id)] = append(by[rank(id)], id)
		}
		for _, ids := range by {
			if len(ids) == 4 {
				add(ids)
			}
		}
	} else {
		held := ofSuit(h, lead.Suit, r)
		chosen := []int{}
		if len(held) < n {
			chosen = append(chosen, held...)
		} else {
			nodes := pairsIn(held, r)
			a := allocate(nodes, demands(lead))
			chosen = maskIDs(nodes, a.Mask)
			for _, id := range held {
				if len(chosen) == n {
					break
				}
				if !slices.Contains(chosen, id) {
					chosen = append(chosen, id)
				}
			}
		}
		for _, id := range h {
			if len(chosen) == n {
				break
			}
			if !slices.Contains(chosen, id) {
				chosen = append(chosen, id)
			}
		}
		add(chosen)
		if n == 1 {
			for _, id := range h {
				add([]int{id})
			}
		} else if len(lead.Parts) == 1 && n%2 == 0 {
			for suit := 0; suit <= 4; suit++ {
				nodes := pairsIn(ofSuit(h, suit, r), r)
				for _, mask := range chains(nodes, uint64(1)<<len(nodes)-1, n/2) {
					add(maskIDs(nodes, mask))
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return cardKey(out[i].Cards) < cardKey(out[j].Cards) })
	return out
}
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
	switch s.Phase {
	case "declare":
		a.Type = "pass"
		ds := s.declarations(p)
		if len(ds) > 0 && s.Declaration == nil {
			best := ds[0]
			score := -1
			for _, d := range ds {
				v := d.Strength * 10
				for _, id := range s.Hands[p] {
					if printedSuit(id) == d.Trump {
						v++
					}
				}
				if v > score {
					score = v
					best = d
				}
			}
			a.Type = "declare"
			a.Cards = best.Cards
		}
	case "bury":
		h := append([]int{}, s.Hands[p]...)
		r := s.rules()
		sort.Slice(h, func(i, j int) bool {
			cost := func(id int) int {
				v := power(id, r) + points([]int{id})*3
				if suitOf(id, r) == 4 {
					v += 100
				}
				return v
			}
			a, b := cost(h[i]), cost(h[j])
			if a != b {
				return a < b
			}
			return h[i] < h[j]
		})
		a.Type = "bury"
		a.Cards = h[:s.bottomCount()]
	case "play":
		ms := s.moves(p)
		if len(ms) == 0 {
			return nil, nil
		}
		chosen := ms[0]
		best := -1 << 30
		r := s.rules()
		winner := Play{}
		if len(s.Trick) > 0 {
			winner = s.Trick[0]
			for _, play := range s.Trick[1:] {
				if beatsTrick(play.Cards, winner.Cards, *s.Lead, r) {
					winner = play
				}
			}
		}
		for _, move := range ms {
			score := len(move.Cards) * 100
			cost := 0
			for _, id := range move.Cards {
				cost += power(id, r)
				if suitOf(id, r) == 4 {
					cost += 8
				}
			}
			score -= cost
			if s.Lead != nil {
				score = -cost
				if winner.Seat%2 == p%2 {
					score += points(move.Cards) * 4
				} else if beatsTrick(move.Cards, winner.Cards, *s.Lead, r) {
					score += 100
				} else {
					score -= points(move.Cards) * 4
				}
			}
			if score > best {
				best = score
				chosen = move
			}
		}
		a.Type = "play"
		a.Cards = chosen.Cards
	case "round_end":
		a.Type = "ready"
	default:
		return nil, nil
	}
	return json.Marshal(a)
}
