package engine

import (
	"math"
	"slices"
	"sort"
)

type aiCandidate struct {
	pokemon Pokemon
	profile AttackProfile
	value   float64
}

func aiAction(state *State, action Action) *Action {
	action.Epoch = state.Epoch
	return &action
}

func aiReferenceLevel(state *State) int {
	level := 40
	for _, collection := range [][]Pokemon{state.Team, state.Box} {
		for _, pokemon := range collection {
			level = max(level, pokemon.Level)
		}
	}
	return min(80, level)
}

func aiProfile(state *State, pokemon Pokemon, level, terrain int) (AttackProfile, bool) {
	species := ProgressionSpecies(state, &pokemon)
	if species.Key == "" {
		return AttackProfile{}, false
	}
	for step := 0; step < 6 && species.EvolutionLevel > 0 && species.EvolutionLevel <= level && species.EvolutionKey != ""; step++ {
		next, found := CatalogData().Species[species.EvolutionKey]
		if !found {
			break
		}
		species = next
	}
	if terrain == 0 {
		terrain = species.Terrains[0]
		if slices.Contains(species.Terrains, 2) && species.AbilityID == "ambusher" {
			terrain = 2
		}
		if slices.Contains(species.Terrains, 3) && species.AbilityID == "swimmer" {
			terrain = 3
		}
		if slices.Contains(species.Terrains, 4) && slices.Contains([]string{"vigilant", "vigilantFrisk"}, species.AbilityID) {
			terrain = 4
		}
	}
	if state.Progression.Challenge.Active && state.Progression.Challenge.LevelCap > 0 {
		level = state.Progression.Challenge.LevelCap
	}
	capacity := ProgressionTeamCapacity(state)
	context := AbilityContext{AbilityID: species.AbilityID, ItemID: pokemon.Item, WeatherID: state.Weather, Level: level, Terrain: terrain, Stars: state.Stars, Hearts: state.Health, MaxHearts: state.MaxHealth, Targets: 1, NearbyAllies: min(3, max(0, capacity-1)), Power: math.Floor(species.PowerBase + species.PowerPerLevel*float64(level)), Range: species.RangeBase + species.RangePerLevel*float64(level), AttackMS: max(80, species.AttackMSBase+species.AttackMSPerLevel*float64(level)), CriticalPct: species.CriticalBase + species.CriticalPerLevel*float64(level), CriticalDamage: 1.5, AttackType: species.AttackType, Shape: species.RangeType, SpeciesID: species.ID, EffectsVolume: state.Progression.Settings.Audio.Effects, MissingAllies: max(0, 10-capacity), Random: 0.5, Day: true}
	for _, key := range aiWave(state).Preview {
		if CatalogData().Enemies[key].IsBoss {
			context.SameTargetHits = 20
			break
		}
	}
	profile, err := BuildAttackProfile(context)
	return profile, err == nil
}

func aiWave(state *State) WaveSpec {
	route, exists := routeForState(state)
	if !exists {
		return WaveSpec{}
	}
	wave, _ := waveFor(route, state.Wave)
	return wave
}

func aiDenseWave(state *State) bool {
	count := 0
	for _, key := range aiWave(state).Sequence {
		if key == nil {
			continue
		}
		enemy := CatalogData().Enemies[*key]
		if enemy.Speed >= 90 && enemy.Health <= 2000 {
			count++
		}
	}
	return count >= 20
}

func aiHasCondition(profile AttackProfile, kind string) bool {
	return slices.ContainsFunc(profile.Conditions, func(condition ConditionSpec) bool { return condition.Kind == kind }) || profile.AuraCondition.Kind == kind
}

func aiValue(state *State, pokemon Pokemon, level int, dense bool) aiCandidate {
	profile, valid := aiProfile(state, pokemon, level, 0)
	if !valid {
		return aiCandidate{pokemon: pokemon}
	}
	interval := max(80, profile.AttackMS)
	if profile.BeamTickMS > 0 {
		interval = float64(profile.BeamTickMS)
	}
	multiplier := profile.DamageMultiplier
	if profile.TickDamageScale > 0 {
		multiplier *= profile.TickDamageScale
	}
	value := profile.Power * multiplier * 1000 / interval * float64(max(1, profile.Burst))
	value *= 1 + min(100, max(0, profile.CriticalPct))*max(0, profile.CriticalMultiplier-1)/100
	species := ProgressionSpecies(state, &pokemon)
	if species.AttackType == "area" || species.AttackType == "stream" || profile.SplashRadius > 0 {
		if dense {
			value *= 4
		} else {
			value *= 1.5
		}
	}
	if profile.Aura.PowerMultiplier > 1 {
		value += 180 * (profile.Aura.PowerMultiplier - 1) * float64(max(1, ProgressionTeamCapacity(state)-1))
	}
	if aiHasCondition(profile, "poison") {
		value += 90
	}
	if aiHasCondition(profile, "slow") || aiHasCondition(profile, "stun") {
		value += 60
	}
	if aiHasCondition(profile, "burn") {
		value += 40
	}
	for _, key := range aiWave(state).Preview {
		if !CatalogData().Enemies[key].IsBoss {
			continue
		}
		for _, condition := range profile.Conditions {
			if condition.Kind == "nightmare" {
				value += condition.Magnitude * min(20, 15000/interval)
			}
		}
		break
	}
	if profile.CanSeeInvisible {
		value += 30
	}
	if state.Health < state.MaxHealth/2 && profile.HeartChance > 0 {
		value += 80
	}
	value *= 0.8 + 0.2*min(1, float64(pokemon.Level)/float64(max(1, level)))
	return aiCandidate{pokemon: pokemon, profile: profile, value: value}
}

func aiUsableTile(state *State, pokemon Pokemon, tile Placement) bool {
	species := ProgressionSpecies(state, &pokemon)
	if !terrainAllowed(pokemon, species, tile.Terrain) {
		return false
	}
	for _, tower := range state.Towers {
		if tower.TileID == tile.ID && tower.PokemonID != pokemon.ID {
			return false
		}
	}
	return true
}

func aiCovers(profile AttackProfile, species Species, center, point Point) bool {
	dx, dy := math.Abs(center.X-point.X), math.Abs(center.Y-point.Y)
	distance := vectorLength(dx, dy)
	switch species.RangeType {
	case "cross":
		return dx <= profile.Range && dy <= 24 || dy <= profile.Range && dx <= 24
	case "horizontalLine":
		return dx <= profile.Range && dy <= 24
	case "xShape":
		return distance <= profile.Range && math.Abs(dx-dy) <= 30
	case "donut":
		return distance <= profile.Range && distance >= species.InnerRange
	default:
		return distance <= profile.Range
	}
}

func aiTileValue(state *State, pokemon Pokemon, tile Placement, live bool) float64 {
	profile, valid := aiProfile(state, pokemon, pokemon.Level, tile.Terrain)
	if !valid {
		return 0
	}
	species := ProgressionSpecies(state, &pokemon)
	center := Point{X: tile.X + 12, Y: tile.Y + 12}
	score := 0.0
	if live {
		for _, enemy := range state.Enemies {
			if enemy.Dead || enemy.Escaped || !enemy.Entered || enemy.HP <= 0 || enemy.Invisible && !profile.CanSeeInvisible {
				continue
			}
			if aiCovers(profile, species, center, Point{X: enemy.X, Y: enemy.Y}) {
				score += 1 + math.Pow(min(5, enemy.Distance/600), 3)
				if enemy.Boss {
					score += 200
				}
			}
		}
		return score
	}
	route, _ := routeForState(state)
	paths := route.Paths
	if ProgressionMirrored(state) && len(route.MirrorPaths) > 0 {
		paths = route.MirrorPaths
	}
	for _, points := range paths {
		for index := 1; index < len(points); index++ {
			from, to := points[index-1], points[index]
			length := vectorLength(to.X-from.X, to.Y-from.Y)
			steps := max(1, int(math.Ceil(length/30)))
			for step := 0; step < steps; step++ {
				fraction := (float64(step) + 0.5) / float64(steps)
				point := Point{X: from.X + (to.X-from.X)*fraction, Y: from.Y + (to.Y-from.Y)*fraction}
				if aiCovers(profile, species, center, point) {
					score += length / float64(steps)
				}
			}
		}
	}
	if species.AbilityID == "ambusher" && tile.Terrain == 2 || species.AbilityID == "swimmer" && tile.Terrain == 3 {
		score *= 1.7
	}
	if species.AttackType == "aura" {
		score = 1
		for _, tower := range state.Towers {
			if tower.PokemonID != pokemon.ID && vectorLength(tower.X-center.X, tower.Y-center.Y) <= profile.Range {
				score += 250
			}
		}
	}
	return score
}

func aiDeployment(state *State, live bool) *Action {
	if state.World.MovementLocked {
		return nil
	}
	route, exists := routeForState(state)
	if !exists {
		return nil
	}
	bestGain := 0.0
	var action *Action
	for _, pokemon := range state.Team {
		if live != (pokemon.TileID >= 0) {
			continue
		}
		if live && (ProgressionSpecies(state, &pokemon).AttackType == "aura" || slices.ContainsFunc(state.Projectiles, func(projectile Projectile) bool { return projectile.PokemonID == pokemon.ID })) {
			continue
		}
		current := 0.0
		for _, tile := range route.Placements {
			if tile.ID == pokemon.TileID {
				current = aiTileValue(state, pokemon, tile, live)
			}
		}
		for _, tile := range route.Placements {
			if tile.ID == pokemon.TileID || !aiUsableTile(state, pokemon, tile) {
				continue
			}
			score := aiTileValue(state, pokemon, tile, live)
			gain := score - current
			if live && score <= current*1.4+2 || score <= 0 {
				continue
			}
			if action == nil || gain > bestGain {
				bestGain = gain
				action = aiAction(state, Action{Type: "deploy", PokemonID: pokemon.ID, TileID: tile.ID})
			}
		}
		if !live && action != nil {
			return action
		}
	}
	return action
}

func aiFormation(state *State) *Action {
	if state.World.MovementLocked || state.WaveActive || state.Progression.Challenge.Active || len(state.Box) == 0 {
		return nil
	}
	level := aiReferenceLevel(state)
	dense := aiDenseWave(state)
	pool := make([]aiCandidate, 0, len(state.Team)+len(state.Box))
	route, _ := routeForState(state)
	for _, collection := range [][]Pokemon{state.Team, state.Box} {
		for _, pokemon := range collection {
			species := ProgressionSpecies(state, &pokemon)
			usable := slices.ContainsFunc(route.Placements, func(tile Placement) bool { return terrainAllowed(pokemon, species, tile.Terrain) })
			if usable {
				pool = append(pool, aiValue(state, pokemon, level, dense))
			}
		}
	}
	sort.Slice(pool, func(left, right int) bool {
		if pool[left].value == pool[right].value {
			return pool[left].pokemon.ID < pool[right].pokemon.ID
		}
		return pool[left].value > pool[right].value
	})
	poison, armor, invisible, stun := !dense, false, false, dense
	cleansingBoss := false
	for _, key := range aiWave(state).Preview {
		enemy := CatalogData().Enemies[key]
		poison = poison || enemy.CanPoison && enemy.Health >= 5000
		armor = armor || enemy.Armor >= 1000
		invisible = invisible || enemy.Invisible
		stun = stun || enemy.IsBoss && enemy.CanStun
		cleansingBoss = cleansingBoss || enemy.IsBoss && enemy.PassiveID == "naturalCure"
	}
	roles := []func(aiCandidate) bool{
		func(candidate aiCandidate) bool {
			return cleansingBoss && aiHasCondition(candidate.profile, "nightmare")
		},
		func(candidate aiCandidate) bool {
			return poison && !cleansingBoss && aiHasCondition(candidate.profile, "poison")
		},
		func(candidate aiCandidate) bool {
			return armor && (candidate.profile.ArmorPierce > 0 || candidate.profile.ArmorRemovePct > 0)
		},
		func(candidate aiCandidate) bool { return invisible && candidate.profile.CanSeeInvisible },
		func(candidate aiCandidate) bool { return aiHasCondition(candidate.profile, "slow") },
		func(candidate aiCandidate) bool { return aiHasCondition(candidate.profile, "burn") },
		func(candidate aiCandidate) bool {
			return stun && !cleansingBoss && aiHasCondition(candidate.profile, "stun")
		},
	}
	wanted := []aiCandidate{}
	capacity := ProgressionTeamCapacity(state)
	for _, role := range roles {
		if len(wanted) >= capacity || slices.ContainsFunc(wanted, role) {
			continue
		}
		for _, candidate := range pool {
			if role(candidate) && !slices.ContainsFunc(wanted, func(member aiCandidate) bool { return member.pokemon.ID == candidate.pokemon.ID }) {
				wanted = append(wanted, candidate)
				break
			}
		}
	}
	for _, candidate := range pool {
		if len(wanted) >= capacity {
			break
		}
		if !slices.ContainsFunc(wanted, func(member aiCandidate) bool { return member.pokemon.ID == candidate.pokemon.ID }) {
			wanted = append(wanted, candidate)
		}
	}
	for _, candidate := range wanted {
		if slices.ContainsFunc(state.Team, func(member Pokemon) bool { return member.ID == candidate.pokemon.ID }) {
			continue
		}
		for _, current := range state.Team {
			if !slices.ContainsFunc(wanted, func(member aiCandidate) bool { return member.pokemon.ID == current.ID }) {
				return aiAction(state, Action{Type: "team", PokemonID: current.ID, Mode: "box"})
			}
		}
		if len(state.Team) < capacity {
			return aiAction(state, Action{Type: "team", PokemonID: candidate.pokemon.ID, Mode: "team"})
		}
	}
	return nil
}

func aiUpgrade(state *State) (*Action, int64) {
	cheapest := int64(math.MaxInt64)
	var action *Action
	best := 0.0
	if state.Progression.Challenge.Active && state.Progression.Challenge.LevelCap > 0 {
		return nil, cheapest
	}
	eligible := append([]Pokemon{}, state.Team...)
	if state.Records[state.Route] >= 90 {
		for _, pokemon := range state.Box {
			profile, _ := aiProfile(state, pokemon, aiReferenceLevel(state), 0)
			if ProgressionSpecies(state, &pokemon).AbilityID == "focus" || aiHasCondition(profile, "nightmare") {
				eligible = append(eligible, pokemon)
			}
		}
	}
	for _, pokemon := range eligible {
		if pokemon.Level >= 100 || pokemon.UpgradeCost <= 0 {
			continue
		}
		cheapest = min(cheapest, pokemon.UpgradeCost)
		if pokemon.UpgradeCost > state.Gold {
			continue
		}
		value := aiValue(state, pokemon, pokemon.Level, false).value
		next := aiValue(state, pokemon, pokemon.Level+1, false).value
		gain := max(1, next-value) / float64(pokemon.UpgradeCost)
		species := ProgressionSpecies(state, &pokemon)
		if species.EvolutionLevel > pokemon.Level && species.EvolutionLevel-pokemon.Level <= 15 {
			prices := CatalogData().UpgradeCosts[species.CostScale]
			var total int64
			for level := pokemon.Level; level < species.EvolutionLevel && level < len(prices); level++ {
				total += prices[level]
			}
			if total > 0 {
				future := aiValue(state, pokemon, species.EvolutionLevel, false).value
				gain = max(gain, (future-value)/float64(total))
			}
		}
		if pokemon.TileID < 0 {
			gain *= 0.5
		}
		if action == nil || gain > best {
			best = gain
			action = aiAction(state, Action{Type: "upgrade", PokemonID: pokemon.ID, Count: 1})
		}
	}
	return action, cheapest
}

func aiEquipment(state *State, cheapest int64) *Action {
	if state.World.MovementLocked || state.Progression.Challenge.Active && state.Progression.Challenge.NoItems {
		return nil
	}
	for _, owned := range state.Inventory {
		item := CatalogData().Items[owned.ID]
		if item.MegaStone {
			continue
		}
		bestID := 0
		bestGain := 0.0
		for _, pokemon := range state.Team {
			if !progressionCanEquip(state, &pokemon, item) || pokemon.Item != "" && pokemon.Item != owned.ID {
				continue
			}
			plain := pokemon
			plain.Item = ""
			with := pokemon
			with.Item = owned.ID
			gain := aiValue(state, with, pokemon.Level, false).value - aiValue(state, plain, pokemon.Level, false).value
			if gain > bestGain {
				bestGain, bestID = gain, pokemon.ID
			}
		}
		if bestID > 0 && bestID != owned.EquippedBy {
			if holder := ownedPokemonFor(state, owned.EquippedBy); holder != nil && holder.TileID >= 0 && slices.Contains([]string{"silphScope", "airBalloon", "heavyDutyBoots", "dampMulch", "assaultVest", "twistedSpoon", "subwoofer", "mitsuesCocktail", "blimpKeys", "pokeSkates"}, owned.ID) {
				continue
			}
			return aiAction(state, Action{Type: "equip", PokemonID: bestID, ItemID: owned.ID})
		}
	}
	for stock, key := range state.Progression.ShopStock {
		item, exists := CatalogData().Items[key]
		if !exists || !item.Equipable || item.MegaStone || item.Price > state.Gold || item.Price > max(6000, min(int64(1000000), cheapest)*3) || progressionOwnsItem(state, key) {
			continue
		}
		for _, pokemon := range state.Team {
			if !progressionCanEquip(state, &pokemon, item) || pokemon.Item != "" {
				continue
			}
			with := pokemon
			with.Item = key
			if aiValue(state, with, pokemon.Level, false).value > aiValue(state, pokemon, pokemon.Level, false).value+1 {
				return aiAction(state, Action{Type: "buyItem", Stock: stock, ItemID: key})
			}
		}
	}
	return nil
}

func Propose(state *State) *Action {
	if state == nil || state.Paused || state.Phase == "practice_complete" || state.Phase == "route_complete" {
		return nil
	}
	if state.Health <= 0 {
		if state.Phase != "defeat" || state.Progression.Challenge.Permadeath {
			return nil
		}
		mode := "checkpoint"
		if state.Wave < 25 || state.Wave >= 90 {
			mode = "restart"
		}
		return aiAction(state, Action{Type: "retry", Mode: mode})
	}
	if state.Progression.Draft.Active {
		best, value := -1, -1.0
		for pick, id := range state.Progression.Draft.Offers {
			pokemon := ownedPokemonFor(state, id)
			if pokemon == nil {
				continue
			}
			candidate := aiValue(state, *pokemon, aiReferenceLevel(state), aiDenseWave(state)).value
			if candidate > value {
				best, value = pick, candidate
			}
		}
		if best >= 0 {
			return aiAction(state, Action{Type: "draftPick", Pick: best, TargetID: state.Progression.Draft.Token})
		}
		return nil
	}
	if state.Speed < 2.5 && state.Clock == 0 {
		return aiAction(state, Action{Type: "speed", Value: 2.5})
	}
	if action := aiFormation(state); action != nil {
		return action
	}
	if action := aiDeployment(state, false); action != nil {
		return action
	}
	upgrade, cheapest := aiUpgrade(state)
	if action := aiEquipment(state, cheapest); action != nil {
		return action
	}
	if state.WaveActive {
		if action := aiDeployment(state, true); action != nil {
			return action
		}
	}
	owned := len(state.Team) + len(state.Box)
	hasNightmare, hasFocus := false, false
	for _, collection := range [][]Pokemon{state.Team, state.Box} {
		for _, pokemon := range collection {
			profile, _ := aiProfile(state, pokemon, aiReferenceLevel(state), 0)
			hasNightmare = hasNightmare || aiHasCondition(profile, "nightmare")
			hasFocus = hasFocus || ProgressionSpecies(state, &pokemon).AbilityID == "focus"
		}
	}
	seekCounter := state.Records[state.Route] >= 90 && owned < 50 && (!hasNightmare || !hasFocus)
	seek := len(state.Team) < ProgressionTeamCapacity(state) || state.Wave >= 30 && owned < 21 || seekCounter
	if !state.Progression.Challenge.Active && seek && len(state.Progression.EggPool) > 0 && state.Progression.EggPrice <= state.Gold && (seekCounter || owned < 3 || state.Progression.EggPrice <= max(100, min(int64(1000000), cheapest)*2)) {
		return aiAction(state, Action{Type: "egg", Quote: len(state.Progression.EggPool)})
	}
	if upgrade != nil && !seekCounter {
		return upgrade
	}
	if !state.WaveActive {
		return aiAction(state, Action{Type: "wave"})
	}
	return nil
}
