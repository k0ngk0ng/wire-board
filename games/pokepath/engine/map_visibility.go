package engine

import "errors"

func progressionMapOpen(state *State) error {
	if state.Paused || state.Progression.Challenge.Active {
		return errors.New("暂停或挑战期间不能打开地图")
	}
	state.Progression.MirageVisible = state.Progression.SecretMaps[30] || NextRandom(state) < .01
	return nil
}

func progressionSecretRouteAvailable(state *State, route RouteSpec) bool {
	return !route.IsSecret || state.Progression.SecretMaps[route.ID] || route.ID == 30 && state.Progression.MirageVisible
}
