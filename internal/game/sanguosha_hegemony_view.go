package game

func (s *State) sgHegPlayerView(i, viewer int) map[string]any {
	g := s.Sanguosha
	p := g.Players[i]
	h := p.Hegemony
	ids := s.sgHegGeneralIDs(i)
	for slot := range ids {
		if i != viewer && !h.Shown[slot] && !p.Dead && !s.Finished {
			ids[slot] = ""
		}
	}
	v := map[string]any{
		"general": ids[0], "deputy": ids[1], "shown": h.Shown,
		"lost": h.Lost, "dead": p.Dead, "hp": p.HP, "maxHP": p.MaxHP,
		"handCount": len(p.Hand), "handLimit": s.sgHandLimit(i),
		"equip": p.Equip, "judgment": p.Judgment, "flipped": p.Flipped,
		"chained": p.Chained, "drank": p.Drank, "buqu": p.Buqu,
		"skills": s.sgHegSkills(i, true), "used": p.Used,
	}
	if s.sgHegShown(i) || p.Dead || s.Finished {
		v["role"] = p.Role
		v["kingdom"] = s.sgKingdom(i)
		v["female"] = s.sgFemale(i)
	}
	if g.Selecting && i != viewer {
		v["hp"], v["maxHP"], v["handLimit"] = 0, 0, 0
	}
	if i == viewer {
		v["knownGenerals"] = h.KnownGenerals
		v["hand"] = p.Hand
		v["choices"] = p.Choices
		v["ownSkills"] = s.sgHegSkills(i, false)
		v["ownKingdom"] = sgGeneral(p.General).Kingdom
		v["companion"] = sgHegCompanions(p.General, h.Deputy)
		v["companionClaimed"] = h.CompanionClaimed
		v["halfHP"] = (sgGeneral(p.General).HP+sgGeneral(h.Deputy).HP)%2 == 1
		v["halfClaimed"] = h.HalfClaimed
	}
	return v
}
