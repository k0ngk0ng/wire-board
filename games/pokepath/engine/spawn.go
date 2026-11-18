package engine

import (
	"errors"
	"math"
)

func SpawnEnemyAt(state *State, key string, position Point, pathIndex, waypoint int) (int, error) {
	specification, ok := CatalogData().Enemies[key]
	paths := pathsForState(state)
	if !ok || !finiteNumber(position.X) || !finiteNumber(position.Y) || pathIndex < 0 || pathIndex >= len(paths) || waypoint < 0 || waypoint >= len(paths[pathIndex]) || len(state.Enemies) >= 4096 {
		return 0, errors.New("敌人生成位置或规则数据无效")
	}
	hp, armor := specification.Health, specification.Armor
	steps := float64((state.Wave - 1) / 5)
	if state.Wave != 100 {
		hp = math.Floor(hp * (1 + 0.02*steps))
		armor = math.Floor(armor * (1 + 0.01*steps))
	}
	gold := int64(math.Floor(float64(specification.Gold+state.Progression.ExtraGold) * (1 + 0.15*steps)))
	if state.Progression.Challenge.Active && state.Progression.Challenge.ToughEnemies > 0 {
		factor := 1 + float64(state.Progression.Challenge.ToughEnemies)/100
		hp, armor = math.Floor(hp*factor), math.Floor(armor*factor)
	}
	if state.Progression.Challenge.Active && state.Progression.Challenge.Draft {
		factor := 0.1
		if state.Wave == 100 {
			factor = 0.25
		}
		hp -= math.Floor(hp * factor)
		armor -= math.Floor(armor * factor)
	}
	if state.Progression.CustomData != nil {
		hp += math.Floor(hp * float64(state.Progression.CustomData.Health) / 100)
		armor += math.Floor(armor * float64(state.Progression.CustomData.Armor) / 100)
	}
	enemy := Enemy{ID: allocateID(state), Key: key, Species: key, X: position.X, Y: position.Y, PathIndex: pathIndex, Waypoint: waypoint, Power: specification.Power, Gold: gold, SpawnAt: state.Tick, Sprite: specification.Sprite, EnemyEffectState: EnemyEffectState{HP: hp, MaxHP: hp, Armor: armor, MaxArmor: armor, BaseSpeed: specification.Speed, Invisible: specification.Invisible, CanStun: specification.CanStun, CanSlow: specification.CanSlow, CanBurn: specification.CanBurn, CanPoison: specification.CanPoison, Boss: specification.IsBoss, PassiveID: specification.PassiveID, WeatherID: state.Weather, Regeneration: specification.Regeneration, Conditions: []Condition{}}}
	ConfigureEnemyPath(state, &enemy)
	if err := SetEnemyWeather(&enemy.EnemyEffectState, state.Weather, true); err != nil {
		return 0, err
	}
	state.Enemies = append(state.Enemies, enemy)
	events, err := EnemyPassiveEvents(state, &state.Enemies[len(state.Enemies)-1], "spawn", 0)
	if err != nil {
		return 0, err
	}
	if err := executePassiveEvents(state, events); err != nil {
		return 0, err
	}
	return enemy.ID, nil
}
