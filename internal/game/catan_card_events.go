package game

import "errors"

// CatanCardEvent persists the text-resolution queue and its production
// continuation. This is not the C&K event die or a verified event catalogue.
// Currently only Earthquake and no-effect cards have internal effect handlers;
// no room option or client action can start an arbitrary event.
type CatanCardEvent struct {
	Kind       string `json:"kind"`
	Production int    `json:"production"`
	Red        int    `json:"red"`  // Independently rolled for C&K; zero otherwise.
	Face       int    `json:"face"` // C&K event die 0–5; zero without C&K.
	Players    []int  `json:"players"`
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
	if kind != "earthquake" && kind != "beautiful_day" {
		return errors.New("该事件牌效果尚未接入")
	}
	if g.Options.Helpers || g.Options.AllHelpers {
		return errors.New("事件牌与助手的组合尚未核验")
	}
	if g.pirateIslands() != nil {
		return errors.New("事件牌与海盗群岛的舰队骰子规则尚未核验")
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
	name := "美好的一天"
	if kind == "earthquake" {
		name = "地震"
		for offset := range len(g.Players) {
			p := (next.Turn + offset) % len(g.Players)
			if len(g.earthquakeRoads(p)) > 0 {
				g.CardEvent.Players = append(g.CardEvent.Players, p)
			}
		}
	}
	next.catanLog(next.Turn, "事件牌：%s，生产点数%d；先结算事件，再生产", name, production)
	if err := next.catanContinueCardEvent(); err != nil {
		return err
	}
	*s = next
	return nil
}

func (s *State) catanContinueCardEvent() error {
	g := s.Catan
	q := g.CardEvent
	if q == nil || (q.Kind != "earthquake" && q.Kind != "beautiful_day") || q.Production < 2 || q.Production > 12 {
		return errors.New("缺少有效的事件牌后续状态")
	}
	for len(q.Players) > 0 {
		p := q.Players[0]
		if p < 0 || p >= len(g.Players) {
			return errors.New("无效事件牌回应座位")
		}
		if len(g.earthquakeRoads(p)) > 0 {
			s.Phase = "catan_card_event"
			return nil
		}
		q.Players = q.Players[1:]
	}
	if g.CitiesKnights != nil {
		if q.Red < 1 || q.Red > 6 || q.Face < 0 || q.Face > 5 || g.CitiesKnights.Event != nil || g.CitiesKnights.Pending != nil {
			return errors.New("无效事件牌城市骑士后续状态")
		}
		g.CardEvent = nil
		return s.catanStartCityDiceEvent(q.Red, 0, q.Face, q.Production)
	}
	g.CardEvent = nil
	return s.catanResolveProductionNumber(q.Production)
}

func (s *State) catanCardEventChoice(player int, a Action) error {
	g := s.Catan
	q := g.CardEvent
	if q == nil || q.Kind != "earthquake" || s.Phase != "catan_card_event" || len(q.Players) == 0 || q.Players[0] != player || a.Type != "catan_earthquake" || a.Choice != "" || a.Skill != "" {
		return errors.New("请等待对应玩家选择地震损坏的道路")
	}
	if err := s.catanDamageRoad(player, a.Edge); err != nil {
		return err
	}
	q.Players = q.Players[1:]
	return s.catanContinueCardEvent()
}

func (s *State) catanCardEventBot(player int) (Action, error) {
	g := s.Catan
	if q := g.CardEvent; q == nil || q.Kind != "earthquake" || s.Phase != "catan_card_event" || len(q.Players) == 0 || q.Players[0] != player {
		return Action{}, errors.New("inactive card-event response seat")
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
