package pal

func battleGroupEnemies(v View) bool {
	n := 0
	for _, e := range v.Battle.Enemies {
		if e.Object != 0 {
			n++
		}
	}
	return n > 1
}

// Observe only the original selector: trying Attack reveals single/all-target
// scope before a command is committed. No equipment table or hidden state is
// read, and an exploratory single-target choice returns to the same role.
func battleAttackScopeKeys(v View, m *battleBotMemory) (int, uint32, bool) {
	b := v.Battle
	r := recoveryRole(m, v.Party[b.Current].Role)
	if r != nil && r.ProbingAttack && b.Action == 2 {
		if b.State == 2 {
			return 4, 1, true
		}
		if b.State == 4 {
			return 4, 2, true
		}
	}
	if b.State != 1 || b.Menu != 0 || !battleGroupEnemies(v) || len(b.Players) != len(v.Party) {
		return 0, 0, false
	}
	if recoveryWanted(v, m, r) {
		covered := recoveryCovered(v, m, b.Current)
		for i, p := range v.Party {
			if p.HP*2 < p.MaxHP && !recoveryCovered(v, m, i) {
				covered = false
			}
		}
		if !covered {
			return 0, 0, false
		}
	}
	if r != nil && (r.AttackScope == 1 || r.Healing || r.Offensive || r.Cooping || r.ItemKind != 0) {
		return 0, 0, false
	}
	if b.Selected != 0 {
		return 2, 16, true
	}
	return 4, 2, true
}

func rememberAttackScopeInput(r *battleBotRole, v View, keys uint32) {
	b := v.Battle
	if b.State == 1 && b.Menu == 0 && b.Selected == 0 && keys == 2 && r.AttackScope == 0 && battleGroupEnemies(v) && len(b.Players) == len(v.Party) {
		r.ProbingAttack = true
	}
	if b.Action == 2 && ((b.State == 2 && (keys == 1 || keys == 2)) || (b.State == 4 && keys == 2)) {
		if b.State == 4 {
			r.AttackScope = 2
		} else {
			r.AttackScope = 1
		}
		r.ProbingAttack = false
	}
}
