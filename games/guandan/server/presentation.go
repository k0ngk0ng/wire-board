package guandan

import "sort"

func orderedPlay(ids []int, c Combination, level int) []int {
	out := append([]int{}, ids...)
	effective := func(id int) int {
		for _, w := range c.Wild {
			if w.Card == id {
				return w.Rank
			}
		}
		return natural(id)
	}
	sequence := c.Kind == "straight" || c.Kind == "straight_flush" || c.Kind == "tube" || c.Kind == "plate"
	order := func(id int) int {
		n := effective(id)
		if sequence {
			if n == 14 && c.High < 14 {
				return 1
			}
			return n
		}
		return strength(n, level)
	}
	body := func(id int) bool { return c.Kind != "full_house" || strength(effective(id), level) == c.High }
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if body(a) != body(b) {
			return body(a)
		}
		if order(a) != order(b) {
			return order(a) > order(b)
		}
		if wild(a, level) != wild(b, level) {
			return !wild(a, level)
		}
		if a%54 != b%54 {
			return a%54 < b%54
		}
		return a < b
	})
	return out
}
func displayPlay(p *Play, level int) *Play {
	if p == nil {
		return nil
	}
	out := *p
	out.Cards = orderedPlay(p.Cards, p.Combination, level)
	return &out
}
func displayTable(table []Play, level int) []Play {
	out := make([]Play, len(table))
	for i := range table {
		out[i] = *displayPlay(&table[i], level)
	}
	return out
}
