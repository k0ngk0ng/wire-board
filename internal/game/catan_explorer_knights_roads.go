package game

import "errors"

// Quote only visible geometry/owned pieces. No resource injection is used to
// determine whether Road Building is playable without a wood/brick hand.
func (s *State) catanExplorerFreeRoadSites(player int) []int {
	g, x := s.Catan, s.Catan.Explorer
	out := []int{}
	for _, edge := range g.Edges {
		if _, err := x.Cargo.roadPrice(g, x.Fleet, player, g.TurnSerial, edge.ID, true); err == nil {
			out = append(out, edge.ID)
		}
	}
	return out
}

func (s *State) catanExplorerBeginFreeRoads(player int) error {
	g := s.Catan
	if s.Phase != "catan_turn" || player != s.Turn || g.FreeRoads != 0 {
		return errors.New("当前不能开始免费道路")
	}
	g.ResumePhase = "catan_turn"
	if len(s.catanExplorerFreeRoadSites(player)) == 0 {
		s.catanLog(player, "没有合法的探险道路位置，卡牌放回牌堆底部")
		return nil
	}
	g.FreeRoads = 2
	s.Phase = "catan_roads"
	s.catanLog(player, "免费修建两条道路；此牌不能建造探险船")
	return nil
}

// Entered only after the private combined boundary validator. Each road is
// its own atomic saved step; an invalid second road preserves the first.
func (s *State) catanExplorerCityRoadAction(player int, a Action) error {
	g := s.Catan
	if s.Finished || s.Phase != "catan_roads" || player != s.Turn || a.Prompt < 1 || uint64(a.Prompt) != g.TurnSerial || a.Skill != "" || a.Choice != "" {
		return errors.New("不是当前免费道路玩家、阶段或序号")
	}
	next := clone(*s)
	ng, nx := next.Catan, next.Catan.Explorer
	switch a.Type {
	case "catan_road":
		if err := nx.Cargo.buildRoadCost(ng, nx.Fleet, player, ng.TurnSerial, a.Edge, true); err != nil {
			return err
		}
		ng.FreeRoads--
		next.catanLog(player, "免费修建道路 #%d", a.Edge+1)
	case "catan_skip_roads":
		if len(next.catanExplorerFreeRoadSites(player)) > 0 {
			return errors.New("仍有合法免费道路可建造")
		}
		ng.FreeRoads = 0
	default:
		return errors.New("请先完成免费道路；不能用此牌造船或执行其他行动")
	}
	if ng.FreeRoads == 0 || len(next.catanExplorerFreeRoadSites(player)) == 0 {
		ng.FreeRoads = 0
		next.Phase = "catan_turn"
	}
	ng.Trade = nil
	next.catanScores()
	next.catanVictory()
	if err := next.validateExplorerCityProduction(); err != nil {
		return err
	}
	*s = next
	return nil
}
