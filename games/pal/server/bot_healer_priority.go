package pal

func recoveryDeferred(v View, m *battleBotMemory, current *battleBotRole, target int) bool {
	b := v.Battle
	if current == nil || current.Healing || current.HealingRank == 0 || target < 0 || v.Party[target].HP*3 <= v.Party[target].MaxHP || len(b.Players) != len(v.Party) {
		return false
	}
	for i := b.Current + 1; i < len(v.Party); i++ {
		p := v.Party[i]
		r := recoveryRole(m, p.Role)
		if r == nil || r.HealingRank <= current.HealingRank || r.Failed && p.MP <= r.CheckedMP || r.Reviving || r.Offensive || r.Cooping || r.ItemKind != 0 {
			continue
		}
		if p.Controller == v.Party[b.Current].Controller && p.HP*3 > p.MaxHP && p.MP > 0 && p.MP*2 > p.MaxMP && b.Players[i].Role == p.Role && b.Players[i].State != 2 {
			return true
		}
	}
	return false
}

func rememberHealingRank(r *battleBotRole, v View, keys uint32) {
	if !r.Healing || v.Battle.State != 1 || v.Battle.Menu != 1 || keys != 2 {
		return
	}
	for _, row := range battleVisibleSpells(v) {
		if row.selected && row.affordable {
			r.HealingRank = max(r.HealingRank, battleSpellRank(row.name, true, 2))
		}
	}
}
