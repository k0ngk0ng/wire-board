package game

import "errors"

const CatanTradersVariantsRules = "wire-board-traders-variants-v1"
const CatanFriendlyTradersFallbackRules = "wire-board-friendly-traders-fallback-v1"

func (g *Catan) tradersGame() bool {
	return g != nil && g.Explorer == nil && (g.Rivers != nil || g.Caravans != nil || g.Attack != nil || g.Transport != nil)
}
func (g *Catan) tradersVariants() bool {
	return g.tradersGame() && g.TradersVariants == CatanTradersVariantsRules
}
func (s *State) markCatanTradersVariants() {
	g := s.Catan
	if !g.tradersGame() || g.TradersVariants != "" {
		return
	}
	g.TradersVariants = CatanTradersVariantsRules

}
func (s *State) validateTradersVariants() error {
	g := s.Catan
	enabled := g.Harbors != nil || g.FriendlyRobber != nil
	if g.TradersVariants == "" {
		// Existing caravan-sea saves keep their original variant rules.
		if enabled && g.tradersGame() && !g.caravansSea() {
			return errors.New("商人与蛮族变体缺少组合标记")
		}
		return nil
	}
	if !g.tradersVariants() || !enabled {
		return errors.New("商人与蛮族变体标记无效")
	}
	return nil
}

// Resolve options after the map exists, then commit only a valid marked state.
func (s *State) configureCatanTradersVariant(friendly bool) error {
	next := clone(*s)
	g := next.Catan
	next.markCatanTradersVariants()
	if g.Two != nil && !g.twoVariantsAvailable() {
		return errors.New("此双人剧本与变体的组合无效")
	}
	if friendly {
		next.enableCatanFriendlyRobber()
		if g.Rivers != nil && g.Seafarers == nil && g.Fishing == nil && g.Attack == nil && g.Transport == nil {
			g.FriendlyRobber.Fallback = CatanFriendlyTradersFallbackRules
		}
	} else {
		next.enableCatanHarbors()
	}
	next.markCatanTwoVariants()
	if err := next.validateEventVariants(); err != nil {
		return err
	}
	if err := g.validateRivers(); err != nil {
		return err
	}
	if err := next.validateCaravans(); err != nil {
		return err
	}
	if err := next.validateCatanAttack(); err != nil {
		return err
	}
	if err := next.validateCatanTransport(); err != nil {
		return err
	}
	if err := next.validateCatanTwo(); err != nil {
		return err
	}
	if err := next.validateCatanEventSession(); err != nil {
		return err
	}
	*s = next
	return nil
}
