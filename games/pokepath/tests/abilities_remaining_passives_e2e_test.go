package pokepath_test

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func TestAbilitiesPassiveTimedSpeedAndChargesMatchHitSequence(t *testing.T) {
	state := passiveWorld(t)
	enemy := passiveEnemy("shellSmash")
	for index := 0; index < 2; index++ {
		engine.ApplyDamage(&enemy.EnemyEffectState, 10, "critical", 0)
		if _, err := engine.EnemyPassiveEvents(state, &enemy, "hit", 0); err != nil {
			t.Fatal(err)
		}
	}
	if enemy.BaseSpeed != 120 {
		t.Fatal("critical shell-smash hits did not stack speed")
	}
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "tick", 2000); err != nil {
		t.Fatal(err)
	}
	if enemy.BaseSpeed != 90 {
		t.Fatal("shared shell-smash timer did not preserve original residual stacking")
	}
	enemy = passiveEnemy("steamEngine")
	enemy.LastHitTerrain, enemy.LastDamageKind = 3, "direct"
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "hit", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "tick", 600); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "hit", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "tick", 400); err != nil {
		t.Fatal(err)
	}
	if enemy.BaseSpeed != 60 {
		t.Fatal("steam-engine refreshed an already active boost")
	}
	enemy = passiveEnemy("dragonsMaw")
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "tick", 2000); err != nil {
		t.Fatal(err)
	}
	engine.ApplyDamage(&enemy.EnemyEffectState, 100, "direct", 0)
	if enemy.LastArmorDamage != 130 {
		t.Fatal("dragon's-maw clock charges did not amplify received damage")
	}
}

func TestAbilitiesPassiveEmptyGrassHealingAndTailwindAreWorldDerived(t *testing.T) {
	state := passiveWorld(t)
	state.Progression.CustomRoute = &engine.RouteSpec{Width: 720, Height: 624, Placements: []engine.Placement{{ID: 1, Terrain: 2, X: 100, Y: 100}, {ID: 2, Terrain: 2, X: 120, Y: 100}, {ID: 3, Terrain: 3, X: 130, Y: 100}, {ID: 4, Terrain: 2, X: 500, Y: 100}}}
	state.Towers = []engine.Tower{{ID: 2, TileID: 2}}
	enemy := passiveEnemy("ingrain")
	enemy.X, enemy.Y = 100, 100
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "tick", 250); err != nil {
		t.Fatal(err)
	}
	if enemy.Regeneration != 2000 {
		t.Fatal("ingrain counted occupied, distant or non-grass tiles")
	}
	state.Enemies = []engine.Enemy{passiveEnemy("tailWind")}
	state.Enemies[0].ID, state.Enemies[0].X, state.Enemies[0].Y = 2, 140, 100
	enemy.PassiveID = "heavyMetal"
	if _, err := engine.EnemyPassiveEvents(state, &enemy, "tick", 20); err != nil {
		t.Fatal(err)
	}
	if engine.EffectiveEnemySpeed(&enemy.EnemyEffectState) != 112.5 {
		t.Fatal("tailwind and heavy-metal received-speed amplification were not combined")
	}
}

func TestAbilitiesPassiveDamageRoutingSuppressesOnlyCriticalFlag(t *testing.T) {
	state := passiveWorld(t)
	victim := passiveEnemy("")
	victim.X, victim.Y, victim.Invisible = 100, 100, true
	absorber := passiveEnemy("stanceChange")
	absorber.ID, absorber.X, absorber.Y = 2, 600, 100
	aura := passiveEnemy("criticalAura")
	aura.ID, aura.X, aura.Y = 3, 500, 200
	state.Enemies = []engine.Enemy{victim, absorber, aura}
	if target := engine.EnemyHitTarget(state, &state.Enemies[0]); target == nil || target.ID != 2 || state.Enemies[0].Invisible {
		t.Fatal("global stance-change absorber did not redirect and reveal the original target")
	}
	if engine.EnemyCriticalFlag(state, true) {
		t.Fatal("entered critical aura did not clear critical classification")
	}
	state.Enemies[2].Entered = false
	if !engine.EnemyCriticalFlag(state, true) {
		t.Fatal("off-canvas critical aura affected the battlefield")
	}
	state.Enemies[1].Dead = true
	if engine.EnemyHitTarget(state, &state.Enemies[0]).ID != 1 {
		t.Fatal("dying absorber continued taking other enemy hits")
	}
}

func TestAbilitiesPassiveHerbogenesisEvadeAndChessDamage(t *testing.T) {
	state := passiveWorld(t)
	enemy := passiveEnemy("herbogenesis")
	enemy.Regeneration, enemy.LastHitAbilityID = 2500, "heal"
	if _, ok, err := engine.PrepareEnemyHit(state, &enemy, 100, "direct"); err != nil || !ok || enemy.Regeneration != 1500 || !enemy.PassiveFlags["regenerationReduced"] {
		t.Fatal("healing attacker did not reduce herbogenesis regeneration")
	}
	enemy.LastHitAbilityID = "burn"
	enemy.LastHitItemID = "shellBell"
	if _, _, err := engine.PrepareEnemyHit(state, &enemy, 100, "direct"); err != nil || enemy.Regeneration != 500 {
		t.Fatal("healing held item did not reduce herbogenesis regeneration")
	}
	enemy = passiveEnemy("sweetScent")
	state.Random = 1
	if _, ok, err := engine.PrepareEnemyHit(state, &enemy, 100, "direct"); err != nil || ok || !enemy.Invisible || !enemy.LastHitEvaded {
		t.Fatal("sweet-scent deterministic evasion was not applied")
	}
	enemy = passiveEnemy("asOne")
	engine.EnemyPassiveEvents(state, &enemy, "tick", 20)
	engine.ApplyDamage(&enemy.EnemyEffectState, 100, "direct", 0)
	if enemy.Armor != 1000 {
		t.Fatal("as-one accepted damage while no chess pieces were active")
	}
	state.World.ChessActive = []string{"king", "queen", "rook"}
	engine.EnemyPassiveEvents(state, &enemy, "tick", 20)
	engine.ApplyDamage(&enemy.EnemyEffectState, 100, "direct", 0)
	if enemy.Armor != 400 {
		t.Fatal("as-one damage did not depend on active chess pieces")
	}
}

func TestAbilitiesPassiveSharedDamageLocksAndStrippingEmitAttributedEvents(t *testing.T) {
	state := passiveWorld(t)
	enemy := passiveEnemy("eonUnion")
	ally := passiveEnemy("eonUnion")
	ally.ID = 2
	state.Enemies = []engine.Enemy{enemy, ally}
	enemy.LastDamageSourceID, enemy.LastHPDamage, enemy.LastArmorDamage, enemy.LastDamageKind = 71, 10, 80, "critical"
	events, err := engine.EnemyAfterHit(state, &enemy)
	if err != nil || len(events) != 1 || events[0].Kind != "sharedDamage" || events[0].EnemyID != 2 || events[0].Amount != 90 || events[0].SourcePokemonID != 71 || events[0].Key != "critical" {
		t.Fatalf("same-kind union damage was not attributed: %+v %v", events, err)
	}
	for _, passive := range []string{"anchorShot", "pressure", "klutz"} {
		enemy = passiveEnemy(passive)
		enemy.Armor, enemy.LastDamageSourceID = 0, 71
		events, err = engine.EnemyAfterHit(state, &enemy)
		want := passive
		if passive == "klutz" {
			want = "stripItem"
		}
		if err != nil || len(events) != 1 || events[0].Kind != want || events[0].SourcePokemonID != 71 {
			t.Fatalf("%s did not emit a real attributed event: %+v %v", passive, events, err)
		}
		if passive != "klutz" {
			events, _ = engine.EnemyAfterHit(state, &enemy)
			if len(events) != 0 {
				t.Fatal("one-time armor break triggered twice")
			}
		}
	}
}

func TestAbilitiesPassiveStallCopiesPathAndTransistorCountsNewWaypoints(t *testing.T) {
	state := passiveWorld(t)
	reviver := passiveEnemy("regenerator")
	reviver.HP = 0
	untouched := passiveEnemy("stall")
	untouched.ID, untouched.X = 2, 10
	state.Enemies = []engine.Enemy{reviver, untouched}
	engine.EnemyPassiveEvents(state, &state.Enemies[0], "death", 0)
	if state.Enemies[1].PassiveFlags["stallUsed"] || state.Enemies[1].X != 10 {
		t.Fatal("first interruptible regenerator phase consumed a stall teleport before final death")
	}
	dead := passiveEnemy("")
	dead.ID, dead.HP, dead.X, dead.Y, dead.Waypoint, dead.Distance = 1, 0, 300, 240, 3, 560
	stall := passiveEnemy("stall")
	stall.ID, stall.HP = 2, 4321
	state.Enemies = []engine.Enemy{dead, stall}
	engine.EnemyPassiveEvents(state, &state.Enemies[0], "death", 0)
	if state.Enemies[1].X != 300 || state.Enemies[1].Waypoint != 3 || state.Enemies[1].Distance != 560 || state.Enemies[1].HP != 4321 {
		t.Fatal("stall did not teleport one eligible Sableye without healing")
	}
	enemy := passiveEnemy("transistor")
	enemy.Waypoint, enemy.CanStun = 2, false
	engine.EnemyPassiveEvents(state, &enemy, "tick", 20)
	if !engine.HasCondition(&enemy.EnemyEffectState, "stun") || enemy.StunApplications != 1 {
		t.Fatal("transistor did not apply its own resistance-independent waypoint stun")
	}
	data, _ := json.Marshal(enemy)
	json.Unmarshal(data, &enemy)
	engine.AdvanceConditions(&enemy.EnemyEffectState, 2000)
	engine.EnemyPassiveEvents(state, &enemy, "tick", 20)
	if engine.HasCondition(&enemy.EnemyEffectState, "stun") {
		t.Fatal("same waypoint repeatedly applied transistor stun")
	}
}

func TestAbilitiesPassiveRemoraidAndSunnyDayProduceWorldEvents(t *testing.T) {
	state := passiveWorld(t)
	enemy := passiveEnemy("remoaid")
	enemy.LastDamageKind = "direct"
	state.Random = 1
	events, err := engine.EnemyPassiveEvents(state, &enemy, "hit", 0)
	if err != nil || len(events) != 1 || events[0].Kind != "spawn" || events[0].Key != "remoraid" || !events[0].AtRouteStart {
		t.Fatal("remoraid hit did not create a route-entry child")
	}
	enemy = passiveEnemy("sunnyDay")
	events, err = engine.EnemyPassiveEvents(state, &enemy, "spawn", 0)
	if err != nil || len(events) != 1 || events[0].Kind != "weather" || events[0].Key != "harshSunlight" {
		t.Fatal("sunny-day spawn did not impose sunlight")
	}
	state.Random = 1
	spinda := engine.Pokemon{Species: "spinda"}
	relic := passiveEnemy("relicSong")
	if !engine.EnemyAttackAllowed(state, &spinda, &relic, false) {
		t.Fatal("ordinary Spinda attack lost its relic-song exemption")
	}
	state.Random = 1
	if engine.EnemyAttackAllowed(state, &spinda, &relic, true) {
		t.Fatal("area path incorrectly kept ordinary relic-song exemption")
	}
}

func TestAbilitiesPassiveWeatherAndHealingPulseClocksSurviveRestore(t *testing.T) {
	enemy := passiveEnemy("")
	enemy.Armor, enemy.HP, enemy.MaxHP, enemy.Regeneration = 0, 5000, 10001, 100
	enemy.WeatherID, enemy.WorldSandMultiplier = "sandstorm", 1.5
	engine.AdvanceWeatherEffects(&enemy.EnemyEffectState, 999)
	engine.AdvanceConditions(&enemy.EnemyEffectState, 999)
	if enemy.HP != 5000 || enemy.HealingEvents != 0 {
		t.Fatal("weather or regeneration applied before its one-second pulse")
	}
	data, _ := json.Marshal(enemy)
	json.Unmarshal(data, &enemy)
	engine.AdvanceWeatherEffects(&enemy.EnemyEffectState, 1)
	engine.AdvanceConditions(&enemy.EnemyEffectState, 1)
	if enemy.HP != 5054 || enemy.HealingEvents != 1 {
		t.Fatalf("restored pulse did not ceil sand damage then heal exactly once: %v", enemy.HP)
	}
	enemy.HP, enemy.MaxHP = 10001, 10001
	engine.AdvanceConditions(&enemy.EnemyEffectState, 1000)
	if enemy.HealingEvents != 2 {
		t.Fatal("full-health regeneration did not emit its heal event")
	}
	synthesis := passiveEnemy("synthesis")
	synthesis.Armor, synthesis.HP = 0, 4000
	engine.AdvanceConditions(&synthesis.EnemyEffectState, 1000)
	if synthesis.HP != 4100 {
		t.Fatal("synthesis did not heal 2.5 percent of current health")
	}
}

func TestAbilitiesPassiveCriticalLearningStillUsesArmorAndLunarKeepsNightmare(t *testing.T) {
	enemy := passiveEnemy("criticalLearning")
	engine.ApplyDamage(&enemy.EnemyEffectState, 9000, "critical", 0)
	if enemy.HP != 10000 || enemy.Armor != 999 {
		t.Fatal("critical-learning one-point damage bypassed armor")
	}
	enemy = passiveEnemy("lunarBlessing")
	enemy.Armor, enemy.HP, enemy.Regeneration = 0, 4000, 200
	engine.ApplyConditions(&enemy.EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: .005, Stacks: 1}, {Kind: "nightmare", Chance: 1, DurationMS: -1, Magnitude: 20, Stacks: 1}}}, 0)
	engine.ApplyDamage(&enemy.EnemyEffectState, 100, "direct", 0)
	if len(enemy.Conditions) != 1 || !engine.HasCondition(&enemy.EnemyEffectState, "nightmare") || enemy.Regeneration != 1000 {
		t.Fatal("lunar blessing did not retain nightmare and replace regeneration")
	}
}

func TestAbilitiesPassiveKingsShieldChecksPreHitHealthAndSpawnsOneOfFourAtEntry(t *testing.T) {
	state := passiveWorld(t)
	enemy := passiveEnemy("kingsShield")
	enemy.Armor, enemy.HP = 0, 8100
	engine.ApplyDamage(&enemy.EnemyEffectState, 300, "direct", 0)
	events, err := engine.EnemyPassiveEvents(state, &enemy, "hit", 0)
	if err != nil || len(events) != 0 {
		t.Fatal("king's shield used post-hit health instead of its pre-hit threshold")
	}
	engine.ApplyDamage(&enemy.EnemyEffectState, 10, "direct", 0)
	events, err = engine.EnemyPassiveEvents(state, &enemy, "hit", 0)
	if err != nil || len(events) != 1 || events[0].Amount != 1 || !events[0].AtRouteStart {
		t.Fatal("king's shield did not emit one route-entry guard at its first actual threshold")
	}
	enemy.HP = 1000
	for index := 0; index < 4; index++ {
		engine.ApplyDamage(&enemy.EnemyEffectState, 1, "direct", 0)
		events, err = engine.EnemyPassiveEvents(state, &enemy, "hit", 0)
		want := 1
		if index == 3 {
			want = 0
		}
		if err != nil || len(events) != want {
			t.Fatal("king's shield spawned more than one guard per hit or exceeded four guards")
		}
	}
}

func TestAbilitiesPassiveDamageCountersDistinguishPhysicalAndResidualHits(t *testing.T) {
	enemy := passiveEnemy("sheerForce")
	engine.ApplyDamage(&enemy.EnemyEffectState, 100, "burn", 0)
	if enemy.Armor != 900 {
		t.Fatal("sheer-force physical threshold incorrectly blocked residual damage")
	}
	enemy = passiveEnemy("ironDefense")
	for _, kind := range []string{"burn", "direct", "critical"} {
		engine.ApplyDamage(&enemy.EnemyEffectState, 100, kind, 0)
	}
	if enemy.Armor != 703 {
		t.Fatal("iron-defense all-source floor reduction did not count each incoming damage")
	}
	enemy = passiveEnemy("dauntlessShield")
	for index := 0; index < 151; index++ {
		engine.ApplyDamage(&enemy.EnemyEffectState, 100, "direct", 0)
	}
	if enemy.Armor != 998 {
		t.Fatal("dauntless-shield threshold is off by one physical hit")
	}
	enemy = passiveEnemy("intrepidSword")
	enemy.Armor, enemy.MaxArmor = 100000, 100000
	engine.ApplyDamage(&enemy.EnemyEffectState, 100, "direct", 0)
	if enemy.Armor != 99802 {
		t.Fatal("intrepid-sword failed to increment its initial negative charge before damage")
	}
	for index := 1; index < 101; index++ {
		engine.ApplyDamage(&enemy.EnemyEffectState, 100, "direct", 0)
	}
	if enemy.BaseSpeed != 72 || enemy.Armor != 90009 {
		t.Fatal("intrepid-sword cycled its initial phase instead of keeping transformed speed and normal damage")
	}
	state := passiveWorld(t)
	enemy = passiveEnemy("honeyGatherBoss")
	enemy.LastDamageKind = "burn"
	events, err := engine.EnemyPassiveEvents(state, &enemy, "hit", 0)
	if err != nil || len(events) != 0 {
		t.Fatal("honey-gather boss counted a residual tick as a physical attack")
	}
}

func TestAbilitiesPassiveHerbogenesisReadsLiveHealingItemOnResidualSource(t *testing.T) {
	state := passiveWorld(t)
	state.Team[0].Item = "shellBell"
	enemy := passiveEnemy("herbogenesis")
	enemy.Armor, enemy.HP, enemy.Regeneration = 0, 5000, 3000
	engine.ApplyConditions(&enemy.EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: .005, Stacks: 1, SourcePokemonID: state.Team[0].ID}}}, 0)
	if err := engine.RefreshConditionSources(state, &enemy); err != nil {
		t.Fatal(err)
	}
	engine.AdvanceConditions(&enemy.EnemyEffectState, 1000)
	if enemy.Regeneration != 2000 || enemy.HP != 6950 || !enemy.PassiveFlags["regenerationReduced"] {
		t.Fatal("residual healing-item source did not suppress herbogenesis before its regeneration pulse")
	}
	state.Team[0].Item = ""
	if err := engine.RefreshConditionSources(state, &enemy); err != nil {
		t.Fatal(err)
	}
	engine.AdvanceConditions(&enemy.EnemyEffectState, 1000)
	if enemy.Regeneration != 2000 || enemy.HP != 8900 {
		t.Fatal("residual source kept a healing item that was no longer held")
	}
}
