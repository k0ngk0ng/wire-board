package game

import (
	"errors"
	"slices"
)

// Private acceptance constructor. Public room options remain closed until
// inventory edge cases, UI and remaining missions pass final acceptance.
func newCatanExplorerState(players int) (*State, error) {
	g, x, err := newCatanExplorerLandHoWorld(players)
	if err != nil {
		return nil, err
	}
	g.Explorer = x
	g.StartPlayer = catanRandom(players)
	g.SetupStep, g.SetupVertex = 2*players, -1
	g.TurnSerial = 1
	g.Dice = []int{0, 0}
	g.DiscardDue = make([]int, players)
	g.Victims = []int{}
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	s := &State{Kind: "catan", Catan: g, Turn: g.StartPlayer, Phase: "catan_roll", Round: 1, Log: []string{"探险家与海盗·初航：随机先手，采用印刷开局；自己回合达到8分获胜", "港口每格只产1资源；不使用发展卡、强盗、最长道路和最大军队"}}
	if err = x.Economy.beginProduction(g, x.Fleet, x.Cargo, s.Turn, 1); err != nil {
		return nil, err
	}
	return s, s.validateCatanExplorer()
}

func (s *State) validateCatanExplorer() error {
	g := s.Catan
	if g == nil || g.Explorer == nil {
		return errors.New("探险家主状态缺失")
	}
	x := g.Explorer
	if g.CitiesKnights != nil || x.Board != nil && x.Board.CitiesKnights {
		return s.validateCatanExplorerCities()
	}
	if err := x.validate(g); err != nil {
		return err
	}
	if err := s.validateCatanExplorerPaired(); err != nil {
		return err
	}
	if x.Setup != nil {
		step := x.Setup.current(len(g.Players))
		if step == nil || s.Kind != "catan" || s.Phase != "catan_explorer_setup" || s.Turn != step.Player || s.Round != 1 || s.Finished || len(s.Winners) > 0 || g.RollID != 0 {
			return errors.New("巢穴主状态开局阶段无效")
		}
		return nil
	}
	if s.Kind != "catan" || s.Turn < 0 || s.Turn >= len(g.Players) || g.StartPlayer < 0 || g.StartPlayer >= len(g.Players) || s.Round < 1 || g.SetupStep != 2*len(g.Players) || g.TurnSerial == 0 || g.TurnSerial > uint64(^uint(0)>>1) || len(g.Dice) != 2 || len(g.DiscardDue) != len(g.Players) {
		return errors.New("探险家整局人数、轮次或起始状态无效")
	}
	t := x.Economy.Turn
	if t == nil || t.Player != s.Turn || t.Sequence != g.TurnSerial || !slices.Equal(t.Discard, g.DiscardDue) {
		return errors.New("探险家主回合与经济回应不一致")
	}
	if x.SkippedRolls < 0 || uint64(x.SkippedRolls) >= g.TurnSerial {
		return errors.New("离场跳过的生产回合数量无效")
	}
	productionTurns := g.TurnSerial
	if g.Paired != nil {
		productionTurns = (g.TurnSerial + 1) / 2
	}
	rolled := int(productionTurns) - x.SkippedRolls
	if t.Phase == "roll" {
		rolled--
	}
	if rolled < 0 || g.RollID != rolled {
		return errors.New("探险家回合与实际生产次数不一致")
	}
	allPresent := true
	for _, p := range g.Players {
		allPresent = allPresent && !p.Eliminated
	}
	if allPresent && g.Paired == nil && (s.Turn != (g.StartPlayer+int((g.TurnSerial-1)%uint64(len(g.Players))))%len(g.Players) || s.Round != 1+int((g.TurnSerial-1)/uint64(len(g.Players)))) {
		return errors.New("探险家随机先手后的顺时针轮转不一致")
	}
	phase := s.catanExplorerPhase()
	if x.Lairs != nil && x.Lairs.RewardVictory != nil && (!s.Finished || x.Lairs.RewardVictory.Player != s.Turn) {
		return errors.New("巢穴领奖达到目标后必须立即结束")
	}
	if s.Finished {
		alive := 0
		for _, p := range g.Players {
			if !p.Eliminated {
				alive++
			}
		}
		if s.Phase != "finished" || len(s.Winners) != 1 || s.Winners[0] != s.Turn || g.Players[s.Turn].Eliminated || g.Players[s.Turn].Score < x.Board.Target && alive != 1 || g.Trade != nil {
			return errors.New("初航胜负或结束阶段无效")
		}
	} else if phase == "" || s.Phase != phase || len(s.Winners) != 0 {
		return errors.New("探险家主游戏阶段不一致")
	}
	if t.Phase != "roll" && (g.Dice[0] != t.Dice[0] || g.Dice[1] != t.Dice[1]) {
		return errors.New("主状态与生产骰子不一致")
	}
	if trade := g.Trade; trade != nil {
		if g.Paired != nil && g.Paired.Second || s.Phase != "catan_turn" || trade.From != s.Turn || trade.ID != g.TradeID || trade.ID <= 0 || !catanBundle(trade.Give) || !catanBundle(trade.Take) || !g.validTradeGold(trade.GoldGive) || !g.validTradeGold(trade.GoldTake) || len(trade.Responses) != len(g.Players) || sum(trade.Give)+trade.GoldGive == 0 || sum(trade.Take)+trade.GoldTake == 0 {
			return errors.New("探险家交易提议无效")
		}
		for _, r := range trade.Responses {
			if r < -1 || r > 1 {
				return errors.New("交易回应无效")
			}
		}
	}
	return nil
}

func (s *State) catanExplorerSyncPhase() {
	g := s.Catan
	if g.Explorer.Economy.Turn != nil {
		g.DiscardDue = slices.Clone(g.Explorer.Economy.Turn.Discard)
	}
	s.Phase = s.catanExplorerPhase()
}

func (s *State) catanExplorerVictory() {
	g := s.Catan
	if !g.Players[s.Turn].Eliminated && g.Players[s.Turn].Score >= g.Explorer.Board.Target {
		s.Finished, s.Phase, s.Winners = true, "finished", []int{s.Turn}
		g.Trade = nil
		s.catanLog(s.Turn, "达到 %d 分，赢得探险任务", g.Players[s.Turn].Score)
	}
}

func (s *State) catanExplorerRoll(dice [2]int) error {
	g := s.Catan
	x := g.Explorer
	result, err := x.Economy.resolveProduction(g, x.Fleet, x.Cargo, s.Turn, g.TurnSerial, dice)
	if err != nil {
		return err
	}
	g.Dice = []int{dice[0], dice[1]}
	g.RollID++
	s.catanLog(s.Turn, "掷出 %d + %d = %d", dice[0], dice[1], dice[0]+dice[1])
	for p, resources := range result.Resources {
		if sum(resources) > 0 || result.Gold[p] > 0 {
			s.catanLog(p, "生产获得 %s", catanTradeText(resources, result.Gold[p]))
		}
	}
	return s.catanExplorerAfterProduction()
}

func (s *State) catanExplorerDiscoverLog(player int, awards []catanExplorerDiscovery) {
	for _, a := range awards {
		if a.Resource >= 0 && a.Resource < 5 && sum(a.Resources) == 0 {
			s.catanLog(player, "探索地块 #%d，%s库存为空，未领取探索资源", a.Tile+1, CatanResources[a.Resource])
		} else {
			s.catanLog(player, "探索地块 #%d，获得 %s", a.Tile+1, catanTradeText(a.Resources, a.Gold))
		}
	}
}

func (s *State) applyCatanExplorer(player int, a Action) error {
	g := s.Catan
	x := g.Explorer
	if x.Setup != nil {
		return s.applyCatanExplorerSetup(player, a)
	}
	sequence := g.TurnSerial
	if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || a.Prompt < 1 || uint64(a.Prompt) != sequence {
		return errors.New("探险行动玩家或回合序号无效")
	}
	if handled, err := s.applyCatanExplorerMission(player, a); handled {
		return err
	}
	if a.Type == "catan_discard" {
		if err := x.Economy.discard(g, x.Fleet, x.Cargo, player, sequence, a.Tokens); err != nil {
			return err
		}
		s.catanLog(player, "七点归还 %d 张资源", sum(a.Tokens)) // Do not reveal discarded composition.
		return s.catanExplorerAfterProduction()
	}
	if a.Type == "catan_trade_accept" || a.Type == "catan_trade_reject" {
		return s.catanRespondTrade(player, a)
	}
	if player != s.Turn {
		return errors.New("还没有轮到你")
	}
	var err error
	switch a.Type {
	case "catan_roll":
		return s.catanExplorerRoll([2]int{catanRandom(6) + 1, catanRandom(6) + 1})
	case "catan_trade_offer":
		return s.catanOffer(player, a)
	case "catan_trade_complete":
		return s.catanCompleteTrade(player, a)
	case "catan_trade_cancel":
		if s.Phase != "catan_turn" || g.Trade == nil || g.Trade.ID != a.Offer {
			return errors.New("此交易已结束")
		}
		g.Trade = nil
		return nil
	case "catan_explorer_bank":
		err = x.Economy.bankTrade(g, x.Fleet, x.Cargo, player, sequence, a.Color, a.Target)
		if err == nil {
			give, take := make([]int, 5), make([]int, 5)
			gg, tg := 0, 0
			if a.Color < 0 {
				gg = 2
			} else {
				give[a.Color] = 3
			}
			if a.Target < 0 {
				tg = 1
			} else {
				take[a.Target] = 1
			}
			s.catanLog(player, "向银行支付 %s，获得 %s", catanTradeText(give, gg), catanTradeText(take, tg))
		}
	case "catan_road":
		err = x.Cargo.buildRoad(g, x.Fleet, player, sequence, a.Edge)
		if err == nil {
			s.catanLog(player, "支付 木材×1、砖块×1，修建道路 #%d", a.Edge+1)
		}
	case "catan_settlement":
		err = x.Cargo.buildSettlement(g, x.Fleet, player, sequence, a.Vertex)
		if err == nil {
			s.catanLog(player, "支付 木砖羊粮各1，建造村庄 #%d", a.Vertex+1)
		}
	case "catan_explorer_harbor":
		err = x.Cargo.buildHarbor(g, x.Fleet, player, sequence, a.Vertex)
		if err == nil {
			s.catanLog(player, "支付 粮食×2、矿石×2，将村庄 #%d 升级为港口", a.Vertex+1)
		}
	case "catan_explorer_ship":
		var awards []catanExplorerDiscovery
		awards, err = x.buildShip(g, player, sequence, a.Slot, a.Edge)
		if err == nil {
			s.catanLog(player, "支付 木材×1、羊毛×1，在海边 #%d 建造船只", a.Edge+1)
			s.catanExplorerDiscoverLog(player, awards)
		}
	case "catan_explorer_unit":
		err = x.Cargo.buildUnitFreight(g, x.Fleet, player, sequence, a.Card, catanExplorerCargoLocation{a.Choice, a.Target}, a.Cards, a.Targets, a.SpiceUnload)
		if err == nil {
			if len(a.Targets) > 0 {
				s.catanLog(player, "归还 鱼群×1 腾出招募舱位，未交付任务")
			}
			if len(a.SpiceUnload) > 0 {
				s.catanLog(player, "归还 香料×1 腾出招募舱位，未交付任务；农场能力保留")
			}
			if a.Card%11 < 2 {
				s.catanLog(player, "支付 木砖羊粮各1，建造移民")
			} else {
				s.catanLog(player, "支付 羊毛×1、矿石×1，招募船员")
			}
		}
	case "catan_explorer_begin_move":
		err = x.Cargo.beginMovement(g, x.Fleet, player, sequence, x.Cargo.farmCount(x.Board, player, "swift"))
		if err == nil {
			s.Phase = "catan_explorer_move"
			s.catanLog(player, "结束交易建设，开始航行")
		}
	case "catan_explorer_sail":
		var result catanExplorerVoyage
		result, err = x.sail(g, player, sequence, a.Slot, a.Targets)
		if err == nil {
			s.catanLog(player, "船只航行 %d 步至海边 #%d", result.Sail.Points, result.Sail.To+1)
			s.catanExplorerDiscoverLog(player, result.Discoveries)
		}
	case "catan_explorer_wool":
		err = x.Fleet.wool(g, player, sequence, a.Slot)
		if err == nil {
			s.catanLog(player, "支付 羊毛×1，为船只增加2点移动")
		}
	case "catan_explorer_transfer":
		err = x.Cargo.transferAllFreight(g, x.Fleet, player, sequence, a.Slot, a.Vertex, a.Give, a.Take, a.Cards, a.Targets, a.SpiceLoad, a.SpiceUnload)
		if err == nil {
			s.catanLog(player, "在港口 #%d 装卸货物", a.Vertex+1)
		}
	case "catan_explorer_settle":
		err = x.Cargo.settle(g, x.Fleet, player, sequence, a.Slot, a.Vertex)
		if err == nil {
			s.catanLog(player, "移民在 #%d 定居，归还船只与移民，获得1分", a.Vertex+1)
		}
	case "catan_end":
		if err = x.Cargo.endMovement(g, x.Fleet, player, sequence); err != nil {
			return err
		}
		if x.Lairs != nil && s.catanExplorerHasReadyLair() {
			s.catanExplorerSyncPhase()
			return nil
		}
		return s.catanExplorerNextTurn()
	default:
		return errors.New("此行动不适用于初航")
	}
	if err != nil {
		return err
	}
	// x may have committed a copied aggregate during sailing/construction.
	g = s.Catan
	g.Trade = nil
	s.catanExplorerMissionScore()
	s.catanExplorerVictory()
	return nil
}

func (s *State) catanExplorerView(v map[string]any, viewer int) {
	g := s.Catan
	x := g.Explorer
	v["explorer"] = map[string]any{"board": x.Board.publicView(), "fleet": clone(x.Fleet), "cargo": clone(x.Cargo), "economy": x.Economy.publicView(), "sequence": g.TurnSerial, "choices": catanExplorerChoiceView(s.catanExplorerChoices(viewer))}
	v["explorer"].(map[string]any)["actionId"] = x.ActionID
	v["explorer"].(map[string]any)["motion"] = clone(x.Motion)
	if x.Setup != nil {
		v["explorer"].(map[string]any)["setup"] = clone(x.Setup)
		v["explorer"].(map[string]any)["setupPlacement"] = x.Setup.current(len(g.Players))
		v["explorer"].(map[string]any)["sequence"] = x.Setup.prompt()
		v["explorer"].(map[string]any)["setupBlocked"] = s.CatanExplorerSetupBlocked()
	}
	if x.Pirate != nil {
		v["explorer"].(map[string]any)["pirate"] = clone(x.Pirate)
	}
	if x.Lairs != nil {
		v["explorer"].(map[string]any)["lairs"] = x.Lairs.publicView(x.Board)
	}
	if x.Spice != nil {
		v["explorer"].(map[string]any)["spice"] = x.Spice.publicView(g)
	}
	if x.Fish != nil {
		v["explorer"].(map[string]any)["fish"] = x.Fish.publicView(len(g.Players))
	}
	v["victoryTarget"] = x.Board.Target
	v["setupLimit"] = g.SetupLimit()
	delete(v, "devDeck")
	delete(v, "devDiscard")
	delete(v, "resumePhase")
	for p, raw := range v["players"].([]any) {
		public := raw.(map[string]any)
		actual := g.Players[p]
		public["resourceCount"] = sum(actual.Resources)
		public["publicScore"] = actual.Score
		public["rates"] = []int{3, 3, 3, 3, 3}
		if g.CitiesKnights != nil {
			public["rates"] = g.explorerCityRates(p)
			public["citiesLeft"] = g.cityPiecesLeft(p)
		}
		delete(public, "dev")
		delete(public, "newDev")
		if p != viewer && !s.Finished {
			delete(public, "resources")
		}
	}
	// No generic base-game choices: explorer actions have different costs and
	// ships. Actor-only choices above provide the Explorer action previews.
	v["legal"] = map[string][]int{}
	if g.CitiesKnights != nil {
		s.catanExplorerCityView(v, viewer)
	}
}
