package game

import "errors"

const CatanTwoRiversWorldRules = "wire-board-two-rivers-world-v1"
const catanTwoRiversWorldNotice = "本站双人河流新世界：采用四人河流地图，两家中立势力各有一座海岸村庄；真人每回合两次生产，造桥后补造中立桥，无合法桥位才补道路。中立不持金币或贸易筹码，金币与贸易筹码分别记账；12分获胜"

// Internal recipe; public configuration and the other sea scenarios retain
// their separate acceptance gates. Never use four live resource-owning seats.
func newCatanTwoRiversWorld(world *CatanRiversWorldMap) (*State, error) {
	var recipe *State
	var err error
	if world == nil {
		recipe, err = newCatanRiversWorld(4)
	} else {
		recipe, err = newCatanRiversWorldWithMap(4, world)
	}
	if err != nil {
		return nil, err
	}
	return newCatanTwoRiversRecipe(recipe, CatanTwoRiversWorldRules, catanTwoRiversWorldNotice)
}

func (g *Catan) twoRiversWorld() bool {
	return g.twoSeafarers() && g.Two.RiversSea == CatanTwoRiversWorldRules && g.riversSea() && g.Seafarers.Scenario == "new_world"
}

func (s *State) validateTwoRiversWorld() error {
	if !s.Catan.twoRiversWorld() {
		return errors.New("双人河流新世界组合无效")
	}
	if err := s.validateCatanTwo(); err != nil {
		return err
	}
	return s.Catan.validateRivers()
}
