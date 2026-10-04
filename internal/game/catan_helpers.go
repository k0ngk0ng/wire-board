package game

import (
	"errors"
	"slices"
)

type CatanHelper struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	OriginalName string `json:"originalName"`
	Title        string `json:"title"`
	Description  string `json:"description"`
}

func CatanHelpers() []CatanHelper {
	return []CatanHelper{
		{1, "阿斯拉", "Asla", "强制交易", "指定一种资源，依次向至多两位对手索取一张；每获得一张，就给出一张自选资源作为交换。"},
		{2, "英格维", "Yngvi", "灵活筑路", "建造一条道路时，可以用任意一张资源替代一张木材或砖块。"},
		{3, "希尔达", "Hilda", "资源补偿", "任意玩家掷出非7且你没有获得生产资源时，可从银行领取一张自选资源。"},
		{4, "霍格尼", "Högni", "迁移道路", "将自己的一条末端道路移动到另一个合法位置。"},
		{5, "托罗夫", "Thorolf", "七点庇护", "任何玩家掷出7时必须使用：超过七张资源免弃牌，否则领取一张自选资源。"},
		{6, "迪亚拉", "Diara", "择选发展卡", "购买发展卡时可替换一种所需资源，查看牌堆顶三张并选一张，其余洗回牌堆。"},
		{7, "瑞安", "Ryan", "向领先者取牌", "掷骰结算后，查看一位公开分数比你高的对手的资源手牌，拿走一张自选资源。"},
		{8, "格雷戈尔", "Gregor", "骑士参与建设", "移除一张已打出的骑士：用木材和砖块各一建村庄，或用矿石二、粮食一升级城市。"},
		{9, "斯蒂娜", "Stina", "集中二比一交易", "选一种资源，一次性以二比一向银行进行任意多笔交换。"},
		{10, "迪古尔", "Digur", "驱逐强盗", "自己掷骰前或结算后，把强盗赶回沙漠，并领取原地块出产的一张资源。"},
		{11, "卡娅", "Kaja", "强盗资源", "领取强盗所在地块对应的一张资源；强盗在沙漠时可以任选一种。"},
		{12, "卡拉", "Carla", "更换发展卡", "把一张未打出的发展卡放到牌堆底，再摸顶牌；新牌依然受本回合不可打出的限制。"},
	}
}

type CatanHelperSeat struct {
	ID           int    `json:"id"`
	Moon         bool   `json:"moon"`
	AcquiredTurn uint64 `json:"acquiredTurn"`
	UsedTurn     uint64 `json:"usedTurn"`
}

type CatanHelperPending struct {
	Player   int    `json:"player"`
	Kind     string `json:"kind"`
	Resume   string `json:"resume"`
	Cards    []int  `json:"cards,omitempty"`
	Target   int    `json:"target,omitempty"`
	Optional bool   `json:"optional,omitempty"`
}

func (g *Catan) helperReady(player, id int) bool {
	if !g.Options.Helpers || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated {
		return false
	}
	h := g.Players[player].Helper
	return h != nil && h.ID == id && h.AcquiredTurn < g.TurnSerial && h.UsedTurn < g.TurnSerial
}

func (s *State) catanHelperAsk(q CatanHelperPending) {
	g := s.Catan
	g.HelperSequence++
	g.HelperPending = &q
	s.Phase = "catan_helper"
}

func (s *State) catanHelperComplete(player int, resume string) {
	g := s.Catan
	h := g.Players[player].Helper
	h.UsedTurn = g.TurnSerial
	s.catanLog(player, "使用助手「%s」的%s能力", CatanHelpers()[h.ID-1].Name, CatanHelpers()[h.ID-1].Title)
	g.Trade = nil
	if s.Finished {
		g.HelperPending = nil
		return
	}
	s.catanHelperAsk(CatanHelperPending{Player: player, Kind: "exchange", Resume: resume})
}

func (s *State) catanHelperRespond(player int, a Action) error {
	g := s.Catan
	q := g.HelperPending
	if q == nil || q.Player != player || a.Type != "catan_helper_choice" {
		return errors.New("请等待对应玩家完成助手操作")
	}
	if a.Choice == "skip" && q.Optional {
		s.Phase = q.Resume
		g.HelperPending = nil
		return nil
	}
	h := g.Players[player].Helper
	switch q.Kind {
	case "exchange":
		if a.Choice == "flip" {
			if h.Moon {
				return errors.New("月面助手使用后必须交换")
			}
			h.Moon = true
			s.catanLog(player, "保留助手「%s」并翻至月面", CatanHelpers()[h.ID-1].Name)
		} else if a.Choice == "exchange" {
			index := slices.Index(g.HelperDisplay, a.Card)
			if index < 0 {
				return errors.New("请选择展示区中的另一位助手")
			}
			g.HelperDisplay[index] = h.ID
			g.Players[player].Helper = &CatanHelperSeat{ID: a.Card, AcquiredTurn: g.TurnSerial, UsedTurn: g.TurnSerial}
			s.catanLog(player, "换取助手「%s」（太阳面）", CatanHelpers()[a.Card-1].Name)
		} else {
			return errors.New("请选择保留翻面或交换助手")
		}
		s.Phase = q.Resume
		g.HelperPending = nil
	case "resource":
		if a.Color < 0 || a.Color >= 5 || g.Bank[a.Color] == 0 {
			return errors.New("请选择银行仍有库存的资源")
		}
		g.Bank[a.Color]--
		g.Players[player].Resources[a.Color]++
		s.catanLog(player, "通过助手领取%s×1", CatanResources[a.Color])
		s.catanHelperComplete(player, q.Resume)
	case "development":
		index := slices.Index(q.Cards, a.Card)
		if index < 0 {
			return errors.New("请选择展示给你的发展卡")
		}
		g.Players[player].Dev[a.Card]++
		g.Players[player].NewDev[a.Card]++
		for i, card := range q.Cards {
			if i != index {
				g.DevDeck = append(g.DevDeck, card)
			}
		}
		shuffle(g.DevDeck)
		s.catanLog(player, "从助手展示的候选中获得一张发展卡，其余洗回牌堆")
		s.catanScores()
		s.catanVictory()
		s.catanHelperComplete(player, q.Resume)
	case "leader":
		if a.Color < 0 || a.Color >= 5 || g.Players[q.Target].Resources[a.Color] == 0 {
			return errors.New("请选择对方手中存在的资源")
		}
		g.Players[q.Target].Resources[a.Color]--
		g.Players[player].Resources[a.Color]++
		// Only the helper user sees the chosen resource and the revealed hand.
		s.catanLog(player, "通过助手从玩家 %d 拿走一张资源", q.Target+1)
		s.catanHelperComplete(player, q.Resume)
	default:
		return errors.New("未知助手选择")
	}
	return nil
}

func helperSubstituteCost(base, selected []int) ([]int, error) {
	if len(selected) == 0 {
		return append([]int{}, base...), nil
	}
	if !catanBundle(selected) || sum(selected) != sum(base) {
		return nil, errors.New("助手只能替代一张所需资源")
	}
	difference := 0
	for i, n := range selected {
		difference += max(0, n-base[i])
	}
	if difference > 1 {
		return nil, errors.New("助手只能替代一张所需资源")
	}
	return append([]int{}, selected...), nil
}

func (s *State) catanHelperBuildCost(player int, a Action) ([]int, error) {
	g := s.Catan
	h := g.Players[player].Helper
	if h == nil || !g.helperReady(player, h.ID) || s.Phase != "catan_turn" {
		return nil, errors.New("当前不能使用这位助手")
	}
	if h.ID == 2 && a.Type == "catan_road" || h.ID == 6 && a.Type == "catan_buy_dev" {
		return helperSubstituteCost(catanPrices[a.Type], a.Tokens)
	}
	if h.ID == 8 && g.Players[player].Knights > 0 {
		if a.Type == "catan_city" {
			return []int{0, 0, 0, 1, 2}, nil
		}
		if a.Type == "catan_settlement" {
			return []int{1, 1, 0, 0, 0}, nil
		}
	}
	return nil, errors.New("该助手不支持此建造，或尚无已打出的骑士")
}

func (s *State) catanHelperSpendKnight(player int) {
	g := s.Catan
	g.Players[player].Knights--
	if i := slices.Index(g.DevDiscard, 0); i >= 0 {
		g.DevDiscard = append(g.DevDiscard[:i], g.DevDiscard[i+1:]...)
	}
	g.HelperExile = append(g.HelperExile, 0)
}

func (s *State) catanHelperDevelopment(player int) error {
	g := s.Catan
	count := min(3, len(g.DevDeck))
	cards := append([]int{}, g.DevDeck[len(g.DevDeck)-count:]...)
	g.DevDeck = g.DevDeck[:len(g.DevDeck)-count]
	s.catanLog(player, "通过助手购买发展卡，私下查看至多三张候选")
	s.catanHelperAsk(CatanHelperPending{Player: player, Kind: "development", Resume: "catan_turn", Cards: cards})
	return nil
}

func (g *Catan) helperEndRoad(player, edge int) bool {
	if edge < 0 || edge >= len(g.Edges) || g.Edges[edge].Owner != player {
		return false
	}
	e := g.Edges[edge]
	for _, end := range []int{e.A, e.B} {
		if g.Vertices[end].Owner == player {
			continue
		}
		connected := false
		for _, other := range g.Edges {
			if other.ID != edge && other.Owner == player && (other.A == end || other.B == end) {
				connected = true
				break
			}
		}
		if !connected {
			return true
		}
	}
	return false
}

func (s *State) catanHelperAction(player int, a Action) error {
	g := s.Catan
	p := &g.Players[player]
	h := p.Helper
	if h == nil || !g.helperReady(player, h.ID) || (s.Phase != "catan_turn" && !(s.Phase == "catan_roll" && h.ID == 10)) {
		return errors.New("这位助手当前不能使用；新获得或本回合使用过的助手需等下一回合")
	}
	resume := s.Phase
	switch h.ID {
	case 1:
		if a.Color < 0 || a.Color >= 5 || len(a.Targets) < 1 || len(a.Targets) > 2 || len(a.Cards) != len(a.Targets) {
			return errors.New("选择一种资源、至多两位对手及分别给出的资源")
		}
		seen := map[int]bool{}
		for i, target := range a.Targets {
			give := a.Cards[i]
			if target < 0 || target >= len(g.Players) || target == player || seen[target] || g.Players[target].Eliminated || give < 0 || give >= 5 {
				return errors.New("交易对象或交换资源不正确")
			}
			seen[target] = true
			if g.Players[target].Resources[a.Color] == 0 {
				s.catanLog(player, "向玩家 %d 索取%s，对方没有该资源", target+1, CatanResources[a.Color])
				continue
			}
			g.Players[target].Resources[a.Color]--
			p.Resources[a.Color]++
			if p.Resources[give] == 0 {
				return errors.New("没有所选资源用于交换")
			}
			p.Resources[give]--
			g.Players[target].Resources[give]++
			s.catanLog(player, "用%s×1向玩家 %d 换取%s×1", CatanResources[give], target+1, CatanResources[a.Color])
		}
	case 4:
		if !g.helperEndRoad(player, a.Edge) || a.Edge == a.Target {
			return errors.New("只能移动己方末端道路到另一个位置")
		}
		g.Edges[a.Edge].Owner = -1
		if !g.canRoad(player, a.Target) {
			return errors.New("新道路位置不符合连接规则")
		}
		g.Edges[a.Target].Owner = player
		s.catanLog(player, "通过助手将道路 #%d 移到 #%d", a.Edge+1, a.Target+1)
		s.catanScores()
		s.catanVictory()
	case 7:
		if a.Target < 0 || a.Target >= len(g.Players) || a.Target == player || g.Players[a.Target].Eliminated {
			return errors.New("请选择一位领先的对手")
		}
		target := g.Players[a.Target]
		if target.Score-target.Dev[4] <= p.Score-p.Dev[4] || sum(target.Resources) == 0 {
			return errors.New("该对手公开分数未领先或没有资源")
		}
		s.catanHelperAsk(CatanHelperPending{Player: player, Kind: "leader", Resume: resume, Target: a.Target})
		return nil
	case 9:
		if !catanBundle(a.Give) || !catanBundle(a.Take) || !catanHas(p.Resources, a.Give) || !catanHas(g.Bank, a.Take) || sum(a.Take) == 0 || sum(a.Give) != 2*sum(a.Take) {
			return errors.New("请一次性以同一种资源按二比一交换")
		}
		types := 0
		for i, n := range a.Give {
			if n > 0 {
				types++
				if a.Take[i] > 0 {
					return errors.New("不能兑换给出的同一种资源")
				}
			}
		}
		if types != 1 {
			return errors.New("二比一集中交易只能给出同一种资源")
		}
		catanMove(p.Resources, g.Bank, a.Give)
		catanMove(g.Bank, p.Resources, a.Take)
		s.catanLog(player, "通过助手支付%s，换得%s", catanText(a.Give), catanText(a.Take))
	case 10:
		resource := g.Tiles[g.Robber].Resource
		if resource == 5 {
			return errors.New("强盗已经在沙漠")
		}
		desert := -1
		for _, tile := range g.Tiles {
			if tile.Resource == 5 {
				desert = tile.ID
				break
			}
		}
		if desert < 0 {
			return errors.New("地图上没有沙漠")
		}
		g.Robber = desert
		if resource >= 0 && resource < 5 && g.Bank[resource] > 0 {
			g.Bank[resource]--
			p.Resources[resource]++
		}
		s.catanLog(player, "通过助手将强盗赶回沙漠")
	case 11:
		resource := g.Tiles[g.Robber].Resource
		if resource == 5 {
			resource = a.Color
		}
		if resource < 0 || resource >= 5 || g.Bank[resource] == 0 {
			return errors.New("请选择有库存的对应资源")
		}
		g.Bank[resource]--
		p.Resources[resource]++
		s.catanLog(player, "通过助手领取%s×1", CatanResources[resource])
	case 12:
		if a.Card < 0 || a.Card >= 5 || p.Dev[a.Card] == 0 {
			return errors.New("请选择自己未打出的发展卡")
		}
		p.Dev[a.Card]--
		if p.NewDev[a.Card] > 0 {
			p.NewDev[a.Card]--
		}
		g.DevDeck = append([]int{a.Card}, g.DevDeck...)
		card := g.DevDeck[len(g.DevDeck)-1]
		g.DevDeck = g.DevDeck[:len(g.DevDeck)-1]
		p.Dev[card]++
		p.NewDev[card]++
		s.catanLog(player, "通过助手将一张发展卡放回牌堆底，并摸一张新牌")
		s.catanScores()
		s.catanVictory()
	default:
		return errors.New("这位助手需要在对应的建造或掷骰结算时使用")
	}
	s.catanHelperComplete(player, resume)
	return nil
}
