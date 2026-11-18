package pokepath_test

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func TestAbilitiesRetargetPreservesFrozenSourceAndReadsLiveVictim(t *testing.T) {
	launch := abilityContext("flashFire")
	launch.Power, launch.CriticalPct = 400, 5
	frozen, err := engine.BuildAttackProfile(launch)
	if err != nil {
		t.Fatal(err)
	}
	frozen.Power, frozen.CriticalPct, frozen.CriticalMultiplier = 550, 25, 2.25
	current := launch
	current.TargetBurned, current.Power, current.CriticalPct = true, 9000, 90
	current.WeatherID = "hail"
	impact, err := engine.RetargetAttackProfile(frozen, launch, current)
	if err != nil {
		t.Fatal(err)
	}
	if impact.Power != 550 || impact.CriticalPct != 25 || impact.CriticalMultiplier != 2.25 || impact.DamageMultiplier != 2 {
		t.Fatalf("impact lost frozen source or ignored live victim: %+v", impact)
	}
	restored, err := engine.RetargetAttackProfile(frozen, launch, current)
	if err != nil || restored.Power != 550 || restored.DamageMultiplier != 2 {
		t.Fatal("retargeting mutated its input")
	}
}

func TestAbilitiesLateTechnicianChecksFinalCriticalAmount(t *testing.T) {
	context := abilityContext("technician")
	context.Power = 900
	profile, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	if !profile.Technician || profile.DamageMultiplier != 1 {
		t.Fatal("technician was decided before final impact damage")
	}
	if engine.FinalizeAttackDamage(profile, 900) != 1350 || engine.FinalizeAttackDamage(profile, 1350) != 1350 || engine.FinalizeAttackDamage(profile, 1000) != 1500 {
		t.Fatal("technician threshold or rounding is wrong")
	}
	context.AbilityID, context.ItemID, context.Power = "burn", "metalCoatReal", 1000
	profile, err = engine.BuildAttackProfile(context)
	if err != nil || !profile.Technician || profile.Power != 750 || profile.DamageMultiplier != 1 {
		t.Fatal("metal coat did not share late technician processing")
	}
}

func TestAbilitiesNewStunSerialAndAdditionalJustifiedConditionsSurviveRestore(t *testing.T) {
	enemy := passiveEnemy("justified")
	profile := engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "stun", Chance: 1, DurationMS: 1000, Stacks: 1}}}
	engine.ApplyConditions(&enemy.EnemyEffectState, profile, 0)
	engine.ApplyConditions(&enemy.EnemyEffectState, profile, 0)
	if enemy.StunApplications != 1 {
		t.Fatal("existing stun counted as a new application")
	}
	engine.AdvanceConditions(&enemy.EnemyEffectState, 1000)
	data, _ := json.Marshal(enemy)
	if err := json.Unmarshal(data, &enemy); err != nil {
		t.Fatal(err)
	}
	engine.ApplyConditions(&enemy.EnemyEffectState, profile, 0)
	if enemy.StunApplications != 2 {
		t.Fatal("stun serial was lost across condition expiration or restore")
	}
	before := enemy.Armor
	engine.ApplyConditions(&enemy.EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "scorch", Chance: 1, DurationMS: 1000, Magnitude: .01, Stacks: 1}, {Kind: "shadowTagSlow", Chance: 1, DurationMS: 1000, Magnitude: .5, Stacks: 1}}}, 0)
	if enemy.Armor != before+10000 {
		t.Fatal("justified did not react to original extra status types")
	}
	engine.ApplyConditions(&enemy.EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "auraSlow", Chance: 1, DurationMS: 1000, Magnitude: .5, Stacks: 1}}}, 0)
	if enemy.Armor != before+10000 {
		t.Fatal("synthetic aura representation incorrectly triggered justified")
	}
	victim := passiveEnemy("vCrate")
	engine.ApplyConditions(&victim.EnemyEffectState, profile, 0)
	if victim.StunApplications != 0 || victim.BaseSpeed != 36 {
		t.Fatal("converted v-crate stun was incorrectly counted as an applied stun")
	}
}

func TestAbilitiesEnemyPathsFreezeSpecialAndMirroredRoutes(t *testing.T) {
	state := passiveWorld(t)
	state.Route = 34
	enemy := passiveEnemy("ownTempo")
	enemy.Key = "superSpinda"
	engine.ConfigureEnemyPath(state, &enemy)
	path := engine.EnemyPath(state, &enemy)
	var expected [][]engine.Point
	if err := json.Unmarshal(engine.CatalogData().Routes["34"].Facts["superSpindaWaypoints"], &expected); err != nil {
		t.Fatal(err)
	}
	if len(path) != len(expected[0]) || path[1] != expected[0][1] {
		t.Fatal("super Spinda special route was not selected")
	}
	state.Progression.WaveMirror = true
	if engine.EnemyPath(state, &enemy)[1] != expected[0][1] {
		t.Fatal("live mirror setting changed an existing enemy path")
	}
	contrary := passiveEnemy("contrary")
	state.Progression.WaveMirror = false
	engine.ConfigureEnemyPath(state, &contrary)
	route := engine.CatalogData().Routes["34"]
	if engine.EnemyPath(state, &contrary)[0] != route.MirrorPaths[0][0] {
		t.Fatal("contrary did not reverse normal route direction")
	}
}

func TestAbilitiesWindAndMudPassivesAffectOnlyMovement(t *testing.T) {
	state := passiveWorld(t)
	state.Progression.CustomRoute = &engine.RouteSpec{GridColumns: 2, GridRows: 1, TileSize: 24, PlacementGrid: []int{8, 7}}
	enemy := passiveEnemy("windRider")
	enemy.X, enemy.Y = 12, 12
	if engine.EnemyTerrainMultiplier(state, &enemy, -1) != .3 || engine.EnemyTerrainMultiplier(state, &enemy, 1) != 4 {
		t.Fatal("wind rider did not use its directional original factors")
	}
	if engine.EffectiveEnemySpeedForDamage(&enemy.EnemyEffectState) != 60 {
		t.Fatal("terrain was included in speed based damage")
	}
	enemy.PassiveID = "gust"
	if engine.EnemyTerrainMultiplier(state, &enemy, -1) != 2 {
		t.Fatal("gust did not reverse opposing wind")
	}
	enemy.X, enemy.PassiveID = 36, "hustle"
	if engine.EnemyTerrainMultiplier(state, &enemy, 1) != 2 {
		t.Fatal("hustle did not reverse mud slow")
	}
}

func TestAbilitiesRetargetConsumesOnlyExplicitLiveTeleportAndAmnesiaCharges(t *testing.T) {
	for _, ability := range []string{"teleport", "amnesia"} {
		launch := abilityContext(ability)
		launch.Teleports, launch.PowerStacks, launch.Power, launch.Random = 2, 2, 100, .8
		frozen, err := engine.BuildAttackProfile(launch)
		if err != nil {
			t.Fatal(err)
		}
		frozen.Power += 30
		current := launch
		current.Teleports, current.PowerStacks, current.Power = 0, 0, 9999
		impact, err := engine.RetargetAttackProfile(frozen, launch, current)
		if err != nil || impact.Power != 130 {
			t.Fatalf("%s consumed charges did not leave frozen base and aura power: %+v %v", ability, impact, err)
		}
		frozen.Power, frozen.SourcePowerScale, frozen.SourcePowerFlat = 390, 1.2, 30
		impact, err = engine.RetargetAttackProfile(frozen, launch, current)
		if err != nil || impact.Power != 150 {
			t.Fatalf("%s consumed generic source aura composition: %+v %v", ability, impact, err)
		}
	}
	launch := abilityContext("teleport")
	launch.ItemID, launch.Teleports = "twistedSpoon", 2
	frozen, _ := engine.BuildAttackProfile(launch)
	current := launch
	current.Teleports = 0
	impact, err := engine.RetargetAttackProfile(frozen, launch, current)
	if err != nil || impact.ExtraDamagePct != 0 {
		t.Fatal("twisted spoon retained a consumed hit bonus")
	}
}

func TestAbilitiesQuickClawPreservesNativeAttackGeometry(t *testing.T) {
	for _, ability := range []string{"flamethrower", "solarBeam", "spikes", "bonemerang"} {
		context := abilityContext(ability)
		ordinary, err := engine.BuildAttackProfile(context)
		if err != nil {
			t.Fatal(err)
		}
		context.ItemID = "quickClaw"
		held, err := engine.BuildAttackProfile(context)
		if err != nil || held.AttackMode != ordinary.AttackMode {
			t.Fatalf("quick claw replaced %s geometry with target selection: %s", ability, held.AttackMode)
		}
	}
}

func TestAbilitiesShinyCharmSchemeChanceAndUTurnChargeAreNotGenericPlaceholders(t *testing.T) {
	context := abilityContext("scheme")
	ordinary, err := engine.BuildAttackProfile(context)
	if err != nil {
		t.Fatal(err)
	}
	context.ItemID = "shinyCharm"
	held, err := engine.BuildAttackProfile(context)
	if err != nil || ordinary.ShinyChance != 1.0/9000 || held.ShinyChance != 1.0/3000 {
		t.Fatal("shiny charm did not increase scheme's per-hit chance")
	}
	context.AbilityID = "burn"
	held, err = engine.BuildAttackProfile(context)
	if err != nil || held.ShinyChance != 0 {
		t.Fatal("shiny charm invented scheme on unrelated attackers")
	}
	context = abilityContext("uTurn")
	context.SameTargetHits = 20
	profile, err := engine.BuildAttackProfile(context)
	if err != nil || profile.DamageMultiplier != 1 || profile.AttackMode != "chainCharge" {
		t.Fatal("u-turn generic profile pre-applied a jump bonus owned by actual jump impact")
	}
}

func TestAbilitiesCoverageSeparatesVerifiedFactsFromMissingWorldIntegration(t *testing.T) {
	ids := []string{"anchorShot", "stickyWeb", "explosion", "bleakwindStorm"}
	for _, entry := range engine.AuditAbilityRules(ids, nil) {
		if entry.Implemented || entry.Stage != "world-hook" || entry.WorldHook == "" {
			t.Fatalf("verified geometry facts were confused with runtime integration: %+v", entry)
		}
	}
	for _, entry := range engine.AuditAbilityRules(ids, []string{"tetherTargetsInRange", "persistentWebZone", "countdownExplosion", "windDisplacement"}) {
		if !entry.Implemented || entry.Stage == "fact-gap" {
			t.Fatalf("verified geometry retained an obsolete fact gap: %+v", entry)
		}
	}
}

func TestAbilitiesSupersonicLaunchesOneOrdinaryProjectileBeforeImpactWaves(t *testing.T) {
	context := abilityContext("supersonic")
	context.AttackType = "single"
	profile, err := engine.BuildAttackProfile(context)
	if err != nil || profile.AttackMode != "single" || profile.Burst != 1 || len(profile.Conditions) != 0 {
		t.Fatalf("supersonic put impact-wave geometry on its ordinary launch: %+v", profile)
	}
	context.AbilityID = "emptyLight"
	profile, err = engine.BuildAttackProfile(context)
	if err != nil || profile.AttackMode != "wave" || profile.Burst != 2 {
		t.Fatal("empty light did not launch exactly two rings")
	}
}

func TestAbilitiesPreciselyObservedHoneyBurnAndWaveBonusesRemainDistinct(t *testing.T) {
	for _, entry := range engine.AuditAbilityRules([]string{"shadowTag", "fieryDance", "pollenPuff", "shiftGear", "psychoCutMega"}, nil) {
		if entry.Stage == "fact-gap" {
			t.Fatal("a precisely observed playable rule retained a public-description fact gap")
		}
	}
	for _, value := range []struct {
		item   string
		radius float64
	}{{"", 75}, {"dragonFang", 205}, {"nectar", 95}} {
		context := abilityContext("pollenPuff")
		context.HoneyStacks, context.ItemID = 10, value.item
		profile, err := engine.BuildAttackProfile(context)
		if err != nil || profile.SplashRadius != value.radius || profile.SplashMultiplier != .5 {
			t.Fatalf("pollen-puff honey radius is not additive: %+v", profile)
		}
	}
	context := abilityContext("fieryDance")
	context.TargetBurned, context.TargetBurnDurationMS = true, 20000
	profile, err := engine.BuildAttackProfile(context)
	if err != nil || profile.MaxHPDamage != .06 || !profile.MaxHPDamageBosses || !profile.RoundBaseDamage || !profile.ConsumeBurn {
		t.Fatal("fiery dance did not expose its nominal-duration burst and required rounding")
	}
	enemy := passiveEnemy("")
	engine.ApplyConditions(&enemy.EnemyEffectState, engine.AttackProfile{Conditions: []engine.ConditionSpec{{Kind: "burn", Chance: 1, DurationMS: 20000, Magnitude: .005, Stacks: 1}}}, 0)
	engine.AdvanceConditions(&enemy.EnemyEffectState, 17000)
	if engine.ConditionNominalDuration(&enemy.EnemyEffectState, "burn") != 20000 {
		t.Fatal("counting down a burn changed its nominal detonation duration")
	}
	context = abilityContext("psychoCutMega")
	context.SameTargetHits = 20
	profile, err = engine.BuildAttackProfile(context)
	if err != nil || profile.Range != context.Range {
		t.Fatal("psycho-cut mega invented permanent tower range from per-wave arc growth")
	}
	context = abilityContext("shiftGear")
	context.AttackCount = 100
	profile, err = engine.BuildAttackProfile(context)
	if err != nil || profile.OrbitSpeed != 0 || profile.WorldHook != "orbitAccelerationOnHit" {
		t.Fatal("shift-gear used launch count instead of actual orbital hit acceleration")
	}
	context = abilityContext("shadowTag")
	profile, err = engine.BuildAttackProfile(context)
	if err != nil || profile.WorldHook != "followingNightmareZone" || profile.TriggerDelayMS != 2000 || profile.AuraRadius != 60 || len(profile.Conditions) != 1 || profile.Conditions[0].Stacks != 2 {
		t.Fatal("shadow-tag did not isolate its moving zone and terminal two-stack nightmare")
	}
	context = abilityContext("auraSphere")
	context.ItemID, context.Hearts, context.MaxHearts = "dragonFang", 4, 14
	profile, err = engine.BuildAttackProfile(context)
	if err != nil || profile.SplashRadius != 260 {
		t.Fatal("dragon fang multiplied additive aura-sphere heart radius")
	}
	context.AbilityID, context.UltraBeastKills = "autotomize", 10
	profile, err = engine.BuildAttackProfile(context)
	if err != nil || profile.SplashRadius != 201.5 {
		t.Fatal("dragon fang multiplied additive ultra-beast radius growth")
	}
	context = abilityContext("armorBreak")
	context.TargetArmor, context.TargetMaxArmor = 0, 100
	profile, err = engine.BuildAttackProfile(context)
	if err != nil || profile.DamageMultiplier != 2 {
		t.Fatal("armor break forgot an enemy's armor identity after its current armor was destroyed")
	}
}

func TestAbilitiesLuckyPunchKeepsItsCappedRangeUntilSixSeconds(t *testing.T) {
	for _, value := range []struct {
		elapsed int64
		streak  int
		bonus   float64
	}{{4999, 1, 10}, {5000, 5, 50}, {5999, 6, 50}, {6000, 5, 0}, {6001, 5, 0}} {
		context := abilityContext("burn")
		context.ItemID, context.SinceCriticalMS, context.CriticalStreak = "luckyPunch", value.elapsed, value.streak
		profile, err := engine.BuildAttackProfile(context)
		if err != nil || profile.Range != context.Range+value.bonus {
			t.Fatalf("lucky-punch range expired at the wrong boundary: elapsed=%d streak=%d range=%v", value.elapsed, value.streak, profile.Range)
		}
	}
}

func TestAbilitiesFullIncenseFloorsCompletedWaveSecondBonuses(t *testing.T) {
	for _, value := range []struct {
		elapsed int64
		bonus   float64
	}{{0, 0}, {999, 0}, {1000, 2}, {1999, 2}, {2000, 5}, {2999, 5}, {3000, 7}} {
		context := abilityContext("burn")
		context.ItemID, context.WaveElapsedMS = "fullIncense", value.elapsed
		profile, err := engine.BuildAttackProfile(context)
		if err != nil || profile.Power != context.Power+value.bonus {
			t.Fatalf("full-incense power leaked between whole-second pulses: elapsed=%d power=%v", value.elapsed, profile.Power)
		}
	}
}

func TestAbilitiesMakeItRainAmuletCoinDoublesOnlyItsPerDigitBonus(t *testing.T) {
	for _, value := range []struct {
		item   string
		digits int
		power  float64
	}{{"", 0, 100}, {"", 4, 130}, {"amuletCoin", 0, 100}, {"amuletCoin", 4, 160}} {
		context := abilityContext("makeItRain")
		context.ItemID, context.TotalGoldDigits = value.item, value.digits
		profile, err := engine.BuildAttackProfile(context)
		if err != nil || profile.Power != value.power {
			t.Fatalf("make-it-rain digit bonus did not match its held-item variant: item=%s digits=%d power=%v", value.item, value.digits, profile.Power)
		}
	}
}

func TestAbilitiesShadowClawLeavesPrimaryTargetGeometryAndFinalRoundingToItsMode(t *testing.T) {
	context := abilityContext("shadowClaw")
	context.AttackCount, context.TargetNightmares = 4, 10
	profile, err := engine.BuildAttackProfile(context)
	if err != nil || profile.AttackMode != "sweep" || profile.Range != context.Range || profile.DamageMultiplier != 1 {
		t.Fatal("shadow claw pre-applied target-centered mode geometry or final-stage damage multipliers")
	}
}

func TestAbilitiesMeteorMashGrowsEveryFifthLaunchAndStopsAtSixTiers(t *testing.T) {
	for _, value := range []struct {
		attacks int
		base    float64
		power   float64
	}{{4, 100, 100}, {5, 100, 120}, {29, 100, 200}, {30, 100, 220}, {35, 100, 220}, {5, 101, 121}} {
		context := abilityContext("meteorMash")
		context.AttackCount, context.Power = value.attacks, value.base
		profile, err := engine.BuildAttackProfile(context)
		if err != nil || profile.Power != value.power {
			t.Fatalf("meteor-mash tier did not follow five-launch boundaries: attacks=%d power=%v", value.attacks, profile.Power)
		}
	}
}

func TestAbilitiesWideLensChangesHitShapeWithoutExtendingRange(t *testing.T) {
	for _, shape := range []string{"horizontalLine", "cross", "xShape", "circle"} {
		context := abilityContext("burn")
		context.ItemID, context.Shape = "wideLens", shape
		profile, err := engine.BuildAttackProfile(context)
		if err != nil || profile.Range != context.Range {
			t.Fatalf("wide lens extended %s range instead of only widening its supported shape: %v", shape, profile.Range)
		}
	}
}

func TestAbilitiesSoftSandUsesFrozenAreaTargetCountBeforeCritical(t *testing.T) {
	context := abilityContext("stunArea")
	context.AttackType, context.ItemID = "area", "softSand"
	profile, err := engine.BuildAttackProfile(context)
	if err != nil || !profile.SoftSand || profile.Power != context.Power {
		t.Fatal("soft sand was unavailable or applied its area bonus before target count was known")
	}
	for _, value := range []struct {
		targets int
		damage  float64
	}{{0, 101}, {1, 505}, {2, 303}, {3, 202}, {4, 151}, {5, 126}, {8, 104}, {9, 102}, {10, 102}, {100, 102}} {
		if damage := engine.SoftSandDamage(101, value.targets); damage != value.damage {
			t.Fatalf("soft-sand area target decay or floor is wrong: targets=%d damage=%v", value.targets, damage)
		}
	}
}

func TestAbilitiesSoftSandEquipGateAcceptsAreaAndRejectsWaveRestrictions(t *testing.T) {
	for _, value := range []struct {
		species string
		allowed bool
	}{{"mareep", true}, {"chingling", false}, {"charizard", false}} {
		state := passiveWorld(t)
		state.Team[0].Species = value.species
		state.Inventory = []engine.OwnedItem{{ID: "softSand"}}
		for index, key := range state.Progression.ShopStock {
			if key == "softSand" {
				state.Progression.ShopStock[index] = ""
			}
		}
		err := engine.Apply(state, 0, engine.Action{Type: "equip", PokemonID: state.Team[0].ID, ItemID: "softSand", Epoch: state.Epoch})
		if (err == nil) != value.allowed || value.allowed && state.Team[0].Item != "softSand" || !value.allowed && state.Team[0].Item != "" {
			t.Fatalf("soft-sand area eligibility or wave exclusion failed: species=%s err=%v", value.species, err)
		}
	}
}

func TestAbilitiesAllCatalogEquipmentConstructsForEveryEligibleRecipient(t *testing.T) {
	coverage, err := engine.AuditItemProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(coverage) != 197 {
		t.Fatalf("item construction audit omitted catalog definitions: %d", len(coverage))
	}
	for _, entry := range coverage {
		if !entry.Equipable || entry.Removed {
			continue
		}
		if entry.EligibleRecipients == 0 || entry.ConstructedProfiles != entry.EligibleRecipients || len(entry.Failures) != 0 {
			t.Errorf("reachable equipment has an unconstructable legal recipient: %+v", entry)
		}
	}
}

func TestAbilitiesRetargetReadsLiveCursedCountAndGoldDigitsWithFrozenAuras(t *testing.T) {
	launch := abilityContext("curse")
	launch.ItemID = "strangeIdol"
	frozen, err := engine.BuildAttackProfile(launch)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []struct {
		cursed     int
		multiplier float64
	}{{0, 1.5}, {5, 1.45}, {49, 1.01}, {50, 1}, {51, 1}, {100, 1}} {
		current := launch
		current.CursedEnemies = value.cursed
		impact, err := engine.RetargetAttackProfile(frozen, launch, current)
		if err != nil {
			t.Fatal(err)
		}
		if !impact.StrangeIdol || impact.DamageMultiplier != 1 {
			t.Fatal("strange idol was pre-applied before the final critical amount")
		}
		closeNumber(t, engine.StrangeIdolDamage(100, value.cursed), 100*value.multiplier)
	}
	if engine.StrangeIdolDamage(11, 10) != 16 {
		t.Fatal("strange idol did not round its bonus after the base critical rounded up to eleven")
	}
	launch = abilityContext("makeItRain")
	launch.TotalGoldDigits = 4
	frozen, err = engine.BuildAttackProfile(launch)
	if err != nil {
		t.Fatal(err)
	}
	frozen.SourcePowerScale, frozen.SourcePowerFlat, frozen.Power = 1.2, 20, 176
	current := launch
	current.TotalGoldDigits, current.Power = 6, 9000
	impact, err := engine.RetargetAttackProfile(frozen, launch, current)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, impact.Power, 194)
}
