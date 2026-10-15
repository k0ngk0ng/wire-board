package xiangqi_test

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/xiangqi/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestXiangqiBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_XIANGQI_BROWSER") != "1" {
		t.Skip("explicit browser acceptance")
	}
	root, e := filepath.Abs("../../..")
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, ".local/xiangqi/browser")
	if e = os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	m := game.Module()
	if fen := os.Getenv("WIRE_BOARD_XIANGQI_FEN"); fen != "" {
		m = position(fen)
	}
	start := m.Start
	// Standard initial board; fix only the colour lottery for replayable
	// mouse/touch coordinates. No injected middle-game state or winner.
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		if e != nil {
			return v, e
		}
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		s.Red, s.Turn = 0, 0
		s.History = []game.Record{{Key: game.PositionKey(s.Board, s.Turn), Seat: -1}}
		v.Data, _ = json.Marshal(s)
		v.Attention = []int{0}
		v.TimeoutActors = []int{0}
		return v, nil
	}
	s, e := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "xiangqi-browser", AdminUsername: "棋类验收", AdminPassword: "xiangqi-browser-only", AssetsBaseURL: os.Getenv("WIRE_BOARD_XIANGQI_ASSETS"), Modules: []gameapi.Module{m}}, os.DirFS(filepath.Join(root, "web/dist")))
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
