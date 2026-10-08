package game

import "errors"

const CatanFishingHelpersRules = "wire-board-fishing-helpers-v1"

func (g *Catan) fishingHelpers() bool {
	return g.Explorer == nil && g.Fishing != nil && g.Options.Helpers && g.Fishing.Helpers == CatanFishingHelpersRules
}
func (s *State) enableFishingHelpers() {
	if !s.Catan.Options.Helpers {
		return
	}
	s.Catan.Fishing.Helpers = CatanFishingHelpersRules
	s.Log = append(s.Log, "本站渔夫助手规则：鱼不算生产资源或七点手牌；迪古尔在无沙漠时驱逐强盗到场外，强盗原在湖泊时任选一张普通资源；卡娅在强盗位于湖泊时同样任选资源")
}
func (s *State) validateFishingHelpers() error {
	g := s.Catan
	if g == nil || g.Explorer != nil || g.Fishing == nil {
		return nil
	}
	f := g.Fishing
	if !g.Options.Helpers {
		if f.Helpers != "" {
			return errors.New("未开启助手却存在渔夫助手规则")
		}
		return nil
	}
	if !g.fishingHelpers() || g.CitiesKnights != nil {
		return errors.New("渔夫助手版本或组合无效")
	}
	if f.Pending != nil && (g.HelperPending != nil || g.GoldPending != nil) {
		return errors.New("换鱼回应不能与助手或金矿回应并存")
	}
	return s.validateEventHelpers()
}
func (g *Catan) fishingHelperDescriptions(rules []CatanHelper) {
	if !g.fishingHelpers() {
		return
	}
	rules[2].Description = "非7点生产未获得资源时，可领一张自选资源；鱼筹码不取消补偿，先完成换鱼和金矿选择。"
	rules[4].Description = "任何玩家生产点数为7时必须使用：超过七张普通资源免弃牌，否则领一张自选资源；鱼筹码不计入手牌。"
	rules[9].Description = "本站渔夫规则：自己生产前或结算后，将强盗赶回沙漠；没有沙漠则移到场外。领取原地块对应资源，原地为金矿或湖泊时任选一张普通资源。"
	rules[10].Description = "领取强盗所在地对应的一张资源；本站渔夫规则：强盗在湖泊、沙漠或金矿时任选一种普通资源。强盗在场外时不能使用。"
}
