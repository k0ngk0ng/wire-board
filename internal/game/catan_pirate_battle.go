package game

import "slices"

type CatanPirateBattle struct {
	ID        int   `json:"id"`
	Player    int   `json:"player"`
	Die       int   `json:"die"`
	Warships  int   `json:"warships"`
	Removed   []int `json:"removed"`
	Remaining int   `json:"remaining"`
}

func (g *Catan) pirateFortressReady(player int) bool {
	p := g.pirateIslands()
	if p == nil || player < 0 || player >= len(p.Fortresses) || p.Fortresses[player].Strength <= 0 {
		return false
	}
	vertices, ok := g.pirateRouteVertices(player)
	return ok && len(vertices) > 1 && vertices[len(vertices)-1] == p.Fortresses[player].Vertex
}

// Called only when ending an action phase. Production dice remain unchanged;
// battle has its own persisted die/event for reconnects and animation.
func (s *State) catanAttackFortress(player, die int) bool {
	g := s.Catan
	if s.Phase != "catan_turn" || s.Turn != player || die < 1 || die > 6 || !g.pirateFortressReady(player) {
		return false
	}
	p := g.pirateIslands()
	f := &p.Fortresses[player]
	warships := 0
	for _, id := range f.Route {
		if g.Edges[id].Warship {
			warships++
		}
	}
	event := &CatanPirateBattle{ID: 1, Player: player, Die: die, Warships: warships, Removed: []int{}}
	if p.Battle != nil {
		event.ID = p.Battle.ID + 1
	}
	if warships > die {
		f.Strength--
		s.catanLog(player, "以 %d 艘战舰攻打要塞，海盗掷出 %d，获胜并移除一层防御（剩余 %d 层）", warships, die, f.Strength)
		if f.Strength == 0 {
			g.Vertices[f.Vertex].Owner = player
			g.Vertices[f.Vertex].Level = 1
			s.catanLog(player, "夺回自己的要塞，恢复为可生产、可升级的村庄")
			all := true
			for _, fort := range p.Fortresses {
				if fort.Strength > 0 {
					all = false
					break
				}
			}
			if all {
				g.Seafarers.Pirate = -1
				s.Log = append(s.Log, "全部海盗要塞已夺回，海盗舰队离开游戏")
			}
		}
	} else {
		remove := 2
		if warships == die {
			remove = 1
		}
		remove = min(remove, len(f.Route))
		for range remove {
			at := len(f.Route) - 1
			id := f.Route[at]
			f.Route = append([]int{}, f.Route[:at]...)
			g.Edges[id].Owner = -1
			g.Edges[id].Ship = false
			g.Edges[id].Warship = false
			g.Seafarers.BuiltShips = slices.DeleteFunc(g.Seafarers.BuiltShips, func(edge int) bool { return edge == id })
			event.Removed = append(event.Removed, id)
		}
		s.catanLog(player, "以 %d 艘战舰攻打要塞，海盗掷出 %d，战败并退回最靠近要塞的 %d 艘船", warships, die, remove)
	}
	event.Remaining = f.Strength
	p.Battle = event
	s.catanScores()
	s.catanVictory()
	return true
}
