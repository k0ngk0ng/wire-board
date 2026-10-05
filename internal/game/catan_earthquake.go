package game

import "errors"

// Earthquake's board effects are shared by builds, legal hints and bots. The
// card-event queue invokes damage after validating the responding player;
// clients cannot invoke catanDamageRoad directly.
func (g *Catan) roadDamageable(player, edge int) bool {
	return player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated &&
		edge >= 0 && edge < len(g.Edges) && g.Edges[edge].Owner == player && !g.Edges[edge].Ship && !g.Edges[edge].Damaged
}

func (g *Catan) roadRepairable(player, edge int) bool {
	return player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated &&
		edge >= 0 && edge < len(g.Edges) && g.Edges[edge].Owner == player && !g.Edges[edge].Ship && g.Edges[edge].Damaged
}

func (g *Catan) hasDamagedRoad(player int) bool {
	for _, edge := range g.Edges {
		if g.roadRepairable(player, edge.ID) {
			return true
		}
	}
	return false
}

// A repair uses a Road Building allowance without using a new road piece.
func (g *Catan) hasFreeRouteAction(player int) bool {
	return g.hasDamagedRoad(player) || g.hasRoute(player)
}

func (s *State) catanDamageRoad(player, edge int) error {
	if !s.Catan.roadDamageable(player, edge) {
		return errors.New("地震必须选择自己尚未受损的一条道路，不能选择船只")
	}
	s.Catan.Edges[edge].Damaged = true
	s.catanLog(player, "地震：道路 #%d 受损，仍计入最长路线；修复前不能新建道路", edge+1)
	return nil
}

func (s *State) catanRepairRoad(player int, a Action) error {
	g := s.Catan
	free := s.Phase == "catan_roads" && g.FreeRoads > 0
	if s.Phase != "catan_turn" && !free {
		return errors.New("当前不能修复道路")
	}
	if a.Skill != "" || a.Choice != "" || !g.roadRepairable(player, a.Edge) {
		return errors.New("请选择自己的一条受损道路")
	}
	if !free {
		cost := catanPrices["catan_repair_road"]
		if !catanHas(g.Players[player].Resources, cost) {
			return errors.New("修复道路需要1木材和1砖块")
		}
		catanMove(g.Players[player].Resources, g.Bank, cost)
	}
	g.Edges[a.Edge].Damaged = false
	if free {
		s.catanLog(player, "使用道路建设免费修复道路 #%d", a.Edge+1)
	} else {
		s.catanLog(player, "支付木材×1、砖块×1，修复道路 #%d", a.Edge+1)
	}
	// Repairing is not placing a new route: do not discover fog, collect tribe
	// rewards, commit a fleet ship or establish another cloth trade relationship.
	s.catanFinishRoute(CatanRouteCompletion{Player: player, Edge: a.Edge, Free: free})
	return nil
}

func (g *Catan) repairRoadBotChoices(player int) []botChoice {
	choices := []botChoice{}
	for _, e := range g.Edges {
		if g.roadRepairable(player, e.ID) {
			choices = append(choices, botChoice{Action{Type: "catan_repair_road", Edge: e.ID}, 250})
		}
	}
	return choices
}
