package pokepath_test

import (
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func linkedProfile(t *testing.T, state *engine.State, ability string) (engine.Projectile, engine.AttackProfile) {
	t.Helper()
	context := abilityContext(ability)
	context.PokemonID, context.TargetCursed = state.Team[0].ID, true
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	return engine.Projectile{ID: 100, PokemonID: state.Team[0].ID, TowerID: 10, TargetID: 1, Context: context, Profile: profile, SourceProfile: profile, DamageScale: 1}, profile
}

func TestAbilitiesProjectileCurseLinksEachExistingVictimOnceWithAttribution(t *testing.T) {
	state := passiveWorld(t)
	for index := 0; index < 4; index++ {
		enemy := passiveEnemy("")
		enemy.ID, enemy.Armor = index+1, 50
		if index < 3 {
			engine.ApplyConditions(&enemy.EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "curse", Chance: 1, DurationMS: -1, Stacks: 1}}}, 0)
		}
		state.Enemies = append(state.Enemies, enemy)
	}
	projectile, profile := linkedProfile(t, state, "curse")
	before := state.TotalDamage
	if err := engine.AfterCoreProjectileHit(state, &projectile, &state.Enemies[0], profile, 100, 100, true); err != nil {
		t.Fatal(err)
	}
	if state.Enemies[1].HP != 9950 || state.Enemies[1].Armor != 0 || state.Enemies[2].HP != 9950 || state.Enemies[3].HP != 10000 || state.TotalDamage != before+200 || state.Team[0].Damage != 200 {
		t.Fatal("curse links did not damage exactly the other cursed victims through armor")
	}
	if state.Enemies[1].LastHitBy != state.Team[0].ID || state.Enemies[1].LastDamageKind != "direct" {
		t.Fatal("curse link lost attribution or incorrectly propagated the critical flag")
	}
	projectile.Kind = "curseLink"
	if err := engine.AfterCoreProjectileHit(state, &projectile, &state.Enemies[1], profile, 100, 100, true); err != nil || state.TotalDamage != before+200 {
		t.Fatal("linked hits recursively re-entered the curse network")
	}
}

func TestAbilitiesProjectileWillOWispBurnsExistingCurseNetworkWithoutApplyingCurse(t *testing.T) {
	state := passiveWorld(t)
	projectile, profile := linkedProfile(t, state, "willOWisp")
	for _, condition := range profile.Conditions {
		if condition.Kind == "curse" {
			t.Fatal("will-o-wisp invented a curse application")
		}
	}
	for index := 0; index < 2; index++ {
		enemy := passiveEnemy("")
		enemy.ID = index + 1
		engine.ApplyConditions(&enemy.EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "curse", Chance: 1, DurationMS: -1, Stacks: 1}}}, 0)
		state.Enemies = append(state.Enemies, enemy)
	}
	if err := engine.AfterCoreProjectileHit(state, &projectile, &state.Enemies[0], profile, 100, 100, false); err != nil || !engine.HasCondition(&state.Enemies[1].EnemyEffectState, "burn") {
		t.Fatal("will-o-wisp did not burn its already-cursed linked victim")
	}
}

func TestAbilitiesProjectileCriticalFollowupsUseTheirActualCooldownAndBounce(t *testing.T) {
	state := passiveWorld(t)
	state.Towers = []engine.Tower{{ID: 10, PokemonID: state.Team[0].ID, CooldownMS: 1000}}
	enemy := passiveEnemy("")
	projectile, profile := linkedProfile(t, state, "flowerTrick")
	if err := engine.AfterCoreProjectileHit(state, &projectile, &enemy, profile, 0, 0, true); err != nil || state.Towers[0].CooldownMS > 0 {
		t.Fatal("flower-trick critical failed to make the tower immediately ready")
	}
	projectile, profile = linkedProfile(t, state, "ceaselessEdge")
	if err := engine.AfterCoreProjectileHit(state, &projectile, &enemy, profile, 100, 100, true); err != nil || state.Towers[0].CooldownMS != 100 {
		t.Fatal("ceaseless-edge critical did not set its 100ms followup")
	}
	projectile, profile = linkedProfile(t, state, "armaldo")
	projectile.Bounces, projectile.RolledCritical = 1, true
	if err := engine.AfterCoreProjectileHit(state, &projectile, &enemy, profile, 100, 100, false); err != nil || projectile.Bounces != 2 {
		t.Fatal("critical ricochet lost a bounce when enemy aura later suppressed the critical flag")
	}
	secondary := enemy
	secondary.ID = 2
	if err := engine.AfterCoreProjectileHit(state, &projectile, &secondary, profile, 50, 50, true); err != nil || projectile.Bounces != 2 {
		t.Fatal("secondary splash damage invented another primary ricochet")
	}
	projectile, profile = linkedProfile(t, state, "flowerTrick")
	state.Towers[0].CooldownMS = 800
	if err := engine.AfterCoreProjectileHit(state, &projectile, &secondary, profile, 50, 50, true); err != nil || state.Towers[0].CooldownMS != 800 {
		t.Fatal("secondary splash damage retriggered the primary critical followup")
	}
}

func TestAbilitiesProjectileSynchronyCopiesOnlyPrimaryConditionsAndOneNightmare(t *testing.T) {
	state := passiveWorld(t)
	projectile, _ := linkedProfile(t, state, "synchronySplash")
	projectile.Context.ItemID = "nightmareCloth"
	profile, err := engine.BuildAttackProfile(projectile.Context)
	if err != nil {
		t.Fatal(err)
	}
	for index, x := range []float64{100, 150, 205} {
		enemy := passiveEnemy("")
		enemy.ID, enemy.X = index+1, x
		state.Enemies = append(state.Enemies, enemy)
	}
	engine.ApplyConditions(&state.Enemies[0].EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: .005, Stacks: 1}, {Kind: "poison", Chance: 1, DurationMS: -1, Magnitude: .001, Stacks: 4}, {Kind: "nightmare", Chance: 1, DurationMS: -1, Magnitude: 20, Stacks: 6}}}, 0)
	if err := engine.AfterCoreProjectileHit(state, &projectile, &state.Enemies[0], profile, 100, 100, false); err != nil {
		t.Fatal(err)
	}
	if err := engine.AfterCoreProjectileSplash(state, &projectile, &state.Enemies[0], profile); err != nil {
		t.Fatal(err)
	}
	if !engine.HasCondition(&state.Enemies[1].EnemyEffectState, "burn") || engine.ConditionStacks(&state.Enemies[1].EnemyEffectState, "poison") != 1 || engine.ConditionStacks(&state.Enemies[1].EnemyEffectState, "nightmare") != 1 || len(state.Enemies[2].Conditions) != 0 {
		t.Fatal("synchrony did not copy status types as individual applications within primary radius")
	}
	if err := engine.AfterCoreProjectileSplash(state, &projectile, &state.Enemies[1], profile); err != nil || len(state.Enemies[2].Conditions) != 0 {
		t.Fatal("splash victim became a recursive synchronization source")
	}
}
