package game

import (
	"errors"
	"slices"
)

// CatanCardEvent persists the text-resolution queue and its production
// continuation. This is not the C&K event die or a verified event catalogue.
// No room option or client action can start an arbitrary event.
type CatanCardEvent struct {
	Kind       string           `json:"kind"`
	Production int              `json:"production"`
	Red        int              `json:"red"`  // Independently rolled for C&K; zero otherwise.
	Face       int              `json:"face"` // C&K event die 0–5; zero without C&K.
	Players    []int            `json:"players"`
	Targets    []int            `json:"targets,omitempty"`  // Frozen lower public-score recipients.
	Gifts      []CatanEventGift `json:"gifts,omitempty"`    // Private choices; never serialize to viewers.
	Optional   bool             `json:"optional,omitempty"` // Printed "may" for unawarded base conflict leader.
}

var catanCardEventNames = map[string]string{
	"beautiful_day": "美好的一天", "earthquake": "地震", "plentiful_year": "丰收年",
	"epidemic": "瘟疫", "robber_attacks": "强盗袭击", "robber_flees": "强盗逃跑",
	"good_neighbors":   "好邻居",
	"helpful_neighbor": "援助邻居",
	"calm_seas":        "风平浪静", "tournament": "比武大会",
	"conflict":        "冲突",
	"trade_advantage": "贸易优势",
}

func (g *Catan) earthquakeRoads(player int) []int {
	out := []int{}
	for _, e := range g.Edges {
		if g.roadDamageable(player, e.ID) {
			out = append(out, e.ID)
		}
	}
	return out
}

// Internal effect entry; the future verified catalogue supplies kind/number,
// and the server supplies independently rolled C&K dice. Atomic even when a
// later production or sea-scenario continuation rejects the action.
func (s *State) catanBeginCardEvent(kind string, production, red, face int) error {
	g := s.Catan
	if g == nil || s.Finished || g.setup() || s.Phase != "catan_roll" || g.CardEvent != nil || s.CatanPendingActor() >= 0 ||
		s.Turn < 0 || s.Turn >= len(g.Players) || g.Players[s.Turn].Eliminated || production < 2 || production > 12 {
		return errors.New("当前不能开始事件牌结算")
	}
	if _, ok := catanCardEventNames[kind]; !ok {
		return errors.New("该事件牌效果尚未接入")
	}
	if (g.Two != nil || g.Rivers != nil || g.Caravans != nil) && g.EventDeck == nil {
		return errors.New("双人、河流或商队事件必须通过牌堆抽取并记录生产")
	}
	if g.Attack != nil {
		if g.Paired != nil && g.Paired.Second {
			return errors.New("配对第二行动不抽取事件牌")
		}
		if !catanAttackEventSupported(kind) {
			return errors.New("蛮族进攻与该事件的组合尚未接入")
		}
		if err := s.validateCatanAttack(); err != nil {
			return err
		}
	}
	if kind == "robber_attacks" && production != 7 {
		return errors.New("强盗袭击必须按点数7处理")
	}
	if kind == "trade_advantage" && g.CitiesKnights != nil && (g.EventDeck == nil || g.EventDeck.Knights != CatanEventKnightsRules) {
		return errors.New("贸易优势与城市骑士需要明确的组合规则版本")
	}
	if g.Fishing != nil {
		return errors.New("事件牌与捕鱼组合尚未接入")
	}
	if g.Options.Helpers || g.Options.AllHelpers {
		if g.EventDeck == nil {
			return errors.New("助手事件必须通过牌堆抽取并记录生产")
		}
		if err := s.validateEventHelpers(); err != nil {
			return err
		}
	}
	if g.pirateIslands() != nil {
		return errors.New("事件牌与海盗群岛的舰队骰子规则尚未核验")
	}
	if kind == "robber_flees" && g.Attack == nil {
		// Forgotten Tribe restricts moves caused by a seven or Knight to
		// numbered hexes. Robber Flees is a separate card instruction and
		// returns the robber to a revealed desert; it does not use robberAllowed.
		for _, tile := range g.fleeDeserts() {
			if !g.clothLand(tile) {
				return errors.New("强盗逃跑与布匹剧本的沙漠限制尚未核验")
			}
		}
	}
	if k := g.CitiesKnights; k != nil {
		if !g.citySeaSupported() || k.Event != nil || k.Pending != nil || red < 1 || red > 6 || face < 0 || face > 5 {
			return errors.New("事件牌缺少合法的城市骑士独立骰子")
		}
	} else if red != 0 || face != 0 {
		return errors.New("基础事件牌不使用城市骑士骰子")
	}
	next := clone(*s)
	g = next.Catan
	g.Trade = nil
	g.Dice = []int{red, 0} // No fabricated yellow die or dice matching the card.
	g.RollID++
	g.CardEvent = &CatanCardEvent{Kind: kind, Production: production, Red: red, Face: face, Players: []int{}}
	if kind == "earthquake" || kind == "plentiful_year" {
		for offset := range len(g.Players) {
			p := (next.Turn + offset) % len(g.Players)
			if !g.Players[p].Eliminated && (kind == "plentiful_year" || len(g.earthquakeRoads(p)) > 0) {
				g.CardEvent.Players = append(g.CardEvent.Players, p)
			}
		}
	}
	next.catanLog(next.Turn, "事件牌：%s，生产点数%d；先结算事件，再生产", catanCardEventNames[kind], production)
	if kind == "good_neighbors" {
		g.beginNeighborGifts(next.Turn)
	}
	if kind == "helpful_neighbor" {
		g.beginHelpfulNeighbor(next.Turn)
	}
	if kind == "calm_seas" || kind == "tournament" {
		g.CardEvent.Players = g.cardEventLeaders(kind, next.Turn)
	}
	if kind == "conflict" {
		g.beginCardConflict(next.Turn)
	}
	if kind == "trade_advantage" && g.LongestOwner >= 0 && g.LongestOwner < len(g.Players) && !g.Players[g.LongestOwner].Eliminated {
		g.CardEvent.Players = []int{g.LongestOwner}
	}
	if kind == "robber_flees" && g.Attack == nil {
		if k := g.CitiesKnights; k != nil && k.Invasions == 0 {
			next.catanLog(next.Turn, "强盗尚未入场，保持休眠")
		} else {
			deserts := g.fleeDeserts()
			switch len(deserts) {
			case 0:
				next.catanFleeRobber(-1)
			case 1:
				next.catanFleeRobber(deserts[0])
			default:
				g.CardEvent.Players = []int{next.Turn}
			}
		}
	}
	if err := next.catanContinueCardEvent(); err != nil {
		return err
	}
	if err := next.validateCatanAttack(); err != nil {
		return err
	}
	*s = next
	return nil
}

func (s *State) catanContinueCardEvent() error {
	g := s.Catan
	q := g.CardEvent
	if q == nil || catanCardEventNames[q.Kind] == "" || q.Production < 2 || q.Production > 12 || (q.Kind == "robber_attacks" && q.Production != 7) {
		return errors.New("缺少有效的事件牌后续状态")
	}
	g.rememberCardEvent()
	for len(q.Players) > 0 {
		p := q.Players[0]
		if p < 0 || p >= len(g.Players) {
			return errors.New("无效事件牌回应座位")
		}
		canChoose := false
		if !g.Players[p].Eliminated {
			switch q.Kind {
			case "earthquake":
				canChoose = len(g.earthquakeRoads(p)) > 0
			case "plentiful_year", "calm_seas", "tournament":
				canChoose = sum(g.Bank[:5]) > 0
				if !canChoose {
					s.catanLog(p, "%s：银行没有剩余普通资源，无法领取", catanCardEventNames[q.Kind])
				}
			case "robber_flees":
				canChoose = len(g.fleeDeserts()) > 0
			case "good_neighbors":
				canChoose = true // Eligibility was frozen before anyone received a card.
			case "helpful_neighbor":
				canChoose = sum(g.Players[p].Resources) > 0 && len(q.Targets) > 0
			case "conflict", "trade_advantage":
				canChoose = len(g.cardTheftTargets(p)) > 0
			default:
				return errors.New("该事件不需要玩家选择")
			}
		}
		if canChoose {
			s.Phase = "catan_card_event"
			return nil
		}
		q.Players = q.Players[1:]
	}
	if q.Kind == "good_neighbors" {
		if err := s.catanTransferNeighborGifts(); err != nil {
			return err
		}
	}
	if g.CitiesKnights != nil {
		if q.Red < 1 || q.Red > 6 || q.Face < 0 || q.Face > 5 || g.CitiesKnights.Event != nil || g.CitiesKnights.Pending != nil {
			return errors.New("无效事件牌城市骑士后续状态")
		}
		g.CardEvent = nil
		return s.catanStartCityDiceEvent(q.Red, 0, q.Face, q.Production, q.Kind == "epidemic")
	}
	g.CardEvent = nil
	// Pirate Islands is gated at entry: its fleet needs a separately verified
	// dice rule. All accepted recipes can proceed directly to this production.
	return s.catanRollProductionEffect(q.Production, q.Kind == "epidemic")
}

func (s *State) catanCardEventChoice(player int, a Action) error {
	g := s.Catan
	q := g.CardEvent
	if q == nil || s.Phase != "catan_card_event" || len(q.Players) == 0 || q.Players[0] != player || a.Choice != "" || a.Skill != "" {
		return errors.New("请等待对应玩家完成事件牌选择")
	}
	switch q.Kind {
	case "helpful_neighbor":
		if err := s.catanHelpfulNeighborChoice(player, a); err != nil {
			return err
		}
	case "conflict", "trade_advantage":
		if err := s.catanCardTheftChoice(player, a); err != nil {
			return err
		}
	case "good_neighbors":
		if err := g.chooseNeighborGift(player, a); err != nil {
			return err
		}
	case "earthquake":
		if a.Type != "catan_earthquake" {
			return errors.New("请选择地震损坏的道路")
		}
		if err := s.catanDamageRoad(player, a.Edge); err != nil {
			return err
		}
	case "plentiful_year", "calm_seas", "tournament":
		if a.Type != "catan_event_resource" || !catanBundle(a.Take) || sum(a.Take) != 1 || !catanHas(g.Bank, a.Take) {
			return errors.New("请选择银行中一张普通资源，不能选择商品")
		}
		catanMove(g.Bank, g.Players[player].Resources, a.Take)
		s.catanLog(player, "%s：领取 %s", catanCardEventNames[q.Kind], catanText(a.Take))
	case "robber_flees":
		if a.Type != "catan_robber_flees" || !slices.Contains(g.fleeDeserts(), a.Tile) {
			return errors.New("请选择强盗逃往的沙漠")
		}
		s.catanFleeRobber(a.Tile)
	default:
		return errors.New("该事件不需要玩家选择")
	}
	q.Players = q.Players[1:]
	return s.catanContinueCardEvent()
}

func (s *State) catanCardEventBot(player int) (Action, error) {
	g := s.Catan
	q := g.CardEvent
	if q == nil || s.Phase != "catan_card_event" || len(q.Players) == 0 || q.Players[0] != player {
		return Action{}, errors.New("inactive card-event response seat")
	}
	if (q.Kind == "plentiful_year" || q.Kind == "calm_seas" || q.Kind == "tournament") && sum(g.Bank[:5]) > 0 {
		return Action{Type: "catan_event_resource", Take: g.catanResourceChoiceBot(player, 1)}, nil
	}
	if q.Kind == "conflict" || q.Kind == "trade_advantage" {
		target := -1
		for _, p := range g.cardTheftTargets(player) {
			// Public score only; exclude hidden VP cards and resource colors.
			if target < 0 || g.Players[p].Score-g.hiddenVictoryPoints(p) > g.Players[target].Score-g.hiddenVictoryPoints(target) {
				target = p
			}
		}
		if target >= 0 {
			return Action{Type: "catan_event_steal", Target: target}, nil
		}
	}
	if q.Kind == "good_neighbors" || q.Kind == "helpful_neighbor" {
		// Give the most plentiful card in our own hand. Never inspect the
		// recipient's hand or another player's pending private selection.
		hand := g.Players[player].Resources
		color := -1
		for i, count := range hand {
			if count > 0 && (color < 0 || count > hand[color]) {
				color = i
			}
		}
		if color >= 0 {
			give := make([]int, len(hand))
			give[color] = 1
			a := Action{Type: "catan_event_gift", Give: give}
			if q.Kind == "helpful_neighbor" {
				if len(q.Targets) == 0 {
					return Action{}, errors.New("no lower-score neighbor")
				}
				a.Target = q.Targets[0]
				for _, target := range q.Targets[1:] {
					if g.Players[target].Score-g.hiddenVictoryPoints(target) < g.Players[a.Target].Score-g.hiddenVictoryPoints(a.Target) {
						a.Target = target
					}
				}
			}
			return a, nil
		}
	}
	if q.Kind == "robber_flees" {
		if deserts := g.fleeDeserts(); len(deserts) > 0 {
			return Action{Type: "catan_robber_flees", Tile: deserts[0]}, nil
		}
	}
	if q.Kind != "earthquake" {
		return Action{}, errors.New("no choice for this card event")
	}
	best, loss := -1, int(^uint(0)>>1)
	for _, edge := range g.earthquakeRoads(player) {
		// Prefer damage that removes fewer useful settlement connections.
		// Only the public board is inspected, never another player's hand.
		temp := *g
		temp.Edges = append([]CatanEdge{}, g.Edges...)
		temp.Edges[edge].Damaged = true
		value := 0
		for _, v := range []int{g.Edges[edge].A, g.Edges[edge].B} {
			if g.canSettlement(player, v, false) && !temp.canSettlement(player, v, false) {
				value += 1 + g.vertexValue(player, v)
			}
		}
		if value < loss {
			best, loss = edge, value
		}
	}
	if best < 0 {
		return Action{}, errors.New("no road available for earthquake")
	}
	return Action{Type: "catan_earthquake", Edge: best}, nil
}

func (g *Catan) fleeDeserts() []int {
	// T&B 2025 p12 changes desert-directed actions to swamps specifically
	// for CATAN for Two + Rivers. Other river games have no desert and use
	// Robber Flees' explicit off-board instruction instead.
	if g.Two != nil && g.Rivers != nil {
		return slices.Clone(g.Rivers.Map.Swamps)
	}
	deserts := []int{}
	for _, tile := range g.Tiles {
		if tile.Resource == CatanDesert {
			deserts = append(deserts, tile.ID)
		}
	}
	return deserts
}

func (s *State) catanFleeRobber(tile int) {
	g := s.Catan
	g.Robber = tile
	g.Victims = []int{}
	if g.CitiesKnights != nil {
		g.CitiesKnights.Chase = ""
	}
	if tile < 0 {
		s.catanLog(s.Turn, "强盗逃跑：没有沙漠，强盗移到场外，不偷牌")
	} else if g.Tiles[tile].Resource == catanSwamp {
		s.catanLog(s.Turn, "强盗逃往沼泽 #%d，不偷牌", tile+1)
	} else {
		s.catanLog(s.Turn, "强盗逃往沙漠 #%d，不偷牌", tile+1)
	}
}
