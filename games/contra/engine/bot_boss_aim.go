package contra

func botVisibleCarrierAim(o BotObservation) (Buttons, bool) {
	p := o.Players[o.Actor]
	if p.X <= 0 || p.Y <= 0 {
		return 0, false
	}
	var target BotEnemy
	found, best := false, 10000
	for _, e := range o.Enemies {
		if e.Family != "snow-carrier" {
			continue
		}
		if distance := absBot(e.X - p.X); distance < best {
			target, best, found = e, distance, true
		}
	}
	if !found {
		return 0, false
	}
	return botToward(p.X, max(24, min(232, target.X)), 3) | ButtonUp, true
}
