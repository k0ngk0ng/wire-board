package game

import "errors"

const CatanTwoCaravansSeaRules = "wire-board-two-caravans-sea-v1"

func (g *Catan) twoCaravansSea() bool {
	return g.twoSeafarers() && g.caravansSea() && g.Two.CaravansSea == CatanTwoCaravansSeaRules && (g.Seafarers.Scenario == "new_world" || g.Seafarers.Scenario == "islands" || g.Seafarers.Scenario == "shores" || g.Seafarers.Scenario == "desert" || g.Seafarers.Scenario == "tribe")
}

func newCatanTwoCaravansSea(scenario string) (*State, error) {
	var recipe *State
	var err error
	switch scenario {
	case "new_world":
		recipe, err = newCatanCaravansWorld(4)
	case "islands":
		recipe, err = newCatanCaravansIslands(4)
	case "shores":
		recipe, err = newCatanCaravansShoresExtended(4)
	case "desert":
		recipe, err = newCatanCaravansDesertSea(4)
	case "tribe":
		recipe, err = newCatanCaravansTribeSea(4)
	default:
		return nil, errors.New("双人商队海图配方尚未接通")
	}
	if err != nil {
		return nil, err
	}
	return newCatanTwoCaravansSeaRecipe(recipe)
}

func newCatanTwoCaravansSeaRecipe(recipe *State) (*State, error) {
	var err error
	s := &State{Kind: "catan", Round: 1}
	s.initCatan(2)
	g, b := s.Catan, recipe.Catan
	g.Tiles, g.Vertices, g.Edges, g.HexSize, g.Ports, g.Robber = b.Tiles, b.Vertices, b.Edges, b.HexSize, b.Ports, b.Robber
	g.Seafarers, g.Caravans, g.DevDeck = b.Seafarers, b.Caravans, b.DevDeck
	g.Seafarers.Seats = make([]CatanSeafarerSeat, 2)
	if tr := g.tribe(); tr != nil {
		tr.Points = make([]int, 2)
		tr.HeldPorts = make([][]int, 2)
	}
	g.Two = &CatanTwo{Rules: CatanTwoRules, Seafarers: CatanTwoSeafarersRules, CaravansSea: CatanTwoCaravansSeaRules, Rolls: []int{}, Tokens: []int{5, 5}, Bank: 10}
	if err = g.prepareTwoSeaNeutrals(); err != nil {
		return nil, err
	}
	g.StartPlayer = catanRandom(2)
	s.Turn, s.Phase = g.StartPlayer, recipe.Phase
	s.Log = append(recipe.Log, "本站双人商队航海：采用四人组合图，中立海岸开局，每回合两次生产；船补中立船，商队每轮尽量放满两辆；中立不领取部落奖励")
	s.catanScores()
	if err = s.validateCaravans(); err != nil {
		return nil, err
	}
	return s, s.validateCatanTwo()
}
