package game

import (
	"errors"
	"slices"
)

func (s *State) sgJieEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "qingjian":
		if !s.sgHas(e.Actor, "qingjian") {
			return true
		}
		e.Cards = slices.DeleteFunc(clone(e.Cards), func(id int) bool { return !s.sgOwn(e.Actor, id, true) })
		if len(e.Cards) > 0 {
			s.sgAsk(e.Actor, "qingjian", "清俭：可将本次获得的牌分给其他角色，或结束分配", e)
			g.Pending.Cards = clone(e.Cards)
		}
	case "jie_lianying":
		if s.sgHas(e.Actor, "jie_lianying") && e.Amount > 0 {
			s.sgAsk(e.Actor, "jie_lianying", "连营：选择不超过失去手牌数的不同角色，各摸一张", e)
			g.Pending.Targets = s.sgOrder(s.Turn)
		}
	case "jie_luoyi":
		if g.SkipDraw || !s.sgHas(e.Actor, "jie_luoyi") {
			return true
		}
		if !e.Flag {
			s.sgOptional(e.Actor, "jie_luoyi", e)
			return true
		}
		if e.Flag {
			g.SkipDraw = true
			g.Players[e.Actor].JieLuoyi = true
			ids := s.sgDrawIDs(3)
			g.Revealed = clone(ids)
			s.sgPlaceTable(-1, ids)
			got := []int{}
			for _, id := range ids {
				c := sgCard(id)
				if sgBasic(c.Kind) || c.Kind == "duel" || SGCardTypes[c.Kind].Slot == "weapon" {
					s.sgTakeTable(id)
					got = append(got, id)
				}
			}
			s.sgFinishCards(ids)
			s.sgGain(e.Actor, got)
			s.sgLog("%s 裸衣亮出三张牌，获得其中 %d 张；跳过摸牌阶段", s.sgName(e.Actor), len(got))
		}
	case "jie_tuxi_take":
		if s.sgAlive(e.Actor) && s.sgAlive(e.Target) && len(g.Players[e.Target].Hand) > 0 {
			ids := clone(g.Players[e.Target].Hand)
			shuffle(ids)
			s.sgLose(e.Target, ids[:1])
			s.sgGain(e.Actor, ids[:1])
			s.sgLog("%s 突袭获得 %s 一张手牌", s.sgName(e.Actor), s.sgName(e.Target))
		}
	case "jie_hurt":
		if !s.sgHas(e.Target, e.Kind) || e.Amount <= 0 {
			return true
		}
		if e.Kind == "jie_jianxiong" {
			s.sgAsk(e.Target, "jie_jianxiong", "奸雄：获得伤害牌，或摸一张牌，或放弃", e)
			g.Pending.Choices = []string{"draw", "pass"}
			if len(e.Cards) > 0 && sgSubset(e.Cards, g.Table) {
				g.Pending.Choices = append([]string{"take"}, g.Pending.Choices...)
			}
			return true
		}
		if (e.Kind == "jie_fankui" || e.Kind == "jie_ganglie") && !s.sgAlive(e.Actor) {
			return true
		}
		if e.Kind == "jie_fankui" && len(g.Players[e.Actor].Hand)+len(g.Players[e.Actor].Equip) == 0 {
			return true
		}
		if !e.Flag {
			s.sgOptional(e.Target, e.Kind, e)
			return true
		}
		next := e
		next.Flag = false
		next.Amount--
		s.sgPush(next)
		switch e.Kind {
		case "jie_fankui":
			s.sgAsk(e.Target, "steal", "反馈：获得来源一张手牌或装备", SGEvent{Actor: e.Target, Target: e.Actor, Kind: e.Kind})
		case "jie_ganglie":
			s.sgPush(SGEvent{Type: "judge", Actor: e.Target, Target: e.Actor, Kind: e.Kind})
		case "jie_yiji":
			s.sgPush(SGEvent{Type: "jie_yiji_targets", Actor: e.Target})
			s.sgDraw(e.Target, 2)
		}
	case "jie_yiji_targets":
		if s.sgAlive(e.Actor) && len(g.Players[e.Actor].Hand) > 0 {
			s.sgAsk(e.Actor, "jie_yiji_targets", "遗计：可选至多两名其他角色，各扣置一至两张手牌，或放弃分配", e)
			g.Pending.Targets = sgRemove(s.sgOrder(s.Turn), e.Actor)
		}
	case "jie_yiji_give":
		if !s.sgAlive(e.Actor) || len(g.Players[e.Actor].Hand) == 0 {
			return true
		}
		for e.Step < len(e.Targets) {
			e.Target = e.Targets[e.Step]
			e.Step++
			if !s.sgAlive(e.Target) {
				continue
			}
			e.Amount = min(2, len(g.Players[e.Actor].Hand))
			if len(g.Players[e.Actor].Hand) == 2 && len(e.Targets) == 2 && e.Step == 1 && s.sgAlive(e.Targets[1]) {
				e.Amount = 1
			}
			if e.Amount < 1 {
				return true
			}
			s.sgAsk(e.Actor, "jie_yiji_give", "遗计：为当前目标扣置一至两张手牌（须为下一目标留牌）", e)
			return true
		}
	default:
		return s.sgJieShuEvent(e)
	}
	return true
}

func (s *State) sgJieRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	e := q.Event
	pass := a.Choice == "pass"
	switch q.Kind {
	case "qingjian":
		if pass {
			return true, nil
		}
		if len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) || len(a.Cards) == 0 || !sgSubset(a.Cards, e.Cards) || s.sgValidateCards(i, a.Cards, -1, true) != nil {
			return true, errors.New("清俭需要从本次所得中选牌，交给另一名存活角色")
		}
		for _, id := range a.Cards {
			e.Cards = sgRemove(e.Cards, id)
		}
		s.sgPush(e)
		s.sgLose(i, a.Cards)
		s.sgGain(a.Targets[0], a.Cards)
		s.sgLog("%s 清俭：交给 %s %d 张牌", s.sgName(i), s.sgName(a.Targets[0]), len(a.Cards))
	case "jie_lianying":
		if pass {
			return true, nil
		}
		if len(a.Targets) == 0 || len(a.Targets) > e.Amount || !sgSubset(a.Targets, s.sgOrder(0)) {
			return true, errors.New("连营需要不超过失去手牌数的不同存活角色")
		}
		es := []SGEvent{}
		for _, who := range s.sgOrder(s.Turn) {
			if slices.Contains(a.Targets, who) {
				es = append(es, SGEvent{Type: "draw", Actor: who, Amount: 1})
			}
		}
		s.sgPush(es...)
	case "jie_jianxiong":
		if !slices.Contains(q.Choices, a.Choice) {
			return true, errors.New("请选择获得伤害牌、摸牌或放弃")
		}
		if a.Choice == "draw" {
			s.sgDraw(i, 1)
		}
		if a.Choice == "take" {
			for _, id := range e.Cards {
				s.sgTakeTable(id)
			}
			s.sgGain(i, e.Cards)
		}
	case "jie_guicai":
		if !pass {
			if s.sgValidateCards(i, a.Cards, 1, false) != nil {
				return true, errors.New("鬼才需要一张手牌或装备")
			}
			s.sgFinishCards([]int{e.Aux})
			e.Aux = a.Cards[0]
			s.sgPush(e)
			s.sgPay(i, a.Cards)
			s.sgLog("%s 发动界鬼才，更换判定牌", s.sgName(i))
		} else {
			s.sgPush(e)
		}
	case "jie_yiji_targets":
		if pass {
			return true, nil
		}
		if len(a.Targets) == 0 || len(a.Targets) > 2 || !sgSubset(a.Targets, q.Targets) {
			return true, errors.New("遗计需要一至两名其他角色，且须有足够手牌")
		}
		e.Targets = clone(a.Targets)
		e.Type = "jie_yiji_give"
		s.sgPush(e)
	case "jie_yiji_give":
		if len(a.Cards) < 1 || len(a.Cards) > e.Amount || s.sgValidateCards(i, a.Cards, -1, true) != nil {
			return true, errors.New("请选择规定数量的手牌扣置给当前遗计目标")
		}
		s.sgPush(e)
		s.sgLose(i, a.Cards)
		g.Players[e.Target].Yiji = append(g.Players[e.Target].Yiji, a.Cards...)
		s.sgLog("%s 为 %s 扣置 %d 张遗计牌，下次进入摸牌阶段时取回", s.sgName(i), s.sgName(e.Target), len(a.Cards))
	default:
		return s.sgJieShuRespond(i, a, q)
	}
	return true, nil
}

func (s *State) sgJieHurt(e SGEvent) []SGEvent {
	out := []SGEvent{}
	for _, skill := range []string{"jie_jianxiong", "jie_fankui", "jie_ganglie", "jie_yiji"} {
		if s.sgHas(e.Target, skill) {
			x := e
			x.Type, x.Kind, x.Flag = "jie_hurt", skill, false
			out = append(out, x)
		}
	}
	return out
}
