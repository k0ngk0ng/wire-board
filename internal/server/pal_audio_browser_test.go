package server

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALOriginalAudioBoardBrowserFixture(t *testing.T) {
	if os.Getenv("PAL_AUDIO_BROWSER") != "1" {
		t.Skip("explicit original audio Board browser fixture")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local/pal/audio-board-browser")
	if os.Getenv("PAL_VISUAL_CDN") == "1" {
		out = filepath.Join(root, ".local/pal/cdn-visual-board-browser")
	}
	webDist := filepath.Join(root, "web/dist")
	if scope := os.Getenv("PAL_AUDIO_BROWSER_SCOPE"); scope != "" {
		if !regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`).MatchString(scope) {
			t.Fatal("invalid audio browser scope", scope)
		}
		out = filepath.Join(root, ".local/pal/performance/battle-recovery-"+scope)
		webDist = filepath.Join(out, "web-dist")
		if _, err := os.Stat(filepath.Join(webDist, "index.html")); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(out, "stop"))
	module := palBoardModuleAt(t, ".local/pal/runtime-candidates/audio/original/rules.wasm")
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", ClientLoading: true, Modules: []gameapi.Module{module}}, os.DirFS(webDist))
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
	audioIndex, err := os.ReadFile(filepath.Join(root, ".local/pal/audio-media-e2e/index.json"))
	if err != nil {
		t.Fatal(err)
	}
	audioHash := sha256.Sum256(audioIndex)
	audioPath := "/pal/audio-v1/" + hex.EncodeToString(audioHash[:]) + "/"
	if os.Getenv("PAL_AUDIO_CDN") == "1" {
		mux.Handle("/pal/audio-v1/", http.NotFoundHandler())
	} else {
		mux.Handle(audioPath, http.StripPrefix(audioPath, http.FileServer(http.Dir(filepath.Join(root, ".local/pal/audio-media-e2e")))))
	}
	mux.Handle("/", s.Handler())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer h.Close()
	go h.Serve(listener)
	if err = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for began := time.Now(); time.Since(began) < 18*time.Minute; {
		if _, err = os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("audio Board fixture timed out")
}
