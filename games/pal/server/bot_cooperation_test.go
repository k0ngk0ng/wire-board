package pal

// Failure cases before adding the ordinary solo cooperative policy: healthy
// low-mana parties never use original HP-funded cooperation; another seat's
// role or a committed command is overridden; a weak party cooperates instead
// of healing; taking over a human target is mistaken for bot intent; a fresh
// saved cursor loses intent; reclaim/animation retains an old approval;
// one exhausted role repeatedly consumes the whole party's turn while another
// role still has an offensive mana budget, starving its ordinary magic.
import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func soloCoopView() View {
	return View{Phase: "battle", Epoch: 1, Party: []PartyMember{{Role: 0, Controller: 0, HP: 100, MaxHP: 100, MP: 25, MaxMP: 100}, {Role: 2, Controller: 0, HP: 100, MaxHP: 100, MP: 25, MaxMP: 100}}, Battle: &BattleView{State: 1, Current: 0, Players: []engine.BattlePlayer{{Role: 0, State: 1}, {Role: 2, State: 0}}}}
}

func TestBattleBotOriginalSoloCooperationIntent(t *testing.T) {
	v := soloCoopView()
	m := &battleBotMemory{}
	a, ok := battleBotAction(v, 0, m)
	if !ok || a.Keys != 32 {
		t.Fatal("healthy mana-reserved solo party did not select the original cooperation icon", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	b, _ := json.Marshal(m)
	if !strings.Contains(string(b), `"cooping":true`) {
		t.Fatal("cooperation intent was not persisted", string(b))
	}
	v.Battle.Selected = 2
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 2 {
		t.Fatal("selected original cooperation icon was not confirmed", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	v.Battle.State = 4
	v.Battle.Action = 4
	resetBattleBotInput(m)
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 2 {
		t.Fatal("saved solo cooperation target lost its intent", a)
	}
	clearBattleBotOwner(m, v, 0)
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 1 {
		t.Fatal("reclaimed human cooperation target was committed", a)
	}
	v = soloCoopView()
	m = &battleBotMemory{}
	rememberBattleBotInput(m, v, 32)
	v.Battle.Phase = 1
	observeBattleBot(m, v)
	b, _ = json.Marshal(m)
	if strings.Contains(string(b), `"cooping":true`) {
		t.Fatal("completed cooperation intent leaked into another turn")
	}
}

func TestBattleBotSoloCooperationAvailability(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*View)
		want   uint32
	}{
		{"high mana keeps ordinary magic", func(v *View) { v.Party[0].MP = 51; v.Party[1].MP = 51 }, 8},
		{"current caster keeps its own mana budget", func(v *View) { v.Party[0].MP = 100 }, 8},
		{"low mana leader does not starve later caster", func(v *View) { v.Party[1].MP = 80 }, 2},
		{"last caster keeps its own original auto action after an earlier physical command", func(v *View) {
			v.Party[1].MP = 80
			v.Battle.Current = 1
			v.Battle.Players[0].State = 2
			v.Battle.Players[0].Action = 2
			v.Battle.Players[1].State = 1
		}, 32768},
		{"another role owner", func(v *View) { v.Party[1].Controller = 1 }, 2},
		{"other role already committed", func(v *View) { v.Battle.Players[1].State = 2 }, 2},
		{"later role cannot replace previous command", func(v *View) { v.Battle.Current = 1 }, 2},
		{"own earlier physical command may join cooperation", func(v *View) {
			v.Battle.Current = 1
			v.Battle.Players[0].State = 2
			v.Battle.Players[0].Action = 2
			v.Battle.Players[1].State = 1
		}, 32},
		{"earlier healing magic is preserved", func(v *View) {
			v.Battle.Current = 1
			v.Battle.Players[0].State = 2
			v.Battle.Players[0].Action = 3
			v.Battle.Players[1].State = 1
		}, 2},
		{"wounded party heals first", func(v *View) { v.Party[1].HP = 79 }, 8},
		{"visible extra enemies allow an HP-funded sweep", func(v *View) {
			v.Party[0].HP = 60
			v.Party[1].HP = 60
			v.Battle.Enemies = []BattleEnemy{{Object: 469}, {Object: 403}}
		}, 32},
		{"critical role heals before a sweep", func(v *View) { v.Party[1].HP = 59; v.Battle.Enemies = []BattleEnemy{{Object: 469}, {Object: 403}} }, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := soloCoopView()
			tc.change(&v)
			a, ok := battleBotAction(v, 0, &battleBotMemory{})
			if !ok || a.Keys != tc.want {
				t.Fatal("unsafe original cooperation policy", a)
			}
		})
	}
}

func TestBattleBotDoesNotLoopFriendlyCooperation(t *testing.T) {
	v := soloCoopView()
	v.Party[0].MP = 30
	v.Party[1].MP = 30
	m := &battleBotMemory{}
	rememberBattleBotInput(m, v, 32)
	v.Battle.State = 5
	v.Battle.Action = 4
	if a, ok := battleBotAction(v, 0, m); !ok || a.Keys != 1 {
		t.Fatal("offensive policy committed a friendly cooperative spell", a)
	}
	rememberBattleBotInput(m, v, 1)
	v = soloCoopView()
	v.Party[0].MP = 30
	v.Party[1].MP = 30
	resetBattleBotInput(m)
	if a, ok := battleBotAction(v, 0, m); !ok || a.Keys != 2 {
		t.Fatal("friendly cooperation is repeatedly selected", a)
	}
}

func TestBattleBotCooperationConfirmationKeepsItsOwnIntent(t *testing.T) {
	v := soloCoopView()
	v.Party[0].HP = 60
	v.Party[1].HP = 60
	v.Battle.Enemies = []BattleEnemy{{Object: 469}, {Object: 403}}
	m := &battleBotMemory{}
	rememberBattleBotInput(m, v, 32)
	v.Battle.Selected = 2
	rememberBattleBotInput(m, v, 2)
	b, _ := json.Marshal(m)
	if !strings.Contains(string(b), `"cooping":true`) || strings.Contains(string(b), `"healing":true`) {
		t.Fatal("cooperation confirmation became healing intent", string(b))
	}
}

func TestBattleBotCombinesOnlyItsOwnQueuedOffensiveMagic(t *testing.T) {
	v := soloCoopView()
	v.Battle.Current = 1
	v.Battle.Players[0].State = 2
	v.Battle.Players[0].Action = 3
	v.Battle.Players[1].State = 1
	var m battleBotMemory
	if err := json.Unmarshal([]byte(`{"roles":[{"role":0,"offensive":true}]}`), &m); err != nil {
		t.Fatal(err)
	}
	resetBattleBotInput(&m)
	if a, ok := battleBotAction(v, 0, &m); !ok || a.Keys != 32 {
		t.Fatal("own queued offensive magic cannot join original cooperation", a)
	}
	clearBattleBotOwner(&m, v, 0)
	if a, ok := battleBotAction(v, 0, &m); !ok || a.Keys != 2 {
		t.Fatal("takeover replaces an unclassified human magic command", a)
	}
	m.Roles = []battleBotRole{{Role: 0, Healing: true}}
	if a, ok := battleBotAction(v, 0, &m); !ok || a.Keys != 2 {
		t.Fatal("cooperation replaces pending healing", a)
	}
}

func TestBattleBotKeepsAcceptedIntentAcrossInterRoleWait(t *testing.T) {
	v := soloCoopView()
	v.Battle.State = 0
	v.Battle.Players[0].State = 2
	v.Battle.Players[0].Action = 3
	m := &battleBotMemory{Roles: []battleBotRole{{Role: 0, Healing: true}, {Role: 2, Healing: true}}}
	observeBattleBot(m, v)
	if !m.Roles[0].Healing || m.Roles[1].Healing {
		t.Fatal("wait between roles lost the queued healing intent or retained an uncommitted one", m.Roles)
	}
	b := []byte(`{"roles":[{"role":0,"offensive":true}]}`)
	if err := json.Unmarshal(b, m); err != nil {
		t.Fatal(err)
	}
	observeBattleBot(m, v)
	encoded, _ := json.Marshal(m)
	if !strings.Contains(string(encoded), `"offensive":true`) {
		t.Fatal("wait between roles lost an accepted offensive intent", string(encoded))
	}
	v.Battle.Phase = 1
	observeBattleBot(m, v)
	encoded, _ = json.Marshal(m)
	if strings.Contains(string(encoded), `"offensive":true`) {
		t.Fatal("performed action retained old offensive intent")
	}
}

// Before revising the failed group heuristic: original single-target
// cooperation is incorrectly rejected when no AOE version is available;
// all-target semantics change; a human target without bot intent is confirmed.
func TestBattleBotKeepsOriginalCooperativeTargetScope(t *testing.T) {
	for _, enemies := range []int{1, 2, 3} {
		v := soloCoopView()
		v.Party[0].MP = 30
		v.Party[1].MP = 30
		for i := 0; i < enemies; i++ {
			v.Battle.Enemies = append(v.Battle.Enemies, BattleEnemy{Object: i + 1})
		}
		m := &battleBotMemory{}
		rememberBattleBotInput(m, v, 32)
		v.Battle.Action = 4
		for _, state := range []int{2, 4} {
			v.Battle.State = state
			if a, ok := battleBotAction(v, 0, m); !ok || a.Keys != 2 {
				t.Fatal("original offensive cooperation target was rejected", enemies, state, a)
			}
		}
		v.Battle.State = 2
		if a, ok := battleBotAction(v, 0, nil); !ok || a.Keys != 1 {
			t.Fatal("unclassified human cooperation was confirmed", a)
		}
	}
}

func TestRecoveryBotTreatsVisibleBurstRiskEarly(t *testing.T) {
	v := soloCoopView()
	v.Battle.Players = nil
	for _, hp := range []int{64, 65, 79, 80, 100} {
		v.Party[1].HP = hp
		a, ok := battleBotAction(v, 0, nil)
		want := uint32(2)
		if hp < 80 {
			want = 8
		}
		if !ok || a.Keys != want {
			t.Fatal("ordinary recovery threshold missed visible burst risk", hp, a)
		}
	}
}

func TestBattleBotCooperationRefusalIsBounded(t *testing.T) {
	v := soloCoopView()
	m := &battleBotMemory{}
	for i := 0; i < 4; i++ {
		a, ok := battleBotAction(v, 0, m)
		if !ok || a.Keys != 32 {
			t.Fatal("original cooperation icon was not tried", i, a)
		}
		rememberBattleBotInput(m, v, a.Keys)
	}
	if a, ok := battleBotAction(v, 0, m); !ok || a.Keys != 2 {
		t.Fatal("unavailable original cooperation icon loops forever", a)
	}
	for _, n := range []int{-1, 5} {
		var bad battleBotMemory
		b := []byte(fmt.Sprintf(`{"roles":[{"role":0,"coopAttempts":%d}]}`, n))
		if err := json.Unmarshal(b, &bad); err != nil {
			t.Fatal(err)
		}
		if validBattleBotMemory(&bad) {
			t.Fatal("invalid persisted cooperation attempt budget accepted", n)
		}
	}
}
