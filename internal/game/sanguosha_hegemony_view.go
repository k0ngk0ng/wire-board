package game

func sgHegSkillCatalog() map[string]SGSkill {
	info := make(map[string]SGSkill, len(SGSkills))
	for id, skill := range SGSkills {
		info[id] = skill
	}
	for id, description := range map[string]string{
		"guzheng":   "其他角色弃牌阶段结束时，可以返还其在该阶段弃置的一张手牌，然后选择是否获得该阶段其余仍在弃牌堆的弃置牌。",
		"kongcheng": "锁定技：成为杀或决斗的目标时，若没有手牌，取消自己作为目标。暗置时可选择是否明置发动。",
		"qianxun":   "锁定技：成为顺手牵羊或乐不思蜀的目标时，取消自己作为目标。暗置时可选择是否明置发动。",
		"weimu":     "锁定技：成为黑色锦囊牌的目标时，取消自己作为目标。暗置时可选择是否明置发动。",
		"huoshou":   "锁定技：南蛮入侵对你无效；其他角色使用南蛮入侵选定目标时，可明置成为本张牌的伤害来源。之后仅为免疫而明置不追溯改变来源。",
		"juxiang":   "锁定技：南蛮入侵对你无效；其他角色使用的实体南蛮入侵结算后进入弃牌堆时，获得此牌。暗置时在这两个时机分别可选择明置发动。",
	} {
		skill := info[id]
		skill.Text = description
		info[id] = skill
	}
	return info
}

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
		"skills": s.sgHegSkills(i, true), "used": p.Used, "marks": p.Marks,
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
		v["canReveal"] = s.sgHegCanShow(i)
		v["ownKingdom"] = sgGeneral(p.General).Kingdom
		v["companion"] = sgHegCompanions(p.General, h.Deputy)
		v["companionClaimed"] = h.CompanionClaimed
		v["halfHP"] = (sgGeneral(p.General).HP+sgGeneral(h.Deputy).HP)%2 == 1
		v["halfClaimed"] = h.HalfClaimed
	}
	return v
}
