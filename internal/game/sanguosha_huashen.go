package game

import (
	"errors"
	"slices"
)

func sgAvatarSkills(id string) []string {
	out := []string{}
	for _, skill := range sgGeneral(id).Skills {
		if !sgLordSkill(skill) && !slices.Contains([]string{"niepan", "luanwu", "zaoxian", "zhiji", "hunzi", "huashen", "yeyan", "baiyin"}, skill) {
			out = append(out, skill)
		}
	}
	return out
}
func (s *State) sgAcquireAvatars(i, n int) {
	g := s.Sanguosha
	p := &g.Players[i]
	pool := []string{}
	for _, general := range g.generalCatalog() {
		if general.ID == "zuoci" || general.ID == p.General || slices.Contains(p.Avatars, general.ID) {
			continue
		}
		used := false
		for _, who := range s.sgOrder(0) {
			if g.Players[who].General == general.ID {
				used = true
			}
		}
		if !used {
			pool = append(pool, general.ID)
		}
	}
	shuffle(pool)
	n = min(n, len(pool))
	p.Avatars = append(p.Avatars, pool[:n]...)
	s.sgLog("%s 获得 %d 张新化身，现有 %d 张（备选仅本人可见）", s.sgName(i), n, len(p.Avatars))
}
func (s *State) sgAskHuashen(i int, required bool) {
	p := s.Sanguosha.Players[i]
	if len(p.Avatars) == 0 {
		return
	}
	s.sgAsk(i, "huashen", "化身：选择一名化身和其中一个可用技能", SGEvent{Actor: i, Flag: required})
	s.Sanguosha.Pending.Choices = clone(p.Avatars)
}
func (s *State) sgSelectAvatar(i int, a Action, q SGPrompt) error {
	p := &s.Sanguosha.Players[i]
	if a.Choice == "pass" && !q.Event.Flag {
		return nil
	}
	if !s.sgHas(i, "huashen") || !slices.Contains(p.Avatars, a.Choice) {
		return errors.New("请选择你已获得的化身")
	}
	skills := sgAvatarSkills(a.Choice)
	if len(skills) > 0 && !slices.Contains(skills, a.Skill) || len(skills) == 0 && a.Skill != "" {
		return errors.New("化身不能选择主公、限定或觉醒技能")
	}
	oldBuqu := s.sgHas(i, "buqu")
	oldQixing := s.sgHas(i, "qixing")
	p.Avatar = a.Choice
	p.AvatarSkill = a.Skill
	p.AvatarKingdom = ""
	s.sgLog("%s 化身为 %s，获得「%s」", s.sgName(i), sgGeneral(a.Choice).Name, SGSkills[a.Skill].Name)
	if sgGeneral(p.Avatar).Kingdom == "god" {
		s.sgPush(SGEvent{Type: "god_avatar_kingdom", Actor: i})
	}
	if oldQixing && !s.sgHas(i, "qixing") {
		s.sgGodClearStars(i)
	}
	if oldBuqu && !s.sgHas(i, "buqu") {
		s.Sanguosha.Discard = append(s.Sanguosha.Discard, p.Buqu...)
		p.Buqu = nil
		p.BuquActive = false
		if p.HP <= 0 {
			s.sgEnterDying(SGEvent{Actor: -1, Target: i, Step: s.Turn})
		}
	}
	return nil
}
