package server

import "time"

// Required choices pause the normal action clock. A gold claimant gets a fresh
// response window; a helper's follow-up exchange stays in the same window.
// Both manual actions and automatic timeout choices go through this transition.
func (r *Room) adjustCatanResponseClock(previousPhase string, previousActor, previousSetupStep int, now time.Time) bool {
	if r.Game.Catan == nil || r.Game.Finished {
		return false
	}
	response := func(phase string) bool {
		return phase == "catan_helper" || phase == "catan_gold" || phase == "catan_port" || phase == "catan_cloth_steal"
	}
	current := r.Game.Phase
	if response(current) {
		if !response(previousPhase) {
			r.CatanTimeLeft = max(0, r.TurnDeadline-now.UnixMilli())
			r.startTurnClock(now)
		} else if current != previousPhase || (current == "catan_gold" && r.Game.CatanPendingActor() != previousActor) {
			r.startTurnClock(now)
		}
		return true
	}
	if response(previousPhase) {
		// Finishing a setup route after its discovery reward starts the next
		// setup seat (or the first production turn), with a full action clock.
		if current == "catan_discard" || r.Game.Catan.SetupStep != previousSetupStep {
			r.startTurnClock(now)
		} else {
			r.TurnDeadline = now.UnixMilli() + r.CatanTimeLeft
		}
		return true
	}
	return false
}
