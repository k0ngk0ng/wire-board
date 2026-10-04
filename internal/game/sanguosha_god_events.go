package game

import "slices"

func (s *State) sgGodDeathResponse(i int) bool {
	q := s.Sanguosha.Pending
	return i >= 0 && i < len(s.Sanguosha.Players) && q != nil && q.Player == i && q.Kind == "wuhun_target"
}
func (s *State) sgGodAddMark(i int, key string, n int) {
	if n == 0 {
		return
	}
	p := &s.Sanguosha.Players[i]
	if p.Marks == nil {
		p.Marks = map[string]int{}
	}
	p.Marks[key] += n
}
func (s *State) sgGodDrawCount(i, n int) int {
	if s.sgHas(i, "jie_yingzi") {
		n++
	}
	if s.sgHas(i, "juejing") {
		p := s.Sanguosha.Players[i]
		n += max(0, p.MaxHP-p.HP)
	}
	return n
}
func (s *State) sgGodArmorOff(i int) bool {
	for _, p := range s.Sanguosha.Players {
		if !p.Dead && slices.Contains(p.ArmorOff, i) {
			return true
		}
	}
	return false
}
func (s *State) sgGodGameStart() {
	es := []SGEvent{}
	for _, i := range s.sgOrder(s.Sanguosha.Lord) {
		es = append(es, SGEvent{Type: "god_kingdom", Actor: i})
	}
	for _, i := range s.sgOrder(s.Sanguosha.Lord) {
		es = append(es, SGEvent{Type: "huashen_init", Actor: i})
	}
	for _, i := range s.sgOrder(s.Sanguosha.Lord) {
		es = append(es, SGEvent{Type: "god_initial_draw", Actor: i})
	}
	es = append(es, SGEvent{Type: "begin", Actor: s.Sanguosha.Lord})
	s.sgPush(es...)
}
func (s *State) sgGodRoundStart(i int) {
	p := &s.Sanguosha.Players[i]
	p.Gale = nil
	p.Fog = nil
}
func (s *State) sgGodTurnCleanup() {
	for i := range s.Sanguosha.Players {
		p := &s.Sanguosha.Players[i]
		p.Temporary = nil
		p.ArmorOff = nil
	}
}
func (s *State) sgGodDeath(i, killer int) {
	p := &s.Sanguosha.Players[i]
	p.Gale = nil
	p.Fog = nil
	p.ArmorOff = nil
	p.Temporary = nil
	if s.sgAlive(killer) {
		s.Sanguosha.Players[killer].TurnKills++
	}
}
func (s *State) sgGodBeforeDamage(target, source, n int) {
	if s.sgHas(target, "wuhun") && source != target && s.sgAlive(source) {
		s.sgGodAddMark(source, "nightmare", n)
		s.sgLog("%s 因武魂获得 %d 梦魇", s.sgName(source), n)
	}
}
func (s *State) sgGodDamageDealt(e SGEvent) {
	if s.sgHas(e.Actor, "kuangbao") {
		s.sgGodAddMark(e.Actor, "wrath", e.Amount)
	}
}
func (s *State) sgGodHurt(e SGEvent) {
	i := e.Target
	if s.sgHas(i, "kuangbao") {
		s.sgGodAddMark(i, "wrath", e.Amount)
	}
	if s.sgHas(i, "renjie") {
		s.sgGodAddMark(i, "bear", e.Amount)
	}
	es := []SGEvent{}
	if s.sgHas(i, "guixin") {
		for range e.Amount {
			es = append(es, SGEvent{Type: "guixin", Actor: i})
		}
	}
	if s.sgHas(i, "jilve") {
		es = append(es, SGEvent{Type: "jilve_fangzhu", Actor: i})
	}
	s.sgPush(es...)
}
func (s *State) sgGodDiscard(i int, ids []int) {
	g := s.Sanguosha
	if !g.DiscardPhase || i != s.Turn {
		return
	}
	n := 0
	for _, id := range ids {
		if s.sgOwn(i, id, false) {
			n++
		}
	}
	g.DiscardedOwn += n
	if s.sgHas(i, "renjie") {
		s.sgGodAddMark(i, "bear", n)
	}
}
func (s *State) sgGodTrick(i int, kind string) {
	if !sgIsTrick(kind) {
		return
	}
	es := []SGEvent{}
	if !sgIsDelayed(kind) {
		es = append(es, SGEvent{Type: "wumou", Actor: i})
	}
	es = append(es, SGEvent{Type: "jie_jizhi", Actor: i}, SGEvent{Type: "jilve_jizhi", Actor: i})
	s.sgPush(es...)
}
func (s *State) sgGodEvent(e SGEvent) bool {
	if s.sgGodStarsEvent(e) || s.sgGodWarEvent(e) || s.sgGodSimaEvent(e) {
		return true
	}
	g := s.Sanguosha
	switch e.Type {
	case "god_kingdom":
		if sgGeneral(g.Players[e.Actor].General).Kingdom == "god" && g.Players[e.Actor].BaseKingdom == "" {
			s.sgAsk(e.Actor, "god_kingdom", "为神将选择本局势力", e)
			g.Pending.Choices = []string{"wei", "shu", "wu", "qun"}
		}
	case "god_avatar_kingdom":
		if s.sgHas(e.Actor, "huashen") && sgGeneral(g.Players[e.Actor].Avatar).Kingdom == "god" {
			s.sgAsk(e.Actor, "god_kingdom", "为当前神将化身选择势力", e)
			g.Pending.Choices = []string{"wei", "shu", "wu", "qun"}
		}
	case "god_initial_draw":
		if s.sgHas(e.Actor, "kuangbao") {
			s.sgGodAddMark(e.Actor, "wrath", 2)
		}
		n := 4
		if s.sgHas(e.Actor, "qixing") {
			n += 7
		}
		s.sgDraw(e.Actor, n)
		if n > 4 {
			s.sgAsk(e.Actor, "qixing_initial", "七星：从起手牌中选择七张置为星（仅自己可见）", e)
			g.Pending.Cards = clone(g.Players[e.Actor].Hand)
		}
	case "god_start":
		if s.sgHas(e.Actor, "baiyin") && g.Players[e.Actor].Marks["baiyin"] == 0 && g.Players[e.Actor].Marks["bear"] >= 4 {
			s.sgAwaken(e.Actor, "baiyin", -1, "jilve")
		}
	case "wuhun":
		high := 0
		targets := []int{}
		for _, i := range s.sgOrder(s.Turn) {
			n := g.Players[i].Marks["nightmare"]
			if n > high {
				high = n
				targets = nil
			}
			if n > 0 && n == high {
				targets = append(targets, i)
			}
		}
		if len(targets) > 0 {
			s.sgAsk(e.Actor, "wuhun_target", "武魂：选择梦魇最多的存活角色进行判定", e)
			g.Pending.Targets = targets
		}
	case "wuhun_clear":
		for i := range g.Players {
			delete(g.Players[i].Marks, "nightmare")
		}
	case "wuhun_resolve":
		s.sgPush(SGEvent{Type: "wuhun_clear"})
		if !e.Flag && s.sgAlive(e.Actor) {
			s.sgLog("%s 武魂判定失败，直接阵亡", s.sgName(e.Actor))
			s.sgDie(e.Actor, -1)
		}
	case "guixin":
		if !s.sgHas(e.Actor, "guixin") {
			return true
		}
		if !e.Flag {
			s.sgOptional(e.Actor, "guixin", e)
			return true
		}
		s.sgPush(SGEvent{Type: "guixin_take", Actor: e.Actor, Targets: sgRemove(s.sgOrder(s.Turn), e.Actor)})
	case "guixin_take":
		if !s.sgAlive(e.Actor) {
			return true
		}
		for e.Step < len(e.Targets) {
			to := e.Targets[e.Step]
			e.Step++
			p := g.Players[to]
			if s.sgAlive(to) && len(p.Hand)+len(p.Equip)+len(p.Judgment) > 0 {
				s.sgPush(e)
				s.sgAsk(e.Actor, "steal", "归心：取得该角色的一张手牌、装备或判定牌", SGEvent{Actor: e.Actor, Target: to, Kind: "guixin"})
				return true
			}
		}
		g.Players[e.Actor].Flipped = !g.Players[e.Actor].Flipped
		s.sgLog("%s 归心结算完毕，将武将牌翻面", s.sgName(e.Actor))
	default:
		return false
	}
	return true
}
