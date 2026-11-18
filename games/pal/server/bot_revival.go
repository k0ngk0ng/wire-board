package pal

func revivalWord(name string) bool {
	return name == "赎魂" || name == "还魂咒" || name == "赎魂咒"
}

func revivalTarget(v View, m *battleBotMemory) int {
	if v.Battle == nil {
		return -1
	}
	for target, member := range v.Party {
		if member.HP != 0 {
			continue
		}
		covered := false
		for i, player := range v.Battle.Players {
			if i >= len(v.Party) || v.Party[i].HP <= 0 {
				continue
			}
			r := recoveryRole(m, player.Role)
			covered = covered || r != nil && r.Reviving && player.State == 2 && player.Action == 3 && (player.Target == target || player.Target == -1)
		}
		if !covered {
			return target
		}
	}
	return -1
}

func revivalWanted(v View, m *battleBotMemory, r *battleBotRole) bool {
	b := v.Battle
	if b == nil || b.State != 1 || b.Menu != 0 || b.Current < 0 || b.Current >= len(v.Party) || revivalTarget(v, m) < 0 {
		return false
	}
	p := v.Party[b.Current]
	if p.MP <= 0 || p.HP*3 <= p.MaxHP || r != nil && (r.ItemKind != 0 || r.Healing || r.Offensive || r.Cooping) {
		return false
	}
	return r == nil || !r.ReviveFailed || p.MP > r.ReviveCheckedMP
}

func observedRevivalKeys(v View) (uint32, bool) {
	rows := battleVisibleSpells(v)
	selected, target := -1, -1
	for i, row := range rows {
		if row.selected {
			selected = i
		}
		if row.affordable && revivalWord(row.name) && (target < 0 || row.selected) {
			target = i
		}
	}
	if selected < 0 || target < 0 {
		return 0, false
	}
	if selected == target {
		return 2, true
	}
	if target < selected {
		return 8, true
	}
	return 32, true
}

func revivalBotKeys(v View, m *battleBotMemory) (int, uint32, bool) {
	b := v.Battle
	r := recoveryRole(m, v.Party[b.Current].Role)
	if r != nil && r.Reviving {
		if b.State == 1 {
			if b.Menu == 0 {
				if b.Selected != 1 {
					return 1, 8, true
				}
				return 4, 2, true
			}
			if b.Menu != 1 {
				return 4, 1, true
			}
			if r.Pending != 0 && r.Settled < 2 {
				return 4, 0, true
			}
			if keys, ok := observedRevivalKeys(v); ok {
				return 4, keys, true
			}
			if r.Moves == 0 && r.Pending == 0 {
				return 4, 65536, true
			}
			if r.Moves >= 128 || r.Pending == 32 && r.Cursor != "" && r.Cursor == fieldBotCursorName(v) {
				return 4, 1, true
			}
			return 4, 32, true
		}
		if b.State == 3 || b.State == 5 {
			target := revivalTarget(v, m)
			if target < 0 {
				return 4, 1, true
			}
			if b.State == 5 || b.Target == target {
				return 4, 2, true
			}
			if target > b.Target {
				return 3, 32, true
			}
			return 1, 8, true
		}
		return 4, 1, true
	}
	if revivalWanted(v, m, r) {
		if b.Selected != 1 {
			return 1, 8, true
		}
		return 4, 2, true
	}
	return 0, 0, false
}

func rememberRevivalBotInput(r *battleBotRole, v View, m *battleBotMemory, keys uint32) bool {
	b := v.Battle
	if b.State == 1 && b.Menu == 0 && revivalWanted(v, m, r) && (keys == 8 || keys == 2 && b.Selected == 1) {
		r.Reviving = true
		r.ReviveFailed = false
		r.Pending, r.Moves, r.Settled = 0, 0, 0
		r.Cursor = ""
	}
	if !r.Reviving {
		return false
	}
	if b.State == 1 && b.Menu == 1 {
		if keys == 32 || keys == 8 || keys == 65536 {
			r.Moves++
			r.Pending, r.Settled = keys, 0
			r.Cursor = fieldBotCursorName(v)
		}
		if keys == 0 && r.Pending != 0 {
			r.Settled = min(2, r.Settled+1)
		}
	}
	if keys == 1 {
		r.Reviving = false
		r.ReviveFailed = true
		r.ReviveCheckedMP = v.Party[b.Current].MP
		r.Pending = 0
	}
	return true
}
