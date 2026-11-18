package pal

// This policy sees only the public party and its already accepted commands.
// Original PAL decides availability, cost, damage and target legality.
func soloCooperationReady(v View, m *battleBotMemory) bool {
	b := v.Battle
	if b == nil || b.State != 1 || b.Menu != 0 || b.Current < 0 || b.Current >= len(v.Party) || len(v.Party) < 2 || len(b.Players) != len(v.Party) {
		return false
	}
	p := v.Party[b.Current]
	health := 80
	enemies := 0
	for _, e := range b.Enemies {
		if e.Object != 0 {
			enemies++
		}
	}
	if enemies > 1 {
		health = 60
	}
	reserve := false
	for i, member := range v.Party {
		if enemies > 1 {
			if r := recoveryRole(m, member.Role); r != nil && r.AttackScope == 2 {
				return false
			}
		}
		if member.Controller != p.Controller || member.MaxHP <= 0 || member.HP*100 < member.MaxHP*health || b.Players[i].Role != member.Role {
			return false
		}
		if i < b.Current {
			previous := recoveryRole(m, member.Role)
			if b.Players[i].State != 2 || !(b.Players[i].Action == 2 || b.Players[i].Action == 3 && previous != nil && previous.Offensive && !previous.Healing) {
				return false
			}
		} else if b.Players[i].State == 2 {
			return false
		}
		// Cooperation consumes the whole party's turn. A low-mana leader
		// must not repeatedly suppress another role's remaining spell budget.
		if member.MaxMP > 0 && member.MP*2 > member.MaxMP {
			return false
		}
		reserve = reserve || member.MaxMP > 0 && member.MP*2 <= member.MaxMP
	}
	return reserve
}

func soloCooperationBotKeys(v View, m *battleBotMemory) (int, uint32, bool) {
	b := v.Battle
	r := recoveryRole(m, v.Party[b.Current].Role)
	if r != nil && r.Cooping && (b.State == 3 || b.State == 5) && b.Action == 4 {
		return 4, 1, true
	}
	if r != nil && r.Cooping && (b.State == 2 || b.State == 4) && b.Action == 4 {
		if battleHasReinforcements(v, m) {
			return battleEnemyTargetKeys(v, m)
		}
		return 4, 2, true
	}
	if !soloCooperationReady(v, m) {
		return 0, 0, false
	}
	if r != nil && r.CoopDeclined {
		return 0, 0, false
	}
	if b.Selected == 2 {
		return 4, 2, true
	}
	if r != nil && r.CoopAttempts >= 4 {
		return 0, 0, false
	}
	return 3, 32, true
}
