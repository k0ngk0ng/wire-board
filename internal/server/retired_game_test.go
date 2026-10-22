package server

// Retirement failures: hidden-only game can still be created; admin can
// resurrect it; saved active rooms crash restart or keep ticking; removal
// deletes historical snapshots, members or rating entries.
import (
	"bytes"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"slices"
	"testing"
	"testing/fstest"
)

func TestRetiredDotaUnavailableHTTP(t *testing.T) {
	s, _, root := adminServer(t)
	if slices.Contains(s.availableGames(), "dota") {
		t.Fatal("retired game still listed")
	}
	root.post("/api/rooms", map[string]any{"kind": "dota", "name": "Retired", "capacity": 2}, 400)
	root.post("/api/admin/games/dota", map[string]bool{"enabled": true}, 404)
	if _, e := game.New("dota", 2); e == nil {
		t.Fatal("retired engine still callable")
	}
}
func TestRetiredDotaRestorePreservesStoredHistory(t *testing.T) {
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite"}
	files := fstest.MapFS{"index.html": {Data: []byte("test")}}
	s, e := New(cfg, files)
	if e != nil {
		t.Fatal(e)
	}
	raw := []byte(`{"id":"retired","kind":"dota","status":"playing","version":2,"game":{"kind":"dota","dota":{"sequence":3,"privateLegacyData":[1,2,3]}},"options":{"old":true}}`)
	if _, e = s.db.Exec("INSERT INTO rooms(id,snapshot) VALUES(?,?)", "retired", raw); e != nil {
		t.Fatal(e)
	}
	if _, e = s.db.Exec("INSERT INTO match_history(id,ended,snapshot) VALUES(?,?,?)", "historic-dota", 1, raw); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = New(cfg, files)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if s.rooms["retired"] != nil {
		t.Fatal("retired room restored as live")
	}
	for _, table := range []string{"rooms", "match_history"} {
		var got []byte
		if e = s.db.QueryRow("SELECT snapshot FROM " + table).Scan(&got); e != nil || !bytes.Equal(raw, got) {
			t.Fatalf("%s bytes changed: %v", table, e)
		}
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	c := newClient(t, ts.URL)
	c.register("after-retirement")
	c.post("/api/rooms", map[string]any{"kind": "splendor", "name": "Still available", "capacity": 2}, 201)
}
