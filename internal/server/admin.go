package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

var gameKinds = []string{"splendor", "rail", "catan", "carcassonne", "sanguosha", "dota"}
var gameNames = map[string]string{"splendor": "璀璨宝石", "rail": "铁路环游", "catan": "卡坦岛", "carcassonne": "卡卡颂", "sanguosha": "三国杀", "dota": "兵线争锋"}

func validName(name string) bool {
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 20 {
		return false
	}
	for _, c := range name {
		if !unicode.IsLetter(c) && !unicode.IsNumber(c) && c != '_' && c != '-' {
			return false
		}
	}
	return true
}

// Administrative metadata is separate from existing users and room snapshots.
// The root role survives removal of bootstrap environment variables and upgrades.
func (s *Server) initAdmin() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS user_controls(user_id TEXT PRIMARY KEY, role TEXT NOT NULL DEFAULT 'player' CHECK(role IN ('player','admin','superadmin')), disabled INTEGER NOT NULL DEFAULT 0, created_at INTEGER NOT NULL DEFAULT 0, last_login INTEGER NOT NULL DEFAULT 0, last_seen INTEGER NOT NULL DEFAULT 0);
CREATE UNIQUE INDEX IF NOT EXISTS one_superadmin ON user_controls(role) WHERE role='superadmin';
CREATE TABLE IF NOT EXISTS game_settings(kind TEXT PRIMARY KEY, enabled INTEGER NOT NULL DEFAULT 1);
CREATE TABLE IF NOT EXISTS admin_audit(id INTEGER PRIMARY KEY AUTOINCREMENT, actor TEXT NOT NULL, actor_name TEXT NOT NULL, action TEXT NOT NULL, target TEXT NOT NULL, detail TEXT NOT NULL, created_at INTEGER NOT NULL);
INSERT OR IGNORE INTO user_controls(user_id) SELECT id FROM users;`)
	if err != nil {
		return err
	}
	rows, err := s.db.Query("SELECT kind,enabled FROM game_settings")
	if err != nil {
		return err
	}
	for rows.Next() {
		var kind string
		var enabled bool
		if err = rows.Scan(&kind, &enabled); err != nil {
			break
		}
		s.hiddenGames[kind] = !enabled
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	if existing := strings.TrimSpace(s.cfg.AdminExistingUsername); existing != "" {
		if !validName(existing) || s.cfg.AdminUsername != "" || s.cfg.AdminPassword != "" {
			return errors.New("ADMIN_EXISTING_USERNAME 需为有效现有昵称，不能与新建管理员配置同时使用")
		}
		var id, name string
		err := s.db.QueryRow("SELECT u.id,u.name FROM users u JOIN user_controls c ON c.user_id=u.id WHERE c.role='superadmin'").Scan(&id, &name)
		if err == nil {
			if name != existing {
				return errors.New("永久管理员已存在，不得更换账号")
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err = s.db.QueryRow("SELECT id FROM users WHERE name=?", existing).Scan(&id); err != nil {
			return errors.New("指定的现有管理员账号不存在")
		}
		return s.adminCommit(User{ID: id, Name: existing}, "root_promoted", id, "服务器私有配置指定现有账号", nil, func(tx *sql.Tx) error {
			_, err := tx.Exec("UPDATE user_controls SET role='superadmin',disabled=0 WHERE user_id=?", id)
			return err
		})
	}
	name, password := strings.TrimSpace(s.cfg.AdminUsername), s.cfg.AdminPassword
	if name == "" && password == "" {
		return nil
	}
	if !validName(name) || len(password) < 16 || len(password) > 72 {
		return errors.New("ADMIN_USERNAME 需 2–20 字，ADMIN_PASSWORD 需 16–72 字节，两项必须同时配置")
	}
	var id, existing, hash string
	err = s.db.QueryRow("SELECT u.id,u.name,u.password FROM users u JOIN user_controls c ON c.user_id=u.id WHERE c.role='superadmin'").Scan(&id, &existing, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && name != existing {
		return errors.New("永久管理员已存在，ADMIN_USERNAME 不得更换；请使用原账号")
	}
	if err == nil && bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil {
		return nil
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	action := "root_password_rotated"
	if id == "" {
		id = randomID(12)
		if _, err = tx.Exec("INSERT INTO users(id,name,password) VALUES(?,?,?)", id, name, string(newHash)); err != nil {
			return errors.New("无法创建永久管理员，昵称可能已被使用；不会自动提升已有普通账号")
		}
		_, err = tx.Exec("INSERT INTO user_controls(user_id,role,created_at) VALUES(?,'superadmin',?)", id, time.Now().Unix())
		action = "root_created"
	} else {
		_, err = tx.Exec("UPDATE users SET password=? WHERE id=?", string(newHash), id)
		if err == nil {
			_, err = tx.Exec("DELETE FROM sessions WHERE user_id=?", id)
		}
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO admin_audit(actor,actor_name,action,target,detail,created_at) VALUES(?,?,?,?,?,?)", id, name, action, id, "服务器私有配置", time.Now().Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Server) role(id string) string {
	var role string
	if s.db.QueryRow("SELECT role FROM user_controls WHERE user_id=?", id).Scan(&role) != nil {
		return "player"
	}
	return role
}
func (s *Server) needAdmin(w http.ResponseWriter, r *http.Request) (User, bool) {
	u, ok := s.needUser(w, r)
	if !ok {
		return u, false
	}
	if role := s.role(u.ID); role != "superadmin" && role != "admin" {
		fail(w, 403, "需要管理员权限")
		return u, false
	}
	return u, true
}
func (s *Server) availableGames() []string {
	out := []string{}
	for _, kind := range gameKinds {
		if !s.hiddenGames[kind] {
			out = append(out, kind)
		}
	}
	return out
}

// Called under mu. Last-seen persistence is throttled; online means authenticated
// HTTP activity or a successful websocket heartbeat within the last 60 seconds.
func (s *Server) seen(id string) {
	now := time.Now()
	s.presence[id] = now
	_, _ = s.db.Exec(`INSERT INTO user_controls(user_id,last_seen) VALUES(?,?) ON CONFLICT(user_id) DO UPDATE SET last_seen=excluded.last_seen WHERE last_seen < ?`, id, now.Unix(), now.Add(-time.Minute).Unix())
}

type adminUser struct {
	User
	Role        string `json:"role"`
	Disabled    bool   `json:"disabled"`
	CreatedAt   int64  `json:"createdAt"`
	LastLogin   int64  `json:"lastLogin"`
	LastSeen    int64  `json:"lastSeen"`
	Online      bool   `json:"online"`
	Connections int    `json:"connections"`
	RoomID      string `json:"roomId,omitempty"`
	Activity    string `json:"activity"`
}

func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.needAdmin(w, r); !ok {
		return
	}
	rows, err := s.db.Query(`SELECT u.id,u.name,COALESCE(c.role,'player'),COALESCE(c.disabled,0),COALESCE(c.created_at,0),COALESCE(c.last_login,0),COALESCE(c.last_seen,0) FROM users u LEFT JOIN user_controls c ON c.user_id=u.id ORDER BY u.name`)
	if err != nil {
		fail(w, 500, "无法读取用户")
		return
	}
	users := []adminUser{}
	online, playing, waiting := 0, 0, 0
	for rows.Next() {
		var u adminUser
		if err = rows.Scan(&u.ID, &u.Name, &u.Role, &u.Disabled, &u.CreatedAt, &u.LastLogin, &u.LastSeen); err != nil {
			break
		}
		if at := s.presence[u.ID]; !at.IsZero() {
			u.LastSeen = max(u.LastSeen, at.Unix())
			u.Online = !u.Disabled && time.Since(at) < time.Minute
		}
		if u.Online {
			online++
			for _, id := range s.connections {
				if id == u.ID {
					u.Connections++
				}
			}
		}
		u.Activity = "离线"
		if u.Online {
			u.Activity = "大厅"
		}
		if room := s.current(u.ID); room != nil && (room.Status == "playing" || room.Status == "waiting") {
			u.RoomID = room.ID
			u.Activity = "等待开局"
			if room.Status == "playing" {
				u.Activity = "对局中"
			}
			if room.Seats[seatIndex(room, u.ID)].AutoPlay {
				u.Activity = "电脑托管"
			}
		} else if room := s.watching(u.ID); room != nil && room.Status == "playing" {
			u.RoomID, u.Activity = room.ID, "观战中"
		}
		users = append(users, u)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(w, 500, "无法读取用户")
		return
	}
	rooms := []map[string]any{}
	for _, room := range s.rooms {
		if room.Status != "waiting" && room.Status != "playing" {
			continue
		}
		if room.Status == "playing" {
			playing++
		} else {
			waiting++
		}
		rooms = append(rooms, summary(room)) // Never include hands, passwords or pending secret choices.
	}
	sort.Slice(rooms, func(i, j int) bool { return rooms[i]["id"].(string) < rooms[j]["id"].(string) })
	games := []map[string]any{}
	for _, kind := range gameKinds {
		games = append(games, map[string]any{"kind": kind, "name": gameNames[kind], "enabled": !s.hiddenGames[kind]})
	}
	respond(w, 200, map[string]any{"users": users, "rooms": rooms, "games": games, "serverNow": time.Now().UnixMilli(), "stats": map[string]int{"users": len(users), "online": online, "playing": playing, "waiting": waiting}})
}

// Metadata, room changes and their audit record commit together. In-memory state
// is published only after persistence succeeds, so failed moderation is atomic.
func (s *Server) adminCommit(actor User, action, target, detail string, rooms []*Room, change func(*sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if change != nil {
		if err = change(tx); err != nil {
			return err
		}
	}
	for _, room := range rooms {
		raw, err := json.Marshal(room)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE rooms SET snapshot=? WHERE id=?", raw, room.ID); err != nil {
			return err
		}
		if err = archiveGame(tx, room); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("INSERT INTO admin_audit(actor,actor_name,action,target,detail,created_at) VALUES(?,?,?,?,?,?)", actor.ID, actor.Name, action, target, detail, time.Now().Unix()); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for _, room := range rooms {
		s.rooms[room.ID] = room
	}
	s.broadcast()
	return nil
}
func cloneAdminRoom(room *Room) *Room {
	raw, _ := json.Marshal(room)
	var next Room
	_ = json.Unmarshal(raw, &next)
	next.Version++
	next.Updated = time.Now().Unix()
	return &next
}
func closeAdminRoom(room *Room, reason string) *Room {
	next := cloneAdminRoom(room)
	next.Status, next.CloseReason = "closed", reason
	next.TurnDeadline, next.BotAt = 0, 0
	for i := range next.Seats {
		next.Seats[i].AutoPlay = false
	}
	return next
}

func (s *Server) adminGame(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needAdmin(w, r)
	if !ok {
		return
	}
	kind := r.PathValue("kind")
	if gameNames[kind] == "" {
		fail(w, 404, "桌游不存在")
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Enabled == nil {
		fail(w, 400, "缺少上架状态")
		return
	}
	changed := []*Room{}
	if !*req.Enabled {
		for _, room := range s.rooms {
			if room.Kind == kind && room.Status == "waiting" {
				changed = append(changed, closeAdminRoom(room, "unpublished"))
			}
		}
	}
	name := "game_unpublished"
	if *req.Enabled {
		name = "game_published"
	}
	err := s.adminCommit(u, name, kind, gameNames[kind], changed, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO game_settings(kind,enabled) VALUES(?,?) ON CONFLICT(kind) DO UPDATE SET enabled=excluded.enabled", kind, *req.Enabled)
		return err
	})
	if err != nil {
		fail(w, 500, "上架状态未保存，请重试")
		return
	}
	s.hiddenGames[kind] = !*req.Enabled
	respond(w, 200, map[string]bool{"ok": true})
}

func (s *Server) adminManageUser(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	actor, ok := s.needAdmin(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	var target User
	if s.db.QueryRow("SELECT id,name FROM users WHERE id=?", id).Scan(&target.ID, &target.Name) != nil {
		fail(w, 404, "用户不存在")
		return
	}
	role := s.role(id)
	if role == "superadmin" || actor.ID == id {
		fail(w, 403, "永久管理员和当前账号不能在此修改；永久管理员密码由服务器私有配置维护")
		return
	}
	var req struct {
		Action   string `json:"action"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !decode(w, r, &req) {
		return
	}
	if (role == "admin" || req.Action == "role") && s.role(actor.ID) != "superadmin" {
		fail(w, 403, "只有永久管理员可以管理其他管理员或分配权限")
		return
	}
	var hash []byte
	var err error
	switch req.Action {
	case "disable", "enable", "logout":
	case "password":
		if len(req.Password) < 12 || len(req.Password) > 72 {
			fail(w, 400, "新密码需 12–72 字节")
			return
		}
		hash, err = bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			fail(w, 500, "无法重置密码")
			return
		}
	case "role":
		if req.Role != "player" && req.Role != "admin" {
			fail(w, 400, "角色只能是普通用户或管理员")
			return
		}
	default:
		fail(w, 400, "未知用户管理操作")
		return
	}
	revoke := req.Action == "disable" || req.Action == "logout" || req.Action == "password"
	changed := []*Room{}
	if revoke {
		for _, room := range s.rooms {
			idx, watch := seatIndex(room, id), spectatorIndex(room, id)
			if idx < 0 && watch < 0 {
				continue
			}
			next := cloneAdminRoom(room)
			if watch >= 0 {
				next.Spectators = append(next.Spectators[:watch], next.Spectators[watch+1:]...)
			}
			if idx >= 0 && next.Status == "playing" {
				next.Seats[idx].AutoPlay = true
			}
			if idx >= 0 && next.Status == "waiting" && req.Action == "disable" {
				next.Seats = append(next.Seats[:idx], next.Seats[idx+1:]...)
				humans := 0
				for i := range next.Seats {
					if !next.Seats[i].Bot {
						humans++
						if next.Host == id {
							next.Host = next.Seats[i].ID
						}
					}
					next.Seats[i].Ready = next.Seats[i].Bot
				}
				if humans == 0 {
					next.Status, next.CloseReason = "closed", "admin"
				}
			}
			changed = append(changed, next)
		}
	}
	detail := target.Name
	if req.Action == "role" {
		detail += " → " + req.Role
	}
	err = s.adminCommit(actor, "user_"+req.Action, id, detail, changed, func(tx *sql.Tx) error {
		if _, err := tx.Exec("INSERT OR IGNORE INTO user_controls(user_id) VALUES(?)", id); err != nil {
			return err
		}
		var err error
		switch req.Action {
		case "disable", "enable":
			_, err = tx.Exec("UPDATE user_controls SET disabled=? WHERE user_id=?", req.Action == "disable", id)
		case "role":
			_, err = tx.Exec("UPDATE user_controls SET role=? WHERE user_id=?", req.Role, id)
		case "password":
			_, err = tx.Exec("UPDATE users SET password=? WHERE id=?", string(hash), id)
		}
		if err == nil && revoke {
			_, err = tx.Exec("DELETE FROM sessions WHERE user_id=?", id)
		}
		return err
	})
	if err != nil {
		fail(w, 500, "用户操作未保存，请重试")
		return
	}
	if revoke {
		delete(s.presence, id)
	}
	respond(w, 200, map[string]bool{"ok": true})
}

func (s *Server) adminCloseRoom(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needAdmin(w, r)
	if !ok {
		return
	}
	room := s.rooms[r.PathValue("id")]
	if room == nil {
		fail(w, 404, "牌桌不存在")
		return
	}
	if room.Status != "playing" && room.Status != "waiting" {
		fail(w, 409, "牌桌已经结束")
		return
	}
	if err := s.adminCommit(u, "room_closed", room.ID, room.Name, []*Room{closeAdminRoom(room, "admin")}, nil); err != nil {
		fail(w, 500, "无法结束牌桌")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}

func (s *Server) adminAudit(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.needAdmin(w, r); !ok {
		return
	}
	var before int64
	if v := r.URL.Query().Get("before"); v != "" {
		if _, err := fmt.Sscan(v, &before); err != nil || before < 1 {
			fail(w, 400, "无效日志位置")
			return
		}
	}
	rows, err := s.db.Query("SELECT id,actor_name,action,target,detail,created_at FROM admin_audit WHERE (?=0 OR id<?) ORDER BY id DESC LIMIT 51", before, before)
	if err != nil {
		fail(w, 500, "无法读取操作记录")
		return
	}
	defer rows.Close()
	entries := []map[string]any{}
	for rows.Next() {
		var id, at int64
		var actor, action, target, detail string
		if err = rows.Scan(&id, &actor, &action, &target, &detail, &at); err != nil {
			break
		}
		entries = append(entries, map[string]any{"id": id, "actor": actor, "action": action, "target": target, "detail": detail, "createdAt": at})
	}
	if err != nil || rows.Err() != nil {
		fail(w, 500, "无法读取操作记录")
		return
	}
	more := len(entries) > 50
	if more {
		entries = entries[:50]
	}
	respond(w, 200, map[string]any{"entries": entries, "hasMore": more})
}
