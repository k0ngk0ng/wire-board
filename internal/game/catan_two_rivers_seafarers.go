package game

import (
	"errors"
	"slices"
)

const CatanTwoRiversSeafarersRules = "wire-board-two-rivers-seafarers-v1"
const catanTwoRiversSeafarersNotice = "本站双人河流航海家：采用四人组合地图，两家中立势力各从海岸村庄开始；每回合两次生产，桥梁与船只分别补造同类中立棋子，没有合法位置才修中立道路。中立不领金币、贸易筹码、探索和部落奖励；真人金币与贸易筹码分开记账，沿用地图胜利条件"

func newCatanTwoRiversSeafarers(scenario, layout string) (*State, error) {
	var recipe *State
	var err error
	if scenario == "tribe" && layout == "" {
		recipe, err = newCatanRiversTribe(4)
	} else {
		recipe, err = newCatanRiversPrintedSeaLayout(4, scenario, layout)
	}
	if err != nil {
		return nil, err
	}
	return newCatanTwoRiversRecipe(recipe, CatanTwoRiversSeafarersRules, catanTwoRiversSeafarersNotice)
}

// Transfer map components only; production, wallets and turns always have two seats.
func newCatanTwoRiversRecipe(recipe *State, rules, notice string) (*State, error) {
	s := &State{Kind: "catan", Round: 1}
	s.initCatan(2)
	g, b := s.Catan, recipe.Catan
	g.Tiles, g.Vertices, g.Edges, g.HexSize, g.Ports, g.Robber = b.Tiles, b.Vertices, b.Edges, b.HexSize, b.Ports, b.Robber
	g.Seafarers, g.Rivers, g.DevDeck = b.Seafarers, b.Rivers, b.DevDeck
	g.Seafarers.Seats = make([]CatanSeafarerSeat, 2)
	g.Rivers.Gold = make([]int, 2)
	if tr := g.tribe(); tr != nil {
		tr.Points = make([]int, 2)
		tr.HeldPorts = make([][]int, 2)
	}
	g.Two = &CatanTwo{Rules: CatanTwoRules, Seafarers: CatanTwoSeafarersRules, RiversSea: rules, Rolls: []int{}, Tokens: []int{5, 5}, Bank: 10}
	if err := g.prepareTwoSeaNeutrals(); err != nil {
		return nil, err
	}
	g.StartPlayer = catanRandom(2)
	s.Turn, s.Phase = g.StartPlayer, recipe.Phase
	s.Log = append(recipe.Log, catanTwoSeafarersNotice, notice, "双人河流组合：贸易筹码将强盗退至另一处沼泽，不退到场外，不影响海盗、不偷牌")
	s.catanScores()
	if err := g.validateRivers(); err != nil {
		return nil, err
	}
	return s, s.validateCatanTwo()
}

func (g *Catan) twoRiversSea() bool {
	return g.twoRiversWorld() || g.twoSeafarers() && g.riversSea() && g.Two.RiversSea == CatanTwoRiversSeafarersRules && slices.Contains([]string{"shores", "fog", "desert", "tribe"}, g.Seafarers.Scenario)
}
func (s *State) validateTwoRiversSea() error {
	if !s.Catan.twoRiversSea() {
		return errors.New("双人河流航海家组合无效")
	}
	if err := s.validateCatanTwo(); err != nil {
		return err
	}
	return s.Catan.validateRivers()
}
