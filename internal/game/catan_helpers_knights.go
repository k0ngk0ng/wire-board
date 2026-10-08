package game

import (
	"errors"
	"slices"
)

// Helpers documents base/Seafarers only. This explicit version pins the site's
// adaptation of development cards, physical knights and commodity production.
const CatanHelpersKnightsRules = "wire-board-helpers-knights-v1"
const catanHelpersKnightsNotice = "本站骑士助手：发展卡能力适配进步牌，格雷戈尔归还实体骑士参与建设；普通资源与商品分开，资源助手不交换商品；希尔达与引水渠分别补偿，托罗夫按城墙手牌上限保护"

type catanHelpersKnights struct {
	Rules      string                     `json:"rules"`
	Production *catanHelperCityProduction `json:"production,omitempty"`
}
type catanHelperCityProduction struct {
	RollID int `json:"rollId"`
	Player int `json:"player"`
}

func (g *Catan) cityHelpers() bool {
	return g.Explorer == nil && g.CitiesKnights != nil && g.Options.Helpers && g.CitiesKnights.Helpers != nil && g.CitiesKnights.Helpers.Rules == CatanHelpersKnightsRules
}
func (g *Catan) cityHelperDescriptions(rules []CatanHelper) {
	if !g.cityHelpers() {
		return
	}
	rules[2].Description = "本站骑士组合：非7点生产未获得资源或商品时，可领一张普通资源；先完成金矿与引水渠，引水渠、鱼筹码和事件奖励不取消此资格。"
	rules[4].Description = "本站骑士组合：7点时必须使用；资源与商品合计超过个人城墙上限则免弃牌，否则领一张普通资源。"
	rules[5].Title = "择选进步牌（本站）"
	rules[5].Description = "支付羊毛、粮食、矿石各1，可用另一种普通资源替换其中1张；选择一种进步牌堆，私下查看顶端至多3张并选1张，其余洗回该堆。胜利点牌立即公开，其余可按进步牌规则使用。"
	rules[6].Description = "查看一位公开分数比你高的对手的普通资源，拿走一张；不查看或拿取商品。"
	rules[7].Description = "本站骑士组合：归还自己任意一名实体骑士（无论等级或激活状态），木砖各1建村或粮1矿2升城；不获得贸易筹码。不能牺牲中立骑士。"
	rules[9].Description = "自己生产前或结算后，把强盗赶回沙漠并领取原地块的一张普通资源；首次蛮族入侵前不能使用。"
	rules[10].Description = "领取强盗所在地的一张普通资源，沙漠或金矿任选；不领取商品，强盗未入场时不能使用。"
	if g.fishingHelpers() {
		rules[9].Description = "本站渔夫骑士组合：自己生产前或结算后把强盗赶回沙漠，无沙漠时移到场外；原地为湖泊或金矿时任选普通资源，不领取商品。强盗尚未入场或已在场外时不能使用。"
		rules[10].Description = "领取强盗所在地的一张普通资源，湖泊、沙漠或金矿任选；不领取商品，强盗在场外时不能使用。"
	}
	rules[11].Title = "更换进步牌（本站）"
	rules[11].Description = "将一张私有进步牌放回对应颜色牌堆底部，再抽该堆顶牌；公开胜利点牌不能换。新牌沿用进步牌的立即使用及手牌上限规则。"
}
func (s *State) catanCityHelperAction(player int, a Action) (bool, error) {
	g := s.Catan
	if !g.cityHelpers() {
		return false, nil
	}
	h := g.Players[player].Helper
	if h == nil || h.ID != 6 && h.ID != 12 {
		return false, nil
	}
	k := g.CitiesKnights
	switch h.ID {
	case 6:
		if a.Choice != "progress_buy" || a.Color < 0 || a.Color > 2 || len(k.ProgressDecks[a.Color]) == 0 {
			return true, errors.New("请选择有牌的进步牌堆")
		}
		cost, err := helperSubstituteCost(catanPrices["catan_buy_dev"], a.Tokens)
		if err != nil {
			return true, err
		}
		if !catanHas(g.Players[player].Resources, cost) {
			return true, errors.New("资源不足")
		}
		catanMove(g.Players[player].Resources, g.Bank, cost)
		deck := k.ProgressDecks[a.Color]
		n := min(3, len(deck))
		cards := slices.Clone(deck[len(deck)-n:])
		k.ProgressDecks[a.Color] = deck[:len(deck)-n]
		s.catanLog(player, "通过本站骑士助手支付%s，私下查看%s进步牌堆顶的%d张牌", catanText(cost), catanCityTracks[a.Color], n)
		s.catanHelperAsk(CatanHelperPending{Player: player, Kind: "progress", Resume: "catan_turn", Target: a.Color, Cards: cards})
		return true, nil
	case 12:
		hand := &k.Players[player].Progress
		at := slices.Index(*hand, a.Card)
		if at < 0 {
			return true, errors.New("请选择自己尚未打出的私有进步牌")
		}
		*hand = slices.Delete(*hand, at, at+1)
		k.returnProgress([]int{a.Card})
		k.recordProgress("return", player, -1, -1, 1, nil)
		s.catanDrawProgress(player, catanProgressRules[a.Card].Track)
		s.catanHelperComplete(player, "catan_turn")
		return true, nil
	}
	return false, nil
}
func (s *State) catanCityHelperProgress(player int, a Action) error {
	g := s.Catan
	q := g.HelperPending
	k := g.CitiesKnights
	if !g.cityHelpers() || q.Kind != "progress" || !slices.Contains(q.Cards, a.Card) {
		return errors.New("请选择本次私下展示的进步牌")
	}
	chosen := false
	for _, card := range q.Cards {
		if card == a.Card && !chosen {
			chosen = true
			continue
		}
		k.ProgressDecks[q.Target] = append(k.ProgressDecks[q.Target], card)
	}
	shuffle(k.ProgressDecks[q.Target])
	// Remove the candidate inventory before granting the chosen card and testing
	// victory. All nonchosen cards are back in their original color deck.
	q.Cards = nil
	s.catanGrantProgress(player, a.Card)
	s.catanHelperComplete(player, q.Resume)
	return nil
}
func (s *State) catanQueueCityHelperProduction(received []int) {
	g := s.Catan
	if !g.cityHelpers() {
		return
	}
	for p, n := range received {
		if n == 0 && g.helperReady(p, 3) {
			g.CitiesKnights.Helpers.Production = &catanHelperCityProduction{RollID: g.RollID, Player: p}
			return
		}
	}
}
func (s *State) catanFinishCityHelperProduction() {
	g := s.Catan
	if !g.cityHelpers() || g.CitiesKnights.Pending != nil {
		return
	}
	q := g.CitiesKnights.Helpers.Production
	g.CitiesKnights.Helpers.Production = nil
	if q != nil && !s.Finished && g.helperReady(q.Player, 3) && sum(g.Bank[:5]) > 0 {
		s.catanHelperAsk(CatanHelperPending{Player: q.Player, Kind: "resource", Resume: "catan_turn", Optional: true})
	}
}

func (s *State) validateCityHelpers() error {
	g := s.Catan
	if g == nil || g.CitiesKnights == nil || g.Explorer != nil {
		return nil
	}
	k := g.CitiesKnights
	if !g.Options.Helpers {
		if k.Helpers != nil {
			return errors.New("未启用助手却包含骑士助手规则")
		}
		return nil
	}
	if !g.cityHelpers() || g.Attack != nil || g.Transport != nil || g.Rivers != nil || g.Caravans != nil || len(g.HelperExile) != 0 || len(g.Bank) != 8 {
		return errors.New("骑士助手标记或组合无效")
	}
	if q := k.Helpers.Production; q != nil {
		if s.Finished || q.RollID != g.RollID || q.RollID < 1 || q.Player < 0 || q.Player >= len(g.Players) || !g.helperReady(q.Player, 3) || s.Phase != "catan_aqueduct" || k.Pending == nil || k.Pending.Kind != "aqueduct" || g.HelperPending != nil {
			return errors.New("骑士助手引水渠接续无效")
		}
	}
	if g.HelperPending != nil && (k.Event != nil || k.Pending != nil) {
		return errors.New("骑士助手与城市回应冲突")
	}
	counts := make([]int, len(catanProgressRules))
	add := func(card int) error {
		if card < 0 || card >= len(counts) {
			return errors.New("骑士助手进步牌无效")
		}
		counts[card]++
		return nil
	}
	for track, deck := range k.ProgressDecks {
		for _, card := range deck {
			if err := add(card); err != nil {
				return err
			}
			if catanProgressRules[card].Track != track {
				return errors.New("进步牌堆颜色错误")
			}
		}
	}
	for p, hand := range k.Players {
		if hand.ProgressPoints != len(hand.PublicProgress) || sum(g.Players[p].Dev)+sum(g.Players[p].NewDev) != 0 {
			return errors.New("骑士助手发展牌或公开分数无效")
		}
		for _, card := range hand.Progress {
			if err := add(card); err != nil {
				return err
			}
			if catanProgressRules[card].Victory {
				return errors.New("胜利点进步牌必须公开")
			}
		}
		for _, card := range hand.PublicProgress {
			if err := add(card); err != nil {
				return err
			}
			if !catanProgressRules[card].Victory {
				return errors.New("公开进步牌必须是胜利点")
			}
		}
	}
	if t := g.tribe(); t != nil {
		for _, r := range t.Development {
			if err := add(r.Card); err != nil {
				return err
			}
		}
	}
	if q := g.HelperPending; q != nil && q.Kind == "progress" {
		for _, card := range q.Cards {
			if err := add(card); err != nil {
				return err
			}
		}
	}
	for card, rule := range catanProgressRules {
		if counts[card] != rule.Count {
			return errors.New("骑士助手进步牌库存不守恒")
		}
	}
	return s.validateEventHelpers()
}

// Legal destinations are calculated after removing the selected own knight.
// This also lets the user build on the newly vacated intersection.
func (g *Catan) helperKnightBuilds(player int) map[int]map[string][]int {
	builds := map[int]map[string][]int{}
	for _, knight := range g.CitiesKnights.Knights {
		if knight.Owner != player {
			continue
		}
		temp := *g
		city := *g.CitiesKnights
		temp.CitiesKnights = &city
		city.Knights = slices.DeleteFunc(slices.Clone(city.Knights), func(n CatanKnight) bool { return n.Vertex == knight.Vertex })
		sites := map[string][]int{"settlements": {}, "cities": {}}
		for _, vertex := range temp.Vertices {
			if temp.settlementPiecesLeft(player) > 0 && temp.canSettlement(player, vertex.ID, false) {
				sites["settlements"] = append(sites["settlements"], vertex.ID)
			}
			if temp.canCityUpgrade(player, vertex.ID) {
				sites["cities"] = append(sites["cities"], vertex.ID)
			}
		}
		builds[knight.Vertex] = sites
	}
	return builds
}
