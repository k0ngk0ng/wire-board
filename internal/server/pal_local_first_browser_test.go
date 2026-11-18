package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALLocalFirstBrowserFixture(t *testing.T) {
	mode := os.Getenv("PAL_LOCAL_FIRST_BROWSER")
	if mode != "battle" && mode != "dialogue" {
		t.Skip("explicit original local-first browser acceptance")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local/pal/verification/local-first", mode+"-fixture")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(out, "stop"))
	module := palBoardModule(t)
	if mode == "dialogue" {
		base := module
		module.Start = func(c gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
			match, err := base.Start(c, options)
			if err != nil {
				return match, err
			}
			clock := c
			for step := 0; step < 4096; step++ {
				value, err := base.View(match, 0)
				if err != nil {
					return match, err
				}
				view := value.(pal.View)
				if view.Dialogue != nil {
					imported, err := palBrowserImportClock(c, match)
					if err != nil {
						return match, err
					}
					return base.ResumeSaved(c, imported)
				} else {
					clock.Now = time.UnixMilli(match.SystemDeadline + 200)
					match, err = base.OnTimer(clock, match)
					if err != nil {
						return match, err
					}
				}
			}
			t.Fatal("original adjacent dialogue boundary was not reached")
			return match, nil
		}
	}
	dist := filepath.Join(root, ".local/pal/local-first-web-dist")
	var core []byte
	if mode == "battle" {
		core, err = os.ReadFile(filepath.Join(root, ".local/pal/local-first-battle/native/rules.wasm"))
		if err != nil {
			t.Fatal(err)
		}
		module = palBoardModuleAt(t, ".local/pal/local-first-battle/native/rules.wasm")
		dist = filepath.Join(root, ".local/pal/local-first-battle/web-dist")
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", ClientLoading: true, Modules: []gameapi.Module{module}}, os.DirFS(dist))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mux := http.NewServeMux()
	hash := sha256.Sum256(core)
	identity := hex.EncodeToString(hash[:]) + ".wasm"
	mux.HandleFunc("GET /pal-e2e-core/{id}", func(w http.ResponseWriter, r *http.Request) {
		if mode != "battle" || r.PathValue("id") != identity {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/wasm")
		http.ServeContent(w, r, identity, time.Time{}, bytes.NewReader(core))
	})
	mux.Handle("/", s.Handler())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer host.Close()
	go host.Serve(listener)
	if err := os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for begin := time.Now(); time.Since(begin) < 25*time.Minute; {
		if _, err := os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("local-first original browser fixture timed out")
}
