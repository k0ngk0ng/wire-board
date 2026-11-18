package engine

func publicList[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

func projectList[T any](values []T, normalize func(*T) bool) []T {
	values = publicList(values)
	var result []T
	for index, value := range values {
		if normalize(&value) {
			if result == nil {
				result = append([]T{}, values...)
			}
			result[index] = value
		}
	}
	if result == nil {
		return values
	}
	return result
}

func projectPokemonLists(value *Pokemon) bool {
	if value.TargetModes == nil {
		value.TargetModes = []string{}
		return true
	}
	return false
}

func projectProfileLists(value *AttackProfile) bool {
	if value.Conditions == nil {
		value.Conditions = []ConditionSpec{}
		return true
	}
	return false
}

func projectProjectileLists(value *Projectile) bool {
	changed := false
	if value.HitIDs == nil {
		value.HitIDs, changed = []int{}, true
	}
	profile := projectProfileLists(&value.Profile)
	source := projectProfileLists(&value.SourceProfile)
	return changed || profile || source
}

func projectEffectLists(value *AttackModeEffect) bool {
	changed := false
	if value.HitIDs == nil {
		value.HitIDs, changed = []int{}, true
	}
	profile := projectProfileLists(&value.Profile)
	source := projectProfileLists(&value.SourceProfile)
	return changed || profile || source
}

func projectAttackModeLists(value AttackModeState) AttackModeState {
	value.Effects = projectList(value.Effects, projectEffectLists)
	var towers map[int]*AttackModeTower
	for id, tower := range value.Towers {
		if tower == nil || tower.Chain == nil || tower.Chain.HitIDs != nil {
			continue
		}
		if towers == nil {
			towers = make(map[int]*AttackModeTower, len(value.Towers))
			for key, existing := range value.Towers {
				towers[key] = existing
			}
		}
		copyTower, copyChain := *tower, *tower.Chain
		copyChain.HitIDs = []int{}
		copyTower.Chain = &copyChain
		towers[id] = &copyTower
	}
	if towers != nil {
		value.Towers = towers
	}
	return value
}

func projectProgressionLists(value *ProgressionView) {
	value.Shop = publicList(value.Shop)
	value.Draft.Pool = publicList(value.Draft.Pool)
	value.Draft.Offers = publicList(value.Draft.Offers)
	value.Draft.Picked = publicList(value.Draft.Picked)
	value.Formations = projectList(value.Formations, func(formation *Formation) bool {
		if formation.Members == nil {
			formation.Members = []FormationMember{}
			return true
		}
		return false
	})
	for route, claims := range value.Claims {
		value.Claims[route] = publicList(claims)
	}
	for id, growth := range value.Growth {
		growth.Forms = publicList(growth.Forms)
		growth.Appearances = publicList(growth.Appearances)
		value.Growth[id] = growth
	}
	value.Hotspots = publicList(value.Hotspots)
	value.Speeds = publicList(value.Speeds)
	value.RewardThresholds = publicList(value.RewardThresholds)
	if value.CustomData != nil && value.CustomData.Wave == nil {
		copy := *value.CustomData
		copy.Wave = []EditorSlot{}
		value.CustomData = &copy
	}
}

func projectViewLists(value View) View {
	value.Tiles = publicList(value.Tiles)
	value.Paths = projectList(value.Paths, func(path *[]Point) bool {
		if *path == nil {
			*path = []Point{}
			return true
		}
		return false
	})
	value.Team = projectList(value.Team, projectPokemonLists)
	value.Box = projectList(value.Box, projectPokemonLists)
	value.Items = publicList(value.Items)
	value.Towers = publicList(value.Towers)
	value.Enemies = projectList(value.Enemies, func(enemy *Enemy) bool {
		if enemy.Conditions == nil {
			enemy.Conditions = []Condition{}
			return true
		}
		return false
	})
	value.Projectiles = projectList(value.Projectiles, projectProjectileLists)
	value.Events = publicList(value.Events)
	value.AttackModes = projectAttackModeLists(value.AttackModes)
	return value
}
