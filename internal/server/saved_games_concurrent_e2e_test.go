package server

// Contract E2E first. Concurrent original accounts/retries must not fork a
// saved match, invoke ResumeSaved twice, duplicate joined identities, change
// progress or create settlement records. All calls are real authenticated
// HTTP against SQLite; retained evidence records each supported roster size.
import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestSavedGameConcurrentResumeHTTP(t *testing.T) {
	reports := []map[string]any{}
	for _, n := range []int{1, 2, 3} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			var resumes atomic.Int32
			m := savedGameFixture()
			m.ResumeSaved = func(_ gameapi.Context, g gameapi.Match) (gameapi.Match, error) { resumes.Add(1); return g, nil }
			s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("concurrent save contract")}})
			if err != nil {
				t.Fatal(err)
			}
			stopBotTicker(s)
			ts := httptest.NewServer(s.Handler())
			defer ts.Close()
			defer s.Close()
			players := make([]*testClient, n)
			for i := range players {
				players[i] = newClient(t, ts.URL)
				players[i].register(fmt.Sprintf("并发续档%d人%d", n, i))
			}
			owner := players[0]
			room := owner.post("/api/rooms", map[string]any{"kind": m.ID, "name": "并发续档", "capacity": n}, 201)
			id := room["id"].(string)
			for _, p := range players[1:] {
				p.command(current(owner), "join", nil, 200)
			}
			for _, p := range players {
				p.command(current(p), "ready", nil, 200)
			}
			owner.command(current(owner), "start", nil, 200)
			before := current(owner)
			beforeData, _ := json.Marshal(before["game"].(map[string]any)["data"])
			s.mu.Lock()
			matchID := s.rooms[id].MatchID
			s.mu.Unlock()
			for _, p := range players {
				p.command(current(p), "save", nil, 200)
			}
			start := make(chan struct{})
			results := make(chan error, n*12)
			var wg sync.WaitGroup
			for _, p := range players {
				for i := 0; i < 12; i++ {
					wg.Add(1)
					go func(p *testClient) {
						defer wg.Done()
						<-start
						req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/saves/"+id+"/resume", strings.NewReader(`{}`))
						if err != nil {
							results <- err
							return
						}
						req.Header.Set("Content-Type", "application/json")
						res, err := p.client.Do(req)
						if err != nil {
							results <- err
							return
						}
						defer res.Body.Close()
						var body map[string]any
						if err = json.NewDecoder(res.Body).Decode(&body); err != nil {
							results <- err
							return
						}
						if res.StatusCode != 200 {
							results <- fmt.Errorf("resume returned %d: %v", res.StatusCode, body)
							return
						}
						results <- nil
					}(p)
				}
			}
			close(start)
			wg.Wait()
			close(results)
			for err := range results {
				if err != nil {
					t.Fatal(err)
				}
			}
			after := current(owner)
			afterData, _ := json.Marshal(after["game"].(map[string]any)["data"])
			if string(beforeData) != string(afterData) || after["status"] != "playing" || after["id"] != id || resumes.Load() != 1 {
				t.Fatal("concurrent resume forked/restarted the saved match", resumes.Load(), after)
			}
			s.mu.Lock()
			r := s.rooms[id]
			same := r.MatchID == matchID
			joined := append([]string(nil), r.ResumeJoined...)
			count := len(s.rooms)
			s.mu.Unlock()
			unique := map[string]bool{}
			for _, id := range joined {
				unique[id] = true
			}
			if !same || len(joined) != n || len(unique) != n || count != 1 {
				t.Fatal("concurrent resume changed the original identity/roster", joined, count)
			}
			var stored, history, ratings int
			for _, q := range []struct {
				query string
				out   *int
			}{{"SELECT COUNT(*) FROM rooms", &stored}, {"SELECT COUNT(*) FROM match_history", &history}, {"SELECT COUNT(*) FROM rating_ledger", &ratings}} {
				if err = s.db.QueryRow(q.query).Scan(q.out); err != nil {
					t.Fatal(err)
				}
			}
			if stored != 1 || history != 0 || ratings != 0 {
				t.Fatal("concurrent resume created duplicate persistence/settlement", stored, history, ratings)
			}
			reports = append(reports, map[string]any{"players": n, "concurrentRequests": n * 12, "resumeCallbackCalls": resumes.Load(), "sameRoomAndMatch": true, "uniqueOriginalConfirmations": len(unique), "storedRooms": stored, "history": history, "ratings": ratings})
		})
	}
	if t.Failed() {
		return
	}
	out := filepath.Join("..", "..", ".local/saved-games")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "real authenticated concurrent HTTP resume/retry for one to three original accounts, SQLite single-instance identity and no settlement; platform contract fixture", "cases": reports}, "", "  ")
	if err := os.WriteFile(filepath.Join(out, "concurrent-resume.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
}
