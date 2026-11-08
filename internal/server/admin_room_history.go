package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"
)

type activityRoom struct {
	ID       string `json:"id"`
	Kind     string `json:"kind,omitempty"`
	Name     string `json:"name,omitempty"`
	GameName string `json:"gameName,omitempty"`
	MatchID  string `json:"matchId,omitempty"`
}

func isRoomActivity(action string) bool {
	switch action {
	case "create_room", "join", "leave", "ready", "start", "close", "restart", "watch", "add_bot", "remove_bot", "autoplay":
		return true
	}
	return false
}
func (s *Server) activityRoomContext(id string) *activityRoom {
	if room := s.rooms[id]; room != nil {
		return &activityRoom{ID: room.ID, Kind: room.Kind, Name: room.Name, MatchID: room.MatchID}
	}
	return nil
}

func (s *Server) initActivityRoomContext() error {
	rows, err := s.db.Query("PRAGMA table_info(user_activity)")
	if err != nil {
		return err
	}
	found := map[string]bool{}
	for rows.Next() {
		var cid, nn, pk int
		var name, kind string
		var def any
		if err = rows.Scan(&cid, &name, &kind, &nn, &def, &pk); err != nil {
			break
		}
		found[name] = true
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, name := range []string{"room_id", "room_kind", "room_name", "match_id"} {
		if !found[name] {
			if _, err = s.db.Exec("ALTER TABLE user_activity ADD COLUMN " + name + " TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
		}
	}
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS user_activity_room ON user_activity(room_id,id DESC);
 CREATE INDEX IF NOT EXISTS match_history_page ON match_history(ended DESC,id DESC);
 UPDATE user_activity SET room_id=target WHERE room_id='' AND target<>'' AND action IN ('create_room','join','leave','ready','start','close','restart','watch','add_bot','remove_bot','autoplay');
 UPDATE user_activity SET
 room_kind=COALESCE((SELECT json_extract(CASE WHEN json_valid(snapshot) THEN snapshot ELSE '{}' END,'$.kind') FROM rooms WHERE id=user_activity.room_id),''),
 room_name=COALESCE((SELECT json_extract(CASE WHEN json_valid(snapshot) THEN snapshot ELSE '{}' END,'$.name') FROM rooms WHERE id=user_activity.room_id),'')
 WHERE room_id<>'' AND room_kind='' AND EXISTS(SELECT 1 FROM rooms WHERE id=user_activity.room_id);`)
	return err
}

type historyCursor struct {
	Ended int64  `json:"ended"`
	ID    string `json:"id"`
}
type adminHistoryEntry struct {
	ID       string        `json:"id"`
	RoomID   string        `json:"roomId,omitempty"`
	Room     string        `json:"room"`
	Kind     string        `json:"kind"`
	GameName string        `json:"gameName"`
	Status   string        `json:"status"`
	Ended    int64         `json:"ended"`
	Players  []MatchPlayer `json:"players"`
	Rated    bool          `json:"rated"`
	Draw     bool          `json:"draw,omitempty"`
}

func (s *Server) adminRoomHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	_, ok := s.needAdmin(w, r)
	names := map[string]string{}
	for k, v := range s.gameNames {
		names[k] = v
	}
	s.mu.Unlock()
	if !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	room := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("roomId")))
	user := strings.TrimSpace(r.URL.Query().Get("userId"))
	status := r.URL.Query().Get("status")
	if utf8.RuneCountInString(q) > 200 || len(kind) > 64 || len(room) > 128 || len(user) > 128 || (status != "" && status != "closed" && status != "finished") {
		fail(w, 400, "无效历史查询")
		return
	}
	where := []string{"json_valid(h.snapshot)"}
	args := []any{}
	if kind != "" {
		where = append(where, "json_extract(h.snapshot,'$.kind')=?")
		args = append(args, kind)
	}
	if room != "" {
		where = append(where, "(lower(json_extract(h.snapshot,'$.roomId'))=? OR h.id IN(SELECT match_id FROM user_activity WHERE room_id=? AND match_id<>''))")
		args = append(args, room, room)
	}
	if user != "" {
		where = append(where, "EXISTS(SELECT 1 FROM match_members m WHERE m.match_id=h.id AND m.user_id=?)")
		args = append(args, user)
	}
	if status != "" {
		where = append(where, "json_extract(h.snapshot,'$.status')=?")
		args = append(args, status)
	}
	if q != "" {
		pattern := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(q) + "%"
		part := `(h.id LIKE ? ESCAPE '\' OR json_extract(h.snapshot,'$.roomId') LIKE ? ESCAPE '\' OR json_extract(h.snapshot,'$.room') LIKE ? ESCAPE '\' OR EXISTS(SELECT 1 FROM json_each(h.snapshot,'$.players') p WHERE json_extract(p.value,'$.name') LIKE ? ESCAPE '\')`
		args = append(args, pattern, pattern, pattern, pattern)
		matching := []string{}
		for k, name := range names {
			if strings.Contains(strings.ToLower(name), strings.ToLower(q)) {
				matching = append(matching, k)
			}
		}
		sort.Strings(matching)
		for _, k := range matching {
			part += " OR json_extract(h.snapshot,'$.kind')=?"
			args = append(args, k)
		}
		where = append(where, part+")")
	}
	if raw := r.URL.Query().Get("before"); raw != "" {
		var c historyCursor
		bytes, err := base64.RawURLEncoding.DecodeString(raw)
		if len(raw) > 512 || err != nil || json.Unmarshal(bytes, &c) != nil || c.Ended < 0 || c.ID == "" || len(c.ID) > 128 {
			fail(w, 400, "无效历史位置")
			return
		}
		where = append(where, "(h.ended<? OR(h.ended=? AND h.id<?))")
		args = append(args, c.Ended, c.Ended, c.ID)
	}
	rows, err := s.db.Query("SELECT h.id,h.ended,h.snapshot FROM match_history h WHERE "+strings.Join(where, " AND ")+" ORDER BY h.ended DESC,h.id DESC LIMIT 51", args...)
	if err != nil {
		fail(w, 500, "无法读取历史牌桌")
		return
	}
	type archive struct {
		id    string
		ended int64
		raw   []byte
	}
	records := []archive{}
	for rows.Next() {
		var a archive
		if err = rows.Scan(&a.id, &a.ended, &a.raw); err != nil {
			break
		}
		records = append(records, a)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(w, 500, "无法读取历史牌桌")
		return
	}
	more := len(records) > 50
	if more {
		records = records[:50]
	}
	entries := []adminHistoryEntry{}
	for _, a := range records {
		var record MatchRecord
		if json.Unmarshal(a.raw, &record) != nil {
			continue
		}
		name := names[record.Kind]
		if name == "" {
			name = record.Kind
		}
		entries = append(entries, adminHistoryEntry{ID: a.id, RoomID: record.RoomID, Room: record.Room, Kind: record.Kind, GameName: name, Status: record.Status, Ended: a.ended, Players: record.Players, Rated: record.Rated, Draw: record.Draw})
	}
	next := ""
	if more && len(records) > 0 {
		last := records[len(records)-1]
		raw, _ := json.Marshal(historyCursor{Ended: last.ended, ID: last.id})
		next = base64.RawURLEncoding.EncodeToString(raw)
	}
	respond(w, 200, map[string]any{"entries": entries, "hasMore": more, "nextBefore": next})
}
