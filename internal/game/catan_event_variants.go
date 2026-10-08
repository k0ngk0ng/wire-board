package game

import "errors"

// T&B 2025 p4 permits these variants with event cards without rule changes.
// Validate the actual saved variants independently from the waiting-room draft.
func (s *State) validateEventVariants() error {
	g := s.Catan
	if g.Harbors == nil && g.FriendlyRobber == nil {
		return nil
	}
	if (len(g.Players) < 3 || g.Two != nil) && (!g.twoVariantsAvailable() || g.Two.Variants != CatanTwoVariantsRules) || g.Rivers != nil || g.Caravans != nil || g.Attack != nil || g.Transport != nil || g.Explorer != nil {
		return errors.New("该剧本的事件牌与友善／港口组合尚未接通")
	}
	if h := g.Harbors; h != nil && (h.Rules != CatanHarborsRules || h.Owner < -1 || h.Owner >= len(g.Players) || h.Owner >= 0 && g.Players[h.Owner].Eliminated) {
		return errors.New("事件牌存档中的港口霸主规则或归属无效")
	}
	if f := g.FriendlyRobber; f != nil {
		if f.Rules != CatanFriendlyRobberRules {
			return errors.New("事件牌存档中的友善强盗规则无效")
		}
		if err := s.validateCatanFriendlyFallback(); err != nil {
			return err
		}
	}
	return nil
}
