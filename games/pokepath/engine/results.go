package engine

import "errors"

func resultActionAllowed(phase, action string) bool {
	switch phase {
	case "route_complete":
		return action == "continueRoute"
	case "defeat":
		return action == "retry"
	case "practice_complete":
		return action == "custom_retry" || action == "custom_exit"
	default:
		return true
	}
}

func recordRouteResultForDeployed(state *State) {
	count := 7
	if rules := state.Progression.Challenge; rules.Active {
		count = 0
		for _, enabled := range [...]bool{rules.LevelCap > 0, rules.SlotLimit > 0, rules.ToughEnemies > 0, rules.Draft, rules.NoItems, rules.Permadeath, rules.Mirror} {
			if enabled {
				count++
			}
		}
	}
	for _, tower := range state.Towers {
		if owner := pokemonFor(state, tower.PokemonID); owner != nil {
			owner.Dex.Challenges += count
		}
	}
}

func resumeRouteAfterResult(state *State, wave, health int) error {
	route, ok := routeForState(state)
	if !ok {
		return errors.New("当前结果无法返回路线")
	}
	orbitals := persistentProjectiles(state)
	passengers := state.World.PassengerHosts
	clearEditorBattle(state, true)
	state.Projectiles = orbitals
	state.World.PassengerHosts = passengers
	resetWaveDamage(state)
	state.Health, state.MaxHealth = health, 14
	state.Wave = wave
	state.RouteWaves[state.Route] = wave
	state.Progression.RouteHealth[state.Route] = health
	state.Phase = "prepare"
	state.Paused = false
	state.Weather, state.World.AppliedWeather = route.WeatherID, route.WeatherID
	state.WeatherUntil = 0
	state.Epoch++
	return worldUpdateWeather(state)
}

func continueCompletedRoute(state *State) error {
	if state.Phase != "route_complete" || state.WaveActive || state.Health <= 0 || state.Progression.CustomRoute != nil {
		return errors.New("当前没有可确认的路线胜利")
	}
	if state.Progression.Challenge.Active {
		if err := progressionEndChallenge(state); err != nil {
			return err
		}
	}
	if err := resumeRouteAfterResult(state, 1, 14); err != nil {
		return err
	}
	switch state.Route {
	case 20:
		GrantProgressionSecret(state, "manaphy")
	case 30:
		updateDexEnemy(state, "mew", 1, 0, 0)
		GrantProgressionSecret(state, "mew")
	case 31:
		GrantProgressionSecret(state, "victini")
	case 32:
		GrantProgressionSecret(state, "marshadow")
	case 33:
		GrantProgressionSecret(state, "farfetchdGalar")
	case 35:
		GrantProgressionSecret(state, "calyrex")
	case 46:
		GrantProgressionSecret(state, "spectrier")
	}
	return nil
}

func retryDefeatedRoute(state *State, mode string) error {
	if state.Health > 0 || state.Phase != "defeat" {
		return errors.New("当前对局不能重试")
	}
	wave, health := 1, 14
	if mode == "" && state.Wave >= 25 {
		mode = "retry"
	}
	switch mode {
	case "", "restart":
	case "retry", "checkpoint":
		if state.Wave < 25 {
			return errors.New("第 25 波起可以从检查点重试")
		}
		wave = min(75, state.Wave/25*25)
		health = 7
		for _, threshold := range [...]int{150, 300, 450} {
			if state.Stars >= threshold {
				health++
			}
		}
		if state.Wave == 100 {
			wave, health = 100, 1
		}
	default:
		return errors.New("请选择重开或检查点重试")
	}
	if err := resumeRouteAfterResult(state, wave, health); err != nil {
		return err
	}
	state.Progression.Profile.Resets++
	return nil
}
