package game

import (
	"errors"
	"slices"
)

// The action serial advances for BOTH paired players, independently of the
// production-turn serial. A recruited/promoted knight keeps its own locks when
// it moves or is displaced; a different knight is never locked by its actions.
type CatanKnight struct {
	Owner       int    `json:"owner"`
	Vertex      int    `json:"vertex"`
	Strength    int    `json:"strength"`
	Active      bool   `json:"active"`
	ActivatedAt uint64 `json:"activatedAt"`
	PromotedAt  uint64 `json:"promotedAt"`
}

func (g *Catan) knightAt(v int) *CatanKnight {
	if g.CitiesKnights != nil {
		for i := range g.CitiesKnights.Knights {
			if g.CitiesKnights.Knights[i].Vertex == v {
				return &g.CitiesKnights.Knights[i]
			}
		}
	}
	return nil
}
func (g *Catan) knightCount(player, strength int) int {
	count := 0
	if k := g.CitiesKnights; k != nil {
		for _, n := range k.Knights {
			if n.Owner == player && n.Strength == strength {
				count++
			}
		}
		if k.Pending != nil && k.Pending.Kind == "knight_retreat" && k.Pending.Knight != nil && k.Pending.Knight.Owner == player && k.Pending.Knight.Strength == strength {
			count++
		}
	}
	return count
}
func (g *Catan) opponentPiece(player, vertex int) bool {
	v := g.Vertices[vertex]
	if v.Level > 0 && v.Owner != player {
		return true
	}
	n := g.knightAt(vertex)
	return n != nil && n.Owner != player
}
func (g *Catan) knightRecruitable(player, vertex int) bool {
	return g.knightCount(player, 1) < 2 && g.knightPlaceable(player, vertex)
}
func (g *Catan) knightPlaceable(player, vertex int) bool {
	if g.CitiesKnights == nil || vertex < 0 || vertex >= len(g.Vertices) || g.Vertices[vertex].Level != 0 || g.knightAt(vertex) != nil || g.clothVillageAt(vertex) || g.Transport != nil && g.Transport.Map.siteAt(vertex) >= 0 || !g.explorerKnightSite(vertex) || !g.pirateKnightSite(vertex) {
		return false
	}
	for _, edge := range g.touching(vertex) {
		if g.Edges[edge].Owner == player && (g.Explorer == nil || !g.Edges[edge].Ship) {
			return true
		}
	}
	return false
}
func (g *Catan) knightCanAct(n *CatanKnight) bool {
	return n != nil && n.Active && n.ActivatedAt != g.CitiesKnights.ActionSerial
}
func (g *Catan) knightCanPromote(n *CatanKnight) bool {
	return n != nil && n.Strength < 3 && n.PromotedAt != g.CitiesKnights.ActionSerial && g.knightCount(n.Owner, n.Strength+1) < 2 && (n.Strength < 2 || n.Owner >= 0 && g.CitiesKnights.Players[n.Owner].Improvements[CatanPolitics] >= 3)
}

// Return every reachable endpoint, including blockers. Expansion stops at an
// opponent piece; the caller may displace a weaker knight at that endpoint.
// Starting on the attacker's new location is allowed for a retreating knight.
func (g *Catan) knightReachable(player, from int) []bool {
	seen := make([]bool, len(g.Vertices))
	if from < 0 || from >= len(seen) {
		return seen
	}
	seen[from] = true
	queue := []int{from}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		if v != from && g.opponentPiece(player, v) {
			continue
		}
		for _, id := range g.touching(v) {
			e := g.Edges[id]
			if e.Owner != player || g.Explorer != nil && e.Ship {
				continue
			}
			next := e.A
			if next == v {
				next = e.B
			}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return seen
}
func (g *Catan) knightDestinations(n CatanKnight, retreat bool) []int {
	out := []int{}
	for v, reachable := range g.knightReachable(n.Owner, n.Vertex) {
		if !reachable || v == n.Vertex || g.Vertices[v].Level > 0 || g.clothVillageAt(v) || g.Transport != nil && g.Transport.Map.siteAt(v) >= 0 || !g.explorerKnightSite(v) || !g.pirateKnightSite(v) {
			continue
		}
		other := g.knightAt(v)
		if other == nil || (!retreat && other.Owner != n.Owner && other.Strength < n.Strength) {
			out = append(out, v)
		}
	}
	return out
}
func (g *Catan) knightCanChase(n *CatanKnight) bool {
	if !g.knightCanAct(n) || g.CitiesKnights.Invasions == 0 || g.Robber < 0 || g.Robber >= len(g.Tiles) {
		return false
	}
	return slices.Contains(g.Tiles[g.Robber].Vertices, n.Vertex)
}
func (g *Catan) knightCanChasePirate(n *CatanKnight) bool {
	if !g.knightCanAct(n) || !g.pirateAllowed(n.Owner) || g.Seafarers.Pirate < 0 || g.Seafarers.Pirate >= len(g.Tiles) {
		return false
	}
	return slices.Contains(g.Tiles[g.Seafarers.Pirate].Vertices, n.Vertex)
}
func (s *State) catanKnightAction(player int, a Action) error {
	return s.catanKnightActionCost(player, a, false)
}

func (s *State) catanKnightActionCost(player int, a Action, freePromotion bool) error {
	g := s.Catan
	k := g.CitiesKnights
	if k == nil || player != s.Turn || s.Phase != "catan_turn" {
		return errors.New("当前不能操作骑士")
	}
	n := g.knightAt(a.Vertex)
	cost := []int{0, 0, 0, 0, 0}
	switch a.Type {
	case "catan_knight_recruit":
		if !g.knightRecruitable(player, a.Vertex) {
			return errors.New("一级骑士库存不足，或该空交点未连接自己的路线")
		}
		cost = []int{0, 0, 1, 0, 1}
	case "catan_knight_activate", "catan_knight_promote", "catan_knight_move", "catan_knight_chase", "catan_knight_warship":
		if n == nil || n.Owner != player {
			return errors.New("请选择自己的骑士")
		}
		switch a.Type {
		case "catan_knight_activate":
			if n.Active {
				return errors.New("骑士已经激活")
			}
			cost = []int{0, 0, 0, 1, 0}
		case "catan_knight_promote":
			if !g.knightCanPromote(n) {
				return errors.New("骑士每回合只能升级一次，须有对应棋子；三级骑士需要政治三级")
			}
			cost = []int{0, 0, 1, 0, 1}
			if freePromotion {
				cost = []int{0, 0, 0, 0, 0}
			}
		case "catan_knight_warship":
			if !g.knightCanWarship(n) {
				return errors.New("需要此前已激活的骑士，且远征线上仍有普通船")
			}
		case "catan_knight_move":
			if !g.knightCanAct(n) || !slices.Contains(g.knightDestinations(*n, false), a.Target) {
				return errors.New("只能移动本行动阶段开始前已激活的骑士，沿自己的连续路线到达空位或较弱敌方骑士")
			}
		case "catan_knight_chase":
			if (a.Choice != "" && a.Choice != "robber" && a.Choice != "pirate") || (a.Choice == "pirate" && !g.knightCanChasePirate(n)) || (a.Choice != "pirate" && !g.knightCanChase(n)) {
				return errors.New("需要相邻且此前已激活的骑士才能驱逐所选的强盗或海盗")
			}
		}
	default:
		return errors.New("未知骑士动作")
	}
	if !catanHas(g.Players[player].Resources, cost) {
		return errors.New("骑士操作所需资源不足")
	}
	catanMove(g.Players[player].Resources, g.Bank, cost)
	g.Trade = nil
	switch a.Type {
	case "catan_knight_recruit":
		k.Knights = append(k.Knights, CatanKnight{Owner: player, Vertex: a.Vertex, Strength: 1})
		s.catanLog(player, "支付羊毛×1、矿石×1，在交点 #%d 招募一级骑士（未激活）", a.Vertex+1)
	case "catan_knight_activate":
		n.Active = true
		n.ActivatedAt = k.ActionSerial
		s.catanLog(player, "支付粮食×1，激活交点 #%d 的骑士，本阶段不能再让它行动", a.Vertex+1)
	case "catan_knight_promote":
		n.Strength++
		n.PromotedAt = k.ActionSerial
		s.catanLog(player, "支付羊毛×%d、矿石×%d，将交点 #%d 的骑士升至%d级", cost[2], cost[4], a.Vertex+1, n.Strength)
	case "catan_knight_warship":
		id := g.pirateNextWarship(player)
		n.Active = false
		g.Edges[id].Warship = true
		s.catanLog(player, "交点 #%d 的骑士转为未激活，将远征线船只 #%d 升级为战舰", a.Vertex+1, id+1)
	case "catan_knight_move":
		var displaced *CatanKnight
		if other := g.knightAt(a.Target); other != nil {
			copy := *other
			displaced = &copy
		}
		n.Vertex = a.Target
		n.Active = false
		if displaced != nil {
			k.Knights = slices.DeleteFunc(k.Knights, func(piece CatanKnight) bool {
				return piece.Owner == displaced.Owner && piece.Vertex == displaced.Vertex
			})
			s.catanLog(player, "将骑士从 #%d 移至 #%d，驱逐%s的%d级骑士", a.Vertex+1, a.Target+1, catanKnightOwnerName(displaced.Owner), displaced.Strength)
			if len(g.knightDestinations(*displaced, true)) > 0 {
				k.Pending = &CatanCityPending{Kind: "knight_retreat", Players: []int{g.knightResponseActor(displaced.Owner, s.Turn)}, Knight: displaced}
				s.Phase = "catan_knight_retreat"
			} else {
				s.catanLog(player, "%s被驱逐的骑士没有合法退路，返回库存", catanKnightOwnerName(displaced.Owner))
			}
		} else {
			s.catanLog(player, "将骑士从 #%d 移至 #%d，骑士转为未激活", a.Vertex+1, a.Target+1)
		}
	case "catan_knight_chase":
		n.Active = false
		g.ResumePhase = "catan_turn"
		s.Phase = "catan_robber"
		k.Chase = "robber"
		target := "强盗"
		if a.Choice == "pirate" {
			k.Chase = "pirate"
			target = "海盗"
		}
		s.catanLog(player, "交点 #%d 的骑士驱逐%s，骑士转为未激活", a.Vertex+1, target)
	}
	// Resolve a displaced knight's final position before awarding longest route.
	if k.Pending == nil {
		s.catanClothKnightRoutes()
		s.catanScores()
		s.catanVictory()
	}
	return nil
}
func (s *State) catanKnightRetreat(player int, a Action) error {
	g := s.Catan
	q := g.CitiesKnights.Pending
	if q.Knight == nil || s.Phase != "catan_knight_retreat" || a.Type != "catan_knight_retreat" || !slices.Contains(g.knightDestinations(*q.Knight, true), a.Vertex) {
		return errors.New("请选择被驱逐骑士沿自己路线可到达的空交点")
	}
	n := *q.Knight
	n.Vertex = a.Vertex
	g.CitiesKnights.Knights = append(g.CitiesKnights.Knights, n)
	s.catanLog(player, "将被驱逐的%d级骑士退至 #%d，保留原激活状态", n.Strength, a.Vertex+1)
	return nil
}

func (g *Catan) knightBotChoices(player int) []botChoice {
	k := g.CitiesKnights
	if k == nil {
		return nil
	}
	_, _, cities := g.pieces(player)
	strength := 0
	for _, n := range k.Knights {
		if n.Owner == player {
			strength += n.Strength
		}
	}
	navy := g.pirateIslands() != nil && g.pirateNextWarship(player) >= 0
	choices := []botChoice{}
	for _, v := range g.Vertices {
		if (strength < cities || navy && strength == 0) && g.knightRecruitable(player, v.ID) {
			choices = append(choices, botChoice{Action{Type: "catan_knight_recruit", Vertex: v.ID}, 560})
		}
	}
	for i := range k.Knights {
		n := &k.Knights[i]
		if n.Owner != player {
			continue
		}
		if !n.Active && (cities > 0 || navy) {
			choices = append(choices, botChoice{Action{Type: "catan_knight_activate", Vertex: n.Vertex}, 760})
		}
		if g.knightCanWarship(n) {
			choices = append(choices, botChoice{Action{Type: "catan_knight_warship", Vertex: n.Vertex}, 800})
		}
		if strength < cities && g.knightCanPromote(n) {
			choices = append(choices, botChoice{Action{Type: "catan_knight_promote", Vertex: n.Vertex}, 580})
		}
		if g.knightCanChase(n) {
			for _, v := range g.Tiles[g.Robber].Vertices {
				if g.Vertices[v].Owner == player && g.Vertices[v].Level > 0 {
					choices = append(choices, botChoice{Action{Type: "catan_knight_chase", Vertex: n.Vertex}, 810})
					break
				}
			}
		}
		if g.knightCanChasePirate(n) {
			for _, e := range g.Edges {
				if e.Ship && e.Owner == player && g.pirateBlocks(e.ID) {
					choices = append(choices, botChoice{Action{Type: "catan_knight_chase", Vertex: n.Vertex, Choice: "pirate"}, 810})
					break
				}
			}
		}
		if g.knightCanAct(n) {
			for _, v := range g.knightDestinations(*n, false) {
				if g.knightAt(v) != nil {
					choices = append(choices, botChoice{Action{Type: "catan_knight_move", Vertex: n.Vertex, Target: v}, 820})
				}
			}
		}
	}
	return choices
}
