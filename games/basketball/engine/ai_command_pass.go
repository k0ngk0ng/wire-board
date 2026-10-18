package engine

import "fmt"

// advanceOriginalCommandPass restores bank5 A461/A476/A208. These modes issue
// pass/reaction commands; player actions and the ordinary ball phases execute
// their motion. No teammate position or game outcome is changed here.
func advanceOriginalCommandPass(result *AIControlDecision, view AIControlView) error {
	initial := *result
	next := initial
	if view.Actor >= 4 {
		return fmt.Errorf("command pass actor %d outside original players", view.Actor)
	}
	s := &next.Control
	switch s.Mode {
	case 10:
		if !s.Entered {
			// A461 writes17 directly, even if the selected action already equals17.
			next.Action, next.ActionEntered = ActionPass, false
			s.Mode, s.Entered = 1, false
		}
	case 11:
		s.Entered = true
		if next.Action >= 0x10 && next.Action < 0x14 {
			next.Action, next.ActionEntered = ActionPass, false
			s.Mode, s.Entered, s.Plan = 1, false, 22
		}
	case 30:
		if !s.Entered {
			s.Entered = true
			s.Destination = 0x10
		}
		if e := reactOriginalBall(&next, view); e != nil {
			return e
		}
		// BACC may bind a loose ball during this call. A21C reads the resulting
		// owner, so clearing the command cannot use only the incoming holder.
		if next.ReactionReady && (next.BindBall || uint8(view.BallHolder) < 4) {
			next.ClearCommandStatus = true
		}
	default:
		return fmt.Errorf("command pass mode %d outside original10/11/30", s.Mode)
	}
	*result = next
	return nil
}
