package game

import "slices"

func (s *State) sgMountainBotPrompt(i int, add func(Action), cardsFor func(string) []Action) {
	g := s.Sanguosha
	p := g.Players[i]
	q := g.Pending
	e := q.Event
	hand := clone(p.Hand)
	slices.SortStableFunc(hand, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
	all := append(clone(hand), p.Equip...)
	slices.SortStableFunc(all, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
	switch q.Kind {
	case "huashen":
		type pick struct {
			avatar, skill string
			value         int
		}
		picks := []pick{}
		for _, avatar := range p.Avatars {
			skills := sgAvatarSkills(avatar)
			if len(skills) == 0 {
				skills = []string{""}
			}
			for _, skill := range skills {
				v := 20
				for n, k := range []string{"yingzi", "guanxing", "jizhi", "paoxiao", "wusheng", "longdan", "kanpo", "bazhen", "yiji", "jieming", "yinghun", "haoshi", "tuntian"} {
					if skill == k {
						v = 100 - n
					}
				}
				if slices.Contains([]string{"benghuai", "duanchang"}, skill) {
					v = -50
				}
				picks = append(picks, pick{avatar, skill, v})
			}
		}
		slices.SortStableFunc(picks, func(a, b pick) int { return b.value - a.value })
		for _, pick := range picks {
			add(Action{Choice: pick.avatar, Skill: pick.skill})
		}
	case "zhiji":
		if p.HP < p.MaxHP && p.HP < 2 {
			add(Action{Choice: "heal"})
		}
		add(Action{Choice: "draw"})
	case "qiaobian":
		use := e.Kind == "judge" && len(p.Judgment) > 0 || e.Kind == "discard" && len(p.Hand)-s.sgHandLimit(i) > 1
		if e.Kind == "draw" && !g.SkipDraw {
			count := 0
			for _, who := range s.sgOrder(0) {
				if who != i && len(g.Players[who].Hand) > 0 && !(p.Role == "loyalist" && who == g.Lord) {
					count++
				}
			}
			use = count >= 2
		}
		if use && len(hand) > 0 {
			add(Action{Cards: hand[:1]})
		}
	case "qiaobian_draw":
		targets := []int{}
		for _, who := range q.Targets {
			if !(p.Role == "loyalist" && who == g.Lord) {
				targets = append(targets, who)
			}
		}
		if len(targets) > 0 {
			add(Action{Targets: targets[:min(2, len(targets))]})
		}
	case "qiaobian_move":
		for _, move := range s.sgQiaobianMoves(i) {
			if move.Targets[1] == i && SGCardTypes[sgCard(move.Card).Kind].Slot != "" {
				add(move)
			}
		}
	case "fangquan":
		if p.Role == "loyalist" && len(hand) > 1 {
			add(Action{Choice: "yes"})
		}
	case "fangquan_give":
		if p.Role == "loyalist" && len(hand) > 0 {
			add(Action{Cards: hand[:1], Targets: []int{g.Lord}})
		}
	case "xiangle":
		for _, id := range hand {
			if sgBasic(sgCard(id).Kind) {
				add(Action{Cards: []int{id}})
			}
		}
	case "tiaoxin":
		if !(p.Role == "loyalist" && e.Actor == g.Lord) {
			for _, a := range cardsFor("slash") {
				add(a)
			}
			add(Action{Choice: "jijiang"})
		}
	case "beige":
		if e.Target == i || p.Role == "loyalist" && e.Target == g.Lord {
			if len(all) > 0 {
				add(Action{Cards: all[:1]})
			}
		}
	case "beige_discard":
		add(Action{Cards: all[:e.Amount]})
	case "guzheng":
		ids := clone(q.Cards)
		slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
		for _, id := range ids {
			add(Action{Card: id})
		}
	case "zhiba_accept", "zhiba_obtain":
		add(Action{Choice: "yes"})
	}
}
func (s *State) sgMountainBotPlay(i int, enemies []int, add func(Action)) {
	g := s.Sanguosha
	p := g.Players[i]
	if s.sgHas(i, "jixi") {
		for _, id := range p.Fields {
			for _, who := range enemies {
				add(Action{Type: "sg_skill", Skill: "jixi", Cards: []int{id}, Targets: []int{who}})
			}
		}
	}
	if s.sgHas(i, "tiaoxin") && p.Used["tiaoxin"] == 0 && p.HP > 1 {
		for _, who := range enemies {
			add(Action{Type: "sg_skill", Skill: "tiaoxin", Targets: []int{who}})
		}
	}
	if s.sgHas(i, "zhijian") {
		for _, id := range p.Hand {
			slot := SGCardTypes[sgCard(id).Kind].Slot
			if slot != "" && s.sgEquip(i, slot) != 0 && p.Role == "loyalist" {
				add(Action{Type: "sg_skill", Skill: "zhijian", Cards: []int{id}, Targets: []int{g.Lord}})
			}
		}
	}
	if i != g.Lord && s.sgKingdom(i) == "wu" && s.sgHas(g.Lord, "zhiba") && p.Used["zhiba_pindian"] == 0 {
		ids := clone(p.Hand)
		slices.SortStableFunc(ids, func(a, b int) int { return sgCard(b).Rank - sgCard(a).Rank })
		if p.Role == "loyalist" {
			slices.Reverse(ids)
		}
		if len(ids) > 0 {
			add(Action{Type: "sg_skill", Skill: "zhiba_pindian", Cards: ids[:1]})
		}
	}
}
