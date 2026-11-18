package engine

import (
	"errors"
	"math"
)

const battleFrameUpperMS int64 = 17
const battleUnitsPerMS int64 = 180
const battleUnitsPerFrame int64 = 3000

func battleStepMS(state *State) float64 {
	return float64(state.StepUnits) / float64(battleUnitsPerMS)
}

func battleStepSeconds(state *State) float64 {
	return float64(state.StepUnits) / float64(battleUnitsPerMS*1000)
}

func battleStepWholeMS(state *State) int64 {
	return state.Tick - (state.SimulationUnits-state.StepUnits)/battleUnitsPerMS
}

func battleClockValid(state *State) bool {
	if state.BattleFrame > 9007199254740991 || state.SimulationUnits < 0 || state.SimulationUnits > 9007199254740991 || state.Tick != state.SimulationUnits/battleUnitsPerMS || state.StepUnits < 0 || state.StepUnits > battleUnitsPerFrame || state.StepUnits > state.SimulationUnits || state.BattleFrame == 0 && (state.SimulationUnits != 0 || state.StepUnits != 0) || state.BattleFrame > 0 && state.StepUnits == 0 || state.RemainderMS < 0 || state.RemainderMS >= battleFrameUpperMS || !finiteNumber(state.SpeedRemainder) || state.SpeedRemainder < 0 || state.SpeedRemainder >= 1 {
		return false
	}
	fraction := state.SpeedRemainder * float64(battleUnitsPerMS)
	units := math.Round(fraction)
	return math.Abs(fraction-units) < 1e-9 && state.RemainderMS*battleUnitsPerMS+int64(units) < battleUnitsPerFrame
}

func startWave(state *State) error {
	if state.WaveActive || state.Paused || state.Health <= 0 || state.Progression.Draft.Active {
		return errors.New("当前不能开始下一波")
	}
	route, ok := routeForState(state)
	if !ok {
		return errors.New("路线未加载")
	}
	wave, ok := waveFor(route, state.Wave)
	if !ok || len(wave.Sequence) == 0 {
		return errors.New("波次数据未加载")
	}
	paths := route.Paths
	if ProgressionMirrored(state) && len(route.MirrorPaths) > 0 {
		paths = route.MirrorPaths
	}
	for _, path := range paths {
		if len(path) < 2 {
			return errors.New("路线需要完整的入口与出口路径")
		}
	}
	state.Enemies = []Enemy{}
	state.Projectiles = persistentProjectiles(state)
	state.WaveKills = 0
	state.WaveGold = 0
	state.WaveHeartsLost = 0
	state.WaveHeartsHealed = 0
	resetWaveDamage(state)
	state.Progression.ActiveMirror = ProgressionMirrored(state)
	state.WaveActive = true
	state.Phase = "battle"
	for index, key := range wave.Sequence {
		if key == nil {
			continue
		}
		specification, exists := CatalogData().Enemies[*key]
		if !exists {
			return errors.New("敌人资料不完整")
		}
		pathIndex := NextInt(state, len(paths))
		if state.Wave == 100 && route.ID == 21 {
			pathIndex = 0
		}
		if state.Wave == 100 && (route.ID == 40 || route.ID == 44) {
			pathIndex = 1
		}
		prototype := Enemy{Key: *key, PathIndex: pathIndex, Sprite: specification.Sprite, EnemyEffectState: EnemyEffectState{PassiveID: specification.PassiveID}}
		ConfigureEnemyPath(state, &prototype)
		position := EnemySpawnOrigin(state, &prototype, index)
		if _, err := SpawnEnemyAt(state, *key, position, pathIndex, 0); err != nil {
			return err
		}
	}
	OnProgressionWaveStarted(state)
	if err := WorldOnWaveStart(state); err != nil {
		return err
	}
	OnAttackModeWaveStarted(state)
	resetWaveCombatCounters(state, false)
	if state.Progression.Challenge.Active && state.Progression.ChallengeStartMS == nil {
		started := state.Clock
		state.Progression.ChallengeStartMS = &started
	}
	addEvent(state, Event{Type: "wave", Amount: float64(state.Wave)})
	return nil
}

func Advance(state *State, milliseconds int64) error {
	if milliseconds < 0 || milliseconds > 600000 {
		return errors.New("无效的规则推进时长")
	}
	if err := Validate(state); err != nil {
		return err
	}
	if state.Paused || milliseconds == 0 || state.Phase == "defeat" || state.Phase == "practice_complete" || state.Phase == "route_complete" {
		return nil
	}
	candidate, err := Clone(state)
	if err != nil {
		return err
	}
	remaining := milliseconds
	for remaining > 0 {
		phase := candidate.RemainderMS*battleUnitsPerMS + int64(math.Round(candidate.SpeedRemainder*float64(battleUnitsPerMS)))
		needed := battleUnitsPerFrame - phase
		consumed := min(remaining, (needed+battleUnitsPerMS-1)/battleUnitsPerMS)
		candidate.Clock += consumed
		remaining -= consumed
		phase += consumed * battleUnitsPerMS
		frameReady := phase >= battleUnitsPerFrame
		if frameReady {
			phase -= battleUnitsPerFrame
		}
		candidate.RemainderMS = phase / battleUnitsPerMS
		candidate.SpeedRemainder = float64(phase%battleUnitsPerMS) / float64(battleUnitsPerMS)
		if frameReady {
			candidate.BattleFrame++
			parts := int64(math.Ceil(candidate.Speed))
			candidate.StepUnits = int64(math.Round(candidate.Speed*10)) * (battleUnitsPerFrame / 10) / parts
			for part := int64(0); part < parts; part++ {
				candidate.SimulationUnits += candidate.StepUnits
				candidate.Tick = candidate.SimulationUnits / battleUnitsPerMS
				candidate.Sequence++
				if err := stepWorld(candidate, part == parts-1); err != nil {
					return err
				}
				if retried, err := automaticDefeatRetry(candidate); err != nil {
					return err
				} else if retried {
					remaining = 0
					break
				}
				if candidate.Phase == "defeat" || candidate.Phase == "practice_complete" || candidate.Phase == "route_complete" {
					remaining = 0
					break
				}
			}
		}
	}
	if err := Validate(candidate); err != nil {
		return err
	}
	*state = *candidate
	return nil
}

func stepWorld(state *State, lastStep bool) error {
	if !state.WaveActive {
		if err := WorldBeforeStep(state); err != nil {
			return err
		}
		if err := advanceOrderedTowers(state, lastStep); err != nil {
			return err
		}
		if err := advancePersistentAreas(state); err != nil {
			return err
		}
		if lastStep {
			return worldAdvanceLinks(state, battleStepMS(state)*math.Ceil(state.Speed))
		}
		return nil
	}
	if err := WorldBeforeStep(state); err != nil {
		return err
	}
	if err := moveEnemies(state); err != nil {
		return err
	}
	if err := settleEnemies(state); err != nil {
		return err
	}
	if state.Phase == "defeat" || state.Phase == "practice_complete" {
		return nil
	}
	if err := advanceOrderedTowers(state, lastStep); err != nil {
		return err
	}
	if err := advancePersistentAreas(state); err != nil {
		return err
	}
	if lastStep {
		if err := worldAdvanceLinks(state, battleStepMS(state)*math.Ceil(state.Speed)); err != nil {
			return err
		}
	}
	if err := settleEnemies(state); err != nil {
		return err
	}
	OnProgressionWorldStep(state)
	if state.WaveActive && len(state.Enemies) == 0 {
		route, _ := routeForState(state)
		recordDexCompletedWave(state, state.Progression.CustomRoute == nil && (state.Wave >= 100 || route.IsSecret))
		if state.Progression.CustomRoute != nil {
			finishPractice(state, true)
			return nil
		}
		completed := state.Wave
		if err := WorldOnWaveEnd(state); err != nil {
			return err
		}
		OnAttackModeWaveEnded(state)
		resetWaveCombatCounters(state, true)
		OnWaveCompleted(state)
		state.WaveActive = false
		state.Phase = "prepare"
		state.Projectiles = persistentProjectiles(state)
		if (completed >= 100 || route.IsSecret) && !ProgressionRepeatsWave(state) {
			state.Wave = completed
			state.Phase = "route_complete"
			state.AutoWave = false
			recordRouteResultForDeployed(state)
		} else if !ProgressionRepeatsWave(state) {
			state.Wave++
		}
		state.RouteWaves[state.Route] = state.Wave
		addEvent(state, Event{Type: "waveComplete", Amount: float64(completed), Key: state.Phase})
		if automaticWaveAllowed(state) {
			return startWave(state)
		}
	}
	return nil
}

func pathsForState(state *State) [][]Point {
	route, _ := routeForState(state)
	if ProgressionMirrored(state) && len(route.MirrorPaths) > 0 {
		return route.MirrorPaths
	}
	return route.Paths
}

func moveEnemies(state *State) error {
	route, _ := routeForState(state)
	stepMS := battleStepWholeMS(state)
	pending := []EnemyPassiveEvent{}
	living := livingEnemyCount(state)
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if enemy.Dead {
			enemy.DeathRemainingMS = max(0, enemy.DeathRemainingMS-stepMS)
			continue
		}
		if enemy.Escaped || enemy.HP <= 0 {
			continue
		}
		enemy.AgeMS += stepMS
		enemy.LivingEnemies = living
		if err := RefreshConditionSources(state, enemy); err != nil {
			return err
		}
		beforeHealing := enemy.HealingEvents
		for _, damage := range advanceConditionsPrecise(&enemy.EnemyEffectState, stepMS, battleStepMS(state)) {
			lost := damage.HPDamage + damage.ArmorDamage + damage.ShieldDamage
			updateDexEnemy(state, enemy.Key, 0, 0, damage.DexCredit)
			recordDexTowerDamage(state, damage.SourcePokemonID, damage.DexCredit, damage.ResolvedDamage)
			if lost <= 0 {
				continue
			}
			recordPokemonDamage(state, damage.SourcePokemonID, lost, damage.ResolvedDamage)
			enemy.LastHitBy, enemy.LastDamageSourceID = damage.SourcePokemonID, damage.SourcePokemonID
			enemy.LastDamageKind = damage.Kind
			enemy.LastHPDamage, enemy.LastArmorDamage, enemy.LastShieldDamage = damage.HPDamage, damage.ArmorDamage, damage.ShieldDamage
			enemy.LastResolvedDamage, enemy.LastIncomingDamage = damage.ResolvedDamage, damage.AttemptedDamage
			enemy.LastPreHitHP = damage.PreHitHP
			enemy.LastHitAbilityID, enemy.LastHitItemID, enemy.LastHitTerrain, enemy.LastHitCritical = "", "", 0, false
			addEvent(state, damageEventFromEnemy(enemy, damage.SourcePokemonID, damage.Kind, false))
			RecordProgressionIndirectDamage(state, enemy, lost)
			if err := worldIndirectHealing(state, enemy, damage.SourcePokemonID, lost); err != nil {
				return err
			}
			events, err := EnemyPassiveEvents(state, enemy, "hit", 0)
			if err != nil {
				return err
			}
			afterEvents, err := EnemyAfterHit(state, enemy)
			if err != nil {
				return err
			}
			pending = append(pending, events...)
			pending = append(pending, afterEvents...)
		}
		for event := beforeHealing; event < enemy.HealingEvents; event++ {
			WorldOnEnemyHeal(state, enemy)
		}
		if enemy.Entered {
			previousHP, previousArmor, previousShield := enemy.HP, enemy.Armor, enemy.Shield
			credit := AdvanceWeatherEffects(&enemy.EnemyEffectState, stepMS)
			updateDexEnemy(state, enemy.Key, 0, 0, credit)
			if enemy.HP < previousHP || enemy.Armor < previousArmor || enemy.Shield < previousShield {
				addEvent(state, damageEventFromEnemy(enemy, 0, "sandstorm", false))
			}
		}
		events, err := EnemyPassiveEvents(state, enemy, "tick", stepMS)
		if err != nil {
			return err
		}
		pending = append(pending, events...)
		if enemy.HP <= 0 {
			continue
		}
		remaining := EffectiveEnemySpeed(&enemy.EnemyEffectState) * battleStepSeconds(state)
		moved := remaining
		path := EnemyPath(state, enemy)
		if enemy.Waypoint < len(path) {
			remaining *= EnemyTerrainMultiplier(state, enemy, path[enemy.Waypoint].X-enemy.X) * persistentAreaSlowFactor(state, enemy)
		}
		for remaining > 0 && enemy.Waypoint < len(path) {
			target := path[enemy.Waypoint]
			dx, dy := target.X-enemy.X, target.Y-enemy.Y
			distance := vectorLength(dx, dy)
			if distance <= remaining+0.000001 {
				enemy.X, enemy.Y = target.X, target.Y
				remaining -= distance
				if enemy.Waypoint+1 < len(path) {
					enemy.Waypoint++
				} else {
					remaining = 0
				}
			} else {
				enemy.X = rounded(enemy.X + dx/distance*remaining)
				enemy.Y = rounded(enemy.Y + dy/distance*remaining)
				remaining = 0
			}
		}
		left, top := enemy.X-enemy.Sprite.FrameWidth/2, enemy.Y-enemy.Sprite.FrameHeight/2
		if left >= 0 && left <= float64(route.Width) && top >= -12 && top <= float64(route.Height) {
			enemy.Entered = true
		}
		if enemy.Entered {
			enemy.Distance += moved
		}
		if enemy.Waypoint == len(path)-1 && (left > float64(route.Width) || left < -30 || top-20 > float64(route.Height) || top < -20) {
			events, err = EnemyPassiveEvents(state, enemy, "escape", 0)
			if err != nil {
				return err
			}
			pending = append(pending, events...)
			power := ProgressionEnemyEscapePower(state, *enemy)
			if enemy.Key == "mew" && enemy.HP == enemy.MaxHP && len(state.Towers) >= 3 {
				power = 0
			}
			if power > 0 {
				power = WorldOnHeartsLost(state, power)
			}
			lost := min(state.Health, power)
			state.Health -= lost
			state.WaveHeartsLost += lost
			addEvent(state, Event{Type: "escape", Target: enemy.ID, X: enemy.X, Y: enemy.Y, Amount: float64(lost)})
			updateDexEnemy(state, enemy.Key, 0, 1, 0)
			enemy.Escaped = true
			if enemy.PassiveID == "grimNeigh" {
				enemy.X, enemy.Y = path[0].X, path[0].Y
				enemy.Waypoint, enemy.Distance, enemy.Entered, enemy.Escaped = 0, 0, false, false
			}
			if state.Health == 0 {
				recordTrainerWave(state, false, state.WaveGold)
				state.WaveActive = false
				state.Phase = "defeat"
				state.Projectiles = []Projectile{}
				if state.Progression.CustomRoute != nil {
					finishPractice(state, false)
				} else {
					addEvent(state, Event{Type: "routeDefeat", Amount: float64(state.Wave)})
				}
				break
			}
		}
	}
	return executePassiveEvents(state, pending)
}

func livingEnemyCount(state *State) int {
	count := 0
	for _, enemy := range state.Enemies {
		if !enemy.Dead && !enemy.Escaped && enemy.HP > 0 {
			count++
		}
	}
	return count
}

func resolveEnemyDeath(state *State, enemy *Enemy) ([]EnemyPassiveEvent, error) {
	if enemy.Escaped || enemy.Dead || enemy.HP > 0 {
		return nil, nil
	}
	events, err := EnemyPassiveEvents(state, enemy, "death", 0)
	if err != nil {
		return nil, err
	}
	if enemy.HP > 0 {
		return events, nil
	}
	if err := WorldOnDeath(state, enemy); err != nil {
		return nil, err
	}
	recordTrainerDefeat(state, enemy)
	RecordProgressionEnemyDefeat(state, *enemy)
	state.WaveKills++
	state.TotalKills++
	if state.Progression.CustomRoute == nil {
		state.Gold += enemy.Gold
		state.WaveGold += enemy.Gold
		state.Progression.TotalGold += enemy.Gold
	}
	if pokemon := pokemonFor(state, enemy.LastHitBy); pokemon != nil {
		pokemon.Kills++
	}
	if pokemon := ownedPokemonFor(state, enemy.LastHitBy); pokemon != nil {
		pokemon.Dex.Defeated++
	}
	enemy.Dead, enemy.DeathRemainingMS = true, 560
	addEvent(state, Event{Type: "defeatEnemy", Source: enemy.LastHitBy, Target: enemy.ID, X: enemy.X, Y: enemy.Y, Amount: float64(enemy.Gold), Key: enemy.Key})
	events = append(events, WorldTakeEnemyEvents(state)...)
	return events, nil
}

func settleEnemies(state *State) error {
	pending := []EnemyPassiveEvent{}
	for index := range state.Enemies {
		events, err := resolveEnemyDeath(state, &state.Enemies[index])
		if err != nil {
			return err
		}
		pending = append(pending, events...)
	}
	kept := state.Enemies[:0]
	for _, enemy := range state.Enemies {
		if !enemy.Escaped && (!enemy.Dead || enemy.DeathRemainingMS > 0) {
			kept = append(kept, enemy)
		}
	}
	state.Enemies = kept
	return executePassiveEvents(state, pending)
}

func persistentProjectiles(state *State) []Projectile {
	result := []Projectile{}
	for _, projectile := range state.Projectiles {
		if projectile.Kind == "orbit" {
			result = append(result, projectile)
		}
	}
	return result
}
