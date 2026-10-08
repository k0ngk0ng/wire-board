package game

import "errors"

const CatanFishingAttackRules = "catan-fishing-attack-2025"

func (g *Catan) fishingAttack() bool {
	return g.Attack != nil && g.Fishing != nil && g.Fishing.Attack == CatanFishingAttackRules
}

// Internal entry until the combined human controls and CK recipe are ready.
// Keep the printed attack map: the official combination has no lakes.
func newCatanFishingAttack(n int) (*State, error) {
	if n < 2 || n > 6 {
		return nil, errors.New("渔夫蛮族进攻需要二至六人")
	}
	s, err := newCatanAttackState(n, CatanOptions{FiveSix: n > 4})
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
	}
	if err = f.Map.addFishingGrounds(g); err != nil {
		return nil, err
	}
	g.Fishing = f
	s.Log = append(s.Log, "渔夫＋蛮族进攻：不放湖泊；2鱼可在回合末将一名己方骑士移动最多5步，7鱼购买并立即使用蛮族发展卡；被征服建筑不产鱼，12分获胜，旧靴需13分")
	if n == 2 {
		s.Log = append(s.Log, "本站双人组合：起始五枚鱼筹码，停用贸易筹码；公开分数落后者鱼行动少付1鱼；战斗每份补偿改为3金币，中立骑士不能用鱼延长移动")
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
	if !g.fishingAttack() || g.CitiesKnights != nil || g.Seafarers != nil || g.Explorer != nil || g.Rivers != nil || g.Caravans != nil || g.Fishing.Rivers != "" || g.Fishing.Caravans != "" || len(f.Lakes) != 0 || f.SeaRecipe != "" || f.NumberRecipe != "" || len(f.ExtraNumbers) != 0 || len(f.NumberSwaps) != 0 {
		return errors.New("蛮族捕鱼标记或无湖配置无效")
	}
	if err := g.Attack.Map.validate(g); err != nil {
		return err
	}
	return f.validateFrame(g)
}
