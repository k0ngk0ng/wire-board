package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
)

func factNumber(facts map[string]json.RawMessage, key string) float64 {
	var value float64
	_ = json.Unmarshal(facts[key], &value)
	return value
}

func contextFor(state *State, tower *Tower, pokemon *Pokemon, target *Enemy, roll float64) AbilityContext {
	species := WorldSpecies(state, pokemon)
	route, _ := routeForState(state)
	terrain := 1
	for _, placement := range route.Placements {
		if placement.ID == tower.TileID {
			terrain = placement.Terrain
			break
		}
	}
	level := pokemon.Level
	if state.Progression.Challenge.Active && state.Progression.Challenge.LevelCap > 0 {
		level = state.Progression.Challenge.LevelCap
	}
	context := AbilityContext{PokemonID: pokemon.ID, AbilityID: species.AbilityID, ItemID: pokemon.Item, WeatherID: state.Weather, Level: level, Terrain: terrain, Stars: state.Stars, Hearts: state.Health, MaxHearts: state.MaxHealth, Targets: len(state.Enemies), Power: pokemon.Stats.Power, Range: pokemon.Stats.Range, AttackMS: pokemon.Stats.AttackMS, CriticalPct: pokemon.Stats.CriticalPct, CriticalDamage: 1.5, SameTargetHits: tower.SameTargetHits, AttackCount: tower.AttackCount, WaveKills: state.WaveKills, HealedHearts: state.WaveHeartsHealed, HeartsLost: state.WaveHeartsLost, MissingAllies: max(0, 10-len(state.Team)), Ribbons: state.Ribbons, DeployedMS: tower.DeployedMS, IdleMS: tower.IdleMS, StoredShots: tower.StoredShots, Charges: tower.Charges, Day: (state.Now/3600000+state.Clock/3600000+8)%24 >= 8 && (state.Now/3600000+state.Clock/3600000+8)%24 < 18, SpeciesID: species.ID, Random: roll, EffectsVolume: state.Progression.Settings.Audio.Effects, AttackType: species.AttackType, StreamAngle: factNumber(species.Facts, "streamAngle")}
	context.Ricochet = int(factNumber(species.Facts, "ricochet"))
	switch state.Route {
	case 3, 4, 5, 10, 21, 22, 23, 24, 42, 43, 44, 45:
		context.SandShotRoute = true
	}
	for _, ally := range state.Towers {
		if ally.ID != tower.ID && vectorLength(ally.X-tower.X, ally.Y-tower.Y) <= pokemon.Stats.Range {
			context.NearbyAllies++
		}
	}
	if target != nil {
		context.TargetHP = target.HP
		context.TargetMaxHP = target.MaxHP
		context.TargetArmor = target.Armor
		context.TargetMaxArmor = target.MaxArmor
		context.TargetSpeed = EffectiveEnemySpeedForDamage(&target.EnemyEffectState)
		context.TargetPassiveID = target.PassiveID
		context.TargetInvisible = target.Invisible
		context.TargetBurned = HasCondition(&target.EnemyEffectState, "burn")
		context.TargetBurnDurationMS = ConditionNominalDuration(&target.EnemyEffectState, "burn")
		context.TargetPoisoned = HasCondition(&target.EnemyEffectState, "poison")
		context.TargetStunned = HasCondition(&target.EnemyEffectState, "stun")
		context.TargetSlowed = HasCondition(&target.EnemyEffectState, "slow")
		context.TargetCursed = HasCondition(&target.EnemyEffectState, "curse")
		context.TargetNightmares = ConditionStacks(&target.EnemyEffectState, "nightmare")
		context.Distance = vectorLength(target.X-tower.X, target.Y-tower.Y)
	}
	if pokemon.Item == "strangeIdol" {
		excluded := 0
		if target != nil {
			excluded = target.ID
		}
		context.CursedEnemies = cursedEnemyCount(state, excluded)
	}
	WorldFillContext(state, tower, pokemon, &context)
	context.TotalGoldDigits = len(strconv.FormatInt(state.Progression.TotalGold, 10))
	context.CriticalStreak, context.SinceCriticalMS = tower.CriticalStreak, tower.SinceCriticalMS
	if pokemon.Item == "fullIncense" {
		context.WaveElapsedMS = tower.PublishedIncense * 1000
	}
	if species.AbilityID == "slowStart" {
		context.DeployedMS = tower.WaveActiveMS
	}
	if species.AbilityID == "speedBoost" || species.AbilityID == "focusEnergy" {
		context.AttackCount = int(tower.Stacks[species.AbilityID])
	}
	if state.Progression.Challenge.Active && species.AbilityID == "marvelScale" {
		context.Shape = "challenge"
	}
	return context
}

func profileFor(state *State, tower *Tower, pokemon *Pokemon, target *Enemy, roll float64) (AttackProfile, error) {
	profile, err := sourceProfileFor(state, tower, pokemon, target, roll)
	if err != nil {
		return profile, err
	}
	if err := WorldAdjustProfile(state, tower, pokemon, target, &profile); err != nil {
		return profile, err
	}
	return profile, nil
}

func sourceProfileFor(state *State, tower *Tower, pokemon *Pokemon, target *Enemy, roll float64) (AttackProfile, error) {
	profile, err := BuildAttackProfile(contextFor(state, tower, pokemon, target, roll))
	if err != nil {
		return profile, err
	}
	profile.SourcePowerScale = 1
	for index := range state.Towers {
		ally := &state.Towers[index]
		if ally.ID == tower.ID {
			continue
		}
		owner := pokemonFor(state, ally.PokemonID)
		if owner == nil {
			continue
		}
		aura, auraErr := BuildAttackProfile(contextFor(state, ally, owner, nil, 0.5))
		if auraErr != nil {
			return profile, auraErr
		}
		if aura.Aura.Shape != "" {
			continue
		}
		if aura.WorldHook == "onlyUnheldAllies" || aura.WorldHook == "justiceAllies" || vectorLength(tower.X-ally.X, tower.Y-ally.Y) > aura.Range {
			continue
		}
		if worldHasHook(aura, "globalRoomServicePower") {
			aura.Aura.PowerFlat = 0
		}
		profile.Power = profile.Power*aura.Aura.PowerMultiplier + aura.Aura.PowerFlat
		profile.SourcePowerScale *= aura.Aura.PowerMultiplier
		profile.SourcePowerFlat = profile.SourcePowerFlat*aura.Aura.PowerMultiplier + aura.Aura.PowerFlat
		profile.Range = profile.Range*aura.Aura.RangeMultiplier + aura.Aura.RangeFlat
		profile.AttackMS *= aura.Aura.AttackMultiplier
		profile.CriticalPct += aura.Aura.CriticalFlat
		profile.CriticalMultiplier *= aura.Aura.CriticalDamageMultiplier
		profile.CanSeeInvisible = profile.CanSeeInvisible || aura.Aura.CanSeeInvisible
	}
	return profile, nil
}

func inAttackRange(tower *Tower, species Species, target *Enemy, attackRange, widthScale float64) bool {
	rawX, rawY := target.X-tower.X, target.Y-tower.Y
	dx, dy := math.Abs(rawX), math.Abs(rawY)
	distance := vectorLength(dx, dy)
	width := 24 * max(1, widthScale)
	switch species.RangeType {
	case "donut":
		return distance <= attackRange && distance >= species.InnerRange
	case "cross":
		return dx <= attackRange && dy <= width || dy <= attackRange && dx <= width
	case "xShape":
		return distance <= attackRange && math.Abs(dx-dy) < width
	case "star":
		return dx <= attackRange && dy <= width || dy <= attackRange && dx <= width || distance <= attackRange && math.Abs(dx-dy) < width
	case "vShape":
		return polygonContains(Point{X: rawX, Y: rawY}, []Point{{X: 0, Y: -.06 * attackRange}, {X: -.7 * attackRange, Y: -.65 * attackRange}, {X: 0, Y: attackRange}, {X: .7 * attackRange, Y: -.65 * attackRange}})
	case "horizontalLine":
		return dx <= attackRange && dy <= width
	case "verticalLine":
		return dy <= attackRange && dx <= width
	default:
		return distance <= attackRange
	}
}

func targetsInRange(state *State, tower *Tower, pokemon *Pokemon, profile AttackProfile) []*Enemy {
	species := WorldSpecies(state, pokemon)
	species.RangeType = combatRangeType(species, *pokemon, profile)
	if profile.InnerRange > 0 {
		species.InnerRange = profile.InnerRange
	}
	candidates := []*Enemy{}
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if enemy.HP <= 0 || !enemy.Entered || enemy.Escaped || enemy.InvulnerableMS > 0 || enemy.Invisible && !profile.CanSeeInvisible || !inAttackRange(tower, species, enemy, profile.Range, combatRangeHalfWidth(state, *pokemon, species.RangeType)/24) {
			continue
		}
		candidates = append(candidates, enemy)
	}
	return candidates
}

func advanceTowers(state *State) error {
	return advanceTowersWhere(state, func(int) bool { return true })
}

func advanceTowersWhere(state *State, include func(int) bool) error {
	for index := range state.Towers {
		tower := &state.Towers[index]
		if !include(tower.ID) {
			continue
		}
		if tower.CooldownMS > 0 {
			continue
		}
		pokemon := pokemonFor(state, tower.PokemonID)
		if pokemon == nil {
			continue
		}
		if WorldSpecies(state, pokemon).AttackType == "orbital" {
			continue
		}
		profile, err := profileFor(state, tower, pokemon, nil, 0.5)
		if err != nil {
			return err
		}
		handled, err := HandleAttackMode(state, tower, pokemon, profile, contextFor(state, tower, pokemon, nil, .5), nil)
		if err != nil {
			return err
		}
		if handled {
			continue
		}
		if WorldHandlesAttack(state, tower, pokemon, profile) {
			continue
		}
		if hasAttachedBeam(state, pokemon.ID, WorldSpecies(state, pokemon).AbilityID) {
			continue
		}
		if profile.Disabled || profile.AttackMS <= 0 {
			tower.IdleMS += battleStepWholeMS(state)
			continue
		}
		targets := selectAttackTargets(state, tower, pokemon, profile)
		if len(targets) == 0 {
			tower.IdleMS += battleStepWholeMS(state)
			continue
		}
		roll := NextRandom(state)
		profile, err = profileFor(state, tower, pokemon, targets[0], roll)
		if err != nil {
			return err
		}
		if profile.Disabled {
			if profile.AttackFailure != "" {
				RegisterEnemyAttackFailure(targets[0], profile.AttackFailure)
			}
			if profile.FailureConsumesCooldown {
				tower.CooldownMS = profile.AttackMS
			}
			continue
		}
		handled, err = HandleAttackMode(state, tower, pokemon, profile, contextFor(state, tower, pokemon, targets[0], roll), targets)
		if err != nil {
			return err
		}
		if handled {
			continue
		}
		count := max(1, profile.TargetCount)
		if onlyAttachedBeam(WorldSpecies(state, pokemon).AbilityID) {
			count = 1
		}
		if profile.AttackMode == "area" || profile.AttackMode == "aura" || profile.AttackMode == "cone" {
			count = len(targets)
		}
		count = min(count, len(targets))
		allowed := true
		for _, target := range targets[:count] {
			if !EnemyAttackAllowed(state, pokemon, target, profile.AttackMode == "area" || profile.AttackMode == "aura") {
				allowed = false
				break
			}
		}
		if !allowed {
			tower.CooldownMS = profile.AttackMS
			continue
		}
		persistentArea, err := createPersistentArea(state, tower, pokemon, profile)
		if err != nil {
			return err
		}
		if !persistentArea {
			for _, target := range targets[:count] {
				context := contextFor(state, tower, pokemon, target, roll)
				context.Targets = len(targets)
				sourceProfile, err := sourceProfileFor(state, tower, pokemon, target, roll)
				if err != nil {
					return err
				}
				if profile.AttackMode == "cone" {
					if profile.ConeHalfAngle > 0 {
						angleA := math.Atan2(targets[0].Y-tower.Y, targets[0].X-tower.X)
						angleB := math.Atan2(target.Y-tower.Y, target.X-tower.X)
						difference := math.Abs(math.Atan2(math.Sin(angleB-angleA), math.Cos(angleB-angleA)))
						if difference > profile.ConeHalfAngle {
							continue
						}
					}
				}
				for shot := 0; shot < max(1, profile.Burst); shot++ {
					species := WorldSpecies(state, pokemon)
					projectileKey := species.ProjectileID
					if profile.ProjectileOverride != "" {
						projectileKey = profile.ProjectileOverride
					}
					projectileSpec, exists := CatalogData().Projectiles[projectileKey]
					if !exists {
						return fmt.Errorf("投射物素材事实不存在: %s", projectileKey)
					}
					sprite := projectileSpec.Sprite
					projectile := Projectile{ID: allocateID(state), TowerID: tower.ID, PokemonID: pokemon.ID, TargetID: target.ID, X: tower.X, Y: tower.Y, TargetX: target.X, TargetY: target.Y, Speed: projectileSpeed(species.ProjectileSpeed), Kind: profile.AttackMode, Power: profile.Power, Profile: profile, SourceProfile: sourceProfile, Context: context, DamageScale: 1, DelayMS: int64(shot) * profile.BurstIntervalMS, Bounces: profile.Ricochet, Pierces: profile.Pierce, HitIDs: []int{}, Sprite: sprite}
					if projectile.Kind == "area" || projectile.Kind == "aura" || projectile.Kind == "cone" || projectile.Kind == "beam" || projectile.Kind == "linkBeam" {
						projectile.Speed = 1e9
					}
					if projectile.Kind == "cone" {
						projectile.DamageScale = profile.TickDamageScale
					}
					if ordinaryHoming(projectile) {
						origin := ordinaryLaunchOrigin(state, tower, projectile)
						projectile.X, projectile.Y = origin.X, origin.Y
					}
					if isRay(projectile) {
						angle := math.Atan2(target.Y-tower.Y, target.X-tower.X)
						if projectile.Kind == "orthogonalBeam" {
							angle += float64(shot) * math.Pi / 2
						}
						projectile.RayDX, projectile.RayDY = math.Cos(angle), math.Sin(angle)
						projectile.RayLength = max(profile.Range, vectorLength(target.X-tower.X, target.Y-tower.Y))
						if isBeamKind(projectile.Kind) {
							configureBeamProjectile(&projectile)
						}
						if species.AbilityID == "steamEruption" {
							distance := vectorLength(target.X-tower.X, target.Y-tower.Y)
							if distance > 0 {
								projectile.RayDX, projectile.RayDY = (target.X-tower.X)/distance, (target.Y-tower.Y)/distance
							}
							projectile.RayLength = 2000
							createSteamTrail(state, tower, pokemon, &projectile)
						}
						deltaX := float64(projectile.RayDX * projectile.RayLength)
						deltaY := float64(projectile.RayDY * projectile.RayLength)
						projectile.TargetX, projectile.TargetY = tower.X+deltaX, tower.Y+deltaY
						projectile.HitAt = map[int]int64{}
					}
					state.Projectiles = append(state.Projectiles, projectile)
					addEvent(state, Event{Type: "fire", Source: tower.ID, Target: target.ID, X: projectile.X, Y: projectile.Y, Key: species.ProjectileID})
				}
			}
		}
		targetIDs := make([]int, count)
		for index, target := range targets[:count] {
			targetIDs[index] = target.ID
		}
		if err := WorldAfterAttack(state, tower.ID, profile, targetIDs); err != nil {
			return err
		}
		tower.AttackCount++
		if tower.TargetID == targets[0].ID {
			tower.SameTargetHits++
		} else {
			tower.TargetID = targets[0].ID
			tower.SameTargetHits = 1
		}
		tower.IdleMS = 0
		interval := profile.AttackMS
		if profile.AttackMode == "cone" && profile.BeamTickMS > 0 {
			interval = float64(profile.BeamTickMS)
		}
		tower.CooldownMS = max(0, interval)
	}
	return nil
}

func enemyFor(state *State, id int) *Enemy {
	for index := range state.Enemies {
		if state.Enemies[index].ID == id {
			return &state.Enemies[index]
		}
	}
	return nil
}

func updateHitContext(state *State, context *AbilityContext, enemy *Enemy, projectile *Projectile) {
	context.TargetHP, context.TargetMaxHP = enemy.HP, enemy.MaxHP
	context.TargetArmor, context.TargetMaxArmor = enemy.Armor, enemy.MaxArmor
	context.TargetSpeed = EffectiveEnemySpeedForDamage(&enemy.EnemyEffectState)
	context.TargetPassiveID = enemy.PassiveID
	context.TargetInvisible = enemy.Invisible
	context.TargetBurned = HasCondition(&enemy.EnemyEffectState, "burn")
	context.TargetBurnDurationMS = ConditionNominalDuration(&enemy.EnemyEffectState, "burn")
	context.TargetPoisoned = HasCondition(&enemy.EnemyEffectState, "poison")
	context.TargetStunned = HasCondition(&enemy.EnemyEffectState, "stun")
	context.TargetSlowed = HasCondition(&enemy.EnemyEffectState, "slow")
	context.TargetCursed = HasCondition(&enemy.EnemyEffectState, "curse")
	context.TargetNightmares = ConditionStacks(&enemy.EnemyEffectState, "nightmare")
	context.FlightMS = projectile.LifeMS
	if context.ItemID == "strangeIdol" {
		context.CursedEnemies = cursedEnemyCount(state, enemy.ID)
	}
	if context.AbilityID == "makeItRain" {
		context.TotalGoldDigits = len(strconv.FormatInt(state.Progression.TotalGold, 10))
	}
}

func applyProjectileHit(state *State, projectile *Projectile, enemy *Enemy, multiplier float64) error {
	return applyProjectileHitWithCritical(state, projectile, enemy, multiplier, projectile.ModeCritical)
}

func applyProjectileHitWithCritical(state *State, projectile *Projectile, enemy *Enemy, multiplier float64, forcedCritical *bool) error {
	if enemy.HP <= 0 {
		return nil
	}
	targetID := enemy.ID
	context := projectile.Context
	updateHitContext(state, &context, enemy, projectile)
	WorldFillHitContext(state, projectile, enemy, &context)
	frozen := projectile.SourceProfile
	if frozen.AttackMS == 0 {
		frozen = projectile.Profile
	}
	profile, err := RetargetAttackProfile(frozen, projectile.Context, context)
	if err != nil {
		return err
	}
	if tower := worldTowerFor(state, projectile.PokemonID); tower != nil {
		if pokemon := pokemonFor(state, projectile.PokemonID); pokemon != nil {
			if err := WorldAdjustProfile(state, tower, pokemon, enemy, &profile); err != nil {
				return err
			}
		}
	}
	if forcedCritical == nil {
		if tower := worldTowerFor(state, projectile.PokemonID); tower != nil {
			switch context.AbilityID {
			case "speedBoost":
				tower.Stacks["speedBoost"] = min(10, tower.Stacks["speedBoost"]+1)
			case "focusEnergy":
				tower.Stacks["focusEnergy"] = min(20, tower.Stacks["focusEnergy"]+1)
				profile.CriticalPct = min(100, profile.CriticalPct+5*(tower.Stacks["focusEnergy"]-float64(projectile.Context.AttackCount)))
			}
		}
	}
	preCurse := slices.Contains([]string{"curse", "magicBounce", "curseDoubleShot", "curseSplash", "trickOrTreat"}, context.AbilityID)
	postProfile := profile
	if preCurse {
		postProfile.Conditions = []ConditionSpec{}
		for _, condition := range profile.Conditions {
			if condition.Kind == "curse" {
				WorldApplyConditions(state, enemy, AttackProfile{Conditions: []ConditionSpec{condition}}, 0)
			} else {
				postProfile.Conditions = append(postProfile.Conditions, condition)
			}
		}
	}
	power := profile.Power
	for bounce := 0; bounce < projectile.RicochetHits; bounce++ {
		power = math.Ceil(power * projectile.Profile.RicochetMultiplier)
	}
	if profile.SoftSand {
		power = SoftSandDamage(power, context.Targets)
	}
	amount := power * profile.DamageMultiplier * projectile.DamageScale * multiplier
	if profile.SourcePowerBonusPct != 0 {
		bonus := math.Ceil(power * math.Abs(profile.SourcePowerBonusPct))
		if profile.SourcePowerBonusPct < 0 {
			bonus = -bonus
		}
		if context.ItemID == "eviolite" && (isBeamKind(projectile.Kind) || slices.Contains([]string{"area", "aura", "cone", "attackMode", "worldTick", "persistentArea"}, projectile.Kind)) {
			bonus = math.Floor(power * profile.DamageMultiplier * profile.SourcePowerBonusPct)
		}
		amount += bonus * projectile.DamageScale * multiplier
	}
	if profile.ExtraDamagePct > 0 {
		bonus := amount * profile.ExtraDamagePct
		if profile.ExtraDamageCeil {
			bonus = math.Ceil(bonus)
		} else {
			bonus = math.Floor(bonus)
		}
		amount += bonus
	}
	if profile.RoundBaseDamage {
		amount = math.Ceil(amount)
	}
	maxHealthBonus := 0.0
	if profile.MaxHPDamage > 0 && (!enemy.Boss || profile.MaxHPDamageBosses) {
		maxHealthBonus = enemy.MaxHP * profile.MaxHPDamage
		if profile.RoundBaseDamage {
			amount += math.Ceil(maxHealthBonus)
			maxHealthBonus = 0
		} else {
			maxHealthBonus = math.Floor(maxHealthBonus)
		}
	}
	critical := profile.ForcedCritical
	if forcedCritical != nil {
		critical = *forcedCritical
	} else if !critical {
		critical = NextRandom(state)*100 < profile.CriticalPct
	}
	kind := "direct"
	if critical {
		amount = math.Ceil(amount * profile.CriticalMultiplier)
		kind = "critical"
	}
	if projectile.ModeBonusPct > 0 {
		amount += math.Ceil(amount * projectile.ModeBonusPct)
	}
	if projectile.ModeFinalMultiplier > 0 {
		amount = math.Floor(amount * projectile.ModeFinalMultiplier)
	}
	amount += maxHealthBonus
	if projectile.ExtraPowerMultiplier > 0 {
		amount = math.Ceil(amount * projectile.ExtraPowerMultiplier)
	}
	linkAmount := amount
	if profile.StrangeIdol {
		amount = StrangeIdolDamage(amount, context.CursedEnemies)
	}
	projectile.RolledCritical = critical
	critical = EnemyCriticalFlag(state, critical)
	if !critical {
		kind = "direct"
	}
	if profile.ArmorRemovePct > 0 {
		amount += math.Ceil(min(enemy.Armor, enemy.MaxArmor*profile.ArmorRemovePct))
	}
	enemy = EnemyHitTarget(state, enemy)
	amount = WorldFinalizeDamage(state, enemy, context, amount)
	amount = FinalizeAttackDamage(profile, amount)
	enemy.LastHitAbilityID, enemy.LastHitItemID = context.AbilityID, context.ItemID
	enemy.LastHitTerrain, enemy.LastHitCritical = context.Terrain, critical
	enemy.LastDamageSourceID = projectile.PokemonID
	amount, allowed, err := PrepareEnemyHit(state, enemy, amount, kind)
	if err != nil {
		return err
	}
	if !allowed {
		if enemy.LastHitEvaded {
			addEvent(state, Event{Type: "hit", Source: projectile.PokemonID, Target: enemy.ID, X: enemy.X, Y: enemy.Y, Key: enemy.Key, SourceKind: "physical", Evaded: true, AttemptedDamage: amount})
		}
		return nil
	}
	before := append([]Condition{}, enemy.Conditions...)
	damageTrackedEnemy(state, enemy, projectile.PokemonID, max(0, math.Ceil(amount)), kind, profile.ArmorPierce)
	lost := enemy.LastHPDamage + enemy.LastArmorDamage + enemy.LastShieldDamage
	hitEvent := damageEventFromEnemy(enemy, projectile.PokemonID, "physical", critical)
	if critical && !enemy.LastHitEvaded && enemy.InvulnerableMS <= 0 && enemy.PassiveID != "iceBody" {
		if tower := worldTowerFor(state, projectile.PokemonID); tower != nil {
			if owner := pokemonFor(state, projectile.PokemonID); owner != nil && owner.Item == "luckyPunch" {
				tower.CriticalStreak, tower.SinceCriticalMS = min(5, tower.CriticalStreak+1), 0
			}
		}
	}
	enemy.LastHitBy = projectile.PokemonID
	fatal := enemy.HP <= 0
	deathEvents := []EnemyPassiveEvent{}
	firstRegeneration := fatal && enemy.PassiveID == "regenerator" && !enemy.PassiveFlags["regeneratorStarted"]
	if firstRegeneration {
		hitEvent.DisplayHPDamage, hitEvent.DisplayArmorDamage = 0, 0
	}
	if fatal {
		RecordProgressionHit(state, enemy, projectile, profile, amount, lost, critical, before)
		deathEvents, err = resolveEnemyDeath(state, enemy)
		if err != nil {
			return err
		}
	}
	WorldApplyConditions(state, enemy, postProfile, NextRandom(state))
	if profile.RegenerationLoss > 0 && enemy.Regeneration > 0 {
		initializePassiveState(enemy)
		enemy.Regeneration = max(0, enemy.Regeneration-profile.RegenerationLoss)
		enemy.PassiveFlags["regenerationReduced"] = true
	}
	if profile.HealBlock {
		WorldApplyConditions(state, enemy, AttackProfile{Conditions: []ConditionSpec{{Kind: "healBlock", Chance: 1, DurationMS: -1, Magnitude: 1, Stacks: 1}}}, 0)
	}
	if !fatal {
		RecordProgressionHit(state, enemy, projectile, profile, amount, lost, critical, before)
	}
	enemy.LastHitBy = projectile.PokemonID
	recordPokemonDamage(state, projectile.PokemonID, lost, hitEvent.ResolvedDamage)
	if profile.ConsumeBurn {
		enemy.Conditions = slices.DeleteFunc(enemy.Conditions, func(condition Condition) bool { return condition.Kind == "burn" })
	}
	if profile.ConsumeNightmare {
		enemy.Conditions = slices.DeleteFunc(enemy.Conditions, func(condition Condition) bool { return condition.Kind == "nightmare" })
	}

	if state.Progression.CustomRoute == nil {
		gain := int64(math.Ceil(float64(enemy.Gold)*profile.GoldTargetFraction) + math.Ceil(amount*profile.GoldDamageFraction))
		state.Gold += gain
		state.WaveGold += gain
		state.Progression.TotalGold += gain
		state.Progression.Profile.StolenGold += gain
	}
	if err := WorldOnHit(state, projectile, enemy, profile, lost); err != nil {
		return err
	}
	if err := AfterCoreProjectileHit(state, projectile, enemy, profile, linkAmount, lost, critical); err != nil {
		return err
	}

	if forcedCritical == nil {
		if original := enemyFor(state, targetID); original != nil {
			if err := OnAttackModeProjectileHit(state, projectile, original); err != nil {
				return err
			}
		}
	}
	events, err := EnemyPassiveEvents(state, enemy, "hit", 0)
	if err != nil {
		return err
	}
	afterEvents, err := EnemyAfterHit(state, enemy)
	if err != nil {
		return err
	}
	if err := executePassiveEvents(state, append(append(deathEvents, events...), afterEvents...)); err != nil {
		return err
	}
	projectile.Critical = critical
	projectile.HitIDs = append(projectile.HitIDs, targetID)
	addEvent(state, hitEvent)
	return nil
}

func advanceProjectiles(state *State) error {
	return advanceProjectilesWhere(state, func(Projectile) bool { return true })
}

func advanceProjectilesWhere(state *State, include func(Projectile) bool) error {
	initialCount := len(state.Projectiles)
	kept := make([]Projectile, initialCount)
	retained := make([]bool, initialCount)
	var order []int
	if initialCount > 1 {
		order = make([]int, initialCount)
		owners := map[int][]int{}
		for index, projectile := range state.Projectiles {
			owners[projectile.TowerID] = append(owners[projectile.TowerID], index)
		}
		for _, positions := range owners {
			for index, position := range positions {
				order[position] = positions[len(positions)-1-index]
			}
		}
	}
	for step := 0; step < initialCount; step++ {
		index := step
		if order != nil {
			index = order[step]
		}
		projectile := state.Projectiles[index]
		if !include(projectile) {
			kept[index], retained[index] = projectile, true
			continue
		}
		if projectile.DelayMS > 0 {
			projectile.DelayMS -= battleStepWholeMS(state)
			kept[index], retained[index] = projectile, true
			continue
		}
		projectile.LifeMS += battleStepWholeMS(state)
		if projectile.Kind == "orbit" || projectile.Profile.Returning {
			var alive bool
			var err error
			if projectile.Kind == "orbit" {
				alive, err = advanceOrbital(state, &projectile)
			} else {
				alive, err = advanceReturning(state, &projectile)
			}
			if err != nil {
				return err
			}
			if alive {
				kept[index], retained[index] = projectile, true
			}
			continue
		}
		lifeLimit := int64(5000)
		if projectile.Context.AbilityID == "electroBall" {
			lifeLimit = 40000
		}
		if projectile.LifeMS >= lifeLimit && !(isBeamKind(projectile.Kind) && projectile.Profile.BeamDurationMS < 0) {
			continue
		}
		if isRay(projectile) {
			alive, err := advanceRay(state, &projectile)
			if err != nil {
				return err
			}
			if alive {
				kept[index], retained[index] = projectile, true
			}
			continue
		}
		target := enemyFor(state, projectile.TargetID)
		if target == nil || target.HP <= 0 || target.Escaped || target.Invisible && !projectile.Profile.CanSeeInvisible {
			origin := Point{X: projectile.X, Y: projectile.Y}
			if target != nil {
				origin = Point{X: target.X, Y: target.Y}
			}
			target = closestProjectileEnemy(state, origin, projectile.TargetID, projectile.Profile.CanSeeInvisible)
			if target == nil {
				continue
			}
			projectile.TargetID = target.ID
		}
		projectile.TargetX, projectile.TargetY = target.X, target.Y
		origin := Point{X: target.X, Y: target.Y}
		ordinary := ordinaryHoming(projectile)
		if ordinary {
			if !advanceOrdinaryHoming(state, &projectile, target) {
				kept[index], retained[index] = projectile, true
				continue
			}
		} else {
			dx, dy := target.X-projectile.X, target.Y-projectile.Y
			distance := vectorLength(dx, dy)
			step := projectile.Speed * battleStepSeconds(state)
			if distance > step+max(8, target.Sprite.FrameWidth/2) {
				projectile.X = rounded(projectile.X + dx/distance*step)
				projectile.Y = rounded(projectile.Y + dy/distance*step)
				kept[index], retained[index] = projectile, true
				continue
			}
			projectile.X, projectile.Y = target.X, target.Y
		}
		if err := applyProjectileHit(state, &projectile, target, 1); err != nil {
			return err
		}
		primaryCritical := projectile.Critical
		primaryRolledCritical := projectile.RolledCritical
		if projectile.Profile.SplashRadius > 0 && (!projectile.Profile.SplashOnCritical || projectile.Critical) {
			for enemyIndex := range state.Enemies {
				enemy := &state.Enemies[enemyIndex]
				if enemy.ID != target.ID && enemy.HP > 0 && vectorLength(enemy.X-target.X, enemy.Y-target.Y) <= projectile.Profile.SplashRadius {
					if err := applyProjectileHitWithCritical(state, &projectile, enemy, projectile.Profile.SplashMultiplier, &primaryCritical); err != nil {
						return err
					}
				}
			}
		}
		projectile.Critical, projectile.RolledCritical = primaryCritical, primaryRolledCritical
		if err := AfterCoreProjectileSplash(state, &projectile, target, projectile.Profile); err != nil {
			return err
		}
		if projectile.Bounces > 0 || projectile.Pierces > 0 {
			next := closestProjectileEnemy(state, origin, target.ID, false)
			if next != nil {
				projectile.TargetID = next.ID
				if projectile.Bounces > 0 {
					projectile.Bounces--
					projectile.RicochetHits++
					if ordinary {
						relaunchRicochet(state, &projectile, origin, next)
						state.Projectiles = append(state.Projectiles, projectile)
						continue
					}
				} else {
					projectile.Pierces--
				}
				kept[index], retained[index] = projectile, true
			}
		}
	}
	count := 0
	for index, projectile := range kept {
		if retained[index] {
			kept[count] = projectile
			count++
		}
	}
	state.Projectiles = append(kept[:count], state.Projectiles[initialCount:]...)
	return nil
}

func isRay(projectile Projectile) bool {
	return projectile.RayLength > 0 || projectile.Pierces > 0 || projectile.Kind == "beam" || projectile.Kind == "trackingBeam" || projectile.Kind == "orthogonalBeam" || projectile.Kind == "linkBeam"
}

func advanceRay(state *State, projectile *Projectile) (bool, error) {
	beam := isBeamKind(projectile.Kind)
	if beam {
		if duration := projectile.Profile.BeamDurationMS; duration > 0 && projectile.LifeMS >= duration {
			return false, nil
		}
		if !updateBeamEndpoint(state, projectile) {
			return false, nil
		}
		projectile.PulseMS -= battleStepWholeMS(state)
		if projectile.PulseMS > 0 {
			return true, nil
		}
		projectile.PulseMS = 50
	}
	origin := Point{X: projectile.X, Y: projectile.Y}
	end := origin
	if beam {
		end.X, end.Y = projectile.TargetX, projectile.TargetY
	} else {
		distance := min(projectile.RayLength-projectile.Travelled, projectile.Speed*battleStepSeconds(state))
		if distance <= 0 {
			return false, nil
		}
		end.X += projectile.RayDX * distance
		end.Y += projectile.RayDY * distance
		projectile.Travelled += distance
	}
	if projectile.HitAt == nil {
		projectile.HitAt = map[int]int64{}
	}
	width := 6 * max(1, projectile.Profile.BeamWidthScale)
	ids := LineHitIDs(origin, end, width, state.Enemies, projectile.Profile.CanSeeInvisible)
	if beam {
		ids = beamRayHitIDs(state, projectile, origin, end)
	}
	for _, id := range ids {
		last, hit := projectile.HitAt[id]
		interval := max(int64(300), projectile.Profile.BeamTickMS)
		if beam && projectile.Profile.BeamTickMS > 0 {
			interval = projectile.Profile.BeamTickMS
		}
		if beam && projectile.LifeMS-last < interval {
			continue
		}
		if hit && (!beam || projectile.LifeMS-last < interval) {
			continue
		}
		enemy := enemyFor(state, id)
		if enemy == nil || enemy.HP <= 0 {
			continue
		}
		if err := applyProjectileHit(state, projectile, enemy, 1); err != nil {
			return false, err
		}
		projectile.HitAt[id] = projectile.LifeMS
		if !beam {
			projectile.Pierces--
			if projectile.Pierces < 0 {
				return false, nil
			}
		}
	}
	if beam {
		return true, nil
	}
	projectile.X, projectile.Y = rounded(end.X), rounded(end.Y)
	return projectile.Travelled < projectile.RayLength, nil
}

func projectileSpeed(raw float64) float64 {
	if raw <= 30 {
		return raw * 60
	}
	return raw
}

func closestProjectileEnemy(state *State, origin Point, excludedID int, canSeeInvisible bool) *Enemy {
	var closest *Enemy
	best := 200.0
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if enemy.ID == excludedID || enemy.HP <= 0 || enemy.Dead || enemy.Escaped || enemy.Invisible && !canSeeInvisible {
			continue
		}
		if distance := vectorLength(enemy.X-origin.X, enemy.Y-origin.Y); distance < best {
			closest, best = enemy, distance
		}
	}
	return closest
}

func cursedEnemyCount(state *State, excludedID int) int {
	count := 0
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if enemy.ID != excludedID && enemy.HP > 0 && !enemy.Dead && !enemy.Escaped && HasCondition(&enemy.EnemyEffectState, "curse") {
			count++
		}
	}
	return count
}
