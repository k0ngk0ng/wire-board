package server

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// A shared planning deadline expires for every outstanding actor. Team seats
// are never eliminated; absent humans switch to the same persistent autoplay
// control used by the other games and may take control back at any time.
func (s *Server) expireDota(room *Room, now time.Time) {
	raw, _ := json.Marshal(room)
	var next Room
	_ = json.Unmarshal(raw, &next)
	actors := next.Game.DotaActors()
	for _, actor := range actors {
		if !next.Seats[actor].Bot && !next.Seats[actor].AutoPlay {
			next.Seats[actor].AutoPlay = true
			next.Game.Log = append(next.Game.Log, fmt.Sprintf("玩家 %d 超时，已开启电脑托管，可随时取消", actor+1))
		}
		a, err := next.Game.BotAction(actor)
		if err == nil {
			err = next.applyGameAction(actor, a, now)
		}
		if err != nil {
			log.Printf("dota timeout room %s: %v", room.ID, err)
			return
		}
	}
	next.Version++
	next.Updated = now.Unix()
	next.BotAt = now.Add(900 * time.Millisecond).UnixMilli()
	if err := s.save(&next); err != nil {
		log.Printf("save dota timeout room %s: %v", room.ID, err)
		return
	}
	s.rooms[next.ID] = &next
	s.broadcast()
}
