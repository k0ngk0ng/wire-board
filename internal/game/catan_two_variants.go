package game

import "errors"

// T&B 2025 permits both variants with CATAN for Two. The rulebook and FAQ
// do not specify neutral participation; keep this supplement explicit in saves.
const CatanTwoVariantsRules = "wire-board-two-variants-v1"

const catanTwoVariantsNotice = "本站双人组合规则：中立势力不受友善强盗保护，不参与港口霸主奖励；中立建筑仍占据交点并阻断路线，最长路线规则不变"

func (g *Catan) twoVariantsAvailable() bool {
	return g.Two != nil && len(g.Players) == 2 && g.Rivers == nil && g.Caravans == nil && g.Attack == nil && g.Transport == nil && g.Explorer == nil && (g.Seafarers == nil || g.twoSeafarers()) && (g.Fishing == nil || g.twoFishing()) && (g.CitiesKnights == nil || g.twoKnights())
}

func (s *State) markCatanTwoVariants() {
	if q := s.Catan.Two; q != nil && q.Variants == "" {
		q.Variants = CatanTwoVariantsRules
		s.Log = append(s.Log, catanTwoVariantsNotice)
	}
}

func (s *State) validateCatanTwoVariants() error {
	g := s.Catan
	if g.Two == nil {
		return nil
	}
	enabled := g.FriendlyRobber != nil || g.Harbors != nil
	if !enabled && g.Two.Variants != "" || enabled && (!g.twoVariantsAvailable() || g.Two.Variants != CatanTwoVariantsRules) {
		return errors.New("双人友善／港口组合或规则版本无效")
	}
	if !enabled {
		return nil
	}
	return s.validateEventVariants()
}
