package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALOriginalGameOverBrowserFixture(t *testing.T) {
	if os.Getenv("PAL_GAME_OVER_BROWSER") != "1" {
		t.Skip("explicit earned original rescue battle, natural defeat and recovery Board fixture")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, ".local/pal/verification/runtime-candidates/game-over")
	data, err := os.ReadFile(filepath.Join(base, "game-over-low-health-state-1.json"))
	if err != nil {
		t.Fatal("run the actual original campaign source route first", err)
	}
	var receipt struct {
		Passed bool
		SHA256 string
	}
	proof, err := os.ReadFile(filepath.Join(base, "game-over-low-health-origin-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(data)
	if err = json.Unmarshal(proof, &receipt); err != nil || !receipt.Passed || receipt.SHA256 != hex.EncodeToString(h[:]) {
		t.Fatal("earned battle source receipt mismatch", err)
	}
	out := filepath.Join(root, ".local/pal/game-over-board-browser")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(out, "stop"))
	m := palBoardModuleAt(t, ".local/pal/runtime-candidates/game-over/original/rules.wasm")
	start := m.Start
	m.Start = func(c gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
		if c.Version > 2 {
			return start(c, options)
		}
		return m.ResumeSaved(c, gameapi.Match{Schema: 1, Unrated: true, Data: append(json.RawMessage(nil), data...)})
	}
	save := m.SaveControl
	m.SaveControl = func(c gameapi.Context, g gameapi.Match, paused bool) (gameapi.Match, error) {
		next, err := save(c, g, paused)
		if err != nil || !paused {
			return next, err
		}
		v, err := m.View(next, 0)
		if err != nil {
			return next, err
		}
		b, err := json.Marshal(v)
		if err == nil {
			err = os.WriteFile(filepath.Join(out, "frozen-view.json"), b, 0600)
		}
		return next, err
	}
	restore := m.RestoreCheckpoint
	m.RestoreCheckpoint = func(c gameapi.Context, saved, active gameapi.Match) (gameapi.Match, error) {
		next, err := restore(c, saved, active)
		if err != nil {
			return next, err
		}
		v, err := m.View(next, 0)
		if err != nil {
			return next, err
		}
		b, err := json.Marshal(v)
		if err == nil {
			err = os.WriteFile(filepath.Join(out, "loaded-view.json"), b, 0600)
		}
		return next, err
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", ClientLoading: true, Modules: []gameapi.Module{m}}, os.DirFS(filepath.Join(root, "web/dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mux := http.NewServeMux()
	mux.Handle("/pal/v1/", http.StripPrefix("/pal/v1/", http.FileServer(http.Dir(filepath.Join(root, ".local/pal/visual/pal/v1")))))
	mux.Handle("/", s.Handler())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer httpServer.Close()
	go httpServer.Serve(listener)
	if err = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for began := time.Now(); time.Since(began) < 20*time.Minute; {
		if _, err := os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("original game-over browser fixture timed out")
}
