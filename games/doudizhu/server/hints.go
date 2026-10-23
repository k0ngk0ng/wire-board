package doudizhu

import (
	"slices"
	"sort"
)

// Rank legal suggestions using only this player's hand. A full-hand finish comes first. Ordinary responses come
// first; preserve held bombs before comparing rank. Split responses remain
// available, followed by bombs from weakest to strongest and finally the rocket.
func sortHints(moves []Move, hand []int, players int) {
	held := map[int]int{}
	for _, id := range hand {
		held[rank(id)]++
	}
	royalSize := 1
	if players == 4 {
		royalSize = 2
	}
	rocket := held[16] == royalSize && held[17] == royalSize

	splitBomb := func(m Move) bool {
		if m.Kind != "bomb" {
			return false
		}
		used := map[int]int{}
		for _, id := range m.Cards {
			used[rank(id)]++
		}
		for r, n := range used {
			if r < 16 && held[r] >= 4 && n < held[r] {
				return true
			}
		}
		return false
	}
	class := func(m Move) int {
		if m.Kind == "rocket" {
			return 1000
		}
		if m.Kind == "bomb" {
			if splitBomb(m) {
				return 200 + m.Count
			}
			return 100 + m.Count
		}
		return 0
	}
	damage := func(m Move) int {
		used := map[int]int{}
		for _, id := range m.Cards {
			used[rank(id)]++
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
		if rocket && (used[16] > 0 || used[17] > 0) && used[16]+used[17] < royalSize*2 {
			cost += 100
		}
		return cost
	}

	pairRank := func(m Move) int {
		if m.Kind != "trio_pair" {
			return 0
		}
		for _, id := range m.Cards {
			if rank(id) != m.High {
				return rank(id)
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
		c := class(m)
		w := 0
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
		return slices.Compare(a.move.Cards, b.move.Cards) < 0
	})
	for i, r := range ranked {
		moves[i] = r.move
	}
}
