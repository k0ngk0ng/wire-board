package game

import (
	"errors"
	"math"
	"slices"
)

// Sea frames have water outside the printed borders, so fishing grounds come
// from actual land/water boundaries instead of the printed fishing frame.
// Candidates are spread by angle around the board and never reuse an edge.
func spreadCoastalFishingGrounds(g *Catan, blocked map[int]bool, allowed func(int) bool) ([]catanFishingGround, error) {
	coast := []int{}
	for _, e := range g.Edges {
		if !blocked[e.ID] && allowed(e.ID) {
			coast = append(coast, e.ID)
		}
	}
	candidates := []catanFishingGround{}
	for i, id := range coast {
		for _, next := range coast[i+1:] {
			a, b := g.Edges[id], g.Edges[next]
			joint, first, last := a.A, a.B, b.A
			if joint != b.A && joint != b.B {
				joint, first = a.B, a.A
			}
			if joint != b.A && joint != b.B {
				continue
			}
			if last == joint {
				last = b.B
			}
			candidates = append(candidates, catanFishingGround{Edges: [2]int{id, next}, Vertices: [3]int{first, joint, last}})
		}
	}
	count := 6
	if len(g.Players) > 4 {
		count = 8
	}
	angle := func(c catanFishingGround) float64 { v := g.Vertices[c.Vertices[1]]; return math.Atan2(v.Y, v.X) }
	var choose func([]catanFishingGround, map[int]bool) ([]catanFishingGround, bool)
	choose = func(out []catanFishingGround, used map[int]bool) ([]catanFishingGround, bool) {
		if len(out) == count {
			return out, true
		}
		target := -math.Pi + 2*math.Pi*(float64(len(out))+.5)/float64(count)
		order := slices.Clone(candidates)
		distance := func(c catanFishingGround) float64 { d := math.Abs(angle(c) - target); return math.Min(d, 2*math.Pi-d) }
		slices.SortStableFunc(order, func(a, b catanFishingGround) int {
			if distance(a) < distance(b) {
				return -1
			}
			if distance(a) > distance(b) {
				return 1
			}
			return a.Edges[0] - b.Edges[0]
		})
		for _, c := range order {
			if used[c.Edges[0]] || used[c.Edges[1]] {
				continue
			}
			used[c.Edges[0]], used[c.Edges[1]] = true, true
			if r, ok := choose(append(out, c), used); ok {
				return r, true
			}
			delete(used, c.Edges[0])
			delete(used, c.Edges[1])
		}
		return nil, false
	}
	result, ok := choose(nil, map[int]bool{})
	if !ok {
		return nil, errors.New("海图缺少足够的有效渔场")
	}
	return result, nil
}
