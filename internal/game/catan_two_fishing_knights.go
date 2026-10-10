package game

import "errors"

const CatanTwoFishingKnightsRules = "wire-board-two-fishing-knights-v1"
const catanTwoFishingKnightsNotice = "本站双人渔夫骑士规则：沿用双人捕鱼地图，每人起始五枚鱼筹码（1、1、2、2、3），不用贸易筹码，起始建筑不再领鱼，也不能用骑士兑换鱼。每回合两次城市事件与生产，保留中立骑士招募与升级；公开分数落后者鱼行动少付1鱼，7鱼从指定类别牌堆抽顶张进步牌。鱼不算资源或商品，仅产鱼仍可使用引水渠；13分获胜，旧靴另加1分"

func NewCatanTwoFishingCitiesKnights(n int, options CatanOptions) (*State, error) {
	o, optionErr := NormalizeCatanOptions(options)
	if optionErr != nil || n != 2 || !CatanTwoHelpersOptions("cities-knights", o) {
		return nil, errors.New("双人渔夫骑士需要两位玩家；不使用五六人扩充")
	}
	s, err := NewCatanTwoFishing(n, CatanOptions{})
	if err != nil {
		return nil, err
	}
	s.enableCitiesKnights()
	s.Catan.Two.Knights = CatanTwoKnightsRules
	s.Catan.Fishing.TwoKnights = CatanTwoFishingKnightsRules
	s.Log = append(s.Log, catanTwoFishingKnightsNotice)
	s.catanScores()
	if err = s.Catan.validateFishing(); err != nil {
		return nil, err
	}
	return s, s.enableTwoHelpers(o)
}

func (g *Catan) twoFishingKnights() bool {
	return g.twoFishing() && g.twoKnights() && g.Fishing.TwoKnights == CatanTwoFishingKnightsRules && (g.Seafarers == nil || g.twoSeafarersKnights() || g.twoAttackKnights())
}
