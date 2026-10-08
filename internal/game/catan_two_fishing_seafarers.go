package game

import "errors"

const CatanTwoFishingSeafarersRules = "wire-board-two-fishing-seafarers-v1"
const catanTwoFishingSeafarersNotice = "本站双人捕鱼航海家规则：采用四人捕鱼海图，两家中立势力各从合法海岸村庄开始；每人起始五枚鱼筹码（1、1、2、2、3），不用贸易筹码，起始建筑不再领鱼。每回合两次生产，公开分数落后者鱼行动少付1鱼。用鱼造船同样补造中立船，两家都不能造船才改修路；中立不持鱼筹码、不领探索或部落奖励、不与布匹村落贸易。保留原剧本胜利条件及旧靴规则"

func NormalizeCatanTwoFishingSeafarersSetup(setup CatanSeafarersSetup) (CatanSeafarersSetup, error) {
	setup, err := NormalizeCatanTwoSeafarersSetup(setup)
	if err != nil {
		return setup, err
	}
	if (setup.Scenario == "desert" || setup.Scenario == "tribe") && setup.Layout != "fixed" {
		return setup, errors.New("双人沙漠、部落捕鱼组合使用固定布局")
	}
	return setup, nil
}

// Build the existing four-player MAP recipe, then install only its board and
// scenario components into a fresh two-player state. Resource-owning seats,
// discards and production remain two-player throughout the live game.
func NewCatanTwoFishingSeafarers(n int, options CatanOptions, setup CatanSeafarersSetup, world *CatanNewWorldMap) (*State, error) {
	setup, err := NormalizeCatanTwoFishingSeafarersSetup(setup)
	if err != nil {
		return nil, err
	}
	o, err := NormalizeCatanOptions(options)
	if err != nil || n != 2 || !CatanTwoHelpersOptions(setup.Scenario, o) {
		return nil, errors.New("双人捕鱼航海家需要两位玩家，不使用五六人扩充")
	}
	var recipe *State
	if setup.Scenario == "new_world" {
		recipe, err = NewCatanFishingNewWorld(4, CatanOptions{}, world)
	} else {
		if world != nil {
			return nil, errors.New("只有新世界可以指定地形")
		}
		recipe, err = NewCatanFishingSeafarers(4, CatanOptions{}, setup, nil)
	}
	if err != nil {
		return nil, err
	}
	s := &State{Kind: "catan", Round: 1}
	s.initCatan(2)
	g, b := s.Catan, recipe.Catan
	g.Tiles, g.Vertices, g.Edges, g.HexSize, g.Ports, g.Robber = b.Tiles, b.Vertices, b.Edges, b.HexSize, b.Ports, b.Robber
	g.Seafarers, g.DevDeck = b.Seafarers, b.DevDeck
	g.Seafarers.Seats = make([]CatanSeafarerSeat, 2)
	if c := g.cloth(); c != nil {
		c.Held = make([]int, 2)
	}
	if tr := g.tribe(); tr != nil {
		tr.Points = make([]int, 2)
		tr.HeldPorts = make([][]int, 2)
	}
	g.Two = &CatanTwo{Rules: CatanTwoRules, Seafarers: CatanTwoSeafarersRules, Rolls: []int{}, Tokens: []int{0, 0}}
	tokens, err := newTwoCatanFishTokens()
	if err != nil {
		return nil, err
	}
	g.Fishing = &CatanFishing{Two: CatanTwoFishingRules, TwoSea: CatanTwoFishingSeafarersRules, Map: b.Fishing.Map, Tokens: *tokens, WorldSetup: b.Fishing.WorldSetup, Started: make([]bool, 2), LastRollID: -1}
	if err = g.prepareTwoSeaNeutrals(); err != nil {
		return nil, err
	}
	g.StartPlayer = catanRandom(2)
	s.Turn = g.StartPlayer
	s.Phase = recipe.Phase
	s.Log = []string{catanTwoFishingSeafarersNotice}
	if g.cloth() != nil {
		s.Log = append(s.Log, "布匹使用三组起始建设，不授予最长路线；第三座起始村庄仅领取普通资源", catanClothSupplyRule)
	}
	s.catanScores()
	if err = s.enableTwoHelpers(o); err != nil {
		return nil, err
	}
	return s, g.validateFishing()
}

func (g *Catan) twoFishingSeafarers() bool {
	return g.twoSeafarers() && g.twoFishing() && g.Fishing.TwoSea == CatanTwoFishingSeafarersRules
}

// Geometry validators use the printed recipe count. This copy is never a game
// state and is never used for actions, production, scores, turns or views.
func (g *Catan) twoFishingRecipeBoard() *Catan {
	b := *g
	b.Two = nil
	b.Players = make([]CatanPlayer, 4)
	return &b
}
