package game

// Hidden compulsory range/count skills are revealed only when this concrete
// card use needs them. Public range queries never activate them. As with the
// rest of card use, invalid targets roll all these changes back atomically.
func (s *State) sgHegUseModifiers(i int, kind string, ids, targets []int, forced bool) error {
	if !s.sgHegemony() {
		return nil
	}
	g := s.Sanguosha
	p := g.Players[i]
	probe := clone(*s)
	probe.sgLose(i, ids)
	if sgIsSlash(kind) && !forced && p.Used["slash"] >= 1+max(0, s.sgTianyi(i)) && probe.sgWeapon(i) != "crossbow" && !s.sgHas(i, "paoxiao") && s.sgHegMayInvoke(i, "paoxiao") {
		if err := s.sgHegRevealSkill(i, "paoxiao"); err != nil {
			return err
		}
	}
	skill := ""
	limit := 1
	if sgIsSlash(kind) && s.sgTianyi(i) != 1 {
		skill, limit = "mashu", probe.sgRange(i)
	} else if kind == "snatch" || kind == "supply_shortage" {
		if s.sgHas(i, "qicai") {
			return nil
		}
		if kind == "supply_shortage" && s.sgHas(i, "duanliang") {
			limit = 2
		}
		if s.sgHegMayInvoke(i, "qicai") {
			skill = "qicai"
		} else {
			skill = "mashu"
		}
	}
	if skill != "" && !s.sgHas(i, skill) && s.sgHegMayInvoke(i, skill) {
		withSkill := clone(probe)
		withSkill.sgHegShow(i, []int{withSkill.sgHegSkillSlot(i, skill)}, false)
		needed := false
		for _, target := range targets {
			if target != i && s.sgAlive(target) && probe.sgDistance(i, target) > limit && (skill == "qicai" || withSkill.sgDistance(i, target) <= limit) {
				needed = true
			}
		}
		if needed {
			if err := s.sgHegRevealSkill(i, skill); err != nil {
				return err
			}
		}
	}
	if sgIsSlash(kind) && !s.sgHas(i, "heg_duanbing") && s.sgHegMayInvoke(i, "heg_duanbing") {
		base := 1
		if probe.sgWeapon(i) == "halberd" && len(ids) == 1 && len(p.Hand) == 1 && p.Hand[0] == ids[0] {
			base = 3
		}
		if s.sgTianyi(i) == 1 {
			base++
		}
		if len(targets) == base+1 && probe.sgDistance(i, targets[base]) == 1 {
			return s.sgHegRevealSkill(i, "heg_duanbing")
		}
	}
	return nil
}
