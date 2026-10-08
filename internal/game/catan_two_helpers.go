package game

import "errors"

// Helpers has no published two-player recipe. Keep this adaptation explicit.
const CatanTwoHelpersRules = "wire-board-two-helpers-v1"

const catanTwoHelpersNotice = "本站双人助手规则：两位真人各领一名助手，中立势力不持有助手或手牌；每回合两次生产共用一次助手机会。助手建造先翻面或交换，再完成中立建设；移动旧路不增加中立道路；强制交易只对唯一真人对手进行一次，与贸易筹码分别结算"

// Only the verified base and Fishing recipes currently share ordinary Helpers.
func CatanTwoHelpersOptions(scenario string, o CatanOptions) bool {
	normal, err := NormalizeCatanOptions(o)
	return err == nil && !normal.FiveSix && (normal == (CatanOptions{}) || (scenario == "" || scenario == "fishing") && normal.Helpers)
}

func (g *Catan) twoHelpers() bool {
	return g.Two != nil && g.Two.Helpers == CatanTwoHelpersRules && g.Options.Helpers && !g.Options.FiveSix && len(g.Players) == 2 && g.CitiesKnights == nil && g.Rivers == nil && g.Caravans == nil && g.Attack == nil && g.Transport == nil && g.Seafarers == nil && g.Explorer == nil && (g.Fishing == nil || g.twoFishing())
}

func (s *State) enableTwoHelpers(o CatanOptions) error {
	if o.Helpers {
		s.Catan.Options = o
		s.Catan.Two.Helpers = CatanTwoHelpersRules
		s.initCatanHelpers()
		if s.Catan.Fishing != nil {
			s.enableFishingHelpers()
		}
		s.Log = append(s.Log, catanTwoHelpersNotice)
	}
	return s.validateCatanTwo()
}

func (s *State) validateTwoHelpers() error {
	g, q := s.Catan, s.Catan.Two
	if q == nil {
		return nil
	}
	if !g.Options.Helpers {
		if q.Helpers != "" || q.AfterHelper != "" || g.Options != (CatanOptions{}) {
			return errors.New("未启用双人助手却存在助手规则或选项")
		}
		return s.validateEventHelpers()
	}
	if !g.twoHelpers() {
		return errors.New("双人助手版本或组合无效")
	}
	if normal, err := NormalizeCatanOptions(g.Options); err != nil || normal != g.Options {
		return errors.New("双人助手选项无效")
	}
	if err := s.validateEventHelpers(); err != nil {
		return err
	}
	if g.HelperPending != nil && (q.Pending != nil || q.Trade != nil) {
		return errors.New("双人助手与中立建设或交易回应冲突")
	}
	if q.AfterHelper != "" {
		h := g.HelperPending
		if s.Finished || (q.AfterHelper != "road" && q.AfterHelper != "settlement") || s.Phase != "catan_helper" || h == nil || h.Kind != "exchange" || h.Player != s.Turn || h.Resume != "catan_turn" || len(q.Rolls) != 2 {
			return errors.New("助手后续中立建设无效")
		}
		id := g.Players[s.Turn].Helper.ID
		if q.AfterHelper == "road" && id != 2 || q.AfterHelper == "settlement" && id != 8 {
			return errors.New("助手与后续中立建设不匹配")
		}
	}
	return s.validateFishingHelpers()
}
