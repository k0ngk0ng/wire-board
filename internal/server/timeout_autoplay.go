package server

import (
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"time"
)

// Freeze only the players who still owe a decision at this shared deadline.
// In particular a response timeout belongs to its responder, not the turn owner.
func (r *Room) timeoutActors() []int {
	g := r.Game
	if g == nil || g.Finished {
		return nil
	}
	if g.Dota != nil {
		return g.DotaActors()
	}
	if g.Sanguosha != nil {
		return g.SanguoshaActors()
	}
	actors := []int{}
	if g.Rail != nil && g.Rail.Setup {
		for p, pending := range g.Rail.SetupPending {
			if len(pending) > 0 {
				actors = append(actors, p)
			}
		}
		return actors
	}
	if g.Catan != nil {
		if g.Phase == "catan_discard" {
			for p, due := range g.Catan.DiscardDue {
				if due > 0 {
					actors = append(actors, p)
				}
			}
			return actors
		}
		if p := g.CatanPendingActor(); p >= 0 {
			return []int{p}
		}
	}
	return []int{g.Turn}
}

// All games use the same persistent computer control after expiry. Applying
// the first decision in the same saved snapshot makes takeover atomic. Later
// substeps keep using runBots, at its visible one-action-per-tick cadence.
func (s *Server) expireToAutoplay(room *Room, now time.Time) {
	actors := room.timeoutActors()
	if len(actors) == 0 {
		return
	}
	raw, _ := json.Marshal(room)
	var next Room
	if err := json.Unmarshal(raw, &next); err != nil {
		log.Printf("restore timeout room %s: %v", room.ID, err)
		return
	}
	changed := false
	for _, p := range actors {
		if p < 0 || p >= len(next.Seats) || next.Seats[p].Left {
			continue
		}
		if !next.Seats[p].Bot && !next.Seats[p].AutoPlay {
			next.Seats[p].AutoPlay = true
			next.Seats[p].TimeoutAutoPlay = true
			next.Game.Log = append(next.Game.Log, fmt.Sprintf("玩家 %d 超时，已自动开启电脑托管，可随时取消", p+1))
			changed = true
		}
	}
	if !changed && next.BotAt > now.UnixMilli() {
		return
	}
	acted := false
	for _, p := range actors {
		if next.Game.Finished || next.TurnDeadline > now.UnixMilli() {
			break
		}
		if p < 0 || p >= len(next.Seats) || !next.Seats[p].computerControlled() || !slices.Contains(next.timeoutActors(), p) {
			continue
		}
		action, err := next.Game.BotAction(p)
		if err == nil {
			err = next.applyGameAction(p, action, now)
		}
		if err != nil {
			log.Printf("timeout autoplay room %s player %d: %v", room.ID, p, err)
			return
		}
		acted = true
	}
	if !changed && !acted {
		return
	}
	if len(next.Game.Log) > 80 {
		next.Game.Log = next.Game.Log[len(next.Game.Log)-80:]
	}
	next.Version++
	next.Updated = now.Unix()
	next.BotAt = now.Add(900 * time.Millisecond).UnixMilli()
	// Forced takeover and its future actions do not refresh inactivity: an
	// abandoned table still closes after a day, even if computers keep playing.
	if err := s.save(&next); err != nil {
		log.Printf("save timeout autoplay room %s: %v", room.ID, err)
		return
	}
	s.rooms[next.ID] = &next
	s.broadcast()
}
