package game

import (
	"errors"
	"slices"
	"sort"
)

// A host may choose coastal positions before the game starts. Edges are actual
// board edges, never arbitrary screen coordinates; the saved map derives its
// three production vertices and the pirate-blocking sea hex from topology.
type CatanFishingGroundPlacement struct {
	Number int    `json:"number"`
	Edges  [2]int `json:"edges"`
}
type CatanFishingCoast struct {
	Edges    [2]int `json:"edges"`
	Vertices [3]int `json:"vertices"`
	Island   int    `json:"island"`
	SeaTile  int    `json:"seaTile"` // -1 is the sea frame, not an extra board hex.
}

// A fishing V occupies two sides of the same water hex, facing two DIFFERENT
// land hexes. Two exposed sides of one land hex form the opposite (convex)
// corner, on which the original V-shaped component cannot sit.
func (g *Catan) fishingCoasts(islands []int) []CatanFishingCoast {
	result := []CatanFishingCoast{}
	if g.Seafarers == nil || len(islands) != len(g.Tiles) {
		return result
	}
	ports := map[int]bool{}
	for _, p := range g.Ports {
		ports[p.Edge] = true
	}
	type coastSide struct{ land, sea int }
	sides := map[int]coastSide{}
	for _, e := range g.Edges {
		if ports[e.ID] {
			continue
		}
		touching := g.edgeTiles(e.ID)
		land, sea := -1, -1
		valid := len(touching) > 0 && len(touching) <= 2
		for _, id := range touching {
			if id < 0 || id >= len(g.Tiles) {
				valid = false
				break
			}
			r := g.Tiles[id].Resource
			if r == CatanSea {
				if sea >= 0 {
					valid = false
				}
				sea = id
			} else if r != CatanFog && islands[id] >= 0 {
				if land >= 0 {
					valid = false
				}
				land = id
			} else {
				valid = false
			}
		}
		if valid && land >= 0 && (sea >= 0 || len(touching) == 1) {
			sides[e.ID] = coastSide{land, sea}
		}
	}
	for _, v := range g.Vertices {
		edges := g.touching(v.ID)
		sort.Ints(edges)
		for i, a := range edges {
			left, ok := sides[a]
			if !ok {
				continue
			}
			for _, b := range edges[i+1:] {
				right, ok := sides[b]
				if !ok || left.land == right.land || left.sea != right.sea || islands[left.land] != islands[right.land] {
					continue
				}
				first, last := g.Edges[a].A, g.Edges[b].A
				if first == v.ID {
					first = g.Edges[a].B
				}
				if last == v.ID {
					last = g.Edges[b].B
				}
				result = append(result, CatanFishingCoast{[2]int{a, b}, [3]int{first, v.ID, last}, islands[left.land], left.sea})
			}
		}
	}
	return result
}

// This is map research/configuration, not a public room choice or a promise
// that the associated scenario's complete action/combination rules are ready.
func (g *Catan) FishingCoasts() []CatanFishingCoast {
	if g.Seafarers != nil && len(g.Players) > 4 && fishingExtendedSeaScenario(g.Seafarers.Scenario) {
		coasts, _, _ := g.fishingExtendedSeaCoasts()
		return coasts
	}
	if g.Seafarers == nil {
		return []CatanFishingCoast{}
	}
	if g.Seafarers.Scenario == "wonders" {
		coasts, _ := g.fishingWondersCoasts()
		return coasts
	}
	if g.Seafarers.Scenario == "fog" {
		coasts, _ := g.fishingFogCoasts()
		return coasts
	}
	if g.Seafarers.Scenario == "tribe" {
		coasts, _ := g.fishingTribeCoasts()
		return coasts
	}
	if g.Seafarers.Scenario == "cloth" {
		coasts, _, _ := g.fishingClothCoasts()
		return coasts
	}
	return g.fishingCoasts(g.findIslands())
}
func (g *Catan) fishingFourIslandGroups() ([]int, []int, error) {
	if g.Seafarers == nil || g.Seafarers.Scenario != "islands" || len(g.Players) < 3 || len(g.Players) > 4 {
		return nil, nil, errors.New("仅已核对的三/四人四岛捕鱼位置")
	}
	islands := g.findIslands()
	counts := map[int]int{}
	for _, id := range islands {
		if id >= 0 {
			counts[id]++
		}
	}
	if len(counts) != 4 {
		return nil, nil, errors.New("四岛地图必须有两座大岛和两座小岛")
	}
	order := []int{}
	for id := range counts {
		order = append(order, id)
	}
	sort.Slice(order, func(i, j int) bool {
		if counts[order[i]] != counts[order[j]] {
			return counts[order[i]] < counts[order[j]]
		}
		return order[i] < order[j]
	})
	if counts[order[1]] >= counts[order[2]] {
		return nil, nil, errors.New("无法区分四岛的大岛与小岛")
	}
	return islands, order, nil
}
func fishingEdgeKey(edges [2]int) [2]int {
	if edges[0] > edges[1] {
		edges[0], edges[1] = edges[1], edges[0]
	}
	return edges
}
func fishingSeaGround(number int, c CatanFishingCoast) catanFishingGround {
	ground := catanFishingGround{Number: number, Edges: c.Edges, Vertices: c.Vertices}
	if c.SeaTile >= 0 {
		id := c.SeaTile
		ground.SeaTile = &id
	}
	return ground
}

// 2025 Fishing + Seafarers p.1: no lake; 4/8 on one small island, 6/10
// on the other; 5 and 9 on different large islands. Either island may take
// either compatible group, and callers may choose any nonoverlapping V.
func (g *Catan) makeFishingFourIslands(placements []CatanFishingGroundPlacement) (*catanFishingMap, error) {
	islands, order, err := g.fishingFourIslandGroups()
	if err != nil {
		return nil, err
	}
	coasts := g.fishingCoasts(islands)
	if placements == nil {
		groups := []struct {
			island  int
			numbers []int
		}{{order[0], []int{4, 8}}, {order[1], []int{6, 10}}, {order[2], []int{5}}, {order[3], []int{9}}}
		used := map[int]bool{}
		for _, group := range groups {
			picks := []CatanFishingGroundPlacement{}
			var choose func(int, int) bool
			choose = func(next, number int) bool {
				if number == len(group.numbers) {
					return true
				}
				for i := next; i < len(coasts); i++ {
					c := coasts[i]
					if c.Island != group.island || used[c.Edges[0]] || used[c.Edges[1]] {
						continue
					}
					used[c.Edges[0]], used[c.Edges[1]] = true, true
					picks = append(picks, CatanFishingGroundPlacement{group.numbers[number], c.Edges})
					if choose(i+1, number+1) {
						return true
					}
					picks = picks[:len(picks)-1]
					delete(used, c.Edges[0])
					delete(used, c.Edges[1])
				}
				return false
			}
			if !choose(0, 0) {
				return nil, errors.New("岛屿没有足够不重叠且不占港口的渔场位置")
			}
			placements = append(placements, picks...)
		}
	}
	byEdges := map[[2]int]CatanFishingCoast{}
	for _, c := range coasts {
		byEdges[c.Edges] = c
	}
	f := &catanFishingMap{Lakes: []catanFishingLake{}, Grounds: []catanFishingGround{}}
	for _, p := range placements {
		c, ok := byEdges[fishingEdgeKey(p.Edges)]
		if !ok {
			return nil, errors.New("渔场必须位于合法海岸凹角且不占港口")
		}
		f.Grounds = append(f.Grounds, fishingSeaGround(p.Number, c))
	}
	if err := f.validateFourIslands(g); err != nil {
		return nil, err
	}
	return f, nil
}
func (f catanFishingMap) validateFourIslands(g *Catan) error {
	islands, order, err := g.fishingFourIslandGroups()
	if err != nil {
		return err
	}
	if len(f.Lakes) != 0 || len(f.Grounds) != 6 {
		return errors.New("四岛捕鱼不使用湖泊，必须有六个渔场")
	}
	byEdges := map[[2]int]CatanFishingCoast{}
	for _, c := range g.fishingCoasts(islands) {
		byEdges[c.Edges] = c
	}
	used := map[int]bool{}
	numbers := map[int]int{}
	for _, ground := range f.Grounds {
		c, ok := byEdges[ground.Edges]
		if !ok || ground.Vertices != c.Vertices {
			return errors.New("渔场顶点与海岸边不匹配")
		}
		sea := -1
		if ground.SeaTile != nil {
			sea = *ground.SeaTile
		}
		if sea != c.SeaTile || (ground.SeaTile == nil) != (c.SeaTile < 0) {
			return errors.New("渔场的海盗封锁海格不匹配")
		}
		if !slices.Contains([]int{4, 5, 6, 8, 9, 10}, ground.Number) {
			return errors.New("非法渔场点数")
		}
		if _, exists := numbers[ground.Number]; exists {
			return errors.New("渔场点数重复")
		}
		numbers[ground.Number] = c.Island
		for _, e := range ground.Edges {
			if used[e] {
				return errors.New("渔场相互重叠")
			}
			used[e] = true
		}
	}
	if !slices.Contains(order[:2], numbers[4]) || !slices.Contains(order[:2], numbers[6]) || numbers[4] != numbers[8] || numbers[6] != numbers[10] || numbers[4] == numbers[6] || !slices.Contains(order[2:], numbers[5]) || !slices.Contains(order[2:], numbers[9]) || numbers[5] == numbers[9] {
		return errors.New("四岛渔场点数必须按两座小岛与两座大岛分组")
	}
	if g.Robber < -1 || g.Robber >= len(g.Tiles) || g.Robber >= 0 && (g.Tiles[g.Robber].Resource == CatanSea || g.Tiles[g.Robber].Resource == CatanFog) {
		return errors.New("非法强盗位置")
	}
	return nil
}
