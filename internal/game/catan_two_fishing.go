package game

import (
	"errors"
	"slices"
)

const CatanTwoFishingRules = "catan-for-two-fishing-2025"

// T&B 2025 p10 replaces the trade-token economy, including all settlement
// rewards, with a fixed starting fish hand. Neutrals and double production
// still follow CATAN for Two; FAQ25 includes fish-funded neutral roads.
func NewCatanTwoFishing(n int, options CatanOptions) (*State, error) {
	o, err := NormalizeCatanOptions(options)
	if err != nil || n != 2 || !CatanTwoHelpersOptions("fishing", o) {
		return nil, errors.New("双人渔夫需要两位玩家，其他组合尚未开放")
	}
	s := &State{Kind: "catan", Round: 1}
	s.initCatan(2)
	g := s.Catan
	g.Two = &CatanTwo{Rules: CatanTwoRules, Rolls: []int{}, Tokens: []int{0, 0}}
	m, err := g.makeFishingMap()
	if err != nil {
		return nil, err
	}
	tokens, err := newTwoCatanFishTokens()
	if err != nil {
		return nil, err
	}
	g.Fishing = &CatanFishing{Two: CatanTwoFishingRules, Map: *m, Tokens: *tokens, LastRollID: -1, Started: make([]bool, 2)}
	if err := g.prepareTwoNeutrals(); err != nil {
		return nil, err
	}
	g.StartPlayer = catanRandom(2)
	s.Turn = g.StartPlayer
	s.catanScores()
	s.Log = append(s.Log, "双人渔夫：每人起始领取五枚鱼筹码（1、1、2、2、3）；不使用贸易筹码，起始村庄不额外领鱼，公开分数落后者鱼行动少付 1 鱼")
	if err := g.validateFishing(); err != nil {
		return nil, err
	}
	return s, s.enableTwoHelpers(o)
}

func newTwoCatanFishTokens() (*catanFishingTokens, error) {
	tokens, err := newCatanFishingTokens(2)
	if err != nil {
		return nil, err
	}
	for p := range 2 {
		for _, value := range []int{1, 1, 2, 2, 3} {
			i := slices.IndexFunc(tokens.DrawPile, func(id int) bool { return catanFishValue(id) == value })
			tokens.Hands[p] = append(tokens.Hands[p], tokens.DrawPile[i])
			tokens.DrawPile = slices.Delete(tokens.DrawPile, i, i+1)
		}
	}
	return tokens, nil
}

func (g *Catan) twoFishing() bool {
	return g.Two != nil && g.Fishing != nil && g.Fishing.Two == CatanTwoFishingRules && len(g.Players) == 2
}

func (g *Catan) fishActionCost(player int, kind string) int {
	cost := catanFishCosts[kind]
	// Hidden VP cards must never change a public price and disclose a hand.
	if cost > 0 && g.twoFishing() && player >= 0 && player < 2 &&
		g.Players[player].Score-g.hiddenVictoryPoints(player) < g.Players[1-player].Score-g.hiddenVictoryPoints(1-player) {
		cost--
	}
	return cost
}
