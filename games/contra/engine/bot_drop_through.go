package contra

// Predict only from current visible platforms. Down+A skips the current
// one-way support; a lower known surface and an unobstructed body are required.
func (b *Bot) planDropThrough(o BotObservation) (int, int, bool) {
	p := o.Players[o.Actor]
	if o.terrainAt(p.X, p.Y+16) != TerrainPlatform || !o.terrainKnownAt(p.X, p.Y+16) {
		return 0, 0, false
	}
	// The original horizontal-stage admission checks the current column,
	// before any later lateral steering is allowed.
	if o.terrainAt(p.X, p.Y+32) == TerrainSolid {
		return 0, 0, false
	}
	supportBelow := false
	for y := p.Y + 32; y < 224; y += 16 {
		if o.terrainKnownAt(p.X, y) && o.terrainAt(p.X, y) != TerrainEmpty {
			supportBelow = true
			break
		}
	}
	if !supportBelow {
		return 0, 0, false
	}
	startRow := (p.Y + 16 + int(o.ViewY)) / 16
	best, bx, by, found := -1000000, p.X, p.Y, false
	for goal := max(28, p.X-32); goal <= min(228, p.X+32); goal += 4 {
		x, y, vy := float64(p.X), float64(p.Y), 0.0
		for tick := 0; tick < 64; tick++ {
			if y >= 216 || o.terrainAt(int(x), int(y)-10) == TerrainSolid || o.terrainAt(int(x), int(y)) == TerrainSolid {
				break
			}
			// stepFall checks current feet before adding gravity or moving X;
			// unlike stepJump it has no integer-speed/16px phase gate.
			if int(y) >= p.Y+20 && (int(y)+16+int(o.ViewY))/16 > startRow {
				kind := o.terrainAt(int(x), int(y)+16)
				if kind == TerrainWater {
					break
				}
				if (kind == TerrainPlatform || kind == TerrainSolid) && o.terrainKnownAt(int(x), int(y)+16) {
					landingY := ((int(y) + int(o.ViewY)) &^ 15) - int(o.ViewY) + 4
					score := -(landingY-p.Y)*4 - absBot(int(x)-p.X) - botNozzleLandingRisk(o, int(x), landingY)
					if score > best {
						best, bx, by, found = score, int(x), landingY, true
					}
					break
				}
			}
			vy += 35.0 / 256.0
			y += vy
			if y >= float64(p.Y+20) {
				if x < float64(goal) {
					x++
				} else if x > float64(goal) {
					x--
				}
			}
		}
	}
	return bx, by, found
}

func (b *Bot) blockedWallDrop(o BotObservation, fire Buttons) (Buttons, bool) {
	p := o.Players[o.Actor]
	if b.JumpCooldown != 0 || b.ViewStillTicks < 30 || o.terrainAt(p.X, p.Y+16) != TerrainPlatform {
		return 0, false
	}
	wall := false
	for _, dx := range []int{8, 12, 16} {
		wall = wall || o.terrainAt(p.X+dx, p.Y-4) == TerrainSolid
	}
	if !wall {
		return 0, false
	}
	x, y, _, canJump := b.planJump(o, false)
	// A nearly vertical first jump can reach the next ledge beside a wall.
	// Do not undo that progress merely because it gains little X initially.
	if canJump && x >= p.X-4 && y < p.Y-12 {
		return 0, false
	}
	x, y, found := b.planDropThrough(o)
	if !found {
		return 0, false
	}
	if botNozzleBlocksRange(o, x, p.Y-24, y+8) || !b.nozzleRangeOpening(o, x, p.Y-24, y+8) {
		return ButtonDown | fire, true
	}
	b.JumpTargetX, b.JumpTargetActive, b.JumpCooldown = x, true, 14
	return ButtonDown | ButtonA | fire, true
}
