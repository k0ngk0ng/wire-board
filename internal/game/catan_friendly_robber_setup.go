package game

import "errors"

// Kept separate from the rules actually used by a saved game.
type CatanFriendlyRobberSetup struct {
	Enabled bool   `json:"enabled"`
	Rules   string `json:"rules"`
}

func NormalizeCatanFriendlyRobberSetup(setup CatanFriendlyRobberSetup) (CatanFriendlyRobberSetup, error) {
	if setup.Rules != "" && setup.Rules != CatanFriendlyRobberRules {
		return setup, errors.New("不支持的友善强盗规则版本")
	}
	setup.Rules = CatanFriendlyRobberRules
	return setup, nil
}

// Only verified base layouts (optionally with Harbors) can enable the variant.
func (s *State) ConfigureCatanFriendlyRobber(request CatanFriendlyRobberSetup) error {
	setup, err := NormalizeCatanFriendlyRobberSetup(request)
	if err != nil {
		return err
	}
	if s == nil || s.Catan == nil || s.Kind != "catan" || s.Finished {
		return errors.New("友善强盗需要尚未结束的卡坦岛")
	}
	g := s.Catan
	manualSetup := g.setup() && g.SetupStep == 0 && g.SetupVertex == -1
	fixedSetup := g.BaseSetup != nil && g.BaseSetup.Layout == "fixed" && g.TurnSerial == 1 && g.RollID == 0 && s.Phase == "catan_roll"
	if g.FriendlyRobber != nil || s.Round != 1 || (!manualSetup && !fixedSetup) {
		return errors.New("友善强盗只能在创建游戏时配置")
	}
	if setup.Enabled {
		if g.Seafarers != nil || g.CitiesKnights != nil || g.Options.Helpers || g.Options.AllHelpers {
			return errors.New("友善强盗目前仅核验基础版及港口霸主组合")
		}
		s.enableCatanFriendlyRobber()
	}
	return nil
}
