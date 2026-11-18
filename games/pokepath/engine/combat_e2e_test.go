package engine

import (
	"bytes"
	"math"
	"testing"
)

func TestCoreLaunchContextFreezesBeforeAttackCountersE2E(t *testing.T) {
	state := coreNew(t, 1)
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: coreTile(t, state)})
	coreApply(t, state, 0, Action{Type: "wave"})
	for index := 0; index < 3000 && len(state.Projectiles) == 0; index++ {
		if err := Advance(state, 20); err != nil {
			t.Fatal(err)
		}
	}
	if len(state.Projectiles) == 0 {
		t.Fatal("the deployed unit never fired a real projectile")
	}
	projectile := state.Projectiles[0]
	if projectile.Context.AttackCount != state.Towers[0].AttackCount-1 {
		t.Fatal("launch context was sampled after mutating attack counters", projectile.Context.AttackCount, state.Towers[0].AttackCount)
	}
	power := projectile.Context.Power
	coreApply(t, state, 0, Action{Type: "upgrade", PokemonID: state.Team[0].ID})
	if state.Projectiles[0].Context.Power != power || state.Team[0].Stats.Power <= power {
		t.Fatal("upgrading a unit changed its in-flight projectile")
	}
}

func TestCoreHundredthWaveHasAnExplicitRouteCompletionE2E(t *testing.T) {
	state := coreNew(t, 1)
	state.Wave = 100
	state.WaveActive = true
	state.Phase = "battle"
	state.AutoWave = true
	if err := Advance(state, 100); err != nil {
		t.Fatal(err)
	}
	if state.Wave != 100 || state.WaveActive || state.AutoWave || state.Phase != "route_complete" || state.Records[0] != 100 {
		t.Fatal("the completed route silently entered an endless next wave", state.Wave, state.Phase, state.WaveActive, state.AutoWave)
	}
}

func TestCoreAllSpeedsAndLargeSeedRemainChunkEquivalentE2E(t *testing.T) {
	for _, speed := range []float64{0.8, 1, 1.2, 1.7, 2.5} {
		state, err := New(Options{Starter: "charmander", Speed: speed}, 2, math.MaxUint64-17, 1730000000000)
		if err != nil {
			t.Fatal(err)
		}
		coreApply(t, state, 1, Action{Type: "wave"})
		other, _ := Clone(state)
		for index := 0; index < 10; index++ {
			if err = Advance(state, 100); err != nil {
				t.Fatal(err)
			}
			for _, duration := range []int64{17, 23, 60} {
				if err = Advance(other, duration); err != nil {
					t.Fatal(err)
				}
			}
			left, _ := Marshal(state)
			right, _ := Marshal(other)
			if !bytes.Equal(left, right) {
				t.Fatal("fixed-step chunk equivalence failed", speed, index)
			}
		}
	}
}

func TestCoreStraightBeamDoesNotChainIntoOffLineTargetsE2E(t *testing.T) {
	state := coreNew(t, 1)
	pokemon, err := newPokemon(state, "bulbasaur", 50)
	if err != nil {
		t.Fatal(err)
	}
	state.Team = []Pokemon{pokemon}
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: pokemon.ID, TileID: coreTile(t, state)})
	state.WaveActive = true
	state.Phase = "battle"
	tower := state.Towers[0]
	var ids []int
	for _, point := range []Point{{X: tower.X + 60, Y: tower.Y}, {X: tower.X + 90, Y: tower.Y}, {X: tower.X + 80, Y: tower.Y + 60}} {
		id, err := SpawnEnemyAt(state, "rattata", point, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		enemy := enemyFor(state, id)
		enemy.Entered = true
		enemy.BaseSpeed = 0
		enemy.HP, enemy.MaxHP = 10000, 10000
		enemy.Distance = float64(100 - len(ids)*10)
		ids = append(ids, id)
	}
	if err := Advance(state, 217); err != nil {
		t.Fatal(err)
	}
	first, second, offLine := enemyFor(state, ids[0]), enemyFor(state, ids[1]), enemyFor(state, ids[2])
	if first.HP >= first.MaxHP || second.HP >= second.MaxHP || offLine.HP != offLine.MaxHP {
		t.Fatal("straight beam bent into an off-line target", first.HP, second.HP, offLine.HP)
	}
}

func TestCoreDefeatIsChunkIndependentAtTheActualStoppingTimeE2E(t *testing.T) {
	state := coreNew(t, 1)
	state.Health = 1
	coreApply(t, state, 0, Action{Type: "wave"})
	other, _ := Clone(state)
	if err := Advance(state, 180000); err != nil {
		t.Fatal(err)
	}
	for elapsed := 0; elapsed < 180000 && other.Phase != "defeat"; elapsed += 1000 {
		if err := Advance(other, 1000); err != nil {
			t.Fatal(err)
		}
	}
	left, _ := Marshal(state)
	right, _ := Marshal(other)
	if state.Phase != "defeat" || !bytes.Equal(left, right) {
		t.Fatal("defeat consumed unused caller time or changed the fixed-step state", state.Clock, other.Clock)
	}
}

func TestCoreArmorDamageAndCriticalRoundingAreRecordedE2E(t *testing.T) {
	state := coreNew(t, 1)
	id, err := SpawnEnemyAt(state, "rattata", Point{X: 100, Y: 100}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	enemy := enemyFor(state, id)
	enemy.Armor, enemy.MaxArmor = 100, 100
	context := AbilityContext{Power: 15, Range: 100, AttackMS: 1000, CriticalPct: 100, CriticalDamage: 1.5, Level: 1, PokemonID: state.Team[0].ID}
	profile, err := BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	projectile := Projectile{PokemonID: state.Team[0].ID, Context: context, Profile: profile, DamageScale: 1}
	if err := applyProjectileHit(state, &projectile, enemy, 1); err != nil {
		t.Fatal(err)
	}
	if enemy.Armor != 77 || state.TotalDamage != 23 || state.Team[0].Damage != 23 {
		t.Fatal("critical ceil or armor damage statistics diverged", enemy.Armor, state.TotalDamage, state.Team[0].Damage)
	}
}

func TestCoreStaticConsumesTheFailedAttackCooldownE2E(t *testing.T) {
	state := coreNew(t, 1)
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: coreTile(t, state)})
	tower := &state.Towers[0]
	id, err := SpawnEnemyAt(state, "zapdos", Point{X: tower.X + 50, Y: tower.Y}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	enemyFor(state, id).Entered = true
	state.Random = 1
	if err := advanceTowers(state); err != nil {
		t.Fatal(err)
	}
	if len(state.Projectiles) != 0 || state.Towers[0].CooldownMS <= 0 || enemyFor(state, id).PassiveCounters["paralyses"] != 1 {
		t.Fatal("a static failure retried without paying its cooldown", state.Towers[0].CooldownMS, len(state.Projectiles))
	}
}

func TestCoreWaveStartRetainsDeployedAttackStateE2E(t *testing.T) {
	state := coreNew(t, 1)
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: coreTile(t, state)})
	state.Towers[0].CooldownMS = 721
	state.Towers[0].AttackCount = 9
	state.Towers[0].SameTargetHits = 4
	coreApply(t, state, 0, Action{Type: "wave"})
	if state.Towers[0].CooldownMS != 721 || state.Towers[0].AttackCount != 9 || state.Towers[0].SameTargetHits != 4 {
		t.Fatal("starting a wave reset a continuously deployed tower")
	}
}

func TestCoreChallengeLevelIsFrozenInAttackContextE2E(t *testing.T) {
	state := coreNew(t, 1)
	state.Progression.Challenge.Active = true
	state.Progression.Challenge.LevelCap = 50
	if err := refreshPokemon(state, &state.Team[0]); err != nil {
		t.Fatal(err)
	}
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: coreTile(t, state)})
	context := contextFor(state, &state.Towers[0], &state.Team[0], nil, 0.5)
	if context.Level != 50 || state.Team[0].Level != 1 {
		t.Fatal("challenge capped stats and ability source levels diverged", context.Level, state.Team[0].Level)
	}
}

func TestCoreCondensedBlizzardActuallyUsesCircularTargetingE2E(t *testing.T) {
	state := coreMotionOwner(t, "cryogonal")
	state.Team[0].Item = "condensedBlizzard"
	profile, err := profileFor(state, &state.Towers[0], &state.Team[0], nil, .5)
	if err != nil {
		t.Fatal(err)
	}
	tower := &state.Towers[0]
	id, err := SpawnEnemyAt(state, "rattata", Point{X: tower.X + profile.Range*.7, Y: tower.Y + profile.Range*.2}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	enemyFor(state, id).Entered = true
	if len(selectTargets(state, tower, &state.Team[0], profile)) != 1 {
		t.Fatal("condensed blizzard displayed a circle but still used the original X targeting")
	}
}

func TestCoreSplashKeepsThePrimaryCriticalRollE2E(t *testing.T) {
	state := coreNew(t, 1)
	ids := []int{}
	for _, point := range []Point{{X: 100, Y: 100}, {X: 120, Y: 100}} {
		id, err := SpawnEnemyAt(state, "rattata", point, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	context := AbilityContext{AbilityID: "splash", Power: 20, Range: 100, AttackMS: 1000, Level: 1, CriticalPct: 50, Random: .5, PokemonID: state.Team[0].ID}
	profile, err := BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	state.Projectiles = []Projectile{{ID: allocateID(state), PokemonID: state.Team[0].ID, TargetID: ids[0], X: 100, Y: 100, Speed: 400, Context: context, Profile: profile, SourceProfile: profile, DamageScale: 1}}
	state.Random = 1
	if err := advanceProjectiles(state); err != nil {
		t.Fatal(err)
	}
	if enemyFor(state, ids[0]).HP != 40 || enemyFor(state, ids[1]).HP != 55 {
		t.Fatal("one splash attack rerolled critical separately for its neighbors", enemyFor(state, ids[0]).HP, enemyFor(state, ids[1]).HP)
	}
}

func TestCoreRegeneratorResolvesBeforePostHitConditionsAndNextProjectileE2E(t *testing.T) {
	for _, shots := range []int{1, 2} {
		state := coreNew(t, 1)
		state.Speed, state.WaveActive, state.Phase = 1, true, "battle"
		id, err := SpawnEnemyAt(state, "hooh", Point{X: 100, Y: 100}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		enemy := enemyFor(state, id)
		enemy.HP, enemy.Armor, enemy.BaseSpeed, enemy.Entered = 1, 0, 0, true
		context := AbilityContext{AbilityID: "nightmare", PokemonID: state.Team[0].ID, Power: 60000, Range: 100, AttackMS: 1000, Level: 1, Random: .5}
		profile, err := BuildAttackProfile(context)
		if err != nil {
			t.Fatal(err)
		}
		for shot := 0; shot < shots; shot++ {
			state.Projectiles = append(state.Projectiles, Projectile{ID: allocateID(state), PokemonID: state.Team[0].ID, TargetID: id, X: 100, Y: 100, Speed: 400, Context: context, Profile: profile, SourceProfile: profile, DamageScale: 1})
		}
		if err := Advance(state, 20); err != nil {
			t.Fatal(err)
		}
		enemy = enemyFor(state, id)
		if shots == 1 && (enemy == nil || enemy.HP != 1 || !HasCondition(&enemy.EnemyEffectState, "nightmare")) {
			t.Fatal("the fatal projectile lost its post-hit nightmare while regeneration was deferred")
		}
		if shots == 2 && (enemy == nil || !enemy.Dead || state.Gold <= 50) {
			t.Fatal("the same-tick second projectile could not interrupt regeneration")
		}
	}
}

func TestCoreFatalCurseStillConnectsTheExistingCurseNetworkE2E(t *testing.T) {
	state := coreNew(t, 1)
	ids := []int{}
	for _, point := range []Point{{X: 100, Y: 100}, {X: 500, Y: 100}} {
		id, err := SpawnEnemyAt(state, "rattata", point, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	enemyFor(state, ids[0]).HP = 1
	ApplyConditions(&enemyFor(state, ids[1]).EnemyEffectState, AttackProfile{Conditions: []ConditionSpec{curseCondition()}}, 0)
	context := AbilityContext{AbilityID: "curse", PokemonID: state.Team[0].ID, Power: 20, Range: 100, AttackMS: 1000, Level: 1, Random: .5}
	profile, err := BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	projectile := Projectile{PokemonID: state.Team[0].ID, TargetID: ids[0], Context: context, Profile: profile, SourceProfile: profile, DamageScale: 1}
	if err := applyProjectileHit(state, &projectile, enemyFor(state, ids[0]), 1); err != nil {
		t.Fatal(err)
	}
	if !enemyFor(state, ids[0]).Dead || enemyFor(state, ids[1]).HP != 50 {
		t.Fatal("a fatal curse failed to apply its pre-hit curse and reach the existing network", enemyFor(state, ids[1]).HP)
	}
}

func TestCoreMaximumHealthDamageUsesItsRecordedRoundingStageE2E(t *testing.T) {
	for _, scenario := range []struct {
		ability  string
		critical float64
		burned   bool
		want     float64
	}{
		{"fieryDance", 100, true, 8},
		{"spiralDrain", 0, false, 4},
		{"spiralDrain", 100, false, 5},
	} {
		state := coreNew(t, 1)
		id, err := SpawnEnemyAt(state, "rattata", Point{X: 100, Y: 100}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		enemy := enemyFor(state, id)
		enemy.HP, enemy.MaxHP = 123, 123
		if scenario.burned {
			ApplyConditions(&enemy.EnemyEffectState, AttackProfile{Conditions: []ConditionSpec{status("burn", 1, 6000, .005, 1)}}, 0)
		}
		context := AbilityContext{AbilityID: scenario.ability, PokemonID: state.Team[0].ID, Power: 1, Range: 100, AttackMS: 1000, Level: 1, Random: .5, CriticalPct: scenario.critical, TargetBurned: scenario.burned, TargetBurnDurationMS: 6000, TargetHP: 123, TargetMaxHP: 123}
		profile, err := BuildAttackProfile(context)
		if err != nil {
			t.Fatal(err)
		}
		projectile := Projectile{PokemonID: state.Team[0].ID, TargetID: id, Context: context, Profile: profile, SourceProfile: profile, DamageScale: 1}
		if err := applyProjectileHit(state, &projectile, enemy, 1); err != nil {
			t.Fatal(err)
		}
		if 123-enemy.HP != scenario.want {
			t.Fatal("maximum-health contribution rounded at the wrong stage", scenario.ability, 123-enemy.HP, scenario.want)
		}
	}
}
