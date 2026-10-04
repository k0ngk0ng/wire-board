package game

import "slices"

// Only our own unrevealed kingdom and other seats' public factions are used.
// Private information about somebody else's generals never determines allies.
func (s *State) sgHegBotAlly(i, target int) bool {
	return s.sgHegFriend(i, target) || s.sgHegWouldFriend(i, target)
}

func (s *State) sgBotAs(i int, ids []int, skill, desired string) (string, error) {
	if s.sgHegemony() && skill != "" && !s.sgHas(i, skill) && s.sgHegMayInvoke(i, skill) {
		probe := clone(*s)
		return probe.sgCommittedAs(i, ids, skill, desired)
	}
	return s.sgAs(i, ids, skill, desired)
}

func (s *State) sgHegBotAOE(i int, kind string) bool {
	advantage := 0
	for _, target := range s.sgOrder(0) {
		if target == i || s.sgArmor(target) == "vine" || kind == "savage_assault" && (s.sgHas(target, "huoshou") || s.sgHas(target, "juxiang")) {
			continue
		}
		weight := 1
		if s.Sanguosha.Players[target].HP <= 1 {
			weight = 3
		}
		if s.sgHegBotAlly(i, target) {
			advantage -= weight
		} else {
			advantage += weight
		}
	}
	return advantage > 0
}

// Shared identity heuristics still generate legal alternatives. This policy
// keeps their faction-inappropriate candidates from taking precedence over a
// safe pass, while mandatory target prompts retain a legal last resort.
func (s *State) sgHegBotActionAllowed(i int, a Action) bool {
	g := s.Sanguosha
	ally := func(to int) bool { return s.sgHegBotAlly(i, to) }
	allTargets := func(friend bool) bool {
		for _, target := range a.Targets {
			if ally(target) != friend {
				return false
			}
		}
		return true
	}
	if q := g.Pending; q != nil {
		if a.Choice == "pass" {
			return true
		}
		e := q.Event
		switch q.Kind {
		case "invoke":
			if e.Kind == "ganglie" || e.Kind == "fankui" {
				return !ally(e.Actor)
			}
		case "heg_invoke":
			if e.Kind == "wushuang" && e.Next != nil {
				other := e.Next.Actor
				if other == i {
					other = e.Next.Target
				}
				return !ally(other)
			}
		case "card":
			if e.Kind == "collateral" {
				return !ally(e.Aux)
			}
		case "leiji", "liuli", "tianxiang", "shensu_judge", "shensu_play", "heg_sijian", "heg_triblade", "qiaobian_draw", "luanwu":
			return allTargets(false)
		case "quhu_target", "heg_shuangren_slash":
			for _, to := range q.Targets {
				if !ally(to) {
					return allTargets(false)
				}
			}
		case "qiaobian_move":
			return len(a.Targets) == 2 && (SGCardTypes[sgCard(a.Card).Kind].Slot != "" && !ally(a.Targets[0]) && ally(a.Targets[1]) || SGCardTypes[sgCard(a.Card).Kind].Slot == "" && ally(a.Targets[0]) && !ally(a.Targets[1]))
		case "qiaobian":
			if e.Kind == "draw" {
				for _, to := range s.sgOrder(0) {
					if to != i && !ally(to) && len(g.Players[to].Hand) > 0 {
						return true
					}
				}
				return false
			}
		case "fangzhu":
			if len(a.Targets) == 1 {
				return ally(a.Targets[0]) == g.Players[a.Targets[0]].Flipped
			}
		case "yinghun":
			return allTargets(a.Choice == "draw_many")
		case "fire_discard", "heg_xiaoguo", "heg_kuangfu", "lieren", "mengjin", "liegong", "tieji", "ice_sword", "kylin_bow", "double_sword", "weapon_after_jink", "xiangle":
			return !ally(e.Target)
		case "heg_shushen", "heg_lirang", "jieming", "fangquan_give", "yiji":
			return allTargets(true)
		case "beige", "peach":
			return ally(e.Target)
		case "draw_phase":
			if a.Choice == "tuxi" {
				return allTargets(false)
			}
		}
		return true
	}
	if a.Type == "sg_skill" {
		switch a.Skill {
		case "heg_rende", "zhijian", "qingnang", "jieyin":
			return allTargets(true)
		case "heg_lijian", "heg_fenxun", "heg_qingcheng", "fanjian", "qiangxi", "quhu", "tianyi":
			return allTargets(false)
		case "dimeng":
			return len(a.Targets) == 2 && ally(a.Targets[0]) && !ally(a.Targets[1])
		case "luanwu":
			return s.sgHegBotAOE(i, "luanwu")
		}
	}
	if a.Type == "sg_play" {
		kind, err := s.sgBotAs(i, a.Cards, a.Skill, "")
		if err != nil {
			return false
		}
		switch kind {
		case "slash", "fire_slash", "thunder_slash", "duel", "snatch", "dismantlement", "indulgence", "supply_shortage", "fire_attack", "known_both":
			return allTargets(false)
		case "savage_assault", "archery_attack":
			return s.sgHegBotAOE(i, kind)
		}
	}
	return true
}

func (s *State) sgHegBotCommonPrompt(i int, add func(Action), cardsFor func(string) []Action) {
	if !s.sgHegemony() || s.Sanguosha.Pending == nil {
		return
	}
	g := s.Sanguosha
	q, p := g.Pending, g.Players[i]
	e := q.Event
	all := append(clone(p.Hand), p.Equip...)
	slices.SortStableFunc(all, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
	allies := []int{}
	for _, to := range s.sgOrder(i) {
		if s.sgHegBotAlly(i, to) {
			allies = append(allies, to)
		}
	}
	switch q.Kind {
	case "peach":
		if e.Target == i {
			for _, a := range cardsFor("analeptic") {
				add(a)
			}
		}
		if s.sgHegBotAlly(i, e.Target) {
			for _, a := range cardsFor("peach") {
				add(a)
			}
		}
		add(Action{Choice: "pass"})
	case "nullification":
		friend := s.sgHegBotAlly(i, e.Target)
		knownEnemy := s.sgHegShown(e.Target) && !friend
		harmful := slices.Contains([]string{"duel", "snatch", "dismantlement", "savage_assault", "archery_attack", "indulgence", "lightning", "collateral", "fire_attack", "supply_shortage", "known_both"}, e.Kind)
		counter := friend && (harmful != e.Flag) || knownEnemy && (harmful == e.Flag)
		if counter {
			for _, a := range cardsFor("nullification") {
				if friend && harmful && !e.Flag && e.CounterDepth == 0 && e.Aux != -2 && !sgIsDelayed(e.Kind) && a.Skill == "" && len(a.Cards) == 1 && sgCard(a.Cards[0]).Kind == "heg_nullification" {
					a.Choice = "faction"
				}
				add(a)
			}
		}
		add(Action{Choice: "pass"})
	case "haoshi_give":
		ids := clone(p.Hand)
		slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
		for _, to := range q.Targets {
			if s.sgHegBotAlly(i, to) {
				add(Action{Cards: ids[:min(e.Amount, len(ids))], Targets: []int{to}})
			}
		}
	case "jieming":
		slices.SortStableFunc(allies, func(a, b int) int {
			pa, pb := g.Players[a], g.Players[b]
			return (min(5, pb.MaxHP) - len(pb.Hand)) - (min(5, pa.MaxHP) - len(pa.Hand))
		})
		for _, to := range allies {
			add(Action{Targets: []int{to}})
		}
	case "yiji":
		for _, to := range allies {
			if to != i && len(g.Players[to].Hand) < len(p.Hand) {
				add(Action{Cards: q.Cards, Targets: []int{to}})
			}
		}
	case "yinghun":
		if e.Amount > 1 {
			for _, to := range allies {
				if to != i {
					add(Action{Choice: "draw_many", Targets: []int{to}})
				}
			}
		}
	case "fangquan":
		if len(allies) > 1 && len(p.Hand) > 1 {
			add(Action{Choice: "yes"})
		}
	case "fangquan_give":
		for _, to := range allies {
			if to != i && len(p.Hand) > 0 {
				ids := clone(p.Hand)
				slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
				add(Action{Cards: ids[:1], Targets: []int{to}})
			}
		}
	case "beige":
		if s.sgHegBotAlly(i, e.Target) && len(all) > 0 {
			add(Action{Cards: all[:1]})
		}
	case "guzheng":
		if s.sgHegBotAlly(i, e.Actor) {
			ids := clone(q.Cards)
			slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(b) - sgBotValue(a) })
			for _, id := range ids {
				add(Action{Card: id})
			}
		}
	case "draw_phase":
		if s.sgHegMayInvoke(i, "tuxi") {
			targets := []int{}
			for _, to := range s.sgOrder(s.sgNext(i)) {
				if to != i && !s.sgHegBotAlly(i, to) && len(g.Players[to].Hand) > 0 {
					targets = append(targets, to)
				}
			}
			if len(targets) >= 2 {
				add(Action{Choice: "tuxi", Targets: targets[:2]})
			}
		}
	}
}
