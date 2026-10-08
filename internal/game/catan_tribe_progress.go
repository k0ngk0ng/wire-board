package game

import (
	"errors"
	"slices"
)

const CatanTribeProgressRules = "wire-board-tribe-progress-v1"

const catanTribeProgressNotice = "本站补充规则：从54张进步牌中随机预留部落奖励，领取前隐藏类别和牌面；领取后按进步牌正常时机使用，胜利点牌立即公开，其余牌遵守回合末四张上限。"

// Keep the existing reward edges, replacing their development cards with a
// uniform sample of physical progress cards. Reserved cards leave their decks.
// The version distinguishes the two card domains in persisted Development.
func (g *Catan) initTribeProgress() {
	t, k := g.tribe(), g.CitiesKnights
	if t == nil || k == nil {
		return
	}
	t.ProgressRules = CatanTribeProgressRules
	for i := range t.Development {
		at := catanRandom(len(k.ProgressDecks[0]) + len(k.ProgressDecks[1]) + len(k.ProgressDecks[2]))
		for track, deck := range k.ProgressDecks {
			if at >= len(deck) {
				at -= len(deck)
				continue
			}
			t.Development[i].Card = deck[at]
			k.ProgressDecks[track] = slices.Delete(deck, at, at+1)
			break
		}
	}
}

func (g *Catan) validateTribeProgress() error {
	t, k := g.tribe(), g.CitiesKnights
	if t == nil {
		return nil
	}
	if k == nil {
		if t.ProgressRules != "" {
			return errors.New("普通部落不能包含进步牌奖励版本")
		}
		return nil
	}
	if t.ProgressRules != CatanTribeProgressRules || len(k.Players) != len(g.Players) || len(g.DevDeck)+len(g.DevDiscard) != 0 {
		return errors.New("部落骑士奖励版本或牌组无效")
	}
	counts := make([]int, len(catanProgressRules))
	seen := map[int]bool{}
	limit := 4
	if len(g.Players) > 4 {
		limit = 6
	}
	if len(t.Development) > limit {
		return errors.New("部落预留进步牌过多")
	}
	for _, reward := range t.Development {
		if reward.Card < 0 || reward.Card >= len(counts) || reward.Edge < 0 || reward.Edge >= len(g.Edges) || seen[reward.Edge] || g.Edges[reward.Edge].Owner >= 0 || !g.edgeTerrain(reward.Edge, true) {
			return errors.New("部落进步牌或奖励位置无效")
		}
		seen[reward.Edge] = true
		counts[reward.Card]++
	}
	for track, deck := range k.ProgressDecks {
		for _, card := range deck {
			if card < 0 || card >= len(counts) || catanProgressRules[card].Track != track {
				return errors.New("部落骑士进步牌堆无效")
			}
			counts[card]++
		}
	}
	for player, hand := range k.Players {
		if hand.ProgressPoints != len(hand.PublicProgress) || sum(g.Players[player].Dev)+sum(g.Players[player].NewDev) != 0 {
			return errors.New("部落骑士胜利点或发展牌混用")
		}
		for _, card := range hand.Progress {
			if card < 0 || card >= len(counts) || catanProgressRules[card].Victory {
				return errors.New("部落骑士私有进步牌无效")
			}
			counts[card]++
		}
		for _, card := range hand.PublicProgress {
			if card < 0 || card >= len(counts) || !catanProgressRules[card].Victory {
				return errors.New("部落骑士公开胜利牌无效")
			}
			counts[card]++
		}
	}
	for card, rule := range catanProgressRules {
		if counts[card] != rule.Count {
			return errors.New("部落预留牌与进步牌总库存不守恒")
		}
	}
	return nil
}
