package game

import (
	"errors"
	"slices"
)

const CatanFishingCaravansRules = "catan-fishing-caravans-2025"

func (g *Catan) fishingCaravans() bool {
	return g.Caravans != nil && g.Fishing != nil && g.Fishing.Caravans == CatanFishingCaravansRules
}

// Pin an interior lake beside each watering hole. With five/six players the
// second lake and second 2/12 pair are an explicitly labelled site extension.
func fishingCaravanLakes(g *Catan) []int {
	inner, _, _ := catanFishingFrame(len(g.Players) > 4)
	result := []int{}
	for _, hole := range g.Caravans.Map.WateringHoles {
		for _, id := range inner {
			if slices.Contains(g.Caravans.Map.WateringHoles, id) || slices.Contains(result, id) {
				continue
			}
			shared := 0
			for _, v := range g.Tiles[id].Vertices {
				if slices.Contains(g.Tiles[hole].Vertices, v) {
					shared++
				}
			}
			if shared == 2 {
				result = append(result, id)
				break
			}
		}
	}
	return result
}

func fishingCaravanNumbers(g *Catan, lakes []int) (map[int]int, []catanFishingExtraNumber) {
	holes, order, sequence := catanCaravanRecipe(len(g.Players) > 4)
	numbers := []int{}
	for _, number := range sequence {
		if number != 12 {
			numbers = append(numbers, number)
		}
	}
	main := map[int]int{}
	extra := []catanFishingExtraNumber{}
	at := 0
	for _, id := range order {
		if slices.Contains(holes, id) || slices.Contains(lakes, id) {
			main[id] = 0
			continue
		}
		main[id] = numbers[at]
		at++
		if main[id] == 2 {
			extra = append(extra, catanFishingExtraNumber{Tile: id, Number: 12})
		}
	}
	return main, extra
}

func newCatanFishingCaravans(n int, knights bool) (*State, error) {
	options := CatanOptions{FiveSix: n > 4}
	var s *State
	var err error
	if knights {
		s, err = NewCatanCaravansCitiesKnights(n, options)
	} else if n == 2 {
		s, err = NewCatanTwoCaravans(n, options)
	} else {
		s, err = NewCatanCaravans(n, options)
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
	if n == 2 {
		f.Two = CatanTwoFishingRules
		g.Two.Tokens = []int{0, 0}
		g.Two.Bank = 0
		if knights {
			f.TwoKnights = CatanTwoFishingKnightsRules
		}
	}
	lakes := fishingCaravanLakes(g)
	if len(lakes) != len(g.Caravans.Map.WateringHoles) {
		return nil, errors.New("商队缺少邻接水源的湖泊位置")
	}
	for i, id := range lakes {
		if g.Tiles[id].Resource != 0 {
			for j := range g.Tiles {
				if g.Tiles[j].Resource == 0 && !slices.Contains(lakes, j) {
					g.Tiles[j].Resource, g.Tiles[id].Resource = g.Tiles[id].Resource, 0
					break
				}
			}
		}
		if g.Tiles[id].Resource != 0 {
			return nil, errors.New("商队没有可替换的森林")
		}
		g.Tiles[id].Resource = catanLake
		numbers := []int{2, 3, 11, 12}
		if i == 1 {
			numbers = []int{4, 10}
		}
		f.Map.Lakes = append(f.Map.Lakes, catanFishingLake{Tile: id, Numbers: numbers})
	}
	main, extra := fishingCaravanNumbers(g, lakes)
	for id, number := range main {
		g.Tiles[id].Number = number
	}
	f.Map.ExtraNumbers = extra
	if err = f.Map.addFishingGrounds(g); err != nil {
		return nil, err
	}
	g.Fishing = f
	s.Log = append(s.Log, "渔夫＋商队：湖泊替换水源旁的森林，2和12共用地块；12分获胜，持旧靴需13分")
	s.Log = append(s.Log, "本站地图配置：湖泊固定在水源旁的内陆格，其余生产数字沿商队顺序跳过水源与湖泊；五六人各水源旁一湖，第二湖在4或10产鱼，两组2/12数字合并")
	if n == 2 {
		s.Log = append(s.Log, "本站双人组合：每人起始五枚鱼筹码，停用贸易筹码；公开分数落后者鱼行动少付1鱼，中立建设和商队最多两辆规则不变")
	}
	if knights {
		s.Log = append(s.Log, "本站渔夫商队骑士组合：15分获胜，持旧靴需16分；商队以木材和砖块出价，7鱼领取指定类别进步牌")
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

func NewCatanFishingCaravans(n int, options CatanOptions, knights bool) (*State, error) {
	o, err := NormalizeCatanOptions(options)
	expected, _ := NormalizeCatanOptions(CatanOptions{FiveSix: n > 4})
	if err != nil || n < 2 || n > 6 || o != expected {
		return nil, errors.New("渔夫商队需要二至六人，五六人启用人数扩充")
	}
	return newCatanFishingCaravans(n, knights)
}

// Verify the combination's numbers, then reconstruct the unchanged ordinary
// caravan recipe for its geometry, ports, terrain inventory and starting huts.
func (m catanCaravanMap) validateFishing(g *Catan) error {
	if !g.fishingCaravans() || g.Fishing.Rivers != "" || g.Rivers != nil || g.Seafarers != nil || g.Explorer != nil {
		return errors.New("渔夫商队组合标记无效")
	}
	f := g.Fishing.Map
	if f.SeaRecipe != "" || f.NumberRecipe != "" || !slices.Equal(f.NumberSwaps, m.NumberSwaps) || len(m.NumberSwaps) > 0 && !g.caravanKnights() {
		return errors.New("渔夫商队地图版本或数字交换记录无效")
	}
	// Check hole IDs before deriving adjacency from potentially malformed saves.
	holes, order, sequence := catanCaravanRecipe(len(g.Players) > 4)
	if !slices.Equal(m.WateringHoles, holes) || len(g.Tiles) != len(order) {
		return errors.New("渔夫商队水源或地块数量无效")
	}
	lakes := fishingCaravanLakes(g)
	if len(lakes) != len(holes) || len(f.Lakes) != len(lakes) {
		return errors.New("渔夫商队湖泊数量无效")
	}
	for i, id := range lakes {
		numbers := []int{2, 3, 11, 12}
		if i == 1 {
			numbers = []int{4, 10}
		}
		if f.Lakes[i].Tile != id || !slices.Equal(f.Lakes[i].Numbers, numbers) || g.Tiles[id].Resource != catanLake || g.Tiles[id].Number != 0 {
			return errors.New("渔夫商队湖泊位置或生产数字无效")
		}
	}
	main, extra := fishingCaravanNumbers(g, lakes)
	if !slices.Equal(f.ExtraNumbers, extra) {
		return errors.New("渔夫商队必须将12叠放在2上")
	}
	original, err := f.numbersBeforeInvention(g)
	if err != nil {
		return err
	}
	for id, want := range main {
		if original[CatanNumberToken{id, 0}] != want {
			return errors.New("渔夫商队生产数字配置不符")
		}
	}
	board := *g
	board.Tiles = slices.Clone(g.Tiles)
	board.Fishing = nil
	for _, id := range lakes {
		board.Tiles[id].Resource = 0
	}
	at := 0
	for _, id := range order {
		if slices.Contains(holes, id) {
			board.Tiles[id].Number = 0
		} else {
			board.Tiles[id].Number = sequence[at]
			at++
		}
	}
	m.NumberSwaps = nil
	return m.validate(&board)
}

func (f catanFishingMap) validateCaravans(g *Catan) error {
	if !g.fishingCaravans() {
		return errors.New("渔夫商队规则标记无效")
	}
	if err := g.Caravans.Map.validate(g); err != nil {
		return err
	}
	return f.validateFrame(g)
}
