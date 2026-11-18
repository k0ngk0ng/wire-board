package pokepath_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func effectEnemy() engine.EnemyEffectState {
	return engine.EnemyEffectState{HP: 10000, MaxHP: 10000, Armor: 1000, MaxArmor: 1000, BaseSpeed: 40, CanStun: true, CanSlow: true, CanBurn: true, CanPoison: true}
}

func abilityContext(id string) engine.AbilityContext {
	return engine.AbilityContext{AbilityID: id, Level: 1, Terrain: 1, Hearts: 14, Power: 100, Range: 120, AttackMS: 1000, CriticalPct: 5, Targets: 10}
}

func closeNumber(t *testing.T, actual, expected float64) {
	t.Helper()
	if math.Abs(actual-expected) > 1e-7 {
		t.Fatalf("got %.12f, want %.12f", actual, expected)
	}
}

func TestAbilitiesIndependentBurnPoisonAndCheckpointChain(t *testing.T) {
	burn, err := engine.BuildAttackProfile(abilityContext("burn"))
	if err != nil {
		t.Fatal(err)
	}
	poison, err := engine.BuildAttackProfile(abilityContext("toxic"))
	if err != nil {
		t.Fatal(err)
	}
	first := effectEnemy()
	engine.ApplyConditions(&first, burn, 0)
	engine.ApplyConditions(&first, poison, 0)
	engine.AdvanceConditions(&first, 2500)
	closeNumber(t, first.HP, 10000)
	closeNumber(t, first.Armor, 860)
	raw, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	var restored engine.EnemyEffectState
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	for _, milliseconds := range []int64{1500, 6000, 2000} {
		engine.AdvanceConditions(&first, milliseconds)
		engine.AdvanceConditions(&restored, milliseconds)
	}
	closeNumber(t, first.HP, 10000)
	closeNumber(t, first.Armor, 260)
	left, _ := json.Marshal(first)
	right, _ := json.Marshal(restored)
	if string(left) != string(right) {
		t.Fatal("JSON checkpoint changed continued effects")
	}
}

func TestAbilitiesIndependentControlResistanceAndExpiry(t *testing.T) {
	plan, err := engine.BuildAttackProfile(abilityContext("moldBreaker"))
	if err != nil {
		t.Fatal(err)
	}
	enemy := effectEnemy()
	enemy.CanStun = false
	engine.ApplyConditions(&enemy, plan, 0.2)
	closeNumber(t, engine.EffectiveEnemySpeed(&enemy), 0)
	engine.AdvanceConditions(&enemy, 749)
	closeNumber(t, engine.EffectiveEnemySpeed(&enemy), 0)
	engine.AdvanceConditions(&enemy, 1)
	closeNumber(t, engine.EffectiveEnemySpeed(&enemy), 40)
	slow, err := engine.BuildAttackProfile(abilityContext("slow"))
	if err != nil {
		t.Fatal(err)
	}
	engine.ApplyConditions(&enemy, slow, 0)
	closeNumber(t, engine.EffectiveEnemySpeed(&enemy), 20)
	engine.AdvanceConditions(&enemy, 2000)
	closeNumber(t, engine.EffectiveEnemySpeed(&enemy), 40)
	enemy.CanSlow = false
	engine.ApplyConditions(&enemy, slow, 0)
	closeNumber(t, engine.EffectiveEnemySpeed(&enemy), 40)
}

func TestAbilitiesIndependentDamageAbsorptionAndPiercing(t *testing.T) {
	enemy := effectEnemy()
	closeNumber(t, engine.ApplyDamage(&enemy, 500, "direct", 0), 0)
	closeNumber(t, enemy.Armor, 500)
	closeNumber(t, engine.ApplyDamage(&enemy, 1000, "direct", 0.5), 500)
	closeNumber(t, enemy.Armor, 0)
	closeNumber(t, enemy.HP, 9500)
	closeNumber(t, engine.ApplyDamage(&enemy, 20000, "residual", 0), 9500)
	closeNumber(t, enemy.HP, 0)
	closeNumber(t, engine.ApplyDamage(&enemy, math.NaN(), "direct", 0), 0)
	closeNumber(t, engine.ApplyDamage(&enemy, 500, "direct", 0), 0)
}

func TestAbilitiesIndependentAttackGeometryAndWeatherComposition(t *testing.T) {
	cases := []struct {
		ability   string
		targets   int
		burst     int
		bounce    bool
		splash    bool
		invisible bool
	}{{"burnDoubleShot", 2, 1, false, false, false}, {"quadraShot", 4, 1, false, false, false}, {"bubbleBeam", 1, 8, false, false, false}, {"slowRicochet", 1, 1, true, false, false}, {"armorBreakSplash", 1, 1, false, true, false}, {"vigilantFrisk", 1, 1, false, false, true}}
	for _, value := range cases {
		profile, err := engine.BuildAttackProfile(abilityContext(value.ability))
		if err != nil {
			t.Fatal(err)
		}
		if profile.TargetCount != value.targets || profile.Burst != value.burst || (profile.Ricochet > 0) != value.bounce || (profile.SplashRadius > 0) != value.splash || profile.CanSeeInvisible != value.invisible {
			t.Fatalf("wrong executable geometry for %s: %+v", value.ability, profile)
		}
	}
	context := abilityContext("swimmer")
	context.Terrain = 3
	context.WeatherID = "rain"
	context.ItemID = "carbos"
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.AttackMS, 450)
	closeNumber(t, profile.DamageMultiplier, 1.2)
}

func TestAbilitiesIndependentExplicitUnsupportedAndImmutableInputs(t *testing.T) {
	context := abilityContext("burn")
	original := context
	if _, err := engine.BuildAttackProfile(context); err != nil {
		t.Fatal(err)
	}
	if context != original {
		t.Fatal("rule calculation mutated input")
	}
	for _, value := range []engine.AbilityContext{{AbilityID: "invented"}, {AbilityID: "burn", ItemID: "invented"}, {AbilityID: "burn", WeatherID: "invented"}} {
		if _, err := engine.BuildAttackProfile(value); err == nil {
			t.Fatalf("accepted unsupported rule %+v", value)
		}
	}
}

func TestAbilitiesIndependentWeatherDamageAndSpawnVisibility(t *testing.T) {
	enemy := effectEnemy()
	if err := engine.SetEnemyWeather(&enemy, "fog", true); err != nil {
		t.Fatal(err)
	}
	if !enemy.Invisible {
		t.Fatal("new fog enemy did not become invisible")
	}
	enemy.Invisible = false
	if err := engine.SetEnemyWeather(&enemy, "fog", false); err != nil {
		t.Fatal(err)
	}
	if enemy.Invisible {
		t.Fatal("weather tick re-applied spawn invisibility")
	}
	if err := engine.SetEnemyWeather(&enemy, "sandstorm", false); err != nil {
		t.Fatal(err)
	}
	engine.AdvanceWeatherEffects(&enemy, 1000)
	closeNumber(t, enemy.HP, 10000)
	enemy.Armor = 0
	engine.AdvanceWeatherEffects(&enemy, 1000)
	closeNumber(t, enemy.HP, 9970)
	if err := engine.SetEnemyWeather(&enemy, "sandAvalanche", false); err != nil {
		t.Fatal(err)
	}
	engine.AdvanceWeatherEffects(&enemy, 1000)
	closeNumber(t, enemy.HP, 9920)
	if err := engine.SetEnemyWeather(&enemy, "strongWinds", false); err != nil {
		t.Fatal(err)
	}
	closeNumber(t, engine.EffectiveEnemySpeed(&enemy), 36)
	if err := engine.SetEnemyWeather(&enemy, "invented", false); err == nil {
		t.Fatal("accepted invented weather")
	}
}

func TestAbilitiesIndependentChatterUsesEffectsVolumeRatherThanCharges(t *testing.T) {
	context := abilityContext("chatter")
	context.EffectsVolume = 8
	context.Charges = 20
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.DamageMultiplier, 0.4)
	context.EffectsVolume = 20
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.DamageMultiplier, 1)
}

func TestAbilitiesIndependentConditionalBeamStormAndChargeProfiles(t *testing.T) {
	context := abilityContext("electroBall")
	context.FlightMS = 3000
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.DamageMultiplier, 2)
	if profile.SplashRadius <= 0 || len(profile.Conditions) != 1 || profile.Conditions[0].Kind != "stun" {
		t.Fatal("electro ball did not emit executable area/control geometry")
	}
	context = abilityContext("wildboltStorm")
	context.WeatherID = "rain"
	context.Random = 0.8
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	if profile.WeatherOverride != "thunderStorm" || profile.Burst != 7 || profile.Disabled {
		t.Fatal("rain did not produce a thunder storm and bounded bolts")
	}
	context.WeatherID = ""
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	if !profile.Disabled {
		t.Fatal("storm ability fired without matching weather")
	}
	context = abilityContext("lightningRod")
	context.Charges = 10
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.Power, 200)
	if profile.SplashRadius <= 0 || profile.ConsumeCharges != 10 {
		t.Fatal("charged lightning did not consume charge on a real area attack")
	}
}

func TestAbilitiesIndependentEquipmentComposesStatusAndGeometry(t *testing.T) {
	context := abilityContext("burn")
	context.ItemID = "magmaStone"
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.Conditions) != 1 || profile.Conditions[0].DurationMS != 20000 {
		t.Fatal("magma stone did not double the real burn duration")
	}
	context = abilityContext("stunMono")
	context.ItemID = "sailFossil"
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	enemy := effectEnemy()
	engine.ApplyConditions(&enemy, profile, 0.4)
	if engine.HasCondition(&enemy, "stun") || !engine.HasCondition(&enemy, "slow") {
		t.Fatal("failed stun did not trigger fossil slow")
	}
	enemy = effectEnemy()
	engine.ApplyConditions(&enemy, profile, 0.2)
	if !engine.HasCondition(&enemy, "stun") || engine.HasCondition(&enemy, "slow") {
		t.Fatal("successful stun also incorrectly triggered fossil slow")
	}
	context = abilityContext("slow")
	context.ItemID = "bindingBand"
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	enemy = effectEnemy()
	enemy.CanSlow = false
	engine.ApplyConditions(&enemy, profile, 0)
	closeNumber(t, engine.EffectiveEnemySpeed(&enemy), 20)
	closeNumber(t, profile.AttackMS, 2500)
}

func TestAbilitiesIndependentCoverageDistinguishesWorldHooksFromRules(t *testing.T) {
	coverage := engine.AuditAbilityRules([]string{"burn", "transform", "placeholder", "invented"}, nil)
	if len(coverage) != 4 {
		t.Fatal("coverage omitted declared identifiers")
	}
	byID := map[string]engine.RuleCoverage{}
	for _, rule := range coverage {
		byID[rule.ID] = rule
	}
	if !byID["burn"].Implemented || byID["transform"].Implemented || byID["placeholder"].Implemented || byID["invented"].Implemented {
		t.Fatalf("coverage falsely reported unfinished rules: %+v", byID)
	}
	if byID["transform"].WorldHook == "" || byID["placeholder"].RemainingFacts == "" {
		t.Fatal("coverage did not explain unfinished behavior")
	}
}

func TestAbilitiesIndependentNightmareStacksUseTheLatestSourcePower(t *testing.T) {
	a := engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "nightmare", Chance: 1, DurationMS: -1, Magnitude: 10, Stacks: 1}}}
	b := engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "nightmare", Chance: 1, DurationMS: -1, Magnitude: 100, Stacks: 1}}}
	enemy := effectEnemy()
	enemy.Armor = 0
	engine.ApplyConditions(&enemy, a, 0)
	engine.ApplyConditions(&enemy, b, 0)
	engine.AdvanceConditions(&enemy, 1000)
	closeNumber(t, enemy.HP, 9800)
	if engine.ConditionStacks(&enemy, "nightmare") != 2 {
		t.Fatal("nightmare stacks were lost")
	}
}

func TestAbilitiesIndependentWeatherImmunityCannotBePiercedBySoftResistanceBypass(t *testing.T) {
	profile, err := engine.BuildAttackProfile(abilityContext("fireBlast"))
	if err != nil {
		t.Fatal(err)
	}
	enemy := effectEnemy()
	enemy.CanBurn = false
	engine.ApplyConditions(&enemy, profile, 0)
	if !engine.HasCondition(&enemy, "burn") {
		t.Fatal("fire blast did not bypass ordinary burn resistance")
	}
	enemy.PassiveID = "waterVeil"
	if err := engine.SetEnemyWeather(&enemy, "rain", false); err != nil {
		t.Fatal(err)
	}
	if !engine.HasCondition(&enemy, "burn") {
		t.Fatal("rain incorrectly cured a condition that was already active")
	}
	enemy.Conditions = nil
	engine.ApplyConditions(&enemy, profile, 0)
	if len(enemy.Conditions) != 0 {
		t.Fatal("soft resistance bypass pierced hard weather immunity")
	}
}

func TestAbilitiesIndependentReferenceStreamCadenceAndNaturalCureClock(t *testing.T) {
	context := abilityContext("flamethrower")
	context.StreamAngle = 0.4
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	if profile.BeamTickMS != 100 {
		t.Fatalf("stream tick is %d, want referenced 100ms", profile.BeamTickMS)
	}
	closeNumber(t, profile.TickDamageScale, 0.1)
	closeNumber(t, profile.ConeHalfAngle, 0.4)
	enemy := effectEnemy()
	enemy.PassiveID = "naturalCure"
	enemy.Armor = 0
	conditions := engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: 0.005, Stacks: 1}, {Kind: "poison", Chance: 1, DurationMS: -1, Magnitude: 0.001, Stacks: 2}, {Kind: "nightmare", Chance: 1, DurationMS: -1, Magnitude: 10, Stacks: 1}}}
	engine.ApplyConditions(&enemy, conditions, 0)
	engine.AdvanceConditions(&enemy, 2500)
	raw, _ := json.Marshal(enemy)
	var restored engine.EnemyEffectState
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	engine.AdvanceConditions(&enemy, 3500)
	engine.AdvanceConditions(&restored, 3500)
	if len(enemy.Conditions) != 1 || !engine.HasCondition(&enemy, "nightmare") {
		t.Fatal("natural cure did not remove only the removable conditions at 5s")
	}
	closeNumber(t, enemy.HP, 9590)
	left, _ := json.Marshal(enemy)
	right, _ := json.Marshal(restored)
	if string(left) != string(right) {
		t.Fatal("natural cure timer changed after restore")
	}
}

func TestAbilitiesIndependentPassiveConditionRemovalDoesNotInvalidateIteration(t *testing.T) {
	enemy := effectEnemy()
	enemy.Armor = 0
	enemy.PassiveID = "lunarBlessing"
	enemy.HP = 1000
	profile := engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: 0.005, Stacks: 1}, {Kind: "poison", Chance: 1, DurationMS: -1, Magnitude: 0.001, Stacks: 1}}}
	engine.ApplyConditions(&enemy, profile, 0)
	engine.AdvanceConditions(&enemy, 1000)
	if len(enemy.Conditions) != 0 || !enemy.LunarActivated {
		t.Fatal("lunar blessing did not remove conditions atomically")
	}
	closeNumber(t, enemy.Regeneration, 1000)
}

func TestAbilitiesIndependentReferenceNightmareTickAndProjectileVariants(t *testing.T) {
	context := abilityContext("nightmare")
	context.Power = 11
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	enemy := effectEnemy()
	enemy.Armor = 0
	engine.ApplyConditions(&enemy, profile, 0)
	engine.AdvanceConditions(&enemy, 999)
	closeNumber(t, enemy.HP, 10000)
	engine.AdvanceConditions(&enemy, 1)
	closeNumber(t, enemy.HP, 9997)
	for _, value := range []struct {
		random     float64
		projectile string
		condition  string
		splash     bool
	}{{0.1, "candy", "curse", false}, {0.3, "candy1", "curse", false}, {0.5, "spinarak", "poison", false}, {0.9, "apple", "", true}} {
		context = abilityContext("trickOrTreat")
		context.Random = value.random
		profile, err = engine.BuildAttackProfile(context)
		if err != nil {
			t.Fatal(err)
		}
		if profile.ProjectileOverride != value.projectile || (profile.SplashRadius > 0) != value.splash {
			t.Fatalf("wrong random projectile geometry: %+v", profile)
		}
		if value.condition != "" && (len(profile.Conditions) == 0 || profile.Conditions[0].Kind != value.condition) {
			t.Fatal("random projectile lost its real condition")
		}
	}
}

func TestAbilitiesIndependentReferenceUnburdenAndFlashFireHealing(t *testing.T) {
	context := abilityContext("unburden")
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.MovementSpeed, 50)
	closeNumber(t, profile.ArrivalThreshold, 6)
	context.ItemID = "blimpKeys"
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.MovementSpeed, 80)
	enemy := effectEnemy()
	enemy.PassiveID = "flashFire"
	enemy.HP = 9000
	engine.ApplyConditions(&enemy, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: 0.005, Stacks: 1}}}, 0)
	engine.AdvanceConditions(&enemy, 1000)
	closeNumber(t, enemy.HP, 9100)
}

func TestAbilitiesIndependentBeamGeometryCannotRicochetAcrossTheMap(t *testing.T) {
	enemies := []engine.Enemy{{ID: 1, X: 150, Y: 0, Entered: true, EnemyEffectState: engine.EnemyEffectState{HP: 100}}, {ID: 2, X: 50, Y: 4, Entered: true, EnemyEffectState: engine.EnemyEffectState{HP: 100}}, {ID: 3, X: 100, Y: 6, Entered: true, EnemyEffectState: engine.EnemyEffectState{HP: 100}}, {ID: 4, X: 250, Y: 0, Entered: true, EnemyEffectState: engine.EnemyEffectState{HP: 100}}, {ID: 5, X: -1, Y: 0, Entered: true, EnemyEffectState: engine.EnemyEffectState{HP: 100}}, {ID: 6, X: 180, Y: 0, Entered: true, EnemyEffectState: engine.EnemyEffectState{HP: 100, Invisible: true}}}
	hits := engine.LineHitIDs(engine.Point{}, engine.Point{X: 200}, 5, enemies, false)
	if len(hits) != 2 || hits[0] != 2 || hits[1] != 1 {
		t.Fatalf("wrong straight beam hits: %v", hits)
	}
	visible := engine.LineHitIDs(engine.Point{}, engine.Point{X: 200}, 5, enemies, true)
	if len(visible) != 3 || visible[2] != 6 {
		t.Fatalf("reveal ability did not add invisible beam target: %v", visible)
	}
	if enemies[0].ID != 1 {
		t.Fatal("geometry mutated source enemy order")
	}
}

func TestAbilitiesIndependentHeatCrashUsesNormalizedSpeedUnits(t *testing.T) {
	context := abilityContext("heatCrash")
	context.TargetSpeed = 60
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.DamageMultiplier, 1)
}

func TestAbilitiesIndependentReferencedItemGeometryAndTeleportBuff(t *testing.T) {
	context := abilityContext("splash")
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.SplashRadius, 65)
	context.ItemID = "dragonFang"
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.SplashRadius, 195)
	context = abilityContext("teleport")
	context.ItemID = "twistedSpoon"
	context.Teleports = 1
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.Power, 100)
	closeNumber(t, profile.ExtraDamagePct, 0.5)
	if profile.MoveMode != "teleportInPlace" || !profile.ExtraDamageCeil {
		t.Fatal("twisted spoon did not preserve in-place teleport and rounded stack bonus")
	}
	context = abilityContext("contrary")
	context.ItemID = "inverter"
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, profile.Range, 300)
	closeNumber(t, profile.InnerRange, 150)
	closeNumber(t, profile.AttackMS, 250)
	if profile.RangeShape != "donut" {
		t.Fatal("inverter did not change range geometry")
	}
	context = abilityContext("synchronySplash")
	context.ItemID = "nightmareCloth"
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	hooks := engine.AllWorldHooks(profile)
	if len(hooks) != 2 {
		t.Fatalf("held item overwrote innate hook: %v", hooks)
	}
	closeNumber(t, profile.NightmareSpreadRadius, 65)
	context = abilityContext("grassPlatform")
	context.ItemID = "berryPouch"
	profile, err = engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	if len(engine.AllWorldHooks(profile)) != 2 {
		t.Fatal("passenger berry sharing overwrote platform placement")
	}
}

func TestAbilitiesReferenceResidualArmorAndNonRefreshingConditions(t *testing.T) {
	enemy := effectEnemy()
	loss := engine.ApplyDamage(&enemy, 1500, "burn", 0)
	closeNumber(t, loss, 500)
	closeNumber(t, enemy.Armor, 0)
	profile, err := engine.BuildAttackProfile(abilityContext("slow"))
	if err != nil {
		t.Fatal(err)
	}
	engine.ApplyConditions(&enemy, profile, 0)
	engine.AdvanceConditions(&enemy, 1000)
	engine.ApplyConditions(&enemy, profile, 0)
	engine.AdvanceConditions(&enemy, 1000)
	closeNumber(t, engine.EffectiveEnemySpeed(&enemy), 40)
	disguised := effectEnemy()
	disguised.PassiveID = "disguise"
	engine.ApplyDamage(&disguised, 5000, "direct", 0)
	closeNumber(t, disguised.HP, 10000)
	closeNumber(t, disguised.Armor, 1000)
}

func TestAbilitiesReferenceDamageTicksRoundAfterStacksAndPreserveHardWeatherExceptions(t *testing.T) {
	enemy := effectEnemy()
	enemy.HP, enemy.MaxHP, enemy.Armor = 70, 70, 0
	engine.ApplyConditions(&enemy, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: 0.005, Stacks: 1}, {Kind: "poison", Chance: 1, DurationMS: -1, Magnitude: 0.001, Stacks: 2}}}, 0)
	engine.AdvanceConditions(&enemy, 1000)
	closeNumber(t, enemy.HP, 68)
	context := abilityContext("nightmare")
	context.Power = 11
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	enemy = effectEnemy()
	enemy.Armor = 0
	engine.ApplyConditions(&enemy, profile, 0)
	engine.ApplyConditions(&enemy, profile, 0)
	engine.AdvanceConditions(&enemy, 1000)
	closeNumber(t, enemy.HP, 9995)
	enemy = effectEnemy()
	enemy.PassiveID = "waterVeil"
	if err := engine.SetEnemyWeather(&enemy, "rain", false); err != nil {
		t.Fatal(err)
	}
	engine.ApplyConditions(&enemy, profile, 0)
	if !engine.HasCondition(&enemy, "nightmare") {
		t.Fatal("rain water veil incorrectly blocked nightmare")
	}
}

func TestAbilitiesReferenceResidualDamageReportsSourceAndArmorLoss(t *testing.T) {
	enemy := effectEnemy()
	engine.ApplyConditions(&enemy, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 10000, Magnitude: 0.005, Stacks: 1, SourcePokemonID: 17}}}, 0)
	records := engine.AdvanceConditions(&enemy, 1000)
	if len(records) != 1 || records[0].SourcePokemonID != 17 || records[0].Kind != "burn" {
		t.Fatalf("wrong residual source records: %+v", records)
	}
	closeNumber(t, records[0].ArmorDamage, 50)
	closeNumber(t, records[0].HPDamage, 0)
}
