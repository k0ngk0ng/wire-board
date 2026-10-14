package guandan

import (
	"slices"
	"sort"
)

// Hint ranking is local to Guandan: keep ordinary responses ahead of bombs,
// retain natural bombs/royal bombs when possible, and use this ruleset's level
// strength and straight-flush hierarchy. Wild declarations stay with each move.
func sortResponseHints(moves []Move, hand []int, level int) {
	held := map[int]int{}
	for _, id := range hand {
		held[natural(id)]++
	}
	rocket := held[16] == 2 && held[17] == 2
	damage := func(m Move) int {
		touched := map[int]bool{}
		for _, id := range m.Cards {
			touched[natural(id)] = true
		}
		cost := 0
		for r := range touched {
			if r < 16 && held[r] >= 4 {
				cost++
			}
		}
		if rocket && (touched[16] || touched[17]) {
			cost++
		}
		return cost
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
	sort.SliceStable(moves, func(i, j int) bool {
		a, b := moves[i], moves[j]
		ac, bc := category(a.Combination), category(b.Combination)
		if ac != bc {
			return ac < bc
		}
		if ac == 0 {
			ad, bd := damage(a), damage(b)
			if ad != bd {
				return ad < bd
			}
		}
		if a.High != b.High {
			return a.High < b.High
		}
		if aw, bw := wildCount(a), wildCount(b); aw != bw {
			return aw < bw
		}
		if c := slices.Compare(a.Cards, b.Cards); c != 0 {
			return c < 0
		}
		return a.Key < b.Key
	})
}
