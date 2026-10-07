package game

import (
	"errors"
	"slices"
)

// Full map/mission components are required for ships and discoveries, unlike
// earlier isolated production fixtures. Public Apply/configuration stay gated.
func (s *State) validateExplorerCityFlow() error {
	if err := s.validateExplorerCityProduction(); err != nil {
		return err
	}
	if err := s.Catan.Explorer.validateComponents(s.Catan); err != nil {
		return err
	}
	if err := s.validateCatanExplorerPaired(); err != nil {
		return err
	}
	g, x := s.Catan, s.Catan.Explorer
	if !slices.Equal(g.DiscardDue, x.Economy.Turn.Discard) {
		return errors.New("组合主状态与七点弃牌记录不一致")
	}
	if x.Cargo.Turn != nil && x.Cargo.Turn.Sequence == g.TurnSerial && x.Cargo.Turn.Phase != "action" && len(g.CitiesKnights.Players[s.Turn].Progress) > 4 {
		return errors.New("航行前必须将进步牌减至四张")
	}
	return nil
}

func (s *State) catanExplorerCityBeginMovement(player int) error {
	g, x := s.Catan, s.Catan.Explorer
	if err := x.Cargo.beginMovement(g, x.Fleet, player, g.TurnSerial, x.Cargo.farmCount(x.Board, player, "swift")); err != nil {
		return err
	}
	g.Trade = nil
	s.catanExplorerSyncPhase()
	s.catanLog(player, "结束交易建设，开始航行")
	return nil
}

// This transaction owns both the progress-limit response and the Explorer
// portion. It never dispatches through the base game's end-turn handler.
func (s *State) catanExplorerCityFlow(player int, a Action) error {
	if err := s.validateExplorerCityFlow(); err != nil {
		return err
	}
	g := s.Catan
	if s.Finished || player != s.Turn || a.Prompt < 1 || uint64(a.Prompt) != g.TurnSerial || a.Skill != "" {
		return errors.New("不是当前组合航行玩家或行动序号")
	}
	next := clone(*s)
	ng := next.Catan
	switch s.Phase {
	case "catan_progress_end":
		if a.Type != "catan_progress_discard" || a.Choice != "" {
			return errors.New("请先选择超出四张上限的进步牌")
		}
		if err := next.catanDiscardProgress(player, a); err != nil {
			return err
		}
		ng.CitiesKnights.Pending = nil
		if err := next.catanExplorerCityBeginMovement(player); err != nil {
			return err
		}
	case "catan_turn":
		if ng.CitiesKnights.Pending != nil || ng.CitiesKnights.Event != nil || !slices.Contains([]string{"catan_explorer_begin_move", "catan_explorer_ship", "catan_explorer_unit", "catan_explorer_transfer"}, a.Type) {
			return errors.New("不是组合船只建设或进入航行动作")
		}
		if a.Type != "catan_explorer_unit" && a.Choice != "" {
			return errors.New("未知组合船只操作选项")
		}
		if a.Type == "catan_explorer_begin_move" {
			next.catanScores()
			next.catanVictory()
			if !next.Finished {
				ng.Trade = nil
				if len(ng.CitiesKnights.Players[player].Progress) > 4 {
					ng.CitiesKnights.Pending = &CatanCityPending{Kind: "progress_discard", Source: "explorer_movement", Players: []int{player}}
					next.Phase = "catan_progress_end"
				} else if err := next.catanExplorerCityBeginMovement(player); err != nil {
					return err
				}
			}
		} else if err := next.applyCatanExplorer(player, a); err != nil {
			return err
		}
	case "catan_explorer_move", "catan_explorer_resolve", "catan_explorer_battle":
		if a.Choice != "" || !slices.Contains([]string{"catan_explorer_sail", "catan_explorer_wool", "catan_explorer_transfer", "catan_explorer_settle", "catan_explorer_chase", "catan_explorer_land", "catan_explorer_pickup", "catan_explorer_resolve", "catan_explorer_battle", "catan_explorer_fish_roll", "catan_explorer_fish_load", "catan_explorer_fish_deliver", "catan_explorer_spice_land", "catan_explorer_spice_deliver", "catan_end"}, a.Type) {
			return errors.New("当前组合阶段只能航行、装卸、任务或结束航行")
		}
		if err := next.applyCatanExplorer(player, a); err != nil {
			return err
		}
		if !next.Finished {
			next.catanExplorerSyncPhase()
		}
	default:
		return errors.New("当前不能执行组合航行操作")
	}
	if err := next.validateExplorerCityFlow(); err != nil {
		return err
	}
	*s = next
	return nil
}
