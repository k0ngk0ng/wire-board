package game

import (
	"errors"
	"reflect"
	"slices"
)

const catanAttackCityMovePhase = "catan_attack_city_move"
const catanAttackCityRetreatPhase = "catan_attack_city_retreat"

type catanAttackCityPlan struct {
	ID      int                     `json:"id"`
	Player  int                     `json:"player"`
	Turn    uint64                  `json:"turn"`
	Before  []catanAttackCityKnight `json:"before"`
	Orders  []catanAttackCityOrder  `json:"orders"`
	Locked  int                     `json:"locked"` // Completed opponent response cannot be undone.
	Pending *catanAttackCityRetreat `json:"pending,omitempty"`
}

func (s *State) catanAttackCityBeginPlan() error {
	g := s.Catan
	if g == nil || !g.attackKnights() || s.Finished || g.setup() || s.Phase != "catan_turn" || g.Attack.City.Plan != nil || g.Attack.City.Treason != nil || g.CitiesKnights.Pending != nil || g.CitiesKnights.Event != nil || g.CardEvent != nil {
		return errors.New("当前不能开始道路骑士回合末行动")
	}
	if err := g.Attack.City.validate(g); err != nil {
		return err
	}
	c := g.Attack.City
	if c.Sequence == int(^uint(0)>>1) {
		return errors.New("道路骑士回应序号超出上限")
	}
	c.Sequence++
	c.Plan = &catanAttackCityPlan{ID: c.Sequence, Player: s.Turn, Turn: g.CitiesKnights.ActionSerial, Before: slices.Clone(c.Knights), Orders: []catanAttackCityOrder{}}
	g.Trade = nil
	s.Phase = catanAttackCityMovePhase
	return nil
}

// Replay a persisted draft from its original pieces. Only the final command
// may await an opponent. Never trust copied response targets or copied pieces.
func (s *State) catanAttackCityReplayPlan() (*catanAttackCity, *catanAttackCityRetreat, int, error) {
	g := s.Catan
	c := g.Attack.City
	q := c.Plan
	if q == nil {
		return nil, nil, 0, errors.New("缺少道路骑士移动计划")
	}
	trial := clone(*g)
	tc := trial.Attack.City
	tc.Plan = nil
	tc.Knights = slices.Clone(q.Before)
	if err := tc.validate(&trial); err != nil {
		return nil, nil, 0, err
	}
	originals := map[int]bool{}
	for _, k := range q.Before {
		if k.Owner == q.Player {
			originals[k.Edge] = true
		}
	}
	moved := map[int]bool{}
	arrivals := map[int]bool{}
	usedDisplacement := false
	locked := 0
	var pending *catanAttackCityRetreat
	for index, order := range q.Orders {
		if !originals[order.From] || moved[order.From] || arrivals[order.From] || order.From == order.To {
			return nil, nil, 0, errors.New("移动计划包含重复骑士或无效起点")
		}
		if tc.at(order.To) >= 0 {
			if usedDisplacement {
				return nil, nil, 0, errors.New("一轮最多驱逐一名道路骑士")
			}
			response, err := tc.beginDisplacement(&trial, q.Player, order.From, order.To)
			if err != nil {
				return nil, nil, 0, err
			}
			usedDisplacement = true
			if response != nil {
				if order.Retreat < 0 {
					if index != len(q.Orders)-1 {
						return nil, nil, 0, errors.New("对手退让前不能继续移动")
					}
					pending = response
				} else {
					if err = tc.completeDisplacement(&trial, response, response.Player, order.Retreat); err != nil {
						return nil, nil, 0, err
					}
					locked = index + 1
				}
			} else if order.Retreat != -1 {
				return nil, nil, 0, errors.New("无退让回应不能填写落点")
			}
		} else {
			if order.Retreat != -1 {
				return nil, nil, 0, errors.New("普通移动夹带他人退让")
			}
			if err := tc.move(&trial, q.Player, order.From, order.To); err != nil {
				return nil, nil, 0, err
			}
		}
		moved[order.From] = true
		arrivals[order.To] = true
	}
	return tc, pending, locked, nil
}
func (s *State) validateAttackCityPlan() error {
	g := s.Catan
	if g == nil || !g.attackKnights() {
		return nil
	}
	c := g.Attack.City
	q := c.Plan
	move := s.Phase == catanAttackCityMovePhase
	retreat := s.Phase == catanAttackCityRetreatPhase
	if c.Sequence < 0 || (q != nil) != (move || retreat) {
		return errors.New("道路骑士计划与阶段不一致")
	}
	if q == nil {
		return nil
	}
	if s.Finished || g.setup() || q.ID < 1 || q.ID != c.Sequence || q.Player != s.Turn || q.Player < 0 || q.Player >= len(g.Players) || g.Players[q.Player].Eliminated || q.Turn != g.CitiesKnights.ActionSerial || c.Treason != nil || g.Trade != nil || g.CitiesKnights.Event != nil || g.CitiesKnights.Pending != nil || g.CardEvent != nil || len(q.Orders) > 6 || len(q.Before) > 6*len(g.Players) {
		return errors.New("道路骑士计划序号、玩家或并行回应无效")
	}
	expected, pending, locked, err := s.catanAttackCityReplayPlan()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(expected.Knights, c.Knights) || !reflect.DeepEqual(pending, q.Pending) || q.Locked != locked || retreat != (pending != nil) {
		return errors.New("道路骑士棋盘、退让或锁定记录不一致")
	}
	return nil
}
func (s *State) catanAttackCitySyncPlan() error {
	c, pending, locked, err := s.catanAttackCityReplayPlan()
	if err != nil {
		return err
	}
	target := s.Catan.Attack.City
	target.Knights = c.Knights
	target.Plan.Pending = pending
	target.Plan.Locked = locked
	s.Phase = catanAttackCityMovePhase
	if pending != nil {
		s.Phase = catanAttackCityRetreatPhase
	}
	return nil
}
func (s *State) catanAttackCityPlanAction(player int, a Action) error {
	if s.Catan == nil || !s.Catan.attackKnights() {
		return errors.New("当前不是道路骑士组合")
	}
	if err := s.validateAttackCityPlan(); err != nil {
		return err
	}
	next := clone(*s)
	if err := next.catanAttackCityPlanStep(player, a); err != nil {
		return err
	}
	if err := next.Catan.Attack.City.validate(next.Catan); err != nil {
		return err
	}
	if err := next.validateAttackCityPlan(); err != nil {
		return err
	}
	*s = next
	return nil
}
func (s *State) catanAttackCityPlanStep(player int, a Action) error {
	g := s.Catan
	c := g.Attack.City
	q := c.Plan
	if q == nil || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || a.Prompt != q.ID || a.Type != "catan_attack_city_move" || a.Skill != "" || len(a.Tokens) != 0 || len(a.Targets) != 0 || len(a.Cards) != 0 {
		return errors.New("道路骑士回应已过期或请求无效")
	}
	if q.Pending != nil {
		if player != q.Pending.Player || a.Choice != "retreat" {
			return errors.New("请等待被驱逐骑士的主人选择退让道路")
		}
		if !slices.Contains(q.Pending.Targets, a.Edge) {
			return errors.New("请选择最近的合法空边")
		}
		q.Orders[len(q.Orders)-1].Retreat = a.Edge
		return s.catanAttackCitySyncPlan()
	}
	if player != q.Player {
		return errors.New("请等待当前玩家完成道路骑士移动")
	}
	switch a.Choice {
	case "move", "displace":
		occupied := c.at(a.Target) >= 0
		if occupied != (a.Choice == "displace") {
			return errors.New("移动或驱逐类型与目标不符")
		}
		q.Orders = append(q.Orders, catanAttackCityOrder{From: a.Edge, To: a.Target, Retreat: -1})
		return s.catanAttackCitySyncPlan()
	case "undo":
		if len(q.Orders) <= q.Locked {
			return errors.New("没有可撤销的移动；对手已确认的退让不能撤回")
		}
		q.Orders = q.Orders[:len(q.Orders)-1]
		return s.catanAttackCitySyncPlan()
	case "confirm":
		for _, choice := range s.catanAttackCityMoveChoices(q.Player) {
			if choice.Required {
				return errors.New("请先将己方骑士移出城堡")
			}
		}
		for _, k := range c.Knights {
			if k.Owner == q.Player && g.Attack.castleEdge(g, k.Edge) {
				s.catanLog(q.Player, "本站补充规则：路线 #%d 的城堡骑士没有合法出口，暂留原位，下回合重新检查", k.Edge+1)
			}
		}
		for _, order := range q.Orders {
			s.catanLog(q.Player, "道路骑士从路线 #%d 移至 #%d，移动后失活", order.From+1, order.To+1)
			if order.Retreat >= 0 {
				s.catanLog(q.Player, "被驱逐的骑士由其主人退至路线 #%d", order.Retreat+1)
			}
		}
		result := &catanAttackCityEnd{Player: q.Player, Orders: slices.Clone(q.Orders), Battles: []catanAttackCityBattle{}}
		c.Plan = nil
		s.Phase = "catan_turn"
		if err := s.catanAttackCityFinishBattles(result, func() int { return catanRandom(6) + 1 }); err != nil {
			return err
		}
		// Battles clone the State; obtain fresh pointers before persisting results.
		s.Catan.Attack.City.End = result
		if !s.Finished {
			s.catanNext()
			s.catanVictory()
		}
		return nil
	default:
		return errors.New("请选择移动、驱逐、退让、撤销或确认")
	}
}
