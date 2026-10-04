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
