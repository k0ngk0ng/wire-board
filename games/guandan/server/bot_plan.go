package guandan

import (
	"math/bits"
	"sort"
)

// A bounded local continuation search. Rank-equivalent physical copies share
// a count; Guandan wild cards have their own group. Only legal lead templates
// from this hand enter the search. A result is a playable upper bound, not a
// claim of exact optimal play or a trained neural policy.
type handPlan struct {
	turns, singles, largest, nodes int
	control, feedOne, feedPair     bool
}
type planShape struct {
	need   uint64
	groups uint16
	count  int
	bomb   bool
}
type planKey struct {
	hand  uint64
	bombs int
}

func (s *State) analyzeHand(hand []int, bombBudget int) handPlan {
	moves := movesFor(hand, s.Level)
	group := func(id int) int {
		if wild(id, s.Level) {
			return 15
		}
		r := natural(id)
		if r >= 16 {
			return r - 3
		}
		return r - 2
	}
	var mask uint64
	for _, id := range hand {
		mask += uint64(1) << uint(group(id)*4)
	}
	unique := map[uint64]planShape{}
	control, feedOne, feedPair := false, false, false
	for _, m := range moves {
		bomb := category(m.Combination) > 0
		if bomb && bombBudget != 0 {
			control = true
		}
		if m.High <= 10 {
			feedOne = feedOne || m.Kind == "single"
			feedPair = feedPair || m.Kind == "pair"
		}
		// Every straight flush is also an ordinary straight for hand-count planning.
		// Suit-dependent control is assessed separately, never fabricated by counts.
		if m.Kind == "straight_flush" {
			continue
		}
		shape := planShape{count: len(m.Cards), bomb: bomb}
		for _, id := range m.Cards {
			g := group(id)
			shape.need += uint64(1) << uint(g*4)
			shape.groups |= uint16(1) << uint(g)
		}
		old, ok := unique[shape.need]
		if !ok || old.bomb && !shape.bomb {
			unique[shape.need] = shape
		}
	}
	shapes := make([]planShape, 0, len(unique))
	for _, p := range unique {
		shapes = append(shapes, p)
	}
	sort.Slice(shapes, func(i, j int) bool {
		if shapes[i].count != shapes[j].count {
			return shapes[i].count > shapes[j].count
		}
		return shapes[i].need < shapes[j].need
	})
	var byGroup [16][]planShape
	for _, p := range shapes {
		for groups := p.groups; groups != 0; groups &= groups - 1 {
			g := bits.TrailingZeros16(groups)
			byGroup[g] = append(byGroup[g], p)
		}
	}
	memo := map[planKey]handPlan{}
	nodes := 0
	var solve func(uint64, int) handPlan
	solve = func(h uint64, quota int) handPlan {
		if h == 0 {
			return handPlan{}
		}
		key := planKey{h, quota}
		if p, ok := memo[key]; ok {
			return p
		}
		count := 0
		for v := h; v != 0; v >>= 4 {
			count += int(v & 15)
		}
		best := handPlan{turns: count, singles: count, largest: 1}
		if nodes >= 512 {
			return best
		}
		nodes++
		pivot := bits.TrailingZeros64(h) / 4
		for _, p := range byGroup[pivot] {
			if quota == 0 && p.bomb {
				continue
			}
			fits := true
			for groups := p.groups; groups != 0; groups &= groups - 1 {
				shift := uint(bits.TrailingZeros16(groups) * 4)
				if (h>>shift)&15 < (p.need>>shift)&15 {
					fits = false
					break
				}
			}
			if !fits {
				continue
			}
			q := quota
			if p.bomb && q > 0 {
				q--
			}
			next := solve(h-p.need, q)
			next.turns++
			if p.count == 1 {
				next.singles++
			}
			next.largest = max(next.largest, p.count)
			if next.turns < best.turns || next.turns == best.turns && next.singles < best.singles {
				best = next
			}
			if best.turns == 1 {
				break
			}
		}
		memo[key] = best
		return best
	}
	result := solve(mask, bombBudget)
	result.nodes = nodes
	result.control = control
	result.feedOne = feedOne
	result.feedPair = feedPair
	return result
}
