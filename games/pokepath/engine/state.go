package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"sync"
)

type clonePlan struct {
	mutable bool
	fields  []int
}

var clonePlans sync.Map

func planStateClone(kind reflect.Type) clonePlan {
	if cached, ok := clonePlans.Load(kind); ok {
		return cached.(clonePlan)
	}
	plan := clonePlan{}
	switch kind.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		plan.mutable = true
	case reflect.Array:
		plan.mutable = planStateClone(kind.Elem()).mutable
	case reflect.Struct:
		for field := 0; field < kind.NumField(); field++ {
			if planStateClone(kind.Field(field).Type).mutable {
				plan.fields = append(plan.fields, field)
			}
		}
		plan.mutable = len(plan.fields) > 0
	}
	actual, _ := clonePlans.LoadOrStore(kind, plan)
	return actual.(clonePlan)
}

func New(options Options, players int, seed uint64, now int64) (*State, error) {
	if players < 1 || players > 4 || now < 0 {
		return nil, errors.New("巡径守卫支持 1～4 人共同守护路线")
	}
	if options.Starter == "" {
		options.Starter = "charmander"
	}
	if options.Starter != "charmander" && options.Starter != "treecko" && options.Starter != "froaki" || options.Route != 0 {
		return nil, errors.New("请选择御三家伙伴并从第一条路线出发")
	}
	if options.Speed == 0 {
		options.Speed = 0.8
	}
	if !validSpeed(options.Speed) {
		return nil, errors.New("无效的战斗速度")
	}
	catalog := CatalogData()
	if catalog == nil {
		return nil, errors.New("规则目录尚未加载")
	}
	route, ok := routeFor(0)
	if !ok || len(route.Paths) == 0 || len(route.Waves) == 0 {
		return nil, errors.New("第一条路线尚未加载")
	}
	random := uint32(seed) ^ uint32(seed>>32)
	if random == 0 {
		random = 0x9e3779b9
	}
	state := &State{Schema: SchemaVersion, RulesVersion: RulesVersion(), Options: options, Players: players, Epoch: 1, Now: now, Random: random, NextID: 1, NextEventID: 1, Phase: "prepare", Gold: 50, Health: 14, MaxHealth: 14, TeamSlots: 6, Route: 0, Wave: 1, Speed: options.Speed, Weather: route.WeatherID, Records: map[int]int{}, RouteWaves: map[int]int{0: 1}, Team: []Pokemon{}, Box: []Pokemon{}, Inventory: []OwnedItem{}, Towers: []Tower{}, Enemies: []Enemy{}, Projectiles: []Projectile{}, Events: []Event{}}
	if err := initializeTrainers(state); err != nil {
		return nil, err
	}
	if err := Validate(state); err != nil {
		return nil, err
	}
	return state, nil
}

func validSpeed(value float64) bool {
	return value == 0.4 || value == 0.8 || value == 1 || value == 1.2 || value == 1.7 || value == 2 || value == 2.5 || value == 5
}

func NextRandom(state *State) float64 {
	value := state.Random
	value ^= value << 13
	value ^= value >> 17
	value ^= value << 5
	state.Random = value
	return float64(value) / 4294967296
}

func NextInt(state *State, count int) int {
	if count <= 0 {
		return 0
	}
	return int(NextRandom(state) * float64(count))
}

func allocateID(state *State) int {
	id := state.NextID
	state.NextID++
	return id
}

func addEvent(state *State, event Event) {
	event.ID = state.NextEventID
	state.NextEventID++
	event.At = state.Tick
	state.Events = append(state.Events, event)
	if len(state.Events) > 128 {
		state.Events = append([]Event{}, state.Events[len(state.Events)-128:]...)
	}
}

func newPokemon(state *State, key string, level int) (Pokemon, error) {
	species, ok := CatalogData().Species[key]
	if !ok || level < 1 || level > 100 {
		return Pokemon{}, errors.New("无效的伙伴种类或等级")
	}
	target := defaultPokemonTarget(species)
	pokemon := Pokemon{ID: allocateID(state), Species: key, Level: level, Target: target, TileID: -1}
	if err := refreshPokemon(state, &pokemon); err != nil {
		return Pokemon{}, err
	}
	return pokemon, nil
}

func defaultPokemonTarget(species Species) string {
	switch species.AttackType {
	case "area", "aura", "bombardment":
		return species.AttackType
	}
	switch species.AbilityID {
	case "scrappy", "frisk", "vigilantFrisk", "moongeistBeam", "grimNeigh":
		return "invisible"
	case "burn":
		return "notBurned"
	case "spinda":
		return "random"
	case "curse", "curseDoubleShot":
		return "curseable"
	case "willOWisp":
		return "cursed"
	default:
		return "first"
	}
}

func refreshPokemon(state *State, pokemon *Pokemon) error {
	_, ok := CatalogData().Species[pokemon.Species]
	species := WorldSpecies(state, pokemon)
	if !ok || pokemon.Level < 1 || pokemon.Level > 100 || !finiteNumber(pokemon.Overdamage) || pokemon.Overdamage < 0 || !finiteNumber(pokemon.Damage) || pokemon.Damage < 0 {
		return errors.New("伙伴数据不完整")
	}
	level := pokemon.Level
	if state.Progression.Challenge.Active && state.Progression.Challenge.LevelCap > 0 {
		level = state.Progression.Challenge.LevelCap
	}
	pokemon.Stats = Stats{Power: math.Floor(species.PowerBase + species.PowerPerLevel*float64(level)), AttackMS: math.Max(0, math.Floor(species.AttackMSBase+species.AttackMSPerLevel*float64(level))), Range: math.Max(0, math.Floor(species.RangeBase+species.RangePerLevel*float64(level))), CriticalPct: math.Max(0, math.Min(100, species.CriticalBase+species.CriticalPerLevel*float64(level)))}
	pokemon.Sprite = species.Sprite
	prices, exists := CatalogData().UpgradeCosts[species.CostScale]
	if !exists || len(prices) <= pokemon.Level {
		return errors.New("升级费用事实表尚未加载")
	}
	pokemon.UpgradeCost = prices[pokemon.Level]
	pokemon.UpgradeCosts = map[string]int64{}
	for _, count := range []int{1, 5, 10} {
		var cost int64
		for level := pokemon.Level; level < min(100, pokemon.Level+count); level++ {
			cost += prices[level]
		}
		if state.Progression.Reset[pokemon.ID] {
			cost = 0
		}
		pokemon.UpgradeCosts[fmt.Sprint(count)] = cost
	}
	if pokemon.Level == 100 || state.Progression.Reset[pokemon.ID] {
		pokemon.UpgradeCost = 0
	}
	ApplyProgressionAppearance(state, pokemon)
	if species.AttackType == "area" || species.AttackType == "aura" || species.AttackType == "bombardment" {
		pokemon.Target = species.AttackType
	} else if pokemon.Target == "area" || pokemon.Target == "aura" || pokemon.Target == "bombardment" {
		pokemon.Target = "first"
	}
	return nil
}

func pokemonFor(state *State, id int) *Pokemon {
	for index := range state.Team {
		if state.Team[index].ID == id {
			return &state.Team[index]
		}
	}
	return nil
}

func ownedPokemonFor(state *State, id int) *Pokemon {
	if pokemon := pokemonFor(state, id); pokemon != nil {
		return pokemon
	}
	for index := range state.Box {
		if state.Box[index].ID == id {
			return &state.Box[index]
		}
	}
	return nil
}

func Marshal(state *State) ([]byte, error) {
	if err := Validate(state); err != nil {
		return nil, err
	}
	return json.Marshal(state)
}

func Unmarshal(data []byte) (*State, error) {
	if len(data) == 0 || len(data) > 8<<20 {
		return nil, errors.New("无效的巡径守卫存档大小")
	}
	var state State
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return nil, err
	}
	if err := Validate(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

func Clone(state *State) (*State, error) {
	if err := Validate(state); err != nil {
		return nil, err
	}
	return cloneStateValue(reflect.ValueOf(state)).Interface().(*State), nil
}

func cloneStateValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(cloneStateValue(value.Elem()))
		return result
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.New(value.Type()).Elem()
		result.Set(cloneStateValue(value.Elem()))
		return result
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		result.Set(value)
		for _, index := range planStateClone(value.Type()).fields {
			result.Field(index).Set(cloneStateValue(value.Field(index)))
		}
		return result
	case reflect.Array:
		if !planStateClone(value.Type()).mutable {
			return value
		}
		result := reflect.New(value.Type()).Elem()
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(cloneStateValue(value.Index(index)))
		}
		return result
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		if !planStateClone(value.Type().Elem()).mutable {
			reflect.Copy(result, value)
			return result
		}
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(cloneStateValue(value.Index(index)))
		}
		return result
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			result.SetMapIndex(iterator.Key(), cloneStateValue(iterator.Value()))
		}
		return result
	default:
		return value
	}
}

func Validate(state *State) error {
	catalog := CatalogData()
	if state == nil || catalog == nil || state.Schema != SchemaVersion || state.RulesVersion != RulesVersion() || state.Players < 1 || state.Players > 4 || state.Epoch < 1 || state.Epoch > 1000000000 || state.Sequence > 9007199254740991 || state.Now < 0 || state.Now > 9007199254740991 || state.Clock < 0 || state.Clock > 9007199254740991 || state.Tick < 0 || state.Tick > 9007199254740991 || !battleClockValid(state) || state.Random == 0 || state.NextID < 1 || state.NextID > 1000000000 || state.NextEventID < 1 || state.NextEventID > 1000000000 || state.Gold < 0 || state.Gold > 9007199254740991 || state.Health < 0 || state.Health > state.MaxHealth || state.MaxHealth < 1 || state.MaxHealth > 10000 || state.TeamSlots < 1 || state.TeamSlots > 10 || len(state.Team) > state.TeamSlots*max(1, len(state.Trainers)) || len(state.Towers) > 20*state.Players || len(state.Enemies) > 4096 || len(state.Projectiles) > 8192 || len(state.Events) > 128 || !finiteNumber(state.TotalDamage) || state.TotalDamage < 0 || !finiteNumber(state.TotalOverdamage) || state.TotalOverdamage < 0 || !validSpeed(state.Speed) || state.Wave < 1 {
		return errors.New("巡径守卫状态结构无效")
	}
	route, ok := routeForState(state)
	if !ok || len(route.Paths) == 0 || state.Records == nil || state.RouteWaves == nil {
		return errors.New("存档路线不完整")
	}
	ids := map[int]bool{}
	for _, pokemon := range append(append([]Pokemon{}, state.Team...), state.Box...) {
		if pokemon.Owner < 0 || pokemon.Owner >= state.Players || pokemon.ID < 1 || ids[pokemon.ID] || pokemon.ID >= state.NextID || pokemon.Level < 1 || pokemon.Level > 100 || !validateDexTowerStats(pokemon.Dex) {
			return errors.New("伙伴标识或等级无效")
		}
		if _, ok := catalog.Species[pokemon.Species]; !ok {
			return fmt.Errorf("未知伙伴 %s", pokemon.Species)
		}
		ids[pokemon.ID] = true
	}
	for _, enemy := range state.Enemies {
		path := EnemyPath(state, &enemy)
		if enemy.ID < 1 || enemy.ID >= state.NextID || !finiteNumber(enemy.X) || !finiteNumber(enemy.Y) || !finiteNumber(enemy.HP) || !finiteNumber(enemy.Armor) || enemy.MaxHP <= 0 || enemy.PathIndex < 0 || enemy.PathIndex >= len(route.Paths) || enemy.Waypoint < 0 || enemy.PathOriginWaypoint < 0 || len(path) == 0 || enemy.Waypoint >= len(path) {
			return errors.New("敌人状态或路径无效")
		}
	}
	for _, tower := range state.Towers {
		if !finiteNumber(tower.DexCountdownMS) || tower.DexCountdownMS > 1550 || tower.WaveActiveMS < 0 || tower.WaveActiveMS > 9007199254740991 || tower.CriticalStreak < 0 || tower.CriticalStreak > 5 || tower.SinceCriticalMS < 0 || tower.SinceCriticalMS > 6000 || !finiteNumber(tower.StatsRefreshMS) || tower.StatsRefreshMS < 0 || tower.StatsRefreshMS > 100 || !finiteNumber(tower.IncenseTimerMS) || tower.IncenseTimerMS < 0 || tower.IncenseTimerMS >= 1000 || tower.IncenseSeconds < 0 || tower.IncenseSeconds > 9007199254740 || tower.PublishedIncense < 0 || tower.PublishedIncense > tower.IncenseSeconds {
			return errors.New("战斗计时或暴击叠层无效")
		}
	}
	for _, projectile := range state.Projectiles {
		if !finiteNumber(projectile.ExtraPowerMultiplier) || projectile.ExtraPowerMultiplier < 0 {
			return errors.New("额外攻击倍率无效")
		}
		if projectile.RicochetHits < 0 || projectile.RicochetHits > 4096 {
			return errors.New("弹射次数无效")
		}
		if projectile.ID < 1 || projectile.ID >= state.NextID || !finiteNumber(projectile.X) || !finiteNumber(projectile.Y) || !finiteNumber(projectile.Power) || !finiteNumber(projectile.BeamWidth) || projectile.BeamWidth < 0 || !finiteNumber(projectile.ModeBonusPct) || projectile.ModeBonusPct < 0 || !finiteNumber(projectile.ModeFinalMultiplier) || projectile.ModeFinalMultiplier < 0 || !finiteNumber(projectile.Angle) || !finiteNumber(projectile.OrbitRadius) || projectile.OrbitRadius < 0 || !finiteNumber(projectile.RemainingDistance) || projectile.RemainingDistance < 0 || projectile.LifeMS < 0 {
			return errors.New("弹道状态无效")
		}
	}
	if err := ValidateWorld(state); err != nil {
		return err
	}
	if err := ValidateAttackModes(state); err != nil {
		return err
	}
	return validateTrainers(state)
}

func Project(state *State) View {
	copy, err := Clone(state)
	if err != nil {
		return View{Phase: "error"}
	}
	view := ProjectTransient(copy)
	view.Tiles = append([]Placement{}, view.Tiles...)
	paths := make([][]Point, len(view.Paths))
	for index := range view.Paths {
		paths[index] = append([]Point{}, view.Paths[index]...)
	}
	view.Paths = paths
	return view
}

func ProjectTransient(state *State) View {
	team, err := projectCombatStats(state)
	if err != nil {
		return View{Phase: "error"}
	}
	route, _ := routeForState(state)
	paths := pathsForState(state)
	return projectTrainers(state, projectViewLists(View{PhysicsClock: state.Tick, Tick: state.Tick, RemainderMS: state.RemainderMS, RulesVersion: state.RulesVersion, Epoch: state.Epoch, Sequence: state.Sequence, Clock: state.Clock, Random: state.Random, Phase: state.Phase, Gold: state.Gold, Health: state.Health, MaxHealth: state.MaxHealth, Stars: state.Stars, Ribbons: state.Ribbons, TeamSlots: state.TeamSlots, Route: state.Route, Wave: state.Wave, WaveActive: state.WaveActive, WaveGold: state.WaveGold, TotalDamage: state.TotalDamage, TotalOverdamage: state.TotalOverdamage, WaveElapsedMS: state.World.WaveElapsedMS, Paused: state.Paused, AutoWave: state.AutoWave, Speed: state.Speed, Weather: state.Weather, Width: route.Width, Height: route.Height, Background: route.Background, Tiles: route.Placements, Paths: paths, Team: team, Box: state.Box, Items: state.Inventory, Towers: state.Towers, Enemies: state.Enemies, Projectiles: state.Projectiles, Events: state.Events, Records: state.Records, Progression: ProjectProgression(trainerLens(state, 0)), World: state.World, AttackModes: state.AttackModes}))
}

func projectCombatStats(state *State) ([]Pokemon, error) {
	team := append([]Pokemon{}, state.Team...)
	for index := range team {
		if err := fillProjectedCombatStats(state, &state.Team[index], &team[index]); err != nil {
			return nil, err
		}
	}
	return team, nil
}

func ProjectPokemon(state *State, id int) (Pokemon, error) {
	if err := Validate(state); err != nil {
		return Pokemon{}, err
	}
	pokemon := ownedPokemonFor(state, id)
	if pokemon == nil {
		return Pokemon{}, errors.New("请选择已拥有的伙伴")
	}
	result := cloneStateValue(reflect.ValueOf(*pokemon)).Interface().(Pokemon)
	if err := fillProjectedCombatStats(state, pokemon, &result); err != nil {
		return Pokemon{}, err
	}
	result.EquipableItems = []string{}
	for _, item := range state.Inventory {
		if progressionEquipEligible(state, pokemon, item.ID) {
			result.EquipableItems = append(result.EquipableItems, item.ID)
		}
	}
	slices.Sort(result.EquipableItems)
	return result, nil
}

func fillProjectedCombatStats(state *State, pokemon, result *Pokemon) error {
	tower := worldTowerFor(state, pokemon.ID)
	var profile AttackProfile
	var err error
	if tower == nil {
		virtual := &Tower{PokemonID: pokemon.ID, TileID: -1, X: -1000000, Y: -1000000}
		context := contextFor(state, virtual, pokemon, nil, .5)
		context.Terrain = 0
		profile, err = BuildAttackProfile(context)
	} else {
		profile, err = profileFor(state, tower, pokemon, nil, .5)
	}
	if err != nil {
		return err
	}
	stats := Stats{Power: profile.Power, Range: profile.Range, AttackMS: profile.AttackMS, CriticalPct: profile.CriticalPct}
	if pokemon.Stats.AttackMS == 0 {
		stats.AttackMS = 0
	}
	result.CombatStats = &stats
	result.TargetModes = pokemonTargetModes(state, pokemon)
	result.Target = forcedTargetMode(state, pokemon)
	result.CombatAttackMode, result.CombatMoveMode = profile.AttackMode, profile.MoveMode
	species := WorldSpecies(state, pokemon)
	result.CombatRangeType = combatRangeType(species, *pokemon, profile)
	result.CombatRangeHalfWidth = combatRangeHalfWidth(state, *pokemon, result.CombatRangeType)
	result.CombatInnerRange = species.InnerRange
	if profile.InnerRange > 0 {
		result.CombatInnerRange = profile.InnerRange
	}
	return nil
}
