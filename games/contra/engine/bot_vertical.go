package contra

// verticalAim targets visible opponents while leaving jump/landing ownership
// with navigation. A stationary actor can turn once and continue firing across
// its platform instead of shooting upward while a runner approaches sideways.
func (b *Bot) verticalAim(o BotObservation, pad Buttons) Buttons {
	p := o.Players[o.Actor]
	best := 10000
	var target BotEnemy
	found := false
	for _, e := range o.Enemies {
		if e.Kind != 4 && e.Kind != 5 && e.Kind != 6 && e.Kind != 7 && e.Kind != 19 && e.Kind != 20 && e.Kind != 21 {
			continue
		}
		if e.Sprite >= 53 && e.Sprite <= 55 {
			continue
		}
		if e.Kind == 5 && e.Sprite < 38 {
			continue
		}
		score := absBot(e.X-p.X) + absBot(e.Y-(p.Y-5))*2
		if score < best && score < 180 {
			best, target, found = score, e, true
		}
	}
	if !found {
		return pad
	}
	dx, dy := target.X-p.X, target.Y-(p.Y-5)
	grounded := o.supportAt(p.X, p.Y+16) != TerrainEmpty && p.Sprite != 8 && p.Sprite != 9
	if grounded && pad&ButtonA == 0 && absBot(dx) < 80 && absBot(dy) < 16 {
		left := dx < 0
		if b.FacingLeft == left {
			return 0
		}
		step, dir := 1, ButtonRight
		if left {
			step, dir = -1, ButtonLeft
		}
		if o.supportAt(p.X+step, p.Y+16) != TerrainEmpty {
			return dir
		}
	}
	if dx > 0 && pad&ButtonRight != 0 || dx < 0 && pad&ButtonLeft != 0 {
		pad &^= ButtonUp | ButtonDown
		if absBot(dy) < 18 {
			return pad
		}
		if dy < 0 {
			return pad | ButtonUp
		}
		return pad | ButtonDown
	}
	return pad
}
