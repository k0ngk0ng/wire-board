package game

import (
	"errors"
	"slices"
)

// No printed caravan-sea-plus-knights page exists, so the nesting reuses the
// sea knight conversion and is labelled as a site rule.
func NewCatanCaravansSeaCitiesKnights(n int, scenario string, world *CatanNewWorldMap) (*State, error) {
	s, err := NewCatanCaravansSea(n, scenario, world)
	if err != nil {
		return nil, err
	}
	logs := slices.Clone(s.Log)
	if err = s.enableCitiesKnightsSeafarers(); err != nil {
		return nil, err
	}
	g := s.Catan
	g.Caravans.Knights, g.Caravans.SeaKnights = CatanCaravansKnightsRules, CatanCaravansSeaKnightsRules
	if g.Two != nil {
		g.Two.Knights, g.Two.SeaKnights = CatanTwoKnightsRules, CatanTwoSeafarersKnightsRules
	}
	s.Log = append(logs, "本站商队海图骑士适配：沿用印刷商队海图、水源与马车规则，出价改用木材或砖块；加入进步牌、商品与城市改良，蛮族船取代海盗，首次进攻前强盗与蛮族船都不入场，目标分在印刷目标上加 2")
	s.catanScores()
	if err = s.validateCaravans(); err != nil {
		return nil, err
	}
	if g.Two != nil {
		err = s.validateCatanTwo()
	}
	return s, err
}

// NewCatanCaravansSea is the plain caravan sea factory used by rooms and the
// knight and fishing nestings, which add their modules afterwards.
func NewCatanCaravansSea(n int, scenario string, world *CatanNewWorldMap) (*State, error) {
	switch scenario {
	case "shores":
		return NewCatanCaravansShoresSeafarers(n)
	case "islands":
		return NewCatanCaravansIslandsSeafarers(n)
	case "desert":
		return NewCatanCaravansDesertSeafarers(n)
	case "tribe":
		return NewCatanCaravansTribeSeafarers(n)
	case "new_world":
		if world != nil {
			return NewCatanCaravansWorldWithMap(n, world)
		}
		return NewCatanCaravansWorld(n)
	}
	return nil, errors.New("商队海图剧本无效")
}
