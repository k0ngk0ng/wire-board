package game

import "slices"

func sgLordSkill(skill string) bool {
	return slices.Contains([]string{"hujia", "jijiang", "jiuyuan", "huangtian", "xueyi", "songwei", "baonue", "ruoyu", "zhiba"}, skill)
}

// Effective skills, allegiance and gender are saved independently of the base
// portrait. Awakening and Huashen therefore work through every existing rule.
func (s *State) sgSkills(i int) []string {
	if i < 0 || i >= len(s.Sanguosha.Players) {
		return nil
	}
	p := s.Sanguosha.Players[i]
	if p.SkillsLost {
		return []string{}
	}
	out := append([]string{}, sgGeneral(p.General).Skills...)
	for _, skill := range append(append(append([]string{}, p.Acquired...), p.Temporary...), p.AvatarSkill) {
		if skill != "" && !slices.Contains(out, skill) {
			out = append(out, skill)
		}
	}
	return out
}
func (s *State) sgKingdom(i int) string {
	if i < 0 || i >= len(s.Sanguosha.Players) {
		return ""
	}
	p := s.Sanguosha.Players[i]
	if p.Avatar != "" && !p.SkillsLost {
		if p.AvatarKingdom != "" {
			return p.AvatarKingdom
		}
		return sgGeneral(p.Avatar).Kingdom
	}
	if p.BaseKingdom != "" {
		return p.BaseKingdom
	}
	return sgGeneral(p.General).Kingdom
}
func (s *State) sgFemale(i int) bool {
	if i < 0 || i >= len(s.Sanguosha.Players) {
		return false
	}
	p := s.Sanguosha.Players[i]
	if p.Avatar != "" && !p.SkillsLost {
		return sgGeneral(p.Avatar).Female
	}
	return sgGeneral(p.General).Female
}
func (s *State) sgAcquire(i int, skills ...string) {
	p := &s.Sanguosha.Players[i]
	for _, skill := range skills {
		if !slices.Contains(p.Acquired, skill) {
			p.Acquired = append(p.Acquired, skill)
		}
	}
}
func (s *State) sgMark(i int, name string) {
	p := &s.Sanguosha.Players[i]
	if p.Marks == nil {
		p.Marks = map[string]int{}
	}
	p.Marks[name]++
}
func (s *State) sgLoseSkills(i int) {
	if !s.sgAlive(i) {
		return
	}
	p := &s.Sanguosha.Players[i]
	p.SkillsLost = true
	p.Acquired, p.Avatars, p.Temporary = nil, nil, nil
	s.sgGodClearStars(i)
	p.Avatar, p.AvatarSkill, p.AvatarKingdom = "", "", ""
	// Losing Buqu clears its pile and may re-enter dying. Field cards remain
	// public, but no longer reduce distance without Tuntian.
	s.Sanguosha.Discard = append(s.Sanguosha.Discard, p.Buqu...)
	p.Buqu = nil
	p.BuquActive = false
	s.sgLog("%s 因断肠失去所有武将技能", s.sgName(i))
	if p.HP <= 0 {
		s.sgEnterDying(SGEvent{Actor: -1, Target: i, Step: s.Turn})
	}
}
