package pal

import "strings"

type battleBotMemory struct {
	Roles           []battleBotRole `json:"roles,omitempty"`
	OriginalEnemies []int           `json:"originalEnemies,omitempty"`
}
type battleBotRole struct {
	AttackScope      int    `json:"attackScope,omitempty"`
	ProbingAttack    bool   `json:"probingAttack,omitempty"`
	ItemKind         int    `json:"itemKind,omitempty"`
	ItemRole         int    `json:"itemRole,omitempty"`
	ItemHome         bool   `json:"itemHome,omitempty"`
	ItemAttempts     int    `json:"itemAttempts,omitempty"`
	NoItems          int    `json:"noItems,omitempty"`
	Cooping          bool   `json:"cooping,omitempty"`
	CoopDeclined     bool   `json:"coopDeclined,omitempty"`
	Offensive        bool   `json:"offensive,omitempty"`
	OffenseFailed    bool   `json:"offenseFailed,omitempty"`
	OffenseCheckedMP int    `json:"offenseCheckedMP,omitempty"`
	OffenseAttempts  int    `json:"offenseAttempts,omitempty"`
	CoopAttempts     int    `json:"coopAttempts,omitempty"`
	Role             int    `json:"role"`
	Healing          bool   `json:"healing,omitempty"`
	HealingRank      int    `json:"healingRank,omitempty"`
	Reviving         bool   `json:"reviving,omitempty"`
	ReviveFailed     bool   `json:"reviveFailed,omitempty"`
	ReviveCheckedMP  int    `json:"reviveCheckedMP,omitempty"`
	Failed           bool   `json:"failed,omitempty"`
	CheckedMP        int    `json:"checkedMP,omitempty"`
	Pending          uint32 `json:"pending,omitempty"`
	Cursor           string `json:"cursor,omitempty"`
	Settled          int    `json:"settled,omitempty"`
	Moves            int    `json:"moves,omitempty"`
}

func validBattleBotMemory(m *battleBotMemory) bool {
	if m == nil {
		return true
	}
	if len(m.Roles) > 6 {
		return false
	}
	if len(m.OriginalEnemies) > 5 {
		return false
	}
	for _, object := range m.OriginalEnemies {
		if object < 0 || object > 65535 {
			return false
		}
	}
	seen := map[int]bool{}
	for _, r := range m.Roles {
		if r.HealingRank < 0 || r.HealingRank > 50 {
			return false
		}
		if r.ReviveCheckedMP < 0 || r.ReviveCheckedMP > 65535 || r.Reviving && (r.Healing || r.Offensive || r.Cooping || r.ItemKind != 0) {
			return false
		}
		if r.AttackScope < 0 || r.AttackScope > 2 {
			return false
		}
		if r.ItemKind < 0 || r.ItemKind > 2 || r.ItemRole < 0 || r.ItemRole > 5 || r.ItemAttempts < 0 || r.ItemAttempts > 4 || r.NoItems < 0 || r.NoItems > 3 {
			return false
		}
		if r.OffenseCheckedMP < 0 || r.OffenseCheckedMP > 65535 || r.OffenseAttempts < 0 || r.OffenseAttempts > 4 {
			return false
		}
		if r.CoopAttempts < 0 || r.CoopAttempts > 4 {
			return false
		}
		if r.Role < 0 || r.Role > 5 || seen[r.Role] || r.CheckedMP < 0 || r.CheckedMP > 65535 || len(r.Cursor) > 256 || r.Settled < 0 || r.Settled > 2 || r.Moves < 0 || r.Moves > 128 || r.Pending != 0 && r.Pending != 8 && r.Pending != 32 && !(r.Pending == 65536 && (r.ItemKind != 0 || r.Healing || r.Offensive || r.Reviving)) {
			return false
		}
		seen[r.Role] = true
	}
	return true
}
func recoveryRole(m *battleBotMemory, role int) *battleBotRole {
	if m != nil {
		for i := range m.Roles {
			if m.Roles[i].Role == role {
				return &m.Roles[i]
			}
		}
	}
	return nil
}
func recoveryCovered(v View, m *battleBotMemory, target int) bool {
	if v.Battle == nil {
		return false
	}
	for i, p := range v.Battle.Players {
		if i >= len(v.Party) || v.Party[i].Role != p.Role || v.Party[i].HP <= 0 {
			continue
		}

		if r := recoveryRole(m, p.Role); r != nil && r.Healing && p.State == 2 && p.Action == 3 && (p.Target == target || p.Target == -1) {

			if (p.Target == i || p.Target == -1) && v.Party[i].HP*3 <= v.Party[i].MaxHP {
				continue
			}
			return true
		}
		if r := recoveryRole(m, p.Role); r != nil && r.ItemKind == 1 && p.State == 2 && p.Action == 7 && (p.Target == target || p.Target == -1) {
			return true
		}
	}
	return false
}
func recoveryTarget(v View, m *battleBotMemory) int {
	weak := -1
	for i, p := range v.Party {
		if !recoveryNeeded(p) || recoveryCovered(v, m, i) {
			continue
		}
		if weak < 0 || int64(p.HP)*int64(v.Party[weak].MaxHP) < int64(v.Party[weak].HP)*int64(p.MaxHP) {
			weak = i
		}
	}
	if weak >= 0 && v.Party[weak].HP*2 > v.Party[weak].MaxHP {
		for i := range v.Party {
			if recoveryCovered(v, m, i) {

				return -1
			}
		}
	}
	return weak
}

func recoveryNeeded(p PartyMember) bool {
	return p.HP > 0 && p.MaxHP > 0 && p.HP*5 < p.MaxHP*4
}
func recoveryWord(text string) bool {
	for _, word := range []string{"气疗术", "观音咒", "凝神归元", "元灵归心术", "五气朝元"} {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}
func recoveryWanted(v View, m *battleBotMemory, r *battleBotRole) bool {
	b := v.Battle
	if b == nil || b.Current < 0 || b.Current >= len(v.Party) || v.Party[b.Current].MP <= 0 || recoveryTarget(v, m) < 0 {
		return false
	}
	if recoveryDeferred(v, m, r, recoveryTarget(v, m)) {
		return false
	}
	return r == nil || !r.Failed || v.Party[b.Current].MP > r.CheckedMP
}

func recoveryBotKeys(v View, m *battleBotMemory) (int, uint32, bool) {
	b := v.Battle
	r := recoveryRole(m, v.Party[b.Current].Role)
	if r != nil && r.Healing {
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
			if keys, ok := observedSpellKeys(v, true); ok {
				return 4, keys, true
			}
			if recoveryWord(fieldBotVisibleText(v, true)) {
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
		if b.State == 3 || b.State == 5 {
			target := recoveryTarget(v, m)
			if target < 0 {
				return 4, 1, true
			}
			if b.State == 5 || b.Target == target {
				return 4, 2, true
			}
			return 4, 8, true
		}
		return 4, 1, true
	}
	if b.State == 1 && b.Menu == 0 && recoveryWanted(v, m, r) {
		if b.Selected != 1 {
			return 1, 8, true
		}
		return 4, 2, true
	}
	return 0, 0, false
}

func rememberBattleBotInput(m *battleBotMemory, v View, keys uint32) {
	b := v.Battle
	if m == nil || b == nil || b.Current < 0 || b.Current >= len(v.Party) {
		return
	}
	rememberBattleEnemyOrigin(m, v)
	p := v.Party[b.Current]
	r := recoveryRole(m, p.Role)
	if r == nil {
		m.Roles = append(m.Roles, battleBotRole{Role: p.Role})
		r = &m.Roles[len(m.Roles)-1]
	}
	if rememberRevivalBotInput(r, v, m, keys) {
		return
	}
	rememberAttackScopeInput(r, v, keys)
	rememberBattleItemInput(r, v, m, keys)
	if soloCooperationReady(v, m) && (keys == 32 || keys == 2 && b.Selected == 2) {
		r.Cooping = true
		r.Healing = false
		if keys == 32 {
			r.CoopAttempts = min(4, r.CoopAttempts+1)
		}
	} else if b.State == 1 && b.Menu == 0 && (keys == 16 || keys == 8 || keys == 1 || keys == 2 && b.Selected != 2) {
		r.Cooping = false
	}
	if b.Action == 4 && (b.State == 2 || b.State == 3 || b.State == 5) && keys == 1 {
		r.Cooping = false
		r.CoopDeclined = true
	}
	if b.State == 1 && b.Menu == 0 && recoveryWanted(v, m, r) && (keys == 8 || keys == 2 && b.Selected == 1) {
		r.Cooping = false
		r.Healing = true
		r.Offensive = false
		r.Failed = false
		r.Moves = 0
		r.Pending = 0
		r.Cursor = ""
		r.Settled = 0
	}
	rememberOffensiveBotInput(r, v, m, keys)
	rememberHealingRank(r, v, keys)
	if !r.Healing {
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
			r.Healing = false
			r.Failed = true
			r.CheckedMP = p.MP
			r.Pending = 0
		}
	}
	if (b.State == 3 || b.State == 5) && keys == 1 {
		r.Healing = false
		r.Pending = 0
	}
}

func resetBattleBotInput(m *battleBotMemory) {
	if m != nil {
		for i := range m.Roles {
			r := &m.Roles[i]
			if r.ItemKind != 0 {
				r.ItemHome = true
			}
			r.Pending = 0
			r.Cursor = ""
			r.Settled = 0
			r.Moves = 0
		}
	}
}
func clearBattleBotOwner(m *battleBotMemory, v View, actor int) {
	if m != nil {
		for _, p := range v.Party {
			if p.Controller == actor {
				if r := recoveryRole(m, p.Role); r != nil {
					*r = battleBotRole{Role: p.Role}
				}
			}
		}
	}
}
func observeBattleBot(m *battleBotMemory, v View) {
	rememberBattleEnemyOrigin(m, v)
	if m != nil && v.Battle != nil && (v.Battle.Phase != 0 || v.Battle.State == 0) {
		for i := range m.Roles {
			r := &m.Roles[i]
			queued := false
			if v.Battle.Phase == 0 {
				for _, p := range v.Battle.Players {
					queued = queued || p.Role == r.Role && p.State == 2
				}
			}

			if !queued {
				r.ProbingAttack = false
				r.ItemKind = 0
				r.ItemHome = false
				r.ItemAttempts = 0
				r.Cooping = false
				r.Offensive = false
				r.Healing = false
				r.Reviving = false
				r.CoopAttempts = 0
			}
			r.Pending = 0
		}
	}
}
