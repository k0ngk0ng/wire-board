package game

import "errors"

func (s *State) explorerHelperReady(player int) bool {
	g := s.Catan
	return g != nil && g.Explorer != nil && g.Explorer.Helpers != nil && g.Explorer.Setup == nil && !s.Finished && player >= 0 && player < len(g.Players) && player == s.Turn && g.Players[player].Helper != nil && g.helperReady(player, g.Players[player].Helper.ID) && s.CatanPendingActor() < 0 && (s.Phase == "catan_turn" || s.Phase == "catan_roll" && g.Players[player].Helper.ID == 10)
}

func (s *State) applyCatanExplorerHelper(player int, a Action) error {
	g, x := s.Catan, s.Catan.Explorer
	if g.HelperPending != nil {
		return s.catanExplorerHelperRespond(player, a)
	}
	if !s.explorerHelperReady(player) || a.Prompt < 1 || uint64(a.Prompt) != g.TurnSerial {
		return errors.New("当前不能使用探险助手，或回应序号已过期")
	}
	id, resume := g.Players[player].Helper.ID, s.Phase
	if a.Skill == "helper" {
		if err := s.catanExplorerHelperBuild(player, a); err != nil {
			return err
		}
		// Ship construction can replace the complete copied aggregate.
		s.catanExplorerMissionScore()
		s.catanExplorerVictory()
		s.catanHelperComplete(player, resume)
		return nil
	}
	if a.Type != "catan_helper" || a.Skill != "" {
		return errors.New("请选择探险助手对应的行动")
	}
	switch id {
	case 1, 9:
		return s.catanHelperAction(player, a)
	case 7:
		if a.Target < 0 || a.Target >= len(g.Players) || a.Target == player || g.Players[a.Target].Eliminated || g.Players[a.Target].Score <= g.Players[player].Score || sum(g.Players[a.Target].Resources) == 0 {
			return errors.New("请选择公开分数领先且仍有手牌的对手")
		}
		g.Trade = nil
		s.catanHelperAsk(CatanHelperPending{Player: player, Kind: "leader", Target: a.Target, Resume: resume})
		return nil
	case 4:
		if !g.helperEndRoad(player, a.Edge) || a.Edge == a.Target {
			return errors.New("只能迁移己方末端道路")
		}
		g.Edges[a.Edge].Owner = -1
		if err := x.Cargo.buildRoadCost(g, x.Fleet, player, g.TurnSerial, a.Target, true); err != nil {
			return err
		}
		s.catanLog(player, "通过霍格尼将道路 #%d 移至 #%d", a.Edge+1, a.Target+1)
	case 10:
		if err := x.Economy.ensureGold(1); err != nil {
			return err
		}
		if x.Pirate != nil {
			x.Pirate.Owner, x.Pirate.Tile = -1, -1
		}
		x.Economy.GoldBank--
		x.Economy.Gold[player]++
		s.catanLog(player, "本站迪古尔：海盗退场，领取1金币；没有海盗时同样领取")
	case 11:
		if a.Color < 0 || a.Color >= 5 || g.Bank[a.Color] == 0 {
			return errors.New("请选择仍有库存的普通资源")
		}
		g.Bank[a.Color]--
		g.Players[player].Resources[a.Color]++
		s.catanLog(player, "本站卡娅：领取%s×1", CatanResources[a.Color])
	case 12:
		if a.Color < 0 || a.Color >= 5 || a.Target < 0 || a.Target >= 5 || a.Target == a.Color || g.Players[player].Resources[a.Color] == 0 || g.Bank[a.Target] == 0 {
			return errors.New("请选择持有的普通资源和银行另一种有库存资源")
		}
		g.Players[player].Resources[a.Color]--
		g.Bank[a.Color]++
		g.Bank[a.Target]--
		g.Players[player].Resources[a.Target]++
		s.catanLog(player, "本站卡拉：归还%s×1，换得%s×1", CatanResources[a.Color], CatanResources[a.Target])
	default:
		return errors.New("这位助手只能在对应建设或生产回应时使用")
	}
	s.catanHelperComplete(player, resume)
	return nil
}

func (s *State) catanExplorerHelperBuild(player int, a Action) error {
	g, x := s.Catan, s.Catan.Explorer
	id := g.Players[player].Helper.ID
	if s.Phase != "catan_turn" || a.Choice != "" {
		return errors.New("探险助手建设只能在交易建设阶段")
	}
	if id == 2 && a.Type == "catan_road" || id == 6 && a.Type == "catan_explorer_ship" {
		base := []int{1, 1, 0, 0, 0}
		if id == 6 {
			base = []int{1, 0, 1, 0, 0}
		}
		cost, err := helperSubstituteCost(base, a.Tokens)
		if err != nil {
			return err
		}
		if !catanExplorerCanPay(g, player, cost) {
			return errors.New("无法支付替换后的普通资源费用")
		}
		if id == 2 {
			if err = x.Cargo.buildRoadCost(g, x.Fleet, player, g.TurnSerial, a.Edge, true); err != nil {
				return err
			}
		} else {
			awards, err := x.buildShipCost(g, player, g.TurnSerial, a.Slot, a.Edge, true)
			if err != nil {
				return err
			}
			s.catanExplorerDiscoverLog(player, awards)
		}
		catanExplorerPay(s.Catan, player, cost)
		s.catanLog(player, "通过助手支付%s，完成%s", catanText(cost), map[int]string{2: "修路", 6: "造船"}[id])
		return nil
	}
	if id != 8 || a.Type != "catan_settlement" && a.Type != "catan_explorer_harbor" && a.Type != "catan_city" || len(a.Tokens) != 0 {
		return errors.New("助手与建设类型不符")
	}
	if !s.explorerHelperBuilderUnit(player, a.Card) {
		return errors.New("请选择己方港口或停靠港口船上的建设人员；初航使用移民，其余任务使用船员")
	}
	if a.Type == "catan_settlement" {
		if err := x.Cargo.buildSettlementPrice(g, x.Fleet, player, g.TurnSerial, a.Vertex, []int{1, 1, 0, 0, 0}); err != nil {
			return err
		}
	} else {
		kind := "harbor"
		if a.Type == "catan_city" {
			kind = "city"
		}
		if err := x.Cargo.upgradeSettlementPrice(g, x.Fleet, player, g.TurnSerial, a.Vertex, kind, []int{0, 0, 0, 1, 2}); err != nil {
			return err
		}
	}
	x.Cargo.Units[a.Card] = catanExplorerCargoLocation{"supply", -1}
	s.catanLog(player, "本站格雷戈尔：归还建设人员 #%d，以优惠费用完成建筑 #%d", a.Card+1, a.Vertex+1)
	return nil
}

func (s *State) explorerHelperBuilderUnit(player, unit int) bool {
	g, x := s.Catan, s.Catan.Explorer
	if unit < player*11 || unit >= (player+1)*11 {
		return false
	}
	if (x.Board.Scenario == "land-ho") != (unit%11 < 2) {
		return false
	}
	return x.Cargo.buildLocation(g, x.Fleet, player, x.Cargo.Units[unit])
}
