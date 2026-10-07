package game

import "errors"

// Validate the persisted development components used by the private combined
// action controller. Full knight/progress/mission validation is still separate.
func (s *State) validateExplorerCityDevelopment() error {
	g, k := s.Catan, s.Catan.CitiesKnights
	for _, p := range k.Players {
		for _, level := range p.Improvements {
			if level < 0 || level > 5 {
				return errors.New("组合城市改良等级无效")
			}
		}
	}
	seen := map[int]bool{}
	walls := make([]int, len(g.Players))
	for _, v := range k.Walls {
		if !g.cityAt(v) || seen[v] || g.Vertices[v].Owner < 0 || g.Vertices[v].Owner >= len(g.Players) {
			return errors.New("组合城墙必须位于不同的玩家城市")
		}
		seen[v] = true
		walls[g.Vertices[v].Owner]++
		if walls[g.Vertices[v].Owner] > 3 {
			return errors.New("组合城墙组件超过三座")
		}
	}
	seen = map[int]bool{}
	for track, v := range k.Metropolises {
		if v == -1 {
			continue
		}
		if !g.cityAt(v) || seen[v] || g.Vertices[v].Owner < 0 || g.Vertices[v].Owner >= len(g.Players) || k.Players[g.Vertices[v].Owner].Improvements[track] < 4 {
			return errors.New("组合大都会必须属于达到四级的不同城市")
		}
		seen[v] = true
	}
	return nil
}
