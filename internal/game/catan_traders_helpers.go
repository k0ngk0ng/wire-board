package game

import (
	"errors"
	"slices"
)

const CatanTradersHelpersRules = "wire-board-traders-helpers-v1"

// Continuations survive the helper's flip/exchange and are never public.
type CatanTradersHelpers struct {
	Rules        string `json:"rules"`
	AttackCard   string `json:"attackCard,omitempty"`
	BuildLanding bool   `json:"buildLanding,omitempty"`
}

func (g *Catan) tradersHelpers() bool {
	return g != nil && g.Explorer == nil && (g.Rivers != nil || g.Caravans != nil || g.Attack != nil || g.Transport != nil) && g.Options.Helpers && g.TradersHelpers != nil && g.TradersHelpers.Rules == CatanTradersHelpersRules
}

func (s *State) EnableCatanTradersHelpers(all bool) error {
	g := s.Catan
	if g == nil || g.Explorer != nil || g.Rivers == nil && g.Caravans == nil && g.Attack == nil && g.Transport == nil || g.SetupStep != 0 || g.RollID != 0 || g.TradersHelpers != nil || g.Options.Helpers {
		return errors.New("商人与蛮族助手只能在新游戏开局前启用")
	}
	next := clone(*s)
	g = next.Catan
	g.TradersHelpers = &CatanTradersHelpers{Rules: CatanTradersHelpersRules}
	if g.caravansSea() {
		g.Caravans.Helpers = CatanCaravansHelpersRules
	}
	g.Options.Helpers, g.Options.AllHelpers = true, all
	g.Options, _ = NormalizeCatanOptions(g.Options)
	if g.Two != nil {
		g.Two.Helpers = CatanTwoHelpersRules
	}
	if g.CitiesKnights != nil {
		g.CitiesKnights.Helpers = &catanHelpersKnights{Rules: CatanHelpersKnightsRules}
	}
	if g.Fishing != nil {
		g.Fishing.Helpers = CatanFishingHelpersRules
	}
	next.initCatanHelpers()
	next.Log = append(next.Log, "本站商人与蛮族 Helpers：普通交易、筑路和生产补偿保留；无强盗时迪古尔领1金币、卡娅领1普通资源；蛮族牌私选后先翻面／交换，再公开执行；格雷戈尔归还己方实体骑士参与建设，卡拉在无手持发展牌时换一张普通资源")
	if g.Two != nil {
		next.Log = append(next.Log, catanTwoHelpersNotice)
	}
	if g.Transport != nil {
		if err := next.validateCatanTransport(); err != nil {
			return err
		}
	} else if g.Attack != nil {
		if err := next.validateCatanAttack(); err != nil {
			return err
		}
	}
	if err := g.validateRivers(); err != nil {
		return err
	}
	if err := next.validateCaravans(); err != nil {
		return err
	}
	if err := next.validateCatanTwo(); err != nil {
		return err
	}
	if err := next.validateTradersHelpers(); err != nil {
		return err
	}
	*s = next
	return nil
}

func (g *Catan) traderHelperKnights(player int) []int {
	out := []int{}
	if !g.tradersHelpers() || g.Attack == nil {
		return out
	}
	if g.attackKnights() {
		for _, k := range g.Attack.City.Knights {
			if k.Owner == player {
				out = append(out, k.Edge)
			}
		}
	} else {
		for _, k := range g.Attack.Knights {
			if k.Player == player {
				out = append(out, k.Edge)
			}
		}
	}
	return out
}

func (g *Catan) traderHelperKnightBuilds(player int) map[int]map[string][]int {
	out := map[int]map[string][]int{}
	for _, id := range g.traderHelperKnights(player) {
		sites := map[string][]int{"settlements": {}, "cities": {}}
		for _, v := range g.Vertices {
			if g.settlementPiecesLeft(player) > 0 && g.canSettlement(player, v.ID, false) {
				sites["settlements"] = append(sites["settlements"], v.ID)
			}
			if g.canCityUpgrade(player, v.ID) {
				sites["cities"] = append(sites["cities"], v.ID)
			}
		}
		out[id] = sites
	}
	return out
}

func (s *State) catanTradersSpendKnight(player int, a Action) error {
	g := s.Catan
	if !g.tradersHelpers() || !g.helperReady(player, 8) || s.Phase != "catan_turn" || !slices.Contains(g.traderHelperKnights(player), a.Target) {
		return errors.New("请选择要归还的己方实体骑士")
	}
	if g.attackKnights() {
		g.Attack.City.Knights = slices.DeleteFunc(g.Attack.City.Knights, func(k catanAttackCityKnight) bool { return k.Edge == a.Target && k.Owner == player })
	} else {
		g.Attack.Knights = slices.DeleteFunc(g.Attack.Knights, func(k catanAttackKnight) bool { return k.Edge == a.Target && k.Player == player })
	}
	s.catanLog(player, "通过本站助手归还路线 #%d 的己方骑士参与建设", a.Target+1)
	return nil
}

func (s *State) validateTradersHelpers() error {
	g := s.Catan
	if g.TradersHelpers == nil {
		if !g.Options.Helpers && s.Phase == "catan_helper" {
			return errors.New("未启用助手不能进入助手回应")
		}
		if g.Options.Helpers && (g.Rivers != nil || g.Attack != nil || g.Transport != nil || g.Caravans != nil && !g.caravanSeaHelpers()) {
			return errors.New("商人与蛮族助手缺少组合规则版本")
		}
		return nil
	}
	if !g.tradersHelpers() || s.Turn < 0 || s.Turn >= len(g.Players) {
		return errors.New("商人与蛮族助手版本或组件无效")
	}
	q := g.TradersHelpers
	if q.AttackCard != "" {
		if g.Attack == nil || g.attackKnights() || !slices.Contains(catanTradersAttackCards(), q.AttackCard) || g.HelperPending == nil || g.HelperPending.Kind != "exchange" || g.HelperPending.Player != s.Turn || g.Players[s.Turn].Helper == nil || g.Players[s.Turn].Helper.ID != 6 || g.Attack.Pending != nil || q.BuildLanding {
			return errors.New("助手发展牌接续无效")
		}
	}
	if q.BuildLanding && (g.Attack == nil || g.HelperPending == nil || g.HelperPending.Kind != "exchange" || g.HelperPending.Player != s.Turn || g.Players[s.Turn].Helper == nil || g.Players[s.Turn].Helper.ID != 8) {
		return errors.New("助手建设登陆接续无效")
	}
	return s.validateEventHelpers()
}

func catanTradersAttackCards() []string {
	return []string{"capture", "knighthood", "swift_knight", "treason"}
}

func (g *Catan) traderHelperDescriptions(rules []CatanHelper) {
	if !g.tradersHelpers() {
		return
	}
	if g.Caravans != nil && g.Rivers == nil {
		rules[9].Description = "本站商队适配：将强盗赶回沙漠；无沙漠时赶至场外。原地为水源或金矿时任选一张普通资源。"
		rules[10].Description = "领取强盗所在生产地对应的普通资源；原地为水源、沙漠或金矿时任选资源。"
	}
	if g.Rivers != nil {
		rules[3].Description = "本站河流适配：移动己方末端道路，不能移动桥梁；移走河岸道路先退还1金币，新位置在河岸时再正常领取1金币。"
		rules[9].Description = "本站河流规则：将强盗赶回沙漠；无沙漠时赶至场外。原地为沼泽、水源或金矿时任选一张普通资源。"
		rules[10].Description = "领取强盗所在生产地对应的普通资源；原地为沼泽、水源、沙漠或金矿时任选资源。"
	}
	if g.Attack != nil || g.Transport != nil {
		rules[9].Title = "军事补给（本站）"
		rules[9].Description = "本站无强盗适配：自己生产前或行动阶段领取1金币；不改变蛮族、舰队或战斗状态。"
		rules[10].Title = "资源补给（本站）"
		rules[10].Description = "本站无强盗适配：行动阶段从银行领取一张自选普通资源。"
	}
	if g.Attack != nil && !g.attackKnights() {
		rules[5].Title = "择选蛮族牌（本站）"
		rules[5].Description = "支付羊毛、粮食、矿石各1，可替换其中一张；私下查看牌堆顶至多三张，选一张。其余洗回；先翻面或交换助手，再公开执行选中的蛮族专用牌。"
		rules[7].Description = "本站蛮族适配：归还一名自己的实体骑士，木砖各1建村或粮1矿2升城；不能归还中立骑士，照常触发登陆及中立建设。"
		rules[11].Title = "更换补给（本站）"
		rules[11].Description = "本站蛮族适配：将一张普通资源还给银行，换另一种有库存的普通资源；不持有或交换已经立即执行的蛮族牌。"
	}
	if g.attackKnights() {
		rules[7].Description = "本站道路骑士适配：归还一名自己的道路骑士，木砖各1建村或粮1矿2升城；不返还中立骑士，不领贸易筹码，照常触发登陆。"
	}
}

func (s *State) catanTradersHelperAction(player int, a Action) (bool, error) {
	g := s.Catan
	h := g.Players[player].Helper
	if !g.tradersHelpers() || h == nil {
		return false, nil
	}
	if (g.Attack != nil || g.Transport != nil) && (h.ID == 10 || h.ID == 11) {
		if h.ID == 10 {
			switch {
			case g.Transport != nil:
				if err := g.Transport.ensureGold(1); err != nil {
					return true, err
				}
				g.Transport.GoldBank--
				g.Transport.Gold[player]++
			case g.Attack != nil:
				if err := g.Attack.ensureGold(1); err != nil {
					return true, err
				}
				g.Attack.GoldBank--
				g.Attack.Gold[player]++
			}
			s.catanLog(player, "通过本站补给助手领取金币×1")
		} else {
			if a.Color < 0 || a.Color >= 5 || g.Bank[a.Color] == 0 {
				return true, errors.New("请选择银行有库存的普通资源")
			}
			g.Bank[a.Color]--
			g.Players[player].Resources[a.Color]++
		}
		s.catanHelperComplete(player, s.Phase)
		return true, nil
	}
	if g.Attack != nil && !g.attackKnights() && h.ID == 12 {
		if a.Card < 0 || a.Card >= 5 || a.Color < 0 || a.Color >= 5 || a.Card == a.Color || g.Players[player].Resources[a.Card] == 0 || g.Bank[a.Color] == 0 {
			return true, errors.New("请归还一张普通资源并选择另一种有库存的资源")
		}
		g.Players[player].Resources[a.Card]--
		g.Bank[a.Card]++
		g.Players[player].Resources[a.Color]++
		g.Bank[a.Color]--
		s.catanHelperComplete(player, s.Phase)
		return true, nil
	}
	return false, nil
}

func (s *State) catanTradersHelperDevelopment(player int) error {
	g := s.Catan
	a := g.Attack
	if len(a.Deck) == 0 {
		a.Deck, a.Discard = a.Discard, []string{}
		shuffle(a.Deck)
	}
	n := min(3, len(a.Deck))
	cards := []int{}
	for _, card := range a.Deck[len(a.Deck)-n:] {
		cards = append(cards, slices.Index(catanTradersAttackCards(), card))
	}
	a.Deck = a.Deck[:len(a.Deck)-n]
	s.catanHelperAsk(CatanHelperPending{Player: player, Kind: "attack_development", Resume: "catan_turn", Cards: cards})
	return nil
}

func (s *State) catanTradersHelperCardChoice(player int, a Action) error {
	g := s.Catan
	q := g.HelperPending
	at := slices.Index(q.Cards, a.Card)
	if !g.tradersHelpers() || g.Attack == nil || at < 0 || a.Card < 0 || a.Card >= 4 {
		return errors.New("请选择本次私下展示的蛮族牌")
	}
	for i, card := range q.Cards {
		if i != at {
			g.Attack.Deck = append(g.Attack.Deck, catanTradersAttackCards()[card])
		}
	}
	shuffle(g.Attack.Deck)
	g.TradersHelpers.AttackCard = catanTradersAttackCards()[a.Card]
	s.catanHelperComplete(player, q.Resume)
	return nil
}

func (s *State) catanTradersAfterHelper() error {
	g := s.Catan
	if !g.tradersHelpers() {
		return nil
	}
	q := g.TradersHelpers
	if q.AttackCard != "" {
		card := q.AttackCard
		q.AttackCard = ""
		g.Attack.Deck = append(g.Attack.Deck, card)
		return s.catanAttackDrawCard(s.Turn)
	}
	if q.BuildLanding {
		q.BuildLanding = false
		return s.catanAttackLanding(func() [2]int { return [2]int{catanRandom(6) + 1, catanRandom(6) + 1} }, catanRandom)
	}
	return nil
}
