package pvz_test

// An opt-in terminal fixture lets the real platform persist and display an
// engine-resolved two-round outcome. The first-round record/last enemy position
// are explicitly controlled; no production option or result is injected by UI.
import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
	pvz "github.com/k0ngk0ng/wire-board/games/pvz/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestPVZFollowupResultBrowserFixture(t *testing.T) {
	if os.Getenv("PVZ_RESULT_BROWSER") != "1" {
		t.Skip("opt-in result browser fixture")
	}
	root, e := filepath.Abs(filepath.Join("..", "..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, ".local/pvz/followup-result")
	if e = os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	m := integrationModule(t)
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		match, e := start(c, o)
		if e != nil {
			return match, e
		}
		var save struct {
			Runtime   engine.State  `json:"runtime"`
			ServerAt  int64         `json:"serverAt"`
			Remainder int64         `json:"remainder"`
			Revision  uint64        `json:"revision"`
			Applied   []pvz.Applied `json:"applied"`
		}
		if e = json.Unmarshal(match.Data, &save); e != nil {
			return match, e
		}
		s := &save.Runtime
		for s.Round < 2 || s.Phase != "battle" {
			if e = engine.Step(s, 1); e != nil {
				return match, e
			}
		}
		s.Rounds[0] = engine.RoundResult{AttackTeam: 1, Breached: true, ElapsedMS: 80050}
		if e = engine.Step(s, 1599); e != nil {
			return match, e
		}
		a := engine.Action{Type: "spawn", Kind: "zombie", Row: 0, Epoch: s.Epoch, Seq: s.Players[0].Seq + 1}
		if e = engine.Apply(s, 0, a); e != nil {
			return match, e
		}
		s.Zombies[0].X = -1
		s.Mowers[0].State = "spent"
		s.Mowers[0].X = 10500
		if e = engine.Validate(*s); e != nil {
			return match, e
		}
		save.ServerAt = c.Now.UnixMilli()
		save.Remainder = 0
		match.Data, e = json.Marshal(save)
		match.SystemDeadline = c.Now.UnixMilli() + 50
		match.CheckpointKey = s.Epoch + "/battle"
		return match, e
	}
	s, e := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "pvz-browser", ClientLoading: true, Modules: []gameapi.Module{m}}, os.DirFS(filepath.Join(root, "web/dist")))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	h := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer h.Close()
	go h.Serve(listener)
	if e = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); e != nil {
		t.Fatal(e)
	}
	for begin := time.Now(); time.Since(begin) < 15*time.Minute; {
		if _, e = os.Stat(filepath.Join(out, "stop")); e == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("result fixture timed out")
}
