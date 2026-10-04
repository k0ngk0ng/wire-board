package game

import (
	"errors"
	"slices"
)

func (s *State) sgHegTriggerEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "heg_start":
		if s.sgHegMayInvoke(e.Actor, "heg_shenzhi") && len(g.Players[e.Actor].Hand) > 0 {
			s.sgAsk(e.Actor, "heg_shenzhi", "神智：可弃全部手牌，弃牌数不少于当前体力时回复一点体力", e)
			g.Pending.Choices = []string{"yes", "pass"}
		}
	case "heg_finish":
		es := []SGEvent{}
		for _, who := range s.sgOrder(s.Turn) {
			if who != e.Actor {
				es = append(es, SGEvent{Type: "heg_xiaoguo", Actor: who, Target: e.Actor})
			}
		}
		s.sgPush(es...)
	case "heg_xiaoguo":
		if !s.sgHegMayInvoke(e.Actor, "heg_xiaoguo") || !s.sgAlive(e.Target) {
			return true
		}
		cards := []int{}
		for _, id := range g.Players[e.Actor].Hand {
			if sgBasic(s.sgCardFor(e.Actor, id).Kind) {
				cards = append(cards, id)
			}
		}
		if len(cards) > 0 {
			s.sgAsk(e.Actor, "heg_xiaoguo", "骁果：可弃一张基本手牌，令结束回合的角色弃装备或受伤", e)
			g.Pending.Cards = cards
		}
	case "heg_xiaoguo_discard":
		if s.sgAlive(e.Target) {
			s.sgAsk(e.Target, "heg_xiaoguo_discard", "骁果：弃一张装备牌，否则受到一点伤害", e)
		}
	case "heg_shushen":
		if !s.sgHegMayInvoke(e.Actor, "heg_shushen") {
			return true
		}
		targets := []int{}
		for _, who := range s.sgOrder(s.Turn) {
			if who != e.Actor && s.sgHegWouldFriend(e.Actor, who) {
				targets = append(targets, who)
			}
		}
		if len(targets) > 0 {
			s.sgAsk(e.Actor, "heg_shushen", "淑慎：可让另一名同势力角色摸一张牌", e)
			g.Pending.Targets = targets
		}
	case "heg_sijian":
		if !s.sgHegMayInvoke(e.Actor, "heg_sijian") {
			return true
		}
		targets := []int{}
		for _, who := range s.sgOrder(s.Turn) {
			if who != e.Actor && len(s.sgDiscardable(e.Actor, who, false)) > 0 {
				targets = append(targets, who)
			}
		}
		if len(targets) > 0 {
			s.sgAsk(e.Actor, "heg_sijian", "死谏：可选择另一名角色，弃置其一张手牌或装备", e)
			g.Pending.Targets = targets
		}
	case "heg_kuangfu":
		if !s.sgHegMayInvoke(e.Actor, "heg_kuangfu") || !s.sgAlive(e.Target) {
			return true
		}
		cards := []int{}
		for _, id := range g.Players[e.Target].Equip {
			if s.sgCanDiscard(e.Actor, e.Target, id) || s.sgEquip(e.Actor, SGCardTypes[sgCard(id).Kind].Slot) == 0 {
				cards = append(cards, id)
			}
		}
		if len(cards) > 0 {
			s.sgAsk(e.Actor, "heg_kuangfu", "狂斧：可选择目标一张装备，弃置或移入自己空着的对应装备栏", e)
			g.Pending.Cards = cards
			g.Pending.Choices = []string{"discard", "move", "pass"}
		}
	default:
		return false
	}
	return true
}

func (s *State) sgHegTriggerRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	e := q.Event
	p := &g.Players[i]
	switch q.Kind {
	case "heg_shenzhi":
		if a.Choice == "pass" {
			return true, nil
		}
		if a.Choice != "yes" {
			return true, errors.New("请选择发动或放弃神智")
		}
		cards := clone(p.Hand)
		if len(cards) == 0 {
			return true, errors.New("没有手牌不能发动神智")
		}
		if len(cards) >= p.HP {
			s.sgPush(SGEvent{Type: "heal", Actor: i, Amount: 1})
		}
		s.sgDiscard(i, cards)
	case "heg_xiaoguo":
		if a.Choice == "pass" {
			return true, nil
		}
		if s.sgValidateCards(i, a.Cards, 1, true) != nil || !slices.Contains(q.Cards, a.Cards[0]) {
			return true, errors.New("骁果需要一张基本手牌")
		}
		e.Type = "heg_xiaoguo_discard"
		s.sgPush(e)
		s.sgDiscard(i, a.Cards)
	case "heg_xiaoguo_discard":
		if a.Choice == "pass" {
			s.sgPush(SGEvent{Type: "damage", Actor: e.Actor, Target: i, Kind: "heg_xiaoguo", Amount: 1})
			return true, nil
		}
		if s.sgValidateCards(i, a.Cards, 1, false) != nil || SGCardTypes[sgCard(a.Cards[0]).Kind].Slot == "" {
			return true, errors.New("请弃置一张装备牌，或放弃并受伤")
		}
		s.sgDiscard(i, a.Cards)
	case "heg_shushen", "heg_sijian":
		if a.Choice == "pass" {
			return true, nil
		}
		if len(a.Targets) != 1 || !slices.Contains(q.Targets, a.Targets[0]) || !s.sgAlive(a.Targets[0]) {
			return true, errors.New("请选择提示中的合法目标")
		}
		if q.Kind == "heg_shushen" {
			s.sgDraw(a.Targets[0], 1)
		} else {
			s.sgAsk(i, "steal", "死谏：弃置目标的一张手牌或装备", SGEvent{Actor: i, Target: a.Targets[0], Kind: "heg_sijian"})
		}
	case "heg_kuangfu":
		if a.Choice == "pass" {
			return true, nil
		}
		if len(a.Cards) != 1 || !slices.Contains(q.Cards, a.Cards[0]) || !slices.Contains(g.Players[e.Target].Equip, a.Cards[0]) {
			return true, errors.New("请选择狂斧目标装备")
		}
		id := a.Cards[0]
		switch a.Choice {
		case "discard":
			if !s.sgCanDiscard(i, e.Target, id) {
				return true, errors.New("不能弃置该装备")
			}
			s.sgDiscard(e.Target, []int{id})
		case "move":
			if s.sgEquip(i, SGCardTypes[sgCard(id).Kind].Slot) != 0 {
				return true, errors.New("对应装备栏须为空")
			}
			s.sgLose(e.Target, []int{id})
			p.Equip = append(p.Equip, id)
		default:
			return true, errors.New("请选择移动、弃置或放弃")
		}
	default:
		return false, nil
	}
	return true, nil
}
