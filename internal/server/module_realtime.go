package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"
)

type realtimeInput struct {
	Actor    int             `json:"actor"`
	Action   json.RawMessage `json:"action"`
	Bot      bool            `json:"bot"`
	AutoPlay bool            `json:"autoPlay"`
}

type realtimeFrame struct {
	Type     string          `json:"type"`
	Deadline int64           `json:"deadline"`
	Timeouts []int           `json:"timeouts"`
	Inputs   []realtimeInput `json:"inputs"`
	Timer    bool            `json:"timer"`
}

// Called with s.mu held. The module chooses its frame frequency through the
// persisted SystemDeadline; a 50 ms wakeup never changes ordinary game pacing.
func (s *Server) expireRealtimeModuleTimers(now time.Time) {
	for _, room := range s.rooms {
		if !s.modules[room.Kind].Realtime || room.Status != "playing" || room.Module == nil || room.Module.Result != nil || room.Module.SystemDeadline == 0 || room.Module.SystemDeadline > now.UnixMilli() {
			continue
		}
		if err := s.runRealtimeModuleFrame(room, now); err != nil {
			log.Printf("realtime frame room %s: %v", room.ID, err)
		}
	}
}

// A single detached frame includes timeout ownership, every currently legal
// controlled actor's ordinary Apply action, and the authoritative system timer.
// Failure anywhere rolls back both gameplay and ownership. Holding s.mu also
// serializes cancellation/closure/reconnect against input collection and commit;
// no stale bot proposal survives a returning player's Reclaim.
func (s *Server) runRealtimeModuleFrame(room *Room, now time.Time) error {
	module := s.modules[room.Kind]
	if !module.Realtime || module.OnTimer == nil {
		return errors.New("missing realtime clock capability")
	}
	raw, err := json.Marshal(room)
	if err != nil {
		return err
	}
	var next Room
	if err = json.Unmarshal(raw, &next); err != nil {
		return err
	}
	event := realtimeFrame{Type: "realtime_frame", Deadline: next.Module.SystemDeadline, Timeouts: []int{}, Inputs: []realtimeInput{}}
	if module.BotAction != nil && next.Module.Deadline > 0 && next.Module.Deadline <= now.UnixMilli() {
		for _, actor := range next.Module.TimeoutActors {
			seat := &next.Seats[actor]
			if !seat.Left && !seat.Bot && !seat.AutoPlay {
				seat.AutoPlay, seat.TimeoutAutoPlay = true, true
				event.Timeouts = append(event.Timeouts, actor)
			}
		}
	}
	// Snapshot the candidate list: a previous input can remove an actor, but
	// cannot make the same actor run twice or reorder the loop underneath us.
	actors := slices.Clone(next.Module.Actors)
	for _, actor := range actors {
		if module.BotAction == nil || next.Module.Result != nil {
			break
		}
		if !next.Seats[actor].computerControlled() || !slices.Contains(next.Module.Actors, actor) {
			continue
		}
		action, err := module.BotAction(moduleContext(&next, actor, next.Version, now), *next.Module)
		if err != nil {
			return fmt.Errorf("actor %d proposal: %w", actor, err)
		}
		if action == nil {
			continue
		}
		if err = s.applyModule(&next, actor, next.Version, action, now); err != nil {
			return fmt.Errorf("actor %d action: %w", actor, err)
		}
		seat := next.Seats[actor]
		event.Inputs = append(event.Inputs, realtimeInput{Actor: actor, Action: action, Bot: seat.Bot, AutoPlay: seat.AutoPlay})
		if !seat.TimeoutAutoPlay {
			next.LastActive = now.Unix()
		}
	}
	if next.Module.Result == nil && next.Module.SystemDeadline != 0 && next.Module.SystemDeadline <= now.UnixMilli() {
		match, err := module.OnTimer(moduleContext(&next, -1, next.Version, now), *next.Module)
		if err != nil {
			return err
		}
		if match.SystemDeadline != 0 && match.SystemDeadline <= now.UnixMilli() {
			return errors.New("realtime timer must clear or advance its deadline")
		}
		if err = module.ValidateMatch(match, len(next.Seats)); err != nil {
			return err
		}
		if err = next.setModule(match); err != nil {
			return err
		}
		event.Timer = true
	}
	next.Version++
	next.Updated = now.Unix()
	snapshot, err := json.Marshal(&next)
	if err != nil {
		return err
	}
	record, err := json.Marshal(event)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE rooms SET snapshot=? WHERE id=?", snapshot, next.ID); err != nil {
		return err
	}
	if err = archiveGame(tx, &next); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO actions VALUES(?,?,?,?,?,?)", next.ID, "", fmt.Sprintf("frame-%d", next.Version), next.Version, record, now.Unix()); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.rooms[next.ID] = &next
	s.broadcastGameChange(&next)
	return nil
}
