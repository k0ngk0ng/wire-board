package pal

import "testing"

func TestOffensiveMagicFocusesTheSamePublicEnemyAsPhysicalAttack(t *testing.T) {
	v := offensiveView()
	v.Battle.State, v.Battle.Action, v.Battle.Target = 2, 3, 0
	v.Battle.Enemies = []BattleEnemy{{Object: 495}, {Object: 485}, {Object: 495}}
	m := &battleBotMemory{Roles: []battleBotRole{{Role: 0, Offensive: true}}, OriginalEnemies: []int{495, 485, 495}}
	if direction, keys, ok := offensiveBotKeys(v, m); !ok || direction != 3 || keys != 32 {
		t.Fatal("ordinary single-target magic did not focus the public target", direction, keys)
	}
	v.Battle.Target = 2
	if _, keys, ok := offensiveBotKeys(v, m); !ok || keys != 2 {
		t.Fatal("matching original spell target did not commit", keys)
	}
	v.Battle.Enemies[2].Object = 0
	if direction, keys, ok := offensiveBotKeys(v, m); !ok || direction != 1 || keys != 8 {
		t.Fatal("spell retained a disappeared public enemy", direction, keys)
	}
	v.Battle.State = 4
	if _, keys, ok := offensiveBotKeys(v, m); !ok || keys != 2 {
		t.Fatal("all-target magic was converted to a single target", keys)
	}
}
