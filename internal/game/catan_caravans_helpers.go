package game

import "errors"

const CatanCaravansHelpersRules = "wire-board-caravans-helpers-v1"

func (g *Catan) caravanSeaHelpers() bool {
	return g.caravansSea() && g.Options.Helpers && g.Caravans.Helpers == CatanCaravansHelpersRules
}
func (s *State) EnableCatanCaravansSeaHelpers(all bool) error {
	g := s.Catan
	if g == nil || !g.caravansSea() || g.SetupStep != 0 || g.Options.Helpers || g.Caravans.Sequence != 0 {
		return errors.New("只能在商队海图开局前启用助手")
	}
	g.Caravans.Helpers = CatanCaravansHelpersRules
	g.Options.Helpers, g.Options.AllHelpers = true, all
	g.Options, _ = NormalizeCatanOptions(g.Options)
	if g.Two != nil {
		g.Two.Helpers = CatanTwoHelpersRules
		s.Log = append(s.Log, catanTwoHelpersNotice)
	}
	s.initCatanHelpers()
	s.Log = append(s.Log, "本站商队助手规则：水源不生产资源；迪古尔无沙漠时可将强盗驱逐到场外，强盗原在水源时任选一张普通资源；卡娅在强盗位于水源时同样任选普通资源。助手建村仍触发回合末商队投票。")
	if err := s.validateCaravans(); err != nil {
		return err
	}
	return s.validateEventHelpers()
}

func (g *Catan) caravanHelperDescriptions(rules []CatanHelper) {
	if !g.caravanSeaHelpers() {
		return
	}
	rules[9].Description = "本站商队规则：将强盗赶回沙漠，无沙漠则移至场外；领取原地块对应资源，原地为水源或金矿时任选一张普通资源。"
	rules[10].Description = "领取强盗所在地对应的一张资源；在水源、沙漠或金矿时任选普通资源，强盗在场外时不能使用。"
}
