package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Runtime bookkeeping is owned by s.mu. Receipts are bounded per room and never
// turn the movement path into a database lookup or action journal.
type realtimeCheckpoint struct {
	savedVersion int
	savedAt      time.Time
	lastError    error
	receipts     map[realtimeNonce]streamAck
	order        []realtimeNonce
}

func (s *Server) realtimeCheckpoint(room *Room) *realtimeCheckpoint {
	if s.realtimeCheckpoints == nil {
		s.realtimeCheckpoints = make(map[string]*realtimeCheckpoint)
	}
	state := s.realtimeCheckpoints[room.ID]
	if state == nil {
		state = &realtimeCheckpoint{savedVersion: room.Version, savedAt: time.Now(), receipts: make(map[realtimeNonce]streamAck)}
		s.realtimeCheckpoints[room.ID] = state
	}
	return state
}
func (s *Server) realtimeSaved(room *Room) {
	if !s.modules[room.Kind].Realtime {
		return
	}
	state := s.realtimeCheckpoint(room)
	state.savedVersion, state.savedAt, state.lastError = room.Version, time.Now(), nil
}
func (c *realtimeCheckpoint) remember(key realtimeNonce, ack streamAck) {
	const limit = 1024
	if _, exists := c.receipts[key]; exists {
		return
	}
	if len(c.order) == limit {
		delete(c.receipts, c.order[0])
		c.order = c.order[1:]
	}
	c.order = append(c.order, key)
	c.receipts[key] = ack
}
func realtimeCritical(before, after *Room) bool {
	return before.Status != after.Status || before.MatchID != after.MatchID || before.Module == nil || after.Module == nil || before.Module.CheckpointKey != after.Module.CheckpointKey || after.Module.Result != nil
}

// Called under s.mu once a second and at graceful shutdown. Failed saves leave
// savedVersion untouched, so the next attempt saves the newest live state.
func (s *Server) flushRealtimeCheckpoints(now time.Time) error {
	var failures []error
	for id, state := range s.realtimeCheckpoints {
		room := s.rooms[id]
		if room == nil {
			delete(s.realtimeCheckpoints, id)
			continue
		}
		if state.savedVersion == room.Version {
			continue
		}
		raw, err := json.Marshal(room)
		if err == nil {
			err = s.persistRealtimeInputBatch(room, room, raw, nil)
		}
		if err != nil {
			state.lastError = err
			failures = append(failures, fmt.Errorf("room %s checkpoint version %d (last saved %d at %s): %w", id, room.Version, state.savedVersion, state.savedAt.Format(time.RFC3339), err))
			continue
		}
		state.savedVersion, state.savedAt, state.lastError = room.Version, now, nil
	}
	return errors.Join(failures...)
}
