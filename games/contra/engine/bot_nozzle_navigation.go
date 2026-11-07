package contra

// A visible mouth can begin emitting before a long flame pattern is visible.
// Prefer landing outside its immediate forward neighborhood. This is a public
// geometric precaution, not a claim about its timer or current attack state.
func botNozzleLandingRisk(o BotObservation, x, y int) int {
	for _, e := range o.Enemies {
		if e.Family != "machine-flame-nozzle-left" {
			continue
		}
		dx, dy := e.X-x, y-e.Y
		if dx >= -8 && dx <= 32 && dy >= -8 && dy <= 24 {
			return 2000
		}
	}
	return 0
}

// The visible tip plus its visible mouth delimit the current horizontal beam.
// Crouching is useful only when that beam is above the grounded body center;
// disappearance is immediate public evidence to resume normal planning.
func botVisibleNozzleBeamAbove(o BotObservation) bool {
	p := o.Players[o.Actor]
	for _, mouth := range o.Enemies {
		if mouth.Family != "machine-flame-nozzle-left" {
			continue
		}
		for _, flame := range o.Enemies {
			if flame.Family != "machine-flame" || flame.Kind != 17 || absBot(flame.Y-mouth.Y) > 4 || flame.X > mouth.X || mouth.X-flame.X > 128 {
				continue
			}
			if p.X >= flame.X-12 && p.X <= mouth.X+8 && p.Y-flame.Y >= 8 && p.Y-flame.Y <= 24 {
				return true
			}
		}
	}
	return false
}

// A short gap in matching does not erase previously visible fire. This memory
// expires after eight public frames and follows only observed camera motion.
func (b *Bot) nozzleHazardView(o BotObservation) BotObservation {
	out := o
	out.Enemies = append([]BotEnemy(nil), o.Enemies...)
	for id, e := range b.PreviousEnemies {
		if e.Family != "machine-flame" && e.Family != "machine-flame-nozzle-left" {
			continue
		}
		seen := b.TrackSeenAt[id]
		if seen == 0 || o.Tick < seen || o.Tick-seen > 8 {
			continue
		}
		current := false
		for _, now := range o.Enemies {
			if now.ID == id && now.Family == e.Family {
				current = true
				break
			}
		}
		if current {
			continue
		}
		e.X -= b.CameraX - b.TrackCameraX[id]
		e.Y -= b.CameraY - b.TrackCameraY[id]
		out.Enemies = append(out.Enemies, e)
	}
	return out
}

func botNozzleBlocksRange(o BotObservation, target, lowY, highY int) bool {
	p := o.Players[o.Actor]
	for _, mouth := range o.Enemies {
		if mouth.Family != "machine-flame-nozzle-left" || mouth.X-min(p.X, target) < -8 || mouth.X-min(p.X, target) > 128 || mouth.Y < lowY || mouth.Y > highY {
			continue
		}
		for _, flame := range o.Enemies {
			if flame.Family == "machine-flame" && flame.Kind == 17 && absBot(flame.Y-mouth.Y) <= 4 && flame.X <= mouth.X && mouth.X-flame.X <= 128 {
				return true
			}
		}
	}
	return false
}

// Start during the first few quiet observations after a beam retracts, rather
// than arriving at an arbitrary point in the emitter's quiet interval. This
// uses only the time of observed artwork; it does not know an emitter clock.
func (b *Bot) nozzleRangeOpening(o BotObservation, target, lowY, highY int) bool {
	p := o.Players[o.Actor]
	relevant, opening := false, false
	for _, mouth := range o.Enemies {
		if mouth.Family != "machine-flame-nozzle-left" || mouth.X-min(p.X, target) < -8 || mouth.X-min(p.X, target) > 128 || mouth.Y < lowY || mouth.Y > highY {
			continue
		}
		relevant = true
		for id, old := range b.PreviousEnemies {
			seen := b.TrackSeenAt[id]
			if seen == 0 || o.Tick < seen || o.Tick-seen < 9 || o.Tick-seen > 16 || old.Family != "machine-flame" || old.Kind != 17 {
				continue
			}
			x := old.X - (b.CameraX - b.TrackCameraX[id])
			y := old.Y - (b.CameraY - b.TrackCameraY[id])
			if absBot(y-mouth.Y) <= 4 && x <= mouth.X && mouth.X-x <= 128 {
				opening = true
			}
		}
	}
	return !relevant || opening
}

func (b *Bot) holdOccludedNozzleCrouch(o BotObservation) bool {
	if o.Tick != b.LastTick+1 {
		return false
	}
	if b.Phase != "battle" || o.Phase != "battle" || b.LastPad&ButtonDown == 0 || b.LastPad & ^(ButtonDown|ButtonB) != 0 {
		return false
	}
	p := o.Players[o.Actor]
	old := b.PlayerMemory[o.Actor]
	if p.GameOverKnown && p.GameOver || p.AliveKnown && !p.Alive || p.LivesKnown && old.LivesKnown && p.Lives < old.Lives {
		return false
	}
	if p.Visible && (p.Sprite != 23 || o.terrainKnownAt(p.X, p.Y+16)) {
		return false
	}
	for _, e := range o.Enemies {
		if e.Family == "machine-flame-nozzle-left" {
			return true
		}
	}
	return false
}

func botNozzleBlocksJump(o BotObservation, target int) bool {
	p := o.Players[o.Actor]
	return botNozzleBlocksRange(o, target, p.Y-88, p.Y+8)
}
func (b *Bot) nozzleJumpOpening(o BotObservation, target int) bool {
	p := o.Players[o.Actor]
	if b.nozzleRangeOpening(o, target, p.Y-88, p.Y+8) {
		return true
	}
	// A distant mouth below the standing center need not trap the actor
	// until another cycle: a normal jump clears its height before a newly
	// extending beam can arrive. Require visible elevated landing support;
	// active beams and mouths intersecting the rising body still use the
	// observed quiet-opening rule above.
	if botNozzleBlocksJump(o, target) {
		return false
	}
	x, landingY, _, found := b.planJump(o, false)
	if !found || x != target || landingY >= p.Y-24 {
		return false
	}
	return botLowNozzleJumpClear(o, target, landingY)
}

func botLowNozzleJumpClear(o BotObservation, target, landingY int) bool {
	p := o.Players[o.Actor]
	relevant := false
	for _, mouth := range o.Enemies {
		if mouth.Family != "machine-flame-nozzle-left" || mouth.X-min(p.X, target) < -8 || mouth.X-min(p.X, target) > 128 || mouth.Y < p.Y-88 || mouth.Y > p.Y+8 {
			continue
		}
		relevant = true
		if mouth.Y < p.Y || mouth.X-p.X < 48 || landingY >= mouth.Y-24 {
			return false
		}
	}
	return relevant
}

func (b *Bot) lowNozzleEscape(o BotObservation) (int, bool) {
	p := o.Players[o.Actor]
	if !p.Visible || p.Sprite != 23 || absBot(p.Y-b.LastY) > 1 || b.JumpCooldown != 0 {
		return 0, false
	}
	hazards := b.nozzleHazardView(o)
	x, y, _, found := b.planJump(hazards, false)
	return x, found && x > p.X+12 && y < p.Y-24 && !botNozzleBlocksJump(hazards, x) && botLowNozzleJumpClear(hazards, x, y)
}
