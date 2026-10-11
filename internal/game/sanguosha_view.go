package game

import "slices"

// Deliberately construct public data instead of serializing and deleting fields:
// queued effects contain private choices, and must never reach another seat.
func (s *State) sgView(viewer int) map[string]any {
	g := s.Sanguosha
	players := []map[string]any{}
	for i, p := range g.Players {
		if s.sgHegemony() {
			players = append(players, s.sgHegPlayerView(i, viewer))
			continue
		}
		general := p.General
		if g.Selecting && !s.sgThreeV3() && i != g.Lord && i != viewer {
			general = ""
		}
		v := map[string]any{"general": general, "marks": p.Marks, "handLimit": s.sgHandLimit(i), "flipped": p.Flipped, "buqu": p.Buqu, "chained": p.Chained, "drank": p.Drank, "hp": p.HP, "maxHP": p.MaxHP, "dead": p.Dead, "handCount": len(p.Hand), "equip": p.Equip, "judgment": p.Judgment, "used": p.Used}
		if general != "" {
			v["skills"] = s.sgSkills(i)
			v["kingdom"] = s.sgKingdom(i)
			v["female"] = s.sgFemale(i)
			v["skillsLost"] = p.SkillsLost
			v["fields"] = p.Fields
			v["starCount"] = len(p.Stars)
			v["silenced"] = p.Silenced
			v["handSealed"] = p.HandSealed
			v["yijiCount"] = len(p.Yiji)
			v["qianxunCount"] = len(p.Qianxun)
			v["jieLuoyi"] = p.JieLuoyi
			disabled := []string{}
			for _, skill := range s.sgSkills(i) {
				if !s.sgHas(i, skill) {
					disabled = append(disabled, skill)
				}
			}
			v["disabledSkills"] = disabled
			v["galeTargets"] = p.Gale
			v["fogTargets"] = p.Fog
			v["armorDisabled"] = s.sgGodArmorOff(i)
			v["avatar"] = p.Avatar
			v["avatarSkill"] = p.AvatarSkill
			v["avatarCount"] = len(p.Avatars)
		}
		if g.Selecting && general == "" {
			v["hp"] = 0
			v["maxHP"] = 0
			v["handLimit"] = 0
		}
		if s.sgThreeV3() {
			// Camps and leader cards are public in 3v3.
			v["role"] = map[bool]string{true: "leader", false: "vanguard"}[s.sgThreeIsLeader(i)]
			v["camp"] = s.sgThreeCamp(i)
		} else if i == viewer || p.Role == "lord" || p.Dead || s.Finished {
			v["role"] = p.Role
		}
		if i == viewer {
			v["hand"] = p.Hand
			v["choices"] = p.Choices
			v["avatars"] = p.Avatars
			v["stars"] = p.Stars
			v["yiji"] = p.Yiji
			v["qianxun"] = p.Qianxun
		}
		players = append(players, v)
	}
	visible := map[string]any{"players": players, "lord": g.Lord, "selecting": g.Selecting, "remaining": len(g.Deck), "discardCount": len(g.Discard), "table": g.Table, "grace": g.Grace, "generals": g.generalCatalog(), "cardTypes": SGCardTypes, "cards": s.sgVisibleCatalog(viewer), "skills": SGSkills, "sequence": g.Sequence, "options": g.Options, "revealed": g.Revealed}
	if s.sgHegemony() {
		visible["lord"] = -1
		visible["skills"] = sgHegSkillCatalog()
		visible["first"] = g.Hegemony.First
		visible["hegemony"] = true
		visible["companions"] = sgHegemonyCompanions
	}
	if three := g.ThreeV3; three != nil {
		visible["lord"] = -1
		visible["threeV3"] = map[string]any{
			"rules": three.Rules, "stage": three.Stage, "side": three.Side,
			"firstSide": three.FirstSide, "pickFirst": three.PickFirst, "pickCamp": three.PickCamp,
			"pickLeft": three.PickLeft, "leaders": three.Leaders, "pool": three.Pool,
			"picked": three.Picked, "assigned": three.Assigned, "acted": three.Acted,
		}
	}
	if len(g.Bluffs) > 0 {
		b := g.Bluffs[len(g.Bluffs)-1]
		visible["bluff"] = map[string]any{"player": b.Player, "declared": b.Declared, "context": s.sgBluffContext(b), "questioned": b.Questioned, "resolved": b.Resolved}
	}
	if p := g.Pending; p != nil {
		prompt := map[string]any{"id": p.ID, "player": p.Player, "kind": p.Kind}
		canRespond := viewer == p.Player || p.Kind == "nullification" && s.sgAlive(viewer) && !slices.Contains(p.Event.Targets, viewer)
		if !canRespond && s.sgHegPrivatePrompt(*p) {
			prompt["kind"] = "hegemony_response"
		}
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
			if three := g.ThreeV3; three != nil {
				switch p.Kind {
				case "sg_3v3_pick":
					prompt["choices"] = three.Pool
				case "sg_3v3_assign":
					camp := min(three.AssignCamp, sgThreeWarm)
					left := []string{}
					for _, id := range three.Picked[camp] {
						if !slices.Contains(three.Assigned[camp], id) {
							left = append(left, id)
						}
					}
					prompt["choices"] = left
				case "sg_3v3_first":
					prompt["choices"] = []string{"us", "them"}
				case "sg_3v3_first_side":
					prompt["choices"] = []string{"cold", "warm"}
				case "sg_3v3_side":
					prompt["choices"] = []string{"leader", "vanguards"}
				}
			}
			if p.Kind == "jie_fanjian" || p.Kind == "jie_tieji_discard" {
				prompt["suit"] = e.Color
			}
			if p.Kind == "steal" && sgStealDiscards(e.Kind) {
				blocked := []int{}
				for _, id := range g.Players[e.Target].Equip {
					if !s.sgCanDiscard(viewer, e.Target, id) {
						blocked = append(blocked, id)
					}
				}
				prompt["protectedCards"] = blocked
			}
			prompt["targets"] = p.Targets
			if s.sgHegemony() && p.Kind == "heg_intel" && e.Kind != "hand" {
				slot := 0
				if e.Kind == "deputy" {
					slot = 1
				}
				prompt["general"] = s.sgHegGeneralIDs(e.Target)[slot]
			}
			if s.sgHegemony() && p.Kind == "nullification" {
				prompt["factionCounter"] = e.CounterDepth == 0 && e.Aux != -2 && !sgIsDelayed(e.Kind)
			}
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
