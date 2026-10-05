package game

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
)

type CatanPlayer struct {
	Helper     *CatanHelperSeat `json:"helper,omitempty"`
	Resources  []int            `json:"resources"`
	Dev        []int            `json:"dev"`
	NewDev     []int            `json:"newDev"`
	Knights    int              `json:"knights"`
	RoadLength int              `json:"roadLength"`
	Score      int              `json:"score"`
	Eliminated bool             `json:"eliminated,omitempty"`
}
type CatanTrade struct {
	ID        int   `json:"id"`
	From      int   `json:"from"`
	Give      []int `json:"give"`
	Take      []int `json:"take"`
	Responses []int `json:"responses"` // 0 waiting, 1 accepted, -1 declined.
}
type Catan struct {
	RevealedEvent  *CatanRevealedEvent  `json:"revealedEvent,omitempty"`
	CardEvent      *CatanCardEvent      `json:"cardEvent,omitempty"`
	FriendlyRobber *CatanFriendlyRobber `json:"friendlyRobber,omitempty"`
	Harbors        *CatanHarbors        `json:"harbors,omitempty"`
	CitiesKnights  *CatanCitiesKnights  `json:"citiesKnights,omitempty"`
	BaseSetup      *CatanBaseSetup      `json:"baseSetup,omitempty"`
	GoldPending    *CatanGoldPending    `json:"goldPending,omitempty"`
	Seafarers      *CatanSeafarers      `json:"seafarers,omitempty"`
	StartPlayer    int                  `json:"startPlayer,omitempty"`
	Paired         *CatanPairedTurn     `json:"paired,omitempty"`
	HexSize        float64              `json:"hexSize,omitempty"`
	Options        CatanOptions         `json:"options"`
	TurnSerial     uint64               `json:"turnSerial,omitempty"`
	HelperDisplay  []int                `json:"helperDisplay,omitempty"`
	HelperPending  *CatanHelperPending  `json:"helperPending,omitempty"`
	HelperSequence uint64               `json:"helperSequence,omitempty"`
	HelperExile    []int                `json:"helperExile,omitempty"`
	Tiles          []CatanTile          `json:"tiles"`
	Vertices       []CatanVertex        `json:"vertices"`
	Edges          []CatanEdge          `json:"edges"`
	Ports          []CatanPort          `json:"ports"`
	Players        []CatanPlayer        `json:"players"`
	Bank           []int                `json:"bank"`
	DevDeck        []int                `json:"devDeck"`
	DevDiscard     []int                `json:"devDiscard"`
	Robber         int                  `json:"robber"`
	Dice           []int                `json:"dice"`
	RollID         int                  `json:"rollId"`
	SetupStep      int                  `json:"setupStep"`
	SetupVertex    int                  `json:"setupVertex"`
	DiscardDue     []int                `json:"discardDue"`
	Victims        []int                `json:"victims"`
	ResumePhase    string               `json:"resumePhase"`
	FreeRoads      int                  `json:"freeRoads"`
	PlayedDev      bool                 `json:"playedDev"`
	LongestOwner   int                  `json:"longestOwner"`
	ArmyOwner      int                  `json:"armyOwner"`
	TradeID        int                  `json:"tradeId"`
	Trade          *CatanTrade          `json:"trade,omitempty"`
}

func catanRandom(n int) int {
	v, e := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if e != nil {
		panic(e)
	}
	return int(v.Int64())
}
func (s *State) initCatan(n int) {
	g := &Catan{Bank: []int{19, 19, 19, 19, 19}, Robber: -1, SetupVertex: -1, LongestOwner: -1, ArmyOwner: -1, Dice: []int{0, 0}, DiscardDue: make([]int, n), Victims: []int{}, DevDiscard: []int{}}
	for range n {
		g.Players = append(g.Players, CatanPlayer{Resources: make([]int, 5), Dev: make([]int, 5), NewDev: make([]int, 5)})
	}
	counts := []int{14, 2, 2, 2, 5}
	if n > 4 {
		g.Options = CatanOptions{Rules: CatanExpansionRules, FiveSix: true}
		g.StartPlayer = catanRandom(n)
		s.Turn = g.StartPlayer
		g.Paired = &CatanPairedTurn{Primary: g.StartPlayer, Secondary: (g.StartPlayer + 3) % n}
		g.Bank = []int{24, 24, 24, 24, 24}
		counts = []int{20, 3, 3, 3, 5}
	}
	for kind, count := range counts {
		for range count {
			g.DevDeck = append(g.DevDeck, kind)
		}
	}
	shuffle(g.DevDeck)
	g.makeMap()
	s.Catan = g
	s.Phase = "catan_setup_settlement"
	s.Log = append(s.Log, "卡坦岛开局：按顺序放置村庄和道路，再逆序放置第二组")
	if n > 4 {
		s.Log = append(s.Log, "五至六人扩充：30块陆地，采用新版配对回合；配对玩家不能与其他玩家自由交易")
	}
}
func (g *Catan) SetupLimit() int {
	if g.cloth() != nil {
		return 3 * len(g.Players)
	}
	return 2 * len(g.Players)
}
func (g *Catan) setup() bool { return g.SetupStep < g.SetupLimit() }
func catanBundle(a []int) bool {
	if len(a) != 5 {
		return false
	}
	for _, n := range a {
		if n < 0 || n > 24 {
			return false
		}
	}
	return true
}
func catanHas(hand, cost []int) bool {
	if (len(hand) != 5 && len(hand) != 8) || (len(cost) != 5 && len(cost) != 8) || len(cost) > len(hand) {
		return false
	}
	for i, n := range cost {
		if n < 0 || hand[i] < n {
			return false
		}
	}
	return true
}
func catanMove(from, to, amount []int) {
	for i, n := range amount {
		from[i] -= n
		to[i] += n
	}
}
func catanText(a []int) string {
	out := []string{}
	for i, n := range a {
		if n > 0 {
			out = append(out, fmt.Sprintf("%s×%d", catanCardName(i), n))
		}
	}
	return strings.Join(out, "、")
}
func (s *State) catanLog(p int, format string, args ...any) {
	s.Log = append(s.Log, fmt.Sprintf("玩家 %d ", p+1)+fmt.Sprintf(format, args...))
}
func (g *Catan) awardHolder(old, minValue int, values []int) int {
	maximum := minValue - 1
	ties := []int{}
	for i, n := range values {
		if g.Players[i].Eliminated {
			continue
		}
		if n > maximum {
			maximum = n
			ties = []int{i}
		} else if n == maximum {
			ties = append(ties, i)
		}
	}
	if maximum < minValue {
		return -1
	}
	for _, i := range ties {
		if i == old {
			return old
		}
	}
	if len(ties) == 1 {
		return ties[0]
	}
	return -1
}
func (s *State) catanScores() {
	g := s.Catan
	s.catanHarborsScore()
	roads, knights := []int{}, []int{}
	for i := range g.Players {
		g.Players[i].RoadLength = g.roadLength(i)
		roads = append(roads, g.Players[i].RoadLength)
		knights = append(knights, g.Players[i].Knights)
	}
	routeName := "最长道路"
	if g.Seafarers != nil {
		routeName = "最长路线"
	}
	oldRoad, oldArmy := g.LongestOwner, g.ArmyOwner
	g.LongestOwner = g.awardHolder(oldRoad, 5, roads)
	if g.cloth() != nil || g.pirateIslands() != nil {
		g.LongestOwner = -1
	}
	g.ArmyOwner = g.awardHolder(oldArmy, 3, knights)
	if g.pirateIslands() != nil || g.CitiesKnights != nil {
		g.ArmyOwner = -1
	}
	if g.LongestOwner != oldRoad {
		if g.LongestOwner < 0 {
			s.Log = append(s.Log, routeName+"奖励暂时无人持有")
		} else {
			s.catanLog(g.LongestOwner, "获得%s（%d 段），奖励 2 分", routeName, roads[g.LongestOwner])
		}
	}
	if g.ArmyOwner != oldArmy {
		if g.ArmyOwner < 0 {
			s.Log = append(s.Log, "最大骑士军队奖励暂时无人持有")
		} else {
			s.catanLog(g.ArmyOwner, "获得最大骑士军队，奖励 2 分")
		}
	}
	for i := range g.Players {
		p := &g.Players[i]
		p.Score = g.hiddenVictoryPoints(i)
		if g.Harbors != nil && g.Harbors.Owner == i {
			p.Score += 2
		}
		if k := g.CitiesKnights; k != nil {
			p.Score += k.Players[i].DefenderPoints + k.Players[i].ProgressPoints
			if k.Merchant != nil && k.Merchant.Owner == i {
				p.Score++
			}
			for track := range 3 {
				if g.cityMetropolisOwner(track) == i {
					p.Score += 2
				}
			}
		}
		if c := g.cloth(); c != nil {
			p.Score += c.Held[i] / 2
		}
		if t := g.tribe(); t != nil && i < len(t.Points) {
			p.Score += t.Points[i]
		}
		if g.Seafarers != nil && i < len(g.Seafarers.Seats) {
			p.Score += g.Seafarers.Seats[i].IslandPoints
		}
		for _, v := range g.Vertices {
			if v.Owner == i {
				p.Score += v.Level
			}
		}
		if g.LongestOwner == i {
			p.Score += 2
		}
		if g.ArmyOwner == i {
			p.Score += 2
		}
	}
}
func (g *Catan) hiddenVictoryPoints(player int) int {
	if g.pirateIslands() != nil || g.CitiesKnights != nil {
		return 0
	}
	return g.Players[player].Dev[4]
}
func (s *State) catanVictory() {
	g := s.Catan
	if g.wonders() != nil {
		if !g.setup() && !g.Players[s.Turn].Eliminated && g.wonderVictory(s.Turn) {
			s.Finished, s.Phase, s.Winners = true, "finished", []int{s.Turn}
			g.Trade = nil
			card := g.wonders().Cards[g.wonderOwned(s.Turn)]
			s.catanLog(s.Turn, "凭借第 %d 级奇迹「%s」和 %d 分，赢得本局", card.Level, catanWonderRules[card.ID].Name, g.Players[s.Turn].Score)
		}
		return
	}
	if p := g.pirateIslands(); p != nil && (s.Turn >= len(p.Fortresses) || p.Fortresses[s.Turn].Strength > 0) {
		return
	}
	goal := g.victoryTarget()
	if !g.setup() && !g.Players[s.Turn].Eliminated && g.Players[s.Turn].Score >= goal {
		s.Finished = true
		s.Phase = "finished"
		s.Winners = []int{s.Turn}
		g.Trade = nil
		s.catanLog(s.Turn, "在自己的回合达到 %d 分，赢得本局", g.Players[s.Turn].Score)
	}
}
func (s *State) catanNext() {
	g := s.Catan
	if g.CitiesKnights != nil {
		g.CitiesKnights.ActionSerial++
		g.CitiesKnights.TradePowers = nil
	}
	g.Trade = nil
	g.PlayedDev = false
	g.FreeRoads = 0
	if g.Seafarers != nil {
		g.Seafarers.BuiltShips = nil
		g.Seafarers.MovedShip = false
	}
	g.Victims = []int{}
	g.ResumePhase = ""
	if g.Paired != nil {
		s.catanNextPaired()
		return
	}
	g.TurnSerial++
	for {
		s.Turn = (s.Turn + 1) % len(g.Players)
		if s.Turn == g.StartPlayer {
			s.Round++
		}
		if !g.Players[s.Turn].Eliminated {
			break
		}
	}
	g.Players[s.Turn].NewDev = make([]int, 5)
	s.Phase = "catan_roll"
}
func (s *State) applyCatan(player int, a Action) error {
	if s.Catan.CardEvent != nil || s.Catan.Options.Helpers || s.Catan.Options.FiveSix || s.Catan.Seafarers != nil || s.Catan.CitiesKnights != nil || s.Catan.Harbors != nil || s.Catan.FriendlyRobber != nil {
		next := clone(*s)
		if err := next.applyCatanStep(player, a); err != nil {
			return err
		}
		if err := next.catanPirateSeven(); err != nil {
			return err
		}
		*s = next
		return nil
	}
	return s.applyCatanStep(player, a)
}
func (s *State) applyCatanStep(player int, a Action) error {
	g := s.Catan
	if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated {
		return errors.New("无法操作此座位")
	}
	if g.CardEvent != nil {
		return s.catanCardEventChoice(player, a)
	}
	if k := g.CitiesKnights; k != nil && k.Pending != nil {
		return s.catanCityChoice(player, a)
	}
	if p := g.pirateIslands(); p != nil && p.Raid != nil {
		return s.catanFleetReward(player, a)
	}
	if t := g.tribe(); t != nil && t.Pending != nil {
		return s.catanPlaceTribePort(player, a)
	}
	if g.HelperPending != nil {
		return s.catanHelperRespond(player, a)
	}
	if g.GoldPending != nil {
		return s.catanChooseGold(player, a)
	}
	if a.Type == "catan_discard" {
		return s.catanDiscard(player, a.Tokens)
	}
	if a.Type == "catan_trade_accept" || a.Type == "catan_trade_reject" {
		return s.catanRespondTrade(player, a)
	}
	if player != s.Turn {
		return errors.New("还没有轮到你")
	}
	p := &g.Players[player]
	if g.setup() {
		if s.Phase == "catan_world_ports" {
			return s.catanWorldPort(player, a)
		}
		if s.Phase == "catan_wonders_start" {
			return s.catanWondersStart(player, a)
		}
		if s.Phase == "catan_cloth_start" {
			return s.catanClothStart(player, a)
		}
		return s.catanSetup(a)
	}
	if a.Type == "catan_helper" {
		return s.catanHelperAction(player, a)
	}
	switch a.Type {
	case "catan_commercial_offer":
		return s.catanCommercialOffer(player, a)
	case "catan_progress":
		return s.catanPlayProgress(player, a)
	case "catan_knight_recruit", "catan_knight_activate", "catan_knight_promote", "catan_knight_move", "catan_knight_chase":
		return s.catanKnightAction(player, a)
	case "catan_wall", "catan_improvement":
		return s.catanCityAction(player, a)
	case "catan_wonder_claim", "catan_wonder_build":
		return s.catanWonderAction(player, a)
	case "catan_roll":
		if g.CitiesKnights != nil {
			return s.catanCityRoll(catanRandom(6)+1, catanRandom(6)+1, catanRandom(6))
		}
		if s.Phase != "catan_roll" {
			return errors.New("当前不能掷骰")
		}
		g.RevealedEvent = nil
		g.Dice = []int{catanRandom(6) + 1, catanRandom(6) + 1}
		g.RollID++
		return s.catanRoll(sum(g.Dice))
	case "catan_end":
		if s.Phase != "catan_turn" {
			return errors.New("请先完成当前行动")
		}
		s.catanVictory()
		if k := g.CitiesKnights; k != nil && !s.Finished && len(k.Players[player].Progress) > 4 {
			k.Pending = &CatanCityPending{Kind: "progress_discard", Players: []int{player}}
			s.Phase = "catan_progress_end"
			g.Trade = nil
			return nil
		}
		if !s.Finished && g.pirateFortressReady(player) {
			s.catanAttackFortress(player, catanRandom(6)+1)
		}
		if !s.Finished && !s.catanClothEnd() {
			s.catanNext()
			s.catanVictory()
		}
	case "catan_road", "catan_ship", "catan_settlement", "catan_city":
		return s.catanBuild(player, a, false)
	case "catan_repair_road":
		return s.catanRepairRoad(player, a)
	case "catan_skip_roads":
		if s.Phase != "catan_roads" || g.hasFreeRouteAction(player) {
			return errors.New("仍有可建造或修复的免费路线")
		}
		g.FreeRoads = 0
		s.Phase = g.ResumePhase
	case "catan_bank":
		if s.Phase != "catan_turn" {
			return errors.New("掷骰后才可交易")
		}
		if !g.cardBundle(a.Give) || !g.cardBundle(a.Take) || !catanHas(p.Resources, a.Give) || !catanHas(g.Bank, a.Take) || sum(a.Take) == 0 {
			return errors.New("交易数量或库存不符合要求")
		}
		units := 0
		rates := g.rates(player)
		for i, n := range a.Give {
			if n > 0 && a.Take[i] > 0 || n%rates[i] != 0 {
				return errors.New("请按港口比例交换不同资源")
			}
			units += n / rates[i]
		}
		if units != sum(a.Take) {
			return errors.New("交换数量不符合港口比例")
		}
		catanMove(p.Resources, g.Bank, a.Give)
		catanMove(g.Bank, p.Resources, a.Take)
		g.Trade = nil
		s.catanLog(player, "向银行支付 %s，换得 %s", catanText(a.Give), catanText(a.Take))
	case "catan_trade_offer":
		return s.catanOffer(player, a)
	case "catan_trade_cancel":
		if s.Phase != "catan_turn" {
			return errors.New("当前不能交易")
		}
		g.Trade = nil
	case "catan_trade_complete":
		return s.catanCompleteTrade(player, a)
	case "catan_buy_dev":
		if s.Phase != "catan_turn" || len(g.DevDeck) == 0 {
			return errors.New("当前不能购买发展卡")
		}
		cost := catanPrices[a.Type]
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
		if a.Skill == "helper" {
			return s.catanHelperDevelopment(player)
		}
		card := g.DevDeck[len(g.DevDeck)-1]
		g.DevDeck = g.DevDeck[:len(g.DevDeck)-1]
		p.Dev[card]++
		p.NewDev[card]++
		g.Trade = nil
		s.catanLog(player, "支付 羊毛×1、粮食×1、矿石×1，购买一张发展卡")
		s.catanScores()
		s.catanVictory()
	case "catan_dev":
		if g.CitiesKnights != nil {
			return errors.New("城市与骑士不使用基础发展卡")
		}
		return s.catanDev(player, a)
	case "catan_move_ship":
		return s.catanMoveShip(player, a)
	case "catan_pirate":
		return s.catanMovePirate(player, a.Tile)
	case "catan_robber":
		return s.catanMoveRobber(player, a.Tile)
	case "catan_steal":
		return s.catanSteal(player, a.Target)
	case "catan_skip_steal":
		if g.pirateIslands() == nil || s.Phase != "catan_steal" {
			return errors.New("当前不能放弃偷取")
		}
		g.Victims = []int{}
		s.Phase = g.ResumePhase
		s.catanLog(player, "放弃本次7点偷取资源")
	case "catan_cloth_steal":
		return s.catanClothSteal(player, a)
	default:
		return errors.New("未知卡坦岛行动")
	}
	return nil
}
func (g *Catan) hasRoad(p int) bool {
	n, _, _ := g.pieces(p)
	if n >= 15 {
		return false
	}
	for _, e := range g.Edges {
		if g.canRoad(p, e.ID) {
			return true
		}
	}
	return false
}
func (s *State) catanSetup(a Action) error {
	g := s.Catan
	p := s.Turn
	if s.Phase == "catan_setup_settlement" || s.Phase == "catan_setup_city" {
		want := "catan_settlement"
		if s.Phase == "catan_setup_city" {
			want = "catan_city"
		}
		if a.Type != want || !g.canSettlement(p, a.Vertex, true) {
			return errors.New("请选择与其他建筑至少相隔两条边的空交点")
		}
		g.Vertices[a.Vertex].Owner = p
		g.Vertices[a.Vertex].Level = 1
		if want == "catan_city" {
			g.Vertices[a.Vertex].Level = 2
		}
		g.SetupVertex = a.Vertex
		s.catanSettleIsland(p, a.Vertex, true)
		s.Phase = "catan_setup_road"
		if want == "catan_city" {
			s.catanLog(p, "放置起始城市 #%d", a.Vertex+1)
		} else {
			s.catanLog(p, "放置起始村庄 #%d", a.Vertex+1)
		}
		if g.SetupStep >= g.SetupLimit()-len(g.Players) {
			gain := make([]int, 5)
			gold := make([]int, len(g.Players))
			for _, t := range g.Tiles {
				if t.Resource >= 5 && t.Resource != CatanGold {
					continue
				}
				for _, v := range t.Vertices {
					if v == a.Vertex {
						if t.Resource == CatanGold {
							gold[p]++
						} else {
							gain[t.Resource]++
						}
					}
				}
			}
			catanMove(g.Bank, g.Players[p].Resources, gain)
			if want == "catan_city" {
				s.catanLog(p, "从起始城市获得普通资源 %s", catanText(gain))
			} else {
				s.catanLog(p, "从第 %d 座起始村庄获得 %s", g.SetupLimit()/len(g.Players), catanText(gain))
			}
			if gold[p] > 0 {
				s.catanStartGold(gold, nil, "catan_setup_road")
			}
		}
		s.catanScores()
		return nil
	}
	if (a.Type != "catan_road" && a.Type != "catan_ship") || !g.setupRoute(p, a.Edge, a.Type == "catan_ship") {
		return errors.New("请在刚放置的村庄旁修建道路")
	}
	e := &g.Edges[a.Edge]
	if e.Owner >= 0 || (e.A != g.SetupVertex && e.B != g.SetupVertex) {
		return errors.New("道路必须紧邻刚放置的村庄")
	}
	e.Owner = p
	e.Ship = a.Type == "catan_ship"
	if e.Ship {
		s.catanLog(p, "放置起始船只 #%d", a.Edge+1)
	} else {
		s.catanLog(p, "放置起始道路 #%d", a.Edge+1)
	}
	return s.catanAfterRoute(CatanRouteCompletion{Player: p, Edge: a.Edge, Setup: true})
}
func (s *State) catanFinishSetupRoute(p, edge int) {
	g := s.Catan
	if g.Options.Helpers && g.SetupStep >= len(g.Players) && g.SetupStep < 2*len(g.Players) {
		// The second setup pass runs backwards. The descending starting stack
		// therefore gives seat i helper i+1 (helper N is picked first).
		helper := (p-g.StartPlayer+len(g.Players))%len(g.Players) + 1
		g.Players[p].Helper = &CatanHelperSeat{ID: helper}
		s.catanLog(p, "获得起始助手「%s」", CatanHelpers()[helper-1].Name)
	}
	g.SetupStep++
	g.SetupVertex = -1
	if !g.setup() {
		g.TurnSerial = 1
		s.Turn = g.StartPlayer
		s.Phase = "catan_roll"
		s.catanLog(s.Turn, "起始建设完成，开始掷骰")
	} else {
		n := len(g.Players)
		if g.SetupStep < n || g.SetupStep >= 2*n {
			s.Turn = (g.StartPlayer + g.SetupStep) % n
		} else {
			s.Turn = (g.StartPlayer + 2*n - 1 - g.SetupStep) % n
		}
		s.Phase = "catan_setup_settlement"
		if g.CitiesKnights != nil && g.SetupStep >= n && g.SetupStep < 2*n {
			s.Phase = "catan_setup_city"
		}
	}
}
func (s *State) catanRoll(total int) error {
	g := s.Catan
	s.catanLog(s.Turn, "掷出 %d + %d = %d", g.Dice[0], g.Dice[1], total)
	return s.catanResolveProductionNumber(total)
}
func (s *State) catanResolveProductionNumber(total int) error {
	if pending, err := s.catanRaidFleet(total); err != nil || pending {
		return err
	}
	return s.catanRollProduction(total)
}
func (s *State) catanRollProduction(total int) error {
	return s.catanRollProductionEffect(total, false)
}

// Epidemic modifies only this production's claims. Pending gold choices keep
// their already computed counts, so no turn-wide flag can leak into a later roll.
func (s *State) catanRollProductionEffect(total int, epidemic bool) error {
	g := s.Catan
	if q := g.RevealedEvent; q != nil && q.RollID == g.RollID {
		q.ProductionStarted = true
	}
	if total == 7 {
		if p := g.pirateIslands(); p != nil {
			p.SevenPending = true
		}
		g.ResumePhase = "catan_turn"
		pending := false
		protected := -1
		if g.Options.Helpers {
			for i := range g.Players {
				if g.helperReady(i, 5) {
					protected = i
					break
				}
			}
		}
		for i, p := range g.Players {
			g.DiscardDue[i] = 0
			if !p.Eliminated && i != protected && sum(p.Resources) > g.catanDiscardLimit(i) {
				g.DiscardDue[i] = sum(p.Resources) / 2
				pending = true
			}
		}
		if pending {
			s.Phase = "catan_discard"
			if g.CitiesKnights != nil {
				s.Log = append(s.Log, "掷出7：资源与商品合计超过个人城墙上限的玩家同时弃一半，120秒后自动弃牌")
			} else {
				s.Log = append(s.Log, "掷出 7：超过 7 张资源的玩家同时弃掉一半（向下取整），120 秒后自动弃牌")
			}
		} else {
			s.catanAfterSevenDiscards()
		}
		if protected >= 0 {
			resume := s.Phase
			if sum(g.Players[protected].Resources) > 7 {
				s.catanLog(protected, "助手托罗夫保护手牌，无需因掷出 7 弃牌")
				s.catanHelperComplete(protected, resume)
			} else if sum(g.Bank) > 0 {
				s.catanHelperAsk(CatanHelperPending{Player: protected, Kind: "resource", Resume: resume})
			} else {
				s.catanHelperComplete(protected, resume)
			}
		}
		return nil
	}
	if err := s.catanProduceCloth(total); err != nil {
		return err
	}
	gold := make([]int, len(g.Players))
	claims := make([][]int, len(g.Players))
	for i := range claims {
		claims[i] = make([]int, len(g.Bank))
	}
	for _, t := range g.Tiles {
		if t.Number != total || t.ID == g.Robber || (t.Resource >= 5 && t.Resource != CatanGold) {
			continue
		}
		for _, id := range t.Vertices {
			v := g.Vertices[id]
			if v.Level > 0 && v.Owner >= 0 && !g.Players[v.Owner].Eliminated {
				level := v.Level
				if epidemic && level == 2 {
					level = 1
				}
				if t.Resource == CatanGold {
					gold[v.Owner] += level
				} else {
					g.cityProduction(claims[v.Owner], t.Resource, level)
				}
			}
		}
	}
	for c := range g.Bank {
		demand, people, only := 0, 0, -1
		for i := range claims {
			demand += claims[i][c]
			if claims[i][c] > 0 {
				people++
				only = i
			}
		}
		if demand > g.Bank[c] {
			if people == 1 {
				claims[only][c] = g.Bank[c]
			} else {
				for i := range claims {
					claims[i][c] = 0
				}
			}
			s.Log = append(s.Log, catanCardName(c)+"供应不足，按基础版库存规则发放")
		}
	}
	for i, gain := range claims {
		if sum(gain) > 0 {
			catanMove(g.Bank, g.Players[i].Resources, gain)
			s.catanLog(i, "获得 %s", catanText(gain))
		}
	}
	received := make([]int, len(g.Players))
	for i, gain := range claims {
		received[i] = sum(gain)
	}
	if sum(gold) > 0 {
		s.catanStartGold(gold, received, "catan_turn")
	} else {
		s.catanAfterProduction(received)
	}
	return nil
}
func (s *State) catanDiscard(p int, amount []int) error {
	g := s.Catan
	if s.Phase != "catan_discard" || g.DiscardDue[p] <= 0 || !g.cardBundle(amount) || sum(amount) != g.DiscardDue[p] || !catanHas(g.Players[p].Resources, amount) {
		return errors.New("请准确选择需要弃置的资源数量")
	}
	catanMove(g.Players[p].Resources, g.Bank, amount)
	if g.CitiesKnights != nil {
		s.catanLog(p, "弃置 %d 张资源或商品", sum(amount))
	} else {
		s.catanLog(p, "弃置 %d 张资源", sum(amount))
	}
	g.DiscardDue[p] = 0
	if sum(g.DiscardDue) == 0 {
		s.catanAfterSevenDiscards()
		return s.catanPirateSeven()
	}
	return nil
}
func (s *State) catanMoveRobber(p, tile int) error {
	g := s.Catan
	if s.Phase != "catan_robber" || !g.robberAllowed(tile) {
		if g.FriendlyRobber != nil {
			return errors.New("请选择合法的强盗位置；友善强盗不能影响公开分数不足3分的玩家")
		}
		return errors.New("请将强盗移到另一块陆地")
	}
	previous := g.Robber
	g.Robber = tile
	if g.CitiesKnights != nil {
		g.CitiesKnights.Chase = ""
	}
	g.Victims = []int{}
	seen := map[int]bool{}
	for _, id := range g.Tiles[tile].Vertices {
		v := g.Vertices[id]
		if v.Level > 0 && v.Owner >= 0 && v.Owner != p && !seen[v.Owner] && !g.Players[v.Owner].Eliminated && !g.friendlyProtected(v.Owner) && sum(g.Players[v.Owner].Resources) > 0 {
			seen[v.Owner] = true
			g.Victims = append(g.Victims, v.Owner)
		}
	}
	if previous == tile {
		s.catanLog(p, "没有其他合法位置，友善强盗留在沙漠")
	} else {
		s.catanLog(p, "将强盗移到地块 #%d", tile+1)
	}
	if len(g.Victims) == 0 {
		s.Phase = g.ResumePhase
	} else if len(g.Victims) == 1 {
		s.Phase = "catan_steal"
		return s.catanSteal(p, g.Victims[0])
	} else {
		s.Phase = "catan_steal"
	}
	return nil
}
func (s *State) catanSteal(p, target int) error {
	g := s.Catan
	if s.Phase != "catan_steal" {
		return errors.New("当前不能偷取资源")
	}
	valid := false
	for _, v := range g.Victims {
		valid = valid || v == target
	}
	if !valid || g.friendlyProtected(target) {
		return errors.New("请选择可偷取资源的对手")
	}
	hand := g.Players[target].Resources
	n := catanRandom(sum(hand))
	for c, count := range hand {
		if n < count {
			hand[c]--
			g.Players[p].Resources[c]++
			break
		}
		n -= count
	}
	if g.CitiesKnights != nil {
		s.catanLog(p, "从玩家 %d 随机偷取一张资源或商品", target+1)
	} else {
		s.catanLog(p, "从玩家 %d 随机偷取一张资源", target+1)
	}
	g.Victims = []int{}
	s.Phase = g.ResumePhase
	return nil
}
func (s *State) catanDev(p int, a Action) error {
	g := s.Catan
	pl := &g.Players[p]
	kind := a.Card
	maxKind := 3
	if g.pirateIslands() != nil {
		maxKind = 4
	}
	if (s.Phase != "catan_turn" && s.Phase != "catan_roll") || g.PlayedDev || kind < 0 || kind > maxKind || pl.Dev[kind]-pl.NewDev[kind] <= 0 {
		return errors.New("每回合最多使用一张发展卡，当回合购买的卡不能使用")
	}
	switch kind {
	case 0, 4:
		if g.pirateIslands() != nil && g.pirateNextWarship(p) < 0 {
			return errors.New("远征航线上没有可升级的普通船只")
		}
	case 1:
		if !g.hasFreeRouteAction(p) {
			return errors.New("没有可建造或修复的路线")
		}
	case 2:
		if !catanBundle(a.Take) || sum(a.Take) != min(2, sum(g.Bank)) || sum(a.Take) == 0 || !catanHas(g.Bank, a.Take) {
			return errors.New("请选择银行中最多两张资源")
		}
	case 3:
		if a.Color < 0 || a.Color >= 5 {
			return errors.New("请选择资源类型")
		}
	}
	pl.Dev[kind]--
	g.DevDiscard = append(g.DevDiscard, kind)
	g.PlayedDev = true
	g.Trade = nil
	g.ResumePhase = s.Phase
	switch kind {
	case 0, 4:
		if g.pirateIslands() != nil {
			id := g.pirateNextWarship(p)
			g.Edges[id].Warship = true
			s.catanLog(p, "使用骑士，将远征航线最靠近起点的普通船只 #%d 升级为战舰", id+1)
			break
		}
		pl.Knights++
		s.Phase = "catan_robber"
		if g.Seafarers != nil {
			s.catanLog(p, "使用骑士，选择移动强盗或海盗")
		} else {
			s.catanLog(p, "使用骑士，移动强盗")
		}
	case 1:
		g.FreeRoads = 2
		s.Phase = "catan_roads"
		if g.hasDamagedRoad(p) {
			s.catanLog(p, "使用道路建设：免费建造或修复，共两次")
		} else if g.Seafarers != nil {
			s.catanLog(p, "使用道路建设，免费建造两条道路或船只")
		} else {
			s.catanLog(p, "使用道路建设，免费修建两条道路")
		}
	case 2:
		catanMove(g.Bank, pl.Resources, a.Take)
		s.catanLog(p, "使用丰收，获得 %s", catanText(a.Take))
	case 3:
		count := 0
		for i := range g.Players {
			if i != p && !g.Players[i].Eliminated {
				n := g.Players[i].Resources[a.Color]
				count += n
				g.Players[i].Resources[a.Color] = 0
				pl.Resources[a.Color] += n
			}
		}
		s.catanLog(p, "使用垄断，收取 %s×%d", CatanResources[a.Color], count)
	}
	s.catanScores()
	s.catanVictory()
	return nil
}
func (s *State) catanOffer(p int, a Action) error {
	g := s.Catan
	if g.Paired != nil && g.Paired.Second {
		return errors.New("配对玩家不能与其他玩家自由交易，可使用银行或港口")
	}
	if s.Phase != "catan_turn" || !g.cardBundle(a.Give) || !g.cardBundle(a.Take) || sum(a.Give) == 0 || sum(a.Take) == 0 || !catanHas(g.Players[p].Resources, a.Give) {
		return errors.New("请提出有效交易，且持有要支付的资源")
	}
	for i := range a.Give {
		if a.Give[i] > 0 && a.Take[i] > 0 {
			return errors.New("不能同时给出和索取同一种资源")
		}
	}
	g.TradeID++
	g.Trade = &CatanTrade{g.TradeID, p, append([]int{}, a.Give...), append([]int{}, a.Take...), make([]int, len(g.Players))}
	s.catanLog(p, "提出交易：给出 %s，换取 %s", catanText(a.Give), catanText(a.Take))
	return nil
}
func (s *State) catanRespondTrade(p int, a Action) error {
	g := s.Catan
	t := g.Trade
	if s.Phase != "catan_turn" || t == nil || a.Offer != t.ID || p == t.From {
		return errors.New("这笔交易已结束或不能回应自己的交易")
	}
	if a.Type == "catan_trade_accept" {
		if !catanHas(g.Players[p].Resources, t.Take) {
			return errors.New("持有资源不足")
		}
		t.Responses[p] = 1
	} else {
		t.Responses[p] = -1
	}
	return nil
}
func (s *State) catanCompleteTrade(p int, a Action) error {
	g := s.Catan
	t := g.Trade
	target := a.Target
	if s.Phase != "catan_turn" || t == nil || t.From != p || t.ID != a.Offer || target < 0 || target >= len(g.Players) || target == p || g.Players[target].Eliminated || t.Responses[target] != 1 {
		return errors.New("请选择已接受本次交易的玩家")
	}
	from, to := g.Players[p].Resources, g.Players[target].Resources
	if !catanHas(from, t.Give) || !catanHas(to, t.Take) {
		return errors.New("玩家资源已变化，请重新提出交易")
	}
	catanMove(from, to, t.Give)
	catanMove(to, from, t.Take)
	s.catanLog(p, "与玩家 %d 完成交易：给出 %s，获得 %s", target+1, catanText(t.Give), catanText(t.Take))
	g.Trade = nil
	return nil
}
func (s *State) AutoCatanPending() {
	g := s.Catan
	if g == nil || s.Finished {
		return
	}
	if g.CardEvent != nil {
		actor := s.CatanPendingActor()
		if a, err := s.catanCardEventBot(actor); err == nil {
			_ = s.applyCatan(actor, a)
		}
		return
	}
	if k := g.CitiesKnights; k != nil && k.Pending != nil {
		actor := s.CatanPendingActor()
		if a, err := s.catanCityChoiceBot(actor); err == nil {
			_ = s.applyCatan(actor, a)
		}
		return
	}
	if p := g.pirateIslands(); p != nil && p.Raid != nil {
		actor := s.CatanPendingActor()
		if a, err := s.catanFleetRewardBot(actor); err == nil {
			_ = s.applyCatan(actor, a)
		}
		return
	}
	if s.Phase == "catan_world_ports" {
		if a, err := s.catanWorldPortBot(s.Turn); err == nil {
			_ = s.applyCatan(s.Turn, a)
		}
		return
	}
	if s.Phase == "catan_cloth_start" || s.Phase == "catan_wonders_start" {
		if a, err := s.catanBot(s.Turn); err == nil {
			_ = s.applyCatan(s.Turn, a)
		}
		return
	}
	if t := g.tribe(); t != nil && t.Pending != nil {
		for step := 0; step < 16 && g.tribe().Pending != nil; step++ {
			actor := g.tribe().Pending.Player
			a, err := s.catanTribePortBot(actor)
			if err != nil {
				break
			}
			if s.applyCatan(actor, a) != nil {
				break
			}
			g = s.Catan
		}
	} else if g.HelperPending != nil {
		for step := 0; step < 4 && s.Catan.HelperPending != nil; step++ {
			actor := s.Catan.HelperPending.Player
			a, err := s.catanBot(actor)
			if err != nil {
				break
			}
			if s.applyCatan(actor, a) != nil {
				break
			}
		}
	} else if g.GoldPending != nil {
		actor := s.CatanPendingActor()
		if a, err := s.catanGoldBot(actor); err == nil {
			_ = s.applyCatan(actor, a)
		}
	} else if s.Phase == "catan_cloth_steal" {
		if a, err := s.catanClothStealBot(s.Turn); err == nil {
			_ = s.applyCatan(s.Turn, a)
		}
	} else if s.Phase == "catan_discard" {
		for i, due := range g.DiscardDue {
			if due == 0 {
				continue
			}
			hand := append([]int{}, g.Players[i].Resources...)
			give := make([]int, len(g.Bank))
			for range due {
				n := catanRandom(sum(hand))
				for c, count := range hand {
					if n < count {
						hand[c]--
						give[c]++
						break
					}
					n -= count
				}
			}
			_ = s.catanDiscard(i, give)
		}
	} else if g.setup() {
		step := g.SetupStep
		for g.SetupStep == step {
			a, e := s.catanBot(s.Turn)
			if e != nil {
				break
			}
			if s.applyCatan(s.Turn, a) != nil {
				break
			}
			g = s.Catan
		}
	}
	if len(s.Log) > 80 {
		s.Log = s.Log[len(s.Log)-80:]
	}
}
func (s *State) EliminateCatan(p int) error {
	g := s.Catan
	if g == nil || g.setup() || s.CatanPendingActor() >= 0 || s.Finished || p != s.Turn || p < 0 || p >= len(g.Players) || s.Phase == "catan_discard" || g.Players[p].Eliminated {
		return errors.New("当前不能移除此玩家")
	}
	pl := &g.Players[p]
	pl.Eliminated = true
	if k := g.CitiesKnights; k != nil {
		// Platform timeout removal is outside the board-game rules. Recover
		// mobile pieces so nobody can later wait on this absent seat to retreat.
		k.Knights = slices.DeleteFunc(k.Knights, func(n CatanKnight) bool { return n.Owner == p })
		k.returnProgress(k.Players[p].Progress)
		k.Players[p].Progress = []int{}
		if k.Merchant != nil && k.Merchant.Owner == p {
			k.Merchant = nil
		}
		s.catanLog(p, "离场骑士返回库存")
	}
	catanMove(pl.Resources, g.Bank, append([]int{}, pl.Resources...))
	for k, n := range pl.Dev {
		for range n {
			g.DevDiscard = append(g.DevDiscard, k)
		}
	}
	pl.Dev = make([]int, 5)
	pl.NewDev = make([]int, 5)
	if pl.Helper != nil {
		g.HelperDisplay = append(g.HelperDisplay, pl.Helper.ID)
		pl.Helper = nil
	}
	g.Trade = nil
	s.catanLog(p, "超时离场：资源归还银行，建筑与道路留在地图上但不再生产")
	s.catanClothKnightRoutes()
	s.catanScores()
	active := []int{}
	for i, v := range g.Players {
		if !v.Eliminated {
			active = append(active, i)
		}
	}
	if len(active) == 1 {
		s.Turn = active[0]
		s.Winners = active
		s.Finished = true
		s.Phase = "finished"
	} else if !s.catanClothEnd() {
		s.catanNext()
		s.catanVictory()
	}
	return nil
}
