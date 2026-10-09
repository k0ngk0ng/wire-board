package game

import "slices"

const catanRiversWorldExtendedNotice = "本站五六人组合：使用63格新世界与第三条三格河，替换同类山脉×1、牧场×2；两块沼泽替换海洋，11个港口、152金币，沿用配对回合与12分获胜"

func riverWorldTerrain(n int) []int {
	if n > 4 {
		return []int{7, 7, 7, 7, 7, 3, 19, 4, 0, 0, 2}
	}
	return []int{5, 4, 5, 5, 4, 0, 17, 0, 0, 0, 2}
}
func riverWorldNumbers(n int) []int {
	if n > 4 {
		return []int{24, 0, 2, 3, 4, 5, 5, 0, 5, 5, 4, 4, 2}
	}
	return []int{19, 0, 1, 3, 3, 3, 2, 0, 2, 3, 3, 2, 1}
}
func riverWorldChannels(n int) [][]int {
	result := [][]int{{4, 1, 2, catanSwamp}, {4, 1, catanSwamp}}
	if n > 4 {
		result = append(result, []int{4, 2, 2})
	}
	return result
}
func riverWorldPorts(n int) int {
	if n > 4 {
		return 11
	}
	return 10
}
func riverWorldBank(n int) int {
	if n > 4 {
		return 152
	}
	return 100
}
func riverWorldLayout(n int, prepared bool) string {
	if n > 4 {
		if prepared {
			return "extended-prepared"
		}
		return "extended"
	}
	if prepared {
		return "prepared"
	}
	return ""
}
func (g *Catan) riverWorldMapFor(candidates []riverSeaCandidate) *catanRiversMap {
	m := &catanRiversMap{DoubleNumberTile: -1}
	for channel, c := range candidates {
		m.Channels = append(m.Channels, catanRiverChannel{Tiles: slices.Clone(c.tiles), Outlet: c.outlet})
		// The extension's third river ends in pasture, not swamp.
		if channel < 2 {
			m.Swamps = append(m.Swamps, c.tiles[len(c.tiles)-1])
		}
		for i := 1; i < len(c.tiles); i++ {
			for _, e := range g.Edges {
				if slices.Contains(e.Tiles, c.tiles[i-1]) && slices.Contains(e.Tiles, c.tiles[i]) {
					m.Bridges = append(m.Bridges, e.ID)
					break
				}
			}
		}
		m.Bridges = append(m.Bridges, c.outlet)
	}
	return m
}
