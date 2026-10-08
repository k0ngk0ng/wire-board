package server

import "time"

// Required choices pause the normal action clock. A gold claimant gets a fresh
// response window; a helper's follow-up exchange stays in the same window.
// Both manual actions and automatic timeout choices go through this transition.
func (r *Room) adjustCatanResponseClock(previousPhase string, previousActor, previousSetupStep int, now time.Time) bool {
	if r.Game.Catan == nil || r.Game.Finished {
		return false
	}
	if r.Game.Catan.Explorer != nil {
		current := r.Game.Phase
		if previousPhase == "catan_explorer_setup" {
			// Every confirmed piece (including two consecutive pieces by the
			// same actor) and the first production turn get their own window.
			r.CatanTimeLeft = 0
			r.startTurnClock(now)
			return true
		}
		endResponse := func(phase string) bool {
			return phase == "catan_explorer_resolve" || phase == "catan_explorer_battle"
		}
		if endResponse(current) {
			// Resolution and all hero rerolls share a deadline. After expiry
			// the server continues one automatic response per tick.
			if !endResponse(previousPhase) {
				r.CatanTimeLeft = 0
				r.startTurnClock(now)
			}
			return true
		}
		if endResponse(previousPhase) {
			r.CatanTimeLeft = 0
			r.startTurnClock(now)
			return true
		}
		response := func(phase string) bool {
			switch phase {
			case "catan_helper", "catan_discard", "catan_fish_replace", "catan_explorer_pirate_place", "catan_explorer_pirate_steal",
				"catan_diplomacy", "catan_espionage", "catan_sabotage", "catan_wedding",
				"catan_treason_remove", "catan_treason_place", "catan_guild_dues", "catan_commercial_harbor",
				"catan_aqueduct", "catan_metropolis", "catan_knight_retreat", "catan_pillage",
				"catan_defender_reward", "catan_progress_discard", "catan_progress_end":
				return true
			}
			return false
		}
		if response(current) {
			if !response(previousPhase) {
				r.CatanTimeLeft = max(0, r.TurnDeadline-now.UnixMilli())
				r.startTurnClock(now)
			} else if current != previousPhase || current != "catan_discard" && r.Game.CatanPendingActor() != previousActor {
				// Each new city responder gets a window. All seven discarders
				// share one; the original action budget survives the full chain.
				r.startTurnClock(now)
			}
			return true // Every discarder shares this response deadline.
		}
		if response(previousPhase) {
			r.TurnDeadline = now.UnixMilli() + r.CatanTimeLeft
			r.CatanTimeLeft = 0
			return true
		}
		// Construction and sailing share the original turn budget. The caller
		// starts a full new clock only when the turn serial/actor changes.
		return false
	}
	endResponse := func(phase string) bool {
		return phase == "catan_transport_move" || phase == "catan_attack_end" || phase == "catan_caravan_bid" || phase == "catan_caravan_vote" || phase == "catan_caravan_place"
	}
	if endResponse(r.Game.Phase) {
		// A two-player winning bidder places two wagons separately. The second
		// confirmed placement is a fresh response even when the actor is unchanged.
		if previousPhase != r.Game.Phase || previousActor != r.Game.CatanPendingActor() || r.Game.Catan.Two != nil && previousPhase == "catan_caravan_place" {
			r.startTurnClock(now)
		}
		return true
	}
	if endResponse(previousPhase) {
		// Battles and caravan votes end an action phase, including paired secondary play.
		// Give the new actor a full turn; never restore the ended action's time.
		r.CatanTimeLeft = 0
		r.startTurnClock(now)
		return true
	}
	if previousPhase == "catan_world_ports" || previousPhase == "catan_world_fish" {
		r.startTurnClock(now)
		return true
	}
	if (previousPhase == "catan_rivers_start" || previousPhase == "catan_cloth_start" || previousPhase == "catan_wonders_start") && r.Game.Phase == "catan_setup_settlement" {
		r.startTurnClock(now)
		return true
	}
	response := func(phase string) bool {
		// All seven discarders share one response window. Include discarding
		// in the same chain as helpers/events so their saved action budget is
		// not replaced by a fresh turn when the last player finishes.
		if phase == "catan_discard" || phase == "catan_attack_card" || phase == "catan_transport_barbarian" {
			return true
		}
		return phase == "catan_two_build" || phase == "catan_two_trade" || phase == "catan_fish_replace" || phase == "catan_card_event" || phase == "catan_diplomacy" || phase == "catan_espionage" || phase == "catan_sabotage" || phase == "catan_wedding" || phase == "catan_treason_remove" || phase == "catan_treason_place" || phase == "catan_guild_dues" || phase == "catan_commercial_harbor" || phase == "catan_helper" || phase == "catan_gold" || phase == "catan_port" || phase == "catan_cloth_steal" || phase == "catan_fleet_reward" || phase == "catan_aqueduct" || phase == "catan_metropolis" || phase == "catan_knight_retreat" || phase == "catan_pillage" || phase == "catan_defender_reward" || phase == "catan_progress_discard" || phase == "catan_progress_end"
	}
	current := r.Game.Phase
	if previousPhase == "catan_discard" && current != previousPhase {
		// Old saves entered discard without recording an action budget. Keep
		// their previous full-turn continuation; newly paused zero budgets
		// are distinguishable and must stay exhausted after a restart.
		if !r.CatanDiscardPaused {
			r.CatanTimeLeft = turnLimit.Milliseconds()
		}
		r.CatanDiscardPaused = false
	}
	if response(current) {
		if current == "catan_discard" && previousPhase != current {
			r.CatanDiscardPaused = true
		}
		if !response(previousPhase) {
			r.CatanTimeLeft = max(0, r.TurnDeadline-now.UnixMilli())
			r.startTurnClock(now)
		} else if current != previousPhase || ((current == "catan_fish_replace" || current == "catan_card_event" || current == "catan_sabotage" || current == "catan_wedding" || current == "catan_gold" || current == "catan_fleet_reward" || current == "catan_aqueduct" || current == "catan_pillage" || current == "catan_defender_reward" || current == "catan_progress_discard") && r.Game.CatanPendingActor() != previousActor) {
			r.startTurnClock(now)
		}
		return true
	}
	if response(previousPhase) {
		// Finishing a setup route after its discovery reward starts the next
		// setup seat (or the first production turn), with a full action clock.
		if previousPhase == "catan_progress_end" || r.Game.Catan.SetupStep != previousSetupStep {
			r.startTurnClock(now)
		} else {
			r.TurnDeadline = now.UnixMilli() + r.CatanTimeLeft
		}
		return true
	}
	return false
}
