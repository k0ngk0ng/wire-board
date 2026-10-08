package game

import "errors"

const CatanTwoSeafarersKnightsRules = "wire-board-two-seafarers-knights-v1"
const catanTwoSeafarersKnightsNotice = "本站双人航海家骑士规则：沿用四人海图与两家中立海岸村庄，真人每回合两次完整城市事件与生产。中立可建道路、船和一二级骑士，但不生产、不激活、不防御、不领奖。造船触发中立造船；外交重放旧船不触发。贸易筹码限量20枚，无沙漠时退强盗到场外，不移动海盗；海图目标分加2，保留布匹和奇迹特殊结算"

func (g *Catan) twoSeafarersKnights() bool {
	return g.twoSeafarers() && g.twoKnights() && g.Two.SeaKnights == CatanTwoSeafarersKnightsRules
}

func NewCatanTwoSeafarersCitiesKnights(n int, options CatanOptions, setup CatanSeafarersSetup, world *CatanNewWorldMap, fishing bool) (*State, error) {
	o, optionErr := NormalizeCatanOptions(options)
	if optionErr != nil || n != 2 || !CatanTwoHelpersOptions("cities-knights", o) {
		return nil, errors.New("双人海图骑士需要两位玩家；不使用五六人扩充")
	}
	var s *State
	var err error
	if fishing {
		s, err = NewCatanTwoFishingSeafarers(n, CatanOptions{}, setup, world)
	} else {
		s, err = NewCatanTwoSeafarers(n, CatanOptions{}, setup, world)
	}
	if err != nil {
		return nil, err
	}
	if err = s.enableCitiesKnightsSeafarers(); err != nil {
		return nil, err
	}
	g := s.Catan
	g.Two.Knights, g.Two.SeaKnights = CatanTwoKnightsRules, CatanTwoSeafarersKnightsRules
	s.Log = append(s.Log, catanTwoSeafarersKnightsNotice)
	if fishing {
		g.Fishing.TwoKnights = CatanTwoFishingKnightsRules
		g.Fishing.SeaKnights = CatanFishingSeaKnightsRules
		s.Log = append(s.Log, "捕鱼替代全部贸易筹码：起始五枚鱼，不额外领起始鱼，不能移除骑士换鱼；鱼造船也补造中立船，公开落后者鱼行动少付1鱼，旧靴额外需要1分")
	}
	s.catanScores()
	if err = g.validateFishing(); err != nil {
		return nil, err
	}
	return s, s.enableTwoHelpers(o)
}
