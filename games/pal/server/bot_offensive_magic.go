package pal

import "strings"

func offensiveWord(text string) bool {
	for _, word := range []string{"御剑术", "气剑指", "剑气斩", "万剑诀", "天剑", "一阳指", "七诀剑气", "斩龙诀", "风咒", "旋风咒", "风卷残云", "雷咒", "五雷咒", "天雷破", "狂雷", "冰咒", "玄冰咒", "风雪冰天", "炎咒", "三昧真火", "炎杀咒", "炼狱真火", "土咒", "飞岩术", "地裂天崩", "万蛊蚀天", "万蚁蚀象", "剑神", "武神", "雷神", "雪妖", "火神", "山神", "风神"} {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func offensiveWanted(v View, m *battleBotMemory, r *battleBotRole) bool {
	p := v.Party[v.Battle.Current]
	return p.MP > 0 && (p.MaxMP <= 0 || p.MP*2 > p.MaxMP || recoveryDeferred(v, m, r, recoveryTarget(v, m))) && (r == nil || !r.OffenseFailed || p.MP > r.OffenseCheckedMP)
}

func offensiveBotKeys(v View, m *battleBotMemory) (int, uint32, bool) {
	b := v.Battle
	r := recoveryRole(m, v.Party[b.Current].Role)
	if r != nil && r.Offensive {
		if b.State == 1 {
			if b.Menu == 0 {
				if r.OffenseAttempts >= 4 {
					return 2, 16, true
				}
				if b.Selected != 1 {
					return 1, 8, true
				}
				return 4, 2, true
			}
			if b.Menu == 1 {
				if r.Pending != 0 && r.Settled < 2 {
					return 4, 0, true
				}
				if r.Moves == 0 && r.Pending == 0 && !offensiveWord(fieldBotVisibleText(v, true)) {
					return 4, 65536, true
				}
				if keys, ok := observedSpellKeys(v, false); ok {
					return 4, keys, true
				}
				if offensiveWord(fieldBotVisibleText(v, true)) {
					return 4, 2, true
				}
				if r.Moves == 0 && r.Pending == 0 {
					return 4, 65536, true
				}
				if r.Moves >= 128 || r.Pending == 32 && r.Cursor != "" && r.Cursor == fieldBotCursorName(v) {
					return 4, 1, true
				}
				return 4, 32, true
			}
		}
		if (b.State == 2 || b.State == 4) && b.Action == 3 {
			return battleEnemyTargetKeys(v, m)
		}
		if (b.State == 3 || b.State == 5) && b.Action == 3 {
			return 4, 1, true
		}
	}
	if b.State == 1 && b.Menu == 0 && offensiveWanted(v, m, r) {
		if b.Selected != 1 {
			return 1, 8, true
		}
		return 4, 2, true
	}
	return 0, 0, false
}

func rememberOffensiveBotInput(r *battleBotRole, v View, m *battleBotMemory, keys uint32) {
	b := v.Battle
	p := v.Party[b.Current]
	if b.State == 1 && b.Menu == 0 {
		if offensiveWanted(v, m, r) && !r.Healing && (keys == 8 || keys == 2 && b.Selected == 1) {
			if !r.Offensive {
				r.OffenseAttempts = 0
				r.Moves = 0
				r.Pending = 0
				r.Cursor = ""
				r.Settled = 0
			}
			r.Offensive = true
			r.OffenseFailed = false
			if keys == 2 {
				r.OffenseAttempts = min(4, r.OffenseAttempts+1)
			}
		}
		if r.Offensive && keys == 16 && r.OffenseAttempts >= 4 {
			r.Offensive = false
			r.OffenseFailed = true
			r.OffenseCheckedMP = p.MP
		}
	}
	if !r.Offensive {
		return
	}
	if b.State == 1 && b.Menu == 1 {
		if keys == 32 || keys == 8 || keys == 65536 {
			r.Moves = min(128, r.Moves+1)
			r.Pending = keys
			r.Cursor = fieldBotCursorName(v)
			r.Settled = 0
		}
		if keys == 0 && r.Pending != 0 {
			r.Settled = min(2, r.Settled+1)
		}
		if keys == 1 {
			r.Offensive = false
			r.OffenseFailed = true
			r.OffenseCheckedMP = p.MP
			r.Pending = 0
		}
	}
	if (b.State == 3 || b.State == 5) && keys == 1 {
		r.Offensive = false
		r.OffenseFailed = true
		r.OffenseCheckedMP = p.MP
		r.Pending = 0
	}
}
