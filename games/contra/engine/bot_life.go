package contra

// BorrowPending is a reason to try a legal input, not a copy of an internal
// elimination flag. A visible last-reserve death or explicit intro-seat result
// supplies the evidence; a visible live body or new reserve clears it.
func (b *Bot) observeBorrowSignals(o BotObservation) {
	for seat := 0; seat < o.Seats; seat++ {
		p := o.Players[seat]
		if p.LivesKnown && p.Lives > 0 {
			b.BorrowPending[seat] = false
		}
		if p.GameOverKnown && p.GameOver {
			b.BorrowPending[seat] = true
			continue
		}
		if !p.Visible {
			continue
		}
		death := p.Sprite >= 10 && p.Sprite <= 12 || p.Sprite == 86
		if !death {
			b.BorrowPending[seat] = false
		} else if p.LivesKnown && p.Lives == 0 {
			b.BorrowPending[seat] = true
		}
	}
}
