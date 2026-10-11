package game

import "slices"

// Support choices supplement the common combat candidates; only our own cards
// and teammates' public health/equipment/hand counts are consulted.
func (s *State) sgThreeBotSupport(i int, add func(Action)) {
	if !s.sgThreeV3() {
		return
	}
	g := s.Sanguosha
	p := g.Players[i]
	allies := s.sgThreeAliveMembers(s.sgThreeCamp(i))
	slices.SortStableFunc(allies, func(a, b int) int {
		priority := func(to int) int {
			value := (g.Players[to].MaxHP - g.Players[to].HP) * 10
			if s.sgThreeIsLeader(to) && g.Players[to].HP <= 2 {
				value += 100
			}
			return value
		}
		return priority(b) - priority(a)
	})
	hand := slices.Clone(p.Hand)
	slices.SortStableFunc(hand, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
	for _, to := range allies {
		if s.sgHas(i, "qingnang") && len(hand) > 0 && g.Players[to].HP < g.Players[to].MaxHP {
			add(Action{Type: "sg_skill", Skill: "qingnang", Cards: hand[:1], Targets: []int{to}})
		}
		if to == i {
			continue
		}
		if s.sgHas(i, "jieyin") && len(hand) >= 2 && !s.sgFemale(to) && g.Players[to].HP < g.Players[to].MaxHP {
			add(Action{Type: "sg_skill", Skill: "jieyin", Cards: hand[:2], Targets: []int{to}})
		}
		if s.sgHas(i, "zhijian") {
			for _, id := range hand {
				slot := SGCardTypes[sgCard(id).Kind].Slot
				if slot != "" && s.sgEquip(to, slot) == 0 && s.sgEquip(i, slot) != 0 {
					add(Action{Type: "sg_skill", Skill: "zhijian", Cards: []int{id}, Targets: []int{to}})
				}
			}
		}
		for _, skill := range []string{"rende", "jie_rende"} {
			if s.sgHas(i, skill) && len(hand) > 2 && len(g.Players[to].Hand) < 3 {
				add(Action{Type: "sg_skill", Skill: skill, Cards: hand[:min(2, len(hand)-2)], Targets: []int{to}})
			}
		}
	}
}
