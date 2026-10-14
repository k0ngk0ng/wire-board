package snooker

import (
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const decisionLimit = 120 * time.Second

func (s *State) decisionSeats() []int {
	switch s.Phase {
	case "rolling", "finished":
		return []int{}
	case "frame_end":
		seats := []int{}
		for p, ready := range s.Ready {
			if !ready {
				seats = append(seats, p)
			}
		}
		return seats
	default:
		return []int{s.Turn}
	}
}

// Ball motion never charges human time. Placement and optional votes retain
// the current clock; a new decision gets its own full interval. Between-frame
// readiness is tracked independently, so one player cannot renew the others.
func (s *State) syncClocks(previousPhase string, previousTurn int, now time.Time) {
	if len(s.Due) != s.Players || previousPhase != s.Phase || previousTurn != s.Turn {
		s.Due = make([]int64, s.Players)
	}
	owed := make([]bool, s.Players)
	for _, p := range s.decisionSeats() {
		owed[p] = true
		if s.Due[p] == 0 {
			s.Due[p] = now.Add(decisionLimit).UnixMilli()
		}
	}
	for p := range s.Due {
		if !owed[p] {
			s.Due[p] = 0
		}
	}
}

func restore(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if len(s.Due) > 0 || s.Phase == "finished" {
		return m, nil
	}
	// Legacy v1.12.1 rooms had no decision clock. Initialize once when loaded;
	// existing clocks (including expired ones) must never refresh on reconnect.
	s.syncClocks("", -1, ctx.Now)
	return s.match()
}

func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	p := ctx.Actor
	if p < 0 || p >= s.Players || len(s.Due) != s.Players || s.Due[p] == 0 || s.Due[p] > ctx.Now.UnixMilli() {
		return m, nil
	}
	for _, owed := range s.decisionSeats() {
		if owed == p {
			s.Due[p] = ctx.Now.Add(decisionLimit).UnixMilli()
			return s.match()
		}
	}
	return m, nil
}
