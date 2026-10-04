package game

import (
	"fmt"
	"slices"
)

// The printed variable setups keep the sea and island outlines unchanged.
// New Shores shuffles the main island and the smaller islands separately;
// Four Islands shuffles all land together and reserves productive numbers
// for forests/pastures. The latter scenario does not prescribe the New
// Shores red-number restriction (2025 rulebook pages 5 and 7).
// Through the Desert keeps the desert belt fixed, shuffles the mainland and
// unexplored land separately, and forbids red numbers on gold (page 11).
func (g *Catan) randomizeSeafarersMap() error {
	if g.cloth() != nil {
		return g.randomizeClothMap()
	}
	if g.tribe() != nil {
		return g.randomizeTribeMap()
	}
	if g.Seafarers != nil && g.Seafarers.Fog != nil {
		return g.randomizeFogMap()
	}
	if len(g.Players) > 4 || g.Seafarers == nil || (g.Seafarers.Scenario != "shores" && g.Seafarers.Scenario != "islands" && g.Seafarers.Scenario != "desert") {
		return fmt.Errorf("该剧本尚未支持可变布局")
	}
	groups := [][]int{{}}
	desert := g.Seafarers.Scenario == "desert"
	shores := g.Seafarers.Scenario == "shores"
	separate := shores || desert
	if separate {
		groups = append(groups, []int{})
	}
	for _, t := range g.Tiles {
		if t.Resource == CatanSea || (desert && t.Resource == CatanDesert) {
			continue
		}
		group := 0
		if separate && !slices.Contains(g.Seafarers.StartIslands, g.Seafarers.Islands[t.ID]) {
			group = 1
		}
		groups[group] = append(groups[group], t.ID)
	}
	// Work on a copy so an invalid recipe never leaves a half-shuffled board.
	tiles := append([]CatanTile(nil), g.Tiles...)
	for _, group := range groups {
		terrain, numbers := []int{}, []int{}
		for _, id := range group {
			terrain = append(terrain, tiles[id].Resource)
			if tiles[id].Number > 0 {
				numbers = append(numbers, tiles[id].Number)
			}
		}
		shuffle(terrain)
		shuffle(numbers)
		land := []int{}
		for i, id := range group {
			tiles[id].Resource = terrain[i]
			tiles[id].Number = 0
			if terrain[i] != CatanDesert {
				land = append(land, id)
			}
		}
		if len(land) != len(numbers) {
			return fmt.Errorf("剧本数字牌与陆地数量不符")
		}
		shuffle(land)
		if separate {
			reds, others := []int{}, []int{}
			for _, n := range numbers {
				if n == 6 || n == 8 {
					reds = append(reds, n)
				} else {
					others = append(others, n)
				}
			}
			neighbors := map[int][]int{}
			for _, e := range g.Edges {
				ids := g.edgeTiles(e.ID)
				if len(ids) == 2 {
					neighbors[ids[0]] = append(neighbors[ids[0]], ids[1])
					neighbors[ids[1]] = append(neighbors[ids[1]], ids[0])
				}
			}
			// Place just the red tokens with finite backtracking, then distribute
			// other numbers. No unbounded retry loop or fallback to an illegal map.
			var place func(int, int) bool
			place = func(start, at int) bool {
				if at == len(reds) {
					return true
				}
				for i := start; i <= len(land)-(len(reds)-at); i++ {
					id := land[i]
					if desert && tiles[id].Resource == CatanGold {
						continue
					}
					valid := true
					for _, other := range neighbors[id] {
						if tiles[other].Number == 6 || tiles[other].Number == 8 {
							valid = false
							break
						}
					}
					if !valid {
						continue
					}
					tiles[id].Number = reds[at]
					if place(i+1, at+1) {
						return true
					}
					tiles[id].Number = 0
				}
				return false
			}
			if !place(0, 0) {
				return fmt.Errorf("无法按规则分开红色数字牌")
			}
			at := 0
			for _, id := range land {
				if tiles[id].Number == 0 {
					tiles[id].Number = others[at]
					at++
				}
			}
		} else {
			productive, low := []int{}, []int{}
			for _, n := range numbers {
				if n == 2 || n == 3 || n == 11 || n == 12 {
					low = append(low, n)
				} else {
					productive = append(productive, n)
				}
			}
			for _, id := range land {
				if tiles[id].Resource == 0 || tiles[id].Resource == 2 {
					if len(productive) == 0 {
						return fmt.Errorf("森林和牧场缺少合规数字牌")
					}
					tiles[id].Number = productive[0]
					productive = productive[1:]
				}
			}
			remaining := append(productive, low...)
			shuffle(remaining)
			at := 0
			for _, id := range land {
				if tiles[id].Number == 0 {
					tiles[id].Number = remaining[at]
					at++
				}
			}
		}
	}
	robber := -1
	if desert {
		robber = g.Robber
	}
	for _, t := range tiles {
		if desert {
			break
		}
		if (shores && len(g.Players) == 4 && t.Resource == CatanDesert) || ((!shores || len(g.Players) == 3) && t.Number == 12) {
			robber = t.ID
			break
		}
	}
	if robber < 0 {
		return fmt.Errorf("剧本缺少强盗起始地块")
	}
	g.shuffleSeafarersPorts()
	g.Tiles, g.Robber = tiles, robber
	g.Seafarers.Variable = true
	return nil
}
