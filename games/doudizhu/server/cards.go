package doudizhu

import (
	"errors"
	"fmt"
	"slices"
	"sort"
)

// IDs identify physical cards: each pack has four 13-card suits then two jokers.
// This package owns its rank and combination rules; other games must not import it.
type Combination struct {
	Kind  string `json:"kind"`
	High  int    `json:"high"`
	Count int    `json:"count"`
	Units int    `json:"units"`
}
type Move struct {
	Cards []int `json:"cards"`
	Combination
}

func rank(id int) int {
	b := id % 54
	if b >= 52 {
		return b - 52 + 16
	}
	r := b%13 + 2
	if r == 2 {
		return 15
	}
	return r
}
func classify(ids []int, n int, allowStraight bool) (Combination, error) {
	invalid := func() (Combination, error) {
		return Combination{}, errors.New("这些牌不能组成当前规则允许的牌型")
	}
	if (n != 3 && n != 4) || len(ids) == 0 || len(ids) > 33 {
		return invalid()
	}
	counts := map[int]int{}
	seen := map[int]bool{}
	limit := 54
	if n == 4 {
		limit = 108
	}
	for _, id := range ids {
		if id < 0 || id >= limit || seen[id] {
			return invalid()
		}
		seen[id] = true
		counts[rank(id)]++
	}
	keys := make([]int, 0, len(counts))
	for r := range counts {
		keys = append(keys, r)
	}
	sort.Ints(keys)
	makeType := func(kind string, high, units int) (Combination, error) {
		return Combination{kind, high, len(ids), units}, nil
	}
	if (n == 3 && len(ids) == 2 && counts[16] == 1 && counts[17] == 1) || (n == 4 && len(ids) == 4 && counts[16] == 2 && counts[17] == 2) {
		return makeType("rocket", 17, 1)
	}
	if len(keys) == 1 {
		r, c := keys[0], len(ids)
		switch c {
		case 1:
			return makeType("single", r, 1)
		case 2:
			return makeType("pair", r, 1)
		case 3:
			if r < 16 {
				return makeType("triple", r, 1)
			}
		}
		if r < 16 && c >= 4 && ((n == 3 && c == 4) || (n == 4 && c <= 8)) {
			return makeType("bomb", r, c)
		}
	}
	if len(keys) == 2 {
		for _, r := range keys {
			if counts[r] == 3 && r < 16 {
				if len(ids) == 4 && n == 3 {
					return makeType("trio_single", r, 1)
				}
				if len(ids) == 5 {
					return makeType("trio_pair", r, 1)
				}
			}
		}
	}
	sequence := func(size, minUnits int) bool {
		if len(keys) < minUnits || keys[0] < 3 || keys[len(keys)-1] > 14 {
			return false
		}
		for i, r := range keys {
			if counts[r] != size || (i > 0 && r != keys[i-1]+1) {
				return false
			}
		}
		return true
	}
	if allowStraight && sequence(1, 5) {
		return makeType("straight", keys[len(keys)-1], len(keys))
	}
	if sequence(2, 3) {
		return makeType("pairs", keys[len(keys)-1], len(keys))
	}
	if sequence(3, 2) {
		return makeType("triples", keys[len(keys)-1], len(keys))
	}
	// Wings never borrow a fourth card from a body rank. An unchosen body rank
	// must not complete an adjacent triple, and a bomb is not four single wings.
	for _, wingSize := range []int{1, 2} {
		if wingSize == 1 && n == 4 {
			continue
		}
		stride := 3 + wingSize
		if len(ids)%stride != 0 {
			continue
		}
		units := len(ids) / stride
		if units < 2 {
			continue
		}
		for first := 3; first+units-1 <= 14; first++ {
			last := first + units - 1
			ok := true
			for r := first; r <= last; r++ {
				if counts[r] != 3 {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			left, groups := 0, 0
			for r, c := range counts {
				if r >= first && r <= last {
					continue
				}
				left += c
				groups++
				if wingSize == 2 && c != 2 {
					ok = false
				}
				if wingSize == 1 && (c >= 4 || (c == 3 && (r == first-1 || r == last+1))) {
					ok = false
				}
			}
			if wingSize == 1 && counts[16] > 0 && counts[17] > 0 {
				ok = false
			}
			if ok && left == units*wingSize && (wingSize == 1 || groups == units) {
				kind := "plane_single"
				if wingSize == 2 {
					kind = "plane_pair"
				}
				return makeType(kind, last, units)
			}
		}
	}
	if n == 3 && (len(ids) == 6 || len(ids) == 8) {
		for r, c := range counts {
			if c != 4 || r >= 16 {
				continue
			}
			ok := true
			other := 0
			for w, amount := range counts {
				if w == r {
					continue
				}
				other++
				if amount >= 4 || (len(ids) == 8 && amount != 2) {
					ok = false
				}
			}
			if counts[16] > 0 && counts[17] > 0 {
				ok = false
			}
			if ok && (len(ids) == 6 || other == 2) {
				kind := "four_single"
				if len(ids) == 8 {
					kind = "four_pair"
				}
				return makeType(kind, r, 1)
			}
		}
	}
	return invalid()
}
func beats(a, b Combination) bool {
	if b.Kind == "" {
		return true
	}
	if b.Kind == "rocket" {
		return false
	}
	if a.Kind == "rocket" {
		return true
	}
	if a.Kind == "bomb" {
		if b.Kind != "bomb" {
			return true
		}
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.High > b.High
	}
	return a.Kind == b.Kind && a.Count == b.Count && a.Units == b.Units && a.High > b.High
}
func sortHand(ids []int) {
	sort.Slice(ids, func(i, j int) bool {
		a, b := rank(ids[i]), rank(ids[j])
		if a != b {
			return a > b
		}
		return ids[i] < ids[j]
	})
}
func movesFor(hand []int, n int, allowStraight bool) []Move {
	groups := map[int][]int{}
	for _, id := range hand {
		groups[rank(id)] = append(groups[rank(id)], id)
	}
	keys := []int{}
	for r := range groups {
		keys = append(keys, r)
		sort.Ints(groups[r])
	}
	sort.Ints(keys)
	result := []Move{}
	seen := map[string]bool{}
	add := func(ids []int) {
		ids = append([]int{}, ids...)
		sort.Ints(ids)
		key := fmt.Sprint(ids)
		if seen[key] {
			return
		}
		c, e := classify(ids, n, allowStraight)
		if e != nil {
			return
		}
		seen[key] = true
		result = append(result, Move{ids, c})
	}
	take := func(r, k int) []int { return append([]int{}, groups[r][:k]...) }
	for _, r := range keys {
		size := len(groups[r])
		for k := 1; k <= size; k++ {
			add(take(r, k))
		}
		if size >= 3 && r < 16 {
			for _, w := range keys {
				if r == w {
					continue
				}
				if n == 3 {
					add(append(take(r, 3), take(w, 1)...))
				}
				if len(groups[w]) >= 2 {
					add(append(take(r, 3), take(w, 2)...))
				}
			}
		}
		if n == 3 && size >= 4 && r < 16 {
			for i, a := range keys {
				if a == r {
					continue
				}
				for _, b := range keys[i:] {
					if b == r {
						continue
					}
					if a == b {
						if len(groups[a]) >= 2 {
							add(append(take(r, 4), take(a, 2)...))
						}
					} else {
						cards := append(take(r, 4), take(a, 1)...)
						add(append(cards, take(b, 1)...))
						if len(groups[a]) >= 2 && len(groups[b]) >= 2 {
							cards = append(take(r, 4), take(a, 2)...)
							add(append(cards, take(b, 2)...))
						}
					}
				}
			}
		}
	}
	kingCount := 1
	if n == 4 {
		kingCount = 2
	}
	if len(groups[16]) >= kingCount && len(groups[17]) >= kingCount {
		add(append(take(16, kingCount), take(17, kingCount)...))
	}
	for size := 1; size <= 3; size++ {
		minUnits := 2
		if size == 1 {
			minUnits = 5
		}
		if size == 2 {
			minUnits = 3
		}
		for first := 3; first <= 14; first++ {
			body := []int{}
			for last := first; last <= 14; last++ {
				if len(groups[last]) < size {
					break
				}
				body = append(body, take(last, size)...)
				units := last - first + 1
				if units < minUnits {
					continue
				}
				add(body)
				if size != 3 {
					continue
				}
				for _, wingSize := range []int{1, 2} {
					if wingSize == 1 && n == 4 {
						continue
					}
					wings := []int{}
					needed := units * wingSize
					for _, r := range keys {
						if r >= first && r <= last {
							continue
						}
						if wingSize == 2 {
							if len(groups[r]) >= 2 && len(wings)+2 <= needed {
								wings = append(wings, take(r, 2)...)
							}
						} else {
							maxTake := min(len(groups[r]), needed-len(wings), 3)
							if r == first-1 || r == last+1 {
								maxTake = min(maxTake, 2)
							}
							if r == 17 {
								hasSmall := false
								for _, id := range wings {
									if rank(id) == 16 {
										hasSmall = true
									}
								}
								if hasSmall {
									maxTake = 0
								}
							}
							if maxTake > 0 {
								wings = append(wings, take(r, maxTake)...)
							}
						}
						if len(wings) == needed {
							break
						}
					}
					if len(wings) == needed {
						add(append(append([]int{}, body...), wings...))
					}
				}
			}
		}
	}
	slices.SortFunc(result, func(a, b Move) int {
		if a.Kind != b.Kind {
			if a.Kind < b.Kind {
				return -1
			}
			return 1
		}
		if a.Count != b.Count {
			return a.Count - b.Count
		}
		return a.High - b.High
	})
	return result
}
