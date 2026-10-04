package game

import (
	"errors"
	"slices"
)

// Decisions are based on the bot's own role/hand and visible seats. Hidden enemy
// roles, other hands and deck order are never inspected to rank an action.
func (s *State) sgBot(i int) (Action, error) {
	g := s.Sanguosha
	if !s.sgAlive(i) && !s.sgGodDeathResponse(i) {
		return Action{}, errors.New("inactive seat")
	}
	p := g.Players[i]
	candidates := []Action{}
	add := func(a Action) {
		if g.Pending != nil {
			a.Prompt = g.Pending.ID
		}
		candidates = append(candidates, a)
	}
	cardsFor := func(want string) []Action {
		out := []Action{}
		ids := append(append([]int{}, p.Hand...), p.Equip...)
		for _, id := range ids {
			c := s.sgCardFor(i, id)
			if s.sgHas(i, "guhuo") && slices.Contains(p.Hand, id) && c.Suit == 1 && (c.Kind == want || want == "slash" && sgIsSlash(c.Kind)) {
				out = append(out, Action{Cards: []int{id}, Skill: "guhuo", Choice: want})
			}
			for _, skill := range []string{"", "wusheng", "qingguo", "longdan", "qixi", "guose", "jijiu", "fan", "huoji", "kanpo", "lianhuan", "shuangxiong", "duanliang", "jiuchi"} {
				a := Action{Cards: []int{id}, Skill: skill}
				if _, err := s.sgAs(i, a.Cards, skill, want); err == nil {
					out = append(out, a)
				}
			}
		}
		if want == "archery_attack" && s.sgHas(i, "luanji") {
			for j, id := range p.Hand {
				for _, other := range p.Hand[j+1:] {
					if s.sgCardFor(i, id).Suit == s.sgCardFor(i, other).Suit {
						out = append(out, Action{Cards: []int{id, other}, Skill: "luanji"})
					}
				}
			}
		}
		if want == "slash" && s.sgWeapon(i) == "spear" && len(p.Hand) >= 2 {
			out = append(out, Action{Cards: append([]int{}, p.Hand[:2]...), Skill: "spear"})
		}
		if s.sgHas(i, "guhuo") && slices.Contains(s.sgGuhuoKinds(), want) {
			for _, id := range p.Hand {
				if sgBotValue(id) <= 50 {
					out = append(out, Action{Cards: []int{id}, Skill: "guhuo", Choice: want})
				}
			}
		}
		out = append(out, s.sgGodBotLonghun(i, want)...)
		return out
	}
	if q := g.Pending; q != nil {
		if q.Player != i && !(q.Kind == "nullification" && !slices.Contains(q.Event.Targets, i)) {
			return Action{}, errors.New("not responding")
		}
		e := q.Event
		s.sgMountainBotPrompt(i, add, cardsFor)
		s.sgGodBotPrompt(i, add)
		s.sgJieBotPrompt(i, add)
		switch q.Kind {
		case "xingshang":
			add(Action{Choice: "yes"})
		case "songwei", "baonue":
			if p.Role == "loyalist" {
				add(Action{Choice: "yes"})
			}
		case "fangzhu":
			for _, target := range s.sgOrder(s.sgNext(i)) {
				if target != i && !(p.Role == "loyalist" && target == g.Lord && !g.Players[target].Flipped) {
					add(Action{Targets: []int{target}})
				}
			}
		case "lieren":
			ids := clone(p.Hand)
			slices.SortStableFunc(ids, func(a, b int) int { return sgCard(b).Rank - sgCard(a).Rank })
			if len(ids) > 0 && sgCard(ids[0]).Rank >= 8 {
				add(Action{Cards: ids[:1]})
			}
		case "yinghun":
			if p.Role == "loyalist" {
				add(Action{Choice: "draw_many", Targets: []int{g.Lord}})
			}
			for _, target := range s.sgOrder(s.sgNext(i)) {
				if target != i && !(p.Role == "loyalist" && target == g.Lord) {
					add(Action{Choice: "draw_one", Targets: []int{target}})
				}
			}
		case "haoshi_give":
			ids := clone(p.Hand)
			slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
			if p.Role == "loyalist" && slices.Contains(q.Targets, g.Lord) {
				add(Action{Cards: ids[:e.Amount], Targets: []int{g.Lord}})
			}
			for _, target := range q.Targets {
				add(Action{Cards: ids[:e.Amount], Targets: []int{target}})
			}
		case "yinghun_discard":
			ids := append(clone(p.Hand), p.Equip...)
			slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
			add(Action{Cards: ids[:e.Amount]})
		case "benghuai":
			if p.MaxHP > p.HP {
				add(Action{Choice: "maxhp"})
			}
			add(Action{Choice: "hp"})
		case "luanwu":
			for _, target := range q.Targets {
				if target == g.Lord && p.Role == "loyalist" {
					continue
				}
				for _, a := range cardsFor("slash") {
					a.Targets = []int{target}
					add(a)
				}
				add(Action{Choice: "jijiang", Targets: []int{target}})
			}
		case "pindian":
			ids := clone(p.Hand)
			slices.SortStableFunc(ids, func(a, b int) int { return sgCard(b).Rank - sgCard(a).Rank })
			for _, id := range ids {
				add(Action{Cards: []int{id}})
			}
		case "quhu_target":
			for _, target := range q.Targets {
				if target != i && !(p.Role == "loyalist" && target == g.Lord) {
					add(Action{Targets: []int{target}})
				}
			}
			for _, target := range q.Targets {
				add(Action{Targets: []int{target}})
			}
		case "jieming":
			targets := []int{i}
			if p.Role == "loyalist" {
				targets = append(targets, g.Lord)
			}
			slices.SortStableFunc(targets, func(a, b int) int {
				pa, pb := g.Players[a], g.Players[b]
				return (min(5, pb.MaxHP) - len(pb.Hand)) - (min(5, pa.MaxHP) - len(pa.Hand))
			})
			for _, target := range targets {
				add(Action{Targets: []int{target}})
			}
		case "niepan", "mengjin":
			add(Action{Choice: "yes"})
		case "guhuo_question":
			// Use only the public declaration, turn token and our own HP/role.
			// Never inspect the private card in the resumable bluff context.
			if p.HP > 1 && !(p.Role == "loyalist" && e.Actor == g.Lord) && q.ID%3 == 0 {
				add(Action{Choice: "challenge"})
			}
		case "general":
			for _, id := range p.Choices {
				add(Action{Choice: id})
			}
		case "shensu_judge", "shensu_play":
			for _, target := range s.sgOrder(s.sgNext(i)) {
				if target == i || (p.Role == "loyalist" && target == g.Lord) {
					continue
				}
				if q.Kind == "shensu_judge" {
					if g.Players[target].HP <= 2 {
						add(Action{Targets: []int{target}})
					}
				} else {
					for _, id := range append(append([]int{}, p.Hand...), p.Equip...) {
						if SGCardTypes[sgCard(id).Kind].Slot != "" {
							add(Action{Cards: []int{id}, Targets: []int{target}})
						}
					}
				}
			}
		case "liegong", "buqu":
			add(Action{Choice: "yes"})
		case "leiji":
			for _, target := range s.sgOrder(s.sgNext(i)) {
				if target != i && !(p.Role == "loyalist" && target == g.Lord) {
					add(Action{Targets: []int{target}})
				}
			}
		case "tianxiang":
			for _, id := range p.Hand {
				if s.sgCardFor(i, id).Suit == 1 {
					for _, target := range s.sgOrder(s.sgNext(i)) {
						if target != i && !(p.Role == "loyalist" && target == g.Lord) {
							add(Action{Cards: []int{id}, Targets: []int{target}})
						}
					}
				}
			}
		case "buqu_remove":
			// Prefer removing duplicate ranks so recovery can resolve a failed Buqu.
			ids := clone(p.Buqu)
			remove := []int{}
			for len(remove) < e.Amount {
				counts := map[int]int{}
				for _, id := range ids {
					counts[sgCard(id).Rank]++
				}
				pick := ids[0]
				for _, id := range ids {
					if counts[sgCard(id).Rank] > 1 {
						pick = id
						break
					}
				}
				remove = append(remove, pick)
				ids = sgRemove(ids, pick)
			}
			add(Action{Cards: remove})
		case "guidao":
			for _, id := range append(append([]int{}, p.Hand...), p.Equip...) {
				c := s.sgCardFor(i, id)
				if c.Suit != 0 && c.Suit != 2 {
					continue
				}
				protect := e.Actor == i || (p.Role == "loyalist" && e.Actor == g.Lord)
				if (e.Kind == "leiji" && !protect && c.Suit == 0) || (e.Kind == "lightning" && protect && !(c.Suit == 0 && c.Rank >= 2 && c.Rank <= 9)) || (e.Kind == "supply_shortage" && protect && c.Suit == 2) {
					add(Action{Cards: []int{id}})
				}
			}
		case "invoke", "keji":
			add(Action{Choice: "yes"})
		case "guanxing":
			add(Action{Cards: q.Cards})
		case "draw_phase":
			if s.sgHas(i, "shelie") {
				add(Action{Choice: "shelie"})
			}
			if s.sgHas(i, "haoshi") {
				add(Action{Choice: "haoshi"})
			}
			if s.sgHas(i, "zaiqi") && p.MaxHP-p.HP >= 2 {
				add(Action{Choice: "zaiqi"})
			}
			if s.sgHas(i, "shuangxiong") && len(p.Hand) >= 3 {
				add(Action{Choice: "shuangxiong"})
			}
			if s.sgHas(i, "yingzi") {
				add(Action{Choice: "yingzi"})
			}
			add(Action{Choice: "normal"})
		case "discard":
			ids := clone(p.Hand)
			slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
			add(Action{Cards: ids[:min(e.Amount, len(ids))]})
		case "card":
			wanted := sgWanted(e)
			if wanted == "jink" {
				add(Action{Choice: "eight_diagram"})
			}
			for _, a := range cardsFor(wanted) {
				add(a)
			}
			if wanted == "jink" {
				add(Action{Choice: "hujia"})
			} else {
				add(Action{Choice: "jijiang"})
			}
		case "support":
			if (p.Role == "loyalist" || p.Role == "lord") && e.Actor == g.Lord {
				wanted := "jink"
				if e.Kind == "hujia" {
					add(Action{Choice: "eight_diagram"})
				}
				if e.Kind == "jijiang" {
					wanted = "slash"
				}
				for _, a := range cardsFor(wanted) {
					add(a)
				}
			}
		case "fire_reveal":
			for _, id := range p.Hand {
				add(Action{Cards: []int{id}})
			}
		case "fire_discard":
			for _, id := range p.Hand {
				if s.sgCardFor(i, id).Suit == e.Aux && sgCard(id).Kind != "peach" {
					add(Action{Cards: []int{id}})
				}
			}
		case "peach":
			if e.Target == i {
				for _, a := range cardsFor("analeptic") {
					add(a)
				}
			}
			if e.Target == i || ((p.Role == "loyalist" || p.Role == "lord") && e.Target == g.Lord) {
				for _, a := range cardsFor("peach") {
					add(a)
				}
			}
		case "nullification": // Never infer allegiance from another hidden identity.
			protects := e.Target == i || ((p.Role == "loyalist" || p.Role == "lord") && e.Target == g.Lord)
			harmful := slices.Contains([]string{"duel", "snatch", "dismantlement", "savage_assault", "archery_attack", "indulgence", "lightning", "collateral", "fire_attack", "supply_shortage"}, e.Kind)
			if protects && harmful && !e.Flag {
				for _, a := range cardsFor("nullification") {
					add(a)
				}
			}
		case "double_sword", "tieji", "tiandu":
			add(Action{Choice: "yes"})
		case "weapon_after_jink":
			if s.sgWeapon(i) == "blade" {
				for _, a := range cardsFor("slash") {
					add(a)
				}
			}
		case "steal":
			for _, id := range g.Players[e.Target].Equip {
				add(Action{Card: id})
			}
			add(Action{Choice: "hand"})
			if e.Kind == "snatch" || e.Kind == "dismantlement" || e.Kind == "guixin" {
				for _, d := range g.Players[e.Target].Judgment {
					add(Action{Card: d.Card})
				}
			}
		case "grace":
			ids := clone(g.Grace)
			slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(b) - sgBotValue(a) })
			for _, id := range ids {
				add(Action{Card: id})
			}
		case "yiji":
			add(Action{Cards: q.Cards, Targets: []int{i}})
		case "fanjian":
			add(Action{Choice: "heart"})
		case "sword_discard":
			if len(p.Hand) > 2 {
				add(Action{Cards: p.Hand[:1]})
			}
		case "ganglie":
			if len(p.Hand) >= 2 {
				add(Action{Cards: p.Hand[:2]})
			}
		case "kylin_bow":
			for _, id := range g.Players[e.Target].Equip {
				slot := SGCardTypes[sgCard(id).Kind].Slot
				if slot == "offense" || slot == "defense" {
					add(Action{Cards: []int{id}})
				}
			}
		}
		add(Action{Choice: "pass"})
	} else {
		if i != s.Turn || s.Phase != "sg_play" {
			return Action{}, errors.New("not playing")
		}
		if p.Role == "loyalist" && s.sgKingdom(i) == "qun" {
			for _, id := range p.Hand {
				if k := sgCard(id).Kind; k == "jink" || k == "lightning" {
					add(Action{Type: "sg_skill", Skill: "huangtian_give", Cards: []int{id}, Targets: []int{g.Lord}})
				}
			}
		}
		for _, a := range cardsFor("peach") {
			a.Type = "sg_play"
			add(a)
		}
		for _, id := range p.Hand {
			c := sgCard(id)
			slot := SGCardTypes[c.Kind].Slot
			if slot != "" && s.sgEquip(i, slot) == 0 {
				add(Action{Type: "sg_play", Cards: []int{id}})
			}
			if c.Kind == "ex_nihilo" || c.Kind == "amazing_grace" || c.Kind == "god_salvation" {
				add(Action{Type: "sg_play", Cards: []int{id}})
			}
		}
		enemies := []int{}
		for _, t := range s.sgOrder(s.sgNext(i)) {
			if t == i || ((p.Role == "lord" || p.Role == "loyalist") && t == g.Lord) {
				continue
			}
			enemies = append(enemies, t)
		}
		slices.SortStableFunc(enemies, func(a, b int) int {
			if p.Role == "rebel" {
				if a == g.Lord {
					return -1
				}
				if b == g.Lord {
					return 1
				}
			}
			if p.Role == "renegade" && len(enemies) > 1 {
				if a == g.Lord {
					return 1
				}
				if b == g.Lord {
					return -1
				}
			}
			return g.Players[a].HP - g.Players[b].HP
		})
		s.sgMountainBotPlay(i, enemies, add)
		s.sgGodBotPlay(i, enemies, add)
		s.sgJieBotPlay(i, enemies, add)
		if s.sgHas(i, "luanwu") && p.Marks["luanwu"] == 0 {
			add(Action{Type: "sg_skill", Skill: "luanwu"})
		}
		if s.sgHas(i, "dimeng") && p.Used["dimeng"] == 0 {
			ids := append(clone(p.Hand), p.Equip...)
			slices.SortStableFunc(ids, func(a, b int) int { return sgBotValue(a) - sgBotValue(b) })
			for _, poor := range s.sgOrder(s.sgNext(i)) {
				if poor == i {
					continue
				}
				for _, rich := range enemies {
					if rich == poor || len(g.Players[rich].Hand) <= len(g.Players[poor].Hand) {
						continue
					}
					if p.Role != "loyalist" && rich == g.Lord || p.Role == "loyalist" && poor == g.Lord {
						due := len(g.Players[rich].Hand) - len(g.Players[poor].Hand)
						if due <= len(ids) {
							add(Action{Type: "sg_skill", Skill: "dimeng", Cards: ids[:due], Targets: []int{poor, rich}})
						}
					}
				}
			}
		}
		if s.sgHas(i, "qiangxi") {
			for _, target := range enemies {
				for _, id := range append(append([]int{}, p.Hand...), p.Equip...) {
					if SGCardTypes[sgCard(id).Kind].Slot == "weapon" {
						add(Action{Type: "sg_skill", Skill: "qiangxi", Cards: []int{id}, Targets: []int{target}})
					}
				}
				if p.HP > 1 {
					add(Action{Type: "sg_skill", Skill: "qiangxi", Targets: []int{target}})
				}
			}
		}
		for _, skill := range []string{"quhu", "tianyi"} {
			if s.sgHas(i, skill) {
				ids := clone(p.Hand)
				slices.SortStableFunc(ids, func(a, b int) int { return sgCard(b).Rank - sgCard(a).Rank })
				if len(ids) > 0 && sgCard(ids[0]).Rank >= 9 {
					for _, target := range enemies {
						add(Action{Type: "sg_skill", Skill: skill, Cards: ids[:1], Targets: []int{target}})
					}
				}
			}
		}
		// Drink only when a legal slash can follow; use only visible armor/HP.
		if p.Drank == 0 {
			canSlash := false
			for _, attack := range cardsFor("slash") {
				for _, target := range enemies {
					trial := clone(*s)
					attack.Type = "sg_play"
					attack.Targets = []int{target}
					if trial.sgApply(i, attack) == nil {
						canSlash = true
						break
					}
				}
				if canSlash {
					break
				}
			}
			if canSlash {
				for _, a := range cardsFor("analeptic") {
					a.Type = "sg_play"
					add(a)
				}
			}
		}
		for _, kind := range []string{"slash", "duel", "snatch", "dismantlement", "indulgence", "supply_shortage", "fire_attack"} {
			for _, a := range cardsFor(kind) {
				for _, t := range enemies {
					a.Type = "sg_play"
					a.Targets = []int{t}
					add(a)
				}
			}
		}
		for _, kind := range []string{"savage_assault", "archery_attack"} {
			for _, a := range cardsFor(kind) {
				a.Type = "sg_play"
				add(a)
			}
		}
		for _, a := range cardsFor("iron_chain") {
			a.Type = "sg_play"
			if p.Chained {
				a.Targets = []int{i}
			}
			add(a)
		}
		if p.HP < p.MaxHP && len(p.Hand) > 0 {
			add(Action{Type: "sg_skill", Skill: "qingnang", Cards: p.Hand[:1], Targets: []int{i}})
		}
		add(Action{Type: "sg_end"})
	}
	for _, a := range candidates {
		trial := clone(*s)
		if trial.sgApply(i, a) == nil {
			return a, nil
		}
	}
	return Action{}, errors.New("no legal sanguosha bot action")
}
func sgBotValue(id int) int {
	c := sgCard(id)
	switch c.Kind {
	case "peach":
		return 100
	case "jink":
		return 80
	case "nullification":
		return 70
	case "ex_nihilo":
		return 90
	case "slash", "fire_slash", "thunder_slash":
		return 40
	case "analeptic":
		return 65
	}
	if SGCardTypes[c.Kind].Slot != "" {
		return 30
	}
	return 50
}
func (s *State) SanguoshaActors() []int {
	if q := s.Sanguosha.Pending; q != nil {
		if q.Kind == "nullification" {
			actors := []int{}
			for _, i := range s.sgOrder(s.Turn) {
				if !slices.Contains(q.Event.Targets, i) {
					actors = append(actors, i)
				}
			}
			return actors
		}
		return []int{q.Player}
	}
	return []int{s.Turn}
}
func (s *State) SanguoshaActor() int {
	actors := s.SanguoshaActors()
	if len(actors) > 0 {
		return actors[0]
	}
	return s.Turn
}

// Mandatory prompts get a legal deterministic completion; optional responses
// time out as a pass, so another player's hand is never auto-spent to rescue.
func (s *State) SanguoshaTimeoutAction() (Action, error) {
	g := s.Sanguosha
	i := s.SanguoshaActor()
	if g.Pending == nil {
		return Action{Type: "sg_end"}, nil
	}
	a := Action{Prompt: g.Pending.ID, Choice: "pass"}
	trial := clone(*s)
	if trial.sgApply(i, a) == nil {
		return a, nil
	}
	return s.sgBot(i)
}
func (s *State) SanguoshaTimeout() error {
	a, err := s.SanguoshaTimeoutAction()
	if err != nil {
		return err
	}
	return s.Apply(s.SanguoshaActor(), a)
}
