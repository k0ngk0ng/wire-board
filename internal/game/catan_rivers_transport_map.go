package game

import (
	"errors"
	"math"
	"slices"
)

const CatanRiversTransportRules = "catan-rivers-transport-2025"

// The official combination permits any two non-touching river components.
// These pinned placements retain the commodity sites, crossed edges and ports.
// The 37-hex extension is an explicit site recipe, not the 30-hex river board.
func riversTransportRecipe(extended bool) ([][]int, [][]int, []int, [5]int) {
	if extended {
		return [][]int{{1, 5, 10, 16}, {7, 13, 20}, {25, 30, 34}}, [][]int{{4, 1, 2, catanSwamp}, {4, 1, catanSwamp}, {4, 2, 2}}, []int{1, 0, 1}, [5]int{6, 3, 3, 6, 2}
	}
	return [][]int{{1, 4, 8, 12}, {6, 10, 14}}, [][]int{{4, 1, 2, catanSwamp}, {4, 1, catanSwamp}}, []int{1, 1}, [5]int{3, 1, 1, 3, 1}
}
func riversTransportFixed(extended bool) map[int]int {
	paths, terrain, _, _ := riversTransportRecipe(extended)
	out := map[int]int{}
	for i, path := range paths {
		for j, id := range path {
			out[id] = terrain[i][j]
		}
	}
	return out
}
func riversTransportNumbers(g *Catan) ([]int, int) {
	order, faces := catanRiverNumberRecipe(false)
	if len(g.Players) > 4 {
		// Read the larger frame in stable angular order, ring by
		// ring; the official combination does not supply an extended number map.
		order = make([]int, len(g.Tiles))
		cx, cy := 0.0, 0.0
		for i, t := range g.Tiles {
			order[i] = i
			cx += t.X
			cy += t.Y
		}
		cx /= float64(len(g.Tiles))
		cy /= float64(len(g.Tiles))
		slices.SortFunc(order, func(a, b int) int {
			x, y := g.Tiles[a], g.Tiles[b]
			ringA := int(math.Round(math.Hypot(x.X-cx, x.Y-cy) / (math.Sqrt(3) * g.HexSize)))
			ringB := int(math.Round(math.Hypot(y.X-cx, y.Y-cy) / (math.Sqrt(3) * g.HexSize)))
			if ringA != ringB {
				return ringB - ringA
			}
			aa, bb := math.Atan2(x.Y-cy, x.X-cx), math.Atan2(y.Y-cy, y.X-cx)
			if aa < bb {
				return -1
			}
			if aa > bb {
				return 1
			}
			return a - b
		})
		full := []int{5, 2, 6, 3, 8, 10, 9, 12, 11, 4, 8, 10, 9, 4, 5, 6, 3, 11}
		faces = append(slices.Clone(faces), full...)
	}
	fixed := riversTransportFixed(len(g.Players) > 4)
	numbers := make([]int, len(g.Tiles))
	at, double := 0, -1
	for _, id := range order {
		if fixed[id] == catanSwamp {
			continue
		}
		numbers[id] = faces[at]
		if faces[at] == 12 && double < 0 {
			double = id
		}
		at++
	}
	return numbers, double
}
func riversTransportChannels(g *Catan) (*catanRiversMap, error) {
	paths, _, outlets, _ := riversTransportRecipe(len(g.Players) > 4)
	_, double := riversTransportNumbers(g)
	r := &catanRiversMap{DoubleNumberTile: double}
	fixed := riversTransportFixed(len(g.Players) > 4)
	for i, path := range paths {
		outlet := catanFishingSide(g, path[len(path)-1], outlets[i])
		if outlet < 0 {
			return nil, errors.New("河流运输河口不存在")
		}
		r.Channels = append(r.Channels, catanRiverChannel{Tiles: slices.Clone(path), Outlet: outlet})
		for j, id := range path {
			if fixed[id] == catanSwamp {
				r.Swamps = append(r.Swamps, id)
			}
			if j == 0 {
				continue
			}
			edge := -1
			for _, e := range g.Edges {
				if slices.Contains(e.Tiles, id) && slices.Contains(e.Tiles, path[j-1]) {
					edge = e.ID
					break
				}
			}
			if edge < 0 {
				return nil, errors.New("河流运输河道不连续")
			}
			r.Bridges = append(r.Bridges, edge)
		}
		r.Bridges = append(r.Bridges, outlet)
	}
	return r, nil
}
func newCatanRiversTransportBoard(n int) (*Catan, *catanTransportMap, *catanRiversMap, error) {
	g, m, err := catanTransportGeometry(n)
	if err != nil {
		return nil, nil, nil, err
	}
	m.Rivers = CatanRiversTransportRules
	fixed := riversTransportFixed(n > 4)
	_, _, _, counts := riversTransportRecipe(n > 4)
	pool := []int{}
	for color, count := range counts {
		for range count {
			pool = append(pool, color)
		}
	}
	shuffle(pool)
	numbers, _ := riversTransportNumbers(g)
	at := 0
	for id := range g.Tiles {
		if color, ok := fixed[id]; ok {
			g.Tiles[id].Resource = color
		} else if g.Tiles[id].Resource != catanTransportTerrain {
			g.Tiles[id].Resource = pool[at]
			at++
		}
		g.Tiles[id].Number = numbers[id]
	}
	ports := []int{-1, -1, -1, -1, 0, 1, 2, 3, 4}
	if n > 4 {
		ports = append(ports, -1, 2)
	}
	shuffle(ports)
	for i, color := range ports {
		g.Ports[i].Resource = color
	}
	r, err := riversTransportChannels(g)
	if err != nil {
		return nil, nil, nil, err
	}
	return g, m, r, m.validate(g)
}
