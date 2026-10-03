package server

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func (seat Seat) computerControlled() bool {
	return (seat.Bot || seat.AutoPlay) && !seat.Left
}

func (r *Room) applyGameAction(player int, action game.Action, now time.Time) error {
	turn, round := r.Game.Turn, r.Game.Round
	phase := r.Game.Phase
	sgSequence := 0
	sgPending := false
	if g := r.Game.Sanguosha; g != nil {
		sgSequence = g.Sequence
		sgPending = g.Pending != nil
		if !sgPending {
			r.SGTimeLeft = max(0, r.TurnDeadline-now.UnixMilli())
		}
	}
	setupStep := -1
	tradeID := -1
	if r.Game.Catan != nil {
		setupStep = r.Game.Catan.SetupStep
		tradeID = r.Game.Catan.TradeID
	}
	setup := r.Game.Rail != nil && r.Game.Rail.Setup
	if err := r.Game.Apply(player, action); err != nil {
		return err
	}
	if r.Game.Catan != nil {
		if phase != "catan_discard" && r.Game.Phase == "catan_discard" {
			r.CatanPendingVersion = r.Version + 1
		}
		if r.Game.Catan.Trade != nil && r.Game.Catan.TradeID != tradeID {
			r.CatanTradeVersion = r.Version + 1
		}
	}
	if r.Game.Finished {
		r.Status = "finished"
	}
	if g := r.Game.Sanguosha; g != nil {
		if r.Game.Finished {
			r.TurnDeadline = 0
			return nil
		}
		newTurn := r.Game.Turn != turn || r.Game.Round != round || phase == "sg_select" && !g.Selecting
		if newTurn {
			r.SGTimeLeft = turnLimit.Milliseconds()
		}
		if g.Pending != nil && g.Sequence != sgSequence {
			limit := 20 * time.Second
			if g.Pending.Kind == "general" {
				limit = turnLimit
			}
			r.TurnDeadline = now.Add(limit).UnixMilli()
		} else if g.Pending == nil && (sgPending || newTurn) {
			r.TurnDeadline = now.UnixMilli() + r.SGTimeLeft
		}
		return nil
	}
	catanClock := r.Game.Catan != nil && (r.Game.Catan.SetupStep != setupStep || (phase != r.Game.Phase && (phase == "catan_discard" || r.Game.Phase == "catan_discard")))
	if catanClock || r.Game.Turn != turn || r.Game.Round != round || r.Game.Finished || (setup && !r.Game.Rail.Setup) {
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
		if room.Game.Sanguosha != nil {
			player = -1
			for _, actor := range room.Game.SanguoshaActors() {
				if room.Seats[actor].computerControlled() {
					player = actor
					break
				}
			}
		}
		if g := room.Game.Rail; g != nil && g.Setup {
			player = -1
			for i, seat := range room.Seats {
				if seat.computerControlled() && len(g.SetupPending[i]) > 0 {
					player = i
					break
				}
			}
		}
		if g := room.Game.Catan; g != nil {
			if room.Game.Phase == "catan_discard" {
				player = -1
				for i, seat := range room.Seats {
					if seat.computerControlled() && g.DiscardDue[i] > 0 {
						player = i
						break
					}
				}
			} else if g.Trade != nil {
				for i, seat := range room.Seats {
					if i != room.Game.Turn && seat.computerControlled() && g.Trade.Responses[i] == 0 {
						player = i
						break
					}
				}
			}
		}
		if player < 0 || player >= len(room.Seats) || !room.Seats[player].computerControlled() {
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
		record, _ := json.Marshal(map[string]any{"type": "action", "action": action, "bot": next.Seats[player].Bot, "autoPlay": next.Seats[player].AutoPlay})
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
