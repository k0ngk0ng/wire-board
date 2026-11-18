package server

// Browser contract declared before its UI: real production React, authenticated
// original players, native save/resume controls, progress preservation, opt-in
// visibility and 320px touch. The test-only opt-in does not change Gomoku policy.

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	gomoku "github.com/k0ngk0ng/wire-board/games/gomoku/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestSavedGameBrowserFixture(t *testing.T) {
	if os.Getenv("SAVED_GAME_BROWSER") != "1" {
		t.Skip("explicit browser acceptance only")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local", "saved-games", "browser")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(out, "stop")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	m := gomoku.Module()
	m.SupportsSave = true
	// Test-only consent pause; Gomoku's production registration stays untouched.
	m.SaveControl = func(c gameapi.Context, g gameapi.Match, saving bool) (gameapi.Match, error) {
		var d map[string]any
		if e := json.Unmarshal(g.Data, &d); e != nil {
			return g, e
		}
		turn := int(d["turn"].(float64))
		if saving {
			d["saveRemainingMS"] = max(int64(1), g.Deadline-c.Now.UnixMilli())
			g.Deadline = 0
			g.TimeoutActors = nil
			g.Attention = []int{}
		} else {
			g.Deadline = c.Now.UnixMilli() + int64(d["saveRemainingMS"].(float64))
			d["deadline"] = g.Deadline
			delete(d, "saveRemainingMS")
			g.TimeoutActors = []int{turn}
			g.Attention = []int{turn}
		}
		g.Paused = saving
		g.Data, _ = json.Marshal(d)
		return g, nil
	}
	m.ResumeSaved = func(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) { return m.SaveControl(c, g, false) }
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", ClientLoading: true, AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}}, os.DirFS(filepath.Join(root, "web", "dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer h.Close()
	go h.Serve(listener)
	if err = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for started := time.Now(); time.Since(started) < 30*time.Minute; {
		if _, err = os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture timed out")
}
