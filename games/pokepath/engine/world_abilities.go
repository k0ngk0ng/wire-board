package engine

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

type WorldTowerState struct {
	Origin          Point              `json:"origin"`
	Destination     Point              `json:"destination"`
	OrbitPosition   *Point             `json:"orbitPosition,omitempty"`
	Angle           float64            `json:"angle"`
	IdleMS          float64            `json:"idleMS"`
	ChargeMS        float64            `json:"chargeMS"`
	BurstMS         float64            `json:"burstMS"`
	BurstRemaining  int                `json:"burstRemaining"`
	BurstTarget     int                `json:"burstTarget"`
	FlightTarget    int                `json:"flightTarget"`
	Teleports       int                `json:"teleports"`
	PowerStacks     int                `json:"powerStacks"`
	BoostMS         int64              `json:"boostMS"`
	Honey           int                `json:"honey"`
	BerryTimers     map[string]int64   `json:"berryTimers,omitempty"`
	ExposureMS      map[int]int64      `json:"exposureMS,omitempty"`
	ExposureApplied map[int]bool       `json:"exposureApplied,omitempty"`
	Chains          map[int]int64      `json:"chains,omitempty"`
	WaveDamage      float64            `json:"waveDamage"`
	Flags           map[string]bool    `json:"flags,omitempty"`
	Timers          map[string]int64   `json:"timers,omitempty"`
	Values          map[string]float64 `json:"values,omitempty"`
}

type WorldLink struct {
	First  int             `json:"first"`
	Second int             `json:"second"`
	AgeMS  float64         `json:"ageMS"`
	HitAt  map[int]float64 `json:"hitAt"`
}

type WorldZone struct {
	ID             int           `json:"id"`
	Kind           string        `json:"kind"`
	PokemonID      int           `json:"pokemonID"`
	X              float64       `json:"x"`
	Y              float64       `json:"y"`
	Radius         float64       `json:"radius"`
	InnerRadius    float64       `json:"innerRadius,omitempty"`
	Smoke          bool          `json:"smoke,omitempty"`
	SlowFactor     float64       `json:"slowFactor,omitempty"`
	Magma          bool          `json:"magma,omitempty"`
	PersistentArea bool          `json:"persistentArea,omitempty"`
	SourceTower    *Tower        `json:"sourceTower,omitempty"`
	AgeMS          int64         `json:"ageMS"`
	LifeMS         int64         `json:"lifeMS"`
	TickMS         int64         `json:"tickMS"`
	HitAt          map[int]int64 `json:"hitAt,omitempty"`
	DelayMS        int64         `json:"delayMS"`
	Phase          string        `json:"phase,omitempty"`
	TargetID       int           `json:"targetID"`
	TargetX        float64       `json:"targetX"`
	TargetY        float64       `json:"targetY"`
	Origin         Point         `json:"origin"`
	Direction      Point         `json:"direction"`
	Speed          float64       `json:"speed"`
	Distance       float64       `json:"distance"`
	Angle          float64       `json:"angle"`
}

type WorldDisplacement struct {
	Direction Point   `json:"direction"`
	Remaining float64 `json:"remaining"`
}

type WorldState struct {
	HealingUses         map[int]float64           `json:"healingUses,omitempty"`
	HealingDamage       map[int]float64           `json:"healingDamage,omitempty"`
	HealingItemUsed     map[string]bool           `json:"healingItemUsed,omitempty"`
	HealScale           int                       `json:"healScale"`
	RoomServicePower    int                       `json:"roomServicePower"`
	PendingHeal         int                       `json:"pendingHeal"`
	Towers              map[int]*WorldTowerState  `json:"towers,omitempty"`
	PassengerHosts      map[int]int               `json:"passengerHosts,omitempty"`
	Links               []WorldLink               `json:"links,omitempty"`
	Zones               []WorldZone               `json:"zones,omitempty"`
	MovementLocked      bool                      `json:"movementLocked"`
	Pressure            bool                      `json:"pressure"`
	AnchorShotTriggered bool                      `json:"anchorShotTriggered"`
	StrippedItems       map[int]string            `json:"strippedItems,omitempty"`
	HeavyMetalUsed      bool                      `json:"heavyMetalUsed"`
	SturdyUsed          bool                      `json:"sturdyUsed"`
	IceBodyActivated    bool                      `json:"iceBodyActivated"`
	ShellSmashActive    bool                      `json:"shellSmashActive"`
	HitsReceived        int                       `json:"hitsReceived"`
	WaveElapsedMS       int64                     `json:"waveElapsedMS"`
	UltraBeastKills     int                       `json:"ultraBeastKills"`
	Honey               int                       `json:"honey"`
	ChessActive         []string                  `json:"chessActive,omitempty"`
	ChessRecomputeAt    int64                     `json:"chessRecomputeAt"`
	EnemyStunSerial     map[int]int               `json:"enemyStunSerial,omitempty"`
	EnemyWeather        string                    `json:"enemyWeather,omitempty"`
	AppliedWeather      string                    `json:"appliedWeather,omitempty"`
	Displacements       map[int]WorldDisplacement `json:"displacements,omitempty"`
	PaidBeamIDs         map[int]bool              `json:"paidBeamIDs,omitempty"`
	PendingEnemyEvents  []EnemyPassiveEvent       `json:"pendingEnemyEvents,omitempty"`
}

func worldEnsure(state *State) {
	if state.World.Towers == nil {
		state.World.Towers = map[int]*WorldTowerState{}
	}
	if state.World.PassengerHosts == nil {
		state.World.PassengerHosts = map[int]int{}
	}
	if state.World.EnemyStunSerial == nil {
		state.World.EnemyStunSerial = map[int]int{}
	}
}

func worldTower(state *State, tower *Tower) *WorldTowerState {
	worldEnsure(state)
	value := state.World.Towers[tower.PokemonID]
	if value == nil {
		value = &WorldTowerState{Origin: Point{X: tower.X, Y: tower.Y}, Destination: Point{X: tower.X, Y: tower.Y}, BerryTimers: map[string]int64{}, ExposureMS: map[int]int64{}, ExposureApplied: map[int]bool{}, Chains: map[int]int64{}, Flags: map[string]bool{}, Timers: map[string]int64{}, Values: map[string]float64{}}
		state.World.Towers[tower.PokemonID] = value
	}
	if value.BerryTimers == nil {
		value.BerryTimers = map[string]int64{}
	}
	if value.ExposureMS == nil {
		value.ExposureMS = map[int]int64{}
	}
	if value.ExposureApplied == nil {
		value.ExposureApplied = map[int]bool{}
	}
	if value.Chains == nil {
		value.Chains = map[int]int64{}
	}
	if value.Flags == nil {
		value.Flags = map[string]bool{}
	}
	if value.Timers == nil {
		value.Timers = map[string]int64{}
	}
	if value.Values == nil {
		value.Values = map[string]float64{}
	}
	return value
}

func worldTowerFor(state *State, pokemonID int) *Tower {
	for index := range state.Towers {
		if state.Towers[index].PokemonID == pokemonID {
			return &state.Towers[index]
		}
	}
	return nil
}

func worldHasHook(profile AttackProfile, hook string) bool {
	return slices.Contains(AllWorldHooks(profile), hook)
}

func WorldSpecies(state *State, pokemon *Pokemon) Species {
	return ProgressionSpecies(state, pokemon)
}

func worldHasAbility(state *State, ability string) bool {
	for _, tower := range state.Towers {
		if owner := pokemonFor(state, tower.PokemonID); owner != nil && WorldSpecies(state, owner).AbilityID == ability {
			return true
		}
	}
	return false
}

func worldPassengerHost(state *State, pokemon *Pokemon, placement Placement) *Tower {
	species := WorldSpecies(state, pokemon)
	for index := range state.Towers {
		host := &state.Towers[index]
		if host.TileID != placement.ID || host.PokemonID == pokemon.ID || state.World.PassengerHosts[host.PokemonID] != 0 {
			continue
		}
		occupied := false
		for passenger, hostID := range state.World.PassengerHosts {
			occupied = occupied || hostID == host.PokemonID && passenger != pokemon.ID
		}
		if occupied {
			continue
		}
		owner := pokemonFor(state, host.PokemonID)
		if owner == nil {
			continue
		}
		switch WorldSpecies(state, owner).AbilityID {
		case "grassPlatform":
			if terrainAllowed(*pokemon, species, 1) || terrainAllowed(*pokemon, species, 2) {
				return host
			}
		case "icePlatform":
			if terrainAllowed(*pokemon, species, 1) || terrainAllowed(*pokemon, species, 3) {
				return host
			}
		case "rest":
			return host
		case "mount":
			if worldClassificationID(state, pokemon) == 115 {
				return host
			}
		}
	}
	return nil
}

func WorldPlacementAllowed(state *State, pokemon *Pokemon, placement Placement) (bool, error) {
	if state.World.MovementLocked || state.World.Pressure {
		return false, errors.New("本波的部署位置已经锁定")
	}
	return worldPassengerHost(state, pokemon, placement) != nil, nil
}

func WorldOnDeploy(state *State, pokemonID, tileID int) error {
	worldEnsure(state)
	tower := worldTowerFor(state, pokemonID)
	owner := pokemonFor(state, pokemonID)
	if tower == nil || owner == nil {
		return errors.New("部署伙伴不存在")
	}
	value := worldTower(state, tower)
	value.Origin = Point{X: tower.X, Y: tower.Y}
	value.Destination = value.Origin
	value.OrbitPosition = nil
	delete(state.World.PassengerHosts, pokemonID)
	if host := worldPassengerHost(state, owner, Placement{ID: tileID}); host != nil {
		state.World.PassengerHosts[pokemonID] = host.PokemonID
		tower.X, tower.Y = host.X, host.Y
	}
	if state.WaveActive && WorldSpecies(state, owner).AbilityID == "heavyMetal" && !state.World.HeavyMetalUsed {
		state.World.HeavyMetalUsed = true
		state.World.MovementLocked = true
		duration := int64(2000)
		if owner.Item == "metalCoat" {
			duration = 4000
		}
		for index := range state.Enemies {
			WorldApplyConditions(state, &state.Enemies[index], AttackProfile{Conditions: []ConditionSpec{status("stun", 1, duration, 1, 1)}}, 0)
		}
	}
	state.World.ChessRecomputeAt = state.Tick + 50
	return nil
}

func WorldOnRetire(state *State, pokemonID int) error {
	if state.World.MovementLocked || state.World.Pressure {
		return errors.New("本波的部署位置已经锁定")
	}
	retiring := map[int]bool{}
	for _, tower := range state.Towers {
		if tower.PokemonID == pokemonID || state.World.PassengerHosts[tower.PokemonID] == pokemonID {
			retiring[tower.ID] = true
		}
	}
	state.World.Links = slices.DeleteFunc(state.World.Links, func(link WorldLink) bool { return retiring[link.First] || retiring[link.Second] })
	state.World.Zones = slices.DeleteFunc(state.World.Zones, func(zone WorldZone) bool {
		return !zone.PersistentArea && (zone.PokemonID == pokemonID || state.World.PassengerHosts[zone.PokemonID] == pokemonID)
	})
	for passenger, host := range state.World.PassengerHosts {
		if host == pokemonID {
			if owner := pokemonFor(state, passenger); owner != nil {
				owner.TileID = -1
			}
			state.Towers = slices.DeleteFunc(state.Towers, func(tower Tower) bool { return tower.PokemonID == passenger })
			state.Projectiles = slices.DeleteFunc(state.Projectiles, func(projectile Projectile) bool { return projectile.PokemonID == passenger })
			delete(state.World.Towers, passenger)
			delete(state.World.PassengerHosts, passenger)
		}
	}
	delete(state.World.PassengerHosts, pokemonID)
	delete(state.World.Towers, pokemonID)
	state.World.ChessRecomputeAt = state.Tick + 50
	return nil
}

func WorldFillContext(state *State, tower *Tower, pokemon *Pokemon, context *AbilityContext) {
	value := state.World.Towers[tower.PokemonID]
	if value == nil {
		value = &WorldTowerState{}
	}
	context.PokemonID = pokemon.ID
	context.Teleports, context.PowerStacks = value.Teleports, value.PowerStacks
	context.BoostRemainingMS, context.WaveElapsedMS = value.BoostMS, state.World.WaveElapsedMS
	context.HoneyStacks, context.UltraBeastKills = state.World.Honey, state.World.UltraBeastKills
	context.ShellSmashActive, context.HeartsLost = state.World.ShellSmashActive, state.World.HitsReceived
	context.DamageThisWave = max(value.WaveDamage, state.World.HealingDamage[pokemon.ID])
	context.HealingUses = state.World.HealingUses[pokemon.ID]
	context.HealedHearts = max(state.WaveHeartsHealed, state.World.HealScale)
	context.CloudNine = worldHasAbility(state, "cloudNine")
	context.Carried = state.World.PassengerHosts[pokemon.ID] != 0
	context.BerryActive = value.BerryTimers[pokemon.Item] > 0
	context.HealingUnit = slices.Contains([]string{"heal", "bitterBlade", "takeHeart", "seedFlare", "soulHeartReactor"}, context.AbilityID)
	context.FossilCount, context.PastParadox, context.FutureParadox = 0, 0, 0
	for index := range state.Team {
		classification := worldClassificationID(state, &state.Team[index])
		if slices.Contains([]int{58, 59, 63, 64, 65, 66, 94, 140, 136, 151, 247}, classification) {
			context.FossilCount++
		}
		if slices.Contains([]int{179, 239}, classification) {
			context.PastParadox++
		}
		if slices.Contains([]int{178, 238}, classification) {
			context.FutureParadox++
		}
	}
	if host := worldTowerFor(state, state.World.PassengerHosts[pokemon.ID]); host != nil {
		if owner := pokemonFor(state, host.PokemonID); owner != nil {
			switch WorldSpecies(state, owner).AbilityID {
			case "grassPlatform":
				context.Terrain = 2
			case "icePlatform":
				context.Terrain = 3
			}
		}
	}
}

func WorldFillHitContext(state *State, projectile *Projectile, enemy *Enemy, context *AbilityContext) {
	if tower := worldTowerFor(state, projectile.PokemonID); tower != nil {
		if owner := pokemonFor(state, projectile.PokemonID); owner != nil {
			WorldFillContext(state, tower, owner, context)
		}
	}
}

func WorldAdjustProfile(state *State, tower *Tower, pokemon *Pokemon, target *Enemy, profile *AttackProfile) error {
	species := WorldSpecies(state, pokemon)
	worldAdjustBerries(state, tower, pokemon, target, profile)
	hostID := state.World.PassengerHosts[pokemon.ID]
	if host := worldTowerFor(state, hostID); host != nil {
		owner := pokemonFor(state, hostID)
		if owner != nil {
			hostProfile, err := BuildAttackProfile(contextFor(state, host, owner, nil, .5))
			if err != nil {
				return err
			}
			profile.Power += hostProfile.PassengerPower
			switch WorldSpecies(state, owner).AbilityID {
			case "grassPlatform", "icePlatform":
				profile.AttackMS *= 0.85
			case "mount":
				if species.AbilityID == "supremeCommanders" {
					profile.Power = math.Ceil(profile.Power * 1.5)
					profile.AttackMS *= 0.75
					profile.CriticalPct += 30
				}
			}
		}
	}
	if species.AbilityID == "mount" {
		for _, host := range state.World.PassengerHosts {
			if host == pokemon.ID {
				profile.Power = math.Ceil(profile.Power * 1.5)
				profile.AttackMS *= 0.75
				profile.CriticalPct += 30
				break
			}
		}
	}
	for index := range state.Towers {
		ally := &state.Towers[index]
		owner := pokemonFor(state, ally.PokemonID)
		if owner == nil {
			continue
		}
		ability := WorldSpecies(state, owner).AbilityID
		if ability == "helpingHand" && (pokemon.Item == "" || species.AbilityID == "scrappy") {
			flat := float64(50)
			if species.AbilityID == "simple" {
				flat = 88
			}
			profile.Power += flat
		}
		if slices.Contains([]int{165, 175, 176, 233}, worldClassificationID(state, pokemon)) {
			switch ability {
			case "swordsDance":
				profile.Power = math.Ceil(profile.Power * 1.2)
			case "heartOfSteel":
				factor := float64(1)
				if state.World.HitsReceived > 0 {
					factor = 3
				}
				profile.Power += 25 * factor
				profile.CriticalPct += 5 * factor
			case "sacredSword":
				profile.Range *= 1.2
			}
		}
	}
	if hasRoomService(state) {
		profile.Power += float64(state.World.RoomServicePower)
	}
	if err := worldShapeAuras(state, pokemon, profile); err != nil {
		return err
	}
	if species.AbilityID == "bulletPunch" || species.AbilityID == "bulletPunchMega" {
		profile.BurstIntervalMS = 90
	}
	if species.AbilityID == "spiritShackle" {
		profile.Burst, profile.SplashRadius = 1, 0
	}
	if state.World.AnchorShotTriggered {
		profile.AttackMS *= 4
	}
	if state.Weather == "hail" && state.World.IceBodyActivated && worldHasAbility(state, "iceBody") {
		profile.CriticalMultiplier *= 1.2
	}
	honeyMultiplier := 1
	if pokemon.Item == "nectar" {
		honeyMultiplier = 3
		if species.AbilityID == "simple" {
			honeyMultiplier = 4
		}
	}
	profile.Power += float64(state.World.Honey * honeyMultiplier)
	return nil
}

func WorldBeforeStep(state *State) error {
	worldEnsure(state)
	if err := worldUpdateWeather(state); err != nil {
		return err
	}
	worldAdvanceDisplacements(state)
	worldUpdateChess(state)
	for id := range state.World.PaidBeamIDs {
		alive := false
		for _, projectile := range state.Projectiles {
			alive = alive || projectile.ID == id
		}
		if !alive {
			delete(state.World.PaidBeamIDs, id)
		}
	}
	if state.WaveActive {
		state.World.WaveElapsedMS += battleStepWholeMS(state)
	}
	return worldAdvanceZones(state)
}

func worldAdvanceDeployedTower(state *State, tower *Tower, lastStep bool) error {
	owner := pokemonFor(state, tower.PokemonID)
	if owner == nil {
		return nil
	}
	value := worldTower(state, tower)
	value.BoostMS = max(0, value.BoostMS-battleStepWholeMS(state))
	for key, duration := range value.BerryTimers {
		value.BerryTimers[key] = max(0, duration-battleStepWholeMS(state))
	}
	profile, err := profileFor(state, tower, owner, nil, 0.5)
	if err != nil {
		return err
	}
	return worldAdvanceTower(state, tower, owner, value, profile, lastStep)
}

func worldAdvanceTower(state *State, tower *Tower, owner *Pokemon, value *WorldTowerState, profile AttackProfile, lastStep bool) error {
	species := WorldSpecies(state, owner)
	if worldHasHook(profile, "flyingTowerMovement") {
		worldMoveFlying(state, tower, owner, value, profile, lastStep)
	}
	if species.AbilityID == "plus" {
		worldRebuildLinks(state, tower)
	}
	if worldHasHook(profile, "idleRechargeCharges") || worldHasHook(profile, "idleRechargeChargesAndKillRefill") {
		if err := worldAdvancePunch(state, tower, owner, value, profile); err != nil {
			return err
		}
	}
	if !state.WaveActive {
		return nil
	}
	targets := targetsInRange(state, tower, owner, profile)
	specialAttack := worldHasHook(profile, "pullToProjectile") || worldHasHook(profile, "windDisplacement") || worldHasHook(profile, "storeAmnesiaPower") || worldHasHook(profile, "followingNightmareZone")
	if specialAttack || worldHasHook(profile, "halfDamageExecuteAura") || worldHasHook(profile, "tetherTargetsInRange") || worldHasHook(profile, "countdownExplosion") {
		orderTargets(state, targets, selectionMode(state, owner, profile, targets))
	}
	if len(targets) == 0 && !profile.Disabled && !WorldHandlesAttack(state, tower, owner, profile) {
		worldAttackIdleToy(state, tower, owner, profile)
	}
	if specialAttack {
		worldAdvanceSpecialAttack(state, tower, owner, value, profile, targets)
	}
	if worldHasHook(profile, "halfDamageExecuteAura") {
		inside := map[string]bool{}
		for _, enemy := range targets {
			key := fmt.Sprintf("maw:%d", enemy.ID)
			if enemy.HP < enemy.MaxHP*0.5 {
				inside[key] = true
				if !value.Flags[key] {
					value.Flags[key] = true
					worldLaunch(state, tower, owner, enemy, profile)
					state.Projectiles[len(state.Projectiles)-1].ExtraPowerMultiplier = 0.5
				}
			}
		}
		for key := range value.Flags {
			if len(key) > 4 && key[:4] == "maw:" && !inside[key] {
				delete(value.Flags, key)
			}
		}
	}
	if worldHasHook(profile, "tetherTargetsInRange") {
		if err := worldAdvanceChains(state, tower, owner, value, profile, targets); err != nil {
			return err
		}
	}
	if worldHasHook(profile, "countdownExplosion") {
		if err := worldAdvanceExplosion(state, tower, owner, value, profile, targets); err != nil {
			return err
		}
	}
	if species.AbilityID == "teleport" {
		if len(targets) == 0 {
			value.IdleMS += battleStepMS(state)
			period := profile.TeleportIntervalMS
			if period <= 0 {
				period = 10000.0 / 3
			}
			if value.IdleMS >= period {
				worldTeleport(state, tower, owner, value)
				value.IdleMS -= period
			}
		} else {
			value.IdleMS = 0
		}
	}
	if worldHasHook(profile, "continuousRangeExposure") || worldHasHook(profile, "continuousRangeConditions") {
		inside := map[int]bool{}
		for _, enemy := range targets {
			inside[enemy.ID] = true
			if profile.OncePerTarget {
				if value.ExposureApplied[enemy.ID] {
					continue
				}
				value.ExposureMS[enemy.ID] += battleStepWholeMS(state)
				if value.ExposureMS[enemy.ID] < profile.TriggerDelayMS {
					continue
				}
				value.ExposureApplied[enemy.ID] = true
			}
			condition := profile.AuraCondition
			condition.SourcePokemonID = owner.ID
			WorldApplyConditions(state, enemy, AttackProfile{Conditions: []ConditionSpec{condition}}, 0)
			if profile.HealBlock {
				WorldApplyConditions(state, enemy, AttackProfile{Conditions: []ConditionSpec{status("healBlock", 1, battleStepWholeMS(state)*2, 1, 1)}}, 0)
			}
		}
		for id := range value.ExposureMS {
			if !inside[id] {
				delete(value.ExposureMS, id)
			}
		}
	}
	return nil
}

func worldLaunch(state *State, tower *Tower, owner *Pokemon, target *Enemy, profile AttackProfile) {
	species := WorldSpecies(state, owner)
	context := contextFor(state, tower, owner, target, NextRandom(state))
	projectile := Projectile{ID: allocateID(state), TowerID: tower.ID, PokemonID: owner.ID, TargetID: target.ID, X: tower.X, Y: tower.Y, TargetX: target.X, TargetY: target.Y, Speed: max(60, species.ProjectileSpeed*60), Kind: "single", Power: profile.Power, Profile: profile, Context: context, DamageScale: 1, HitIDs: []int{}, Sprite: CatalogData().Projectiles[species.ProjectileID].Sprite}
	projectile.SourceProfile, _ = sourceProfileFor(state, tower, owner, target, context.Random)
	if ordinaryHoming(projectile) {
		origin := ordinaryLaunchOrigin(state, tower, projectile)
		projectile.X, projectile.Y = origin.X, origin.Y
	}
	state.Projectiles = append(state.Projectiles, projectile)
	addEvent(state, Event{Type: "fire", Source: tower.ID, Target: target.ID, X: projectile.X, Y: projectile.Y, Key: species.ProjectileID})
}

func worldAdvancePunch(state *State, tower *Tower, owner *Pokemon, value *WorldTowerState, profile AttackProfile) error {
	maximum := profile.RequiredCharges
	if value.BurstRemaining > 0 {
		target := enemyFor(state, value.BurstTarget)
		if target == nil || target.HP <= 0 || target.Escaped {
			value.BurstRemaining = 0
			return nil
		}
		value.BurstMS += battleStepMS(state)
		for value.BurstRemaining > 0 && value.BurstMS >= 90 {
			value.BurstMS -= 90
			value.BurstRemaining--
			worldLaunch(state, tower, owner, target, profile)
		}
		return nil
	}
	if tower.Charges < maximum {
		value.ChargeMS += battleStepMS(state)
		interval := max(1, profile.AttackMS)
		for tower.Charges < maximum && value.ChargeMS >= interval {
			value.ChargeMS -= interval
			tower.Charges++
		}
	}
	if !state.WaveActive || tower.Charges < maximum {
		return nil
	}
	targets := selectTargets(state, tower, owner, profile)
	if len(targets) == 0 {
		return nil
	}
	value.BurstRemaining, value.BurstTarget, value.BurstMS = tower.Charges-1, targets[0].ID, 0
	tower.Charges = 0
	value.ChargeMS = 0
	worldLaunch(state, tower, owner, targets[0], profile)
	return nil
}

func worldTeleport(state *State, tower *Tower, owner *Pokemon, value *WorldTowerState) {
	if owner.Item == "twistedSpoon" {
		value.Teleports++
		addEvent(state, Event{Type: "teleport", Source: owner.ID, X: tower.X, Y: tower.Y, Amount: float64(value.Teleports)})
		return
	}
	route, _ := routeForState(state)
	legal := []Placement{}
	for _, placement := range route.Placements {
		if placement.ID == tower.TileID {
			continue
		}
		occupied := false
		for _, existing := range state.Towers {
			occupied = occupied || existing.TileID == placement.ID
		}
		if !occupied && slices.Contains([]int{1, 2, 4}, placement.Terrain) || worldPassengerHost(state, owner, placement) != nil {
			legal = append(legal, placement)
		}
	}
	if len(legal) == 0 {
		return
	}
	placement := legal[NextInt(state, len(legal))]
	tileSize := max(1, route.TileSize)
	tower.TileID, owner.TileID = placement.ID, placement.ID
	tower.X, tower.Y = placement.X+float64(tileSize)/2, placement.Y+float64(tileSize)/2
	delete(state.World.PassengerHosts, owner.ID)
	if host := worldPassengerHost(state, owner, placement); host != nil {
		state.World.PassengerHosts[owner.ID] = host.PokemonID
		tower.X, tower.Y = host.X, host.Y
	}
	value.Teleports++
	state.Projectiles = slices.DeleteFunc(state.Projectiles, func(projectile Projectile) bool { return projectile.PokemonID == owner.ID })
	addEvent(state, Event{Type: "teleport", Source: owner.ID, X: tower.X, Y: tower.Y, Amount: float64(value.Teleports)})
}

func worldMoveFlying(state *State, tower *Tower, owner *Pokemon, value *WorldTowerState, profile AttackProfile, lastStep bool) {
	direction := float64(1)
	if owner.Item == "inverter" {
		direction = -1
	}
	switch profile.MoveMode {
	case "circleOrigin":
		value.Angle += direction * battleStepSeconds(state)
		tower.X = rounded(value.Origin.X + math.Cos(value.Angle)*70)
		tower.Y = rounded(value.Origin.Y + math.Sin(value.Angle)*70)
		position := Point{X: rounded(tower.X - owner.Sprite.FrameWidth/2), Y: rounded(tower.Y - owner.Sprite.FrameHeight/2)}
		value.OrbitPosition = &position
		if lastStep {
			tower.X, tower.Y = position.X+12, position.Y+12
		}
	case "circleTarget":
		mode := forcedTargetMode(state, owner)
		targets := []*Enemy{}
		for index := range state.Enemies {
			enemy := &state.Enemies[index]
			if enemy.HP > 0 && !enemy.Dead && !enemy.Escaped && enemy.InvulnerableMS <= 0 && (!enemy.Invisible || profile.CanSeeInvisible) {
				targets = append(targets, enemy)
			}
		}
		orderTargets(state, targets, mode)
		if len(targets) == 0 {
			value.FlightTarget = 0
			return
		}
		target := targets[0]
		if value.FlightTarget != target.ID {
			value.FlightTarget = target.ID
			value.Angle = math.Atan2(tower.Y-target.Y, tower.X-target.X)
		}
		destination := Point{X: target.X + math.Cos(value.Angle)*60, Y: target.Y + math.Sin(value.Angle)*60}
		if worldMoveToward(state, tower, destination, 300) <= 1 {
			value.Angle += direction * 2.5 * battleStepSeconds(state)
		}
	case "followPointer":
		if vectorLength(tower.X-value.Destination.X, tower.Y-value.Destination.Y) >= max(1, profile.ArrivalThreshold) {
			worldMoveToward(state, tower, value.Destination, max(1, profile.MovementSpeed))
		}
	case "wander":
		if vectorLength(tower.X-value.Destination.X, tower.Y-value.Destination.Y) <= max(1, profile.ArrivalThreshold) {
			route, _ := routeForState(state)
			value.Destination = Point{X: 16 + NextRandom(state)*float64(max(1, route.Width-32)), Y: 16 + NextRandom(state)*float64(max(1, route.Height-32))}
		}
		worldMoveToward(state, tower, value.Destination, max(1, profile.MovementSpeed))
	}
}

func worldMoveToward(state *State, tower *Tower, destination Point, speed float64) float64 {
	dx, dy := destination.X-tower.X, destination.Y-tower.Y
	distance := vectorLength(dx, dy)
	step := min(distance, speed*battleStepSeconds(state))
	if step == distance {
		tower.X, tower.Y = destination.X, destination.Y
	} else {
		stepX, stepY := float64(dx/distance*step), float64(dy/distance*step)
		tower.X = math.Round((tower.X+stepX)*1e9) / 1e9
		tower.Y = math.Round((tower.Y+stepY)*1e9) / 1e9
	}
	return distance - step
}

func worldRebuildLinks(state *State, first *Tower) {
	needed := map[int]bool{}
	for _, second := range state.Towers {
		owner := pokemonFor(state, second.PokemonID)
		if owner == nil || first.ID == second.ID || WorldSpecies(state, owner).AbilityID != "minus" || vectorLength(first.X-second.X, first.Y-second.Y) > 2000 {
			continue
		}
		needed[second.ID] = true
		if !slices.ContainsFunc(state.World.Links, func(link WorldLink) bool { return link.First == first.ID && link.Second == second.ID }) {
			state.World.Links = append(state.World.Links, WorldLink{First: first.ID, Second: second.ID, HitAt: map[int]float64{}})
		}
	}
	state.World.Links = slices.DeleteFunc(state.World.Links, func(link WorldLink) bool { return link.First == first.ID && !needed[link.Second] })
}

func worldAdvanceLinks(state *State, milliseconds float64) error {
	for index := len(state.World.Links) - 1; index >= 0; index-- {
		link := &state.World.Links[index]
		var a, b *Tower
		for index := range state.Towers {
			tower := &state.Towers[index]
			if tower.ID == link.First {
				a = tower
			}
			if tower.ID == link.Second {
				b = tower
			}
		}
		if a == nil || b == nil {
			state.World.Links = slices.Delete(state.World.Links, index, index+1)
			continue
		}
		ownerA, ownerB := pokemonFor(state, a.PokemonID), pokemonFor(state, b.PokemonID)
		link.AgeMS += milliseconds
		for enemyID := range link.HitAt {
			if enemyFor(state, enemyID) == nil {
				delete(link.HitAt, enemyID)
			}
		}
		if !state.WaveActive || link.AgeMS < 1000 {
			continue
		}
		profileA, err := profileFor(state, a, ownerA, nil, 0.5)
		if err != nil {
			return err
		}
		profileB, err := profileFor(state, b, ownerB, nil, 0.5)
		if err != nil {
			return err
		}
		for index := range state.Enemies {
			enemy := &state.Enemies[index]
			radius := max(enemy.Sprite.FrameWidth, enemy.Sprite.FrameHeight) / 4
			if radius == 0 {
				radius = 3
			}
			if enemy.HP <= 0 || enemy.InvulnerableMS > 0 || enemy.Escaped || enemy.Invisible && !profileA.CanSeeInvisible || link.AgeMS-link.HitAt[enemy.ID] < 1000 || worldSegmentDistance(Point{X: enemy.X, Y: enemy.Y}, Point{X: a.X, Y: a.Y}, Point{X: b.X, Y: b.Y}) > 6+radius {
				continue
			}
			if err := worldDirectHit(state, b, ownerB, enemy, profileB, 1); err != nil {
				return err
			}
			if enemy.HP > 0 {
				if err := worldDirectHit(state, a, ownerA, enemy, profileA, 1); err != nil {
					return err
				}
			}
			link.HitAt[enemy.ID] = link.AgeMS
		}
	}
	return nil
}

func worldSegmentDistance(point, first, second Point) float64 {
	dx, dy := second.X-first.X, second.Y-first.Y
	length := dx*dx + dy*dy
	if length == 0 {
		return vectorLength(point.X-first.X, point.Y-first.Y)
	}
	ratio := max(0, min(1, ((point.X-first.X)*dx+(point.Y-first.Y)*dy)/length))
	return vectorLength(point.X-first.X-ratio*dx, point.Y-first.Y-ratio*dy)
}

func worldDirectHit(state *State, tower *Tower, owner *Pokemon, target *Enemy, profile AttackProfile, scale float64) error {
	projectile := Projectile{ID: allocateID(state), TowerID: tower.ID, PokemonID: owner.ID, TargetID: target.ID, X: target.X, Y: target.Y, Kind: "worldTick", Power: profile.Power, Context: contextFor(state, tower, owner, target, NextRandom(state)), Profile: profile, DamageScale: scale, HitIDs: []int{}}
	var err error
	projectile.SourceProfile, err = sourceProfileFor(state, tower, owner, target, projectile.Context.Random)
	if err != nil {
		return err
	}
	return applyProjectileHit(state, &projectile, target, 1)
}

func WorldHandlesAttack(state *State, tower *Tower, pokemon *Pokemon, profile AttackProfile) bool {
	return worldHasHook(profile, "idleRechargeCharges") || worldHasHook(profile, "idleRechargeChargesAndKillRefill") || worldHasHook(profile, "tetherTargetsInRange") || worldHasHook(profile, "countdownExplosion") || worldHasHook(profile, "pullToProjectile") || worldHasHook(profile, "windDisplacement") || worldHasHook(profile, "storeAmnesiaPower") || worldHasHook(profile, "followingNightmareZone")
}

func WorldAfterAttack(state *State, towerID int, profile AttackProfile, targetIDs []int) error {
	for index := range state.Towers {
		tower := &state.Towers[index]
		if tower.ID != towerID {
			continue
		}
		owner := pokemonFor(state, tower.PokemonID)
		if owner != nil && slices.Contains([]int{178, 238}, worldClassificationID(state, owner)) {
			for allyIndex := range state.Towers {
				ally := &state.Towers[allyIndex]
				if partner := pokemonFor(state, ally.PokemonID); partner != nil && WorldSpecies(state, partner).AbilityID == "aggressiveNature" {
					ally.CooldownMS = 0
				}
			}
		}
		if profile.ConsumeCharges > 0 && !worldHasHook(profile, "teleportDestinations") {
			tower.Charges = max(0, tower.Charges-profile.ConsumeCharges)
		}
	}
	return nil
}

func WorldOnHit(state *State, projectile *Projectile, enemy *Enemy, profile AttackProfile, lost float64) error {
	if err := worldDamageHealing(state, projectile.PokemonID, enemy, profile, projectile.Context, lost); err != nil {
		return err
	}
	if worldHasHook(profile, "randomControlCondition") && NextRandom(state) < .3 {
		choices := []ConditionSpec{burnCondition(), slowCondition(), status("stun", 1, 2000, 1, 1)}
		condition := choices[NextInt(state, len(choices))]
		condition.SourcePokemonID = projectile.PokemonID
		WorldApplyConditions(state, enemy, AttackProfile{Conditions: []ConditionSpec{condition}}, 0)
	}
	if worldHasHook(profile, "beamOverFiveLoseHeart") && !state.World.PaidBeamIDs[projectile.ID] {
		unique := map[int]bool{}
		for _, id := range projectile.HitIDs {
			unique[id] = true
		}
		unique[enemy.ID] = true
		if len(unique) > 5 {
			if state.World.PaidBeamIDs == nil {
				state.World.PaidBeamIDs = map[int]bool{}
			}
			state.World.PaidBeamIDs[projectile.ID] = true
			if state.Health > 1 {
				loss := min(state.Health-1, WorldOnHeartsLost(state, 1))
				state.Health -= loss
				state.WaveHeartsLost += loss
			}
		}
	}
	if tower := worldTowerFor(state, projectile.PokemonID); tower != nil {
		value := worldTower(state, tower)
		value.WaveDamage += lost
		if worldHasHook(profile, "teleportDestinations") {
			value.Teleports = 0
		}
		if worldHasHook(profile, "storeAmnesiaPower") {
			value.PowerStacks = 0
		}
		if worldHasHook(profile, "addHoneyOnAttack") {
			state.World.Honey++
		}
		if worldHasHook(profile, "chargeBoostMeter") && value.BoostMS == 0 {
			tower.Charges++
			if tower.Charges >= 50 {
				tower.Charges = 0
				value.BoostMS = 10000
			}
		}
		if worldHasHook(profile, "idleRechargeChargesAndKillRefill") && enemy.HP <= 0 {
			tower.Charges = 5
		}
		if profile.BerryAfterAttack {
			if owner := pokemonFor(state, tower.PokemonID); owner != nil {
				worldActivateBerry(state, tower, owner.Item)
			}
		}
		if projectile.Kind != "worldTick" && projectile.TargetID == enemy.ID {
			if worldHasHook(profile, "persistentWebZone") {
				worldCreateZone(state, "web", tower.PokemonID, Point{X: projectile.X, Y: projectile.Y}, 30, 4100, 500)
			}
			if worldHasHook(profile, "impactSkyArrows") {
				route, _ := routeForState(state)
				for index := 0; index < 3; index++ {
					angle, distance := NextRandom(state)*2*math.Pi, math.Sqrt(NextRandom(state))*75
					point := Point{X: max(16, min(float64(route.Width-16), enemy.X+math.Cos(angle)*distance)), Y: max(16, min(float64(route.Height-16), enemy.Y+math.Sin(angle)*distance))}
					zone := WorldZone{ID: allocateID(state), Kind: "skyArrow", PokemonID: tower.PokemonID, X: point.X, Y: point.Y, Radius: 36, LifeMS: int64(630 + index*90), DelayMS: int64(450 + index*90), HitAt: map[int]int64{}}
					state.World.Zones = append(state.World.Zones, zone)
					addEvent(state, Event{Type: "skyArrow", Source: tower.PokemonID, Target: zone.ID, X: zone.X, Y: zone.Y, Amount: float64(zone.DelayMS)})
				}
			}
			if worldHasHook(profile, "chance25Area") {
				if NextRandom(state) < value.Values["spikyChance"] {
					value.Values["spikyChance"] = 0
					period := int64(500)
					if owner := pokemonFor(state, tower.PokemonID); owner != nil && owner.Item == "rockyHelmetSpikes" {
						period = 250
					}
					worldCreateZone(state, "spikes", tower.PokemonID, Point{X: projectile.X, Y: projectile.Y}, 60, 2600, period)
				} else {
					value.Values["spikyChance"] += 0.25
				}
			}
		}
	}
	return nil
}

func WorldOnDeath(state *State, enemy *Enemy) error {
	delete(state.World.Displacements, enemy.ID)
	for _, value := range state.World.Towers {
		delete(value.Chains, enemy.ID)
	}
	if killer := pokemonFor(state, enemy.LastHitBy); killer != nil && slices.Contains([]string{"whiteRay", "autotomize"}, WorldSpecies(state, killer).AbilityID) {
		state.World.UltraBeastKills++
	}
	if !HasCondition(&enemy.EnemyEffectState, "burn") {
		return nil
	}
	var source *Tower
	var owner *Pokemon
	for index := range state.Towers {
		candidate := &state.Towers[index]
		partner := pokemonFor(state, candidate.PokemonID)
		if partner != nil && WorldSpecies(state, partner).AbilityID == "magmaStorm" {
			source, owner = candidate, partner
		}
	}
	if source == nil {
		return nil
	}
	amount := math.Ceil(enemy.MaxHP * 0.06)
	pending := []EnemyPassiveEvent{}
	for index := range state.Enemies {
		other := &state.Enemies[index]
		if other.ID == enemy.ID || other.HP <= 0 || other.Dead || other.Escaped || vectorLength(other.X-enemy.X, other.Y-enemy.Y) > 50 {
			continue
		}
		damageTrackedEnemy(state, other, owner.ID, amount, "direct", 0)
		other.LastHitCritical = false
		lost := other.LastHPDamage + other.LastArmorDamage + other.LastShieldDamage
		addEvent(state, damageEventFromEnemy(other, owner.ID, "physical", false))
		other.LastHitBy = owner.ID
		recordPokemonDamage(state, owner.ID, lost, other.LastResolvedDamage)
		if err := worldIndirectHealing(state, other, owner.ID, lost); err != nil {
			return err
		}
		if other.HP <= 0 {
			events, err := resolveEnemyDeath(state, other)
			if err != nil {
				return err
			}
			pending = append(pending, events...)
		}
		condition := burnCondition()
		condition.SourcePokemonID = owner.ID
		if owner.Item == "magmaStone" {
			condition.DurationMS = 20000
		} else if owner.Item == "falmeOrb" {
			condition.Magnitude = 0.0075
		}
		WorldApplyConditions(state, other, AttackProfile{Conditions: []ConditionSpec{condition}}, 0)
		addEvent(state, Event{Type: "magmaExplosion", Source: source.PokemonID, Target: other.ID, X: enemy.X, Y: enemy.Y, Amount: lost})
	}
	state.World.PendingEnemyEvents = append(state.World.PendingEnemyEvents, pending...)
	return nil
}

func WorldTakeEnemyEvents(state *State) []EnemyPassiveEvent {
	events := state.World.PendingEnemyEvents
	state.World.PendingEnemyEvents = nil
	return events
}

func WorldOnHeartsLost(state *State, amount int) int {
	worldEnsure(state)
	state.World.HitsReceived++
	if !state.World.IceBodyActivated && worldHasAbility(state, "iceBody") {
		state.World.IceBodyActivated = true
	}
	if !state.World.SturdyUsed && worldHasAbility(state, "sturdy") {
		state.World.SturdyUsed = true
		amount = max(0, amount-1)
	}
	if amount > 0 {
		for index := range state.Towers {
			tower := &state.Towers[index]
			if owner := pokemonFor(state, tower.PokemonID); owner != nil {
				worldActivateBerry(state, tower, owner.Item)
			}
		}
	}
	return amount
}

func WorldOnWaveStart(state *State) error {
	worldEnsure(state)
	state.World.EnemyWeather = ""
	state.World.AppliedWeather = state.Weather
	for _, enemy := range state.Enemies {
		if enemy.PassiveID == "sunnyDay" {
			state.World.EnemyWeather = "harshSunlight"
		}
	}
	state.World.MovementLocked, state.World.HeavyMetalUsed, state.World.SturdyUsed = false, false, false
	state.World.IceBodyActivated, state.World.ShellSmashActive, state.World.HitsReceived, state.World.WaveElapsedMS = false, false, 0, 0
	state.World.Honey = 0
	state.World.HealingUses, state.World.HealingDamage, state.World.HealingItemUsed = map[int]float64{}, map[int]float64{}, map[string]bool{}
	state.World.HealScale, state.World.RoomServicePower, state.World.PendingHeal = 0, 0, 0
	state.World.UltraBeastKills, state.World.AnchorShotTriggered, state.World.Pressure = 0, false, false
	for _, value := range state.World.Towers {
		value.Honey, value.WaveDamage = 0, 0
		value.ExposureMS, value.ExposureApplied, value.Chains = map[int]int64{}, map[int]bool{}, map[int]int64{}
	}
	if state.Health > 1 && worldHasAbility(state, "shellSmash") {
		loss := WorldOnHeartsLost(state, 1)
		state.Health -= loss
		state.WaveHeartsLost += loss
		state.World.ShellSmashActive = true
	}
	if worldHasAbility(state, "gluttony") {
		for index := range state.Towers {
			tower := &state.Towers[index]
			if owner := pokemonFor(state, tower.PokemonID); owner != nil {
				worldActivateBerry(state, tower, owner.Item)
			}
		}
	}
	return nil
}

func worldActivateBerry(state *State, tower *Tower, item string) {
	if !slices.Contains([]string{"salacBerry", "liechiBerry", "lansatBerry", "starfBerry"}, item) {
		return
	}
	duration := int64(18000)
	if worldHasAbility(state, "harvest") {
		duration *= 2
	}
	worldTower(state, tower).BerryTimers[item] = duration
	if host := worldTowerFor(state, state.World.PassengerHosts[tower.PokemonID]); host != nil {
		if owner := pokemonFor(state, host.PokemonID); owner != nil && owner.Item == "berryPouch" {
			worldTower(state, host).BerryTimers[item] = duration
		}
	}
}

func worldAdjustBerries(state *State, tower *Tower, owner *Pokemon, target *Enemy, profile *AttackProfile) {
	value := state.World.Towers[owner.ID]
	if value == nil {
		return
	}
	ability := WorldSpecies(state, owner).AbilityID
	context := contextFor(state, tower, owner, target, 0.5)
	context.BerryActive = false
	base, err := BuildAttackProfile(context)
	if err != nil {
		return
	}
	unnerve := worldHasAbility(state, "unnerve")
	for _, berry := range []string{"salacBerry", "liechiBerry", "lansatBerry", "starfBerry"} {
		if value.BerryTimers[berry] <= 0 || owner.Item != berry && owner.Item != "berryPouch" {
			continue
		}
		scale := float64(1)
		if ability == "ripen" {
			scale = 2
		} else if ability == "simple" {
			scale = 1.75
		}
		if unnerve {
			scale *= 1.1
		}
		switch berry {
		case "salacBerry":
			factor := 0.5
			if ability == "ripen" {
				factor = 0.25
			} else if ability == "simple" {
				factor = 0.375
			}
			if unnerve {
				factor *= 0.9
			}
			profile.AttackMS = base.AttackMS * factor
		case "liechiBerry":
			profile.Power = base.Power + math.Round(200*scale)
		case "lansatBerry":
			profile.CriticalPct = base.CriticalPct + math.Round(60*scale)
		case "starfBerry":
			profile.Range = base.Range + math.Round(80*scale)
		}
	}
}

func worldAdvanceChains(state *State, tower *Tower, owner *Pokemon, value *WorldTowerState, profile AttackProfile, targets []*Enemy) error {
	for _, target := range targets {
		if len(value.Chains) >= 6 {
			break
		}
		if _, exists := value.Chains[target.ID]; !exists {
			value.Chains[target.ID] = 0
			addEvent(state, Event{Type: "chain", Source: owner.ID, Target: target.ID, X: tower.X, Y: tower.Y})
		}
	}
	ids := make([]int, 0, len(value.Chains))
	for id := range value.Chains {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		enemy := enemyFor(state, id)
		extra := float64(40)
		if enemy != nil && HasCondition(&enemy.EnemyEffectState, "curse") {
			extra = 100
		}
		if enemy == nil || enemy.HP <= 0 || enemy.Escaped || vectorLength(enemy.X-tower.X, enemy.Y-tower.Y) >= profile.Range+extra {
			delete(value.Chains, id)
			continue
		}
		value.Chains[id] += battleStepWholeMS(state)
		if value.Chains[id] >= 500 {
			value.Chains[id] -= 500
			current, err := profileFor(state, tower, owner, enemy, 0.5)
			if err != nil {
				return err
			}
			if err := worldDirectHit(state, tower, owner, enemy, current, 1); err != nil {
				return err
			}
		}
	}
	return nil
}

func worldAdvanceExplosion(state *State, tower *Tower, owner *Pokemon, value *WorldTowerState, profile AttackProfile, targets []*Enemy) error {
	if value.Timers["explosionCooldown"] > 0 {
		value.Timers["explosionCooldown"] -= battleStepWholeMS(state)
		return nil
	}
	remaining := value.Timers["explosionRemaining"]
	if remaining <= 0 {
		remaining = 3000
	}
	remaining -= battleStepWholeMS(state)
	if len(targets) < 5 {
		remaining = max(1000, remaining)
	}
	value.Timers["explosionRemaining"] = remaining
	if remaining > 0 {
		return nil
	}
	for _, enemy := range targets {
		if err := worldDirectHit(state, tower, owner, enemy, profile, 1); err != nil {
			return err
		}
	}
	if !worldHasAbility(state, "damp") {
		loss := WorldOnHeartsLost(state, 2)
		state.Health = max(0, state.Health-loss)
		state.WaveHeartsLost += loss
	}
	value.Timers["explosionCooldown"] = int64(profile.AttackMS)
	value.Timers["explosionRemaining"] = 3000
	addEvent(state, Event{Type: "explosion", Source: owner.ID, X: tower.X, Y: tower.Y, Amount: max(40, profile.Range)})
	if state.Health == 0 {
		state.Phase, state.WaveActive = "defeat", false
	}
	return nil
}

func worldCreateZone(state *State, kind string, pokemonID int, point Point, radius float64, lifeMS, tickMS int64) {
	for _, existing := range state.World.Zones {
		if existing.Kind == kind && existing.PokemonID == pokemonID && existing.AgeMS <= 250 && vectorLength(existing.X-point.X, existing.Y-point.Y) <= 32 {
			return
		}
	}
	zone := WorldZone{ID: allocateID(state), Kind: kind, PokemonID: pokemonID, X: point.X, Y: point.Y, Radius: radius, LifeMS: lifeMS, TickMS: tickMS, HitAt: map[int]int64{}}
	state.World.Zones = append(state.World.Zones, zone)
	addEvent(state, Event{Type: "zone", Source: pokemonID, Target: zone.ID, X: point.X, Y: point.Y, Amount: radius, Key: kind})
}

func worldAdvanceZones(state *State) error {
	kept := []WorldZone{}
	for _, zone := range state.World.Zones {
		if zone.PersistentArea {
			kept = append(kept, zone)
			continue
		}
		zone.AgeMS += battleStepWholeMS(state)
		tower := worldTowerFor(state, zone.PokemonID)
		owner := pokemonFor(state, zone.PokemonID)
		if zone.AgeMS >= zone.LifeMS || tower == nil || owner == nil {
			continue
		}
		if zone.Kind == "tornado" || zone.Kind == "singularity" || zone.Kind == "toyProjectile" || zone.Kind == "shadowTag" {
			alive, err := worldAdvanceMovingZone(state, &zone, tower, owner)
			if err != nil {
				return err
			}
			if alive {
				kept = append(kept, zone)
			}
			continue
		}
		for index := range state.Enemies {
			enemy := &state.Enemies[index]
			radius := zone.Radius
			if zone.Kind == "skyArrow" {
				radius += max(enemy.Sprite.FrameWidth, enemy.Sprite.FrameHeight) / 2
			}
			if !state.WaveActive || !enemy.Entered || enemy.HP <= 0 || enemy.Escaped || enemy.InvulnerableMS > 0 || vectorLength(enemy.X-zone.X, enemy.Y-zone.Y) > radius {
				continue
			}
			profile, err := profileFor(state, tower, owner, enemy, 0.5)
			if err != nil {
				return err
			}
			if enemy.Invisible && !profile.CanSeeInvisible {
				continue
			}
			if zone.Kind == "web" {
				WorldApplyConditions(state, enemy, AttackProfile{Conditions: []ConditionSpec{status("slow", 1, battleStepWholeMS(state)*2, 0.5, 1)}}, 0)
			}
			tick := zone.TickMS > 0 && zone.AgeMS/zone.TickMS > (zone.AgeMS-battleStepWholeMS(state))/zone.TickMS
			if zone.Kind == "skyArrow" {
				tick = zone.AgeMS >= zone.DelayMS && zone.AgeMS-battleStepWholeMS(state) < zone.DelayMS
			}
			if tick {
				if err := worldDirectHit(state, tower, owner, enemy, profile, 1); err != nil {
					return err
				}
			}
		}
		kept = append(kept, zone)
	}
	state.World.Zones = kept
	return nil
}

func WorldOnWaveEnd(state *State) error {
	for _, tower := range state.Towers {
		owner := pokemonFor(state, tower.PokemonID)
		if owner != nil && WorldSpecies(state, owner).AbilityID == "triage" && NextRandom(state) < 0.05 {
			before := state.Health
			state.Health = min(state.MaxHealth, state.Health+1)
			state.WaveHeartsHealed += state.Health - before
		}
	}
	if state.World.PendingHeal > 0 {
		before := state.Health
		state.Health = min(state.MaxHealth, state.Health+state.World.PendingHeal)
		state.WaveHeartsHealed += state.Health - before
		state.World.PendingHeal = 0
	}
	state.World.MovementLocked = false
	state.World.Pressure, state.World.AnchorShotTriggered = false, false
	for pokemonID, itemID := range state.World.StrippedItems {
		owner := ownedPokemonFor(state, pokemonID)
		if owner == nil || owner.Item != "" {
			continue
		}
		for index := range state.Inventory {
			item := &state.Inventory[index]
			if item.ID == itemID && item.EquippedBy == 0 {
				owner.Item, item.EquippedBy = itemID, pokemonID
			}
		}
	}
	state.World.StrippedItems = nil
	return nil
}

func WorldApplyConditions(state *State, enemy *Enemy, profile AttackProfile, roll float64) {
	for index := range state.Towers {
		tower := &state.Towers[index]
		owner := pokemonFor(state, tower.PokemonID)
		if owner != nil && WorldSpecies(state, owner).AbilityID == "spiralDrain" && vectorLength(tower.X-enemy.X, tower.Y-enemy.Y) <= 120 {
			profile.Conditions = append([]ConditionSpec{}, profile.Conditions...)
			for conditionIndex := range profile.Conditions {
				condition := &profile.Conditions[conditionIndex]
				if condition.Kind == "slow" || condition.Kind == "auraSlow" || condition.Kind == "shadowTagSlow" {
					condition.Magnitude = min(1, condition.Magnitude*1.2)
				}
			}
			break
		}
	}
	before := enemy.StunApplications
	previousConditions := trainerConditionCounts(enemy)
	ApplyConditions(&enemy.EnemyEffectState, profile, roll)
	recordTrainerConditions(state, previousConditions, enemy)
	if enemy.StunApplications > before {
		WorldOnConditionApplied(state, enemy, "stun")
	}
}

func WorldOnConditionApplied(state *State, enemy *Enemy, kind string) {
	if kind != "stun" {
		return
	}
	for index := range state.Towers {
		tower := &state.Towers[index]
		owner := pokemonFor(state, tower.PokemonID)
		if owner == nil || WorldSpecies(state, owner).AbilityID != "lightningRod" || tower.Charges >= 10 {
			continue
		}
		profile, err := profileFor(state, tower, owner, enemy, 0.5)
		if err == nil && vectorLength(tower.X-enemy.X, tower.Y-enemy.Y) <= profile.Range {
			tower.Charges++
			return
		}
	}
}

func SupportedWorldHooks() []string {
	return []string{
		"addHoneyOnAttack", "allBurnUnitsSun25", "allyIdleTarget", "attackWithFutureParadoxAlly", "beamOverFiveLoseHeart",
		"brutalFauna", "burnDeathExplosion", "chance25Area", "chargeBoostMeter", "chargeOnNearbyStun", "commanderStats",
		"continuousRangeConditions", "continuousRangeExposure", "copyFirstSpeciesAtOwnLevel", "countdownExplosion", "crossShapeWidth",
		"deployGlobalStunAndMovementLock", "endWaveHeartChance", "erraticStatic", "finalEvolutionShiny", "firstHeartLossDoubleHail",
		"firstWaveHeartLoss", "flyingTowerMovement", "followingNightmareZone", "globalBerryDuration", "globalBerryEffect",
		"globalBerryWaveStart", "halfDamageExecuteAura", "idleRechargeCharges", "idleRechargeChargesAndKillRefill", "ignoreItemPowerPenalty",
		"impactSkyArrows", "justiceAllies", "onlyUnheldAllies", "oppositeChargeLink", "passengerPlacement", "persistentWebZone",
		"preventEnemyWeatherBonuses", "preventExplosionHeartCost", "pullToProjectile", "ragingBlossom", "randomControlCondition",
		"rangeSandDamage50", "sereneWaves", "spreadPrimaryConditions", "spreadNightmare", "teleportDestinations", "tetherTargetsInRange",
		"virulentReef", "waveStartHeartCost", "windDisplacement", "restoreInvertedHeartsAtWaveEnd", "waveDamageHealing", "globalRoomServicePower",
	}
}

func WorldCrossWidth(state *State) float64 {
	if worldHasAbility(state, "victoryStar") {
		return 1.5
	}
	return 1
}

func WorldFinalizeDamage(state *State, enemy *Enemy, context AbilityContext, amount float64) float64 {
	active := map[string]bool{}
	for index := range state.Towers {
		tower := &state.Towers[index]
		owner := pokemonFor(state, tower.PokemonID)
		if owner != nil && vectorLength(enemy.X-tower.X, enemy.Y-tower.Y) <= 120 {
			active[WorldSpecies(state, owner).AbilityID] = true
		}
	}
	if active["naturesJudgement"] && enemy.Armor <= 0 {
		amount = math.Ceil(amount * 1.2)
	}
	if active["machBolt"] && HasCondition(&enemy.EnemyEffectState, "stun") {
		amount = math.Ceil(amount * 1.2)
	}
	if active["charmedCharm"] && slices.Contains([]string{"heal", "bitterBlade", "takeHeart", "seedFlare", "soulHeartReactor"}, context.AbilityID) {
		amount = math.Ceil(amount * 1.2)
	}
	if enemy.HP < enemy.MaxHP*0.5 {
		switch context.AbilityID {
		case "hyperCutter":
			amount += math.Ceil(amount * 0.5)
		case "destructiveMaw":
			amount += math.Ceil(amount)
		}
	}
	if isSun(state.Weather) && HasCondition(&enemy.EnemyEffectState, "burn") && worldHasAbility(state, "solarPower") {
		base, err := BuildAttackProfile(AbilityContext{AbilityID: context.AbilityID, Power: 1, Range: 1, AttackMS: 1000, Random: 0.5})
		if err == nil {
			for _, condition := range base.Conditions {
				if condition.Kind == "burn" {
					amount = math.Ceil(amount * 1.25)
					break
				}
			}
		}
	}
	return amount
}

func WorldEnemyEvent(state *State, event EnemyPassiveEvent) error {
	switch event.Kind {
	case "anchorShot":
		state.World.AnchorShotTriggered = true
	case "pressure":
		state.World.Pressure, state.World.MovementLocked = true, true
	case "stripItem":
		pokemonID := event.SourcePokemonID
		if enemy := enemyFor(state, event.EnemyID); enemy != nil && pokemonID == 0 {
			pokemonID = enemy.LastDamageSourceID
			if pokemonID == 0 {
				pokemonID = enemy.LastHitBy
			}
		}
		owner := pokemonFor(state, pokemonID)
		if owner == nil || owner.Item == "" || WorldSpecies(state, owner).AbilityID == "scrappy" {
			return nil
		}
		if item, ok := CatalogData().Items[owner.Item]; !ok || !item.Retirable {
			return nil
		}
		if state.World.StrippedItems == nil {
			state.World.StrippedItems = map[int]string{}
		}
		state.World.StrippedItems[owner.ID] = owner.Item
		for index := range state.Inventory {
			if state.Inventory[index].ID == owner.Item && state.Inventory[index].EquippedBy == owner.ID {
				state.Inventory[index].EquippedBy = 0
			}
		}
		owner.Item = ""
	default:
		return fmt.Errorf("unknown world enemy event %q", event.Kind)
	}
	return nil
}

func ValidateWorld(state *State) error {
	if state.World.HealScale < 0 || state.World.RoomServicePower < 0 || state.World.PendingHeal < 0 || state.World.PendingHeal > 10000 || len(state.World.HealingUses) > 1024 || len(state.World.HealingDamage) > 1024 || len(state.World.HealingItemUsed) > 3 {
		return errors.New("治疗计数或挂起回血无效")
	}
	for id, uses := range state.World.HealingUses {
		if id < 1 || id >= state.NextID || !finiteNumber(uses) || uses < 0 || uses > 3 || uses*2 != math.Trunc(uses*2) {
			return errors.New("伙伴治疗次数无效")
		}
	}
	for id, damage := range state.World.HealingDamage {
		if id < 1 || id >= state.NextID || !finiteNumber(damage) || damage < 0 {
			return errors.New("本波治疗伤害计数无效")
		}
	}
	for key := range state.World.HealingItemUsed {
		if !slices.Contains([]string{"leftovers", "shellBell", "clefairyDoll"}, key) {
			return errors.New("治疗道具领取标识无效")
		}
	}
	world := &state.World
	if len(world.Towers) > 1024 || len(world.PassengerHosts) > 10 || len(world.Links) > 100 || len(world.Zones) > 8192 || world.HitsReceived < 0 || world.Honey < 0 || world.UltraBeastKills < 0 || world.WaveElapsedMS < 0 || world.ChessRecomputeAt < 0 || len(world.ChessActive) > 16 {
		return errors.New("世界能力状态无效")
	}
	for id, value := range world.Towers {
		if value != nil && value.OrbitPosition != nil && (!finiteNumber(value.OrbitPosition.X) || !finiteNumber(value.OrbitPosition.Y)) {
			return errors.New("盘旋塔位置无效")
		}
		if id < 1 || id >= state.NextID || ownedPokemonFor(state, id) == nil || value == nil || !finiteNumber(value.Origin.X) || !finiteNumber(value.Origin.Y) || !finiteNumber(value.Destination.X) || !finiteNumber(value.Destination.Y) || !finiteNumber(value.Angle) || !finiteNumber(value.IdleMS) || !finiteNumber(value.ChargeMS) || !finiteNumber(value.BurstMS) || !finiteNumber(value.WaveDamage) || value.IdleMS < 0 || value.ChargeMS < 0 || value.BurstMS < 0 || value.BurstRemaining < 0 || value.BurstRemaining > 10 || value.Teleports < 0 || value.PowerStacks < 0 || value.BoostMS < 0 || value.BoostMS > 10000 || len(value.Chains) > 6 || len(value.ExposureMS) > 4096 || len(value.ExposureApplied) > 4096 {
			return errors.New("伙伴世界能力状态无效")
		}
		for _, duration := range value.BerryTimers {
			if duration < 0 || duration > 36000 {
				return errors.New("树果计时无效")
			}
		}
		for _, number := range value.Values {
			if !finiteNumber(number) {
				return errors.New("世界能力数值无效")
			}
		}
		for targetID, duration := range value.ExposureMS {
			if targetID < 1 || targetID >= state.NextID || duration < 0 || duration > 600000 {
				return errors.New("持续暴露状态无效")
			}
		}
		for targetID := range value.ExposureApplied {
			if targetID < 1 || targetID >= state.NextID {
				return errors.New("持续暴露目标无效")
			}
		}
		for targetID, duration := range value.Chains {
			if targetID < 1 || targetID >= state.NextID || duration < 0 || duration >= 500 {
				return errors.New("锚链计时无效")
			}
		}
	}
	towerIDs := map[int]bool{}
	for _, tower := range state.Towers {
		if tower.ID < 1 || tower.ID >= state.NextID || towerIDs[tower.ID] || pokemonFor(state, tower.PokemonID) == nil || !finiteNumber(tower.X) || !finiteNumber(tower.Y) || !finiteNumber(tower.CooldownMS) || tower.Charges < 0 || tower.Charges > 50 || tower.StoredShots < 0 || tower.IdleMS < 0 || tower.DeployedMS < 0 {
			return errors.New("部署塔能力状态无效")
		}
		towerIDs[tower.ID] = true
		for _, number := range tower.Stacks {
			if !finiteNumber(number) {
				return errors.New("部署塔叠层无效")
			}
		}
	}
	for passenger, host := range world.PassengerHosts {
		p, h := worldTowerFor(state, passenger), worldTowerFor(state, host)
		if passenger == host || p == nil || h == nil || p.TileID != h.TileID || world.PassengerHosts[host] != 0 {
			return errors.New("搭乘关系无效")
		}
	}
	for _, link := range world.Links {
		if !towerIDs[link.First] || !towerIDs[link.Second] || link.First == link.Second || !finiteNumber(link.AgeMS) || link.AgeMS < 0 || len(link.HitAt) > 4096 {
			return errors.New("电荷连接无效")
		}
		for targetID, at := range link.HitAt {
			if targetID < 1 || targetID >= state.NextID || !finiteNumber(at) || at < 0 || at > link.AgeMS {
				return errors.New("电荷命中记录无效")
			}
		}
	}
	for _, zone := range world.Zones {
		if zone.ID <= 0 || zone.ID >= state.NextID || ownedPokemonFor(state, zone.PokemonID) == nil || !finiteNumber(zone.X) || !finiteNumber(zone.Y) || !finiteNumber(zone.Radius) || !finiteNumber(zone.InnerRadius) || zone.InnerRadius < 0 || zone.InnerRadius > zone.Radius || !finiteNumber(zone.TargetX) || !finiteNumber(zone.TargetY) || !finiteNumber(zone.Origin.X) || !finiteNumber(zone.Origin.Y) || !finiteNumber(zone.Direction.X) || !finiteNumber(zone.Direction.Y) || !finiteNumber(zone.Speed) || !finiteNumber(zone.Distance) || !finiteNumber(zone.Angle) || zone.Radius < 0 || zone.AgeMS < 0 || zone.LifeMS <= 0 || zone.LifeMS > 60000 || zone.TickMS < 0 || zone.DelayMS < 0 || zone.DelayMS >= zone.LifeMS || !slices.Contains([]string{"web", "spikes", "magmaSpikes", "smoke", "skyArrow", "singularity", "tornado", "toyProjectile", "shadowTag"}, zone.Kind) {
			return errors.New("持续场域无效")
		}
		if zone.PersistentArea && zone.Smoke {
			if zone.Kind != "smoke" || !finiteNumber(zone.SlowFactor) || zone.SlowFactor <= 0 || zone.SlowFactor > 1 {
				return errors.New("持续烟雾范围或减速无效")
			}
		} else if zone.PersistentArea {
			if zone.SourceTower == nil || zone.SourceTower.PokemonID != zone.PokemonID || zone.SourceTower.ID < 1 || zone.SourceTower.ID >= state.NextID || !finiteNumber(zone.SourceTower.X) || !finiteNumber(zone.SourceTower.Y) || zone.TickMS <= 0 || !slices.Contains([]string{"spikes", "magmaSpikes"}, zone.Kind) || zone.Magma != (zone.Kind == "magmaSpikes") {
				return errors.New("持续尖刺来源或计时无效")
			}
			for _, value := range zone.SourceTower.Stacks {
				if !finiteNumber(value) {
					return errors.New("持续尖刺来源叠层无效")
				}
			}
		}
	}
	for enemyID, displacement := range world.Displacements {
		if enemyID < 1 || enemyID >= state.NextID || !finiteNumber(displacement.Direction.X) || !finiteNumber(displacement.Direction.Y) || !finiteNumber(displacement.Remaining) || displacement.Remaining < 0 || displacement.Remaining > 15 {
			return errors.New("敌人位移状态无效")
		}
	}
	return nil
}

func worldUpdateWeather(state *State) error {
	route, _ := routeForState(state)
	if state.Weather != state.World.AppliedWeather {
		state.World.EnemyWeather = state.Weather
	}
	weather := route.WeatherID
	if state.World.EnemyWeather != "" {
		weather = state.World.EnemyWeather
	}
	forced := route.WeatherID != ""
	thunder, avalanche, pixilate := false, false, false
	for index := range state.Towers {
		tower := &state.Towers[index]
		owner := pokemonFor(state, tower.PokemonID)
		if owner == nil {
			continue
		}
		ability := WorldSpecies(state, owner).AbilityID
		thunder = thunder || ability == "wildboltStorm"
		avalanche = avalanche || ability == "sandsearStorm"
		pixilate = pixilate || ability == "pixilate"
		if forced {
			continue
		}
		if slices.Contains([]string{"drizzle", "berserkMega"}, ability) || owner.Item == "dampRock" {
			weather = "rain"
		}
		if slices.Contains([]string{"sandStream", "sandStreamTyranitar", "sandStreamMegaTyranitar"}, ability) || owner.Item == "smoothRock" {
			weather = "sandstorm"
		}
		if ability == "snowWarning" || owner.Item == "icyRock" {
			weather = "hail"
		}
		if slices.Contains([]string{"drought", "burnNerf"}, ability) || owner.Item == "heatRockWeather" {
			weather = "harshSunlight"
		}
		if ability == "bleakwindStorm" {
			weather = "strongWinds"
		}
	}
	if thunder && (weather == "rain" || weather == "heavyRain") {
		weather = "thunderStorm"
	}
	if avalanche && weather == "sandstorm" {
		weather = "sandAvalanche"
	}
	if pixilate && weather != "" {
		weather = "starryNight"
	}
	state.Weather, state.World.AppliedWeather = weather, weather
	cloudNine := worldHasAbility(state, "cloudNine")
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		enemy.CloudNine = cloudNine && !enemy.Boss
		enemy.WorldSandMultiplier = 1
		if err := SetEnemyWeather(&enemy.EnemyEffectState, weather, false); err != nil {
			return err
		}
		for towerIndex := range state.Towers {
			tower := &state.Towers[towerIndex]
			owner := pokemonFor(state, tower.PokemonID)
			if owner == nil {
				continue
			}
			ability := WorldSpecies(state, owner).AbilityID
			if slices.Contains([]string{"sandStreamTyranitar", "sandStreamMegaTyranitar"}, ability) {
				profile, err := profileFor(state, tower, owner, enemy, 0.5)
				if err != nil {
					return err
				}
				if vectorLength(tower.X-enemy.X, tower.Y-enemy.Y) <= profile.Range {
					enemy.WorldSandMultiplier *= 1.5
				}
			}
		}
	}
	return nil
}

func WorldOnEnemyHeal(state *State, enemy *Enemy) {
	if enemy == nil || enemy.HP <= 0 || enemy.Escaped || !enemy.CanPoison || enemy.InvulnerableMS > 0 {
		return
	}
	for index := range state.Towers {
		tower := &state.Towers[index]
		owner := pokemonFor(state, tower.PokemonID)
		if owner != nil && WorldSpecies(state, owner).AbilityID == "corrosionMega" && vectorLength(tower.X-enemy.X, tower.Y-enemy.Y) <= 120 {
			condition := poisonCondition(1)
			condition.SourcePokemonID = owner.ID
			WorldApplyConditions(state, enemy, AttackProfile{Conditions: []ConditionSpec{condition}}, 0)
			return
		}
	}
}

func WorldOnRouteChange(state *State) {
	state.World = WorldState{}
	worldEnsure(state)
}

func worldClassificationID(state *State, pokemon *Pokemon) int {
	if pokemon.Species == "ditto" {
		if copied, ok := CatalogData().Species[state.Progression.DNA[pokemon.ID]]; ok {
			return copied.ID
		}
	}
	return WorldSpecies(state, pokemon).ID
}

func worldAdvanceSpecialAttack(state *State, tower *Tower, owner *Pokemon, value *WorldTowerState, profile AttackProfile, targets []*Enemy) {
	value.Timers["specialCooldown"] = max(0, value.Timers["specialCooldown"]-battleStepWholeMS(state))
	if value.Timers["specialCooldown"] > 0 || len(targets) == 0 {
		return
	}
	target := targets[0]
	value.Timers["specialCooldown"] = int64(profile.AttackMS)
	if worldHasHook(profile, "storeAmnesiaPower") {
		if owner.Item == "laggingTail" || NextRandom(state) < 0.5 {
			value.PowerStacks++
			addEvent(state, Event{Type: "amnesia", Source: owner.ID, Amount: float64(value.PowerStacks)})
			return
		}
		worldLaunch(state, tower, owner, target, profile)
		return
	}
	species := WorldSpecies(state, owner)
	zone := WorldZone{ID: allocateID(state), PokemonID: owner.ID, X: tower.X, Y: tower.Y, Origin: Point{X: tower.X, Y: tower.Y}, TargetID: target.ID, TargetX: target.X, TargetY: target.Y, Speed: max(60, species.ProjectileSpeed*60), LifeMS: 30000, HitAt: map[int]int64{}}
	if worldHasHook(profile, "pullToProjectile") {
		zone.Kind, zone.Phase, zone.Radius = "singularity", "travel", 90
	} else if worldHasHook(profile, "followingNightmareZone") {
		zone.Kind, zone.Phase, zone.Radius = "shadowTag", "travel", 60
	} else {
		zone.Kind, zone.Radius, zone.Angle = "tornado", 6, NextRandom(state)*math.Pi*2
		dx, dy := target.X-tower.X, target.Y-tower.Y
		length := vectorLength(dx, dy)
		if length == 0 {
			dx, length = 1, 1
		}
		zone.Direction = Point{X: dx / length, Y: dy / length}
	}
	state.World.Zones = append(state.World.Zones, zone)
	addEvent(state, Event{Type: "fire", Source: tower.ID, Target: target.ID, X: tower.X, Y: tower.Y, Key: species.ProjectileID})
}

func worldAdvanceDisplacements(state *State) {
	for id, displacement := range state.World.Displacements {
		enemy := enemyFor(state, id)
		if enemy == nil || enemy.HP <= 0 || enemy.Escaped || displacement.Remaining <= 0 {
			delete(state.World.Displacements, id)
			continue
		}
		step := min(displacement.Remaining, 50*battleStepSeconds(state))
		enemy.X = rounded(enemy.X + displacement.Direction.X*step)
		enemy.Y = rounded(enemy.Y + displacement.Direction.Y*step)
		displacement.Remaining -= step
		if displacement.Remaining <= 0 {
			delete(state.World.Displacements, id)
		} else {
			state.World.Displacements[id] = displacement
		}
	}
}

func worldUpdateChess(state *State) {
	if state.Route != 35 {
		state.World.ChessActive = nil
		return
	}
	if state.World.ChessRecomputeAt <= 0 || state.Tick < state.World.ChessRecomputeAt {
		return
	}
	rules := map[string]map[int][]string{
		"kingdra":   {17: {"h1", "h2"}, 23: {"h1"}, 14: {"h1"}, 26: {"h1"}, 16: {"h2"}},
		"vespiquen": {10: {"q1"}, 13: {"q1"}},
		"pawniard":  {19: {"p4"}, 23: {"p6"}, 25: {"p6", "p7"}},
		"bisharp":   {0: {"b1"}, 8: {"b1"}, 18: {"b1"}, 21: {"b1"}, 9: {"b2"}, 14: {"b2"}, 26: {"b1", "b2"}, 30: {"b1"}, 2: {"b1", "b2"}},
		"kingambit": {24: {"k1"}, 25: {"k1"}},
		"stakataka": {8: {"t1"}, 9: {"t1"}, 5: {"t2"}, 14: {"t2"}, 19: {"t2"}, 21: {"t2"}, 23: {"t2"}, 24: {"t2"}, 25: {"t2"}, 26: {"t2"}, 10: {"t1", "t2"}},
	}
	active := map[string]bool{}
	for index := range state.Towers {
		tower := &state.Towers[index]
		owner := pokemonFor(state, tower.PokemonID)
		if owner == nil {
			continue
		}
		key := owner.Species
		if key == "ditto" {
			key = state.Progression.DNA[owner.ID]
		}
		for _, piece := range rules[key][tower.TileID] {
			active[piece] = true
		}
	}
	state.World.ChessActive = []string{}
	for piece := range active {
		state.World.ChessActive = append(state.World.ChessActive, piece)
	}
	slices.Sort(state.World.ChessActive)
	state.World.ChessRecomputeAt = 0
}

func worldAdvanceMovingZone(state *State, zone *WorldZone, tower *Tower, owner *Pokemon) (bool, error) {
	if zone.Kind == "toyProjectile" {
		target := worldTowerFor(state, zone.TargetID)
		if target == nil {
			return false, nil
		}
		if zone.AgeMS < zone.DelayMS {
			return true, nil
		}
		dx, dy := target.X-zone.X, target.Y-zone.Y
		distance := vectorLength(dx, dy)
		step := zone.Speed * battleStepSeconds(state)
		if distance > max(4, step) {
			zone.X, zone.Y = rounded(zone.X+dx/distance*step), rounded(zone.Y+dy/distance*step)
			return true, nil
		}
		profile, err := profileFor(state, tower, owner, nil, 0.5)
		if err != nil {
			return false, err
		}
		if worldHasHook(profile, "addHoneyOnAttack") {
			state.World.Honey++
		}
		addEvent(state, Event{Type: "toyHit", Source: owner.ID, Target: target.PokemonID, X: target.X, Y: target.Y})
		return false, nil
	}
	if zone.Kind == "tornado" {
		return worldAdvanceTornado(state, zone, tower, owner)
	}
	if zone.Phase == "travel" {
		if enemy := enemyFor(state, zone.TargetID); enemy != nil && enemy.HP > 0 && !enemy.Escaped {
			zone.TargetX, zone.TargetY = enemy.X, enemy.Y
		}
		dx, dy := zone.TargetX-zone.X, zone.TargetY-zone.Y
		distance := vectorLength(dx, dy)
		step := zone.Speed * battleStepSeconds(state)
		if distance <= max(6, step) {
			zone.X, zone.Y, zone.Phase, zone.AgeMS = zone.TargetX, zone.TargetY, "waiting", 0
			if zone.Kind == "shadowTag" {
				zone.Phase = "following"
			}
		} else {
			zone.X, zone.Y = rounded(zone.X+dx/distance*step), rounded(zone.Y+dy/distance*step)
		}
		return true, nil
	}
	if zone.Kind == "shadowTag" {
		return worldAdvanceShadowTag(state, zone, tower, owner)
	}
	if zone.Phase == "waiting" {
		if zone.AgeMS >= 1000 {
			zone.Phase, zone.AgeMS = "pulling", 0
		}
		return true, nil
	}
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if enemy.HP <= 0 || enemy.Dead || enemy.Escaped || !enemy.Entered || enemy.InvulnerableMS > 0 {
			continue
		}
		profile, err := profileFor(state, tower, owner, enemy, 0.5)
		if err != nil {
			return false, err
		}
		if enemy.Invisible && !profile.CanSeeInvisible {
			continue
		}
		dx, dy := enemy.X-zone.X, enemy.Y-zone.Y
		distance := vectorLength(dx, dy)
		if distance > zone.Radius {
			continue
		}
		if enemy.CanStun && distance > 2 {
			step := min(distance, 220*battleStepSeconds(state))
			enemy.X, enemy.Y = rounded(enemy.X-dx/distance*step), rounded(enemy.Y-dy/distance*step)
		}
		if zone.AgeMS >= 450 {
			if err := worldDirectHit(state, tower, owner, enemy, profile, 1); err != nil {
				return false, err
			}
		}
	}
	if zone.AgeMS >= 450 {
		addEvent(state, Event{Type: "singularityExplosion", Source: owner.ID, X: zone.X, Y: zone.Y, Amount: zone.Radius})
		return false, nil
	}
	return true, nil
}

func worldAttackIdleToy(state *State, tower *Tower, owner *Pokemon, profile AttackProfile) {
	if tower.CooldownMS > 0 {
		return
	}
	var target *Tower
	best := math.MaxFloat64
	for index := range state.Towers {
		candidate := &state.Towers[index]
		partner := pokemonFor(state, candidate.PokemonID)
		if candidate.ID == tower.ID || partner == nil || WorldSpecies(state, partner).AbilityID != "squishyToy" {
			continue
		}
		distance := vectorLength(tower.X-candidate.X, tower.Y-candidate.Y)
		if distance <= profile.Range && distance < best {
			target, best = candidate, distance
		}
	}
	if target == nil {
		return
	}
	species := WorldSpecies(state, owner)
	for shot := 0; shot < max(1, profile.Burst); shot++ {
		zone := WorldZone{ID: allocateID(state), Kind: "toyProjectile", PokemonID: owner.ID, X: tower.X, Y: tower.Y, TargetID: target.PokemonID, LifeMS: 30000, DelayMS: int64(shot) * profile.BurstIntervalMS, Speed: max(60, species.ProjectileSpeed*60)}
		state.World.Zones = append(state.World.Zones, zone)
		addEvent(state, Event{Type: "fire", Source: tower.ID, Target: target.PokemonID, X: tower.X, Y: tower.Y, Key: species.ProjectileID})
	}
	tower.AttackCount++
	tower.CooldownMS = max(80, profile.AttackMS)
	_ = WorldAfterAttack(state, tower.ID, profile, nil)
}

func worldAdvanceShadowTag(state *State, zone *WorldZone, tower *Tower, owner *Pokemon) (bool, error) {
	if target := enemyFor(state, zone.TargetID); target != nil && target.HP > 0 {
		zone.X, zone.Y = target.X, target.Y
	}
	radius := min(60, float64(zone.AgeMS)*60/2000)
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if enemy.HP <= 0 || enemy.Dead || enemy.Escaped || enemy.InvulnerableMS > 0 || vectorLength(enemy.X-zone.X, enemy.Y-zone.Y) > radius {
			continue
		}
		if zone.AgeMS < 2000 {
			if enemy.CanSlow {
				WorldApplyConditions(state, enemy, AttackProfile{Conditions: []ConditionSpec{status("shadowTagSlow", 1, 100, .5, 1)}}, 0)
			}
			continue
		}
		profile, err := profileFor(state, tower, owner, enemy, 0.5)
		if err != nil {
			return false, err
		}
		if err := worldDirectHit(state, tower, owner, enemy, profile, 1); err != nil {
			return false, err
		}
		enemy.Conditions = slices.DeleteFunc(enemy.Conditions, func(condition Condition) bool { return condition.Kind == "shadowTagSlow" })
	}
	if zone.AgeMS >= 2000 {
		addEvent(state, Event{Type: "shadowTagExplosion", Source: owner.ID, X: zone.X, Y: zone.Y, Amount: 60})
		return false, nil
	}
	return true, nil
}

func worldAdvanceTornado(state *State, zone *WorldZone, tower *Tower, owner *Pokemon) (bool, error) {
	if zone.HitAt == nil {
		zone.HitAt = map[int]int64{}
	}
	previous := Point{X: zone.X, Y: zone.Y}
	zone.Distance += zone.Speed * battleStepSeconds(state)
	offset := math.Sin(zone.Distance*2*math.Pi/95+zone.Angle) * 32
	zone.X = rounded(zone.Origin.X + zone.Direction.X*zone.Distance - zone.Direction.Y*offset)
	zone.Y = rounded(zone.Origin.Y + zone.Direction.Y*zone.Distance + zone.Direction.X*offset)
	route, _ := routeForState(state)
	if zone.X < -100 || zone.Y < -100 || zone.X > float64(route.Width+100) || zone.Y > float64(route.Height+100) {
		return false, nil
	}
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if enemy.HP <= 0 || enemy.Dead || enemy.Escaped || !enemy.Entered || enemy.InvulnerableMS > 0 {
			continue
		}
		profile, err := profileFor(state, tower, owner, enemy, 0.5)
		if err != nil {
			return false, err
		}
		if enemy.Invisible && !profile.CanSeeInvisible {
			continue
		}
		if last, exists := zone.HitAt[enemy.ID]; exists && zone.AgeMS-last < 300 {
			continue
		}
		radius := max(enemy.Sprite.FrameWidth, enemy.Sprite.FrameHeight) / 4
		if worldSegmentDistance(Point{X: enemy.X, Y: enemy.Y}, previous, Point{X: zone.X, Y: zone.Y}) > zone.Radius+radius {
			continue
		}
		if err := worldDirectHit(state, tower, owner, enemy, profile, 1); err != nil {
			return false, err
		}
		zone.HitAt[enemy.ID] = zone.AgeMS
		if state.World.Displacements == nil {
			state.World.Displacements = map[int]WorldDisplacement{}
		}
		state.World.Displacements[enemy.ID] = WorldDisplacement{Direction: zone.Direction, Remaining: 15}
		addEvent(state, Event{Type: "windPush", Source: owner.ID, Target: enemy.ID, X: enemy.X, Y: enemy.Y, Amount: 15})
	}
	return true, nil
}
