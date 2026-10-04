package game

import "slices"

func (s *State) sgJieBotPrompt(i int, add func(Action)) {
	g := s.Sanguosha
	q := g.Pending
	e := q.Event
	p := g.Players[i]
	ally := func(who int) bool { return who == i || p.Role == "loyalist" && who == g.Lord }
	ids := append(clone(p.Hand), p.Equip...)
	slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
	switch q.Kind {
	case "draw_phase":
		if s.sgHas(i, "jie_tuxi") {
			targets := []int{}
			for _, who := range s.sgOrder(s.sgNext(i)) {
				if !ally(who) && len(g.Players[who].Hand) > 0 && len(g.Players[who].Hand) >= len(p.Hand) {
					targets = append(targets, who)
				}
			}
			if len(targets) > 0 {
				add(Action{Choice: "jie_tuxi", Targets: targets[:min(len(targets), s.sgGodDrawCount(i, 2))]})
			}
		}
	case "qingjian", "jie_rende":
		if p.Role == "loyalist" && i != g.Lord && s.sgAlive(g.Lord) && len(g.Players[g.Lord].Hand) < 4 && len(q.Cards) > 0 {
			add(Action{Cards: q.Cards[:1], Targets: []int{g.Lord}})
		}
	case "jie_lianying":
		targets := []int{i}
		if p.Role == "loyalist" && i != g.Lord && s.sgAlive(g.Lord) && e.Amount >= 2 {
			targets = append(targets, g.Lord)
		}
		add(Action{Targets: targets})
	case "jie_jianxiong":
		if slices.Contains(q.Choices, "take") {
			add(Action{Choice: "take"})
		}
		add(Action{Choice: "draw"})
	case "jie_yiji_targets":
		if p.Role == "loyalist" && i != g.Lord && s.sgAlive(g.Lord) && len(p.Hand) > s.sgHandLimit(i) {
			add(Action{Targets: []int{g.Lord}})
		}
	case "jie_yiji_give":
		hand := clone(p.Hand)
		slices.SortStableFunc(hand, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
		add(Action{Cards: hand[:min(e.Amount, len(hand))]})
	case "jie_guicai":
		for _, id := range ids {
			c := s.sgCardFor(e.Actor, id)
			protect := ally(e.Actor)
			good := (e.Kind == "indulgence" && c.Suit == 1) || (e.Kind == "supply_shortage" && c.Suit == 2) || (e.Kind == "lightning" && !(c.Suit == 0 && c.Rank >= 2 && c.Rank <= 9))
			if protect && good {
				add(Action{Cards: []int{id}})
			}
		}
	case "jie_tieji", "tishen", "jie_qianxun":
		add(Action{Choice: "yes"})
	case "jie_tieji_discard":
		for _, id := range ids {
			if s.sgCardFor(i, id).Suit == e.Color {
				add(Action{Cards: []int{id}})
			}
		}
	case "yijue_heal":
		if ally(e.Target) {
			add(Action{Choice: "yes"})
		}
	case "yajiao":
		if slices.Contains(q.Choices, "give") {
			add(Action{Choice: "give", Targets: []int{i}})
		} else {
			add(Action{Choice: "discard"})
		}
	case "jie_fanjian":
		count := 0
		for _, id := range ids {
			if s.sgCardFor(i, id).Suit == e.Color {
				count++
			}
		}
		if count <= 1 || p.HP <= 1 {
			add(Action{Choice: "discard"})
		}
		add(Action{Choice: "lose_hp"})
	case "liyu":
		for _, who := range q.Targets {
			if !ally(who) {
				add(Action{Targets: []int{who}})
			}
		}
	case "fenwei":
		if slices.Contains([]string{"savage_assault", "archery_attack", "iron_chain"}, e.Kind) {
			targets := []int{}
			for _, who := range q.Targets {
				if ally(who) {
					targets = append(targets, who)
				}
			}
			if len(targets) > 0 {
				add(Action{Targets: targets})
			}
		}
	}
}

func (s *State) sgJieBotPlay(i int, enemies []int, add func(Action)) {
	g := s.Sanguosha
	p := g.Players[i]
	ids := append(clone(p.Hand), p.Equip...)
	slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
	if s.sgHas(i, "jie_rende") && p.Role == "loyalist" && s.sgAlive(g.Lord) && i != g.Lord && len(p.Hand) > 2 {
		add(Action{Type: "sg_skill", Skill: "jie_rende", Cards: clone(p.Hand[:min(2, len(p.Hand))]), Targets: []int{g.Lord}})
	}
	if s.sgHas(i, "yijue") {
		for _, id := range p.Hand {
			if sgCard(id).Rank >= 9 {
				for _, who := range enemies {
					add(Action{Type: "sg_skill", Skill: "yijue", Cards: []int{id}, Targets: []int{who}})
				}
			}
		}
	}
	if s.sgHas(i, "jie_kurou") && p.HP > 1 && len(ids) > 0 {
		add(Action{Type: "sg_skill", Skill: "jie_kurou", Cards: ids[:1]})
	}
	for _, skill := range []string{"jie_fanjian", "jie_guose"} {
		if !s.sgHas(i, skill) {
			continue
		}
		for _, id := range ids {
			if skill == "jie_guose" && s.sgCardFor(i, id).Suit == 3 {
				for _, who := range s.sgOrder(0) {
					if who == i || p.Role == "loyalist" && who == g.Lord {
						add(Action{Type: "sg_skill", Skill: skill, Choice: "remove", Cards: []int{id}, Targets: []int{who}})
					}
				}
			}
			for _, who := range enemies {
				add(Action{Type: "sg_skill", Skill: skill, Choice: "use", Cards: []int{id}, Targets: []int{who}})
			}
		}
	}
	if s.sgHas(i, "chuli") && len(ids) > 0 {
		targets := []int{}
		kingdoms := map[string]bool{}
		for _, who := range enemies {
			k := s.sgKingdom(who)
			if !kingdoms[k] && len(s.sgDiscardable(i, who, false)) > 0 {
				kingdoms[k] = true
				targets = append(targets, who)
			}
		}
		if len(targets) > 0 {
			add(Action{Type: "sg_skill", Skill: "chuli", Cards: ids[:1], Targets: targets})
		}
	}
	if s.sgHas(i, "jie_lijian") && len(ids) > 0 {
		for _, target := range enemies {
			for _, source := range enemies {
				if target != source {
					add(Action{Type: "sg_skill", Skill: "jie_lijian", Cards: ids[:1], Targets: []int{target, source}})
				}
			}
		}
	}
}
