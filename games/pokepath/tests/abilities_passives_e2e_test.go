package pokepath_test

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func passiveWorld(t *testing.T) *engine.State {
	t.Helper()
	state, err := engine.New(engine.Options{Starter: "charmander"}, 1, 15123, 1794916800000)
	if err != nil {
		t.Fatal(err)
	}
	state.Speed = 1
	return state
}

func passiveEnemy(id string) engine.Enemy {
	return engine.Enemy{ID: 1, Key: id, Entered: true, EnemyEffectState: engine.EnemyEffectState{HP: 10000, MaxHP: 10000, Armor: 1000, MaxArmor: 1000, BaseSpeed: 60, CanBurn: true, CanSlow: true, CanStun: true, CanPoison: true, PassiveID: id}}
}

func TestAbilitiesPassiveCriticalImmunityCountsRealBlockedHits(t *testing.T) {
	enemy := passiveEnemy("iceBody")
	for index := 0; index < 50; index++ {
		engine.ApplyDamage(&enemy.EnemyEffectState, 3000, "critical", 0)
	}
	if enemy.HP != 10000 || enemy.Armor != 1000 || enemy.CriticalBlocks != 50 {
		t.Fatalf("critical immunity was not real: %+v", enemy)
	}
}

func TestAbilitiesPassiveRegeneratorHasAnInterruptibleRevivalWindow(t *testing.T) {
	state := passiveWorld(t)
	enemy := passiveEnemy("regenerator")
	enemy.HP = 0
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "death", 0); err != nil {
		t.Fatal(err)
	}
	if enemy.HP != 1 || enemy.Armor != 50000 || enemy.ReviveRemainingMS <= 0 || enemy.Revived {
		t.Fatalf("wrong regeneration phase: %+v", enemy)
	}
	if engine.EffectiveEnemySpeed(&enemy.EnemyEffectState) != 0 {
		t.Fatal("reviving enemy kept walking")
	}
	engine.ApplyDamage(&enemy.EnemyEffectState, 60000, "direct", 0)
	if enemy.HP != 0 {
		t.Fatal("reviving enemy was incorrectly invulnerable")
	}
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "death", 0); err != nil {
		t.Fatal(err)
	}
	if enemy.ReviveRemainingMS != 0 || enemy.Revived {
		t.Fatal("early kill did not interrupt revival")
	}
	fresh := passiveEnemy("regenerator")
	fresh.HP = 0
	if _, err := engine.EnemyPassiveEvents(state, &fresh, "death", 0); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(fresh)
	var restored engine.Enemy
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.EnemyPassiveEvents(state, &restored, "tick", 5000); err != nil {
		t.Fatal(err)
	}
	if restored.HP != restored.MaxHP || restored.Armor != 0 || restored.MaxArmor != 0 || !restored.Revived {
		t.Fatal("revival did not survive JSON restore")
	}
}

func TestAbilitiesPassiveSchoolingAndWeatherAreSingleTypedEvents(t *testing.T) {
	state := passiveWorld(t)
	enemy := passiveEnemy("schooling")
	enemy.HP = 0
	events, err := engine.EnemyPassiveEvents(state, &enemy, "death", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != "spawn" || events[0].Key != "wishiwashiSolo" || events[0].Amount != 100 {
		t.Fatalf("schooling produced no real children: %+v", events)
	}
	repeated, err := engine.EnemyPassiveEvents(state, &enemy, "death", 0)
	if err != nil || len(repeated) != 0 {
		t.Fatal("death children could spawn twice")
	}
	boss := passiveEnemy("drought")
	if _, err := engine.EnemyPassiveEvents(state, &boss, "spawn", 0); err != nil {
		t.Fatal(err)
	}
	boss.HP = 4000
	events, err = engine.EnemyPassiveEvents(state, &boss, "tick", 20)
	if err != nil {
		t.Fatal(err)
	}
	if boss.Armor != 76000 || len(events) != 1 || events[0].Kind != "weather" || events[0].Key != "extremelyHarshSunlight" {
		t.Fatalf("weather threshold did not happen: %+v", events)
	}
	engine.ApplyDamage(&boss.EnemyEffectState, 100000, "direct", 0)
	if boss.HP != 4000 {
		t.Fatal("weather transformation lacked temporary invulnerability")
	}
}

func TestAbilitiesPassiveShadowAndTimeTravelPreserveTimedState(t *testing.T) {
	state := passiveWorld(t)
	enemy := passiveEnemy("shadowShield")
	enemy.Invisible = true
	enemy.LastDamageKind = "direct"
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "hit", 0); err != nil {
		t.Fatal(err)
	}
	if enemy.Invisible {
		t.Fatal("hit did not break shadow invisibility")
	}
	enemy.Armor = 0
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "tick", 3999); err != nil {
		t.Fatal(err)
	}
	if enemy.Invisible {
		t.Fatal("shadow restored before 4s")
	}
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "tick", 1); err != nil {
		t.Fatal(err)
	}
	if !enemy.Invisible || enemy.Armor != 1000 {
		t.Fatal("shadow did not restore original shield")
	}
	traveler := passiveEnemy("timeTravel")
	traveler.HP = 1000
	events, err := engine.EnemyPassiveEvents(state, &traveler, "tick", 8000)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != "rewind" || events[0].Amount < 150 || events[0].Amount > 200 || traveler.HP != 8000 {
		t.Fatalf("time travel did not rewind/heal: %+v", events)
	}
}

func TestAbilitiesPassiveNearbyAttackRulesUseExplicitRangeAndFailure(t *testing.T) {
	context := engine.AbilityContext{AbilityID: "burn", Power: 100, Range: 200, AttackMS: 1000, Targets: 1, Hearts: 14, EffectsVolume: 8, TargetPassiveID: "static", Distance: 115, Random: 0.1}
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	if !profile.Disabled || profile.AttackFailure != "static" || !profile.FailureConsumesCooldown {
		t.Fatal("static did not actually interrupt the attack")
	}
	context.Distance = 116
	profile, err = engine.BuildAttackProfile(context)
	if err != nil || profile.Disabled {
		t.Fatal("static interrupted outside its range")
	}
	context.TargetPassiveID = "snowCloak"
	context.Distance = 160
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.AttackMS, 1500)
	context.TargetPassiveID = "flameBody"
	context.Distance = 119
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.DamageMultiplier, 0.5)
}

func TestAbilitiesPassiveJustifiedRecordsOnlyItsFourActualTriggerConditions(t *testing.T) {
	enemy := passiveEnemy("justified")
	engine.ApplyConditions(&enemy.EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "curse", Chance: 1, DurationMS: -1, Stacks: 1}}}, 0)
	if enemy.Shield != 0 {
		t.Fatal("curse incorrectly triggered justified")
	}
	engine.ApplyConditions(&enemy.EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: 0.005, Stacks: 1}}}, 0)
	if enemy.Armor != 6000 || enemy.MaxArmor != 6000 || !enemy.PassiveFlags["justifiedTriggered"] || enemy.PassiveCounters["justified"] != 1 {
		t.Fatal("actual justified trigger was not persisted")
	}
}

func TestAbilitiesReferenceNightmareTracksItsLastCasterAfterUpgrade(t *testing.T) {
	state := passiveWorld(t)
	owner := state.Team[0]
	enemy := passiveEnemy("")
	enemy.Armor = 0
	engine.ApplyConditions(&enemy.EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "nightmare", Chance: 1, DurationMS: -1, Magnitude: 0, Stacks: 1, SourcePokemonID: owner.ID}}}, 0)
	if err := engine.RefreshConditionSources(state, &enemy); err != nil {
		t.Fatal(err)
	}
	old := enemy.Conditions[0].Magnitude
	if err := engine.Apply(state, 0, engine.Action{Type: "upgrade", Epoch: state.Epoch, PokemonID: owner.ID}); err != nil {
		t.Fatal(err)
	}
	if err := engine.RefreshConditionSources(state, &enemy); err != nil {
		t.Fatal(err)
	}
	if enemy.Conditions[0].Magnitude <= old {
		t.Fatal("existing nightmare retained obsolete caster power")
	}
	closeNumber(t, enemy.Conditions[0].Magnitude, state.Team[0].Stats.Power*0.2)
}
