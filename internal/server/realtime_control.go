package server

import (
	"encoding/json"
	"reflect"
	"slices"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Real-time play has no turn obligation or idle takeover. The game's independent
// SystemDeadline still drives starts, selection stages, simulation and results.
func (s *Server) setModule(room *Room, match gameapi.Match) error {
	return setModuleResult(room, s.modules[room.Kind], match)
}

func setModuleResult(room *Room, module gameapi.Module, match gameapi.Match) error {
	if module.Realtime {
		match.Deadline = 0
		match.TimeoutActors = nil
	}
	return room.setModule(match, module)
}

// Old timeout takeovers must not silently become manual autoplay after upgrade.
// Manual autoplay remains selected; returning humans lose only stale controls.
func (s *Server) migrateRealtimeControl(room *Room) (bool, error) {
	module := s.modules[room.Kind]
	if !module.Realtime || room.Module == nil || room.Status != "playing" {
		return false, nil
	}
	before, _ := json.Marshal(room)
	match := *room.Module
	for i := range room.Seats {
		seat := &room.Seats[i]
		if seat.Bot || (seat.AutoPlay && !seat.TimeoutAutoPlay) {
			continue
		}
		seat.AutoPlay, seat.TimeoutAutoPlay = false, false
		if module.Reclaim != nil {
			deadline := match.SystemDeadline
			var err error
			match, err = module.Reclaim(moduleContext(room, i, room.Version, time.Now()), match)
			if err != nil {
				return false, err
			}
			match.SystemDeadline = deadline
		}
	}
	if err := module.ValidateMatch(match, len(room.Seats)); err != nil {
		return false, err
	}
	votes := slices.Clone(room.SaveVotes)
	if err := s.setModule(room, match); err != nil {
		return false, err
	}
	if module.SaveControl != nil && match.Paused {
		room.SaveVotes = votes
	}
	after, _ := json.Marshal(room)
	return !reflect.DeepEqual(before, after), nil
}
