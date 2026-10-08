package game

import "errors"

const CatanFishingAttackRules = "catan-fishing-attack-2025"

func (g *Catan) fishingAttack() bool {
	return g.Attack != nil && g.Fishing != nil && g.Fishing.Attack == CatanFishingAttackRules
}

// Keep the printed attack map: the official combination has no lakes.
func newCatanFishingAttack(n int, knights ...bool) (*State, error) {
	if n < 2 || n > 6 {
		return nil, errors.New("渔夫蛮族进攻需要二至六人")
	}
	var s *State
	var err error
	city := len(knights) > 0 && knights[0]
	if city {
		s, err = newCatanAttackCityCore(n)
	} else {
		s, err = newCatanAttackState(n, CatanOptions{FiveSix: n > 4})
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
	f := &CatanFishing{Attack: CatanFishingAttackRules, Tokens: *tokens, LastRollID: -1, Started: make([]bool, n)}
	if n == 2 {
		f.Two = CatanTwoFishingRules
		g.Two.Tokens, g.Two.Bank = []int{0, 0}, 0
		if city {
			f.TwoKnights = CatanTwoFishingKnightsRules
		}
	}
	f.Map.Lakes = []catanFishingLake{}
	if err = f.Map.addFishingGrounds(g); err != nil {
		return nil, err
	}
	g.Fishing = f
	if !city {
		s.Log = append(s.Log, "渔夫＋蛮族进攻：不放湖泊；2鱼可在回合末将一名己方骑士移动最多5步，7鱼购买并立即使用蛮族发展卡；被征服建筑不产鱼，12分获胜，旧靴需13分")
	} else {
		s.Log = append(s.Log, "渔夫＋蛮族城市骑士：不放湖泊，被征服建筑不产鱼，鱼筹码面值仅本人可见")
	}
	if n == 2 {
		s.Log = append(s.Log, "本站双人组合：起始五枚鱼筹码，停用贸易筹码；公开分数落后者鱼行动少付1鱼；战斗补偿只领取金币，中立骑士不能用鱼延长移动")
	}
	if city {
		s.Log = append(s.Log, "本站渔夫蛮族骑士组合：13分获胜，旧靴需14分；7鱼领取进步牌；回合末2鱼让未激活的己方骑士最多移动5步，不能激活或驱逐，中立骑士不适用")
	}
	s.catanScores()
	if err = s.validateCatanAttack(); err != nil {
		return nil, err
	}
	if err = g.validateFishing(); err != nil {
		return nil, err
	}
	return s, s.validateCatanTwo()
}

func (f catanFishingMap) validateAttack(g *Catan) error {
	if !g.fishingAttack() || g.CitiesKnights != nil && !g.attackKnights() || g.Seafarers != nil || g.Explorer != nil || g.Rivers != nil || g.Caravans != nil || g.Fishing.Rivers != "" || g.Fishing.Caravans != "" || len(f.Lakes) != 0 || f.SeaRecipe != "" || f.NumberRecipe != "" || len(f.ExtraNumbers) != 0 || len(f.NumberSwaps) != 0 {
		return errors.New("蛮族捕鱼标记或无湖配置无效")
	}
	board := g
	if g.attackKnights() {
		if err := g.Attack.City.validateNumbers(g); err != nil {
			return err
		}
		restored := clone(*g)
		for id, n := range catanAttackBoardRecipe(len(g.Players) > 4).numbers {
			restored.Tiles[id].Number = n
		}
		board = &restored
	}
	if err := g.Attack.Map.validate(board); err != nil {
		return err
	}
	return f.validateFrame(g)
}

// Attack public rooms infer the map and paired turns from the actual seats.
func NewCatanFishingAttack(n int, knights bool) (*State, error) {
	return newCatanFishingAttack(n, knights)
}
