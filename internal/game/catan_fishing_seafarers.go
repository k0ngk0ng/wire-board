package game

import "errors"

// Public combinations use verified recipes: all listed scenarios for three/four,
// and dedicated, versioned recipes for five/six. New World has its own
// constructor. Never apply a three/four-player recipe to five/six.
func NewCatanFishingSeafarers(n int, options CatanOptions, setup CatanSeafarersSetup, placements []CatanFishingGroundPlacement) (*State, error) {
	if setup.Scenario != "shores" && setup.Scenario != "islands" && setup.Scenario != "fog" && setup.Scenario != "desert" && setup.Scenario != "tribe" && setup.Scenario != "cloth" && setup.Scenario != "wonders" || n < 3 || n > 6 {
		return nil, errors.New("此捕鱼航海家剧本或人数尚未接入")
	}
	if setup.Scenario == "shores" {
		s, err := newCatanFishingShores(n, options, setup, placements)
		if err == nil {
			s.enableFishingHelpers()
		}
		return s, err
	}
	s, err := NewCatanSeafarers(n, options, setup, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	var m *catanFishingMap
	if n > 4 && fishingExtendedSeaScenario(g.Seafarers.Scenario) {
		m, err = g.makeFishingSeaExtended(placements)
	} else if setup.Scenario == "fog" {
		m, err = g.makeFishingFog(placements)
	} else if setup.Scenario == "desert" {
		m, err = g.makeFishingDesert(placements)
	} else if setup.Scenario == "tribe" {
		m, err = g.makeFishingTribe(placements)
	} else if setup.Scenario == "cloth" {
		m, err = g.makeFishingCloth(placements)
	} else if setup.Scenario == "wonders" {
		m, err = g.makeFishingWonders(placements)
	} else {
		m, err = g.makeFishingFourIslands(placements)
	}
	if err != nil {
		return nil, err
	}
	tokens, err := newCatanFishingTokens(n)
	if err != nil {
		return nil, err
	}
	g.Fishing = &CatanFishing{Map: *m, Tokens: *tokens, LastRollID: -1, Started: make([]bool, n)}
	s.enableFishingHelpers()
	// The 2025 combination explicitly uses scenario setup except for the
	// listed lake/grounds changes: retain the scenario's robber and pirate.
	if n > 4 && fishingExtendedSeaScenario(g.Seafarers.Scenario) {
		s.Log = append(s.Log, "本站五六人捕鱼海图：八处渔场；六岛每岛至少一处、两座小岛各两处，布匹两主岛各四处；沙漠与部落采用固定双湖配方，保留剧本胜利及旧靴规则")
	} else if setup.Scenario == "fog" {
		s.Log = append(s.Log, "迷雾捕鱼：不放湖泊，渔场沿起始岛屿海岸放置；2鱼可驱离海盗，5鱼可造船；12分获胜，持旧靴子需13分")
	} else if setup.Scenario == "desert" {
		s.Log = append(s.Log, "沙漠捕鱼：按剧本替换湖泊并迁移数字，双点数地块在任一点数掷中时产出；14分获胜，持旧靴子需15分")
	} else if setup.Scenario == "tribe" {
		s.Log = append(s.Log, "部落捕鱼：12点麦田替换为湖泊，六处渔场沿主岛海岸放置；强盗只能移至主岛，奖励港口不能覆盖渔场；13分获胜，持旧靴子需14分")
	} else if setup.Scenario == "cloth" {
		s.Log = append(s.Log, "布匹捕鱼：不放湖泊，两座大岛各放三处渔场；第三座起始村庄领取资源和鱼；建立村落贸易后才可用2鱼驱离海盗；14分获胜，持旧靴子需15分，五村耗尽的终局规则保留")
	} else if setup.Scenario == "wonders" {
		s.Log = append(s.Log, "奇迹捕鱼：不放湖泊与海盗，渔场沿大小岛屿海岸放置；建成4级奇迹获胜，或达到10分且奇迹等级领先，持旧靴子时分数门槛为11分")
	} else {
		s.Log = append(s.Log, "四岛捕鱼：不放湖泊，保留四岛起始强盗与海盗；2鱼可驱离海盗，5鱼可造船；13分获胜，持旧靴子需14分")
	}
	return s, nil
}

func (g *Catan) fishingSeaSupported() bool {
	if g.riversSea() && g.fishingRiversSea() {
		return true
	}
	if g.transportSea() && g.fishingTransport() && g.Fishing.Map.SeaRecipe == CatanTransportSeaFishingRules {
		return true
	}
	if g.twoFishingSeafarers() {
		_, err := NormalizeCatanTwoFishingSeafarersSetup(CatanSeafarersSetup{Scenario: g.Seafarers.Scenario, Layout: g.Seafarers.Layout, Rules: g.Seafarers.Rules})
		return err == nil
	}
	if g.Seafarers != nil && len(g.Players) > 4 && fishingExtendedSeaScenario(g.Seafarers.Scenario) {
		return len(g.Players) <= 6 && !g.Seafarers.Variable
	}
	if g.Seafarers != nil && (g.Seafarers.Scenario == "shores" || g.Seafarers.Scenario == "new_world" || g.Seafarers.Scenario == "fog" || g.Seafarers.Scenario == "wonders") {
		return len(g.Players) >= 3 && len(g.Players) <= 6
	}
	return g.Seafarers != nil && (g.Seafarers.Scenario == "islands" || g.Seafarers.Scenario == "cloth" || (g.Seafarers.Scenario == "desert" || g.Seafarers.Scenario == "tribe") && !g.Seafarers.Variable) &&
		len(g.Players) >= 3 && len(g.Players) <= 4
}

func (g *Catan) fishCanRemovePirate(player int) bool {
	return g.Seafarers != nil && g.Seafarers.Pirate >= 0 && g.pirateAllowed(player)
}
