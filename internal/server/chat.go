package server

import (
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type ChatMessage struct {
	Spectator bool   `json:"spectator,omitempty"`
	ID        string `json:"id"`
	Sender    User   `json:"sender"`
	Text      string `json:"text"`
	SentAt    int64  `json:"sentAt"`
	Nonce     string `json:"nonce"`
}

// Chat shares room persistence and notifications, but never advances a game
// version or clock: talking must not invalidate an in-progress player action.
func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	room := s.rooms[r.PathValue("id")]
	if room == nil || (seatIndex(room, u.ID) < 0 && spectatorIndex(room, u.ID) < 0) {
		fail(w, http.StatusForbidden, "只能在自己所在的牌桌聊天")
		return
	}
	var req struct {
		Text  string `json:"text"`
		Nonce string `json:"nonce"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Text = strings.TrimSpace(strings.ReplaceAll(req.Text, "\r\n", "\n"))
	if len(req.Nonce) < 8 || len(req.Nonce) > 100 || req.Text == "" || utf8.RuneCountInString(req.Text) > 500 {
		fail(w, http.StatusBadRequest, "消息需 1–500 字，并带有效发送编号")
		return
	}
	for _, c := range req.Text {
		if unicode.IsControl(c) && c != '\n' && c != '\t' {
			fail(w, http.StatusBadRequest, "消息包含无效字符")
			return
		}
	}
	now := time.Now().UnixMilli()
	for _, m := range room.Chat {
		if m.Sender.ID != u.ID {
			continue
		}
		if m.Nonce == req.Nonce {
			if m.Text != req.Text {
				fail(w, http.StatusConflict, "发送编号已使用，请重新发送")
				return
			}
			respond(w, http.StatusOK, m)
			return
		}
	}
	limitKey := "chat:" + u.ID
	limit := s.limits[limitKey]
	if time.Since(limit.At) >= time.Minute {
		limit = bucket{At: time.Now()}
	}
	if limit.Count >= 20 {
		fail(w, http.StatusTooManyRequests, "发送太快了，请稍后再试")
		return
	}
	message := ChatMessage{ID: randomID(12), Sender: u, Text: req.Text, SentAt: now, Nonce: req.Nonce, Spectator: seatIndex(room, u.ID) < 0}
	next := *room
	next.Chat = append(append([]ChatMessage(nil), room.Chat...), message)
	if len(next.Chat) > 100 {
		next.Chat = next.Chat[len(next.Chat)-100:]
	}
	if err := s.save(&next); err != nil {
		fail(w, http.StatusInternalServerError, "消息未保存，请重试")
		return
	}
	s.rooms[room.ID] = &next
	limit.Count++
	s.limits[limitKey] = limit
	for key, value := range s.limits {
		if strings.HasPrefix(key, "chat:") && time.Since(value.At) > 2*time.Minute {
			delete(s.limits, key)
		}
	}
	s.broadcast()
	respond(w, http.StatusCreated, message)
}
