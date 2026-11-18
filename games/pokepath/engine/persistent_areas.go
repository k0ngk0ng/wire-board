package engine

import (
	"errors"
	"maps"
	"math"
)

func createPersistentArea(state *State, tower *Tower, owner *Pokemon, profile AttackProfile) (bool, error) {
	ability := WorldSpecies(state, owner).AbilityID
	if ability != "spikes" && ability != "precipiceBlades" {
		return false, nil
	}
	if profile.AuraDurationMS <= 0 || profile.AuraTickMS <= 0 {
		return true, errors.New("持续尖刺缺少寿命或跳伤间隔")
	}
	for _, existing := range state.World.Zones {
		if existing.PersistentArea && existing.PokemonID == owner.ID && existing.LifeMS-existing.AgeMS > existing.LifeMS/10 && vectorLength(existing.X-tower.X, existing.Y-tower.Y) <= 32 {
			return true, nil
		}
	}
	kind, inner, magma := "spikes", 0.0, false
	if ability == "precipiceBlades" {
		kind, inner, magma = "magmaSpikes", WorldSpecies(state, owner).InnerRange, true
		if profile.InnerRange > 0 {
			inner = profile.InnerRange
		}
	}
	source := *tower
	source.Stacks = maps.Clone(tower.Stacks)
	zone := WorldZone{ID: allocateID(state), Kind: kind, PokemonID: owner.ID, X: tower.X, Y: tower.Y, Radius: profile.Range, InnerRadius: inner, Magma: magma, PersistentArea: true, SourceTower: &source, LifeMS: profile.AuraDurationMS, TickMS: profile.AuraTickMS}
	state.World.Zones = append(state.World.Zones, zone)
	addEvent(state, Event{Type: "zone", Source: owner.ID, Target: zone.ID, X: zone.X, Y: zone.Y, Amount: zone.Radius, Key: zone.Kind})
	return true, nil
}

func advancePersistentAreas(state *State) error {
	count := len(state.World.Zones)
	kept := make([]WorldZone, 0, count)
	for index := 0; index < count; index++ {
		zone := state.World.Zones[index]
		if !zone.PersistentArea {
			kept = append(kept, zone)
			continue
		}
		previousAge := zone.AgeMS
		zone.AgeMS += battleStepWholeMS(state)
		if zone.AgeMS >= zone.LifeMS {
			continue
		}
		if zone.Smoke {
			kept = append(kept, zone)
			continue
		}
		owner := ownedPokemonFor(state, zone.PokemonID)
		tower := worldTowerFor(state, zone.PokemonID)
		if tower == nil || tower.ID != zone.SourceTower.ID {
			tower = zone.SourceTower
		}
		if owner == nil || tower == nil {
			continue
		}
		if state.WaveActive && zone.AgeMS/zone.TickMS > previousAge/zone.TickMS {
			ids := []int{}
			for _, enemy := range state.Enemies {
				distance := vectorLength(enemy.X-zone.X, enemy.Y-zone.Y)
				if enemy.HP > 0 && !enemy.Dead && !enemy.Escaped && enemy.InvulnerableMS <= 0 && distance >= zone.InnerRadius && distance <= zone.Radius {
					ids = append(ids, enemy.ID)
				}
			}
			for _, id := range ids {
				enemy := enemyFor(state, id)
				if enemy == nil || enemy.HP <= 0 || enemy.Dead {
					continue
				}
				profile, err := profileFor(state, tower, owner, enemy, .5)
				if err != nil {
					return err
				}
				if enemy.Invisible && !profile.CanSeeInvisible {
					continue
				}
				if err := worldDirectHit(state, tower, owner, enemy, profile, 1); err != nil {
					return err
				}
				enemy = enemyFor(state, id)
				if enemy == nil {
					continue
				}
				if zone.Magma && !HasCondition(&enemy.EnemyEffectState, "burn") {
					condition := burnCondition()
					condition.SourcePokemonID = owner.ID
					if owner.Item == "magmaStone" {
						condition.DurationMS = 20000
					}
					if owner.Item == "falmeOrb" {
						condition.Magnitude = .0075
					}
					WorldApplyConditions(state, enemy, AttackProfile{Conditions: []ConditionSpec{condition}}, 0)
				}
			}
		}
		kept = append(kept, zone)
	}
	state.World.Zones = append(kept, state.World.Zones[count:]...)
	return nil
}

func createSteamTrail(state *State, tower *Tower, owner *Pokemon, projectile *Projectile) {
	length := projectile.RayLength
	radius := projectile.Profile.AuraRadius
	steps := max(1, int(math.Ceil(length/(radius*1.5))))
	for index := 0; index <= steps; index++ {
		fraction := float64(index) / float64(steps)
		distance := float64(length * fraction)
		dx := float64(projectile.RayDX * distance)
		dy := float64(projectile.RayDY * distance)
		point := Point{X: tower.X + dx, Y: tower.Y + dy}
		exists := false
		for _, zone := range state.World.Zones {
			if zone.Smoke && zone.PokemonID == owner.ID && zone.LifeMS-zone.AgeMS > zone.LifeMS/10 && zone.X == point.X && zone.Y == point.Y {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		zone := WorldZone{ID: allocateID(state), Kind: "smoke", PokemonID: owner.ID, X: point.X, Y: point.Y, Radius: radius, LifeMS: projectile.Profile.AuraDurationMS, TickMS: projectile.Profile.AuraTickMS, PersistentArea: true, Smoke: true, SlowFactor: .8}
		state.World.Zones = append(state.World.Zones, zone)
	}
}

func persistentAreaSlowFactor(state *State, enemy *Enemy) float64 {
	factor := 1.0
	for _, zone := range state.World.Zones {
		if zone.Smoke && zone.AgeMS < zone.LifeMS && vectorLength(enemy.X-zone.X, enemy.Y-zone.Y) <= zone.Radius {
			factor = min(factor, zone.SlowFactor)
		}
	}
	return factor
}
