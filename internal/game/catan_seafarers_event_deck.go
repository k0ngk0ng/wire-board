package game

import "errors"

// Private integration entry. The legacy reference deck is deliberately not
// exposed as a verified 2025 component set or a public room option.
func newCatanSeafarersReferenceEvents(n int, setup CatanSeafarersSetup) (*State, error) {
	return newCatanSeafarersReferenceEventsOptions(n, CatanOptions{FiveSix: n > 4}, setup)
}

func newCatanSeafarersReferenceEventsOptions(n int, options CatanOptions, setup CatanSeafarersSetup) (*State, error) {
	s, err := NewCatanSeafarers(n, options, setup, nil)
	if err != nil {
		return nil, err
	}
	s.Catan.EventDeck = &catanEventSession{Catalogue: catanEventReferenceCatalogue, Deck: newCatanEventDeck()}
	if s.Catan.cloth() != nil {
		s.Catan.EventDeck.ClothFallback = CatanEventClothFallbackRules
	}
	if s.Catan.pirateIslands() != nil {
		s.Catan.EventDeck.FleetRules = CatanEventFleetRules
	}
	s.Log = append(s.Log, "内部测试：航海家先完成事件再生产；使用旧版参考事件牌表，尚非已核验的2025正式牌表")
	return s, s.validateCatanEventSession()
}

func (g *Catan) validateSeafarersEventDeck() error {
	sea := g.Seafarers
	if sea == nil {
		return nil
	}
	if g.transportSea() {
		return g.Transport.Map.validateSea(g)
	}
	switch sea.Scenario {
	case "shores", "islands", "six_islands", "fog", "desert", "tribe", "cloth", "pirate_islands", "wonders", "new_world":
	default:
		return errors.New("该航海家剧本尚未接入完整事件牌抽取")
	}
	if g.Two != nil && !g.twoSeafarers() || g.BaseSetup != nil {
		return errors.New("该航海家事件牌多重组合尚未接入")
	}
	setup := CatanSeafarersSetup{Scenario: sea.Scenario, Layout: sea.Layout, Rules: sea.Rules}
	if sea.Scenario == "six_islands" {
		if len(g.Players) < 5 {
			return errors.New("六岛需要五至六位玩家")
		}
		setup.Scenario = "islands"
	} else if sea.Scenario == "islands" && len(g.Players) > 4 {
		return errors.New("五六人不能使用四岛地图")
	}
	n := len(g.Players)
	if g.twoSeafarers() {
		n = 4
	}
	normalized, err := NormalizeCatanSeafarersSetup(n, setup)
	if err != nil || normalized != setup || (sea.Scenario == "fog") != (sea.Fog != nil) || (sea.Scenario == "tribe") != (sea.Tribe != nil) || (sea.Scenario == "pirate_islands") != (sea.PirateIslands != nil) || (sea.Scenario == "cloth") != (sea.Cloth != nil) || (sea.Scenario == "wonders") != (sea.Wonders != nil) || (sea.Scenario == "new_world") != (sea.NewWorld != nil) {
		return errors.New("航海家事件牌剧本或布局记录不一致")
	}
	return g.validateClothSupply()
}
