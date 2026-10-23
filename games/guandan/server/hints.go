package guandan

import (
	"slices"
	"sort"
)

// Hint ranking is local to Guandan: keep ordinary responses ahead of bombs,
// retain natural bombs/royal bombs when possible, and use this ruleset's level
// strength and straight-flush hierarchy. Wild declarations stay with each move.
func sortHints(moves []Move, hand []int, level int) {
	held := map[int]int{}
	for _, id := range hand {
		held[natural(id)]++
	}
	rocket := held[16] == 2 && held[17] == 2
	damage := func(m Move) int {
		used := map[int]int{}
		for _, id := range m.Cards {
			used[natural(id)]++
		}
		cost := 0
		for r, n := range used {
			if n >= held[r] {
				continue
			}
			if r < 16 && held[r] >= 4 {
				cost += 100
			} else if held[r] == 3 {
				cost += 10
			} else if held[r] == 2 {
				cost += 1
			}
		}
		if rocket && (used[16] > 0 || used[17] > 0) && used[16]+used[17] < 4 {
			cost += 100
		}
		return cost
	}

	splitBomb := func(m Move) bool {
		if m.Kind != "bomb" {
			return false
		}
		used := map[int]int{}
		for _, id := range m.Cards {
			used[natural(id)]++
		}
		for r, n := range used {
			if r < 16 && held[r] >= 4 && n < held[r] {
				return true
			}
		}
		return false
	}
	wildCount := func(m Move) int {
		n := 0
		for _, id := range m.Cards {
			if wild(id, level) {
				n++
			}
		}
		return n
	}

	pairRank := func(m Move) int {
		if m.Kind != "full_house" {
			return 0
		}
		assigned := map[int]int{}
		for _, w := range m.Wild {
			assigned[w.Card] = w.Rank
		}
		counts := map[int]int{}
		for _, id := range m.Cards {
			r := natural(id)
			if v, ok := assigned[id]; ok {
				r = v
			}
			counts[r]++
		}
		for r, n := range counts {
			if n == 2 {
				return strength(r, level)
			}
		}
		return 0
	}
	type rankedMove struct {
		move                         Move
		category, damage, pair, wild int
		finish                       bool
	}
	ranked := make([]rankedMove, len(moves))
	for i, m := range moves {
		c := category(m.Combination)
		if splitBomb(m) {
			c += 200
		}
		if m.Kind == "rocket" {
			c = 1000
		}
		w := wildCount(m)
		ranked[i] = rankedMove{m, c, damage(m), pairRank(m), w, len(m.Cards) == len(hand)}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if a.finish != b.finish {
			return a.finish
		}
		if a.category != b.category {
			return a.category < b.category
		}
		if a.category == 0 && a.damage != b.damage {
			return a.damage < b.damage
		}
		if a.move.High != b.move.High {
			return a.move.High < b.move.High
		}
		if a.pair != b.pair {
			return a.pair < b.pair
		}
		if a.wild != b.wild {
			return a.wild < b.wild
		}
		if c := slices.Compare(a.move.Cards, b.move.Cards); c != 0 {
			return c < 0
		}
		return a.move.Key < b.move.Key
	})
	for i, r := range ranked {
		moves[i] = r.move
	}
}
