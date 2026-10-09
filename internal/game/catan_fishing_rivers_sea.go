package game

import (
	"errors"
	"slices"
)

// No printed rivers + seafarers + fishing page exists: the nesting keeps the
// river gold ledger, drops every lake and lays the fishing grounds on the real
// coast of whichever printed river-sea map is selected.
const CatanFishingRiversSeaRules = "wire-board-fishing-rivers-sea-v1"

func (g *Catan) fishingRiversSea() bool {
	return g.riversSea() && g.Fishing != nil && g.Fishing.Rivers == CatanFishingRiversRules && g.Fishing.Map.SeaRecipe == CatanFishingRiversSeaRules
}

func NewCatanFishingRiversSea(n int, setup CatanRiversSeafarersSetup, world *CatanRiversWorldMap, knights bool) (*State, error) {
	var s *State
	var err error
	if knights {
		s, err = NewCatanRiversSeaCitiesKnights(n, setup, world)
	} else {
		s, err = NewCatanRiversSeafarers(n, setup, world)
	}
	if err != nil {
		return nil, err
	}
	g := s.Catan
	tokens, err := newCatanFishingTokens(n)
	if n == 2 {
		tokens, err = newTwoCatanFishTokens()
	}
	if err != nil {
		return nil, err
	}
	f := &CatanFishing{Rivers: CatanFishingRiversRules, Tokens: *tokens, LastRollID: -1, Started: make([]bool, n)}
	f.Map.Lakes = []catanFishingLake{}
	f.Map.SeaRecipe = CatanFishingRiversSeaRules
	if f.Map.Grounds, err = riversSeaFishingGrounds(g); err != nil {
		return nil, err
	}
	f.Map.assignRiversSeaNumbers(n)
	if knights {
		f.SeaKnights = CatanFishingSeaKnightsRules
	}
	if n == 2 {
		f.Two, f.TwoSea = CatanTwoFishingRules, CatanTwoFishingSeafarersRules
		g.Two.Tokens = []int{0, 0}
		g.Two.Bank = 0
		if knights {
			f.TwoKnights = CatanTwoFishingKnightsRules
		}
	}
	g.Fishing = f
	s.Log = append(s.Log, "本站渔夫＋河流海图：不放湖泊，渔场沿实际海岸布置并避开港口；保留河流金币账本与河岸付款，6鱼免费建桥并领取金币，旧靴额外需要1分")
	if n == 2 {
		s.Log = append(s.Log, "本站双人组合：每人起始五枚鱼筹码，不使用贸易筹码；中立势力不持有鱼，鱼建桥后照常安排中立建设")
	}
	if knights {
		s.Log = append(s.Log, "渔夫＋河流海图＋城市骑士：沿用海图骑士起始建设与目标分，鱼不算资源或商品，7鱼抽指定类别进步牌，引水渠只处理鱼")
	}
	s.catanScores()
	if err = g.validateRivers(); err != nil {
		return nil, err
	}
	return s, g.validateFishing()
}

func riversSeaFishingGrounds(g *Catan) ([]catanFishingGround, error) {
	blocked := map[int]bool{}
	for _, p := range g.Ports {
		blocked[p.Edge] = true
	}
	return spreadCoastalFishingGrounds(g, blocked, func(id int) bool {
		return g.edgeTerrain(id, true) && g.edgeTerrain(id, false)
	})
}

// River-sea fishing keeps the printed fishing numbers, then spreads them over
// the derived coastal grounds.
func (f *catanFishingMap) assignRiversSeaNumbers(players int) {
	numbers := []int{4, 5, 6, 8, 9, 10}
	if players > 4 {
		numbers = append(numbers, 5, 9)
	}
	shuffle(numbers)
	for i := range f.Grounds {
		f.Grounds[i].Number = numbers[i]
	}
}

func (f catanFishingMap) validateRiversSea(g *Catan) error {
	if !g.fishingRiversSea() || g.Explorer != nil || len(f.Lakes) != 0 || f.NumberRecipe != "" || len(f.ExtraNumbers) != 0 || len(f.NumberSwaps) != 0 {
		return errors.New("河流海图捕鱼标记或无湖配置无效")
	}
	if err := g.validateRivers(); err != nil {
		return err
	}
	expected := []int{4, 5, 6, 8, 9, 10}
	if len(g.Players) > 4 {
		expected = append(expected, 5, 9)
	}
	if len(f.Grounds) != len(expected) {
		return errors.New("河流海图渔场数量无效")
	}
	numbers := []int{}
	seen := map[int]bool{}
	for _, ground := range f.Grounds {
		a, b := ground.Edges[0], ground.Edges[1]
		if a < 0 || b < 0 || a >= len(g.Edges) || b >= len(g.Edges) || a == b || seen[a] || seen[b] {
			return errors.New("河流海图渔场边无效")
		}
		if !g.edgeTerrain(a, true) || !g.edgeTerrain(a, false) || !g.edgeTerrain(b, true) || !g.edgeTerrain(b, false) {
			return errors.New("河流海图渔场必须沿海")
		}
		ea, eb := g.Edges[a], g.Edges[b]
		joint := -1
		for _, v := range [2]int{ea.A, ea.B} {
			if v == eb.A || v == eb.B {
				joint = v
				break
			}
		}
		if joint < 0 || ground.Vertices[1] != joint {
			return errors.New("河流海图渔场交点无效")
		}
		seen[a], seen[b] = true, true
		numbers = append(numbers, ground.Number)
	}
	slices.Sort(numbers)
	slices.Sort(expected)
	if !slices.Equal(numbers, expected) {
		return errors.New("河流海图渔场点数无效")
	}
	return nil
}
