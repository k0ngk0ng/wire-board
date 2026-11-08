package stoneage

import "fmt"

// ComboCheck/ComboCheck2: consecutive ordinary melee attacks after initiative
// ordering, same side and target, with a 50% opening roll. Ineligible entries
// break the run; an unsuccessful opening can try again at the next member.
func (s *State) combinedEligible(t battleTurn) bool {
	if !s.active(t.id) || t.loyaltyNote != "" || !(t.command.Kind == "attack" || t.command.Kind == "skill" && t.command.ID == 1) {
		return false
	}
	u := s.unit(t.id)
	if u.Status.Stone > 0 || u.Status.Sleep > 0 || u.Status.Confusion > 0 {
		return false
	}
	if t.id%2 == 0 {
		weapon, _ := gearByID(s.Pool.Weapons, s.Players[t.id/2].Loadout.Weapon)
		return !isThrow(weapon.WeaponType)
	}
	return true
}
func (s *State) assignCombinedAttacks(order []battleTurn) {
	group := 0
	for i := 0; i < len(order)-1; i++ {
		if !s.combinedEligible(order[i]) {
			continue
		}
		end := i + 1
		for end < len(order) && s.combinedEligible(order[end]) && s.friendly(order[i].id, order[end].id) && order[i].command.Target == order[end].command.Target {
			end++
		}
		if end-i < 2 || s.randomN(100) >= 50 {
			continue
		}
		group++
		for j := i; j < end; j++ {
			order[j].combo = group
		}
		i = end - 1
	}
}

// BATTLE_Combo computes each participant's normal damage without dodge or
// counter, accumulates the original rider partition, then applies one impact.
// No member loses their hit because an earlier contribution would be lethal.
func (s *State) combinedAttack(actors []int, target int) {
	type contribution struct{ actor, body, ride int }
	parts := make([]contribution, 0, len(actors))
	defender := s.unit(target)
	_, defense, _ := s.offense(target)
	if defender.Status.Stone > 0 {
		defense *= 2
	}
	anyCritical := false
	for _, actor := range actors {
		attack, _, speed := s.offense(actor)
		_, _, defSpeed := s.offense(target)
		critical := s.randomN(100) < max(3, min(18, 5+(speed-defSpeed)/15))
		amount := physicalBase(float64(attack), float64(defense)*.7, s.randomN(10000))
		if critical {
			amount = max(amount, attack)
		}
		amount = int(float64(amount) * elementMultiplier(s.unit(actor).Elements, defender.Elements))
		if defender.Guard {
			amount = guardDamage(amount, 1+s.randomN(100))
		}
		if defender.Status.Shield > 0 {
			amount = amount * 65 / 100
		}
		body, ride := s.partitionDamage(target, max(1, amount))
		parts = append(parts, contribution{actor, body, ride})
		anyCritical = anyCritical || critical
	}
	lost, rideLost := 0, 0
	for _, part := range parts {
		n, r := s.applyDamageParts(part.actor, target, part.body, part.ride)
		lost += n
		rideLost += r
	}
	s.event("combo", actors[0], target, fmt.Sprintf("%d人合击", len(actors)), lost, rideLost, anyCritical)
	s.Events[len(s.Events)-1].Actors = append([]int(nil), actors...)
	s.timeEvent(&s.Events[len(s.Events)-1])
}
