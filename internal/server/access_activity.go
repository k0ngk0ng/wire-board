package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Only explicitly configured proxies may supply forwarding headers. Walk from
// the trusted peer toward the client, stopping at the first untrusted address.
func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	trusted := func(ip string) bool {
		for _, value := range strings.Split(s.cfg.TrustedProxies, ",") {
			value = strings.TrimSpace(value)
			if net.ParseIP(value) != nil && net.ParseIP(value).Equal(net.ParseIP(ip)) {
				return true
			}
			if _, network, e := net.ParseCIDR(value); e == nil && network.Contains(net.ParseIP(ip)) {
				return true
			}
		}
		return false
	}
	if !trusted(host) {
		return host
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		next := strings.TrimSpace(chain[i])
		if net.ParseIP(next) == nil {
			return host
		}
		host = next
		if !trusted(host) {
			break
		}
	}
	return host
}

func (s *Server) initAccessActivity() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS user_access(user_id TEXT PRIMARY KEY, registered_ip TEXT NOT NULL DEFAULT '', last_login_ip TEXT NOT NULL DEFAULT '');
 CREATE TABLE IF NOT EXISTS user_activity(id INTEGER PRIMARY KEY AUTOINCREMENT,user_id TEXT NOT NULL,action TEXT NOT NULL,target TEXT NOT NULL,ip TEXT NOT NULL,created_at INTEGER NOT NULL);
 CREATE INDEX IF NOT EXISTS user_activity_page ON user_activity(user_id,id DESC);`)
	if err != nil {
		return err
	}
	if err = s.initActivityRoomContext(); err != nil {
		return err
	}
	rows, err := s.db.Query("PRAGMA table_info(admin_audit)")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, nn, pk int
		var name, kind string
		var def any
		if err = rows.Scan(&cid, &name, &kind, &nn, &def, &pk); err != nil {
			break
		}
		found = found || name == "ip"
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	if !found {
		_, err = s.db.Exec("ALTER TABLE admin_audit ADD COLUMN ip TEXT NOT NULL DEFAULT ''")
	}
	return err
}
func activityInsert(tx *sql.Tx, id, action, target, ip string, contexts ...*activityRoom) error {
	meta := activityRoom{}
	if len(contexts) > 0 && contexts[0] != nil {
		meta = *contexts[0]
	}
	_, err := tx.Exec("INSERT INTO user_activity(user_id,action,target,ip,created_at,room_id,room_kind,room_name,match_id) VALUES(?,?,?,?,?,?,?,?,?)", id, action, target, ip, time.Now().Unix(), meta.ID, meta.Kind, meta.Name, meta.MatchID)
	return err
}

func (s *Server) adminUserActivity(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.needAdmin(w, r); !ok {
		return
	}
	id := r.PathValue("id")
	var exists int
	if s.db.QueryRow("SELECT 1 FROM users WHERE id=?", id).Scan(&exists) != nil {
		fail(w, 404, "用户不存在")
		return
	}
	before := int64(0)
	if raw := r.URL.Query().Get("before"); raw != "" {
		var e error
		before, e = strconv.ParseInt(raw, 10, 64)
		if e != nil || before < 1 {
			fail(w, 400, "无效动态位置")
			return
		}
	}
	rows, err := s.db.Query("SELECT id,action,target,ip,created_at,room_id,room_kind,room_name,match_id FROM user_activity WHERE user_id=? AND (?=0 OR id<?) ORDER BY id DESC LIMIT 51", id, before, before)
	if err != nil {
		fail(w, 500, "无法读取玩家动态")
		return
	}
	defer rows.Close()
	entries := []map[string]any{}
	for rows.Next() {
		var n, at int64
		var action, target, ip string
		meta := activityRoom{}
		if err = rows.Scan(&n, &action, &target, &ip, &at, &meta.ID, &meta.Kind, &meta.Name, &meta.MatchID); err != nil {
			break
		}
		entry := map[string]any{"id": n, "action": action, "target": target, "ip": ip, "createdAt": at}
		if meta.ID == "" && isRoomActivity(action) {
			meta.ID = target
		}
		if meta.ID != "" {
			entry["room"] = &meta
		}
		entries = append(entries, entry)
	}
	if err != nil || rows.Err() != nil {
		fail(w, 500, "无法读取玩家动态")
		return
	}
	rows.Close()
	cache := map[string]activityRoom{}
	for _, entry := range entries {
		meta, ok := entry["room"].(*activityRoom)
		if !ok {
			continue
		}
		if meta.Kind == "" {
			if known, exists := cache[meta.ID]; exists {
				meta.Kind, meta.Name = known.Kind, known.Name
			} else {
				if current := s.activityRoomContext(meta.ID); current != nil {
					meta.Kind, meta.Name = current.Kind, current.Name
				} else {
					_ = s.db.QueryRow("SELECT room_kind,room_name FROM user_activity WHERE room_id=? AND room_kind<>'' ORDER BY id DESC LIMIT 1", meta.ID).Scan(&meta.Kind, &meta.Name)
				}
				cache[meta.ID] = *meta
			}
		}
		meta.GameName = s.gameNames[meta.Kind]
		if meta.GameName == "" {
			meta.GameName = meta.Kind
		}
	}
	more := len(entries) > 50
	if more {
		entries = entries[:50]
	}
	respond(w, 200, map[string]any{"entries": entries, "hasMore": more})
}

// Record successful, meaningful player actions, never chat text, passwords,
// private cards, controller input, or high-frequency gameplay frames.
func (s *Server) recordActivity(id, action, target, ip string, contexts ...*activityRoom) {
	tx, err := s.db.Begin()
	if err == nil {
		defer tx.Rollback()
		if err = activityInsert(tx, id, action, target, ip, contexts...); err == nil {
			err = tx.Commit()
		}
	}
	if err != nil {
		fmt.Println("player activity persistence failed")
	}
}

type activityResponse struct {
	http.ResponseWriter
	status int
}

func (w *activityResponse) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (w *activityResponse) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(data)
}
func (s *Server) withActivity(action string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestAction := action
		s.mu.Lock()
		u, err := s.user(r)
		s.mu.Unlock()
		// Decode only the small command envelope; restore the body for the handler.
		target := r.PathValue("id")
		if requestAction == "room" || requestAction == "friend" {
			raw, e := io.ReadAll(r.Body)
			if e != nil {
				fail(w, 400, "请求内容过大或不可读")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(raw))
			var req struct {
				Type   string `json:"type"`
				Action string `json:"action"`
			}
			_ = json.Unmarshal(raw, &req)
			if requestAction == "room" {
				switch req.Type {
				case "join", "leave", "ready", "start", "close", "add_bot", "remove_bot", "autoplay", "restart":
					requestAction = req.Type
				default:
					requestAction = ""
				}
			} else {
				requestAction = "friend_" + req.Action
			}
		}
		var context *activityRoom
		if isRoomActivity(requestAction) && target != "" {
			s.mu.Lock()
			context = s.activityRoomContext(target)
			s.mu.Unlock()
		}
		recorder := &activityResponse{ResponseWriter: w}
		next(recorder, r)
		if err == nil && requestAction != "" && recorder.status >= 200 && recorder.status < 300 {
			s.mu.Lock()
			defer s.mu.Unlock()
			if requestAction == "create_room" {
				if room := s.current(u.ID); room != nil {
					target = room.ID
					context = s.activityRoomContext(target)
				}
			}
			if isRoomActivity(requestAction) {
				if current := s.activityRoomContext(target); current != nil {
					context = current
				}
			}
			s.recordActivity(u.ID, requestAction, target, s.clientIP(r), context)
		}
	}
}
