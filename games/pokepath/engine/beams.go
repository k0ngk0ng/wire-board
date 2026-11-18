package engine

func isBeamKind(kind string) bool {
	return kind == "beam" || kind == "trackingBeam" || kind == "orthogonalBeam" || kind == "linkBeam"
}

func onlyAttachedBeam(abilityID string) bool {
	return abilityID == "signalBeam" || abilityID == "cryogenicCannon"
}

func hasAttachedBeam(state *State, pokemonID int, abilityID string) bool {
	if !onlyAttachedBeam(abilityID) {
		return false
	}
	for _, projectile := range state.Projectiles {
		if projectile.PokemonID == pokemonID && projectile.Context.AbilityID == abilityID {
			return true
		}
	}
	return false
}

func configureBeamProjectile(projectile *Projectile) {
	width := 24.0
	projectile.RayLength = 2000
	switch projectile.Context.AbilityID {
	case "hyperBeam":
		width = 50
	case "iceBeam", "machBolt", "psybeam":
		width = 20
	case "zapCannon", "meteorBeam":
		width = 8
	case "hydroCannon":
		width, projectile.RayLength = 18, max(100, projectile.Profile.Range)
	case "moongeistBeam":
		width, projectile.RayLength = 14, max(100, projectile.Profile.Range)
	case "steamEruption":
		width = 15
	case "signalBeam":
		width = 5
	case "cryogenicCannon":
		width = 3
	}
	if projectile.Kind == "orthogonalBeam" {
		projectile.RayLength = projectile.Profile.Range
	}
	projectile.BeamWidth = width * projectile.Profile.BeamWidthScale
	if projectile.Profile.BeamDurationMS == 0 {
		projectile.Profile.BeamDurationMS = 260
	}
	if projectile.Profile.BeamTickMS == 0 {
		projectile.Profile.BeamTickMS = 140
	}
	if onlyAttachedBeam(projectile.Context.AbilityID) {
		projectile.Profile.BeamDurationMS = -1
	}
}

func updateBeamEndpoint(state *State, projectile *Projectile) bool {
	if projectile.Kind == "trackingBeam" {
		target := enemyFor(state, projectile.TargetID)
		attached := onlyAttachedBeam(projectile.Context.AbilityID)
		if attached && (target == nil || target.HP <= 0 || target.Dead || target.Escaped) {
			return false
		}
		if target != nil && target.HP > 0 && !target.Dead && !target.Escaped {
			if attached {
				tower := worldTowerFor(state, projectile.PokemonID)
				if tower == nil || vectorLength(target.X-tower.X, target.Y-tower.Y) >= projectile.Profile.Range+20 {
					return false
				}
			}
			dx, dy := target.X-projectile.X, target.Y-projectile.Y
			if distance := vectorLength(dx, dy); distance > 0 {
				projectile.RayDX, projectile.RayDY = dx/distance, dy/distance
				if attached {
					projectile.RayLength = distance
				}
			}
		}
	}
	dx := float64(projectile.RayDX * projectile.RayLength)
	dy := float64(projectile.RayDY * projectile.RayLength)
	projectile.TargetX, projectile.TargetY = projectile.X+dx, projectile.Y+dy
	return true
}

func beamRayHitIDs(state *State, projectile *Projectile, origin, end Point) []int {
	ids := []int{}
	dx, dy := end.X-origin.X, end.Y-origin.Y
	lengthSquared := dx*dx + dy*dy
	for _, enemy := range state.Enemies {
		if enemy.HP <= 0 || enemy.Dead || enemy.Escaped || enemy.InvulnerableMS > 0 {
			continue
		}
		if onlyAttachedBeam(projectile.Context.AbilityID) {
			if enemy.ID == projectile.TargetID {
				ids = append(ids, enemy.ID)
			}
			continue
		}
		if enemy.Invisible && !projectile.Profile.CanSeeInvisible {
			continue
		}
		t := 0.0
		if lengthSquared > 0 {
			t = max(0, min(1, ((enemy.X-origin.X)*dx+(enemy.Y-origin.Y)*dy)/lengthSquared))
		}
		point := Point{X: origin.X + t*dx, Y: origin.Y + t*dy}
		tolerance := projectile.BeamWidth/2 + max(enemy.Sprite.FrameWidth, enemy.Sprite.FrameHeight)/4
		if vectorLength(enemy.X-point.X, enemy.Y-point.Y) <= tolerance {
			ids = append(ids, enemy.ID)
		}
	}
	return ids
}
