package engine

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

func Apply(state *State, actor int, action Action) error {
	if err := Validate(state); err != nil {
		return err
	}
	if actor < 0 || actor >= state.Players || action.Epoch != state.Epoch {
		return errors.New("操作席位或控制代次已失效")
	}
	if !resultActionAllowed(state.Phase, action.Type) {
		return errors.New("请先确认当前波次结果")
	}
	if state.Progression.Draft.Active {
		switch action.Type {
		case "draftPick", "draftReroll", "draftRestart":
		default:
			return errors.New("请先完成轮抽")
		}
	}
	candidate, err := Clone(state)
	if err != nil {
		return err
	}
	if err := applyTrainerAction(candidate, actor, action); err != nil {
		return err
	}
	candidate.Sequence++
	if err = Validate(candidate); err != nil {
		return err
	}
	*state = *candidate
	return nil
}

func executeAction(state *State, actor int, action Action) error {
	handled, err := handleEditorAction(state, actor, action)
	if err == nil && !handled {
		handled, err = ApplyProgressionAction(state, actor, action)
	}
	if err == nil && !handled {
		err = applyCoreAction(state, actor, action)
	}
	return err
}

func applyCoreAction(state *State, actor int, action Action) error {
	switch action.Type {
	case "aim":
		return ApplyAttackModeAim(state, action)
	case "deploy", "move":
		return deployPokemon(state, action.PokemonID, action.TileID)
	case "retire":
		return retirePokemon(state, action.PokemonID)
	case "upgrade":
		return upgradePokemon(state, action.PokemonID, action.Count)
	case "target":
		pokemon := ownedPokemonFor(state, action.PokemonID)
		if pokemon == nil || !slices.Contains(pokemonTargetModes(state, pokemon), action.Mode) {
			return errors.New("该伙伴当前不能使用这个攻击优先级")
		}
		pokemon.Target = action.Mode
		return nil
	case "wave":
		return startWave(state)
	case "auto":
		state.AutoWave = !state.AutoWave
		if state.AutoWave && !state.WaveActive && state.Health > 0 && !state.Paused {
			return startWave(state)
		}
		return nil
	case "pause":
		if state.Paused {
			return errors.New("对局已经暂停")
		}
		state.Paused = true
		return nil
	case "resume":
		if !state.Paused {
			return errors.New("对局尚未暂停")
		}
		state.Paused = false
		return nil
	case "continueRoute":
		return continueCompletedRoute(state)
	case "speed":
		value := action.Value
		if value == 0 {
			options := []float64{0.8, 1.2, 1.7, 2, 2.5, 5, 0.4}
			index := slices.Index(options, state.Speed)
			value = options[(index+1)%len(options)]
		}
		if !validSpeed(value) {
			return errors.New("请选择有效速度")
		}
		state.Speed = value
		return nil
	case "retry":
		return retryDefeatedRoute(state, action.Mode)
	case "bell":
		if state.Route != 27 || !state.WaveActive || state.Wave == 50 || state.Wave == 100 {
			return errors.New("当前路线或波次不能使用普通钟声")
		}
		state.Progression.BellUses++
		pending := []EnemyPassiveEvent{}
		for index := range state.Enemies {
			enemy := &state.Enemies[index]
			WorldApplyConditions(state, enemy, AttackProfile{Conditions: []ConditionSpec{{Kind: "stun", Chance: 1, DurationMS: 1000 + int64(state.Wave)*40, Magnitude: 1, Stacks: 1}}}, 0)
			events, err := EnemyPassiveEvents(state, enemy, "bell", 0)
			if err != nil {
				return err
			}
			pending = append(pending, events...)
		}
		return executePassiveEvents(state, pending)
	default:
		return fmt.Errorf("未知操作 %s", action.Type)
	}
}

func terrainAllowed(pokemon Pokemon, species Species, terrain int) bool {
	if slices.Contains(species.Terrains, terrain) {
		return true
	}
	switch pokemon.Item {
	case "airBalloon":
		return terrain == 4
	case "heavyDutyBoots", "assaultVest":
		return terrain == 2
	case "dampMulch":
		return terrain == 1
	case "mitsuesCocktail":
		return terrain == 3
	}
	return false
}

func checkDeployingPartner(state *State, pokemon *Pokemon) error {
	if pokemon == nil || state.Health <= 0 || state.Progression.Draft.Active || !resultActionAllowed(state.Phase, "deploy") {
		return errors.New("请选择编队中的伙伴")
	}
	if state.Paused && WorldSpecies(state, pokemon).AbilityID != "timeWarp" {
		return errors.New("暂停期间不能调整该伙伴")
	}
	return nil
}

func checkDeploymentSite(state *State, pokemon *Pokemon, placement Placement) (string, error) {
	override, err := WorldPlacementAllowed(state, pokemon, placement)
	if err != nil {
		return "blocked", err
	}
	if override {
		return "host", nil
	}
	if !terrainAllowed(*pokemon, WorldSpecies(state, pokemon), placement.Terrain) {
		return "blocked", errors.New("该伙伴不能部署在这类地形")
	}
	for _, tower := range state.Towers {
		if tower.TileID == placement.ID && tower.PokemonID != pokemon.ID {
			return "occupied", errors.New("部署位置已被占用")
		}
	}
	return "empty", nil
}

type DeploymentPreview struct {
	TileSize int            `json:"tileSize"`
	Sites    map[int]string `json:"sites"`
}

func ProjectDeployment(state *State, id int) DeploymentPreview {
	route, _ := routeForState(state)
	result := DeploymentPreview{TileSize: route.TileSize, Sites: make(map[int]string, len(route.Placements))}
	if result.TileSize <= 0 {
		result.TileSize = 24
	}
	pokemon := pokemonFor(state, id)
	allowed := checkDeployingPartner(state, pokemon) == nil
	for _, tile := range route.Placements {
		kind := "blocked"
		if allowed {
			kind, _ = checkDeploymentSite(state, pokemon, tile)
		}
		result.Sites[tile.ID] = kind
	}
	return result
}

type DeploymentRange struct {
	Center     Point   `json:"center"`
	Range      float64 `json:"range"`
	Shape      string  `json:"shape"`
	InnerRange float64 `json:"innerRange"`
	HalfWidth  float64 `json:"halfWidth"`
}

func ProjectDeploymentRange(state *State, id, tileID int) (DeploymentRange, error) {
	candidate, err := Clone(state)
	if err != nil {
		return DeploymentRange{}, err
	}
	if err := deployPokemon(candidate, id, tileID); err != nil {
		return DeploymentRange{}, err
	}
	pokemon := pokemonFor(candidate, id)
	tower := worldTowerFor(candidate, id)
	profile, err := profileFor(candidate, tower, pokemon, nil, .5)
	if err != nil {
		return DeploymentRange{}, err
	}
	species := WorldSpecies(candidate, pokemon)
	shape := combatRangeType(species, *pokemon, profile)
	inner := species.InnerRange
	if profile.InnerRange > 0 {
		inner = profile.InnerRange
	}
	return DeploymentRange{Center: Point{X: tower.X, Y: tower.Y}, Range: profile.Range, Shape: shape, InnerRange: inner, HalfWidth: combatRangeHalfWidth(candidate, *pokemon, shape)}, nil
}

func deployPokemon(state *State, id, tileID int) error {
	pokemon := pokemonFor(state, id)
	if err := checkDeployingPartner(state, pokemon); err != nil {
		return err
	}
	route, _ := routeForState(state)
	var placement *Placement
	for index := range route.Placements {
		if route.Placements[index].ID == tileID {
			placement = &route.Placements[index]
			break
		}
	}
	if placement == nil {
		return errors.New("该伙伴不能部署在这类地形")
	}
	if _, err := checkDeploymentSite(state, pokemon, *placement); err != nil {
		return err
	}
	tileSize := route.TileSize
	if tileSize <= 0 {
		tileSize = 24
	}
	if pokemon.TileID >= 0 {
		if err := WorldOnRetire(state, id); err != nil {
			return err
		}
	}
	for index := range state.Towers {
		if state.Towers[index].PokemonID == id {
			state.Towers[index].TileID = tileID
			state.Towers[index].X = placement.X + float64(tileSize)/2
			state.Towers[index].Y = placement.Y + float64(tileSize)/2
			pokemon.TileID = tileID
			if err := WorldOnDeploy(state, id, tileID); err != nil {
				return err
			}
			if err := OnAttackModeDeploy(state, &state.Towers[index], pokemon); err != nil {
				return err
			}
			if err := syncOrbitalFor(state, id); err != nil {
				return err
			}
			recordDexDeploy(&state.Towers[index], pokemon)
			addEvent(state, Event{Type: "move", Source: id, X: state.Towers[index].X, Y: state.Towers[index].Y})
			return nil
		}
	}
	pokemon.TileID = tileID
	tower := Tower{ID: allocateID(state), PokemonID: id, TileID: tileID, X: placement.X + float64(tileSize)/2, Y: placement.Y + float64(tileSize)/2, Stacks: map[string]float64{}}
	state.Towers = append(state.Towers, tower)
	if err := WorldOnDeploy(state, id, tileID); err != nil {
		return err
	}
	if err := OnAttackModeDeploy(state, &state.Towers[len(state.Towers)-1], pokemon); err != nil {
		return err
	}
	if err := syncOrbitalFor(state, id); err != nil {
		return err
	}
	recordDexDeploy(&state.Towers[len(state.Towers)-1], pokemon)
	addEvent(state, Event{Type: "deploy", Source: id, X: tower.X, Y: tower.Y})
	return nil
}

func retirePokemon(state *State, id int) error {
	pokemon := pokemonFor(state, id)
	if pokemon == nil || pokemon.TileID < 0 {
		return errors.New("该伙伴尚未部署")
	}
	if err := WorldOnRetire(state, id); err != nil {
		return err
	}
	OnAttackModeRetire(state, id)
	pokemon.TileID = -1
	state.Towers = slices.DeleteFunc(state.Towers, func(tower Tower) bool { return tower.PokemonID == id })
	state.Projectiles = slices.DeleteFunc(state.Projectiles, func(projectile Projectile) bool { return projectile.PokemonID == id })
	addEvent(state, Event{Type: "retire", Source: id})
	return nil
}

func upgradePokemon(state *State, id, requested int) error {
	pokemon := ownedPokemonFor(state, id)
	if pokemon == nil || state.Paused || state.Health <= 0 || state.Progression.Challenge.Active && state.Progression.Challenge.LevelCap > 0 {
		return errors.New("当前不能升级该伙伴")
	}
	if requested == 0 {
		requested = 1
	}
	if requested != 1 && requested != 5 && requested != 10 || pokemon.Level >= 100 {
		return errors.New("请选择升级 1、5 或 10 级")
	}
	requested = min(requested, 100-pokemon.Level)
	oldKey := pokemon.Species
	candidate := *pokemon
	var cost int64
	prices := CatalogData().UpgradeCosts[ProgressionSpecies(state, pokemon).CostScale]
	for level := pokemon.Level; level < pokemon.Level+requested; level++ {
		cost += prices[level]
	}
	if state.Progression.Reset[pokemon.ID] {
		cost = 0
	}
	for index := 0; index < requested; index++ {
		candidate.Level++
		for {
			species := CatalogData().Species[candidate.Species]
			if species.EvolutionKey == "" || candidate.Level < species.EvolutionLevel || species.EvolutionLevel <= 0 {
				break
			}
			if species.ID == 95 && candidate.Item != "inverter" && !state.Progression.Reset[candidate.ID] {
				break
			}
			key := species.EvolutionKey
			if candidate.Species == "rockruff" {
				hour := (state.Now/3600000 + state.Clock/3600000 + 8) % 24
				if hour < 8 || hour >= 18 {
					key = "lycanrocNight"
				}
			}
			if _, ok := CatalogData().Species[key]; !ok {
				return errors.New("进化资料尚未加载")
			}
			candidate.Species = key
		}
		if err := refreshPokemon(state, &candidate); err != nil {
			return err
		}
	}
	if cost < 0 || state.Gold < cost {
		return errors.New("金币不足")
	}
	state.Gold -= cost
	*pokemon = candidate
	OnProgressionPokemonChanged(state, id, oldKey)
	addEvent(state, Event{Type: "upgrade", Source: id, Amount: float64(cost), Key: candidate.Species})
	return nil
}

func changeRoute(state *State, id int) error {
	route, ok := routeFor(id)
	if !ok || len(route.Waves) == 0 || state.WaveActive || state.Paused || state.Stars < route.UnlockStars || !progressionSecretRouteAvailable(state, route) {
		return errors.New("该路线尚未开放或当前仍在战斗")
	}
	state.Route = id
	state.Wave = max(1, state.RouteWaves[id])
	state.Weather = route.WeatherID
	state.Towers = []Tower{}
	state.Enemies = []Enemy{}
	state.Projectiles = []Projectile{}
	state.Health = state.MaxHealth
	state.Phase = "prepare"
	resetWaveDamage(state)
	for index := range state.Team {
		state.Team[index].TileID = -1
	}
	state.Epoch++
	return nil
}

func routeForState(state *State) (RouteSpec, bool) {
	if state.Progression.CustomRoute != nil {
		return *state.Progression.CustomRoute, true
	}
	return routeFor(state.Route)
}

func rounded(value float64) float64 {
	return math.Round(value*1000000) / 1000000
}
