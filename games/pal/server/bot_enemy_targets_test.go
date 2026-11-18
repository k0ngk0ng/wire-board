package pal

// Failure cases before changing physical target selection: later visible
// enemies accumulate while attacks always confirm the first slot; an empty
// or removed slot is confirmed; the original directional selector is skipped;
// all-target attacks lose their semantics; an unfinished human magic command
// is committed; target navigation acts for another owner or pending input.
import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

// Failure scenarios before persistent target changes: a newly summoned later
// slot steals physical/magic focus; removed or replaced original enemies are
// still targeted; save loses observed history; all-enemy and ally scopes are
// changed; native whole-party Force bypasses the chosen target after a summon;
// malformed saved history is accepted. Only prior public occupancy is used.
func TestBattleBotKeepsObservedOriginalEnemyAcrossSummons(t *testing.T) {
	v := View{Phase: "battle", Party: []PartyMember{{Role: 0, Controller: 0, HP: 100, MaxHP: 100, MP: 20, MaxMP: 100}}, Battle: &BattleView{State: 2, Action: 2, Target: 1, Enemies: []BattleEnemy{{}, {Object: 77}, {}}}}
	m := &battleBotMemory{}
	observeBattleBot(m, v)
	v.Battle.Enemies[2].Object = 55
	v.Battle.Target = 2
	for _, magic := range []bool{false, true} {
		v.Battle.Action = 2
		m.Roles = nil
		if magic {
			v.Battle.Action = 3
			m.Roles = []battleBotRole{{Role: 0, Offensive: true}}
		}
		a, ok := battleBotAction(v, 0, m)
		if !ok || a.Keys != 8 {
			t.Fatal("summon displaced observed original enemy", magic, a)
		}
		v.Battle.Target = 1
		if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 2 {
			t.Fatal("original enemy not confirmed", magic, a)
		}
		v.Battle.State = 4
		if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 2 {
			t.Fatal("all-enemy scope changed", magic, a)
		}
		v.Battle.State = 2
		v.Battle.Target = 2
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var saved battleBotMemory
	if err = json.Unmarshal(b, &saved); err != nil || !validBattleBotMemory(&saved) {
		t.Fatal("observed target history cannot save", err)
	}
	if a, ok := battleBotAction(v, 0, &saved); !ok || a.Keys != 8 {
		t.Fatal("save lost original enemy focus", a)
	}
	v.Battle.Enemies[1].Object = 0
	if a, ok := battleBotAction(v, 0, &saved); !ok || a.Keys != 2 {
		t.Fatal("removed original enemy remains targeted", a)
	}
	v.Battle.Enemies[1].Object = 99
	v.Battle.Target = 1
	if a, ok := battleBotAction(v, 0, &saved); !ok || a.Keys != 32 {
		t.Fatal("replacement impersonated original enemy", a)
	}
}

func TestBattleBotSummonsDoNotCommitNativeForce(t *testing.T) {
	v := View{Phase: "battle", Party: []PartyMember{{Role: 0, Controller: 0, HP: 100, MaxHP: 100, MP: 100, MaxMP: 100}, {Role: 1, Controller: 0, HP: 100, MaxHP: 100, MP: 100, MaxMP: 100}}, Battle: &BattleView{State: 1, Enemies: []BattleEnemy{{}, {Object: 77}, {}}, Players: []engine.BattlePlayer{{Role: 0}, {Role: 1}}}}
	m := &battleBotMemory{}
	observeBattleBot(m, v)
	v.Battle.Enemies[2].Object = 55
	if battleRoundAutoReady(v, 0, m) {
		t.Fatal("native Force bypassed observed target after a summon")
	}
	v.Battle.Current = 1
	v.Battle.Players[0].State = 2
	if battleLastRoleAutoReady(v, 0, m) {
		t.Fatal("last-role Force bypassed observed target after a summon")
	}
	for _, text := range []string{`{"originalEnemies":[-1]}`, `{"originalEnemies":[65536]}`, `{"originalEnemies":[1,2,3,4,5,6]}`} {
		var bad battleBotMemory
		if err := json.Unmarshal([]byte(text), &bad); err != nil {
			t.Fatal(err)
		}
		if validBattleBotMemory(&bad) {
			t.Fatal("invalid saved enemy history accepted", text)
		}
	}
}

func TestBattleBotUsesVisibleOccupiedEnemySlots(t *testing.T) {
	v := View{Phase: "battle", Party: []PartyMember{{Role: 0, Controller: 0, HP: 100, MaxHP: 100, MP: 20, MaxMP: 100}}, Battle: &BattleView{State: 2, Action: 2, Target: 1, Enemies: []BattleEnemy{{}, {Object: 469}, {}, {Object: 403}, {}}}}
	for target := 0; target < 5; target++ {
		v.Battle.Target = target
		a, ok := battleBotAction(v, 0, nil)
		want := uint32(32)
		if target > 3 {
			want = 8
		}
		if target == 3 {
			want = 2
		}
		if !ok || a.Keys != want {
			t.Fatal("physical target ignored visible occupied slots", target, a)
		}
	}
	v.Battle.Enemies[3].Object = 0
	v.Battle.Target = 1
	if a, ok := battleBotAction(v, 0, nil); !ok || a.Keys != 2 {
		t.Fatal("removed enemy remained the target", a)
	}
	v.Battle.State = 4
	if a, ok := battleBotAction(v, 0, nil); !ok || a.Keys != 2 {
		t.Fatal("original all-target physical attack changed", a)
	}
	v.Battle.State = 2
	v.Battle.Action = 3
	if a, ok := battleBotAction(v, 0, nil); !ok || a.Keys != 1 {
		t.Fatal("target policy committed unfinished human magic", a)
	}
	v.Battle.Action = 2
	v.Seq = 1
	if a, ok := battleBotAction(v, 0, nil); ok {
		t.Fatal("target policy ran ahead of pending input", a)
	}
	v.Seq = 0
	if a, ok := battleBotAction(v, 1, nil); ok {
		t.Fatal("target policy controlled another owner", a)
	}
}
