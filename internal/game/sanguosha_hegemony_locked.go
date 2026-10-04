package game

import "errors"

// A compulsory shown skill resolves immediately; while hidden the owner may
// reveal it. Both branches are persistent events, so passing cannot lose the
// enclosing damage/card/judgment and a restart cannot repeat the offer.
func (s *State) sgHegGate(i int, skill string, yes, no SGEvent) bool {
	if s.sgHas(i, skill) {
		s.sgPush(yes)
		return true
	}
	if !s.sgHegMayInvoke(i, skill) {
		return false
	}
	s.sgAsk(i, "heg_invoke", "是否明置武将并发动「"+SGSkills[skill].Name+"」？", SGEvent{Actor: i, Kind: skill, Next: &yes, Fallback: &no})
	s.Sanguosha.Pending.Choices = []string{"yes", "pass"}
	return true
}

func (s *State) sgHegLockedRespond(_ int, a Action, q SGPrompt) (bool, error) {
	if q.Kind != "heg_invoke" {
		return false, nil
	}
	if a.Choice != "yes" && a.Choice != "pass" {
		return true, errors.New("请选择明置发动或放弃")
	}
	next := q.Event.Next
	if a.Choice == "pass" {
		next = q.Event.Fallback
	}
	if next != nil {
		s.sgPush(*next)
	}
	return true, nil
}

func (s *State) sgHegLockedEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "heg_cancel_target":
		s.sgLog("%s 发动%s，取消本次牌对自己的目标", s.sgName(e.Actor), SGSkills[e.Kind].Name)
		return true
	case "slash_start", "null_window", "effect":
		if !s.sgAlive(e.Target) || e.Aux == -2 {
			return false
		}
		if e.Type != "effect" && (sgIsSlash(e.Kind) || e.Kind == "duel") {
			owners := []int{e.Actor}
			if e.Kind == "duel" {
				owners = append(owners, e.Target)
			}
			for _, who := range owners {
				if who >= 0 && e.HegWushuang&(1<<who) == 0 && !s.sgHas(who, "wushuang") && s.sgHegMayInvoke(who, "wushuang") {
					e.HegWushuang |= 1 << who
					return s.sgHegGate(who, "wushuang", e, e)
				}
			}
		}
		checks := []struct {
			skill string
			bit   int
			when  bool
		}{
			{"kongcheng", 1, (sgIsSlash(e.Kind) || e.Kind == "duel") && len(g.Players[e.Target].Hand) == 0},
			{"qianxun", 2, e.Kind == "snatch" || e.Kind == "indulgence"},
			{"weimu", 4, sgIsTrick(e.Kind) && e.Color == 2},
			{"huoshou", 8, e.Type == "effect" && e.Kind == "savage_assault"},
			{"juxiang", 16, e.Type == "effect" && e.Kind == "savage_assault"},
		}
		for _, check := range checks {
			if check.when && e.HegProtection&check.bit == 0 && s.sgHegMayInvoke(e.Target, check.skill) {
				e.HegProtection |= check.bit
				return s.sgHegGate(e.Target, check.skill, SGEvent{Type: "heg_cancel_target", Actor: e.Target, Kind: check.skill}, e)
			}
		}
	case "xiangle":
		if !s.sgHas(e.Target, "xiangle") && s.sgHegMayInvoke(e.Target, "xiangle") {
			no := e
			no.Type = "slash_weapon"
			return s.sgHegGate(e.Target, "xiangle", e, no)
		}
	case "judge_result":
		if !e.HegHongyanChecked && s.sgCardFor(e.Actor, e.Aux).Suit == 0 && s.sgHegMayInvoke(e.Actor, "hongyan") {
			e.HegHongyanChecked = true
			return s.sgHegGate(e.Actor, "hongyan", e, e)
		}
	case "dying":
		if !e.HegWanshaChecked && s.sgAlive(e.Target) && g.Players[e.Target].HP <= 0 && !s.sgHas(s.Turn, "wansha") && s.sgHegMayInvoke(s.Turn, "wansha") {
			e.HegWanshaChecked = true
			return s.sgHegGate(s.Turn, "wansha", e, e)
		}
	}
	return false
}
