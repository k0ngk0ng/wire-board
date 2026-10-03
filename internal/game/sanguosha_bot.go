package game

import (
	"errors"
	"slices"
)

// Decisions are based on the bot's own role/hand and visible seats. Hidden enemy
// roles, other hands and deck order are never inspected to rank an action.
func (s *State) sgBot(i int) (Action, error) {
	g := s.Sanguosha
	if !s.sgAlive(i) {
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
			for _, skill := range []string{"", "wusheng", "qingguo", "longdan", "qixi", "guose", "jijiu"} {
				a := Action{Cards: []int{id}, Skill: skill}
				if _, err := s.sgAs(i, a.Cards, skill, want); err == nil {
					out = append(out, a)
				}
			}
		}
		if want == "slash" && s.sgWeapon(i) == "spear" && len(p.Hand) >= 2 {
			out = append(out, Action{Cards: append([]int{}, p.Hand[:2]...), Skill: "spear"})
		}
		return out
	}
	if q := g.Pending; q != nil {
		if q.Player != i && !(q.Kind == "nullification" && !slices.Contains(q.Event.Targets, i)) {
			return Action{}, errors.New("not responding")
		}
		e := q.Event
		switch q.Kind {
		case "general":
			for _, id := range p.Choices {
				add(Action{Choice: id})
			}
		case "invoke", "keji":
			add(Action{Choice: "yes"})
		case "guanxing":
			add(Action{Cards: q.Cards})
		case "draw_phase":
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
		case "peach":
			if e.Target == i || ((p.Role == "loyalist" || p.Role == "lord") && e.Target == g.Lord) {
				for _, a := range cardsFor("peach") {
					add(a)
				}
			}
		case "nullification": // Never infer allegiance from another hidden identity.
			protects := e.Target == i || ((p.Role == "loyalist" || p.Role == "lord") && e.Target == g.Lord)
			harmful := slices.Contains([]string{"duel", "snatch", "dismantlement", "savage_assault", "archery_attack", "indulgence", "lightning", "collateral"}, e.Kind)
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
			if e.Kind == "snatch" || e.Kind == "dismantlement" {
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
		for _, kind := range []string{"slash", "duel", "snatch", "dismantlement", "indulgence"} {
			for _, a := range cardsFor(kind) {
				for _, t := range enemies {
					a.Type = "sg_play"
					a.Targets = []int{t}
					add(a)
				}
			}
		}
		for _, id := range p.Hand {
			if kind := sgCard(id).Kind; kind == "savage_assault" || kind == "archery_attack" {
				add(Action{Type: "sg_play", Cards: []int{id}})
			}
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
	case "slash":
		return 40
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
