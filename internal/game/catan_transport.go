package game

import (
	"errors"
	"slices"
	"strings"
)

// The extended deck uses a labelled site recipe; the base deck is unchanged.
const CatanTransportExtendedDeck = "wire-board-transport-deck-v1"
const catanTransportDeckNotice = "本站牌组配置：五六人在原25张运输发展牌中加入8张骑士、2张道路建设、2张快速旅程，共37张"

func catanTransportDeckCounts(n int) []int {
	if n > 4 {
		return []int{24, 5, 5, 0, 3}
	}
	return []int{16, 3, 3, 0, 3}
}

// NewCatanTransport chooses the board and paired turns for actual players.
func NewCatanTransport(n int) (*State, error) {
	return newCatanTransportState(n, CatanOptions{FiveSix: n > 4})
}

func newCatanTransportState(n int, options CatanOptions) (*State, error) {
	if n < 2 || n > 6 || options.Helpers || options.AllHelpers || options.FiveSix != (n > 4) {
		return nil, errors.New("运输支持2至6人，五六人使用配对回合；不混用其他扩展")
	}
	var s *State
	var err error
	if n == 2 {
		s, err = NewCatanTwo(n, options)
	} else {
		s, err = NewCatan(n, options)
	}
	if err != nil {
		return nil, err
	}
	board, m, err := newCatanTransportBoard(n)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.Ports, g.HexSize = board.Tiles, board.Vertices, board.Edges, board.Ports, board.HexSize
	g.Robber = -1
	g.Transport, err = newCatanTransportPieces(g, m)
	if err != nil {
		return nil, err
	}
	if n == 2 {
		if err := g.prepareTwoNeutrals(); err != nil {
			return nil, err
		}
	}
	g.DevDeck = []int{}
	for kind, count := range catanTransportDeckCounts(n) {
		for range count {
			g.DevDeck = append(g.DevDeck, kind)
		}
	}
	shuffle(g.DevDeck)
	s.Log = []string{"运输任务：先建村庄，再逆序建城市；起始城市只领每邻格1资源，马车随城市放置", "运输任务：不使用强盗与最长道路；自己回合达到13分获胜"}
	if n > 4 {
		g.Transport.DeckRecipe = CatanTransportExtendedDeck
		s.Log = append(s.Log, catanTransportDeckNotice, "五六人运输：2与12正常生产；配对玩家分别完成建设交易和马车移动")
	}
	s.catanScores()
	return s, s.validateCatanTransport()
}
func (s *State) validateCatanTransport() error {
	g := s.Catan
	if g == nil || g.Transport == nil {
		return nil
	}
	t := g.Transport
	if err := g.validateRivers(); err != nil {
		return err
	}
	if err := s.validateCaravans(); err != nil {
		return err
	}
	n := len(g.Players)
	options, _ := NormalizeCatanOptions(CatanOptions{FiveSix: n > 4})
	if n < 2 || n > 6 || (g.Paired != nil) != (n > 4) || g.Options != options || g.Harbors != nil || g.FriendlyRobber != nil || g.BaseSetup != nil || g.EventDeck == nil && (g.CardEvent != nil || g.RevealedEvent != nil) || g.Robber != -1 || g.LongestOwner != -1 {
		return errors.New("运输整局人数、组合或基础棋子状态无效")
	}
	if !g.transportKnights() && n > 4 && t.DeckRecipe != CatanTransportExtendedDeck || (n <= 4 || g.transportKnights()) && t.DeckRecipe != "" {
		return errors.New("运输发展牌配置版本无效")
	}
	if pair := g.Paired; pair != nil {
		if pair.Primary < 0 || pair.Primary >= n || pair.Secondary < 0 || pair.Secondary >= n {
			return errors.New("运输配对玩家无效")
		}
		actor := pair.Primary
		if pair.Second {
			actor = pair.Secondary
		}
		if !g.setup() && !s.Finished && (s.Turn != actor || pair.Second && s.Phase == "catan_roll") {
			return errors.New("运输配对行动阶段无效")
		}
	}
	if g.fishingTransport() {
		if err := g.validateFishing(); err != nil {
			return err
		}
		if (g.Fishing.Pending != nil) != (s.Phase == "catan_fish_replace") {
			return errors.New("运输捕鱼回应阶段冲突")
		}
	}
	if err := s.validateTransportKnights(); err != nil {
		return err
	}
	if err := t.validate(g); err != nil {
		return err
	}

	phases := []string{"catan_setup_settlement", "catan_setup_city", "catan_setup_road", "catan_roll", "catan_turn", "catan_discard", "catan_roads", "catan_transport_barbarian", "catan_transport_move", "catan_two_build", "catan_two_trade", "catan_card_event", "catan_fish_replace", "finished"}
	if g.caravansTransport() {
		phases = append(phases, "catan_caravan_bid", "catan_caravan_vote", "catan_caravan_place")
	}
	if g.transportKnights() {
		phases = append(phases, catanTransportCityPhases...)
	}
	if !slices.Contains(phases, s.Phase) || s.Phase == "catan_card_event" && (g.EventDeck == nil || g.CardEvent == nil) {
		return errors.New("运输游戏阶段无效")
	}

	if strings.HasPrefix(s.Phase, "catan_two_") && g.Two == nil {
		return errors.New("双人响应缺少双人控制器")
	}
	if s.Finished != (s.Phase == "finished") || t.BarbarianPending && g.ResumePhase != "catan_roll" && g.ResumePhase != "catan_turn" {
		return errors.New("运输结束标志或蛮族返回阶段无效")
	}
	if !s.Finished && t.Travel != nil && s.Phase != "catan_transport_move" {
		return errors.New("运输移动尚未结束，不能返回普通行动")
	}
	if !s.Finished && s.Phase == "catan_transport_move" && t.Travel != nil && t.Travel.Ended && (t.Travel.Arrived < 0 || t.ArrivalResolved) {
		return errors.New("运输已完成，应进入下一次移动或下一玩家")
	}
	if !s.Finished && !g.setup() && (t.Active != s.Turn || t.GameTurn != g.TurnSerial) {
		return errors.New("运输回合与主游戏不一致")
	}
	if t.BarbarianPending != (s.Phase == "catan_transport_barbarian") || t.BarbarianPending && t.BarbarianSequence == 0 || t.Moves < 0 || t.Moves > 2 || t.Moves == 2 && !t.Swift {
		return errors.New("运输蛮族或额外移动状态无效")
	}
	if s.Phase == "catan_transport_move" && (t.Travel == nil || t.Moves == 0) {
		return errors.New("运输移动记录缺失")
	}
	counts := make([]int, 5)
	for _, deck := range [][]int{g.DevDeck, g.DevDiscard} {
		for _, card := range deck {
			if card < 0 || card >= 5 {
				return errors.New("运输发展牌种类无效")
			}
			counts[card]++
		}
	}
	for _, p := range g.Players {
		if !catanBundle(p.Dev) || !catanBundle(p.NewDev) {
			return errors.New("运输手中发展牌无效")
		}
		for card, n := range p.Dev {
			if p.NewDev[card] > n {
				return errors.New("新购发展牌数量无效")
			}
			counts[card] += n
		}
	}
	want := catanTransportDeckCounts(n)
	if g.transportKnights() {
		want = make([]int, 5)
	}
	if !slices.Equal(counts, want) {
		return errors.New("运输发展牌库存不守恒")
	}
	return s.validateCatanTwo()
}
func (s *State) catanTransportSyncTurn() error {
	g := s.Catan
	if g == nil || g.Transport == nil || g.setup() || s.Finished {
		return nil
	}
	t := g.Transport
	if t.Active == s.Turn && t.GameTurn == g.TurnSerial {
		return nil
	}
	// Only the main game advances turns; timeout departure can skip an ordinary
	// action phase, so synchronize here rather than pretending a move was made.
	t.Active = s.Turn
	t.TurnSerial++
	t.GameTurn = g.TurnSerial
	t.Bought = 0
	t.Moves = 0
	t.Swift = false
	t.Travel = nil
	t.ArrivalResolved = false
	return nil
}
func (s *State) applyCatanTransport(player int, a Action) error {
	g := s.Catan
	t := g.Transport
	if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated {
		return errors.New("无法操作此座位")
	}
	if s.Phase == "catan_transport_barbarian" {
		return s.catanTransportBarbarian(player, a)
	}
	if s.Phase == "catan_transport_move" {
		return s.catanTransportMoveAction(player, a)
	}
	if strings.HasPrefix(a.Type, "catan_transport_") && a.Type != "catan_transport_upgrade" && a.Type != "catan_transport_knight_chase" {
		return errors.New("当前没有对应的运输选择")
	}
	var err error
	switch {
	case g.Two != nil && (g.Two.Pending != nil || g.Two.Trade != nil):
		err = s.applyCatanStep(player, a)
	case g.setup():
		err = s.applyCatanStep(player, a)
		if err == nil && a.Type == "catan_city" {
			err = t.placeWagon(g, player, a.Vertex)
		}
	case player == s.Turn && a.Type == "catan_roll":
		if g.transportKnights() {
			err = s.applyCatanStep(player, a)
		} else if g.EventDeck != nil {
			err = s.catanDrawEvent()
		} else {
			err = s.catanTransportRoll(func() [2]int { return [2]int{catanRandom(6) + 1, catanRandom(6) + 1} })
		}
	case player == s.Turn && a.Type == "catan_dev":
		err = s.catanTransportDev(player, a)
	case player == s.Turn && a.Type == "catan_end" && s.Phase == "catan_turn":
		s.catanScores()
		s.catanVictory()
		if s.Finished {
			return nil
		}
		if g.transportKnights() {
			err = s.applyCatanStep(player, a)
		} else {
			err = s.catanTransportBeginTravel(player)
		}
	case a.Type == "catan_transport_upgrade" || a.Type == "catan_coin_buy" || a.Type == "catan_coin_sell":
		if player != s.Turn || s.Phase != "catan_turn" {
			return errors.New("请在自己的行动阶段操作马车或金币")
		}
		switch a.Type {
		case "catan_transport_upgrade":
			err = t.upgrade(g, player)
			if err == nil {
				s.catanLog(player, "升级马车至%d级", t.Wagons[player].Level+1)
			}
		case "catan_coin_buy":
			err = t.buyResource(g, player, a.Color)
			if err == nil {
				s.catanLog(player, "支付2金币，获得%s×1", catanCardName(a.Color))
			}
		case "catan_coin_sell":
			rate := 0
			if a.Color >= 0 && a.Color < len(g.Bank) {
				rate = g.rates(player)[a.Color]
			}
			err = t.sellResource(g, player, a.Color)
			if err == nil {
				s.catanLog(player, "支付%s×%d，获得1金币", catanCardName(a.Color), rate)
			}
		}
		if err == nil {
			g.Trade = nil
			s.catanScores()
			s.catanVictory()
		}
	case a.Type == "catan_robber" || a.Type == "catan_pirate":
		return errors.New("运输剧本只移动蛮族，不使用强盗或海盗")
	default:
		err = s.applyCatanStep(player, a)
	}
	if err != nil {
		return err
	}
	return s.catanTransportSyncTurn()
}
func (s *State) catanTransportRoll(roll func() [2]int) error {
	g := s.Catan
	if s.Phase != "catan_roll" || g.EventDeck != nil {
		return errors.New("当前不能掷骰")
	}
	for {
		dice := roll()
		if dice[0] < 1 || dice[0] > 6 || dice[1] < 1 || dice[1] > 6 {
			return errors.New("骰子无效")
		}
		total := dice[0] + dice[1]
		if len(g.Players) <= 4 && !g.riversTransport() && !g.caravansTransport() && (total == 2 || total == 12) {
			s.catanLog(s.Turn, "运输掷出%d，重新掷骰", total)
			continue
		}
		if g.Two != nil {
			if len(g.Two.Rolls) == 1 && total == g.Two.Rolls[0] {
				s.catanLog(s.Turn, "双人第二次掷出相同点数%d，重新掷骰", total)
				continue
			}
			return s.catanTwoRoll(dice[0], dice[1])
		}
		g.Dice = []int{dice[0], dice[1]}
		g.RollID++
		return s.catanRoll(total)
	}
}
func (s *State) catanTransportDev(player int, a Action) error {
	g := s.Catan
	t := g.Transport
	p := &g.Players[player]
	if a.Card == 1 {
		return s.catanDev(player, a)
	}
	if (s.Phase != "catan_roll" && s.Phase != "catan_turn") || g.PlayedDev || (a.Card != 0 && a.Card != 2) || p.Dev[a.Card]-p.NewDev[a.Card] <= 0 {
		return errors.New("本回合只能用一张旧发展牌：骑士、道路建设或快速旅程")
	}
	p.Dev[a.Card]--
	g.DevDiscard = append(g.DevDiscard, a.Card)
	g.PlayedDev = true
	g.Trade = nil
	if a.Card == 2 {
		t.Swift = true
		s.catanLog(player, "使用快速旅程：本回合马车可再移动一次")
	} else {
		p.Knights++
		g.ResumePhase = s.Phase
		s.catanTransportBeginBarbarian()
		s.catanLog(player, "使用骑士：移动一名蛮族，若落到对手道路则偷取1资源")
	}
	s.catanScores()
	s.catanVictory()
	if s.Finished {
		t.BarbarianPending = false
	}
	return nil
}
func (s *State) catanTransportBeginBarbarian() {
	t := s.Catan.Transport
	t.BarbarianSequence++
	t.BarbarianPending = true
	s.Phase = "catan_transport_barbarian"
}
func (s *State) catanTransportBarbarian(player int, a Action) error {
	g := s.Catan
	t := g.Transport
	if player != s.Turn || a.Type != "catan_transport_barbarian" || a.Offer <= 0 || uint64(a.Offer) != t.BarbarianSequence || a.Card < 0 || a.Card >= 3 || a.Edge < 0 || a.Edge >= len(g.Edges) || slices.Contains(t.Barbarians[:], a.Edge) {
		return errors.New("请选择一名蛮族及另一个没有蛮族的边")
	}
	t.Barbarians[a.Card] = a.Edge
	t.BarbarianPending = false
	s.Phase = g.ResumePhase
	s.catanLog(player, "移动蛮族%d到道路 #%d", a.Card+1, a.Edge+1)
	owner := g.Edges[a.Edge].Owner
	if owner >= 0 && owner != player && !g.Players[owner].Eliminated && sum(g.Players[owner].Resources) > 0 {
		g.Victims = []int{owner}
		s.Phase = "catan_steal"
		return s.catanSteal(player, owner)
	}
	return nil
}
func (s *State) catanTransportMoveAction(player int, a Action) error {
	g := s.Catan
	t := g.Transport
	if a.Offer <= 0 || uint64(a.Offer) != t.Sequence || player != s.Turn {
		return errors.New("运输行动已过期或不是当前玩家")
	}
	var err error
	switch a.Type {
	case "catan_transport_step":
		var step catanTransportStep
		step, err = t.move(g, player, t.Sequence, a.Edge)
		if err == nil {
			if step.Pay >= 0 && g.Players[step.Pay].Eliminated {
				t.Gold[step.Pay] -= step.Toll
				t.GoldBank += step.Toll
			}
			s.catanLog(player, "马车沿道路 #%d 移动，花费%d移动点、%d金币", a.Edge+1, step.MP, step.Toll)
		}
	case "catan_transport_fish":
		err = s.catanTransportFish(player, a)
	case "catan_transport_wheat":
		err = t.wheat(g, player, t.Sequence)
		if err == nil {
			s.catanLog(player, "支付粮食×1，马车增加2移动点")
		}
	case "catan_transport_stop":
		err = t.stop(g, player, t.Sequence)
	case "catan_transport_drive":
		var success bool
		die := catanRandom(6) + 1
		success, err = t.driveOff(g, player, t.Sequence, a.Card, die)
		if err == nil {
			s.catanLog(player, "马车驱赶蛮族掷出%d，成功：%t", die, success)
		}
	case "catan_transport_relocate":
		err = t.relocate(g, player, t.Sequence, a.Edge)
	case "catan_transport_arrival":
		if a.Choice != "deliver" && a.Choice != "keep" {
			return errors.New("请选择交货或保留当前货物")
		}
		s.catanScores()
		win := a.Choice == "deliver" && g.Players[player].Score+1 >= g.victoryTargetFor(player)
		var result catanTransportArrivalResult
		result, err = t.resolveArrivalWithStop(g, player, t.Sequence, a.Choice == "deliver", win)
		if err == nil {
			if result.Delivered != 0 {
				s.catanLog(player, "交付%s，获得1分和%d金币", transportCargoName(t, result.Delivered), result.Gold)
			}
			if result.Loaded != 0 {
				s.catanLog(player, "装载%s", transportCargoName(t, result.Loaded))
			}
			s.catanScores()
			s.catanVictory()
		}
	default:
		return errors.New("请先完成马车移动或装卸")
	}
	if err != nil {
		return err
	}
	// A toll can transfer the richest-Catanian point during movement.
	if g.riversTransport() {
		s.catanScores()
		s.catanVictory()
	}
	if s.Finished {
		return nil
	}
	q := t.Travel
	if q.Ended && (q.Arrived < 0 || t.ArrivalResolved) {
		if t.Swift && t.Moves == 1 {
			next, e := newCatanTransportTravel(g, t.Map, player, q.Position, q.Level)
			if e != nil {
				return e
			}
			next.WheatUsed = q.WheatUsed
			next.FishUsed = q.FishUsed
			next.Attempted = q.Attempted
			t.Travel = next
			t.Sequence++
			t.Moves = 2
			t.ArrivalResolved = false
			s.catanLog(player, "快速旅程：开始第二次马车移动")
			return nil
		}
		return s.catanAfterTransportTravel()
	}
	return nil
}
func transportCargoName(t *catanTransport, id int) string {
	token, _ := t.token(id)
	return map[string]string{"marble": "大理石", "sand": "沙", "glass": "玻璃", "tools": "工具"}[token.Cargo]
}
func (s *State) catanTransportView(v map[string]any, player int) {
	t := s.Catan.Transport
	public := t.publicView()
	v["transport"] = map[string]any{"rules": catanTransportRules, "map": t.Map, "state": public, "canAct": player == s.Turn && !s.Finished, "canDeliver": t.canDeliver(), "swift": t.Swift, "moves": t.Moves, "barbarianPending": t.BarbarianPending, "barbarianSequence": t.BarbarianSequence, "bought": t.Bought, "choices": s.catanTransportChoices(player)}
	if s.Catan.transportKnights() {
		v["transport"].(map[string]any)["knights"] = CatanTransportKnightsRules
		return
	}
	v["developmentNames"] = []string{"骑士", "道路建设", "快速旅程", "", "胜利点"}
	if t.DeckRecipe != "" {
		v["transport"].(map[string]any)["deckRecipe"] = t.DeckRecipe
	}
}
