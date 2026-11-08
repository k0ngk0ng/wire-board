package server

// Failure inventory before implementation: room activity has only an opaque ID;
// last leave deletes its context; reused user/current-room metadata labels an
// earlier game incorrectly; archive search/pagination is unavailable; legacy
// records without roomId disappear; ordinary/revoked users gain admin history;
// room passwords, game hands or chat leak through archived snapshots.
import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
)

func TestAdminActivityRoomContextAndArchiveSearchHTTP(t *testing.T) {
	s, ts, root := adminServer(t)
	owner, other := newClient(t, ts.URL), newClient(t, ts.URL)
	owner.register("历史牌桌玩家")
	other.register("非管理员历史访客")
	uid := owner.state()["user"].(map[string]any)["id"].(string)
	makeMatch := func(kind, name string) (string, string) {
		owner.post("/api/rooms", map[string]any{"kind": kind, "name": name, "capacity": 2, "password": "private-room-password-only"}, 201)
		room := current(owner)
		rid := room["id"].(string)
		live := managed(t, root, uid)
		if live["roomKind"] != kind || live["roomName"] != name || live["roomGameName"] == "" {
			t.Fatal("live activity omitted its game", live)
		}
		owner.command(room, "add_bot", nil, 200)
		owner.command(current(owner), "ready", nil, 200)
		owner.command(current(owner), "start", nil, 200)
		mid := current(owner)["matchId"].(string)
		owner.command(current(owner), "close", nil, 200)
		owner.command(current(owner), "leave", nil, 200)
		return rid, mid
	}
	rid, mid := makeMatch("splendor", "查询%_特殊牌桌")
	_, activity := root.request("GET", "/api/admin/users/"+uid+"/activity", nil)
	for _, raw := range activity["entries"].([]any) {
		entry := raw.(map[string]any)
		if entry["action"] == "register" {
			continue
		}
		room, ok := entry["room"].(map[string]any)
		if !ok || room["id"] != rid || room["kind"] != "splendor" || room["name"] != "查询%_特殊牌桌" || room["gameName"] != "璀璨宝石" {
			t.Fatal("activity lost actual game/room context after deletion", entry)
		}
	}
	makeMatch("carcassonne", "另一种游戏")
	path := "/api/admin/rooms/history"
	code, page := root.request("GET", path+"?roomId="+strings.ToUpper(rid), nil)
	if code != 200 || len(page["entries"].([]any)) != 1 {
		t.Fatal("closed table cannot be queried by room ID", code, page)
	}
	record := page["entries"].([]any)[0].(map[string]any)
	if record["id"] != mid || record["roomId"] != rid || record["kind"] != "splendor" || record["gameName"] != "璀璨宝石" || record["status"] != "closed" {
		t.Fatal("wrong archived identity", record)
	}
	for query, want := range map[string]int{"q=" + url.QueryEscape("%_"): 1, "kind=carcassonne": 1, "userId=" + uid: 2, "q=" + url.QueryEscape("历史牌桌玩家"): 2, "q=" + url.QueryEscape("璀璨宝石"): 1} {
		_, p := root.request("GET", path+"?"+query, nil)
		if len(p["entries"].([]any)) != want {
			t.Fatal("archive filter", query, p)
		}
	}
	// Old immutable archives remain searchable even when they predate roomId.
	if _, e := s.db.Exec("UPDATE match_history SET snapshot=json_remove(snapshot,'$.roomId') WHERE id=?", mid); e != nil {
		t.Fatal(e)
	}
	_, legacy := root.request("GET", path+"?q="+url.QueryEscape("查询%_特殊牌桌"), nil)
	if len(legacy["entries"].([]any)) != 1 {
		t.Fatal("legacy archive disappeared", legacy)
	}
	_, linked := root.request("GET", path+"?roomId="+rid, nil)
	if len(linked["entries"].([]any)) != 1 {
		t.Fatal("explicit activity match link did not resolve legacy archive", linked)
	}
	raw, _ := json.Marshal(linked)
	for _, secret := range []string{"private-room-password-only", "reserved", "tokens", "password", "chat", "deck"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("archive disclosed private content", secret)
		}
	}
	for i := 0; i < 51; i++ {
		makeMatch("splendor", fmt.Sprintf("翻页验收 %02d", i))
	}
	_, first := root.request("GET", path, nil)
	if len(first["entries"].([]any)) != 50 || first["hasMore"] != true || first["nextBefore"] == "" {
		t.Fatal("archive first page", first)
	}
	_, second := root.request("GET", path+"?before="+url.QueryEscape(first["nextBefore"].(string)), nil)
	ids := map[string]bool{}
	for _, p := range []map[string]any{first, second} {
		for _, e := range p["entries"].([]any) {
			id := e.(map[string]any)["id"].(string)
			if ids[id] {
				t.Fatal("duplicate archive row", id)
			}
			ids[id] = true
		}
	}
	if len(ids) != 53 || second["hasMore"] != false {
		t.Fatal("missing old archives", len(ids), second)
	}
	if code, _ := other.request("GET", path, nil); code != 403 {
		t.Fatal("non-admin archive access", code)
	}
	root.post("/api/admin/users/"+other.state()["user"].(map[string]any)["id"].(string), map[string]any{"action": "role", "role": "admin"}, 200)
	if code, _ := other.request("GET", path, nil); code != 200 {
		t.Fatal("delegated admin cannot inspect history", code)
	}
	root.post("/api/admin/users/"+other.state()["user"].(map[string]any)["id"].(string), map[string]any{"action": "role", "role": "player"}, 200)
	if code, _ := other.request("GET", path, nil); code != 403 {
		t.Fatal("revoked admin retained history access", code)
	}
	if code, _ := newClient(t, ts.URL).request("GET", path, nil); code != 401 {
		t.Fatal("anonymous archive access", code)
	}
	for _, bad := range []string{"?before=invalid", "?q=" + strings.Repeat("x", 301)} {
		if code, _ := root.request("GET", path+bad, nil); code != 400 {
			t.Fatal("invalid archive query", bad, code)
		}
	}
	cfg := s.cfg
	ts.Close()
	s.Close()
	restored, e := New(cfg, fstest.MapFS{"index.html": {Data: []byte("Wire Board")}})
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	after := httptest.NewServer(restored.Handler())
	defer after.Close()
	root = newClient(t, after.URL)
	root.post("/api/login", map[string]any{"name": cfg.AdminUsername, "password": cfg.AdminPassword}, 200)
	_, again := root.request("GET", path+"?roomId="+rid, nil)
	if len(again["entries"].([]any)) != 1 {
		t.Fatal("archive link lost on restart", again)
	}
	_, activity = root.request("GET", "/api/admin/users/"+uid+"/activity", nil)
	if len(activity["entries"].([]any)) != 50 {
		t.Fatal("activity pagination lost after schema migration")
	}
}
