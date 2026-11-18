package pal

// Remember only the first occupied public lineup. Later arrivals must not
// indefinitely divert single-target actions from enemies already being fought.
func rememberBattleEnemyOrigin(m *battleBotMemory, v View) {
	if m == nil || v.Battle == nil || len(m.OriginalEnemies) != 0 {
		return
	}
	occupied := false
	for _, e := range v.Battle.Enemies {
		occupied = occupied || e.Object != 0
	}
	if !occupied {
		return
	}
	m.OriginalEnemies = make([]int, len(v.Battle.Enemies))
	for i, e := range v.Battle.Enemies {
		m.OriginalEnemies[i] = e.Object
	}
}

func battleHasReinforcements(v View, m *battleBotMemory) bool {
	if m == nil || len(m.OriginalEnemies) == 0 {
		return false
	}
	for i, e := range v.Battle.Enemies {
		if e.Object != 0 && (i >= len(m.OriginalEnemies) || e.Object != m.OriginalEnemies[i]) {
			return true
		}
	}
	return false
}

func battleEnemyTarget(v View, m *battleBotMemory) int {
	target, original := -1, -1
	for i, e := range v.Battle.Enemies {
		if e.Object == 0 {
			continue
		}
		target = i
		if m != nil && i < len(m.OriginalEnemies) && m.OriginalEnemies[i] == e.Object {
			original = i
		}
	}
	if original >= 0 {
		return original
	}
	return target
}

func battleEnemyTargetKeys(v View, m *battleBotMemory) (int, uint32, bool) {
	b := v.Battle
	target := battleEnemyTarget(v, m)
	if b.State == 2 && target >= 0 && b.Target != target {
		// Native backward selection stops at an empty slot zero. Navigate
		// toward the intended slot instead of wrapping through that boundary.
		if target > b.Target {
			return 3, 32, true
		}
		return 1, 8, true
	}
	return 4, 2, true
}
