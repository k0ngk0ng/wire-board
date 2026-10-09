package game

import "errors"

// Reserve actual scenario cards at the tribe's printed reward edges. They
// stay out of both draw and discard piles until claimed, so reshuffles cannot
// duplicate rewards. Integer indices preserve the existing hidden-edge view.
var catanAttackTribeCards = []string{"capture", "knighthood", "swift_knight", "treason"}

const CatanAttackTribeRewardRules = "catan-attack-tribe-rewards-2025"

func (s *State) reserveAttackTribeRewards() error {
	g := s.Catan
	if g == nil || g.Attack == nil || g.tribe() == nil {
		return errors.New("蛮族部落奖励缺少组件")
	}
	tr, a := g.tribe(), g.Attack
	if tr.AttackRules != "" || tr.ProgressRules != "" || a.Pending != nil || len(a.Deck) < len(tr.Development) {
		return errors.New("蛮族部落奖励不能重复初始化")
	}
	// Resolve all codes before mutation, keeping rejected initialization atomic.
	codes := make([]int, len(tr.Development))
	for i := range codes {
		card := a.Deck[len(a.Deck)-1-i]
		code := -1
		for j, name := range catanAttackTribeCards {
			if name == card {
				code = j
				break
			}
		}
		if code < 0 {
			return errors.New("蛮族奖励牌无效")
		}
		codes[i] = code
	}
	for i, code := range codes {
		tr.Development[i].Card = code
	}
	a.Deck = a.Deck[:len(a.Deck)-len(codes)]
	tr.AttackRules = CatanAttackTribeRewardRules
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	return nil
}
func (g *Catan) attackTribeReservedCards() (map[string]int, error) {
	result := map[string]int{}
	tr := g.tribe()
	if tr == nil || tr.AttackRules == "" {
		return result, nil
	}
	if tr.AttackRules != CatanAttackTribeRewardRules || tr.ProgressRules != "" || g.Attack == nil {
		return nil, errors.New("蛮族部落奖励版本无效")
	}
	edges := map[int]bool{}
	for _, reward := range tr.Development {
		if reward.Card < 0 || reward.Card >= len(catanAttackTribeCards) || reward.Edge < 0 || reward.Edge >= len(g.Edges) || edges[reward.Edge] {
			return nil, errors.New("蛮族部落奖励牌或位置无效")
		}
		edges[reward.Edge] = true
		result[catanAttackTribeCards[reward.Card]]++
	}
	return result, nil
}

// Put the reserved card on top and enter the ordinary immediate-effect flow.
// Calling route completion must wait for this response before port/neutral work.
func (s *State) claimAttackTribeReward(player, index int) error {
	g := s.Catan
	tr := g.tribe()
	if tr == nil || tr.AttackRules != CatanAttackTribeRewardRules || g.Attack == nil || g.Attack.Pending != nil || player != s.Turn || s.Phase != "catan_turn" || index < 0 || index >= len(tr.Development) {
		return errors.New("当前不能领取蛮族部落奖励")
	}
	if _, err := g.attackTribeReservedCards(); err != nil {
		return err
	}
	card := catanAttackTribeCards[tr.Development[index].Card]
	tr.Development = append(tr.Development[:index], tr.Development[index+1:]...)
	g.Attack.Deck = append(g.Attack.Deck, card)
	s.catanLog(player, "从遗忘部落领取一张蛮族发展卡并立即结算")
	return s.catanAttackDrawCard(player)
}

func (s *State) validateAttackTribeRoute() error {
	g := s.Catan
	if g == nil || g.Attack == nil || g.Attack.TribeRoute == nil {
		return nil
	}
	q := g.Attack.TribeRoute
	t := g.tribe()
	if t == nil || t.AttackRules != CatanAttackTribeRewardRules || g.Attack.Pending == nil || s.Phase != "catan_attack_card" || q.Player != s.Turn || q.Edge < 0 || q.Edge >= len(g.Edges) || !g.Edges[q.Edge].Ship || g.Edges[q.Edge].Owner != q.Player || q.Setup || q.Helper {
		return errors.New("蛮族部落奖励航路接续无效")
	}
	return nil
}
