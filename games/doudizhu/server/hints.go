package doudizhu

import (
	"slices"
	"sort"
)

// Rank legal responses using only this player's hand. Ordinary responses come
// first; preserve held bombs before comparing rank. Split responses remain
// available, followed by bombs from weakest to strongest and finally the rocket.
func sortResponseHints(moves []Move, hand []int, players int) {
	held := map[int]int{}
	for _, id := range hand {
		held[rank(id)]++
	}
	royalSize := 1
	if players == 4 {
		royalSize = 2
	}
	rocket := held[16] == royalSize && held[17] == royalSize
	class := func(m Move) int {
		if m.Kind == "rocket" {
			return 1000
		}
		if m.Kind == "bomb" {
			return 100 + m.Count
		}
		return 0
	}
	damage := func(m Move) int {
		touched := map[int]bool{}
		for _, id := range m.Cards {
			touched[rank(id)] = true
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
	sort.SliceStable(moves, func(i, j int) bool {
		a, b := moves[i], moves[j]
		ac, bc := class(a), class(b)
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
		return slices.Compare(a.Cards, b.Cards) < 0
	})
}
