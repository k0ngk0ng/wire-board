package engine

import (
	"fmt"
	"math"
	"slices"
)

func AfterCoreProjectileHit(state *State, projectile *Projectile, enemy *Enemy, profile AttackProfile, attempted, lost float64, critical bool) error {
	if state == nil || projectile == nil || enemy == nil || !finiteNumber(attempted) || !finiteNumber(lost) || attempted < 0 || lost < 0 {
		return fmt.Errorf("invalid linked projectile hit")
	}
	if projectile.Kind == "curseLink" {
		return nil
	}
	primary := projectile.TargetID == 0 || projectile.TargetID == enemy.ID || enemy.PassiveID == "stanceChange" && !slices.Contains(projectile.HitIDs, projectile.TargetID)
	if primary && profile.AttackMode == "criticalRicochet" && (critical || projectile.RolledCritical) {
		projectile.Bounces++
	}
	if primary && profile.RepeatOnCritical && critical {
		for index := range state.Towers {
			tower := &state.Towers[index]
			if tower.ID != projectile.TowerID && tower.PokemonID != projectile.PokemonID {
				continue
			}
			if projectile.Context.AbilityID == "ceaselessEdge" {
				tower.CooldownMS = 100
			} else {
				tower.CooldownMS = -999999
			}
			break
		}
	}
	if profile.CurseLink && HasCondition(&enemy.EnemyEffectState, "curse") && attempted > 0 {
		if err := spreadCurseHit(state, projectile, enemy, profile, attempted); err != nil {
			return err
		}
	}
	return nil
}

func AfterCoreProjectileSplash(state *State, projectile *Projectile, primary *Enemy, profile AttackProfile) error {
	if state == nil || projectile == nil || primary == nil {
		return fmt.Errorf("invalid projectile splash context")
	}
	if projectile.Kind != "curseLink" && projectile.Context.AbilityID == "synchronySplash" && (projectile.TargetID == 0 || projectile.TargetID == primary.ID) {
		spreadSynchronizedConditions(state, projectile, primary, profile)
	}
	return nil
}

func spreadCurseHit(state *State, projectile *Projectile, source *Enemy, profile AttackProfile, amount float64) error {
	ids := []int{}
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if enemy.ID != source.ID && enemy.HP > 0 && !enemy.Dead && !enemy.Escaped && HasCondition(&enemy.EnemyEffectState, "curse") {
			ids = append(ids, enemy.ID)
		}
	}
	for _, id := range ids {
		enemy := enemyFor(state, id)
		if enemy == nil || enemy.HP <= 0 || enemy.Dead || enemy.Escaped {
			continue
		}
		enemy = EnemyHitTarget(state, enemy)
		enemy.LastHitAbilityID, enemy.LastHitItemID = projectile.Context.AbilityID, projectile.Context.ItemID
		enemy.LastHitTerrain, enemy.LastHitCritical = projectile.Context.Terrain, false
		enemy.LastDamageSourceID, enemy.LastHitBy = projectile.PokemonID, projectile.PokemonID
		incoming := amount
		if profile.StrangeIdol {
			peers := 0
			for index := range state.Enemies {
				other := &state.Enemies[index]
				if other.ID != enemy.ID && other.HP > 0 && !other.Dead && !other.Escaped && HasCondition(&other.EnemyEffectState, "curse") {
					peers++
				}
			}
			incoming = StrangeIdolDamage(incoming, peers)
		}
		incoming, allowed, err := PrepareEnemyHit(state, enemy, incoming, "direct")
		if err != nil {
			return err
		}
		if !allowed {
			continue
		}
		before := append([]Condition{}, enemy.Conditions...)
		damageTrackedEnemy(state, enemy, projectile.PokemonID, math.Ceil(incoming), "direct", profile.ArmorPierce)
		actual := enemy.LastHPDamage + enemy.LastArmorDamage + enemy.LastShieldDamage
		addEvent(state, damageEventFromEnemy(enemy, projectile.PokemonID, "physical", false))
		recordPokemonDamage(state, projectile.PokemonID, actual, enemy.LastResolvedDamage)
		linked := *projectile
		linked.Kind, linked.TargetID, linked.Critical, linked.RolledCritical = "curseLink", enemy.ID, false, false
		RecordProgressionHit(state, enemy, &linked, profile, incoming, actual, false, before)
		deathEvents := []EnemyPassiveEvent{}
		if enemy.HP <= 0 {
			deathEvents, err = resolveEnemyDeath(state, enemy)
			if err != nil {
				return err
			}
		}
		if projectile.Context.AbilityID == "willOWisp" {
			condition := burnCondition()
			condition.SourcePokemonID = projectile.PokemonID
			if projectile.Context.ItemID == "falmeOrb" {
				condition.Magnitude = .0075
			}
			WorldApplyConditions(state, enemy, AttackProfile{Conditions: []ConditionSpec{condition}}, 0)
		}
		if projectile.Context.ItemID == "amuletCoin" && state.Progression.CustomRoute == nil {
			gold := int64(math.Ceil(amount * .0005 * float64(state.Stars)))
			state.Gold += gold
			state.WaveGold += gold
		}
		if err := WorldOnHit(state, &linked, enemy, profile, actual); err != nil {
			return err
		}
		events, err := EnemyPassiveEvents(state, enemy, "hit", 0)
		if err != nil {
			return err
		}
		after, err := EnemyAfterHit(state, enemy)
		if err != nil {
			return err
		}
		if err := executePassiveEvents(state, append(append(deathEvents, events...), after...)); err != nil {
			return err
		}
		addEvent(state, Event{Type: "curseLink", Source: projectile.PokemonID, Target: id, X: enemy.X, Y: enemy.Y, Amount: actual})
	}
	return nil
}

func spreadSynchronizedConditions(state *State, projectile *Projectile, source *Enemy, profile AttackProfile) {
	conditions := []ConditionSpec{}
	for _, active := range source.Conditions {
		if active.RemainingMS == 0 {
			continue
		}
		var condition ConditionSpec
		switch active.Kind {
		case "burn":
			condition = burnCondition()
		case "scorch":
			condition = status("scorch", 1, 10000, .005, 1)
		case "poison":
			condition = poisonCondition(1)
		case "slow":
			condition = slowCondition()
		case "stun":
			condition = status("stun", 1, 2000, 1, 1)
		case "nightmare":
			if !profile.SpreadNightmare {
				continue
			}
			condition = status("nightmare", 1, -1, profile.Power*.2, 1)
		default:
			continue
		}
		condition.SourcePokemonID = projectile.PokemonID
		conditions = append(conditions, condition)
	}
	if len(conditions) == 0 {
		return
	}
	radius := profile.SplashRadius
	if radius <= 0 {
		radius = 65
	}
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if enemy.ID == source.ID || enemy.HP <= 0 || enemy.Dead || enemy.Escaped || vectorLength(enemy.X-source.X, enemy.Y-source.Y) > radius {
			continue
		}
		WorldApplyConditions(state, enemy, AttackProfile{Conditions: conditions}, 0)
	}
}
