package engine

import (
	"math"
	"slices"
)

func ordinaryHoming(projectile Projectile) bool {
	return !projectile.Profile.Returning && !isRay(projectile) && slices.Contains([]string{"single", "criticalRicochet", "webProjectile"}, projectile.Kind)
}

func ordinaryLaunchOrigin(state *State, tower *Tower, projectile Projectile) Point {
	route, _ := routeForState(state)
	cellSize := route.TileSize
	if cellSize <= 0 {
		cellSize = 24
	}
	position := Point{X: tower.X - float64(cellSize)/2, Y: tower.Y - float64(cellSize)/2}
	if value := state.World.Towers[tower.PokemonID]; value != nil && value.OrbitPosition != nil {
		if owner := pokemonFor(state, tower.PokemonID); owner != nil && WorldSpecies(state, owner).AbilityID == "aerialAce" {
			position = *value.OrbitPosition
		}
	}
	y := position.Y - 6 + projectile.Sprite.FrameHeight/2
	if state.World.PassengerHosts[tower.PokemonID] != 0 {
		y -= 13
	}
	return Point{X: position.X - 6 + projectile.Sprite.FrameWidth/2, Y: y}
}

func advanceOrdinaryHoming(state *State, projectile *Projectile, target *Enemy) bool {
	dx, dy := target.X-projectile.X, target.Y-projectile.Y
	distance := vectorLength(dx, dy)
	radius := max(8, target.Sprite.FrameWidth/2)
	travel := projectile.Speed * battleStepSeconds(state)
	if travel <= 0 {
		return distance < radius
	}
	stepCount := max(1, math.Ceil(travel/max(2, radius/2)))
	spacing := travel / stepCount
	first := max(1, math.Floor((distance-radius)/spacing)+1)
	contact := first <= stepCount && math.Abs(distance-first*spacing) < radius
	if contact {
		travel = first * spacing
	}
	if distance == 0 {
		projectile.X = rounded(projectile.X + travel)
	} else {
		stepX, stepY := float64(dx/distance*travel), float64(dy/distance*travel)
		projectile.X = rounded(projectile.X + stepX)
		projectile.Y = rounded(projectile.Y + stepY)
	}
	return contact
}

func relaunchRicochet(state *State, projectile *Projectile, origin Point, target *Enemy) {
	projectile.ID = allocateID(state)
	dx := (NextRandom(state) - .5) * 6
	dy := (NextRandom(state) - .5) * 6
	projectile.X = rounded(origin.X + dx + projectile.Sprite.FrameWidth/2)
	projectile.Y = rounded(origin.Y + dy + projectile.Sprite.FrameHeight/2)
	projectile.TargetID, projectile.TargetX, projectile.TargetY = target.ID, target.X, target.Y
	projectile.LifeMS = 0
	projectile.Critical, projectile.RolledCritical = false, false
	projectile.HitIDs = []int{}
}

func syncOrbitals(state *State) error {
	for index := range state.Towers {
		if err := syncOrbitalFor(state, state.Towers[index].PokemonID); err != nil {
			return err
		}
	}
	return nil
}

func syncOrbitalFor(state *State, pokemonID int) error {
	pokemon := pokemonFor(state, pokemonID)
	tower := worldTowerFor(state, pokemonID)
	if pokemon == nil || tower == nil {
		return nil
	}
	species := WorldSpecies(state, pokemon)
	if species.AttackType != "orbital" {
		state.Projectiles = slices.DeleteFunc(state.Projectiles, func(projectile Projectile) bool {
			return projectile.PokemonID == pokemonID && projectile.Kind == "orbit"
		})
		return nil
	}
	profile, err := profileFor(state, tower, pokemon, nil, .5)
	if err != nil {
		return err
	}
	count := int(factNumber(species.Facts, "orbital")) + profile.OrbitalCount
	existing := 0
	for _, projectile := range state.Projectiles {
		if projectile.PokemonID == pokemonID && projectile.Kind == "orbit" {
			existing++
		}
	}
	if existing == count {
		return nil
	}
	state.Projectiles = slices.DeleteFunc(state.Projectiles, func(projectile Projectile) bool {
		return projectile.PokemonID == pokemonID && projectile.Kind == "orbit"
	})
	for index := 0; index < count; index++ {
		angle := float64(index) * 2 * math.Pi / float64(count)
		projectile := Projectile{ID: allocateID(state), TowerID: tower.ID, PokemonID: pokemonID, X: tower.X + math.Cos(angle)*pokemon.Stats.Range, Y: tower.Y + math.Sin(angle)*pokemon.Stats.Range, Kind: "orbit", OrbitRadius: pokemon.Stats.Range, Angle: angle, Profile: profile, DamageScale: 1, HitAt: map[int]int64{}, Sprite: CatalogData().Projectiles[species.ProjectileID].Sprite}
		state.Projectiles = append(state.Projectiles, projectile)
	}
	return nil
}

func advanceOrbital(state *State, projectile *Projectile) (bool, error) {
	tower := worldTowerFor(state, projectile.PokemonID)
	owner := pokemonFor(state, projectile.PokemonID)
	if tower == nil || owner == nil || WorldSpecies(state, owner).AttackType != "orbital" {
		return false, nil
	}
	species := WorldSpecies(state, owner)
	profile, err := profileFor(state, tower, owner, nil, .5)
	if err != nil {
		return false, err
	}
	speed := math.Pi / 5 * profile.OrbitSpeed
	if species.AbilityID == "shiftGear" {
		speed = min(5, tower.Stacks["orbitalSpeed"])
	}
	if owner.Item == "inverter" {
		speed = -math.Abs(speed)
	}
	if owner.Item == "oddKeystone" {
		previous := (projectile.LifeMS - battleStepWholeMS(state)) / 200
		current := projectile.LifeMS / 200
		projectile.OrbitRadius = max(70, projectile.OrbitRadius-2*float64(current-previous))
	} else {
		projectile.OrbitRadius = owner.Stats.Range
	}
	interval := int64(350)
	steps := max(1, int(math.Ceil(math.Abs(speed)*battleStepSeconds(state)*projectile.OrbitRadius/4)))
	if species.AbilityID == "sacredFire" {
		interval, steps = 500, 1
	}
	for index := 0; index < steps; index++ {
		projectile.Angle += speed * battleStepSeconds(state) / float64(steps)
		point := Point{X: tower.X + math.Cos(projectile.Angle)*projectile.OrbitRadius, Y: tower.Y + math.Sin(projectile.Angle)*projectile.OrbitRadius}
		if species.AbilityID == "sacredFire" {
			chaseSpeed := 120.0
			var nearest *Enemy
			best := profile.Range
			for enemyIndex := range state.Enemies {
				enemy := &state.Enemies[enemyIndex]
				if enemy.HP <= 0 || enemy.Dead || enemy.Escaped || enemy.InvulnerableMS > 0 || enemy.Invisible && !profile.CanSeeInvisible {
					continue
				}
				if distance := vectorLength(enemy.X-tower.X, enemy.Y-tower.Y); distance < best {
					nearest, best = enemy, distance
				}
			}
			if nearest != nil {
				point, chaseSpeed = Point{X: nearest.X, Y: nearest.Y}, 220
			}
			projectile.X, projectile.Y = pointToward(Point{X: projectile.X, Y: projectile.Y}, point, chaseSpeed*battleStepSeconds(state))
			maximum := max(0, profile.Range-max(projectile.Sprite.FrameWidth, projectile.Sprite.FrameHeight)/2)
			if distance := vectorLength(projectile.X-tower.X, projectile.Y-tower.Y); distance > maximum {
				projectile.X = tower.X + (projectile.X-tower.X)/distance*maximum
				projectile.Y = tower.Y + (projectile.Y-tower.Y)/distance*maximum
			}
		} else {
			projectile.X, projectile.Y = point.X, point.Y
		}
		for enemyIndex := range state.Enemies {
			enemy := &state.Enemies[enemyIndex]
			if enemy.HP <= 0 || enemy.Dead || enemy.Escaped || enemy.InvulnerableMS > 0 || enemy.Invisible && !profile.CanSeeInvisible {
				continue
			}
			radius := max(6, projectile.Sprite.FrameWidth/2) + 2
			if species.AbilityID == "sacredFire" {
				radius = projectile.Sprite.FrameWidth/2 + 8
			}
			if vectorLength(enemy.X-projectile.X, enemy.Y-projectile.Y) > radius || projectile.LifeMS-projectile.HitAt[enemy.ID] < interval {
				continue
			}
			if err := refreshMotionAttack(state, projectile, tower, owner, enemy); err != nil {
				return false, err
			}
			if err := applyProjectileHit(state, projectile, enemy, 1); err != nil {
				return false, err
			}
			projectile.HitAt[enemy.ID] = projectile.LifeMS
			if species.AbilityID == "shiftGear" {
				gain := .03
				if owner.Item == "lustrousOrb" {
					gain = .06
				}
				tower.Stacks["orbitalSpeed"] = min(5, tower.Stacks["orbitalSpeed"]+gain)
			}
			if owner.Item == "oddKeystone" {
				for otherIndex := range state.Projectiles {
					other := &state.Projectiles[otherIndex]
					if other.PokemonID == owner.ID && other.Kind == "orbit" {
						other.OrbitRadius = min(100, other.OrbitRadius+3)
					}
				}
				projectile.OrbitRadius = min(100, projectile.OrbitRadius+3)
			}
		}
	}
	return true, nil
}

func refreshMotionAttack(state *State, projectile *Projectile, tower *Tower, owner *Pokemon, enemy *Enemy) error {
	context := contextFor(state, tower, owner, enemy, NextRandom(state))
	source, err := sourceProfileFor(state, tower, owner, enemy, context.Random)
	if err != nil {
		return err
	}
	profile := source
	if err := WorldAdjustProfile(state, tower, owner, enemy, &profile); err != nil {
		return err
	}
	projectile.Context, projectile.SourceProfile, projectile.Profile = context, source, profile
	return nil
}

func advanceReturning(state *State, projectile *Projectile) (bool, error) {
	tower := worldTowerFor(state, projectile.PokemonID)
	owner := pokemonFor(state, projectile.PokemonID)
	if tower == nil || owner == nil || projectile.LifeMS >= 6000 {
		return false, nil
	}
	if projectile.MotionPhase == "" {
		projectile.MotionPhase = "outbound"
	}
	step := projectile.Speed * battleStepSeconds(state)
	steps := max(1, int(math.Ceil(step/6)))
	for index := 0; index < steps; index++ {
		amount := step / float64(steps)
		if projectile.MotionPhase == "outbound" {
			target := enemyFor(state, projectile.TargetID)
			if target == nil || target.HP <= 0 || target.Escaped {
				projectile.MotionPhase = "return"
			} else if vectorLength(target.X-projectile.X, target.Y-projectile.Y) <= max(8, target.Sprite.FrameWidth/2) {
				distance := max(1, vectorLength(target.X-projectile.X, target.Y-projectile.Y))
				projectile.RayDX, projectile.RayDY = (target.X-projectile.X)/distance, (target.Y-projectile.Y)/distance
				projectile.MotionPhase, projectile.RemainingDistance = "overshoot", 50
			} else {
				projectile.X, projectile.Y = pointToward(Point{X: projectile.X, Y: projectile.Y}, Point{X: target.X, Y: target.Y}, amount)
			}
		}
		if projectile.MotionPhase == "overshoot" {
			distance := min(amount, projectile.RemainingDistance)
			projectile.X += projectile.RayDX * distance
			projectile.Y += projectile.RayDY * distance
			projectile.RemainingDistance -= distance
			if projectile.RemainingDistance <= 0 {
				projectile.MotionPhase = "return"
			}
		}
		if projectile.MotionPhase == "return" {
			if vectorLength(projectile.X-tower.X, projectile.Y-tower.Y) <= 8 {
				return false, nil
			}
			projectile.X, projectile.Y = pointToward(Point{X: projectile.X, Y: projectile.Y}, Point{X: tower.X, Y: tower.Y}, amount)
		}
		for enemyIndex := range state.Enemies {
			enemy := &state.Enemies[enemyIndex]
			if enemy.HP <= 0 || enemy.Dead || enemy.Escaped || enemy.InvulnerableMS > 0 || enemy.Invisible && !projectile.Profile.CanSeeInvisible || vectorLength(enemy.X-projectile.X, enemy.Y-projectile.Y) > max(10, enemy.Sprite.FrameWidth/2) || projectile.LifeMS-projectile.HitAt[enemy.ID] < 220 {
				continue
			}
			if err := refreshMotionAttack(state, projectile, tower, owner, enemy); err != nil {
				return false, err
			}
			if err := applyProjectileHit(state, projectile, enemy, 1); err != nil {
				return false, err
			}
			projectile.HitAt[enemy.ID] = projectile.LifeMS
		}
	}
	return true, nil
}

func pointToward(origin, target Point, distance float64) (float64, float64) {
	dx, dy := target.X-origin.X, target.Y-origin.Y
	length := vectorLength(dx, dy)
	if length <= distance || length == 0 {
		return target.X, target.Y
	}
	return origin.X + dx/length*distance, origin.Y + dy/length*distance
}
