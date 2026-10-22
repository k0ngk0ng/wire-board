package server

import "time"

// Resume obsolete empty response windows saved before capability filtering.
// Game rules own eligibility; this adapter only restores the appropriate clock.
func (r *Room) resumeSanguoshaResponses(now time.Time) bool {
	if r.Loading != nil || r.Status != "playing" || r.Game == nil || r.Game.Sanguosha == nil {
		return false
	}
	lastActive := r.LastActive
	if lastActive == 0 {
		lastActive = r.Updated
	}
	if lastActive > 0 && lastActive <= now.Add(-roomIdleLimit).Unix() {
		return false // Startup's inactivity sweep closes this room without play.
	}
	turn, round, sequence := r.Game.Turn, r.Game.Round, r.Game.Sanguosha.TurnSequence
	if !r.Game.ResumeSanguoshaResponses() {
		return false
	}
	r.Version++
	r.Updated = now.Unix()
	g := r.Game.Sanguosha
	if r.Game.Finished {
		r.Status, r.TurnDeadline = "finished", 0
		return true
	}
	if g.TurnSequence != sequence || r.Game.Turn != turn || r.Game.Round != round {
		r.SGTimeLeft = turnLimit.Milliseconds()
	}
	if g.Pending != nil {
		r.TurnDeadline = now.Add(sanguoshaPromptLimit(g.Pending.Kind)).UnixMilli()
	} else {
		r.TurnDeadline = now.UnixMilli() + r.SGTimeLeft
	}
	return true
}
