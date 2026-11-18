package engine

import (
	"fmt"
	"math"
)

func executePassiveEvents(state *State, events []EnemyPassiveEvent) error {
	for _, event := range events {
		switch event.Kind {
		case "spawn":
			for index := 0; index < int(event.Amount); index++ {
				position := Point{X: event.X, Y: event.Y}
				spec := CatalogData().Enemies[event.Key]
				if event.Scatter > 0 {
					position.X += NextRandom(state)*event.Scatter + event.Offset + spec.Sprite.FrameWidth/2
					position.Y += NextRandom(state)*event.Scatter + event.Offset + spec.Sprite.FrameHeight/2
				}
				waypoint := event.Waypoint
				if event.PathOriginWaypoint > 0 {
					waypoint = 0
				}
				if event.AtRouteStart {
					spec := CatalogData().Enemies[event.Key]
					prototype := Enemy{Key: event.Key, PathIndex: event.PathIndex, Sprite: spec.Sprite, EnemyEffectState: EnemyEffectState{PassiveID: spec.PassiveID}}
					ConfigureEnemyPath(state, &prototype)
					path := EnemyPath(state, &prototype)
					if len(path) == 0 {
						return fmt.Errorf("enemy spawn route is empty")
					}
					position = Point{X: path[0].X + spec.Sprite.FrameWidth/2, Y: path[0].Y + spec.Sprite.FrameHeight/2}
				}
				id, err := SpawnEnemyAt(state, event.Key, position, event.PathIndex, waypoint)
				if err != nil {
					return err
				}
				child := enemyFor(state, id)
				child.PathOriginWaypoint = event.PathOriginWaypoint
				if event.HealthScale != 0 && event.HealthScale != 1 {
					enemy := enemyFor(state, id)
					enemy.HP = math.Floor(enemy.HP * event.HealthScale)
					enemy.MaxHP = enemy.HP
				}
			}
		case "anchorShot", "pressure", "stripItem":
			if err := WorldEnemyEvent(state, event); err != nil {
				return err
			}
		case "sharedDamage":
			if enemy := enemyFor(state, event.EnemyID); enemy != nil && enemy.HP > 0 {
				damageTrackedEnemy(state, enemy, event.SourcePokemonID, event.Amount, event.Key, 0)
				enemy.LastHitCritical = event.Key == "critical"
				lost := enemy.LastHPDamage + enemy.LastArmorDamage + enemy.LastShieldDamage
				addEvent(state, damageEventFromEnemy(enemy, event.SourcePokemonID, event.Key, event.Key == "critical"))
				enemy.LastHitBy = event.SourcePokemonID
				recordPokemonDamage(state, event.SourcePokemonID, lost, enemy.LastResolvedDamage)
				if err := worldSourceHealing(state, enemy, event.SourcePokemonID, lost, event.SourceAbilityID); err != nil {
					return err
				}
				RecordProgressionIndirectDamage(state, enemy, lost)
				deathEvents, err := resolveEnemyDeath(state, enemy)
				if err != nil {
					return err
				}
				if err := executePassiveEvents(state, deathEvents); err != nil {
					return err
				}
			}
		case "heal":
			for index := range state.Enemies {
				enemy := &state.Enemies[index]
				if enemy.HP > 0 && !enemy.Dead && !enemy.Escaped && vectorLength(enemy.X-event.X, enemy.Y-event.Y) <= event.Radius && !HasCondition(&enemy.EnemyEffectState, "healBlock") {
					enemy.HP = min(enemy.MaxHP, enemy.HP+event.Amount)
					enemy.HealingEvents++
					WorldOnEnemyHeal(state, enemy)
				}
			}
		case "weather":
			state.Weather = event.Key
			for index := range state.Enemies {
				if err := SetEnemyWeather(&state.Enemies[index].EnemyEffectState, event.Key, false); err != nil {
					return err
				}
			}
		case "rewind":
			if enemy := enemyFor(state, event.EnemyID); enemy != nil {
				rewindEnemy(state, enemy, event.Amount)
			}
		case "revive", "phase":
		case "heart":
			if event.Amount >= 0 {
				before := state.Health
				state.Health = min(state.MaxHealth, state.Health+int(event.Amount))
				state.WaveHeartsHealed += state.Health - before
				if event.Key == "healer" {
					state.World.HealScale += int(event.Amount)
					state.World.RoomServicePower += 20
				}
			} else {
				lost := min(state.Health, WorldOnHeartsLost(state, int(-event.Amount)))
				state.Health -= lost
				state.WaveHeartsLost += lost
				if state.Health == 0 {
					state.Phase, state.WaveActive = "defeat", false
					state.Projectiles = []Projectile{}
				}
			}
		case "power":
			if event.Key != "waveHoney" {
				return fmt.Errorf("unknown enemy power event %q", event.Key)
			}
			state.World.Honey += int(event.Amount)
		default:
			return fmt.Errorf("unknown enemy passive event %q", event.Kind)
		}
		addEvent(state, Event{Type: event.Kind, Target: event.EnemyID, X: event.X, Y: event.Y, Amount: event.Amount, Key: event.Key})
	}
	return nil
}

func rewindEnemy(state *State, enemy *Enemy, amount float64) {
	path := EnemyPath(state, enemy)
	remaining := amount
	for remaining > 0 && enemy.Waypoint > 0 {
		target := path[enemy.Waypoint-1]
		dx, dy := target.X-enemy.X, target.Y-enemy.Y
		distance := vectorLength(dx, dy)
		if distance <= remaining {
			enemy.X, enemy.Y = target.X, target.Y
			remaining -= distance
			enemy.Waypoint--
		} else {
			enemy.X += dx / distance * remaining
			enemy.Y += dy / distance * remaining
			remaining = 0
		}
	}
}
