package game

import "slices"

func (s *State) sgHegBotPlay(i int, enemies []int, add func(Action), cardsFor func(string) []Action) {
	if !s.sgHegemony() {
		return
	}
	g := s.Sanguosha
	p := g.Players[i]
	hand := clone(p.Hand)
	slices.SortStableFunc(hand, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
	all := append(clone(hand), p.Equip...)
	slices.SortStableFunc(all, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
	allies := []int{}
	for _, to := range s.sgOrder(s.sgNext(i)) {
		if to != i && s.sgHegBotAlly(i, to) {
			allies = append(allies, to)
		}
	}
	slices.SortStableFunc(allies, func(a, b int) int { return len(g.Players[a].Hand) - len(g.Players[b].Hand) })
	if s.sgHegMayInvoke(i, "heg_huoshui") && !s.sgHas(i, "heg_huoshui") {
		add(Action{Type: "sg_skill", Skill: "heg_huoshui"})
	}
	if s.sgHegMayInvoke(i, "heg_xiongyi") && p.Marks["heg_xiongyi"] == 0 && (len(allies) > 0 || p.HP <= 2) {
		add(Action{Type: "sg_skill", Skill: "heg_xiongyi"})
	}
	if s.sgHegMayInvoke(i, "heg_rende") && len(allies) > 0 && len(hand) > 0 {
		n := max(0, len(hand)-s.sgHandLimit(i))
		if p.HP < p.MaxHP && p.Used["heg_rende"] < 3 && len(hand) >= 3-p.Used["heg_rende"] {
			n = max(n, 3-p.Used["heg_rende"])
		}
		if n > 0 {
			add(Action{Type: "sg_skill", Skill: "heg_rende", Cards: hand[:min(n, len(hand))], Targets: allies[:1]})
		}
	}
	if s.sgHegMayInvoke(i, "heg_zhiheng") && p.Used["heg_zhiheng"] == 0 {
		ids := []int{}
		for _, id := range hand {
			kind := sgCard(id).Kind
			slot := SGCardTypes[kind].Slot
			if sgBotValue(id) <= 50 && (slot != "" && s.sgEquip(i, slot) != 0 || sgIsSlash(kind) && p.Used["slash"] > 0 || len(hand) > s.sgHandLimit(i)) {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			add(Action{Type: "sg_skill", Skill: "heg_zhiheng", Cards: ids[:min(p.MaxHP, len(ids))]})
		}
	}
	if s.sgHegMayInvoke(i, "heg_lijian") && p.Used["heg_lijian"] == 0 && len(all) > 0 {
		men := []int{}
		for _, to := range enemies {
			if s.sgMale(to) {
				men = append(men, to)
			}
		}
		if len(men) >= 2 {
			add(Action{Type: "sg_skill", Skill: "heg_lijian", Cards: all[:1], Targets: men[:2]})
		}
	}
	if s.sgHegMayInvoke(i, "heg_fenxun") && p.Used["heg_fenxun"] == 0 && len(all) > 0 && len(cardsFor("slash")) > 0 && (p.Used["slash"] == 0 || s.sgHas(i, "paoxiao") || s.sgWeapon(i) == "crossbow") {
		for _, cost := range all {
			if sgIsSlash(sgCard(cost).Kind) || sgBotValue(cost) > 50 {
				continue
			}
			for _, to := range enemies {
				if s.sgDistance(i, to) > s.sgRange(i) {
					add(Action{Type: "sg_skill", Skill: "heg_fenxun", Cards: []int{cost}, Targets: []int{to}})
				}
			}
		}
	}
	if s.sgHegMayInvoke(i, "heg_qingcheng") {
		for _, cost := range all {
			if SGCardTypes[sgCard(cost).Kind].Slot == "" {
				continue
			}
			for _, to := range enemies {
				h := g.Players[to].Hegemony
				if !h.Shown[0] || !h.Shown[1] {
					continue
				}
				slot, score := "head", -1
				for j, id := range s.sgHegGeneralIDs(to) { // Both slots are public.
					value := len(sgGeneral(id).Skills)
					if h.Lost[j] {
						value = 0
					}
					if value > score {
						slot, score = []string{"head", "deputy"}[j], value
					}
				}
				if score > 0 {
					add(Action{Type: "sg_skill", Skill: "heg_qingcheng", Cards: []int{cost}, Targets: []int{to}, Choice: slot})
				}
			}
		}
	}
	for _, a := range cardsFor("await_exhausted") {
		a.Type = "sg_play"
		add(a)
	}
	if s.sgHegMayInvoke(i, "heg_duoshi") && p.Used["heg_duoshi"] < 4 && len(allies) > 0 {
		for _, id := range hand {
			if sgBotValue(id) <= 50 {
				add(Action{Type: "sg_play", Skill: "heg_duoshi", Cards: []int{id}})
			}
		}
	}
	for _, kind := range []string{"befriend_attacking", "known_both"} {
		for _, a := range cardsFor(kind) {
			for _, to := range enemies {
				a.Type, a.Targets = "sg_play", []int{to}
				add(a)
			}
			if kind == "known_both" {
				a.Type, a.Targets = "sg_play", nil
				add(a)
			}
		}
	}
	for _, to := range allies {
		if g.Players[to].HP < g.Players[to].MaxHP && len(hand) > 0 {
			add(Action{Type: "sg_skill", Skill: "qingnang", Cards: hand[:1], Targets: []int{to}})
			if s.sgMale(to) && len(hand) >= 2 && p.HP < p.MaxHP {
				add(Action{Type: "sg_skill", Skill: "jieyin", Cards: hand[:2], Targets: []int{to}})
			}
		}
		if s.sgHegMayInvoke(i, "zhijian") {
			for _, id := range hand {
				if slot := SGCardTypes[sgCard(id).Kind].Slot; slot != "" && s.sgEquip(to, slot) == 0 {
					add(Action{Type: "sg_skill", Skill: "zhijian", Cards: []int{id}, Targets: []int{to}})
				}
			}
		}
		if s.sgHegMayInvoke(i, "dimeng") && p.Used["dimeng"] == 0 {
			for _, enemy := range enemies {
				due := len(g.Players[enemy].Hand) - len(g.Players[to].Hand)
				if due > 0 && due <= len(all) {
					add(Action{Type: "sg_skill", Skill: "dimeng", Cards: all[:due], Targets: []int{to, enemy}})
				}
			}
		}
	}
}
