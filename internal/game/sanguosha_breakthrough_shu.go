package game

import (
	"errors"
	"slices"
)

func sgCardCategory(kind string) int {
	if sgBasic(kind) {
		return 1
	}
	if SGCardTypes[kind].Slot != "" {
		return 3
	}
	return 2
}

// Call only for committed use/response, before payment removes hand cards.
// Pindian, retrials, recasting and discard/give costs deliberately do not call it.
func (s *State) sgJieCardUsed(i int, kind string, ids []int) {
	if i == s.Turn || len(ids) == 0 || !s.sgHas(i, "yajiao") {
		return
	}
	for _, id := range ids {
		v := s.Sanguosha.Virtual
		if !s.sgOwn(i, id, true) && !(v != nil && v.Player == i && v.Card == id && v.Paid) {
			return
		}
	}
	s.sgPush(SGEvent{Type: "yajiao", Actor: i, Aux: sgCardCategory(kind)})
}

func (s *State) sgJieShuEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "jie_start":
		if !s.sgAlive(e.Actor) {
			return true
		}
		p := &g.Players[e.Actor]
		threshold := 3
		if len(g.Players) >= 7 {
			threshold = 2
		}
		if s.sgHas(e.Actor, "qinxue") && p.Marks["qinxue"] == 0 && len(p.Hand)-p.HP >= threshold {
			s.sgAwaken(e.Actor, "qinxue", -1, "gongxin")
		}
		if s.sgHas(e.Actor, "tishen") && p.Marks["tishen"] == 0 && p.PreviousHPSet && p.HP < min(p.PreviousHP, p.MaxHP) {
			s.sgAsk(e.Actor, "tishen", "替身：整局限一次，回复至自己上回合结束体力并摸等量牌", e)
		}
	case "jie_rende_continue":
		if !s.sgHas(e.Actor, "jie_rende") {
			return true
		}
		e.Cards = slices.DeleteFunc(clone(e.Cards), func(id int) bool { return !s.sgOwn(e.Actor, id, true) })
		if len(e.Cards) > 0 {
			s.sgAsk(e.Actor, "jie_rende", "仁德：继续分配发动时已有的手牌，或结束本阶段仁德", e)
			g.Pending.Cards = clone(e.Cards)
		}
	case "yajiao":
		if !s.sgHas(e.Actor, "yajiao") {
			return true
		}
		if !e.Flag {
			s.sgOptional(e.Actor, "yajiao", e)
			return true
		}
		ids := s.sgDrawIDs(1)
		if len(ids) == 0 {
			return true
		}
		s.sgPlaceTable(-1, ids)
		e.Cards = ids
		e.Flag = sgCardCategory(sgCard(ids[0]).Kind) == e.Aux
		s.sgLog("%s 涯角亮出「%s」", s.sgName(e.Actor), SGCardTypes[sgCard(ids[0]).Kind].Name)
		message := "涯角：类别不同，可弃置亮牌或放回牌堆顶"
		if e.Flag {
			message = "涯角：类别相同，可交给任意存活角色或放回牌堆顶"
		}
		s.sgAsk(e.Actor, "yajiao", message, e)
		g.Pending.Cards = ids
		g.Pending.Choices = []string{"discard", "pass"}
		if e.Flag {
			g.Pending.Choices = []string{"give", "pass"}
			g.Pending.Targets = s.sgOrder(s.Turn)
		}
	case "jie_jizhi_reveal":
		s.sgJizhiReveal(e.Actor)
	case "jie_jizhi":
		if !e.Flag {
			s.sgOptional(e.Actor, "jie_jizhi", e)
		} else {
			s.sgJizhiReveal(e.Actor)
		}
	case "jie_ganglie_black":
		if s.sgAlive(e.Actor) && len(s.sgDiscardable(e.Actor, e.Target, false)) > 0 {
			s.sgAsk(e.Actor, "steal", "刚烈：弃置来源一张手牌或装备", SGEvent{Actor: e.Actor, Target: e.Target, Kind: "jie_ganglie"})
		}
	default:
		return s.sgJieWuEvent(e)
	}
	return true
}

func (s *State) sgJieShuRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	e := q.Event
	p := &g.Players[i]
	pass := a.Choice == "pass"
	switch q.Kind {
	case "tishen":
		if pass {
			return true, nil
		}
		if a.Choice != "yes" {
			return true, errors.New("请选择发动或放弃替身")
		}
		s.sgMark(i, "tishen")
		amount := min(p.MaxHP, p.PreviousHP) - p.HP
		s.sgHeal(i, amount)
		s.sgDraw(i, amount)
	case "jie_rende":
		if pass {
			return true, nil
		}
		return true, s.sgJieRende(i, a, e.Cards)
	case "yijue_heal":
		if pass {
			return true, nil
		}
		if a.Choice != "yes" {
			return true, errors.New("请选择回复或放弃")
		}
		s.sgHeal(e.Target, 1)
	case "jie_tieji":
		if pass {
			s.sgPush(s.sgSlashResponse(e))
			return true, nil
		}
		if a.Choice != "yes" {
			return true, errors.New("请选择发动或放弃铁骑")
		}
		g.Players[e.Target].Silenced = true
		s.sgPush(SGEvent{Type: "judge", Actor: i, Target: e.Target, Kind: "jie_tieji", Next: &e})
	case "jie_tieji_discard":
		if pass {
			next := *e.Next
			next.Type = "hit"
			s.sgPush(next)
			return true, nil
		}
		if s.sgValidateCards(i, a.Cards, 1, false) != nil || s.sgCardFor(i, a.Cards[0]).Suit != e.Color {
			return true, errors.New("铁骑需要弃置与判定花色相同的手牌或装备")
		}
		s.sgPush(s.sgSlashResponse(*e.Next))
		s.sgDiscard(i, a.Cards)
	case "yajiao":
		if !slices.Contains(q.Choices, a.Choice) {
			return true, errors.New("请选择涯角的处理方式")
		}
		id := e.Cards[0]
		if a.Choice == "give" {
			if len(a.Targets) != 1 || !s.sgAlive(a.Targets[0]) {
				return true, errors.New("请选择涯角牌的获得者")
			}
			s.sgTakeTable(id)
			s.sgGain(a.Targets[0], []int{id})
		} else if a.Choice == "discard" {
			s.sgFinishCards(e.Cards)
		} else {
			s.sgTakeTable(id)
			g.Deck = append([]int{id}, g.Deck...)
		}
	default:
		return s.sgJieWuRespond(i, a, q)
	}
	return true, nil
}

func (s *State) sgJieRende(i int, a Action, available []int) error {
	if len(a.Cards) == 0 || !sgSubset(a.Cards, available) || s.sgValidateCards(i, a.Cards, -1, true) != nil || len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) {
		return errors.New("仁德只能将发动时已有的手牌交给另一名存活角色")
	}
	p := &s.Sanguosha.Players[i]
	remaining := clone(available)
	for _, id := range a.Cards {
		remaining = sgRemove(remaining, id)
	}
	s.sgPush(SGEvent{Type: "jie_rende_continue", Actor: i, Cards: remaining})
	before := p.Used["jie_rende_given"]
	p.Used["jie_rende_given"] += len(a.Cards)
	s.sgLose(i, a.Cards)
	s.sgGain(a.Targets[0], a.Cards)
	if before < 2 && p.Used["jie_rende_given"] >= 2 {
		s.sgHeal(i, 1)
	}
	s.sgLog("%s 仁德交给 %s %d 张牌", s.sgName(i), s.sgName(a.Targets[0]), len(a.Cards))
	return nil
}

// Shared by the revised Huang Yueying and the pinned Jilve reference.
func (s *State) sgJizhiReveal(i int) {
	if !s.sgAlive(i) {
		return
	}
	ids := s.sgDrawIDs(1)
	if len(ids) == 0 {
		return
	}
	id := ids[0]
	s.sgPlaceTable(-1, ids)
	s.sgLog("%s 集智亮出「%s」", s.sgName(i), SGCardTypes[sgCard(id).Kind].Name)
	if !sgBasic(sgCard(id).Kind) {
		s.sgTakeTable(id)
		s.sgGain(i, ids)
	} else if len(s.Sanguosha.Players[i].Hand) > 0 {
		s.sgAsk(i, "jilve_jizhi_exchange", "集智：可将一张手牌置于牌堆顶，获得亮出的基本牌；否则弃置此牌", SGEvent{Actor: i, Cards: ids})
		s.Sanguosha.Pending.Cards = ids
	} else {
		s.sgFinishCards(ids)
	}
}
