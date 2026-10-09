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
	s := &State{Kind: "catan", Round: 1}
	s.initCatan(2)
	g, b := s.Catan, recipe.Catan
	g.Tiles, g.Vertices, g.Edges, g.HexSize, g.Ports, g.Robber = b.Tiles, b.Vertices, b.Edges, b.HexSize, b.Ports, b.Robber
	g.Seafarers, g.Rivers = b.Seafarers, b.Rivers
	g.Seafarers.Seats = make([]CatanSeafarerSeat, 2)
	g.Rivers.Gold = make([]int, 2)
	g.Two = &CatanTwo{Rules: CatanTwoRules, Seafarers: CatanTwoSeafarersRules, RiversSea: CatanTwoRiversWorldRules, Rolls: []int{}, Tokens: []int{5, 5}, Bank: 10}
	if err = g.prepareTwoSeaNeutrals(); err != nil {
		return nil, err
	}
	g.StartPlayer = catanRandom(2)
	s.Turn, s.Phase = g.StartPlayer, recipe.Phase
	s.Log = append(recipe.Log, catanTwoSeafarersNotice, catanTwoRiversWorldNotice)
	s.catanScores()
	if err = g.validateRivers(); err != nil {
		return nil, err
	}
	return s, s.validateCatanTwo()
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
