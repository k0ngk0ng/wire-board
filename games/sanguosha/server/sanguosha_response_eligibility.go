package sanguosha

import "slices"

// Capability is independent of AI strategy: declining to use a card is not the
// same as having no legal response. Queries must not reveal hidden generals or
// inspect anyone else's hand on behalf of the viewer.
func (s *State) sgHasResponseCard(player int, wanted string) bool {
	if !s.sgAlive(player) {
		return false
	}
	g := s.Sanguosha
	p := g.Players[player]
	if v := g.Virtual; v != nil && v.Player == player {
		if _, err := s.sgAs(player, []int{v.Card}, "guhuo", wanted); err == nil {
			return true
		}
	}
	ids := append(append([]int{}, p.Hand...), p.Equip...)
	for _, id := range ids {
		if _, err := s.sgAs(player, []int{id}, "", wanted); err == nil {
			return true
		}
	}
	for _, skill := range []string{"kanpo", "qingguo", "longdan", "wusheng", "jijiu", "jiuchi"} {
		if !s.sgHegMayInvoke(player, skill) {
			continue
		}
		for _, id := range ids {
			if _, err := s.sgBotAs(player, []int{id}, skill, wanted); err == nil {
				return true
			}
		}
	}
	if wanted == "slash" && s.sgWeapon(player) == "spear" && len(p.Hand) >= 2 {
		if _, err := s.sgAs(player, p.Hand[:2], "spear", wanted); err == nil {
			return true
		}
	}
	if s.sgHas(player, "longhun") {
		groups := [4][]int{}
		for _, id := range ids {
			if s.sgHandUsable(player, []int{id}) {
				groups[s.sgCardFor(player, id).Suit] = append(groups[s.sgCardFor(player, id).Suit], id)
			}
		}
		for _, group := range groups {
			n := max(1, p.HP)
			if len(group) >= n {
				if _, err := s.sgAs(player, group[:n], "longhun", wanted); err == nil {
					return true
				}
			}
		}
	}
	// A bluff can use an unrelated card. Do not suppress this real choice just
	// because the declared card is not literally present in the player's hand.
	if s.sgHas(player, "guhuo") && slices.Contains(s.sgGuhuoKinds(), wanted) {
		for _, id := range p.Hand {
			if s.sgHandUsable(player, []int{id}) {
				return true
			}
		}
	}
	return false
}

func (s *State) sgCanNullify(player int, passed []int) bool {
	return !slices.Contains(passed, player) && s.sgHasResponseCard(player, "nullification")
}

func (s *State) sgCanAnswerPrompt(player int, prompt SGPrompt) bool {
	if prompt.Kind == "nullification" {
		return s.sgCanNullify(player, prompt.Event.Targets)
	}
	return player >= 0 && player == prompt.Player
}

func (s *State) sgCanSupport(player int, e SGEvent) bool {
	if e.Kind == "hujia" {
		return s.sgHasResponseCard(player, "jink") || e.Step&1 == 0 && s.sgArmor(player) == "eight_diagram"
	}
	return s.sgHasResponseCard(player, "slash")
}

func (s *State) sgHasSupporter(player int, skill string) bool {
	kingdom := "wei"
	if skill == "jijiang" {
		kingdom = "shu"
	}
	for _, other := range s.sgOrder(0) {
		if other != player && s.sgKingdom(other) == kingdom && s.sgCanSupport(other, SGEvent{Kind: skill}) {
			return true
		}
	}
	return false
}

// Unknown prompts stay interactive. In particular free skills, judgement
// activation and Guhuo challenges are meaningful without a card in hand.
func (s *State) sgPromptHasOption(prompt SGPrompt) bool {
	if prompt.Kind == "nullification" {
		return !s.sgAllPassed(prompt.Event.Targets)
	}
	i, e := prompt.Player, prompt.Event
	if !s.sgAlive(i) {
		return true
	} // Special death prompts have their own rules.
	p := s.Sanguosha.Players[i]
	switch prompt.Kind {
	case "card":
		wanted := sgWanted(e)
		if s.sgHasResponseCard(i, wanted) {
			return true
		}
		if wanted == "jink" && e.Step&1 == 0 && !s.sgIgnoreArmor(e) && (s.sgArmor(i) == "eight_diagram" || s.sgEquip(i, "armor") == 0 && s.sgHegMayInvoke(i, "bazhen")) {
			return true
		}
		skill := "jijiang"
		if wanted == "jink" {
			skill = "hujia"
		}
		return e.Step&2 == 0 && s.sgHas(i, skill) && s.sgHasSupporter(i, skill)
	case "support":
		return s.sgCanSupport(i, e)
	case "peach":
		return s.sgHasResponseCard(i, "peach") || i == e.Target && s.sgHasResponseCard(i, "analeptic")
	case "fire_discard":
		for _, id := range p.Hand {
			if s.sgCardFor(i, id).Suit == e.Aux {
				return true
			}
		}
		return false
	case "guidao":
		for _, id := range append(append([]int{}, p.Hand...), p.Equip...) {
			if suit := s.sgCardFor(i, id).Suit; suit == 0 || suit == 2 {
				return true
			}
		}
		return false
	case "guicai", "sword_discard":
		return len(p.Hand) > 0
	case "jie_guicai":
		return len(p.Hand)+len(p.Equip) > 0
	case "jilve_guicai":
		return p.Marks["bear"] > 0 && len(p.Hand)+len(p.Equip) > 0
	case "ganglie":
		return len(p.Hand) >= 2
	case "weapon_after_jink":
		if s.sgWeapon(i) == "axe" {
			return len(p.Hand)+len(p.Equip)-1 >= 2
		}
		return s.sgHasResponseCard(i, "slash")
	case "jie_tieji_discard":
		for _, id := range append(append([]int{}, p.Hand...), p.Equip...) {
			if s.sgCardFor(i, id).Suit == e.Color {
				return true
			}
		}
		return false
	}
	return true
}

// ResumeSanguoshaResponses upgrades already persisted all-player response
// windows. Existing usable prompts and their clocks remain unchanged.
func (s *State) ResumeSanguoshaResponses() bool {
	if s == nil || s.Sanguosha == nil || s.Finished || s.Sanguosha.Pending == nil || s.sgPromptHasOption(*s.Sanguosha.Pending) {
		return false
	}
	s.sgRun()
	return true
}
