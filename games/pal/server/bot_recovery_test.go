package pal

// Before recovery implementation: wrong owner/queued input acts; an unfinished
// human spell is confirmed; stale glyph selection casts an unrelated spell;
// absent/unaffordable healing loops forever; role/slot reorder heals the wrong
// ally; save/release loses intent; reclaim confirms the human's old selection;
// malformed persisted memory or oversized cursor history is accepted.
import (
	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	"testing"
)

func engineGlyph(code, color int) engine.UIOp { return engine.UIOp{Kind: 0, ID: code, Color: color} }

func recoveryView() View {
	return View{Phase: "battle", Epoch: 5, ControlAt: 1000, Party: []PartyMember{{Role: 0, Controller: 0, HP: 30, MaxHP: 180, MP: 100}, {Role: 1, Controller: 1, HP: 240, MaxHP: 240, MP: 240}}, Battle: &BattleView{Phase: 0, State: 1, Menu: 0, Current: 1}}
}

func TestRecoveryBotPublicIntentAndTarget(t *testing.T) {
	v := recoveryView()
	if a, ok := battleBotAction(v, 0, nil); ok {
		t.Fatal("teammate acts", a)
	}
	a, ok := battleBotAction(v, 1, nil)
	if !ok || a.Direction != 1 || a.Keys != 8 {
		t.Fatal("injured party did not open original magic icon", a)
	}
	v.Battle.Menu = 1
	if a, _ := battleBotAction(v, 1, nil); a.Keys != 1 {
		t.Fatal("bot confirmed unfinished human spell", a)
	}
	m := &battleBotMemory{Roles: []battleBotRole{{Role: 1, Healing: true}}}
	for _, r := range []rune("观音咒") {
		v.Overlay = append(v.Overlay, engineGlyph(int(r), 250))
	}
	if a, _ := battleBotAction(v, 1, m); a.Keys != 2 {
		t.Fatal("visible affordable healing was not selected", a)
	}
	m.Roles[0].Pending = 32
	if a, _ := battleBotAction(v, 1, m); a.Keys != 0 {
		t.Fatal("stale cursor confirmed before redraw", a)
	}
	m.Roles[0].Pending = 0
	v.Battle.Menu = 0
	v.Battle.State = 3
	v.Battle.Target = 1
	if a, _ := battleBotAction(v, 1, m); a.Keys != 8 {
		t.Fatal("weak visible ally slot not selected", a)
	}
	v.Battle.Target = 0
	if a, _ := battleBotAction(v, 1, m); a.Keys != 2 {
		t.Fatal("weak ally not confirmed", a)
	}
	v.Battle.State = 1
	v.Battle.Menu = 0
	m.Roles[0] = battleBotRole{Role: 1, Failed: true, CheckedMP: 240}
	if a, _ := battleBotAction(v, 1, m); a.Keys != 8 {
		t.Fatal("absent heal did not fall back", a)
	}
	resetBattleBotInput(m)
	m.Roles[0].Healing = true
	m.Roles[0].Pending = 32
	resetBattleBotInput(m)
	if !m.Roles[0].Healing || m.Roles[0].Pending != 0 {
		t.Fatal("save release lost healing intent")
	}
	clearBattleBotOwner(m, v, 1)
	if m.Roles[0].Healing {
		t.Fatal("manual reclaim retained healing approval")
	}
}

// Failures to isolate before changing coordination: duplicate healing of an
// already covered ally; skipping another wounded ally; treating a queued
// attack or an uncommitted/stale spell as healing; all-target healing covering
// only one ally; save input release forgetting a committed heal; or a previous
// round's heal suppressing the next round's recovery.
func TestRecoveryBotCoordinatesCommittedHealing(t *testing.T) {
	v := recoveryView()
	v.Battle.Players = []engine.BattlePlayer{
		{Role: 0, State: 2, Action: 3, Target: 0},
		{Role: 1, State: 1},
	}
	m := &battleBotMemory{Roles: []battleBotRole{{Role: 0, Healing: true}}}
	assertKey := func(want uint32) {
		t.Helper()
		if a, ok := battleBotAction(v, 1, m); !ok || a.Keys != want {
			t.Fatalf("coordination keys=%d, want %d (action=%+v)", a.Keys, want, a)
		}
	}
	assertKey(8)
	resetBattleBotInput(m)
	assertKey(8)
	v.Party[1].HP = 80
	assertKey(8) // The other wounded ally is still uncovered.
	v.Battle.Players[0].Target = -1
	assertKey(8)
	v.Battle.Players[0].Action = 2
	assertKey(8)
	v.Battle.Players[0].Action = 3
	v.Battle.Players[0].State = 0
	assertKey(8)
	v.Battle.Players[0].State = 2
	v.Battle.Phase = 1
	observeBattleBot(m, v)
	v.Battle.Phase = 0
	assertKey(8)
}

// Actual ambush replay exposed a stalemate: both roles repeatedly heal while
// neither damages the enemies. One accepted heal permits an offensive action
// when all other allies remain above half health; critical or uncommitted
// recovery must still take precedence, including a single remaining enemy.
func TestRecoveryBotKeepsAnOffensiveActionDuringModerateRecovery(t *testing.T) {
	v := soloCoopView()
	v.Party[0].HP, v.Party[1].HP = 60, 65
	v.Party[0].MP, v.Party[1].MP = 100, 100
	v.Battle.Current = 1
	v.Battle.Players[0].State = 2
	v.Battle.Players[0].Action = 3
	v.Battle.Players[0].Target = 0
	v.Battle.Players[1].State = 1
	m := &battleBotMemory{Roles: []battleBotRole{{Role: 0, Healing: true}}}
	if target := recoveryTarget(v, m); target != -1 {
		t.Fatal("moderate recovery still commits every role to healing", target)
	}
	v.Party[1].HP = 40
	if target := recoveryTarget(v, m); target != 1 {
		t.Fatal("offense displaced a critical uncovered ally", target)
	}
	v.Party[1].HP = 65
	v.Battle.Players[0].State = 1
	if target := recoveryTarget(v, m); target != 0 {
		t.Fatal("an uncommitted heal suppressed necessary recovery", target)
	}
}

// A critical self-healer may fall before its accepted spell executes. A later
// healthy ally must not treat that promise as sufficient protection, nor may
// a dead caster's old action suppress recovery. Healthy committed recovery
// and ordinary moderate-wound coordination must remain intact across release.
func TestRecoveryBotDoesNotRelyOnCriticalSelfHealing(t *testing.T) {
	v := soloCoopView()
	v.Party[0].HP, v.Party[0].MaxHP, v.Party[0].MP = 8, 180, 57
	v.Party[1].HP, v.Party[1].MaxHP, v.Party[1].MP = 123, 240, 166
	v.Battle.Current = 1
	v.Battle.Players[0].State, v.Battle.Players[0].Action, v.Battle.Players[0].Target = 2, 3, 0
	v.Battle.Players[1].State = 1
	m := &battleBotMemory{Roles: []battleBotRole{{Role: 0, Healing: true}}}
	for _, release := range []bool{false, true} {
		if release {
			resetBattleBotInput(m)
		}
		if recoveryCovered(v, m, 0) {
			t.Fatal("critical self-healing was treated as reliable protection")
		}
		if a, ok := battleBotAction(v, 0, m); !ok || a.Keys != 8 {
			t.Fatal("healthy ally skipped a critical self-healer", a)
		}
	}
	v.Party[0].HP = 100
	if !recoveryCovered(v, m, 0) {
		t.Fatal("healthy accepted healing was discarded")
	}
	v.Party[0].HP = 0
	if recoveryCovered(v, m, 0) {
		t.Fatal("dead caster's queued spell still covers an ally")
	}
}
