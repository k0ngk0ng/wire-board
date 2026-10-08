package game

import "errors"

const CatanFishingSeaKnightsRules = "wire-board-fishing-sea-knights-v1"

// Use the accepted fishing map first, then the normal sea knights setup.
// No extra terrain or starting production is inserted by this combination.
func NewCatanFishingCitiesKnightsSeafarers(n int, options CatanOptions, setup CatanSeafarersSetup, world *CatanNewWorldMap) (*State, error) {
	var s *State
	var err error
	if setup.Scenario == "new_world" {
		normalized, e := NormalizeCatanSeafarersSetup(n, setup)
		if e != nil {
			return nil, e
		}
		if normalized.Layout != "prepared" {
			return nil, errors.New("新世界需先确认地图")
		}
		s, err = NewCatanFishingNewWorld(n, options, world)
	} else {
		if world != nil {
			return nil, errors.New("只有新世界使用预设地图")
		}
		s, err = NewCatanFishingSeafarers(n, options, setup, nil)
	}
	if err != nil {
		return nil, err
	}
	if err = s.enableCitiesKnightsSeafarers(); err != nil {
		return nil, err
	}
	s.Catan.Fishing.SeaKnights = CatanFishingSeaKnightsRules
	s.Log[0] = "渔夫＋航海家＋城市与骑士开局"
	if s.Catan.Fishing.Map.SeaRecipe != "" {
		s.Log = append(s.Log, "捕鱼地图沿用对应本站海图配方，湖泊与渔场位置不因加入骑士改变")
	}
	if s.Catan.Fishing.WorldSetup != nil {
		s.Log = append(s.Log, "新世界先轮流放港口，再轮流放渔场，之后开始村庄与城市的起始建设")
	}
	s.Log = append(s.Log, "本站渔夫海图与骑士组合：沿用捕鱼地图与海图骑士起始建设和目标分；鱼不算资源或商品，7鱼抽指定类别顶张进步牌，只有鱼收入仍可使用引水渠；旧靴子额外需要1分")
	return s, s.Catan.validateFishing()
}

func (g *Catan) validateFishingSeaKnights() error {
	f := g.Fishing
	combined := g.Seafarers != nil && g.CitiesKnights != nil
	if combined {
		if f.SeaKnights != CatanFishingSeaKnightsRules || !g.citySeaSupported() || g.Options.Helpers && !g.cityHelpers() {
			return errors.New("渔夫海图骑士规则版本或组合无效")
		}
	} else if f.SeaKnights != "" {
		return errors.New("非海图骑士不能包含渔夫海图骑士规则")
	}
	return nil
}
