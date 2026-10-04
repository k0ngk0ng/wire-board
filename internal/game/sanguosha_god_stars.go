package game

import (
	"errors"
	"slices"
)

func (s *State) sgGodClearStars(i int) {
	p := &s.Sanguosha.Players[i]
	s.Sanguosha.Discard = append(s.Sanguosha.Discard, p.Stars...)
	p.Stars = nil
}
func (s *State) sgGodForeseen(e *SGEvent) bool {
	gale, fog := false, false
	for _, p := range s.Sanguosha.Players {
		gale = gale || slices.Contains(p.Gale, e.Target)
		fog = fog || slices.Contains(p.Fog, e.Target)
	}
	if fog && e.Nature != "thunder" {
		s.sgLog("%s 的大雾防止了此次伤害", s.sgName(e.Target))
		return true
	}
	if gale && e.Nature == "fire" {
		e.Amount++
		s.sgLog("%s 受狂风影响，火焰伤害+1", s.sgName(e.Target))
	}
	return false
}
func (s *State) sgGodStarsEvent(e SGEvent) bool {
	g := s.Sanguosha
	switch e.Type {
	case "qixing_exchange":
		if !g.SkipDraw && s.sgHas(e.Actor, "qixing") && len(g.Players[e.Actor].Hand) > 0 && len(g.Players[e.Actor].Stars) > 0 {
			s.sgAsk(e.Actor, "qixing_exchange", "七星：选择等量手牌与星交换，或保留原状", e)
			g.Pending.Cards = clone(g.Players[e.Actor].Stars)
		}
	case "god_finish":
		s.sgPush(SGEvent{Type: "kuangfeng", Actor: e.Actor}, SGEvent{Type: "dawu", Actor: e.Actor})
	case "kuangfeng", "dawu":
		if s.sgHas(e.Actor, e.Type) && len(g.Players[e.Actor].Stars) > 0 {
			s.sgAsk(e.Actor, e.Type, "选择消耗的星与目标，或放弃发动「"+SGSkills[e.Type].Name+"」", e)
			g.Pending.Cards = clone(g.Players[e.Actor].Stars)
		}
	default:
		return false
	}
	return true
}
func (s *State) sgGodStarsRespond(i int, a Action, q SGPrompt) (bool, error) {
	g := s.Sanguosha
	p := &g.Players[i]
	switch q.Kind {
	case "god_kingdom":
		if !slices.Contains(q.Choices, a.Choice) {
			return true, errors.New("请选择魏、蜀、吴或群")
		}
		if q.Event.Type == "god_avatar_kingdom" {
			p.AvatarKingdom = a.Choice
		} else {
			p.BaseKingdom = a.Choice
		}
		s.sgLog("%s 选择势力：%s", s.sgName(i), map[string]string{"wei": "魏", "shu": "蜀", "wu": "吴", "qun": "群"}[a.Choice])
	case "qixing_initial":
		if s.sgValidateCards(i, a.Cards, min(7, len(p.Hand)), true) != nil {
			return true, errors.New("七星：请选择七张起手牌")
		}
		// Initial dealing is not an ordinary loss from hand and triggers no loss skills.
		for _, id := range a.Cards {
			p.Hand = sgRemove(p.Hand, id)
		}
		p.Stars = clone(a.Cards)
		s.sgLog("%s 将 %d 张起手牌置为星", s.sgName(i), len(a.Cards))
	case "qixing_exchange":
		if a.Choice == "pass" {
			return true, nil
		}
		if len(a.Cards) == 0 || len(a.Cards) != len(a.Take) || !sgSubset(a.Take, p.Stars) || s.sgValidateCards(i, a.Cards, -1, true) != nil {
			return true, errors.New("请选择等量手牌与星进行交换")
		}
		// Atomic exchange: Lianying observes the resulting hand, never a temporary empty hand.
		for _, id := range a.Cards {
			p.Hand = sgRemove(p.Hand, id)
		}
		for _, id := range a.Take {
			p.Stars = sgRemove(p.Stars, id)
		}
		p.Hand = append(p.Hand, a.Take...)
		p.Stars = append(p.Stars, a.Cards...)
		s.sgLog("%s 交换了 %d 张手牌与星", s.sgName(i), len(a.Cards))
	case "kuangfeng", "dawu":
		if a.Choice == "pass" {
			return true, nil
		}
		if len(a.Cards) == 0 || len(a.Cards) != len(a.Targets) || !sgSubset(a.Cards, p.Stars) || q.Kind == "kuangfeng" && len(a.Cards) != 1 {
			return true, errors.New("请选择等量的星和目标；狂风只能选择一名角色")
		}
		seen := map[int]bool{}
		for _, to := range a.Targets {
			if !s.sgAlive(to) || seen[to] {
				return true, errors.New("请选择不同的存活角色")
			}
			seen[to] = true
		}
		for _, id := range a.Cards {
			p.Stars = sgRemove(p.Stars, id)
		}
		g.Discard = append(g.Discard, a.Cards...)
		if q.Kind == "kuangfeng" {
			p.Gale = clone(a.Targets)
		} else {
			p.Fog = clone(a.Targets)
		}
		for _, to := range a.Targets {
			s.sgLog("%s 对 %s 发动「%s」", s.sgName(i), s.sgName(to), SGSkills[q.Kind].Name)
		}
	default:
		return false, nil
	}
	return true, nil
}
