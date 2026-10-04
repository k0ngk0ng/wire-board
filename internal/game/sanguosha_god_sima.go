package game

import "errors"

func (s *State) sgGodSimaSkill(i int, a Action) (bool, error) {
	if a.Skill != "jilve" {
		return false, nil
	}
	p := &s.Sanguosha.Players[i]
	if p.Marks["bear"] <= 0 {
		return true, errors.New("极略需要1忍")
	}
	if a.Choice != "zhiheng" && a.Choice != "wansha" {
		return true, errors.New("出牌阶段极略可选择制衡或完杀")
	}
	if p.Used["jilve_"+a.Choice] > 0 {
		return true, errors.New("本阶段已经发动此项极略")
	}
	if a.Choice == "zhiheng" && (len(a.Cards) == 0 || s.sgValidateCards(i, a.Cards, -1, false) != nil) {
		return true, errors.New("请选择要制衡的手牌或装备")
	}
	s.sgGodAddMark(i, "bear", -1)
	p.Used["jilve_"+a.Choice]++
	if a.Choice == "wansha" {
		p.Temporary = append(p.Temporary, "wansha")
	} else {
		s.sgPush(SGEvent{Type: "draw", Actor: i, Amount: len(a.Cards)})
		s.sgDiscard(i, a.Cards)
	}
	s.sgLog("%s 弃1忍发动极略·%s", s.sgName(i), SGSkills[a.Choice].Name)
	return true, nil
}
func (s *State) sgGodSimaEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "jilve_jizhi", "jilve_fangzhu":
		if s.sgHas(e.Actor, "jilve") && g.Players[e.Actor].Marks["bear"] > 0 {
			s.sgAsk(e.Actor, e.Type, "是否弃1忍发动极略·"+map[string]string{"jilve_jizhi": "集智", "jilve_fangzhu": "放逐"}[e.Type]+"？", e)
		}
	case "lianpo":
		// A stable seat cursor survives dying and nested effects during end phase.
		for e.Step < len(g.Players) {
			who := (s.Turn + e.Step) % len(g.Players)
			e.Step++
			if s.sgHas(who, "lianpo") && g.Players[who].TurnKills > 0 {
				s.sgAsk(who, "lianpo", "连破：本回合击杀过角色，是否获得额外回合？", e)
				return true
			}
		}
	default:
		return false
	}
	return true
}
func (s *State) sgGodSimaRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	e := q.Event
	p := &g.Players[i]
	switch q.Kind {
	case "jilve_guicai":
		if a.Choice == "pass" {
			s.sgPush(e)
			return true, nil
		}
		if p.Marks["bear"] <= 0 || s.sgValidateCards(i, a.Cards, 1, false) != nil {
			return true, errors.New("极略鬼才需要1忍和一张手牌或装备")
		}
		s.sgGodAddMark(i, "bear", -1)
		s.sgFinishCards([]int{e.Aux})
		e.Aux = a.Cards[0]
		s.sgPush(e)
		s.sgPay(i, a.Cards)
		s.sgLog("%s 弃1忍发动极略·鬼才，更换判定牌", s.sgName(i))
	case "jilve_jizhi":
		if a.Choice == "pass" {
			return true, nil
		}
		if a.Choice != "yes" || p.Marks["bear"] <= 0 {
			return true, errors.New("请选择发动或放弃极略集智")
		}
		s.sgGodAddMark(i, "bear", -1)
		ids := s.sgDrawIDs(1)
		if len(ids) == 0 {
			return true, nil
		}
		id := ids[0]
		s.sgPlaceTable(-1, ids)
		s.sgLog("%s 极略·集智亮出「%s」", s.sgName(i), SGCardTypes[sgCard(id).Kind].Name)
		if !sgBasic(sgCard(id).Kind) {
			s.sgTakeTable(id)
			p.Hand = append(p.Hand, id)
		} else if len(p.Hand) > 0 {
			s.sgAsk(i, "jilve_jizhi_exchange", "集智：可将一张手牌置于牌堆顶，获得亮出的基本牌；否则弃置此牌", SGEvent{Actor: i, Cards: ids})
			g.Pending.Cards = ids
		} else {
			s.sgFinishCards(ids)
		}
	case "jilve_jizhi_exchange":
		if a.Choice == "pass" {
			s.sgFinishCards(e.Cards)
			return true, nil
		}
		if s.sgValidateCards(i, a.Cards, 1, true) != nil {
			return true, errors.New("选择一张手牌置于牌堆顶")
		}
		// Both movements are one exchange. Lianying must not see an empty intermediate hand.
		p.Hand = sgRemove(p.Hand, a.Cards[0])
		p.Hand = append(p.Hand, e.Cards[0])
		s.sgTakeTable(e.Cards[0])
		g.Deck = append(clone(a.Cards), g.Deck...)
		s.sgTuntianLoss(i, true)
	case "jilve_fangzhu":
		if a.Choice == "pass" {
			return true, nil
		}
		if p.Marks["bear"] <= 0 || len(a.Targets) != 1 || a.Targets[0] == i || !s.sgAlive(a.Targets[0]) {
			return true, errors.New("极略放逐需要1忍，并选择另一角色")
		}
		s.sgGodAddMark(i, "bear", -1)
		to := a.Targets[0]
		s.sgDraw(to, max(0, p.MaxHP-p.HP))
		g.Players[to].Flipped = !g.Players[to].Flipped
		s.sgLog("%s 极略·放逐令 %s 翻面", s.sgName(i), s.sgName(to))
	case "lianpo":
		if a.Choice != "pass" && a.Choice != "yes" {
			return true, errors.New("请选择发动或放弃连破")
		}
		s.sgPush(e)
		p.TurnKills = 0
		if a.Choice == "yes" {
			g.ExtraTurns = append([]int{i}, g.ExtraTurns...)
			s.sgLog("%s 发动连破，获得额外回合", s.sgName(i))
		}
	default:
		return false, nil
	}
	return true, nil
}
