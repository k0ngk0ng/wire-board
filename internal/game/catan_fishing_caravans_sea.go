package game

import (
	"errors"
	"slices"
)

// The caravan sea maps have no printed lakes, so the fishing nesting drops the
// lake rule and lays the grounds on the real coast instead.
const CatanFishingCaravansSeaRules = "wire-board-fishing-caravans-sea-v1"

func (g *Catan) fishingCaravansSea() bool {
	return g.caravansSea() && g.fishingCaravans() && g.Fishing.Map.SeaRecipe == CatanFishingCaravansSeaRules
}

func NewCatanFishingCaravansSea(n int, scenario string, world *CatanNewWorldMap, knights bool) (*State, error) {
	var s *State
	var err error
	if knights {
		s, err = NewCatanCaravansSeaCitiesKnights(n, scenario, world)
	} else {
		s, err = NewCatanCaravansSea(n, scenario, world)
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
	f := &CatanFishing{Caravans: CatanFishingCaravansRules, Tokens: *tokens, LastRollID: -1, Started: make([]bool, n)}
	f.Map.Lakes = []catanFishingLake{}
	f.Map.SeaRecipe = CatanFishingCaravansSeaRules
	if f.Map.Grounds, err = caravansSeaFishingGrounds(g); err != nil {
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
	s.Log = append(s.Log, "本站渔夫＋商队海图：不放湖泊，渔场沿实际海岸布置并避开港口与商队出发点；马车仍可沿海边延伸，旧靴额外需要1分")
	if n == 2 {
		s.Log = append(s.Log, "本站双人组合：每人起始五枚鱼筹码，不使用贸易筹码；中立势力不持有鱼，鱼建路后照常安排中立建设")
	}
	if knights {
		s.Log = append(s.Log, "渔夫＋商队海图＋城市骑士：沿用海图骑士起始建设与目标分，出价用木材或砖块，鱼不算资源或商品，7鱼抽指定类别进步牌")
	}
	s.catanScores()
	if err = s.validateCaravans(); err != nil {
		return nil, err
	}
	if err = g.validateFishing(); err != nil {
		return nil, err
	}
	return s, s.validateCatanTwo()
}

func caravansSeaFishingGrounds(g *Catan) ([]catanFishingGround, error) {
	blocked := map[int]bool{}
	for _, p := range g.Ports {
		blocked[p.Edge] = true
	}
	// Wagon routes start on these edges and must stay free of fishing sites.
	for _, start := range g.Caravans.Map.Starts {
		blocked[start.Edge] = true
	}
	return spreadCoastalFishingGrounds(g, blocked, func(id int) bool {
		return g.edgeTerrain(id, true) && g.edgeTerrain(id, false)
	})
}

func (f catanFishingMap) validateCaravansSea(g *Catan) error {
	if !g.fishingCaravansSea() || g.Explorer != nil || len(f.Lakes) != 0 || f.NumberRecipe != "" || len(f.ExtraNumbers) != 0 || len(f.NumberSwaps) != 0 {
		return errors.New("商队海图捕鱼标记或无湖配置无效")
	}
	expected := []int{4, 5, 6, 8, 9, 10}
	if len(g.Players) > 4 {
		expected = append(expected, 5, 9)
	}
	if len(f.Grounds) != len(expected) {
		return errors.New("商队海图渔场数量无效")
	}
	numbers := []int{}
	seen := map[int]bool{}
	starts := map[int]bool{}
	for _, start := range g.Caravans.Map.Starts {
		starts[start.Edge] = true
	}
	for _, ground := range f.Grounds {
		a, b := ground.Edges[0], ground.Edges[1]
		if a < 0 || b < 0 || a >= len(g.Edges) || b >= len(g.Edges) || a == b || seen[a] || seen[b] || starts[a] || starts[b] {
			return errors.New("商队海图渔场边无效")
		}
		if !g.edgeTerrain(a, true) || !g.edgeTerrain(a, false) || !g.edgeTerrain(b, true) || !g.edgeTerrain(b, false) {
			return errors.New("商队海图渔场必须沿海")
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
			return errors.New("商队海图渔场交点无效")
		}
		seen[a], seen[b] = true, true
		numbers = append(numbers, ground.Number)
	}
	slices.Sort(numbers)
	slices.Sort(expected)
	if !slices.Equal(numbers, expected) {
		return errors.New("商队海图渔场点数无效")
	}
	return nil
}
