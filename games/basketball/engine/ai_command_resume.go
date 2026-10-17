package engine

import "errors"

// A8C1 restores a command-selected strategy. Although the tactic profile uses
// actor/2, its shared 9C6D helper retains actor in X. Preserve the original
// bounded static lookup instead of correcting that index to a team number.
func advanceOriginalCommandResume(result *AIControlDecision, view AIControlView) error {
	if view.HumanTactics == nil {
		return errors.New("original AI command resume requires current human tactic profiles")
	}
	if view.Actor >= 4 || view.BallHolder < 0 || view.BallHolder > 6 {
		return errors.New("original AI command resume holder or actor outside restored domain")
	}
	profile := view.HumanTactics[view.Actor/2]
	if profile >= 6 {
		return errors.New("original AI command resume tactic profile outside source table")
	}
	mode := originalCommandResumePlans[profile][view.Actor][view.BallHolder]
	result.Control.Mode, result.Control.Entered = AIMode(mode&127), mode&128 != 0
	result.ClearCommandStatus = true
	return nil
}
