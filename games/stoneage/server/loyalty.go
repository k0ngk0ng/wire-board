package stoneage

// Thresholds follow battle.c::BATTLE_PetLoyalCheck, with a roll in [1,100].
func loyaltyMode(loyalty, roll int) string {
	switch {
	case loyalty >= 80:
		return "normal"
	case loyalty >= 70:
		if roll < 10 {
			return "target"
		}
	case loyalty >= 60:
		if roll < 20 {
			return "target"
		}
	case loyalty >= 50:
		if roll < 35 {
			return "target"
		}
	case loyalty >= 40:
		if roll < 50 {
			return "target"
		}
	case loyalty >= 20:
		if roll < 70 {
			return "random"
		}
	case loyalty >= 10:
		if roll < 80 {
			return "owner"
		}
		return "enemy"
	default:
		if roll < 60 {
			return "owner"
		}
		return "flee"
	}
	return "normal"
}

// Commands are finalized once before guard/guardian registration, so a refused
// defensive order cannot still grant its original protection. Results remain
// private until the pet's turn is presented on the server timeline.
func (s *State) loyalCommand(id int, command Command) (Command, string, bool) {
	u := s.unit(id)
	if id%2 == 0 || !s.active(id) || u.Loyalty >= 80 || command.Kind == "wait" || u.Status.Stone > 0 || u.Status.Sleep > 0 || u.Status.Confusion > 0 {
		return command, "", false
	}
	wait := Command{Kind: "wait", Target: id}
	enemy := func() int {
		candidates := []int{}
		for target := 0; target < len(s.Players)*2; target++ {
			if s.active(target) && !s.friendly(id, target) {
				candidates = append(candidates, target)
			}
		}
		if len(candidates) == 0 {
			return -1
		}
		return candidates[s.randomN(len(candidates))]
	}
	mode := loyaltyMode(u.Loyalty, s.randomN(100)+1)
	switch mode {
	case "target":
		if command.Target == id || command.Kind == "guard" {
			return wait, "不服从：发呆", false
		}
		command.Target = enemy()
		return command, "不服从：擅自改变目标", false
	case "random":
		if command.Target == id || command.Kind == "guard" {
			return wait, "不服从：发呆", false
		}
		// The original draws a learned slot, including empty slots, out of seven.
		skills := s.Players[id/2].Loadout.Skills
		slot := s.randomN(7)
		if slot >= len(skills) {
			return wait, "不服从：发呆", false
		}
		skill := skills[slot]
		target := id
		if skill != 2 {
			target = enemy()
		}
		return Command{Kind: "skill", ID: skill, Target: target}, "不服从：擅自使用技能", false
	case "owner":
		return Command{Kind: "attack", Target: id - 1}, "不服从：攻击主人", true
	case "enemy":
		return Command{Kind: "attack", Target: enemy()}, "不服从：擅自攻击", false
	case "flee":
		return Command{Kind: "flee", Target: id}, "不服从：逃跑", false
	}
	return command, "", false
}

func reliablePets(pets []PetDefinition) []PetDefinition {
	out := []PetDefinition{}
	for _, p := range pets {
		if p.Loyalty >= 80 {
			out = append(out, p)
		}
	}
	if len(out) > 0 {
		return out
	}
	best := pets[0]
	for _, p := range pets {
		if p.Loyalty > best.Loyalty {
			best = p
		}
	}
	return []PetDefinition{best}
}
