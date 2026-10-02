package server

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func spectatorIndex(room *Room, userID string) int {
	for i, user := range room.Spectators {
		if user.ID == userID {
			return i
		}
	}
	return -1
}

func (s *Server) watching(userID string) *Room {
	for _, room := range s.rooms {
		if spectatorIndex(room, userID) >= 0 {
			return room
		}
	}
	return nil
}

// Viewing membership is persisted independently of game versions and clocks.
func (s *Server) watch(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needUser(w, r)
	if !ok {
		return
	}
	room := s.rooms[r.PathValue("id")]
	if room == nil {
		fail(w, 404, "牌桌不存在")
		return
	}
	var req struct {
		Leave    bool   `json:"leave"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	index := spectatorIndex(room, u.ID)
	next := *room
	next.Spectators = append([]User(nil), room.Spectators...)
	if req.Leave {
		if index < 0 {
			respond(w, 200, map[string]bool{"ok": true})
			return
		}
		next.Spectators = append(next.Spectators[:index], next.Spectators[index+1:]...)
	} else {
		if index >= 0 {
			respond(w, 200, map[string]bool{"ok": true})
			return
		}
		if s.current(u.ID) != nil || s.watching(u.ID) != nil {
			fail(w, 409, "请先离开当前牌桌或结束观战")
			return
		}
		if room.Game == nil || (room.Status != "playing" && room.Status != "finished") {
			fail(w, 400, "牌桌尚未开局或已关闭")
			return
		}
		if room.Password != "" && bcrypt.CompareHashAndPassword([]byte(room.Password), []byte(req.Password)) != nil {
			fail(w, 403, "房间密码不正确")
			return
		}
		next.Spectators = append(next.Spectators, u)
	}
	if err := s.save(&next); err != nil {
		fail(w, 500, "无法保存观战状态，请重试")
		return
	}
	s.rooms[room.ID] = &next
	s.broadcast()
	respond(w, 200, map[string]bool{"ok": true})
}
