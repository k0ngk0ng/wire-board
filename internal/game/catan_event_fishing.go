package game

import "errors"

// Event numbers replace production dice for both resources and fish. Keep the
// fish queue after card/city effects, before gold and Aqueduct responses.
func (s *State) validateEventFishing() error {
	g := s.Catan
	f := g.Fishing
	if f == nil {
		return nil
	}
	if g.Explorer != nil {
		return s.validateExplorerFishingState()
	}
	if err := g.validateFishing(); err != nil {
		return err
	}
	if g.Two != nil && !g.twoFishing() || g.Rivers != nil && !g.fishingRivers() || g.Caravans != nil && !g.fishingCaravans() || g.Attack != nil && !g.fishingAttack() || g.Transport != nil || g.Explorer != nil || g.BaseSetup != nil {
		return errors.New("该捕鱼事件牌剧本组合尚未接通")
	}
	if (f.Pending != nil) != (s.Phase == "catan_fish_replace") || f.Pending != nil && (g.CardEvent != nil || g.CitiesKnights != nil && (g.CitiesKnights.Event != nil || g.CitiesKnights.Pending != nil)) {
		return errors.New("事件牌的捕鱼回应阶段冲突")
	}
	if g.RollID == 0 {
		if f.LastRollID != -1 {
			return errors.New("尚未抽牌却已记录捕鱼生产")
		}
		return nil
	}
	produced := g.RevealedEvent != nil && g.RevealedEvent.ProductionStarted
	if g.EventDeck.alchemyLatest(g.RollID) {
		produced = g.CitiesKnights != nil && g.CitiesKnights.Event == nil && !s.Finished
	}
	if produced && f.LastRollID != g.RollID || !produced && !s.Finished && f.LastRollID >= g.RollID {
		return errors.New("捕鱼生产次数与当前事件牌不一致")
	}
	return nil
}
