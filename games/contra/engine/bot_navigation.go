package contra

import "math"

// A falling respawn can drift past its only reachable platform before there
// is any opportunity to jump. Aim at a visible landing using observed descent
// and the same public movement envelope as ordinary jump planning.
func (b *Bot) planDescent(o BotObservation) (int, bool) {
	p := o.Players[o.Actor]
	velocity := float64(max(0, min(8, p.Y-b.LastY)))
	best, bestX, found := -100000, p.X, false
	for goal := min(p.X, max(28, p.X-48)); goal <= min(228, p.X+64); goal += 4 {
		x, y, vy := float64(p.X), float64(p.Y), velocity
		for tick := 0; tick < 80 && y < 216; tick++ {
			kind := o.terrainAt(int(x), int(y)+16)
			if vy >= 1 && y >= float64(p.Y+2) && (kind == TerrainPlatform || kind == TerrainSolid) {
				if o.terrainAt(int(x)-6, int(y)+16) == kind && o.terrainAt(int(x)+6, int(y)+16) == kind {
					// Stabilize the nearest safe landing first. Forward progress
					// resumes after touching down, when jumping is available.
					score := -absBot(int(x)-p.X)*4 - (int(y) - p.Y)
					if score > best {
						best, bestX, found = score, int(x), true
					}
				}
				break
			}
			vy += 35.0 / 256.0
			y += vy
			if x < float64(goal) {
				x++
			} else if x > float64(goal) {
				x--
			}
		}
	}
	return bestX, found
}

// planJump is a small public-geometry predictor, not a copy of World or a rule
// executor. It searches visible landing surfaces using the observed character
// position and known movement envelope. Enemy movement is estimated from two
// visible samples; no hostile timers, velocities, random values or HP are read.
func (b *Bot) planJump(o BotObservation, vertical bool) (int, int, int, bool) {
	return b.planJumpRoute(o, vertical, false)
}

func (b *Bot) planJumpRoute(o BotObservation, vertical, detour bool) (int, int, int, bool) {
	p := o.Players[o.Actor]
	best := -1000000
	bestX, bestY := p.X, p.Y
	bestPlatform := -1
	found := false
	hazardScore, hazardX, hazardY, hazardPlatform := -1000000, p.X, p.Y, -1
	hazardFound := false
	approach, hasApproach := botClimbApproach(o, p.X, p.Y+16)
	for goal := max(28, p.X-48); goal <= min(228, p.X+64); goal += 4 {
		x, y, vy := float64(p.X), float64(p.Y), -4.0625
		risk := 0
		for t := 1; t <= 95; t++ {
			if vy < 0 && o.terrainAt(int(x), int(y)-10) == TerrainSolid {
				// Original head contact clears rising velocity, then gravity
				// continues; a lower visible landing can still be reachable.
				vy = 0
			}
			if y >= 224 || y < 0 {
				break
			}
			for _, e := range o.Enemies {
				if e.Kind == 0 || e.Kind == 2 || e.Kind == 3 || o.Stage == 2 && (e.Kind == 16 || e.Kind == 18) {
					continue
				}
				vx, vy := b.observedVelocity(o, e)
				ex, ey := int(float64(e.X)+vx*float64(t)), int(float64(e.Y)+vy*float64(t))
				if o.Stage == 2 && e.Kind == 19 {
					ey += t * t / 32
				}
				width, top, bottom := 12, 22, 7
				if vertical {
					width, top, bottom = 16, 32, 20
				}
				if absBot(ex-int(x)) < width && ey > int(y)-top && ey < int(y)+bottom {
					risk += 200
				}
			}
			kind := o.terrainAt(int(x), int(y)+16)
			platform := -1
			if kind == TerrainEmpty && o.Stage == 2 {
				for _, e := range o.Enemies {
					if e.Kind != 16 || e.Sprite != 72 {
						continue
					}
					vx, vy := b.observedVelocity(o, e)
					ex, ey := float64(e.X)+vx*float64(t), float64(e.Y)+vy*float64(t)
					if math.Abs(x-ex) < 12 && y >= ey-32 && y < ey-24 {
						kind = TerrainPlatform
						platform = e.ID
						break
					}
				}
			}
			floorGate := int(vy) >= 1 && (int(vy) >= 4 || (int(y)+int(o.ViewY))&15 < 8)
			if platform >= 0 {
				floorGate = vy >= 1
			}
			if t > 12 && floorGate && kind != TerrainEmpty {
				// A supporting cell is not a landing when the body would
				// occupy a visible wall, either now or at the steering goal.
				if o.terrainAt(int(x), int(y)-10) == TerrainSolid || o.terrainAt(goal, int(y)-10) == TerrainSolid || o.terrainAt(int(x), int(y)) == TerrainSolid || o.terrainAt(goal, int(y)) == TerrainSolid {
					break
				}
				if kind == TerrainWater {
					break
				}
				// The original can land when the foot center first reaches a
				// ledge. Keep steering toward a point safely inside that ledge;
				// do not require the whole body to cross before first contact.
				if platform < 0 && (o.terrainAt(goal-4, int(y)+16) == TerrainEmpty || o.terrainAt(goal+4, int(y)+16) == TerrainEmpty) {
					break
				}
				height := p.Y - int(y)
				heightWeight := 1
				if p.Y >= 160 && o.terrainAt(p.X+6, p.Y+16) != TerrainEmpty {
					heightWeight = 5
				}
				score := (int(x)-p.X)*4 + height*heightWeight - risk
				if b.ViewStillTicks > 240 && !vertical {
					score = (int(x)-p.X)*20 + height - risk/10
				}
				if vertical {
					score = height*12 - absBot(int(x)-p.X) - risk
					if hasApproach {
						score = height*12 - absBot(int(x)-approach)*3 - risk
					}
				}
				if detour {
					// Only a currently visible landing which brings the actor
					// closer to the higher ledge can justify losing height.
					gain := absBot(p.X-approach) - absBot(goal-approach)
					if !hasApproach || gain < 16 || height < -32 || risk != 0 {
						break
					}
					score = gain*12 + height
				}
				penalty := botNozzleLandingRisk(o, goal, int(y))
				if !vertical && penalty > 0 && goal > p.X+12 && int(y) < p.Y-12 && score > hazardScore {
					hazardScore, hazardX, hazardY, hazardPlatform, hazardFound = score, goal, int(y), platform, true
				}
				score -= penalty
				if score > best {
					best, bestX, bestY, found = score, goal, int(y), true
					bestPlatform = platform
				}
				break
			}
			vy += 35.0 / 256.0
			y += vy
			if x < float64(goal) && !botVisibleAirSideBlocked(o, int(x), int(y), 1) {
				x++
			} else if x > float64(goal) && !botVisibleAirSideBlocked(o, int(x), int(y), -1) {
				x--
			}
		}
	}
	// A safe forward or upward route remains preferred. If the only such
	// route is near a nozzle, retain it as a plan; play still waits for the
	// observed firing window before sending A.
	if hazardFound && (!found || bestX <= p.X+12 && bestY >= p.Y-12) {
		return hazardX, hazardY, hazardPlatform, true
	}
	return bestX, bestY, bestPlatform, found
}

func (b *Bot) observedVelocity(o BotObservation, e BotEnemy) (float64, float64) {
	if e.ID < 0 || e.ID >= botTrackLimit || !b.EnemySeen[e.ID] || b.PreviousEnemies[e.ID].Kind != e.Kind {
		return 0, 0
	}
	old := b.PreviousEnemies[e.ID]
	vx, vy := float64(e.X-old.X+botViewDelta(o.ViewX, b.LastViewX, 16)), float64(e.Y-old.Y+botViewDelta(o.ViewY, b.LastViewY, 16))
	if math.Abs(vx) > 12 || math.Abs(vy) > 12 {
		return 0, 0
	}
	return (b.EnemyVX[e.ID] + vx) / 2, (b.EnemyVY[e.ID] + vy) / 2
}

func botViewDelta(now, before uint8, span int) int {
	d := int(now)%span - int(before)%span
	if d > span/2 {
		d -= span
	}
	if d < -span/2 {
		d += span
	}
	return d
}

func (b *Bot) airEscape(o BotObservation) Buttons {
	p := o.Players[o.Actor]
	for _, e := range o.Enemies {
		if e.Kind != 1 && e.Kind != 11 && e.Kind != 15 && !(o.Vertical && (e.Kind == 17 || e.Kind == 19)) {
			continue
		}
		if e.ID < 0 || e.ID >= botTrackLimit || !b.EnemySeen[e.ID] {
			continue
		}
		old := b.PreviousEnemies[e.ID]
		if old.Kind != e.Kind {
			continue
		}
		vx, vy := e.X-old.X+botViewDelta(o.ViewX, b.LastViewX, 16), e.Y-old.Y+botViewDelta(o.ViewY, b.LastViewY, 16)
		if absBot(vx) > 8 || absBot(vy) > 8 {
			continue
		}
		for t := 1; t <= 12; t++ {
			ey := e.Y + vy*t
			if o.Stage == 2 && e.Kind == 19 {
				ey += t * t / 32
			}
			py := float64(p.Y+(p.Y-b.LastY+botViewDelta(o.ViewY, b.LastViewY, 16))*t) + 0.5*(35.0/256.0)*float64(t*t)
			if absBot(e.X+vx*t-p.X) < 14 && float64(ey) > py-23 && float64(ey) < py+7 {
				if e.X >= p.X {
					return ButtonLeft
				}
				return ButtonRight
			}
		}
	}
	return 0
}

func (b *Bot) climbAvoid(o BotObservation, desired Buttons) Buttons {
	p := o.Players[o.Actor]
	best, bestRisk := desired, 1000000
	for _, choice := range []Buttons{desired, 0, ButtonLeft, ButtonRight} {
		dir := 0
		if choice&ButtonLeft != 0 {
			dir = -1
		}
		if choice&ButtonRight != 0 {
			dir = 1
		}
		risk := 0
		if choice != desired {
			risk = 1
		}
		for t := 1; t <= 20; t++ {
			x := p.X + dir*t
			if t <= 2 && (x < 27 || x > 229 || o.supportAt(x, p.Y+16) == TerrainEmpty) {
				risk += 1000
				continue
			}
			for _, e := range o.Enemies {
				if e.Kind == 0 || e.Kind == 2 || e.Kind == 3 || e.Kind == 16 || e.Kind == 18 {
					continue
				}
				vx, vy := 0, 0
				if e.ID >= 0 && e.ID < botTrackLimit && b.EnemySeen[e.ID] {
					old := b.PreviousEnemies[e.ID]
					vx = e.X - old.X
					vy = e.Y - old.Y + botViewDelta(o.ViewY, b.LastViewY, 16)
				}
				if absBot(vx) > 8 || absBot(vy) > 8 {
					continue
				}
				ex, ey := e.X+vx*t, e.Y+vy*t
				if e.Kind == 19 {
					ey += t * t / 32
				}
				if absBot(ex-x) < 18 && ey > p.Y-32 && ey < p.Y+20 {
					risk += 2000
				}
			}
		}
		if risk < bestRisk {
			best, bestRisk = choice, risk
		}
	}
	return best
}

func (b *Bot) safeDrop(o BotObservation) bool {
	p := o.Players[o.Actor]
	x, y, vy := float64(p.X), float64(p.Y), 0.0
	falling := false
	for tick := 0; tick < 96; tick++ {
		if x >= 248 || y >= 216 {
			return false
		}
		nextX := int(x) + 1
		blocked := o.terrainAt(nextX+6, int(y)-4) == TerrainSolid
		if !falling {
			if blocked {
				return false
			}
			x++
			support := o.terrainAt(int(x)+8, int(y)+16)
			if (support == TerrainPlatform || support == TerrainSolid) && (!o.GridKnown || o.terrainKnownAt(int(x)+8, int(y)+16)) {
				continue
			}
			falling = true
		} else if !blocked {
			x++
		}
		vy += 35.0 / 256.0
		y += vy
		kind := o.terrainAt(int(x), int(y)+16)
		if kind == TerrainWater {
			return false
		}
		if y >= float64(p.Y+20) && (kind == TerrainPlatform || kind == TerrainSolid) && (!o.GridKnown || o.terrainKnownAt(int(x), int(y)+16)) && o.terrainAt(int(x), int(y)-10) != TerrainSolid {
			return true
		}
	}
	return false
}

func (b *Bot) bossEvade(o BotObservation, desired Buttons) Buttons {
	p := o.Players[o.Actor]
	best, bestRisk := desired, 100000000
	for _, choice := range []Buttons{desired, 0, ButtonLeft, ButtonRight} {
		direction := 0
		if choice&ButtonLeft != 0 {
			direction = -1
		}
		if choice&ButtonRight != 0 {
			direction = 1
		}
		risk := 0
		if choice != desired {
			risk = 1
		}
		for _, e := range o.Enemies {
			if e.Kind != 1 && e.Kind != 27 && e.Kind != 29 {
				continue
			}
			if e.ID < 0 || e.ID >= botTrackLimit || !b.EnemySeen[e.ID] {
				continue
			}
			old := b.PreviousEnemies[e.ID]
			vx, vy := e.X-old.X, e.Y-old.Y
			if absBot(vx) > 8 || absBot(vy) > 8 {
				continue
			}
			for t := 1; t <= 28; t++ {
				ex, ey := e.X+vx*t, e.Y+vy*t
				px := max(48, min(208, p.X+direction*t))
				if absBot(ex-px) < 22 && ey > p.Y-32 && ey < p.Y+20 {
					risk += 1000 + (22-absBot(ex-px))*20
				}
			}
		}
		if risk < bestRisk {
			best, bestRisk = choice, risk
		}
	}
	return best
}

func (b *Bot) simpleBossEvade(o BotObservation) Buttons {
	p := o.Players[o.Actor]
	for _, e := range o.Enemies {
		if e.Kind != 1 && e.Kind != 27 && e.Kind != 29 || e.ID < 0 || e.ID >= botTrackLimit || !b.EnemySeen[e.ID] {
			continue
		}
		if e.Sprite >= 53 && e.Sprite <= 55 {
			continue
		}
		old := b.PreviousEnemies[e.ID]
		vx, vy := e.X-old.X, e.Y-old.Y
		if absBot(vx) > 8 || absBot(vy) > 8 {
			continue
		}
		lookahead, width, top, bottom := 40, 22, 40, 32
		if e.Kind == 1 {
			lookahead, width, top, bottom = 24, 8, 20, 20
		}
		for t := 1; t <= lookahead; t++ {
			ex, ey := e.X+vx*t, e.Y+vy*t
			px := p.X + (p.X-b.LastX)*t
			if absBot(ex-px) < width && ey > p.Y-top && ey < p.Y+bottom {
				if p.X < 64 {
					return ButtonRight
				}
				if p.X > 192 {
					return ButtonLeft
				}
				if e.X > p.X || e.X == p.X && p.X > 128 {
					return ButtonLeft
				}
				return ButtonRight
			}
		}
	}
	return 0
}
