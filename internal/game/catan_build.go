package game

import (
	"errors"
	"slices"
)

// Medicine is an internal price override; client fields cannot activate it.
func (s *State) catanBuild(player int, a Action, medicine bool) error {
	return s.catanBuildOptions(player, a, medicine, false)
}

// Pirate immunity is internal to Diplomacy, never a client-provided flag.
func (s *State) catanBuildOptions(player int, a Action, medicine, diplomacyShip bool) error {
	g := s.Catan
	p := &g.Players[player]
	if s.Phase != "catan_turn" && !(s.Phase == "catan_roads" && (a.Type == "catan_road" || a.Type == "catan_ship")) {
		return errors.New("当前不能建造")
	}
	roads, _, _ := g.pieces(player)
	switch a.Type {
	case "catan_ship":
		allowed := g.canShip(player, a.Edge)
		if diplomacyShip {
			allowed = g.canDiplomacyShip(player, a.Edge)
		}
		if g.shipCount(player) >= 15 || !allowed {
			return errors.New("船只必须连接己方船只或建筑，不能穿过对手建筑或停入海盗所在海洋，且最多15艘")
		}
	case "catan_road":
		if g.hasDamagedRoad(player) {
			return errors.New("请先修复所有受损道路，再新建道路")
		}
		if roads >= 15 || !g.canRoad(player, a.Edge) {
			return errors.New("道路必须连接己方建筑或道路，不能穿过对手建筑，且最多 15 条")
		}
	case "catan_settlement":
		if g.settlementPiecesLeft(player) <= 0 || !g.canSettlement(player, a.Vertex, false) {
			return errors.New("村庄必须连接自己的道路，与所有建筑至少相隔两条边，且最多 5 座")
		}
	case "catan_city":
		if !g.canCityUpgrade(player, a.Vertex) {
			return errors.New("只能升级自己的村庄，且最多 4 座城市")
		}
	}
	free := s.Phase == "catan_roads"
	if !free {
		cost := catanPrices[a.Type]
		if medicine && a.Type == "catan_city" {
			cost = []int{0, 0, 0, 1, 2}
		}
		if a.Skill == "helper" {
			var err error
			cost, err = s.catanHelperBuildCost(player, a)
			if err != nil {
				return err
			}
		}
		if !catanHas(p.Resources, cost) {
			return errors.New("资源不足")
		}
		catanMove(p.Resources, g.Bank, cost)
		if a.Skill == "helper" && p.Helper.ID == 8 {
			s.catanHelperSpendKnight(player)
		}
	} else if a.Skill == "helper" {
		return errors.New("免费道路不使用助手")
	}
	if a.Type == "catan_ship" {
		g.Edges[a.Edge].Owner = player
		g.Edges[a.Edge].Ship = true
		g.Seafarers.BuiltShips = append(g.Seafarers.BuiltShips, a.Edge)
		s.catanLog(player, "建造船只 #%d", a.Edge+1)
	} else if a.Type == "catan_road" {
		g.Edges[a.Edge].Owner = player
		s.catanLog(player, "修建道路 #%d", a.Edge+1)
	} else {
		v := &g.Vertices[a.Vertex]
		v.Owner = player
		v.Level++
		if g.Caravans != nil {
			g.Caravans.Built = true
		}
		if v.Level == 2 {
			if k := g.CitiesKnights; k != nil {
				k.FallenCities = slices.DeleteFunc(k.FallenCities, func(id int) bool { return id == a.Vertex })
			}
			s.catanLog(player, "将村庄 #%d 升级为城市", a.Vertex+1)
		} else {
			s.catanLog(player, "建造村庄 #%d", a.Vertex+1)
			if g.riverVertex(a.Vertex) {
				if err := s.catanRiverReward(player, 1); err != nil {
					return err
				}
			}
			s.catanSettleIsland(player, a.Vertex, false)
		}
	}
	if a.Type == "catan_road" || a.Type == "catan_ship" {
		return s.catanAfterRoute(CatanRouteCompletion{Player: player, Edge: a.Edge, Free: free, Helper: a.Skill == "helper"})
	}
	g.Trade = nil
	s.catanClothTrade(player)
	s.catanScores()
	s.catanVictory()
	if g.Attack != nil && !s.Finished {
		return s.catanAttackLanding(func() [2]int { return [2]int{catanRandom(6) + 1, catanRandom(6) + 1} }, catanRandom)
	}
	if s.catanAskTribePort(player, s.Phase, nil, a.Skill == "helper") {
		return nil
	}
	if a.Skill == "helper" {
		s.catanHelperComplete(player, "catan_turn")
	}
	return nil
}
