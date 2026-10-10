package game

import (
	"errors"
	"slices"
)

// Attack sea maps keep their printed landing, castle and barbarian rules. The
// fishing nesting drops the printed lakes and lays the grounds on the real
// coast, as the other sea fishing nestings do.
const CatanFishingAttackSeaRules = "wire-board-fishing-attack-sea-v1"

func (g *Catan) fishingAttackSea() bool {
	return g.attackSea() && g.fishingAttack() && g.Fishing.Map.SeaRecipe == CatanFishingAttackSeaRules
}

// NewCatanAttackSea is the plain attack sea factory shared with the fishing
// nesting; the room layer maps its scenario ids to recipe names.
func NewCatanAttackSea(n int, scenario string) (*State, error) {
	switch scenario {
	case "shores":
		return NewCatanAttackShores(n)
	case "desert":
		return NewCatanAttackDesert(n)
	case "tribe":
		return NewCatanAttackTribe(n)
	case "wonders":
		return NewCatanAttackWonders(n)
	case "pirates":
		return NewCatanAttackPirates(n)
	}
	return nil, errors.New("蛮族海图剧本无效")
}

func NewCatanFishingAttackSea(n int, scenario string) (*State, error) {
	s, err := NewCatanAttackSea(n, scenario)
	if err != nil {
		return nil, err
	}
	return attachAttackSeaFishing(s, scenario)
}

// NewCatanFishingAttackSeaCitiesKnights is the combined nesting: printed
// attack sea map, road knights and the lake-free coastal fishing grounds.
func NewCatanFishingAttackSeaCitiesKnights(n int, scenario string) (*State, error) {
	s, err := NewCatanAttackSeaCitiesKnights(n, scenario)
	if err != nil {
		return nil, err
	}
	return attachAttackSeaFishing(s, scenario)
}

func attachAttackSeaFishing(s *State, scenario string) (*State, error) {
	n := len(s.Catan.Players)
	g := s.Catan
	tokens, err := newCatanFishingTokens(n)
	if n == 2 {
		tokens, err = newTwoCatanFishTokens()
	}
	if err != nil {
		return nil, err
	}
	f := &CatanFishing{Attack: CatanFishingAttackRules, Tokens: *tokens, LastRollID: -1, Started: make([]bool, n)}
	if n == 2 {
		f.Two, f.TwoSea = CatanTwoFishingRules, CatanTwoFishingSeafarersRules
		g.Two.Tokens, g.Two.Bank = []int{0, 0}, 0
	}
	if g.CitiesKnights != nil {
		f.SeaKnights = CatanFishingSeaKnightsRules
		if n == 2 {
			f.TwoKnights = CatanTwoFishingKnightsRules
		}
	}
	f.Map.Lakes = []catanFishingLake{}
	f.Map.SeaRecipe = CatanFishingAttackSeaRules
	if f.Map.Grounds, err = attackSeaFishingGrounds(g); err != nil {
		return nil, err
	}
	f.Map.assignRiversSeaNumbers(n)
	g.Fishing = f
	s.Log = append(s.Log, "本站渔夫＋蛮族海图：不放湖泊，渔场沿实际海岸布置并避开港口；被征服建筑不产鱼，2鱼在回合末延长己方骑士移动最多5步，7鱼购买并立即使用蛮族发展卡")
	if n == 2 {
		s.Log = append(s.Log, "本站双人组合：起始五枚鱼筹码，停用贸易筹码；公开分数落后者鱼行动少付1鱼，中立骑士不能用鱼延长移动")
	}
	s.catanScores()
	if err = s.validateCatanAttack(); err != nil {
		return nil, err
	}
	return s, g.validateFishing()
}

func attackSeaFishingGrounds(g *Catan) ([]catanFishingGround, error) {
	blocked := map[int]bool{}
	for _, p := range g.Ports {
		blocked[p.Edge] = true
	}
	return spreadCoastalFishingGrounds(g, blocked, func(id int) bool {
		return g.edgeTerrain(id, true) && g.edgeTerrain(id, false)
	})
}

func (f catanFishingMap) validateAttackSea(g *Catan) error {
	if !g.fishingAttackSea() || g.Explorer != nil || len(f.Lakes) != 0 || f.NumberRecipe != "" || len(f.ExtraNumbers) != 0 || len(f.NumberSwaps) != 0 {
		return errors.New("蛮族海图捕鱼标记或无湖配置无效")
	}
	expected := []int{4, 5, 6, 8, 9, 10}
	if len(g.Players) > 4 {
		expected = append(expected, 5, 9)
	}
	if len(f.Grounds) != len(expected) {
		return errors.New("蛮族海图渔场数量无效")
	}
	numbers := []int{}
	seen := map[int]bool{}
	for _, ground := range f.Grounds {
		a, b := ground.Edges[0], ground.Edges[1]
		if a < 0 || b < 0 || a >= len(g.Edges) || b >= len(g.Edges) || a == b || seen[a] || seen[b] {
			return errors.New("蛮族海图渔场边无效")
		}
		if !g.edgeTerrain(a, true) || !g.edgeTerrain(a, false) || !g.edgeTerrain(b, true) || !g.edgeTerrain(b, false) {
			return errors.New("蛮族海图渔场必须沿海")
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
			return errors.New("蛮族海图渔场交点无效")
		}
		seen[a], seen[b] = true, true
		numbers = append(numbers, ground.Number)
	}
	slices.Sort(numbers)
	slices.Sort(expected)
	if !slices.Equal(numbers, expected) {
		return errors.New("蛮族海图渔场点数无效")
	}
	return nil
}
