package server

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const deadlineWorkerLimit = 4
const deadlineConflictLimit = 2

func (s *Server) scheduleDeadlineFrames(ctx context.Context, now time.Time) {
	if ctx.Err() != nil || len(s.deadlineFrames) >= deadlineWorkerLimit {
		return
	}
	if s.deadlineFrames == nil {
		s.deadlineFrames = make(map[string]bool)
		s.deadlineRetries = make(map[string]int)
	}
	for id := range s.deadlineRetries {
		if current := s.rooms[id]; current == nil || current.Status != "playing" {
			delete(s.deadlineRetries, id)
		}
	}
	var ready []*Room
	for _, room := range s.rooms {
		module := s.modules[room.Kind]
		if module.RealtimeDeadlineFrames && !s.deadlineFrames[room.ID] && realtimeFrameDue(room, module, now) {
			ready = append(ready, room)
		}
	}
	sort.Slice(ready, func(i, j int) bool {
		a, b := ready[i], ready[j]
		dueA, dueB := a.Module.SystemDeadline+a.ClockOffsetMS, b.Module.SystemDeadline+b.ClockOffsetMS
		return dueA < dueB || dueA == dueB && a.ID < b.ID
	})
	for _, room := range ready {
		if len(s.deadlineFrames) >= deadlineWorkerLimit {
			break
		}
		if s.deadlineRetries[room.ID] >= deadlineConflictLimit {
			delete(s.deadlineRetries, room.ID)
			if err := s.runRealtimeModuleFrame(room, time.Now()); err != nil {
				log.Printf("realtime fallback frame room %s: %v", room.ID, err)
			}
			continue
		}
		detached, err := cloneBotRoom(room)
		if err != nil {
			log.Printf("detach realtime frame room %s: %v", room.ID, err)
			continue
		}
		s.deadlineFrames[room.ID] = true
		s.deadlineWG.Add(1)
		go s.runDeadlineFrame(ctx, room.ID, room, detached, s.modules[room.Kind])
	}
}

func (s *Server) runDeadlineFrame(ctx context.Context, id string, base, candidate *Room, module gameapi.Module) {
	defer s.deadlineWG.Done()
	now := time.Now()
	version, matchID := candidate.Version, candidate.MatchID
	event, err := computeDeadlineFrame(candidate, module, now)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.deadlineFrames, id)
	if ctx.Err() != nil {
		return
	}
	current := s.rooms[id]
	if current != base || current.Version != version || current.MatchID != matchID || !s.modules[current.Kind].RealtimeDeadlineFrames {
		if current != nil && current.Status == "playing" && current.MatchID == matchID {
			s.deadlineRetries[id]++
		} else {
			delete(s.deadlineRetries, id)
		}
		return
	}
	delete(s.deadlineRetries, id)
	if err == nil {
		err = s.commitRealtimeModuleFrame(current, candidate, event, now)
	}
	if err != nil {
		log.Printf("realtime deadline frame room %s: %v", id, err)
	}
}

func computeDeadlineFrame(candidate *Room, module gameapi.Module, now time.Time) (event realtimeFrame, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("realtime rule panic: %v", failure)
		}
	}()
	return computeRealtimeModuleFrame(candidate, module, now)
}
