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

// Apply only to a new base game or a verified sea recipe, optionally with Harbors.
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
		if g.Seafarers != nil && !CatanFriendlySeafarersSupported(len(g.Players), g.Seafarers.Scenario) {
			return errors.New("此人数或剧本暂不支持友善强盗，请更换剧本或关闭此变体")
		}
		s.enableCatanFriendlyRobber()
	}
	return nil
}
