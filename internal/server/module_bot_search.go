package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type moduleBotResult struct {
	action json.RawMessage
	err    error
}
type moduleBotSearch struct {
	room, match, kind, seat string
	version, actor          int
	expired                 bool
	ctx                     context.Context
	cancel                  context.CancelFunc
	done                    chan moduleBotResult
}

func cloneBotRoom(room *Room) (*Room, error) {
	raw, err := json.Marshal(room)
	if err != nil {
		return nil, err
	}
	var next Room
	err = json.Unmarshal(raw, &next)
	return &next, err
}

func (s *Server) validBotSearch(job *moduleBotSearch) bool {
	r := s.rooms[job.room]
	return r != nil && r.Loading == nil && r.Module != nil && r.Status == "playing" && r.Module.Result == nil &&
		r.Kind == job.kind && r.MatchID == job.match && r.Version == job.version &&
		job.actor >= 0 && job.actor < len(r.Seats) && r.Seats[job.actor].ID == job.seat &&
		r.Seats[job.actor].computerControlled() && slices.Contains(r.Module.Actors, job.actor)
}

// Called under the host lock after committed changes, before notifying clients.
// Exact versions include control changes, so off/on cannot resurrect old work.
func (s *Server) cancelStaleBotSearches() {
	for id, job := range s.botSearches {
		if !s.validBotSearch(job) {
			delete(s.botSearches, id)
			job.cancel()
		}
	}
	for id := range s.botCursor {
		if r := s.rooms[id]; r == nil || r.Status != "playing" {
			delete(s.botCursor, id)
		}
	}
}

func botInput(search *gameapi.BotSearch, ctx gameapi.Context, m gameapi.Match) (input json.RawMessage, err error) {
	defer func() {
		if v := recover(); v != nil {
			input, err = nil, fmt.Errorf("bot input panic: %v", v)
		}
	}()
	input, err = search.Input(ctx, m)
	if err == nil && input != nil && (len(input) > 1<<20 || !json.Valid(input) || string(input) == "null") {
		return nil, errors.New("invalid bot observation")
	}
	return input, err
}

func thinkBot(search *gameapi.BotSearch, ctx context.Context, input json.RawMessage) (result moduleBotResult) {
	defer func() {
		if v := recover(); v != nil {
			result = moduleBotResult{err: fmt.Errorf("bot search panic: %v", v)}
		}
	}()
	result.action, result.err = search.Think(ctx, input)
	if result.err == nil && result.action != nil && (len(result.action) > 64<<10 || !json.Valid(result.action) || string(result.action) == "null") {
		result = moduleBotResult{err: errors.New("invalid bot proposal")}
	}
	// Do not retain storage owned by an engine after it has returned a proposal.
	result.action = append(json.RawMessage(nil), result.action...)
	return result
}

func (s *Server) wakeBotCollector() {
	select {
	case s.botWake <- struct{}{}:
	default:
	}
}

// True means scheduled/already running/idle; false asks the existing fast legal
// fallback to act. Workers never acquire s.mu, touch rooms, or access SQLite.
func (s *Server) startModuleBotSearch(room *Room, now time.Time, expired bool) bool {
	if s.botContext == nil || s.botSlots == nil {
		return false
	}
	if s.botContext.Err() != nil {
		return true
	}
	s.cancelStaleBotSearches()
	if s.botSearches[room.ID] != nil {
		return true
	}
	if len(s.botSlots) == cap(s.botSlots) {
		return false
	}
	m := s.modules[room.Kind]
	actors := room.Module.Actors
	if expired {
		actors = room.Module.TimeoutActors
	}
	if len(actors) == 0 {
		return true
	}
	start := s.botCursor[room.ID] % len(actors)
	for offset := range actors {
		index := (start + offset) % len(actors)
		actor := actors[index]
		if !room.Seats[actor].computerControlled() {
			continue
		}
		input, err := botInput(m.BotSearch, moduleContext(room, actor, room.Version, now), *room.Module)
		if err != nil {
			log.Printf("bot input room %s: %v", room.ID, err)
			return false
		}
		if input == nil {
			continue
		}
		budget := m.BotSearch.Budget
		if budget == 0 {
			budget = time.Second
		}
		ctx, cancel := context.WithTimeout(s.botContext, budget)
		job := &moduleBotSearch{room: room.ID, match: room.MatchID, kind: room.Kind, seat: room.Seats[actor].ID,
			version: room.Version, actor: actor, expired: expired, ctx: ctx, cancel: cancel, done: make(chan moduleBotResult, 1)}
		s.botSearches[room.ID] = job
		s.botCursor[room.ID] = (index + 1) % len(actors)
		s.botSlots <- struct{}{}
		context.AfterFunc(ctx, s.wakeBotCollector)
		// A misbehaving Think may ignore cancellation, but continues occupying its
		// slot until it returns. Timed-out rooms use fallback; goroutines cannot
		// grow without bound. Local process engines must use CommandContext.
		go func(input json.RawMessage) {
			defer func() { <-s.botSlots }()
			job.done <- thinkBot(m.BotSearch, ctx, input)
			s.wakeBotCollector()
		}(append(json.RawMessage(nil), input...))
		return true
	}
	return true
}

// Called only by the timer loop with s.mu held. Late/malformed results are not
// trusted: apply through the module, then commit once with normal action logs.
func (s *Server) collectModuleBotSearches(now time.Time) {
	s.cancelStaleBotSearches()
	if s.botContext == nil || s.botContext.Err() != nil {
		return
	}
	for id, job := range s.botSearches {
		var result moduleBotResult
		select {
		case result = <-job.done:
		default:
			if job.ctx.Err() == nil {
				continue
			}
			result.err = job.ctx.Err()
		}
		if err := job.ctx.Err(); err != nil {
			result.err = err
		}
		delete(s.botSearches, id)
		job.cancel()
		if result.err == nil && result.action == nil {
			continue
		}
		room := s.rooms[id]
		next, err := cloneBotRoom(room)
		if err != nil {
			continue
		}
		if result.err == nil {
			err = s.commitModuleBotDecision(next, job.actor, result.action, now, job.expired)
			if err == nil {
				continue
			}
			result.err = err
		}
		log.Printf("bot search fallback room %s: %v", id, result.err)
		// Failed Apply or transaction must not contaminate the fallback snapshot.
		next, err = cloneBotRoom(room)
		if err != nil {
			continue
		}
		fallback, err := s.modules[room.Kind].BotAction(moduleContext(next, job.actor, next.Version, now), *next.Module)
		if err == nil && fallback != nil {
			err = s.commitModuleBotDecision(next, job.actor, fallback, now, job.expired)
		}
		if err != nil {
			log.Printf("bot search fallback failed room %s: %v", id, err)
		}
	}
}
