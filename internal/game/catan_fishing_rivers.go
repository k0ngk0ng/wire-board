package game

import "errors"

const CatanFishingRiversRules = "catan-fishing-rivers-2025"

func (g *Catan) fishingRivers() bool {
	return g.Rivers != nil && g.Fishing != nil && g.Fishing.Rivers == CatanFishingRiversRules
}

// Keep the river board, swamps and port inventory; the official combination
// omits every lake. Public option validation lives in NewCatanFishingRivers.
func newCatanFishingRivers(n int, knights bool) (*State, error) {
	o := CatanOptions{FiveSix: n > 4}
	var s *State
	var err error
	if knights {
		s, err = NewCatanRiversCitiesKnights(n, o)
	} else if n == 2 {
		s, err = NewCatanTwoRivers(n, o)
	} else {
		s, err = NewCatanRivers(n, o)
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
	if n == 2 {
		f.Two = CatanTwoFishingRules
		g.Two.Tokens = []int{0, 0}
		g.Two.Bank = 0
		if knights {
			f.TwoKnights = CatanTwoFishingKnightsRules
		}
	}
	f.Map.Lakes = []catanFishingLake{}
	if err = f.Map.addFishingGrounds(g); err != nil {
		return nil, err
	}
	g.Fishing = f
	s.Log = append(s.Log, "渔夫＋河流：不放湖泊，海岸产鱼；6鱼免费建桥并领取3金币，旧靴额外需要1分")
	if n == 2 {
		s.Log = append(s.Log, "本站双人组合：每人起始五枚鱼筹码，不使用贸易筹码；公开分数落后者鱼行动少付1鱼，鱼建桥后照常为中立势力建桥，无法建桥则修路")
	}
	if knights {
		s.Log = append(s.Log, "本站渔夫河流骑士组合：沿用河流骑士13分目标，旧靴需14分；7鱼抽指定类别进步牌，鱼不算资源或商品")
	}
	s.catanScores()
	if err = g.validateRivers(); err != nil {
		return nil, err
	}
	if err = g.validateFishing(); err != nil {
		return nil, err
	}
	return s, s.validateCatanTwo()
}
func (f catanFishingMap) validateRivers(g *Catan) error {
	if !g.fishingRivers() || g.Seafarers != nil || g.Explorer != nil || len(f.Lakes) != 0 || f.SeaRecipe != "" || f.NumberRecipe != "" || len(f.ExtraNumbers) != 0 || len(f.NumberSwaps) != 0 {
		return errors.New("河流捕鱼标记或无湖配置无效")
	}
	if err := g.validateRivers(); err != nil {
		return err
	}
	return f.validateFrame(g)
}

func NewCatanFishingRivers(n int, options CatanOptions, knights bool) (*State, error) {
	o, e := NormalizeCatanOptions(options)
	expected, _ := NormalizeCatanOptions(CatanOptions{FiveSix: n > 4})
	if e != nil || n < 2 || n > 6 || o != expected {
		return nil, errors.New("渔夫河流需要二至六人，五六人启用人数扩充")
	}
	return newCatanFishingRivers(n, knights)
}

func (f *catanFishingMap) addFishingGrounds(g *Catan) error {
	_, _, grounds := catanFishingFrame(len(g.Players) > 4)
	numbers := []int{4, 5, 6, 8, 9, 10}
	if len(g.Players) > 4 {
		numbers = append(numbers, 5, 9)
	}
	shuffle(numbers)
	for i, at := range grounds {
		a, b := catanFishingSide(g, at[0], at[1]), catanFishingSide(g, at[2], at[3])
		if a < 0 || b < 0 {
			return errors.New("河流海岸缺少渔场边")
		}
		ea, eb := g.Edges[a], g.Edges[b]
		joint, first, last := ea.A, ea.B, eb.A
		if joint != eb.A && joint != eb.B {
			joint, first = ea.B, ea.A
		}
		if last == joint {
			last = eb.B
		}
		f.Grounds = append(f.Grounds, catanFishingGround{Number: numbers[i], Edges: [2]int{a, b}, Vertices: [3]int{first, joint, last}})
	}
	return nil
}
