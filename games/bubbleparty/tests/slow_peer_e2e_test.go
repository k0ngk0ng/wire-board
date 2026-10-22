package bubbleparty_test

// Failure inventory: a slow receiver freezes the shared match clock or another
// human's movement; no peer input is treated as a global pause; stale recovery
// changes the world's deadline. Uses the production module and real HTTP users.
import (
	"context"
	"encoding/json"
	bubbleparty "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type delayedBubbleResponse struct {
	http.ResponseWriter
	started chan struct{}
	once    *sync.Once
}

func (w delayedBubbleResponse) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.started) })
	time.Sleep(1800 * time.Millisecond)
	return w.ResponseWriter.Write(p)
}
func TestBubblepartySlowPeerKeepsServerClockAndHealthyHumanMovingHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 2, bubbleparty.Module(), "bubbleparty")
	h.Start(map[string]any{"map": "community10", "roundSeconds": 180})
	confirmCharacters(t, h.People)
	initial := waitPlaying(t, h.People[0], 6*time.Second)
	started := make(chan struct{})
	var once sync.Once
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.Host.Handler().ServeHTTP(delayedBubbleResponse{w, started, &once}, r)
	}))
	defer proxy.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, "GET", proxy.URL+"/api/state", nil)
	done := make(chan error, 1)
	go func() {
		res, e := h.People[1].HTTP.Do(request)
		if e == nil {
			_, e = io.Copy(io.Discard, res.Body)
			res.Body.Close()
		}
		done <- e
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("slow peer never entered response write")
	}
	p := initial.Players[0]
	move := "none"
	for _, d := range []struct {
		name string
		x, y int
	}{{"right", 1, 0}, {"left", -1, 0}, {"down", 0, 1}, {"up", 0, -1}} {
		x, y := p.X+d.x, p.Y+d.y
		if x >= 0 && x < bubbleparty.Cols && y >= 0 && y < bubbleparty.Rows && initial.Cells[y*bubbleparty.Cols+x] == 0 {
			move = d.name
			break
		}
	}
	if move == "none" {
		t.Fatal("no open spawn route")
	}
	samples := []map[string]any{}
	begin := time.Now()
	maxRequest := time.Duration(0)
	for i := 0; i < 10; i++ {
		v := httpView(t, h.People[0])
		at := time.Now()
		h.People[0].Action(map[string]any{"type": "input", "epoch": v.Epoch, "seq": v.Players[0].Seq + 1, "move": move}, 200)
		maxRequest = max(maxRequest, time.Since(at))
		samples = append(samples, map[string]any{"serverAt": v.ServerAt, "x": v.Players[0].PX, "y": v.Players[0].PY, "seq": v.Players[0].Seq})
		time.Sleep(100 * time.Millisecond)
	}
	after := httpView(t, h.People[0])
	elapsed := time.Since(begin)
	if after.ServerAt-initial.ServerAt < 800 || after.MatchEndsAt != initial.MatchEndsAt {
		t.Fatal("slow peer changed server timeline", initial.ServerAt, after.ServerAt)
	}
	if math.Hypot(after.Players[0].PX-p.PX, after.Players[0].PY-p.PY) < .3 {
		t.Fatal("healthy player did not move")
	}
	if elapsed > 1600*time.Millisecond || maxRequest > 350*time.Millisecond {
		t.Fatal("slow response blocked healthy player", elapsed, maxRequest)
	}
	select {
	case e := <-done:
		t.Fatal("slow response was not outstanding during healthy movement", e)
	default:
	}
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	h.Record("slow response isolated; server clock and healthy player continue")
	root, _ := filepath.Abs("../../..")
	out := filepath.Join(root, ".local/optimization-20261023")
	_ = os.MkdirAll(out, 0755)
	evidence := map[string]any{"passed": true, "scope": "real HTTP production bubbleparty; one 1800ms response stall; independent human sends 10 inputs", "elapsedMS": elapsed.Milliseconds(), "maxHealthyRequestMS": maxRequest.Milliseconds(), "serverAdvanceMS": after.ServerAt - initial.ServerAt, "samples": samples}
	b, _ := json.MarshalIndent(evidence, "", "  ")
	if e := os.WriteFile(filepath.Join(out, "slow-peer-http.json"), b, 0644); e != nil {
		t.Fatal(e)
	}
}
