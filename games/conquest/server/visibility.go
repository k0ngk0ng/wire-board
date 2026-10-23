package conquest

// Visibility is projected once for humans, spectators, and the bot. A caller
// can never recover hidden garrisons through summaries, previews, or events.
func (s *State) mask(v *View, seat int) {
	g, _ := mapFor(s.MapID)
	visible := make([]bool, len(s.Provinces))
	for i, p := range s.Provinces {
		visible[i] = !s.Options.Fog || (seat >= 0 && p.Owner == seat)
	}
	if s.Options.Fog && seat >= 0 {
		for i, p := range s.Provinces {
			if p.Owner == seat && p.Tower {
				for _, id := range g.data.Provinces[i].Neighbors {
					visible[g.index[id]] = true
				}
			}
		}
	}
	conceal := func(p *Province) { p.Troops = -1; p.Mobile = -1; p.Population = -1; p.Economy = -1; p.Morale = -1 }
	for i := range v.Provinces {
		if !visible[i] {
			conceal(&v.Provinces[i])
			conceal(&v.Projected[i])
		}
	}
	for i := range v.Players {
		p := &v.Players[i]
		if i != seat {
			p.Gold = -1
			p.Income = -1
			p.Upkeep = -1
			p.EconomySpending = -1
			p.Movement = -1
			p.MP10 = -1
			p.Tax = -1
			p.EconomyRate = -1
			p.MilitaryRate = -1
			p.Morale = -1
		}
		if s.Options.Fog && i != seat {
			known := true
			for j, province := range s.Provinces {
				if province.Owner == i && !visible[j] {
					known = false
					break
				}
			}
			if !known {
				p.Troops = -1
			}
			if j, ok := g.index[p.King]; ok && !visible[j] && !s.Provinces[j].Town {
				p.King = ""
			}
		}
	}
	if s.Options.Fog {
		filtered := []Event{}
		for _, event := range v.Events {
			own := event.Seat == seat && seat >= 0
			known := false
			for _, id := range []string{event.From, event.To, event.Province} {
				if j, ok := g.index[id]; ok && visible[j] {
					known = true
				}
			}
			if !own && !known {
				continue
			}
			if j, ok := g.index[event.To]; ok && !visible[j] {
				event.Before = nil
				event.After = nil
			}
			if !own {
				if j, ok := g.index[event.From]; ok && !visible[j] {
					event.Amount = -1
				}
			}
			filtered = append(filtered, event)
		}
		v.Events = filtered
	}
}
