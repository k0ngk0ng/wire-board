package doudizhu

import "sort"

// Preserve the body ahead of attachments, then show high ranks on the left.
// A new slice keeps presentation from mutating snapshots or caller input.
func orderedPlay(ids []int, c Combination) []int {
	out := append([]int{}, ids...)
	body := func(id int) bool {
		r := rank(id)
		switch c.Kind {
		case "trio_single", "trio_pair", "four_single", "four_pair":
			return r == c.High
		case "plane_single", "plane_pair":
			return r <= c.High && r > c.High-c.Units
		}
		return true
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if body(a) != body(b) {
			return body(a)
		}
		if rank(a) != rank(b) {
			return rank(a) > rank(b)
		}
		if a%54 != b%54 {
			return a%54 < b%54
		}
		return a < b
	})
	return out
}
func displayPlay(p *Play, huanle ...bool) *Play {
	if p == nil {
		return nil
	}
	out := *p
	out.Cards = orderedPlay(p.Cards, p.Combination)
	if len(huanle) > 0 && huanle[0] {
		out.Cards = orderedHuanlePlay(p.Cards, p.Combination)
	}
	return &out
}
func displayTable(table []Play, huanle ...bool) []Play {
	out := make([]Play, len(table))
	for i := range table {
		out[i] = *displayPlay(&table[i], huanle...)
	}
	return out
}
