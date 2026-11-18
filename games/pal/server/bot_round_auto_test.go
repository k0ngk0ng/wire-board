package pal

// Original Classic Force commits the whole party. Before using it at a safe
// round boundary, reject: another owner, any wound/mana reserve, an accepted
// command, a later role, active healing/item/magic intent, or an observed group
// weapon that must remain available. No native rule data enters the policy.
import "testing"

func TestBattleBotOriginalAutoOnlyAtAHealthyRoundStart(t *testing.T) {
	v := soloCoopView()
	v.Party[0].MP, v.Party[1].MP = 100, 100
	if a, ok := battleBotAction(v, 0, nil); !ok || a.Keys != 32768 {
		t.Fatal("healthy untouched solo round did not use original auto selection", a)
	}
	for _, change := range []func(*View){
		func(v *View) { v.Party[1].Controller = 1 },
		func(v *View) { v.Party[1].HP-- },
		func(v *View) { v.Party[1].MP = 50 },
		func(v *View) { v.Battle.Current = 1 },
		func(v *View) { v.Battle.Players[0].State = 2 },
	} {
		x := soloCoopView()
		x.Party[0].MP, x.Party[1].MP = 100, 100
		change(&x)
		if a, _ := battleBotAction(x, 0, nil); a.Keys == 32768 {
			t.Fatal("whole-party auto overwrote an independent decision", x)
		}
	}
	m := &battleBotMemory{Roles: []battleBotRole{{Role: 0, Healing: true}}}
	if a, _ := battleBotAction(v, 0, m); a.Keys == 32768 {
		t.Fatal("whole-party auto replaced selected healing")
	}
	for _, action := range controls().Actions {
		if action.ID == "force" && action.Label != "自动选招" {
			t.Fatal("original whole-party shortcut is labeled as one role", action.Label)
		}
	}
}

// Original Force affects the current and later uncommitted roles; it does not
// rewrite earlier accepted actions. At the last role it can select a stronger
// original spell while the earlier healing command stays intact. Reject a
// critical uncovered ally, low mana, another owner or unfinished earlier role.
func TestBattleBotLastRoleAutoPreservesEarlierHealing(t *testing.T) {
	v := soloCoopView()
	v.Battle.Current = 1
	v.Party[0].HP, v.Party[1].HP = 60, 65
	v.Party[0].MP, v.Party[1].MP = 100, 100
	v.Battle.Players[0].State = 2
	v.Battle.Players[0].Action = 3
	v.Battle.Players[0].Target = 0
	v.Battle.Players[1].State = 1
	m := &battleBotMemory{Roles: []battleBotRole{{Role: 0, Healing: true}}}
	before := v.Battle.Players[0]
	if a, ok := battleBotAction(v, 0, m); !ok || a.Keys != 32768 {
		t.Fatal("last role did not use the original independent auto selection", a)
	}
	if v.Battle.Players[0] != before || !m.Roles[0].Healing {
		t.Fatal("last role replaced the accepted healing command")
	}
	v.Party[1].HP = 40
	if a, _ := battleBotAction(v, 0, m); a.Keys == 32768 {
		t.Fatal("last-role auto displaced critical recovery")
	}
	v.Party[1].HP = 65
	v.Party[1].MP = 25
	if a, _ := battleBotAction(v, 0, m); a.Keys == 32768 {
		t.Fatal("last-role auto exhausted the mana reserve")
	}
}
