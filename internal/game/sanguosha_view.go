package game

import "slices"

// Deliberately construct public data instead of serializing and deleting fields:
// queued effects contain private choices, and must never reach another seat.
func (s *State) sgView(viewer int) map[string]any {
	g := s.Sanguosha
	players := []map[string]any{}
	for i, p := range g.Players {
		general := p.General
		if g.Selecting && i != g.Lord && i != viewer {
			general = ""
		}
		v := map[string]any{"general": general, "marks": p.Marks, "handLimit": s.sgHandLimit(i), "flipped": p.Flipped, "buqu": p.Buqu, "chained": p.Chained, "drank": p.Drank, "hp": p.HP, "maxHP": p.MaxHP, "dead": p.Dead, "handCount": len(p.Hand), "equip": p.Equip, "judgment": p.Judgment, "used": p.Used}
		if general != "" {
			v["skills"] = s.sgSkills(i)
			v["kingdom"] = s.sgKingdom(i)
			v["female"] = s.sgFemale(i)
			v["skillsLost"] = p.SkillsLost
			v["fields"] = p.Fields
			v["avatar"] = p.Avatar
			v["avatarSkill"] = p.AvatarSkill
			v["avatarCount"] = len(p.Avatars)
		}
		if g.Selecting && general == "" {
			v["hp"] = 0
			v["maxHP"] = 0
			v["handLimit"] = 0
		}
		if i == viewer || p.Role == "lord" || p.Dead || s.Finished {
			v["role"] = p.Role
		}
		if i == viewer {
			v["hand"] = p.Hand
			v["choices"] = p.Choices
			v["avatars"] = p.Avatars
		}
		players = append(players, v)
	}
	visible := map[string]any{"players": players, "lord": g.Lord, "selecting": g.Selecting, "remaining": len(g.Deck), "discardCount": len(g.Discard), "table": g.Table, "grace": g.Grace, "generals": g.generalCatalog(), "cardTypes": SGCardTypes, "cards": s.sgVisibleCatalog(viewer), "skills": SGSkills, "sequence": g.Sequence, "options": g.Options, "revealed": g.Revealed}
	if len(g.Bluffs) > 0 {
		b := g.Bluffs[len(g.Bluffs)-1]
		visible["bluff"] = map[string]any{"player": b.Player, "declared": b.Declared, "context": s.sgBluffContext(b), "questioned": b.Questioned, "resolved": b.Resolved}
	}
	if p := g.Pending; p != nil {
		prompt := map[string]any{"id": p.ID, "player": p.Player, "kind": p.Kind}
		canRespond := viewer == p.Player || p.Kind == "nullification" && s.sgAlive(viewer) && !slices.Contains(p.Event.Targets, viewer)
		prompt["canRespond"] = canRespond
		if canRespond {
			e := p.Event
			prompt["message"] = p.Message
			prompt["cards"] = p.Cards
			prompt["source"] = e.Actor
			prompt["target"] = e.Target
			prompt["effect"] = e.Kind
			prompt["amount"] = e.Amount
			prompt["count"] = e.Count
			prompt["step"] = e.Step
			prompt["choices"] = p.Choices
			prompt["targets"] = p.Targets
			if p.Kind == "huashen" {
				prompt["required"] = p.Event.Flag
				choices := map[string][]string{}
				for _, id := range p.Choices {
					choices[id] = sgAvatarSkills(id)
				}
				prompt["avatarSkills"] = choices
			}
			if p.Kind == "qiaobian_move" {
				prompt["moves"] = s.sgQiaobianMoves(viewer)
			}
			if p.Kind == "card" {
				prompt["wanted"] = sgWanted(e)
				prompt["ignoreArmor"] = s.sgIgnoreArmor(e)
			}
			if p.Kind == "support" {
				if e.Kind == "hujia" {
					prompt["wanted"] = "jink"
				} else {
					prompt["wanted"] = "slash"
				}
			}
			if p.Kind == "nullification" {
				prompt["cancelled"] = e.Flag
			}
		}
		visible["pending"] = prompt
	}
	if viewer >= 0 && viewer < len(g.Players) {
		dist := []int{}
		for i := range g.Players {
			dist = append(dist, s.sgDistance(viewer, i))
		}
		visible["distances"] = dist
		visible["range"] = s.sgRange(viewer)
		visible["armor"] = s.sgArmor(viewer)
		if s.sgHas(viewer, "guhuo") {
			visible["guhuoKinds"] = s.sgGuhuoKinds()
		}
		visible["zhibaPindian"] = viewer != g.Lord && s.sgKingdom(viewer) == "wu" && s.sgHas(g.Lord, "zhiba")
		visible["huangtianGive"] = viewer != g.Lord && s.sgKingdom(viewer) == "qun" && s.sgHas(g.Lord, "huangtian")
	}
	return map[string]any{"kind": s.Kind, "turn": s.Turn, "phase": s.Phase, "round": s.Round, "finished": s.Finished, "winners": s.Winners, "log": s.Log, "sanguosha": visible}
}
