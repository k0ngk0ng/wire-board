package server

import "github.com/k0ngk0ng/wire-board/internal/gameapi"

// Eligibility only becomes more restrictive during a challenge. A temporary
// computer participant or practice flag cannot be removed to restore ratings.
// Human-requested autoplay is intentionally not an exclusion.
func (r *Room) observeChallengeEligibility(match gameapi.Match) {
	if r.RatingPolicy != gameapi.RatingCoopCampaignV1 {
		return
	}
	reason := ""
	if match.HasComputerPlayers {
		reason = "computer"
	}
	for _, seat := range r.Seats {
		if seat.Bot {
			reason = "computer"
		}
	}
	if match.Unrated || match.Result != nil && match.Result.Unrated {
		reason = "practice"
	}
	if reason != "" {
		r.Rated = false
		if r.RatingUnratedReason == "" || reason == "practice" {
			r.RatingUnratedReason = reason
		}
	}
}

// A host termination is separate from the game's final state. Closing or
// expiring a challenge retains the exact module snapshot and records a platform
// failure, rather than inventing a game Result or applying an untrusted action.
func rateCooperativeChallenge(r *Room, record *MatchRecord) bool {
	var result *gameapi.Result
	if r.Module != nil {
		result = r.Module.Result
	}
	cooperative := result != nil && result.Cooperative
	if r.RatingPolicy == "" && !cooperative {
		return false
	}
	record.RatingPolicy = r.RatingPolicy
	record.Cooperative = cooperative || r.RatingPolicy == gameapi.RatingCoopCampaignV1
	record.Rated = false
	switch {
	case r.Status == "finished" && cooperative:
		record.Outcome = "failed"
		if len(result.Winners) == len(r.Seats) {
			record.Outcome = "cleared"
		}
	case r.Status == "closed" && r.RatingPolicy == gameapi.RatingCoopCampaignV1:
		switch {
		case !r.RatingStarted:
			record.Outcome, record.UnratedReason = "cancelled", "not_started"
		case r.CloseReason == "abandoned":
			record.Outcome = "abandoned"
		case r.CloseReason == "inactive":
			record.Outcome = "expired"
		default:
			record.Outcome, record.UnratedReason = "interrupted", "administrative"
		}
	default:
		record.UnratedReason = "unsupported_policy"
		return true
	}
	for i := range record.Players {
		record.Players[i].Rank = 1
	}
	if record.UnratedReason != "" {
		return true
	}
	// Check both the frozen exclusion and the final trusted state. This also
	// describes old, unversioned cooperative archives without opting them in.
	reason := r.RatingUnratedReason
	if r.Module != nil && r.Module.HasComputerPlayers {
		reason = "computer"
	}
	for _, seat := range r.Seats {
		if seat.Bot {
			reason = "computer"
		}
	}
	if record.Unrated {
		reason = "practice"
	}
	if reason == "" {
		switch {
		case r.RatingPolicy != gameapi.RatingCoopCampaignV1:
			reason = "cooperative_unrated"
		case !r.RatingStarted:
			reason = "not_started"
		case !r.Rated:
			reason = "ineligible"
		}
	}
	record.UnratedReason = reason
	if reason != "" {
		return true
	}
	record.Rated = true
	delta := -10
	if record.Outcome == "cleared" {
		delta = 20
	}
	for i := range record.Players {
		record.Players[i].RatingDelta = delta
	}
	return true
}
