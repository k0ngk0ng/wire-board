package guandan

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

type Replacement struct {
	Card int `json:"card"`
	Rank int `json:"rank"`
	Suit int `json:"suit"`
}
type Combination struct {
	Key   string        `json:"key"`
	Kind  string        `json:"kind"`
	High  int           `json:"high"`
	Count int           `json:"count"`
	Wild  []Replacement `json:"wild"`
}
type Move struct {
	Cards []int `json:"cards"`
	Combination
}

func natural(id int) int {
	b := id % 54
	if b >= 52 {
		return b - 52 + 16
	}
	return b%13 + 2
}
func suit(id int) int {
	if id%54 >= 52 {
		return -1
	}
	return (id % 54) / 13
}
func wild(id, level int) bool { return suit(id) == 2 && natural(id) == level }
func strength(r, level int) int {
	if r == level {
		return 15
	}
	return r
}
func sortHand(ids []int, level int) {
	sort.Slice(ids, func(i, j int) bool {
		a, b := strength(natural(ids[i]), level), strength(natural(ids[j]), level)
		if a != b {
			return a > b
		}
		if wild(ids[i], level) != wild(ids[j], level) {
			return wild(ids[i], level)
		}
		return ids[i] < ids[j]
	})
}
func consecutive(start, size int) []int {
	out := make([]int, size)
	for i := range out {
		r := start + i
		if r == 1 {
			r = 14
		}
		out[i] = r
	}
	return out
}
func interpret(ids []int, level int) ([]Combination, error) {
	bad := func() ([]Combination, error) {
		return nil, errors.New("这些牌无法组成江苏规则允许的牌型")
	}
	if level < 2 || level > 14 || len(ids) == 0 || len(ids) > 10 {
		return bad()
	}
	counts := map[int]int{}
	wilds := []int{}
	fixed := []int{}
	seen := map[int]bool{}
	for _, id := range ids {
		if id < 0 || id >= 108 || seen[id] {
			return bad()
		}
		seen[id] = true
		if wild(id, level) {
			wilds = append(wilds, id)
		} else {
			counts[natural(id)]++
			fixed = append(fixed, id)
		}
	}
	sort.Ints(wilds)
	result := []Combination{}
	keys := map[string]bool{}
	add := func(kind string, high int, replacements []Replacement) {
		parts := []string{kind, fmt.Sprint(high), fmt.Sprint(len(ids))}
		for _, r := range replacements {
			parts = append(parts, fmt.Sprintf("%d=%d/%d", r.Card, r.Rank, r.Suit))
		}
		key := strings.Join(parts, ":")
		if keys[key] {
			return
		}
		keys[key] = true
		result = append(result, Combination{key, kind, high, len(ids), append([]Replacement{}, replacements...)})
	}
	// Wilds played alone (including the pair of both wilds) are level cards.
	if len(ids) <= 2 && len(wilds) == len(ids) {
		rs := []Replacement{}
		for _, id := range wilds {
			rs = append(rs, Replacement{id, level, 2})
		}
		kind := "single"
		if len(ids) == 2 {
			kind = "pair"
		}
		add(kind, 15, rs)
		return result, nil
	}
	if len(ids) == 4 && len(wilds) == 0 && counts[16] == 2 && counts[17] == 2 {
		add("rocket", 17, nil)
		return result, nil
	}
	match := func(want map[int]int, kind string, high int, requiredSuit int) {
		total := 0
		for _, n := range want {
			total += n
		}
		if total != len(ids) {
			return
		}
		for r, n := range counts {
			if n > want[r] {
				return
			}
		}
		if requiredSuit >= 0 {
			for _, id := range fixed {
				if suit(id) != requiredSuit {
					return
				}
			}
		}
		ranks := []int{}
		for r := range want {
			ranks = append(ranks, r)
		}
		sort.Ints(ranks)
		assignments := []Replacement{}
		wi := 0
		for _, r := range ranks {
			missing := want[r] - counts[r]
			if r >= 16 && missing > 0 {
				return
			}
			for j := 0; j < missing; j++ {
				if wi >= len(wilds) {
					return
				}
				ss := 2
				if requiredSuit >= 0 {
					ss = requiredSuit
				}
				assignments = append(assignments, Replacement{wilds[wi], r, ss})
				wi++
			}
		}
		if wi != len(wilds) {
			return
		}
		if kind == "straight" {
			same := true
			ss := -1
			for _, id := range fixed {
				if ss < 0 {
					ss = suit(id)
				} else if ss != suit(id) {
					same = false
				}
			}
			if same {
				if len(assignments) == 0 {
					return
				}
				assignments[0].Suit = (ss + 1) % 4
			}
		}
		add(kind, high, assignments)
	}
	if len(ids) <= 3 {
		kind := map[int]string{1: "single", 2: "pair", 3: "triple"}[len(ids)]
		for r := 2; r <= 17; r++ {
			if r == 15 || (r >= 16 && len(ids) > 2) {
				continue
			}
			match(map[int]int{r: len(ids)}, kind, strength(r, level), -1)
		}
	}
	if len(ids) == 5 {
		for triple := 2; triple <= 14; triple++ {
			for pair := 2; pair <= 17; pair++ {
				if pair == 15 || pair == triple {
					continue
				}
				match(map[int]int{triple: 3, pair: 2}, "full_house", strength(triple, level), -1)
			}
		}
		for start := 1; start <= 10; start++ {
			want := map[int]int{}
			for _, r := range consecutive(start, 5) {
				want[r] = 1
			}
			high := start + 4
			match(want, "straight", high, -1)
			for ss := 0; ss < 4; ss++ {
				match(want, "straight_flush", high, ss)
			}
		}
	}
	if len(ids) == 6 {
		for _, v := range []struct {
			kind        string
			units, size int
		}{{"tube", 3, 2}, {"plate", 2, 3}} {
			for start := 1; start <= 15-v.units; start++ {
				want := map[int]int{}
				for _, r := range consecutive(start, v.units) {
					want[r] = v.size
				}
				match(want, v.kind, start+v.units-1, -1)
			}
		}
	}
	if len(ids) >= 4 {
		for r := 2; r <= 14; r++ {
			match(map[int]int{r: len(ids)}, "bomb", strength(r, level), -1)
		}
	}
	if len(result) == 0 {
		return bad()
	}
	slices.SortFunc(result, func(a, b Combination) int {
		ca, cb := category(a), category(b)
		if ca != cb {
			return ca - cb
		}
		if a.High != b.High {
			return a.High - b.High
		}
		return strings.Compare(a.Key, b.Key)
	})
	return result, nil
}
func category(c Combination) int {
	if c.Kind == "rocket" {
		return 100
	}
	if c.Kind == "straight_flush" {
		return 20
	}
	if c.Kind == "bomb" {
		if c.Count >= 6 {
			return 30 + c.Count
		}
		return 10 + c.Count
	}
	return 0
}
func beats(a, b Combination) bool {
	if b.Kind == "" {
		return true
	}
	ac, bc := category(a), category(b)
	if ac != bc {
		return ac > bc
	}
	if a.Kind == "rocket" || b.Kind == "rocket" {
		return false
	}
	return a.Kind == b.Kind && a.Count == b.Count && a.High > b.High
}
func movesFor(hand []int, level int) []Move {
	groups := map[int][]int{}
	wilds := []int{}
	for _, id := range hand {
		if wild(id, level) {
			wilds = append(wilds, id)
		} else {
			groups[natural(id)] = append(groups[natural(id)], id)
		}
	}
	for r := range groups {
		sort.Ints(groups[r])
	}
	sort.Ints(wilds)
	result := []Move{}
	seen := map[string]bool{}
	add := func(ids []int) {
		ids = append([]int{}, ids...)
		sort.Ints(ids)
		combos, e := interpret(ids, level)
		if e != nil {
			return
		}
		for _, c := range combos {
			key := fmt.Sprint(ids) + c.Key
			if !seen[key] {
				seen[key] = true
				result = append(result, Move{ids, c})
			}
		}
	}
	// Candidate construction knows only this player's hand. Exact interpretation
	// remains the authority for both generated hints and manually selected cards.
	template := func(want map[int]int, ss int) {
		selected := []int{}
		missing := 0
		ranks := []int{}
		for r := range want {
			ranks = append(ranks, r)
		}
		sort.Ints(ranks)
		for _, r := range ranks {
			need := want[r]
			for _, id := range groups[r] {
				if ss < 0 || suit(id) == ss {
					selected = append(selected, id)
					need--
					if need == 0 {
						break
					}
				}
			}
			if r >= 16 && need > 0 {
				return
			}
			missing += need
		}
		if missing > len(wilds) {
			return
		}
		selected = append(selected, wilds[:missing]...)
		add(selected)
	}
	for _, id := range hand {
		add([]int{id})
	}
	if len(wilds) == 2 {
		add(wilds)
	}
	for r := 2; r <= 17; r++ {
		if r == 15 {
			continue
		}
		maximum := min(10, len(groups[r])+len(wilds))
		if r >= 16 {
			maximum = min(2, len(groups[r]))
		}
		for n := 2; n <= maximum; n++ {
			template(map[int]int{r: n}, -1)
		}
	}
	if len(groups[16]) == 2 && len(groups[17]) == 2 {
		add(append(append([]int{}, groups[16]...), groups[17]...))
	}
	for tr := 2; tr <= 14; tr++ {
		for pr := 2; pr <= 17; pr++ {
			if tr == pr || pr == 15 {
				continue
			}
			template(map[int]int{tr: 3, pr: 2}, -1)
		}
	}
	for _, shape := range []struct{ units, size int }{{5, 1}, {3, 2}, {2, 3}} {
		for start := 1; start <= 15-shape.units; start++ {
			want := map[int]int{}
			for _, r := range consecutive(start, shape.units) {
				want[r] = shape.size
			}
			template(want, -1)
			if shape.size == 1 {
				for ss := 0; ss < 4; ss++ {
					template(want, ss)
				}
			}
		}
	}
	return result
}
