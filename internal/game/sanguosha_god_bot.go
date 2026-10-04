package game

import "slices"

func (s *State) sgGodBotLonghun(i int, want string) []Action {
	if !s.sgHas(i, "longhun") {
		return nil
	}
	p := s.Sanguosha.Players[i]
	grouped := make([][]int, 4)
	for _, id := range append(clone(p.Hand), p.Equip...) {
		c := s.sgCardFor(i, id)
		grouped[c.Suit] = append(grouped[c.Suit], id)
	}
	out := []Action{}
	for _, ids := range grouped {
		slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
		n := max(1, p.HP)
		if len(ids) >= n {
			if _, err := s.sgAs(i, ids[:n], "longhun", want); err == nil {
				out = append(out, Action{Cards: clone(ids[:n]), Skill: "longhun"})
			}
		}
	}
	return out
}
func (s *State) sgGodBotEnemy(i, to int) bool {
	if i == to {
		return false
	}
	p := s.Sanguosha.Players[i]
	return !(p.Role == "loyalist" && to == s.Sanguosha.Lord)
}
func (s *State) sgGodBotPrompt(i int, add func(Action)) {
	g := s.Sanguosha
	q := g.Pending
	p := g.Players[i]
	cheap := func(ids []int) []int {
		ids = clone(ids)
		slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
		return ids
	}
	switch q.Kind {
	case "god_kingdom":
		kingdom := "qun"
		if p.Role == "loyalist" {
			kingdom = s.sgKingdom(g.Lord)
		}
		add(Action{Choice: kingdom})
		add(Action{Choice: "wei"})
	case "wuhun_target":
		for _, to := range q.Targets {
			if s.sgGodBotEnemy(i, to) {
				add(Action{Targets: []int{to}})
			}
		}
		if len(q.Targets) > 0 {
			add(Action{Targets: q.Targets[:1]})
		}
	case "shelie":
		ids := cheap(q.Cards)
		slices.Reverse(ids)
		seen := map[int]bool{}
		take := []int{}
		for _, id := range ids {
			suit := sgCard(id).Suit
			if !seen[suit] {
				seen[suit] = true
				take = append(take, id)
			}
		}
		add(Action{Cards: take})
	case "gongxin":
		if s.sgGodBotEnemy(i, q.Event.Target) {
			for _, id := range q.Cards {
				if s.sgCardFor(q.Event.Target, id).Suit == 1 {
					add(Action{Card: id, Choice: "discard"})
				}
			}
		}
	case "qixing_initial":
		ids := cheap(p.Hand)
		add(Action{Cards: ids[:min(7, len(ids))]})
	case "qixing_exchange":
		hand, stars := cheap(p.Hand), cheap(p.Stars)
		slices.Reverse(stars)
		put, take := []int{}, []int{}
		for j := 0; j < min(len(hand), len(stars)); j++ {
			if sgBotValue(stars[j]) > sgBotValue(hand[j]) {
				put = append(put, hand[j])
				take = append(take, stars[j])
			}
		}
		if len(put) > 0 {
			add(Action{Cards: put, Take: take})
		}
	case "dawu":
		if len(p.Stars) > 0 {
			targets := []int{i}
			if p.Role == "loyalist" && s.sgAlive(g.Lord) && len(p.Stars) > 1 {
				targets = append(targets, g.Lord)
			}
			add(Action{Cards: cheap(p.Stars)[:len(targets)], Targets: targets})
		}
	case "kuangfeng":
		// Keep protection stars unless a visibly chained enemy can amplify fire damage.
		if len(p.Stars) > 2 {
			for _, to := range s.sgOrder(s.Turn) {
				if s.sgGodBotEnemy(i, to) && g.Players[to].Chained {
					add(Action{Cards: cheap(p.Stars)[:1], Targets: []int{to}})
				}
			}
		}
	case "qinyin":
		friendlyWounds := p.MaxHP - p.HP
		if p.Role == "loyalist" {
			friendlyWounds += g.Players[g.Lord].MaxHP - g.Players[g.Lord].HP
		}
		if friendlyWounds > 0 {
			add(Action{Choice: "heal"})
		} else if p.HP > 1 && (p.Role != "loyalist" || g.Players[g.Lord].HP > 1) {
			add(Action{Choice: "lose"})
		}
	case "wumou":
		add(Action{Choice: "wrath"})
		add(Action{Choice: "hp"})
	case "shenfen_hand":
		ids := cheap(p.Hand)
		if len(ids) >= 4 {
			add(Action{Cards: ids[:4]})
		}
	case "jilve_jizhi":
		add(Action{Choice: "yes"})
	case "jilve_jizhi_exchange":
		ids := cheap(p.Hand)
		if len(ids) > 0 && sgBotValue(ids[0]) < sgBotValue(q.Cards[0]) {
			add(Action{Cards: ids[:1]})
		}
	case "jilve_fangzhu":
		for _, to := range s.sgOrder(s.sgNext(i)) {
			if s.sgGodBotEnemy(i, to) && !g.Players[to].Flipped {
				add(Action{Targets: []int{to}})
			}
		}
	case "jilve_guicai":
		// Compare only the public judgment and our own cards, never the draw pile.
		e := q.Event
		good := func(id int) bool {
			c := s.sgCardFor(e.Actor, id)
			switch e.Kind {
			case "lightning":
				return !(c.Suit == 0 && c.Rank >= 2 && c.Rank <= 9)
			case "indulgence":
				return c.Suit == 1
			case "supply_shortage":
				return c.Suit == 2
			case "wuhun":
				return c.Kind == "peach" || c.Kind == "god_salvation"
			case "eight_diagram", "support_eight", "tieji":
				return c.Suit == 1 || c.Suit == 3
			case "tuntian":
				return c.Suit != 1
			}
			return true
		}
		wantGood := !s.sgGodBotEnemy(i, e.Actor)
		if good(e.Aux) != wantGood {
			for _, id := range cheap(append(clone(p.Hand), p.Equip...)) {
				if good(id) == wantGood {
					add(Action{Cards: []int{id}})
				}
			}
		}
	case "lianpo":
		add(Action{Choice: "yes"})
	}
}
func (s *State) sgGodBotPlay(i int, enemies []int, add func(Action)) {
	g := s.Sanguosha
	p := g.Players[i]
	if s.sgHas(i, "shenfen") && p.Marks["wrath"] >= 6 {
		add(Action{Type: "sg_skill", Skill: "shenfen"})
	}
	if s.sgHas(i, "wuqian") && p.Marks["wrath"] >= 2 && p.Used["slash"] == 0 && !s.sgHas(i, "wushuang") {
		for _, to := range enemies {
			if s.sgDistance(i, to) <= s.sgRange(i) && len(p.Hand) > 0 {
				add(Action{Type: "sg_skill", Skill: "wuqian", Targets: []int{to}})
			}
		}
	}
	if s.sgHas(i, "gongxin") {
		for _, to := range enemies {
			if len(g.Players[to].Hand) > 0 {
				add(Action{Type: "sg_skill", Skill: "gongxin", Targets: []int{to}})
			}
		}
	}
	if s.sgHas(i, "yeyan") && p.Marks["yeyan"] == 0 {
		bySuit := map[int]int{}
		for _, id := range p.Hand {
			suit := s.sgCardFor(i, id).Suit
			if old := bySuit[suit]; old == 0 || sgBotValue(id) < sgBotValue(old) {
				bySuit[suit] = id
			}
		}
		if len(bySuit) == 4 && p.HP > 3 {
			for _, to := range enemies {
				if g.Players[to].HP <= 3 {
					ids := []int{bySuit[0], bySuit[1], bySuit[2], bySuit[3]}
					add(Action{Type: "sg_skill", Skill: "yeyan", Cards: ids, Targets: []int{to}})
				}
			}
		}
		if len(enemies) > 0 {
			add(Action{Type: "sg_skill", Skill: "yeyan", Targets: enemies[:min(3, len(enemies))]})
		}
	}
	if s.sgHas(i, "jilve") && p.Marks["bear"] > 0 {
		cheap := []int{}
		for _, id := range p.Hand {
			if sgBotValue(id) <= 50 {
				cheap = append(cheap, id)
			}
		}
		if len(cheap) > 0 {
			add(Action{Type: "sg_skill", Skill: "jilve", Choice: "zhiheng", Cards: cheap})
		}
		if !s.sgHas(i, "wansha") {
			for _, to := range enemies {
				if g.Players[to].HP == 1 {
					add(Action{Type: "sg_skill", Skill: "jilve", Choice: "wansha"})
				}
			}
		}
	}
}
