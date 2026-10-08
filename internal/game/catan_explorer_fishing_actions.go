package game

import (
	"errors"
	"slices"
)

var catanExplorerFishCosts = map[string]int{
	"catan_fish_pirate": 2, "catan_fish_steal": 3, "catan_fish_resource": 4,
	"catan_fish_road": 5, "catan_fish_ship": 5, "catan_fish_voyage": 7,
}

func (g *Catan) explorerFishCost(player int, kind string) int {
	cost := catanExplorerFishCosts[kind]
	if len(g.Players) == 2 && player >= 0 && player < 2 && g.Players[player].Score < g.Players[1-player].Score {
		cost-- // T&B two-player fish discount, without importing its turn rules.
	}
	return cost
}

func (s *State) explorerFishReady(player int) bool {
	g := s.Catan
	return g != nil && g.Explorer != nil && g.Fishing != nil && g.Explorer.Setup == nil && !s.Finished &&
		player == s.Turn && player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated && s.CatanPendingActor() < 0 &&
		(s.Phase == "catan_roll" || s.Phase == "catan_turn" || s.Phase == "catan_explorer_move")
}

func (s *State) explorerFishEffect(player int, a Action) error {
	g, x := s.Catan, s.Catan.Explorer
	switch a.Type {
	case "catan_fish_boot":
		if !slices.Contains(g.fishBootTargets(player), a.Target) {
			return errors.New("旧靴子只能交给公开分数不少于你的对手")
		}
	case "catan_fish_steal":
		if !slices.Contains(g.cardTheftTargets(player), a.Target) {
			return errors.New("请选择仍有资源或商品的在场对手")
		}
	case "catan_fish_resource":
		if a.Color < 0 || a.Color >= 5 || g.Bank[a.Color] < 1 {
			return errors.New("请选择银行仍有库存的普通资源")
		}
	case "catan_fish_road":
		if s.Phase != "catan_turn" {
			return errors.New("用鱼建路须在交易建设阶段")
		}
		_, err := x.Cargo.roadPrice(g, x.Fleet, player, g.TurnSerial, a.Edge, true)
		return err
	case "catan_fish_ship":
		if s.Phase != "catan_turn" {
			return errors.New("用鱼造船须在交易建设阶段")
		}
		// Construction-only probe; it must never reveal a hidden hex or copy
		// hidden terrain into legal-choice decisions.
		cargo, fleet := clone(*x.Cargo), clone(*x.Fleet)
		base := *g
		base.Players, base.Bank = slices.Clone(g.Players), slices.Clone(g.Bank)
		base.Players[player].Resources = slices.Clone(g.Players[player].Resources)
		return cargo.buildShipCost(&base, &fleet, player, g.TurnSerial, a.Slot, a.Edge, true)
	case "catan_fish_pirate":
		if s.Phase != "catan_explorer_move" || x.Pirate == nil || x.Pirate.Owner < 0 || x.Pirate.Owner == player || x.Fleet.Turn == nil || x.Fleet.Turn.FishPirate {
			return errors.New("仅航行阶段可用鱼忽略对手海盗，且本航行阶段只需支付一次")
		}
		return x.Fleet.allowed(g, player, g.TurnSerial)
	case "catan_fish_voyage":
		if s.Phase != "catan_explorer_move" {
			return errors.New("额外航行只能在航行阶段使用")
		}
		if err := x.Fleet.allowed(g, player, g.TurnSerial); err != nil {
			return err
		}
		t := x.Fleet.Turn
		if a.Slot < 0 || a.Slot >= len(t.Ships) || a.Slot/3 != player || x.Fleet.Positions[a.Slot] < 0 {
			return errors.New("请选择在地图上的己方船只")
		}
		move := t.Ships[a.Slot]
		if move.Second != nil || !move.Closed && move.Remaining > 0 || move.Spent == 0 && !slices.Contains(x.Cargo.Turn.BuildStopped, a.Slot) {
			return errors.New("只能让已结束首次移动的船再次航行，每船每回合最多一次")
		}
	default:
		return errors.New("此鱼行动不适用于探索者")
	}
	return nil
}

// The caller is State.Apply's complete snapshot transaction. No client field
// chooses a free price or changes movement records directly.
func (s *State) applyCatanExplorerFishingAction(player int, a Action) error {
	g := s.Catan
	if g.Fishing != nil && g.Fishing.Pending != nil {
		return s.catanExplorerReplaceFish(player, a)
	}
	if !s.explorerFishReady(player) || a.Prompt < 1 || uint64(a.Prompt) != g.TurnSerial || a.Skill != "" || a.Choice != "" || len(a.Take)+len(a.Give)+len(a.Cards)+len(a.Targets) > 0 {
		return errors.New("请在自己的有效探险阶段单独使用鱼筹码")
	}
	if err := s.explorerFishEffect(player, a); err != nil {
		return err
	}
	x, f := g.Explorer, g.Fishing
	if a.Type == "catan_fish_boot" {
		if len(a.Tokens) != 0 {
			return errors.New("传递旧靴子不支付鱼筹码")
		}
		points := make([]int, len(g.Players))
		for p := range points {
			points[p] = g.Players[p].Score
		}
		if err := f.Tokens.passBoot(player, a.Target, points); err != nil {
			return err
		}
		s.catanLog(player, "把旧靴子交给玩家 %d，对方获胜需要额外1分", a.Target+1)
	} else {
		cost := g.explorerFishCost(player, a.Type)
		if err := f.Tokens.spend(player, a.Tokens, cost); err != nil {
			return err
		}
		paid := 0
		for _, token := range a.Tokens {
			paid += catanFishValue(token)
		}
		s.catanLog(player, "支付 %d 鱼（费用 %d，多付不找零）", paid, cost)
		switch a.Type {
		case "catan_fish_steal":
			hand := g.Players[a.Target].Resources
			pick := catanRandom(sum(hand))
			for color, count := range hand {
				if pick < count {
					hand[color]--
					g.Players[player].Resources[color]++
					break
				}
				pick -= count
			}
			s.catanLog(player, "用鱼从玩家 %d 处抽取1张手牌（牌面保密）", a.Target+1)
		case "catan_fish_resource":
			g.Bank[a.Color]--
			g.Players[player].Resources[a.Color]++
			s.catanLog(player, "用鱼领取 %s×1", CatanResources[a.Color])
		case "catan_fish_road":
			if err := x.Cargo.buildRoadCost(g, x.Fleet, player, g.TurnSerial, a.Edge, true); err != nil {
				return err
			}
			s.catanLog(player, "用鱼修建道路 #%d", a.Edge+1)
		case "catan_fish_ship":
			awards, err := x.buildShipCost(g, player, g.TurnSerial, a.Slot, a.Edge, true)
			if err != nil {
				return err
			}
			s.catanLog(player, "用鱼在海边 #%d 建造船只", a.Edge+1)
			s.catanExplorerDiscoverLog(player, awards)
		case "catan_fish_pirate":
			x.Fleet.Turn.FishPirate = true
			s.catanLog(player, "用鱼忽略海盗通行费，本航行阶段所有船只有效")
		case "catan_fish_voyage":
			t := x.Fleet.Turn
			move := &t.Ships[a.Slot]
			if t.Current >= 0 && t.Current != a.Slot {
				t.Ships[t.Current].Closed, t.Ships[t.Current].Remaining = true, 0
			}
			move.Second = &catanExplorerSecondVoyage{Stopped: move.Spent == 0, Spent: move.Spent, Wool: move.Wool}
			move.Closed, move.Remaining = false, 4+t.Speed
			t.Current = a.Slot
			if move.Spent == 0 {
				t.Current = -1
			}
			s.catanLog(player, "用鱼让船只 #%d 再次航行，获得 %d 移动点；羊毛每船每回合仍限一次", a.Slot+1, move.Remaining)
		}
	}
	s.Catan.Trade = nil
	s.catanExplorerMissionScore()
	s.catanExplorerVictory()
	return nil
}

func (s *State) catanExplorerFishingChoices(player int) []Action {
	choices := []Action{}
	if !s.explorerFishReady(player) {
		return choices
	}
	g, x := s.Catan, s.Catan.Explorer
	add := func(a Action) {
		a.Prompt = int(g.TurnSerial)
		if a.Type != "catan_fish_boot" {
			a.Tokens = g.fishPayment(player, g.explorerFishCost(player, a.Type))
			if len(a.Tokens) == 0 {
				return
			}
		}
		if s.explorerFishEffect(player, a) == nil {
			choices = append(choices, a)
		}
	}
	for _, target := range g.fishBootTargets(player) {
		add(Action{Type: "catan_fish_boot", Target: target})
	}
	for _, target := range g.cardTheftTargets(player) {
		add(Action{Type: "catan_fish_steal", Target: target})
	}
	for color := range 5 {
		add(Action{Type: "catan_fish_resource", Color: color})
	}
	if s.Phase == "catan_turn" {
		if g.fishPayment(player, g.explorerFishCost(player, "catan_fish_road")) != nil {
			for _, edge := range g.Edges {
				add(Action{Type: "catan_fish_road", Edge: edge.ID})
			}
		}
		if g.fishPayment(player, g.explorerFishCost(player, "catan_fish_ship")) != nil {
			for ship := player * 3; ship < (player+1)*3; ship++ {
				for _, edge := range g.Edges {
					if x.Cargo.holder(g, x.Fleet, player, catanExplorerCargoLocation{"harbor", edge.A}) || x.Cargo.holder(g, x.Fleet, player, catanExplorerCargoLocation{"harbor", edge.B}) {
						add(Action{Type: "catan_fish_ship", Slot: ship, Edge: edge.ID})
					}
				}
			}
		}
	}
	if s.Phase == "catan_explorer_move" {
		add(Action{Type: "catan_fish_pirate"})
		for ship := player * 3; ship < (player+1)*3; ship++ {
			add(Action{Type: "catan_fish_voyage", Slot: ship})
		}
	}
	return choices
}

func (s *State) catanExplorerFishingLegal(player int) map[string]any {
	costs := clone(catanExplorerFishCosts)
	for kind := range costs {
		costs[kind] = s.Catan.explorerFishCost(player, kind)
	}
	legal := map[string]any{"costs": costs, "actions": []string{}, "resources": []int{}, "targets": []int{}, "roads": []int{}, "ships": []int{}, "bootTargets": []int{}, "progressTracks": []int{}, "voyages": []int{}, "shipBuilds": []map[string]int{}}
	for _, a := range s.catanExplorerFishingChoices(player) {
		if !slices.Contains(legal["actions"].([]string), a.Type) {
			legal["actions"] = append(legal["actions"].([]string), a.Type)
		}
		key, value := "", 0
		switch a.Type {
		case "catan_fish_boot":
			key, value = "bootTargets", a.Target
		case "catan_fish_steal":
			key, value = "targets", a.Target
		case "catan_fish_resource":
			key, value = "resources", a.Color
		case "catan_fish_road":
			key, value = "roads", a.Edge
		case "catan_fish_voyage":
			key, value = "voyages", a.Slot
		case "catan_fish_ship":
			legal["shipBuilds"] = append(legal["shipBuilds"].([]map[string]int), map[string]int{"slot": a.Slot, "edge": a.Edge})
		}
		if key != "" {
			legal[key] = append(legal[key].([]int), value)
		}
	}
	return legal
}

// Uses only own fish faces, own hand and public geometry/counts.
func (s *State) catanExplorerFishingBot(player int) (Action, bool) {
	if s.Catan == nil || s.Catan.Fishing == nil {
		return Action{}, false
	}
	choices := s.catanExplorerFishingChoices(player)
	g := s.Catan
	for _, a := range choices {
		if a.Type == "catan_fish_boot" {
			return a, true
		}
	}
	if s.Phase == "catan_turn" {
		road := catanExplorerBotRoad(g, player)
		for _, a := range choices {
			if a.Type == "catan_fish_road" && a.Edge == road {
				return a, true
			}
		}
		// Prefer an unused ship slot; avoid gratuitous dismantling of cargo.
		if g.Explorer.Fleet.Positions[player*3+1] < 0 {
			for _, a := range choices {
				if a.Type == "catan_fish_ship" && a.Slot == player*3+1 {
					return a, true
				}
			}
		}
		best := -1
		for i, a := range choices {
			if a.Type == "catan_fish_resource" && (best < 0 || g.Players[player].Resources[a.Color] < g.Players[player].Resources[choices[best].Color]) {
				best = i
			}
		}
		if best >= 0 {
			return choices[best], true
		}
		for _, a := range choices {
			if a.Type == "catan_fish_steal" {
				return a, true
			}
		}
	}
	if s.Phase == "catan_explorer_move" {
		for _, a := range choices {
			if a.Type == "catan_fish_pirate" && g.Explorer.Economy.Gold[player] == 0 {
				return a, true
			}
			if a.Type == "catan_fish_voyage" {
				return a, true
			}
		}
	}
	return Action{}, false
}
