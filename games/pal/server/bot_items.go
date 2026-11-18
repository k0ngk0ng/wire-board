package pal

import (
	"regexp"
	"strconv"
	"strings"
)

var battleItemHP = regexp.MustCompile(`HP(?:MP)?\+([0-9]+)`)
var battleItemMP = regexp.MustCompile(`(?:HP)?MP\+([0-9]+)`)

func recoveryItemAmount(v View, kind int) int {
	pattern := battleItemHP
	if kind == 2 {
		pattern = battleItemMP
	}
	text := strings.ReplaceAll(strings.ReplaceAll(fieldBotVisibleText(v, false), " ", ""), "\n", "")
	match := pattern.FindStringSubmatch(text)
	if len(match) != 2 {
		return 0
	}
	n, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return n
}

func battleItemWanted(v View, m *battleBotMemory, r *battleBotRole) (int, int) {
	b := v.Battle
	p := v.Party[b.Current]
	if r == nil || b.State != 1 || b.Menu != 0 || p.MaxMP <= 0 || r.Healing || r.Offensive || r.Cooping {
		return 0, 0
	}
	if r.NoItems&1 == 0 && (p.MP*2 <= p.MaxMP || r.Failed && p.MP <= r.CheckedMP) {
		target := -1
		for i, ally := range v.Party {
			if ally.HP > 0 && ally.MaxHP > 0 && ally.HP*2 <= ally.MaxHP && !recoveryCovered(v, m, i) {
				if target < 0 || ally.HP*v.Party[target].MaxHP < v.Party[target].HP*ally.MaxHP {
					target = i
				}
			}
		}
		if target >= 0 {
			return 1, v.Party[target].Role
		}
	}
	if r.NoItems&2 == 0 && p.MP*4 <= p.MaxMP {
		return 2, p.Role
	}
	return 0, 0
}

func battleItemBotKeys(v View, m *battleBotMemory) (int, uint32, bool) {
	b := v.Battle
	r := recoveryRole(m, v.Party[b.Current].Role)
	if r != nil && r.ItemKind != 0 {
		if b.State == 1 {
			if b.Menu == 0 {
				if r.ItemAttempts >= 4 {
					return 2, 16, true
				}
				return 4, 2048, true
			}
			if b.Menu == 2 {
				if r.ItemHome {
					return 4, 65536, true
				}
				if r.Pending != 0 && r.Settled < 2 {
					return 4, 0, true
				}
				target := -1
				for i, p := range v.Party {
					if p.Role == r.ItemRole && p.HP > 0 {
						target = i
					}
				}
				if target < 0 {
					return 4, 1, true
				}
				p := v.Party[target]
				missing, minimum := p.MaxHP-p.HP, max(1, p.MaxHP/3)
				if r.ItemKind == 2 {
					missing, minimum = p.MaxMP-p.MP, max(1, p.MaxMP/4)
				}
				if fieldBotVisibleText(v, true) != "" && recoveryItemAmount(v, r.ItemKind) >= min(missing, minimum) {
					return 4, 2, true
				}
				if r.Moves >= 128 || r.Pending == 32 && r.Cursor != "" && r.Cursor == fieldBotCursorName(v) {
					return 4, 1, true
				}
				return 4, 32, true
			}
			return 4, 1, true
		}
		if (b.State == 3 || b.State == 5) && b.Action == 7 {
			target := -1
			for i, p := range v.Party {
				if p.Role == r.ItemRole && p.HP > 0 {
					target = i
				}
			}
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
	}
	if kind, _ := battleItemWanted(v, m, r); kind != 0 {
		return 4, 2048, true
	}
	return 0, 0, false
}

func rememberBattleItemInput(r *battleBotRole, v View, m *battleBotMemory, keys uint32) {
	b := v.Battle
	if b.State == 1 && b.Menu == 0 && keys == 2048 {
		if r.ItemKind == 0 {
			kind, role := battleItemWanted(v, m, r)
			if kind != 0 {
				r.ItemKind = kind
				r.ItemRole = role
				r.ItemHome = true
				r.ItemAttempts = 0
				r.Healing = false
				r.Offensive = false
				r.Cooping = false
				r.Pending = 0
				r.Cursor = ""
				r.Moves = 0
				r.Settled = 0
			}
		}
		if r.ItemKind != 0 {
			r.ItemAttempts = min(4, r.ItemAttempts+1)
		}
	}
	if r.ItemKind == 0 {
		return
	}
	failed := false
	if b.State == 1 && b.Menu == 0 && keys == 16 && r.ItemAttempts >= 4 {
		failed = true
	}
	if b.State == 1 && b.Menu == 2 {
		if keys == 65536 {
			r.ItemHome = false
			r.Pending = keys
			r.Cursor = ""
			r.Settled = 0
		}
		if keys == 32 || keys == 8 {
			r.Moves = min(128, r.Moves+1)
			r.Pending = keys
			r.Cursor = fieldBotCursorName(v)
			r.Settled = 0
		}
		if keys == 0 && r.Pending != 0 {
			r.Settled = min(2, r.Settled+1)
		}
		if keys == 1 {
			failed = true
		}
	}
	if (b.State == 3 || b.State == 5) && b.Action == 7 && keys == 1 {
		failed = true
	}
	if failed {
		r.NoItems |= r.ItemKind
		r.ItemKind = 0
		r.ItemHome = false
		r.Pending = 0
	}
}
