package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"slices"
	"testing"
	"testing/fstest"

	registry "github.com/k0ngk0ng/wire-board/games"
)

func TestRetiredIndependentGamesHTTP(t *testing.T) {
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "RetireRoot", AdminPassword: "retirement-local-password", Modules: registry.Modules()}
	s, err := New(cfg, fstest.MapFS{"index.html": {Data: []byte("retirement acceptance")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	root := newClient(t, ts.URL)
	root.post("/api/login", map[string]any{"name": "RetireRoot", "password": "retirement-local-password"}, 200)
	for _, kind := range []string{"stoneage", "spf2t"} {
		if slices.Contains(s.availableGames(), kind) {
			t.Fatalf("retired game remains available: %s", kind)
		}
		root.post("/api/rooms", map[string]any{"kind": kind, "name": "Retired", "capacity": 2}, 400)
		root.post("/api/admin/games/"+kind, map[string]bool{"enabled": true}, 404)
		t.Logf("%s: unavailable; create=400; admin resurrection=404", kind)
	}
	root.post("/api/rooms", map[string]any{"kind": "gomoku", "name": "Still available", "capacity": 2}, 201)
}

func TestRetiredIndependentGamesRestoreKeepsRecords(t *testing.T) {
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: registry.Modules()}
	files := fstest.MapFS{"index.html": {Data: []byte("retirement restore")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	originals := map[string][]byte{}
	for _, kind := range []string{"stoneage", "spf2t"} {
		raw, marshalErr := json.Marshal(map[string]any{"id": "retired-" + kind, "kind": kind, "status": "playing", "capacity": 2, "version": 2, "module": map[string]any{"kind": kind, "data": map[string]any{"retiredVersion": 1, "private": []int{1, 2, 3}}}})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		originals[kind] = raw
		if _, err = s.db.Exec("INSERT INTO rooms(id,snapshot) VALUES(?,?)", "retired-"+kind, raw); err != nil {
			t.Fatal(err)
		}
		if _, err = s.db.Exec("INSERT INTO match_history(id,ended,snapshot) VALUES(?,?,?)", "historic-"+kind, 1, raw); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for kind, raw := range originals {
		if s.rooms["retired-"+kind] != nil {
			t.Fatalf("retired match restored as live: %s", kind)
		}
		for _, record := range []struct{ table, id string }{{"rooms", "retired-" + kind}, {"match_history", "historic-" + kind}} {
			var actual []byte
			if err = s.db.QueryRow("SELECT snapshot FROM "+record.table+" WHERE id=?", record.id).Scan(&actual); err != nil || !bytes.Equal(actual, raw) {
				t.Fatalf("%s %s changed stored bytes: %v", kind, record.table, err)
			}
		}
		t.Logf("%s: retired active room skipped; room and history bytes preserved", kind)
	}
}
