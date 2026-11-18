package pal

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestRecoveryDefersModerateWoundsToObservedStrongerLaterHealer(t *testing.T) {
	v := soloCoopView()
	v.Party[0].HP, v.Party[0].MP = 65, 80
	v.Party[1].HP, v.Party[1].MP = 70, 90
	v.Battle.Current = 0
	v.Battle.Players[0].State, v.Battle.Players[1].State = 1, 0
	m := &battleBotMemory{Roles: []battleBotRole{{Role: v.Party[0].Role, HealingRank: 10}, {Role: v.Party[1].Role, HealingRank: 20}}}
	if recoveryWanted(v, m, &m.Roles[0]) {
		t.Fatal("weaker early healer consumed the stronger later healer's role")
	}
	resetBattleBotInput(m)
	if m.Roles[1].HealingRank != 20 || recoveryWanted(v, m, &m.Roles[0]) {
		t.Fatal("save release lost observed healing priority")
	}
	for name, change := range map[string]func(*View, *battleBotMemory){
		"unobserved":      func(_ *View, m *battleBotMemory) { m.Roles[1].HealingRank = 0 },
		"equal":           func(_ *View, m *battleBotMemory) { m.Roles[1].HealingRank = 10 },
		"critical-target": func(v *View, _ *battleBotMemory) { v.Party[0].HP = 20 },
		"critical-healer": func(v *View, _ *battleBotMemory) { v.Party[1].HP = 20 },
		"low-mana":        func(v *View, _ *battleBotMemory) { v.Party[1].MP = 10 },
		"other-owner":     func(v *View, _ *battleBotMemory) { v.Party[1].Controller = 1 },
		"committed":       func(v *View, _ *battleBotMemory) { v.Battle.Players[1].State = 2 },
		"failed":          func(v *View, m *battleBotMemory) { m.Roles[1].Failed = true; m.Roles[1].CheckedMP = v.Party[1].MP },
		"offensive":       func(_ *View, m *battleBotMemory) { m.Roles[1].Offensive = true },
		"ongoing-heal":    func(_ *View, m *battleBotMemory) { m.Roles[0].Healing = true },
	} {
		t.Run(name, func(t *testing.T) {
			copy := v
			copy.Party = append([]PartyMember(nil), v.Party...)
			battle := *v.Battle
			battle.Players = append([]engine.BattlePlayer(nil), v.Battle.Players...)
			copy.Battle = &battle
			memory := &battleBotMemory{Roles: append([]battleBotRole(nil), m.Roles...)}
			change(&copy, memory)
			if !recoveryWanted(copy, memory, &memory.Roles[0]) {
				t.Fatal("unsafe future healing displaced current recovery")
			}
		})
	}
}

func TestRecoveryLearnsHealingRankOnlyFromSelectedVisibleSpell(t *testing.T) {
	v := recoveryView()
	v.Battle.Menu = 1
	m := &battleBotMemory{Roles: []battleBotRole{{Role: v.Party[1].Role, Healing: true}}}
	for i, character := range []rune("观音咒") {
		v.Overlay = append(v.Overlay, engine.UIOp{Kind: 0, ID: int(character), X: 35 + i*16, Y: 54, Color: 250})
	}
	rememberBattleBotInput(m, v, 32)
	if m.Roles[0].HealingRank != 0 {
		t.Fatal("merely visiting a healing spell recorded a committed capability")
	}
	rememberBattleBotInput(m, v, 2)
	if m.Roles[0].HealingRank != 20 {
		t.Fatal("selected original visible healing was not remembered")
	}
	for _, payload := range []string{`{"roles":[{"role":0,"healingRank":-1}]}`, `{"roles":[{"role":0,"healingRank":51}]}`} {
		var memory battleBotMemory
		if err := json.Unmarshal([]byte(payload), &memory); err != nil || validBattleBotMemory(&memory) {
			t.Fatal("invalid healing-rank memory was accepted", payload, err)
		}
	}
}

func TestObservedHealingSpecialistAllowsTheOtherRoleToUseItsRemainingOffensiveMana(t *testing.T) {
	v := soloCoopView()
	v.Party[0].HP, v.Party[0].MP = 65, 35
	v.Party[1].HP, v.Party[1].MP = 70, 90
	v.Battle.Current = 0
	v.Battle.Players[0].State, v.Battle.Players[1].State = 1, 0
	m := &battleBotMemory{Roles: []battleBotRole{{Role: v.Party[0].Role, HealingRank: 10}, {Role: v.Party[1].Role, HealingRank: 20}}}
	if _, keys, ok := offensiveBotKeys(v, m); !ok || keys != 8 {
		t.Fatal("damage role reserved half its mana despite a known stronger healer", keys, ok)
	}
	m.Roles[1].HealingRank = 0
	if _, _, ok := offensiveBotKeys(v, m); ok {
		t.Fatal("unobserved healing bypassed the ordinary mana reserve")
	}
	m.Roles[1].HealingRank = 20
	v.Party[0].HP = 20
	if _, _, ok := offensiveBotKeys(v, m); ok {
		t.Fatal("critical recovery was replaced by extra offensive magic")
	}
}
