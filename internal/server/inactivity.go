package server

import (
	"encoding/json"
	"log"
	"time"
)

const roomIdleLimit = 24 * time.Hour

// Called under s.mu, or during startup before the server is exposed. Only player
// commands and bot actions renew LastActive; reads, chat, spectators and forced
// timeout actions cannot keep an abandoned room alive forever.
func (s *Server) expireIdleRooms(now time.Time) {
	for id, room := range s.rooms {
		if room.Status != "waiting" && room.Status != "playing" {
			continue
		}
		if room.LastActive <= 0 || room.LastActive > now.Add(-roomIdleLimit).Unix() {
			continue
		}
		raw, _ := json.Marshal(room)
		var next Room
		_ = json.Unmarshal(raw, &next)
		next.Status, next.CloseReason = "closed", "inactive"
		next.TurnDeadline, next.BotAt = 0, 0
		next.Updated = now.Unix()
		next.Version++
		for i := range next.Seats {
			next.Seats[i].AutoPlay = false
		}
		if next.Game != nil {
			next.Game.Log = append(next.Game.Log, "牌桌连续 24 小时无人操作，已自动关闭，本局不计胜负与积分。")
			if len(next.Game.Log) > 80 {
				next.Game.Log = next.Game.Log[len(next.Game.Log)-80:]
			}
		}
		if err := s.save(&next); err != nil {
			log.Printf("close inactive room %s: %v", id, err)
			continue
		}
		s.rooms[id] = &next
		s.broadcast()
	}
}
