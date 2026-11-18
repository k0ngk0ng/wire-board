package server

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALEndingBoardBrowserFixture(t *testing.T) {
	if os.Getenv("PAL_ENDING_BOARD_BROWSER") != "1" {
		t.Skip("explicit isolated original ending Board browser fixture")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local/pal/ending-board-browser")
	program := ".local/pal/ending-fixture/rules.wasm"
	if os.Getenv("PAL_ENDING_BOARD_CREDITS") == "1" {
		out = filepath.Join(root, ".local/pal/ending-credits-board-browser")
		program = ".local/pal/ending-credits-fixture/rules.wasm"
	}
	if os.Getenv("PAL_ENDING_BOARD_PRESENTATION") == "1" {
		out = filepath.Join(root, ".local/pal/presentation-ending-board-browser")
		program = ".local/pal/presentation-ending-fixture/rules.wasm"
	}
	if os.Getenv("PAL_ENDING_BOARD_CHECKPOINT") == "1" {
		out = filepath.Join(root, ".local/pal/checkpoint-ending-board-browser")
		program = ".local/pal/presentation-ending-fixture/rules.wasm"
	}
	if os.Getenv("PAL_ENDING_CDN") == "1" {
		out = filepath.Join(filepath.Dir(out), "cdn-"+filepath.Base(out))
	}
	if os.Getenv("PAL_VISUAL_CDN") == "1" {
		out = filepath.Join(filepath.Dir(out), "visual-"+filepath.Base(out))
	}
	campaign := os.Getenv("PAL_ENDING_CAMPAIGN") == "1"
	if campaign {
		out = filepath.Join(filepath.Dir(out), "campaign-"+filepath.Base(out))
	}
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(out, "stop"))
	var module gameapi.Module
	if campaign {
		var checkpoint []byte
		module, checkpoint = palCampaignBrowserModule(t, "ending-first")
		if err := os.WriteFile(filepath.Join(out, "entry-state.json"), checkpoint, 0600); err != nil {
			t.Fatal(err)
		}
	} else {
		module = palBoardModuleAt(t, program)
	}
	freeze := module.SaveControl
	module.SaveControl = func(c gameapi.Context, g gameapi.Match, paused bool) (gameapi.Match, error) {
		next, err := freeze(c, g, paused)
		if err != nil || !paused {
			return next, err
		}
		v, err := module.View(next, 0)
		if err != nil {
			return next, err
		}
		data, err := json.Marshal(v)
		if err == nil {
			err = os.WriteFile(filepath.Join(out, "frozen-view.json"), data, 0600)
		}
		return next, err
	}
	resume := module.ResumeSaved
	module.ResumeSaved = func(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) {
		next, err := resume(c, g)
		if err != nil {
			return next, err
		}
		v, err := module.View(next, 0)
		if err != nil {
			return next, err
		}
		data, err := json.Marshal(v)
		if err == nil {
			err = os.WriteFile(filepath.Join(out, "restored-view.json"), data, 0600)
		}
		return next, err
	}
	checkpoint := module.RestoreCheckpoint
	module.RestoreCheckpoint = func(c gameapi.Context, saved, active gameapi.Match) (gameapi.Match, error) {
		next, err := checkpoint(c, saved, active)
		if err != nil {
			return next, err
		}
		v, err := module.View(next, 0)
		if err != nil {
			return next, err
		}
		data, err := json.Marshal(v)
		if err == nil {
			err = os.WriteFile(filepath.Join(out, "checkpoint-restored-view.json"), data, 0600)
		}
		return next, err
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", ClientLoading: true, Modules: []gameapi.Module{module}}, os.DirFS(filepath.Join(root, "web/dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mux := http.NewServeMux()
	if os.Getenv("PAL_VISUAL_CDN") == "1" {
		mux.Handle("/pal/v1/", http.NotFoundHandler())
	} else {
		mux.Handle("/pal/v1/", http.StripPrefix("/pal/v1/", http.FileServer(http.Dir(filepath.Join(root, ".local/pal/visual/pal/v1")))))
	}
	if os.Getenv("PAL_ENDING_CDN") == "1" {
		mux.Handle("/pal/ending-v1/", http.NotFoundHandler())
	} else {
		mux.Handle("/pal/ending-v1/", http.StripPrefix("/pal/ending-v1/", http.FileServer(http.Dir(filepath.Join(root, ".local/pal/ending-media-e2e")))))
	}
	mux.Handle("/", s.Handler())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer h.Close()
	go h.Serve(listener)
	if err := os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for began := time.Now(); time.Since(began) < 20*time.Minute; {
		if _, err := os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("ending browser fixture timed out")
}
