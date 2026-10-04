package game

import (
	"errors"
	"slices"
)

func (s *State) sgHegDamage(e *SGEvent) bool {
	if !s.sgHegemony() || e.HegDamageChecked {
		return false
	}
	e.HegDamageChecked = true
	if s.sgHegMayInvoke(e.Target, "heg_mingshi") && e.Actor >= 0 && e.Actor < len(s.Sanguosha.Players) {
		h := s.Sanguosha.Players[e.Actor].Hegemony
		if !h.Shown[0] || !h.Shown[1] {
			if !s.sgHas(e.Target, "heg_mingshi") {
				next := *e
				next.Type = "damage"
				s.sgAsk(e.Target, "heg_mingshi", "名士：可明置武将，令本次伤害减少一点", SGEvent{Actor: e.Target, Next: &next})
				s.Sanguosha.Pending.Choices = []string{"yes", "pass"}
				return true
			}
			e.Amount--
			s.sgLog("%s 的名士令本次伤害减少一点", s.sgName(e.Target))
			return e.Amount <= 0
		}
	}
	return false
}

// Called before the movement while original hand/equipment ownership is known.
// Its event runs after loss triggers and offers only cards still discarded.
// Responses, uses, recasts, judgments and equipment replacement never call it.
func (s *State) sgHegDiscarded(i int, ids []int) {
	if !s.sgHegemony() || !s.sgAlive(i) {
		return
	}
	p := s.Sanguosha.Players[i]
	cards := []int{}
	for _, id := range ids {
		if slices.Contains(p.Hand, id) || slices.Contains(p.Equip, id) {
			cards = append(cards, id)
		}
	}
	if len(cards) > 0 {
		s.sgPush(SGEvent{Type: "heg_lirang", Actor: i, Cards: cards})
	}
}

func (s *State) sgHegDeathResponse(i int) bool {
	q := s.Sanguosha.Pending
	return s.sgHegemony() && q != nil && q.Player == i && q.Kind == "heg_duanchang"
}

func (s *State) sgHegRulesEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "heg_hide":
		if s.sgAlive(e.Target) {
			g.Players[e.Target].Hegemony.Shown[e.Aux] = false
			s.sgLog("%s 发动倾城，暗置 %s 的%s", s.sgName(e.Actor), s.sgName(e.Target), []string{"主将", "副将"}[e.Aux])
		}
	case "heg_lirang":
		if !s.sgHegMayInvoke(e.Actor, "heg_lirang") {
			return true
		}
		e.Cards = slices.DeleteFunc(clone(e.Cards), func(id int) bool { return !slices.Contains(g.Discard, id) })
		if len(e.Cards) > 0 {
			s.sgAsk(e.Actor, "heg_lirang", "礼让：可将本次弃置的牌交给其他角色，或结束分配", e)
			g.Pending.Cards = clone(e.Cards)
			g.Pending.Targets = sgRemove(s.sgOrder(s.Turn), e.Actor)
		}
	case "heg_shuangren":
		if !s.sgHegMayInvoke(e.Actor, "heg_shuangren") || len(g.Players[e.Actor].Hand) == 0 {
			return true
		}
		targets := []int{}
		for _, who := range s.sgOrder(s.Turn) {
			if who != e.Actor && len(g.Players[who].Hand) > 0 {
				targets = append(targets, who)
			}
		}
		if len(targets) > 0 {
			s.sgAsk(e.Actor, "heg_shuangren", "双刃：可选择一张手牌和另一角色拼点，未赢则跳过出牌阶段", e)
			g.Pending.Targets = targets
		}
	case "dying":
		if e.HegDyingChecked || !s.sgAlive(e.Target) || g.Players[e.Target].HP > 0 {
			return false
		}
		e.HegDyingChecked = true
		es := []SGEvent{}
		if e.Actor >= 0 {
			for _, who := range s.sgOrder(s.Turn) {
				if who != e.Target {
					es = append(es, SGEvent{Type: "heg_suishi", Actor: who, Target: e.Actor, Kind: "dying"})
				}
			}
		}
		es = append(es, e)
		s.sgPush(es...)
	case "heg_suishi":
		if s.sgHegMayInvoke(e.Actor, "heg_suishi") && s.sgHegWouldFriend(e.Actor, e.Target) {
			if !s.sgHas(e.Actor, "heg_suishi") {
				s.sgAsk(e.Actor, "heg_suishi", "随势：可明置武将发动此锁定技；明置后须按规则执行", e)
				g.Pending.Choices = []string{"yes", "pass"}
				return true
			}
			if e.Kind == "dying" {
				s.sgDraw(e.Actor, 1)
			} else {
				s.sgPush(SGEvent{Type: "lose_hp", Target: e.Actor, Amount: 1, Kind: "heg_suishi"})
			}
		}
	case "heg_kuanggu":
		if s.sgHas(e.Actor, "kuanggu") {
			s.sgHeal(e.Actor, e.Amount)
		} else if s.sgHegMayInvoke(e.Actor, "kuanggu") && g.Players[e.Actor].HP < g.Players[e.Actor].MaxHP {
			s.sgAsk(e.Actor, "kuanggu", "狂骨：可明置武将，按造成的伤害回复体力", e)
			g.Pending.Choices = []string{"yes", "pass"}
		}
	case "heg_duanchang":
		s.sgAsk(e.Actor, "heg_duanchang", "断肠：令伤害来源的主将或副将失去技能，暗将身份不会因此公开", e)
		g.Pending.Choices = []string{"head", "deputy"}
	default:
		return false
	}
	return true
}

func (s *State) sgHegShuangrenResult(e SGEvent, win bool) {
	g := s.Sanguosha
	if !win {
		g.SkipPlay = true
		return
	}
	if !s.sgAlive(e.Actor) {
		return
	}
	targets := []int{}
	for _, who := range s.sgOrder(s.Turn) {
		if (who == e.Target || s.sgHegFriend(e.Target, who)) && s.sgCanTargetRange(e.Actor, who, "slash", true) {
			targets = append(targets, who)
		}
	}
	if len(targets) > 0 {
		s.sgAsk(e.Actor, "heg_shuangren_slash", "双刃胜出：选择对手或其已明置同势力角色，使用无距离限制的杀", e)
		g.Pending.Targets = targets
	}
}

func (s *State) sgHegRulesRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	e := q.Event
	switch q.Kind {
	case "heg_mingshi":
		if a.Choice != "yes" && a.Choice != "pass" {
			return true, errors.New("请选择发动或放弃名士")
		}
		next := *e.Next
		if a.Choice == "yes" {
			next.Amount--
			s.sgLog("%s 发动名士，本次伤害减少一点", s.sgName(i))
		}
		if next.Amount > 0 {
			s.sgPush(next)
		}
	case "heg_suishi", "kuanggu":
		if a.Choice != "yes" && a.Choice != "pass" {
			return true, errors.New("请选择明置发动或放弃")
		}
		if a.Choice == "yes" {
			s.sgPush(e)
		}
	case "heg_lirang":
		if a.Choice == "pass" {
			return true, nil
		}
		if len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) || len(a.Cards) == 0 || !sgSubset(a.Cards, q.Cards) || !sgSubset(a.Cards, g.Discard) {
			return true, errors.New("请选择本次仍在弃牌堆中的牌，交给另一名角色")
		}
		for _, id := range a.Cards {
			g.Discard = sgRemove(g.Discard, id)
			e.Cards = sgRemove(e.Cards, id)
		}
		s.sgPush(e)
		s.sgGain(a.Targets[0], a.Cards)
		s.sgLog("%s 发动礼让，交给 %s %d 张弃牌", s.sgName(i), s.sgName(a.Targets[0]), len(a.Cards))
	case "heg_shuangren":
		if a.Choice == "pass" {
			return true, nil
		}
		if s.sgValidateCards(i, a.Cards, 1, true) != nil || len(a.Targets) != 1 || !slices.Contains(q.Targets, a.Targets[0]) || !s.sgAlive(a.Targets[0]) || len(g.Players[a.Targets[0]].Hand) == 0 {
			return true, errors.New("双刃需要自己一张手牌和另一名有手牌的角色")
		}
		s.sgAsk(a.Targets[0], "pindian", "双刃：选择一张手牌拼点", SGEvent{Actor: i, Target: a.Targets[0], Kind: "heg_shuangren", Cards: clone(a.Cards)})
	case "heg_shuangren_slash":
		if len(a.Targets) != 1 || !slices.Contains(q.Targets, a.Targets[0]) || !s.sgCanTargetRange(i, a.Targets[0], "slash", true) {
			return true, errors.New("请选择双刃可攻击的角色")
		}
		p := &g.Players[i]
		amount := 1 + p.Drank
		p.Drank = 0
		p.Used["keji_slash"]++
		s.sgPush(SGEvent{Type: "slash_start", Actor: i, Target: a.Targets[0], Kind: "slash", Amount: amount, Color: 3})
		s.sgLog("%s 发动双刃，对 %s 使用杀", s.sgName(i), s.sgName(a.Targets[0]))
	case "heg_duanchang":
		slot := 0
		if a.Choice == "deputy" {
			slot = 1
		} else if a.Choice != "head" {
			return true, errors.New("请选择主将或副将失去技能")
		}
		p := &g.Players[e.Target]
		hadBuqu := s.sgHas(e.Target, "buqu")
		p.Hegemony.Lost[slot] = true
		s.sgLog("%s 的%s因断肠失去技能", s.sgName(e.Target), []string{"主将", "副将"}[slot])
		if hadBuqu && !s.sgHas(e.Target, "buqu") {
			g.Discard = append(g.Discard, p.Buqu...)
			p.Buqu, p.BuquActive = nil, false
			if p.HP <= 0 {
				s.sgEnterDying(SGEvent{Actor: -1, Target: e.Target, Step: s.Turn})
			}
		}
	default:
		return false, nil
	}
	return true, nil
}
