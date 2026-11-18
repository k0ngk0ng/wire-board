package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type realtimeInput struct {
	Actor    int             `json:"actor"`
	Action   json.RawMessage `json:"action"`
	Bot      bool            `json:"bot"`
	AutoPlay bool            `json:"autoPlay"`
}

type realtimeFrame struct {
	Type        string          `json:"type"`
	Deadline    int64           `json:"deadline"`
	Timeouts    []int           `json:"timeouts"`
	Inputs      []realtimeInput `json:"inputs"`
	Timer       bool            `json:"timer"`
	PausedTimer bool            `json:"pausedTimer,omitempty"`
	TimerAt     int64           `json:"timerAt,omitempty"`
}

type realtimeFrameCadence uint8

const (
	realtimeAllFrames realtimeFrameCadence = iota
	realtimeDefaultFrames
	realtimeDeadlineFrames
)

func realtimeFrameDue(room *Room, module gameapi.Module, now time.Time) bool {
	return room.Status == "playing" && room.Loading == nil && room.Module != nil && module.Realtime && module.OnTimer != nil &&
		(!room.Module.Paused || module.OnPausedTimer != nil) && room.Module.Result == nil &&
		room.Module.SystemDeadline > 0 && room.Module.SystemDeadline <= room.gameNow(now).UnixMilli()
}

// Called with s.mu held. The module chooses its frame frequency through the
// persisted SystemDeadline; a 50 ms wakeup never changes ordinary game pacing.
func (s *Server) expireRealtimeModuleTimers(now time.Time) {
	s.expireRealtimeModuleTimersAtCadence(now, realtimeAllFrames)
}

func (s *Server) expireRealtimeModuleTimersAtCadence(now time.Time, cadence realtimeFrameCadence) {
	for _, room := range s.rooms {
		module := s.modules[room.Kind]
		if cadence != realtimeAllFrames && module.RealtimeDeadlineFrames != (cadence == realtimeDeadlineFrames) {
			continue
		}
		if !realtimeFrameDue(room, module, now) {
			continue
		}
		if err := s.runRealtimeModuleFrame(room, now); err != nil {
			log.Printf("realtime frame room %s: %v", room.ID, err)
		}
	}
}

// A single detached frame includes every currently legal
// controlled actor's ordinary Apply action, and the authoritative system timer.
// Callback or critical-commit failure publishes no partial frame. Holding s.mu also
// serializes cancellation/closure/reconnect against input collection and commit;
// no stale bot proposal survives a returning player's Reclaim.
func (s *Server) runRealtimeModuleFrame(room *Room, now time.Time) error {
	module := s.modules[room.Kind]
	if room.Status != "playing" || room.Module == nil || room.Module.Result != nil || room.Loading != nil || !module.Realtime || module.OnTimer == nil || (room.Module.Paused && module.OnPausedTimer == nil) {
		return errors.New("missing realtime clock capability")
	}
	candidate, err := cloneBotRoom(room)
	if err != nil {
		return err
	}
	event, err := computeRealtimeModuleFrame(candidate, module, now)
	if err != nil {
		return err
	}
	return s.commitRealtimeModuleFrame(room, candidate, event, now)
}

func computeRealtimeModuleFrame(next *Room, module gameapi.Module, now time.Time) (realtimeFrame, error) {
	event := realtimeFrame{Type: "realtime_frame", Deadline: next.Module.SystemDeadline, Timeouts: []int{}, Inputs: []realtimeInput{}}
	// Snapshot the candidate list: a previous input can remove an actor, but
	// cannot make the same actor run twice or reorder the loop underneath us.
	actors := slices.Clone(next.Module.Actors)
	for _, actor := range actors {
		if module.BotAction == nil || next.Module.Paused || next.Module.Result != nil {
			break
		}
		if !next.Seats[actor].computerControlled() || !slices.Contains(next.Module.Actors, actor) {
			continue
		}
		ctx := moduleContext(next, actor, next.Version, now)
		action, err := module.BotAction(ctx, *next.Module)
		if err != nil {
			return event, fmt.Errorf("actor %d proposal: %w", actor, err)
		}
		if action == nil {
			continue
		}
		if err = applyModuleRules(next, module, ctx, action); err != nil {
			return event, fmt.Errorf("actor %d action: %w", actor, err)
		}
		seat := next.Seats[actor]
		event.Inputs = append(event.Inputs, realtimeInput{Actor: actor, Action: action, Bot: seat.Bot, AutoPlay: seat.AutoPlay})
		if !seat.TimeoutAutoPlay {
			next.LastActive = now.Unix()
		}
	}
	if next.Module.Result == nil && next.Module.SystemDeadline != 0 && next.Module.SystemDeadline <= next.gameNow(now).UnixMilli() {
		ctx := moduleContext(next, -1, next.Version, now)
		var match gameapi.Match
		var err error
		if next.Module.Paused {
			match, err = runPausedPresentationTimer(module, ctx, *next.Module)
			event.PausedTimer, event.TimerAt = true, ctx.Now.UnixMilli()
		} else {
			match, err = module.OnTimer(ctx, *next.Module)
		}
		if err != nil {
			return event, err
		}
		if match.SystemDeadline != 0 && match.SystemDeadline <= next.gameNow(now).UnixMilli() {
			return event, errors.New("realtime timer must clear or advance its deadline")
		}
		if err = module.ValidateMatch(match, len(next.Seats)); err != nil {
			return event, err
		}
		if err = setModuleResult(next, module, match); err != nil {
			return event, err
		}
		event.Timer = true
	}
	next.Version++
	next.Updated = now.Unix()
	return event, nil
}

func (s *Server) commitRealtimeModuleFrame(room, next *Room, event realtimeFrame, now time.Time) error {
	s.realtimeCheckpoint(room)
	if !realtimeCritical(room, next) {
		s.rooms[next.ID] = next
		s.broadcastGameChange(next)
		return nil
	}
	snapshot, err := json.Marshal(next)
	if err != nil {
		return err
	}
	record, err := json.Marshal(event)
	if err != nil {
		return err
	}
	entry := realtimeJournalEntry{
		job:     &queuedRealtimeInput{input: streamInput{Nonce: fmt.Sprintf("frame-%d", next.Version)}},
		version: next.Version, record: record, at: now.Unix(),
	}
	if err = s.persistRealtimeInputBatch(room, next, snapshot, []realtimeJournalEntry{entry}); err != nil {
		return err
	}

	s.rooms[next.ID] = next
	s.realtimeSaved(next)
	s.broadcastGameChange(next)
	return nil
}
