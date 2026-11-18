package engine

import "testing"

func TestCoreRecordedRangeRegionsExcludeTheirEmptySpaceE2E(t *testing.T) {
	for _, scenario := range []struct {
		shape  string
		x      float64
		y      float64
		inside bool
	}{
		{"star", 80, 50, false},
		{"star", 100, 20, true},
		{"star", 70, 60, true},
		{"vShape", 0, -50, false},
		{"vShape", 60, -55, true},
		{"vShape", 70, 65, false},
		{"vShape", 0, 99, true},
		{"xShape", 50, 26, false},
	} {
		actual := inAttackRange(&Tower{}, Species{RangeType: scenario.shape}, &Enemy{X: scenario.x, Y: scenario.y}, 100, 1)
		if actual != scenario.inside {
			t.Errorf("recorded region %s (%v,%v) = %v, want %v", scenario.shape, scenario.x, scenario.y, actual, scenario.inside)
		}
	}
}

func TestCoreWideLensExpandsCrossWidthWithoutExtendingItsArmsE2E(t *testing.T) {
	state := coreMotionOwner(t, "cubone")
	state.Team[0].Item = "wideLens"
	state.Inventory = append(state.Inventory, OwnedItem{ID: "wideLens", EquippedBy: state.Team[0].ID})
	tower := &state.Towers[0]
	for _, point := range []Point{{X: tower.X + 100, Y: tower.Y + 40}, {X: tower.X + 300, Y: tower.Y}} {
		id, err := SpawnEnemyAt(state, "rattata", point, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		enemyFor(state, id).Entered = true
	}
	profile, err := profileFor(state, tower, &state.Team[0], nil, .5)
	if err != nil {
		t.Fatal(err)
	}
	targets := selectTargets(state, tower, &state.Team[0], profile)
	if profile.Range != state.Team[0].Stats.Range || len(targets) != 1 || targets[0].Y != tower.Y+40 {
		t.Fatal("wide lens extended range or failed to widen its real cross", profile.Range, len(targets))
	}
}

func TestCoreUndeployedHeldItemStatsRemainVisibleE2E(t *testing.T) {
	state := coreNew(t, 1)
	state.Team[0].Item = "protein"
	state.Inventory = append(state.Inventory, OwnedItem{ID: "protein", EquippedBy: state.Team[0].ID})
	view := ProjectTransient(state)
	if view.Team[0].CombatStats == nil || view.Team[0].CombatStats.Power != state.Team[0].Stats.Power+15 {
		t.Fatal("an undeployed held item disappeared from the authoritative detail stats")
	}
}
