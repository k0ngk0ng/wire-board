package pal

// Failure cases before observing physical attack scope: a naturally all-target
// weapon is never used because the bot always opens magic; an exploratory
// single-target selector is committed instead of returning to the same role;
// a human target is mistaken for a probe; save drops the pending probe; a
// known all-target attack displaces healing; invalid persisted scope is trusted.
import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func scopeView() View {
	return View{Phase: "battle", Party: []PartyMember{{Role: 0, Controller: 0, HP: 100, MaxHP: 100, MP: 100, MaxMP: 100}}, Battle: &BattleView{State: 1, Enemies: []BattleEnemy{{Object: 1}, {Object: 2}}, Players: []engine.BattlePlayer{{Role: 0, State: 1}}}}
}

func TestBattleBotObservesOriginalAttackScope(t *testing.T) {
	v := scopeView()
	m := &battleBotMemory{}
	a, ok := battleBotAction(v, 0, m)
	if !ok || a.Keys != 2 {
		t.Fatal("bot never observes the ordinary physical selector", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	v.Battle.State = 2
	v.Battle.Action = 2
	resetBattleBotInput(m)
	a, ok = battleBotAction(v, 0, m)
	if !ok || a.Keys != 1 {
		t.Fatal("saved single-target probe spent an action", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	v.Battle.State = 1
	v.Battle.Selected = 0
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 8 {
		t.Fatal("known single-target attack prevented original magic", a)
	}
	v = scopeView()
	m = &battleBotMemory{}
	rememberBattleBotInput(m, v, 2)
	v.Battle.State = 4
	v.Battle.Action = 2
	a, ok = battleBotAction(v, 0, m)
	if !ok || a.Keys != 2 {
		t.Fatal("original all-target attack was rejected", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	v.Battle.State = 1
	v.Battle.Action = 0
	v.Battle.Selected = 1
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 16 {
		t.Fatal("known all-target weapon was not selected", a)
	}
	v.Battle.Selected = 0
	v.Party[0].HP = 60
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 8 {
		t.Fatal("all-target attack displaced necessary healing", a)
	}
	for _, scope := range []int{-1, 3} {
		var bad battleBotMemory
		b := []byte(fmt.Sprintf(`{"roles":[{"role":0,"attackScope":%d}]}`, scope))
		if err := json.Unmarshal(b, &bad); err != nil {
			t.Fatal(err)
		}
		if validBattleBotMemory(&bad) {
			t.Fatal("invalid observed scope accepted", scope)
		}
	}
}

func TestBattleBotPreservesAnObservedGroupWeapon(t *testing.T) {
	v := soloCoopView()
	v.Party[0].MP = 30
	v.Party[1].MP = 30
	v.Battle.Enemies = []BattleEnemy{{Object: 1}, {Object: 2}}
	var m battleBotMemory
	if err := json.Unmarshal([]byte(`{"roles":[{"role":0,"attackScope":1},{"role":2,"attackScope":2}]}`), &m); err != nil {
		t.Fatal(err)
	}
	if a, ok := battleBotAction(v, 0, &m); !ok || a.Keys != 2 {
		t.Fatal("cooperation suppressed a known ordinary all-target attack", a)
	}
}

// A queued heal already protects the all-target wielder. When both roles
// remain above half health, returning that role to healing the other forever
// prevents any group attack. Critical/uncovered situations must still heal.
func TestBattleBotCoveredRoleCanUseItsGroupAttack(t *testing.T) {
	v := soloCoopView()
	v.Battle.Current = 1
	v.Party[0].HP = 60
	v.Party[1].HP = 60
	v.Party[1].MP = 100
	v.Battle.Enemies = []BattleEnemy{{Object: 1}, {Object: 2}}
	v.Battle.Players[0].State = 2
	v.Battle.Players[0].Action = 3
	v.Battle.Players[0].Target = 1
	v.Battle.Players[1].State = 1
	m := &battleBotMemory{Roles: []battleBotRole{{Role: 0, Healing: true}, {Role: 2, AttackScope: 2}}}
	if a, ok := battleBotAction(v, 0, m); !ok || a.Keys != 2 {
		t.Fatal("covered group wielder kept the party in a mutual healing loop", a)
	}
	if v.Battle.Players[0].Action != 3 || !m.Roles[0].Healing {
		t.Fatal("group attack changed the earlier accepted heal")
	}
	v.Party[0].HP = 20
	if a, ok := battleBotAction(v, 0, m); !ok || a.Keys != 8 {
		t.Fatal("group attack displaced a critical uncovered ally's healing", a)
	}
	v.Party[0].HP = 60
	m.Roles[0].Healing = false
	if a, ok := battleBotAction(v, 0, m); !ok || a.Keys != 8 {
		t.Fatal("uncovered injured party attacked instead of healing", a)
	}
}
