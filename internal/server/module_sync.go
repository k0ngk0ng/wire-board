package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"net/http"
	"time"
)

// Checkpoint transfer is explicitly requested for initialization/recovery, not
// part of ordinary stream snapshots. Copy under the platform lock, then let
// the module compute/encode outside it so a slow reader cannot stall games.
func (s *Server) moduleSyncSnapshot(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	user, err := s.user(r)
	if err != nil {
		s.mu.Unlock()
		fail(w, 401, "请先登录")
		return
	}
	room := s.current(user.ID)
	if room == nil {
		room = s.watching(user.ID)
	}
	if room == nil || room.ID != r.PathValue("id") {
		s.mu.Unlock()
		fail(w, 403, "无权读取此房间的同步状态")
		return
	}
	module := s.modules[room.Kind]
	if room.Module == nil || module.SyncSnapshot == nil {
		s.mu.Unlock()
		fail(w, 404, "该游戏没有同步恢复入口")
		return
	}
	seat := seatIndex(room, user.ID)
	id, version, instance := room.ID, room.Version, s.instance
	raw, err := json.Marshal(room.Module)
	s.mu.Unlock()
	if err != nil {
		fail(w, 500, "无法读取同步状态")
		return
	}
	var match gameapi.Match
	if err = json.Unmarshal(raw, &match); err != nil {
		fail(w, 500, "无法读取同步状态")
		return
	}
	value, err := module.SyncSnapshot(match, seat)
	if err != nil {
		fail(w, 500, "无法读取同步状态")
		return
	}
	respond(w, 200, map[string]any{"roomId": id, "version": version, "serverInstance": instance, "serverNow": time.Now().UnixMilli(), "data": value})
}
