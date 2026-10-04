package game

import "slices"

// Only this seat's candidates, hand and explicitly public factions are used.
// In particular, the server's knowledge of hidden teammates is not consulted.
func (s *State) sgHegBotPrompt(i int, add func(Action)) {
	if !s.sgHegemony() || s.Sanguosha.Pending == nil {
		return
	}
	g := s.Sanguosha
	q := g.Pending
	p := g.Players[i]
	switch q.Kind {
	case "draw_phase":
		if s.sgHegMayInvoke(i, "yingzi") && s.sgHegMayInvoke(i, "haoshi") {
			add(Action{Choice: "yingzi+haoshi"})
		}
	case "heg_invoke", "heg_guzheng_obtain":
		add(Action{Choice: "yes"})
	case "heg_mingshi", "kuanggu":
		add(Action{Choice: "yes"})
	case "heg_suishi":
		if q.Event.Kind == "dying" {
			add(Action{Choice: "yes"})
		}
	case "heg_reveal_turn":
		for _, choice := range []string{"both", "head", "deputy"} {
			if slices.Contains(q.Choices, choice) {
				add(Action{Choice: choice})
			}
		}
	case "heg_duanchang":
		best, n := "head", -1
		for slot, id := range s.sgHegGeneralIDs(q.Event.Target) {
			h := g.Players[q.Event.Target].Hegemony
			value := 1
			if h.Lost[slot] {
				value = -1
			} else if h.Shown[slot] {
				value = len(sgGeneral(id).Skills)
			}
			if value > n {
				best, n = []string{"head", "deputy"}[slot], value
			}
		}
		add(Action{Choice: best})
	case "heg_shuangren":
		ids := clone(p.Hand)
		slices.SortStableFunc(ids, func(a, b int) int { return sgCard(b).Rank - sgCard(a).Rank })
		if len(ids) > 0 && sgCard(ids[0]).Rank >= 10 {
			for _, target := range q.Targets {
				if !s.sgHegFriend(i, target) {
					add(Action{Cards: ids[:1], Targets: []int{target}})
				}
			}
		}
	case "heg_shuangren_slash":
		for _, target := range q.Targets {
			if !s.sgHegFriend(i, target) {
				add(Action{Targets: []int{target}})
			}
		}
		for _, target := range q.Targets { // Required after choosing to initiate.
			add(Action{Targets: []int{target}})
		}
	case "heg_lirang":
		for _, target := range q.Targets {
			if s.sgHegBotAlly(i, target) {
				add(Action{Cards: q.Cards, Targets: []int{target}})
			}
		}
	case "heg_shenzhi":
		if p.HP < p.MaxHP && len(p.Hand) >= p.HP && len(p.Hand) <= 3 {
			add(Action{Choice: "yes"})
		}
	case "heg_xiaoguo":
		if !s.sgHegFriend(i, q.Event.Target) {
			ids := clone(q.Cards)
			slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
			for _, id := range ids {
				add(Action{Cards: []int{id}})
			}
		}
	case "heg_xiaoguo_discard":
		ids := append(clone(p.Hand), p.Equip...)
		slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
		for _, id := range ids {
			if SGCardTypes[sgCard(id).Kind].Slot != "" {
				add(Action{Cards: []int{id}})
			}
		}
	case "heg_shushen", "heg_sijian":
		for _, target := range q.Targets {
			if s.sgHegBotAlly(i, target) == (q.Kind == "heg_shushen") {
				add(Action{Targets: []int{target}})
			}
		}
	case "heg_kuangfu":
		if !s.sgHegFriend(i, q.Event.Target) {
			for _, id := range q.Cards {
				add(Action{Choice: "move", Cards: []int{id}})
				add(Action{Choice: "discard", Cards: []int{id}})
			}
		}
	case "heg_luoshen", "heg_luoshen_tiandu":
		add(Action{Choice: "yes"})
	case "heg_generals":
		best, score := "", -1
		for _, head := range p.Choices {
			for _, deputy := range p.Choices {
				a, b := sgGeneral(head), sgGeneral(deputy)
				if head == deputy || a.Kingdom != b.Kingdom {
					continue
				}
				n := 3*(a.HP+b.HP) + len(a.Skills) + len(b.Skills)
				if sgHegCompanions(head, deputy) {
					n += 4
				}
				if n > score {
					best, score = head+"+"+deputy, n
				}
			}
		}
		if best != "" {
			add(Action{Choice: best})
		}
	case "heg_reward":
		if p.HP < p.MaxHP && slices.Contains(q.Choices, "heal") {
			add(Action{Choice: "heal"})
		}
		add(Action{Choice: "draw"})
	case "heg_known_both":
		for _, choice := range []string{"head", "deputy", "hand"} {
			if slices.Contains(q.Choices, choice) {
				add(Action{Choice: choice})
			}
		}
	case "heg_intel":
		add(Action{Choice: "ok"})
	case "heg_await_discard":
		ids := append(clone(p.Hand), p.Equip...)
		slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
		add(Action{Cards: ids[:min(q.Event.Amount, len(ids))]})
	case "heg_triblade":
		ids := clone(p.Hand)
		slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
		for _, target := range q.Targets {
			if len(ids) > 0 && !s.sgHegFriend(i, target) {
				add(Action{Cards: ids[:1], Targets: []int{target}})
			}
		}
	}
}
