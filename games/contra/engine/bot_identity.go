package contra

// Complete, palette-identified bodies are current evidence. Partial artwork
// only continues a recent track when the joint assignment is unambiguous.
func (b *Bot) associateVisiblePlayers(o *BotObservation) {
	occupied := func(actor int, p BotPlayer) bool {
		for j := 0; j < o.Seats; j++ {
			q := o.Players[j]
			if j != actor && q.Visible && absBot(p.X-q.X) <= 4 && absBot(p.Y-q.Y) <= 4 {
				return true
			}
		}
		return false
	}
	var options [2][]int
	for actor := 0; actor < o.Seats; actor++ {
		options[actor] = []int{-1}
		if o.Players[actor].Visible || b.PlayerSeenAt[actor] == 0 || o.Tick < b.PlayerSeenAt[actor] || o.Tick-b.PlayerSeenAt[actor] > 30 {
			continue
		}
		old := b.PlayerMemory[actor]
		for i, c := range o.PlayerCandidates {
			if c.Mask&(1<<uint(actor)) == 0 || occupied(actor, c.Player) {
				continue
			}
			if absBot(c.Player.X-old.X)+absBot(c.Player.Y-old.Y) <= 16+int(o.Tick-b.PlayerSeenAt[actor])*4 {
				options[actor] = append(options[actor], i)
			}
		}
	}
	if o.Seats == 1 {
		options[1] = []int{-1}
	}
	bestCount, bestCost := -1, 0
	chosen := [2]int{-1, -1}
	ambiguous := [2]bool{}
	for _, a := range options[0] {
		for _, c := range options[1] {
			if a >= 0 && c >= 0 {
				p, q := o.PlayerCandidates[a].Player, o.PlayerCandidates[c].Player
				if absBot(p.X-q.X) <= 4 && absBot(p.Y-q.Y) <= 4 {
					continue
				}
			}
			links := [2]int{a, c}
			count, cost := 0, 0
			for actor, idx := range links {
				if idx < 0 {
					continue
				}
				p, old := o.PlayerCandidates[idx].Player, b.PlayerMemory[actor]
				count++
				cost += absBot(p.X-old.X) + absBot(p.Y-old.Y)
			}
			if count > bestCount || count == bestCount && cost < bestCost {
				bestCount, bestCost, chosen, ambiguous = count, cost, links, [2]bool{}
			} else if count == bestCount && cost == bestCost {
				for actor, idx := range links {
					prev := chosen[actor]
					if prev == idx {
						continue
					}
					if prev < 0 || idx < 0 {
						ambiguous[actor] = true
						continue
					}
					p, q := o.PlayerCandidates[prev], o.PlayerCandidates[idx]
					if p.Player.X != q.Player.X || p.Player.Y != q.Player.Y {
						ambiguous[actor] = true
						continue
					}
					// Same position, deterministic best pose; input ordering is irrelevant.
					if q.Exact && !p.Exact || q.Exact == p.Exact && (q.Player.Sprite < p.Player.Sprite || q.Player.Sprite == p.Player.Sprite && q.Player.Appearance < p.Player.Appearance) {
						chosen[actor] = idx
					}
				}
			}
		}
	}
	for i := 0; i < o.Seats; i++ {
		p := &o.Players[i]
		if p.Visible && i == o.Actor && b.PlayerSeenAt[i] != 0 {
			old := b.PlayerMemory[i]
			if absBot(p.X-old.X) > 12 || absBot(p.Y-old.Y) > 20 {
				b.JumpTargetActive, b.JumpCooldown, b.JumpPlatform = false, 0, 0
				b.LastX, b.LastY, b.StillTicks = p.X, p.Y, 0
			}
		}
		if !p.Visible && chosen[i] >= 0 && !ambiguous[i] {
			q := o.PlayerCandidates[chosen[i]].Player
			p.Visible, p.X, p.Y, p.Sprite, p.Appearance = true, q.X, q.Y, q.Sprite, q.Appearance
			if !p.AliveKnown {
				p.Alive = true
			}
		}
		if p.Visible {
			b.PlayerMemory[i], b.PlayerSeenAt[i] = *p, o.Tick
			continue
		}
	}
	// Apply fallback only after both current assignments are visible, so seat
	// iteration order cannot duplicate the other seat's newly resolved body.
	for i := 0; i < o.Seats; i++ {
		p := &o.Players[i]
		if p.Visible {
			continue
		}
		if b.PlayerSeenAt[i] != 0 && o.Tick >= b.PlayerSeenAt[i] && o.Tick-b.PlayerSeenAt[i] <= 8 && (!p.AliveKnown || p.Alive) {
			old := b.PlayerMemory[i]
			if occupied(i, old) {
				continue
			}
			p.X, p.Y, p.Sprite, p.Appearance = old.X, old.Y, old.Sprite, old.Appearance
			if !p.AliveKnown {
				p.Alive = old.Alive
			}
		}
	}
}
