package game

import "errors"

// Kept separate from award ownership, which belongs to the saved game.
type CatanHarborsSetup struct {
	Enabled bool   `json:"enabled"`
	Rules   string `json:"rules"`
}

func NormalizeCatanHarborsSetup(setup CatanHarborsSetup) (CatanHarborsSetup, error) {
	if setup.Rules != "" && setup.Rules != CatanHarborsRules {
		return setup, errors.New("不支持的港口霸主规则版本")
	}
	setup.Rules = CatanHarborsRules
	return setup, nil
}

// Apply only to a freshly constructed game before giving it to players. All
// map constructors run first so their original victory target stays intact.
func (s *State) ConfigureCatanHarbors(request CatanHarborsSetup) error {
	setup, err := NormalizeCatanHarborsSetup(request)
	if err != nil {
		return err
	}
	if s == nil || s.Catan == nil || s.Kind != "catan" || s.Finished {
		return errors.New("港口霸主需要尚未结束的卡坦岛")
	}
	g := s.Catan
	manualSetup := g.setup() && g.SetupStep == 0 && g.SetupVertex == -1
	fixedSetup := g.BaseSetup != nil && g.BaseSetup.Layout == "fixed" && g.TurnSerial == 1 && g.RollID == 0 && s.Phase == "catan_roll"
	if g.Harbors != nil || s.Round != 1 || (!manualSetup && !fixedSetup) {
		return errors.New("港口霸主只能在创建游戏时配置")
	}
	if setup.Enabled {
		if g.tradersGame() {
			return s.configureCatanTradersVariant(false)
		}
		if g.Two != nil && !g.twoVariantsAvailable() {
			return errors.New("此双人剧本的港口霸主组合尚未接通")
		}
		s.enableCatanHarbors()
		s.markCatanTwoVariants()
	}
	return nil
}
