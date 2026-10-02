package server

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func (r *Room) applyGameAction(player int, action game.Action, now time.Time) error {
	turn, round := r.Game.Turn, r.Game.Round
	setup := r.Game.Rail != nil && r.Game.Rail.Setup
	if err := r.Game.Apply(player, action); err != nil {
		return err
	}
	if r.Game.Finished {
		r.Status = "finished"
	}
	if r.Game.Turn != turn || r.Game.Round != round || r.Game.Finished || (setup && !r.Game.Rail.Setup) {
		r.startTurnClock(now)
	}
	return nil
}

// Called under s.mu. One action per room/tick keeps play visible and serializes
// against human commands, closure and timeout removal. Persistent seats and
// BotAt resume naturally after a process restart, without browser timers.
func (s *Server) runBots(now time.Time) {
	for id, room := range s.rooms {
		if room.Status != "playing" || room.Game == nil || room.BotAt > now.UnixMilli() {
			continue
		}
		player := room.Game.Turn
		if g := room.Game.Rail; g != nil && g.Setup {
			player = -1
			for i, seat := range room.Seats {
				if seat.Bot && !seat.Left && len(g.SetupPending[i]) > 0 {
					player = i
					break
				}
			}
		}
		if player < 0 || player >= len(room.Seats) || !room.Seats[player].Bot || room.Seats[player].Left {
			continue
		}
		action, err := room.Game.BotAction(player)
		if err != nil {
			log.Printf("bot choose room %s: %v", id, err)
			continue
		}
		raw, _ := json.Marshal(room)
		var next Room
		_ = json.Unmarshal(raw, &next)
		if err = next.applyGameAction(player, action, now); err != nil {
			log.Printf("bot apply room %s: %v", id, err)
			continue
		}
		next.Version++
		next.Updated = now.Unix()
		next.BotAt = now.Add(900 * time.Millisecond).UnixMilli()
		snapshot, _ := json.Marshal(&next)
		record, _ := json.Marshal(map[string]any{"type": "action", "action": action, "bot": true})
		tx, err := s.db.Begin()
		if err == nil {
			_, err = tx.Exec("UPDATE rooms SET snapshot=? WHERE id=?", snapshot, id)
			if err == nil {
				err = archiveGame(tx, &next)
			}
			if err == nil {
				_, err = tx.Exec("INSERT INTO actions VALUES(?,?,?,?,?,?)", id, next.Seats[player].ID, fmt.Sprintf("bot-%d", next.Version), next.Version, record, now.Unix())
			}
			if err == nil {
				err = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
		}
		if err != nil {
			log.Printf("save bot room %s: %v", id, err)
			continue
		}
		s.rooms[id] = &next
		s.broadcast()
	}
}
