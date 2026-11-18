package pal

// Classic Force is a whole-party command. Use the same original shortcut a
// human has only at a healthy, untouched solo round boundary, never as the
// fallback for one role after another role selected healing or an item.
func battleRoundAutoReady(v View, actor int, m *battleBotMemory) bool {
	b := v.Battle
	if battleHasReinforcements(v, m) {
		return false
	}
	if b.State != 1 || b.Menu != 0 || b.Current != 0 || len(v.Party) < 2 || len(b.Players) != len(v.Party) {
		return false
	}
	for i, p := range v.Party {
		if p.Controller != actor || p.HP <= 0 || p.HP != p.MaxHP || p.MaxMP > 0 && p.MP*5 < p.MaxMP*4 || b.Players[i].Role != p.Role || b.Players[i].State == 2 {
			return false
		}
		if r := recoveryRole(m, p.Role); r != nil && (r.Healing || r.Offensive || r.Cooping || r.ItemKind != 0 || r.ProbingAttack || r.AttackScope == 2 && battleGroupEnemies(v)) {
			return false
		}
	}
	return true
}

// At the last role there are no later decisions for Classic Force to consume.
// The original loop skips earlier accepted actions, so their heal/item choices
// remain intact. Recovery policy runs before this check.
func battleLastRoleAutoReady(v View, actor int, m *battleBotMemory) bool {
	b := v.Battle
	if battleHasReinforcements(v, m) {
		return false
	}
	if b.State != 1 || b.Menu != 0 || len(v.Party) < 2 || b.Current != len(v.Party)-1 || len(b.Players) != len(v.Party) {
		return false
	}
	for i, p := range v.Party {
		if p.Controller != actor || b.Players[i].Role != p.Role || i < b.Current && p.HP > 0 && b.Players[i].State != 2 {
			return false
		}
	}
	p := v.Party[b.Current]
	if p.MaxMP <= 0 || p.MP*2 <= p.MaxMP {
		return false
	}
	if r := recoveryRole(m, p.Role); r != nil && (r.Healing || r.Offensive || r.Cooping || r.ItemKind != 0 || r.ProbingAttack) {
		return false
	}
	return true
}
