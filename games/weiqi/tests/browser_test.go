package weiqi_test

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/weiqi/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestWeiqiBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_WEIQI_BROWSER") != "1" {
		t.Skip("explicit browser acceptance")
	}
	root, e := filepath.Abs("../../..")
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, ".local/weiqi/browser")
	if e = os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	m := game.Module()
	if os.Getenv("WIRE_BOARD_WEIQI_SCENARIO") == "capture" {
		m = fixture(func(s *game.State) {
			for _, p := range []int{0, 2, 12, 18, 20, 28} {
				s.Board[p] = 1
			}
			for _, p := range []int{1, 9, 11, 19} {
				s.Board[p] = 2
			}
		})
	}
	s, e := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "weiqi-browser", AdminUsername: "棋类验收", AdminPassword: "weiqi-browser-only", AssetsBaseURL: os.Getenv("WIRE_BOARD_WEIQI_ASSETS"), Modules: []gameapi.Module{m}}, os.DirFS(filepath.Join(root, "web/dist")))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	httpServer := http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer httpServer.Close()
	go httpServer.Serve(l)
	if e = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+l.Addr().String()), 0600); e != nil {
		t.Fatal(e)
	}
	for until := time.Now().Add(12 * time.Minute); time.Now().Before(until); {
		if _, e = os.Stat(filepath.Join(out, "stop")); e == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser acceptance timeout")
}
