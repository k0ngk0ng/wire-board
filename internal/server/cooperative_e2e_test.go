package server

// Failure scenarios: human-only cooperative wins mint competitive points;
// shared loss creates a fake winner; match history/profile wins omitted or
// duplicated across restart and rematch; solo registered games cannot start.
import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestCooperativeHistoryHTTP(t *testing.T) {
	for _, win := range []bool{false, true} {
		for _, n := range []int{1, 2} {
			t.Run(fmt.Sprintf("win-%t-players-%d", win, n), func(t *testing.T) {
				m := acceptanceModule()
				m.MinPlayers = 1
				m.Configure = func(_ int, _ json.RawMessage) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }
				m.Apply = func(c gameapi.Context, match gameapi.Match, _ json.RawMessage) (gameapi.Match, error) {
					ranks, winners := []int{}, []int{}
					for i := 0; i < c.Players; i++ {
						ranks = append(ranks, 1)
						if win {
							winners = append(winners, i)
						}
					}
					raw, _ := json.Marshal(map[string]any{"ranks": ranks, "winners": winners, "cooperative": true})
					var result gameapi.Result
					_ = json.Unmarshal(raw, &result)
					match.Result = &result
					match.Actors = nil
					match.TimeoutActors = nil
					match.Deadline = 0
					return match, nil
				}
				cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}}
				files := fstest.MapFS{"index.html": {Data: []byte("cooperative HTTP")}}
				s, err := New(cfg, files)
				if err != nil {
					t.Fatal(err)
				}
				ts := httptest.NewServer(s.Handler())
				t.Cleanup(func() { ts.Close(); s.Close() })
				publishModuleForTest(t, ts.URL, "acceptance")
				people := []*testClient{}
				for i := 0; i < n; i++ {
					c := newClient(t, ts.URL)
					c.register(fmt.Sprintf("合作玩家%d", i))
					people = append(people, c)
				}
				a := people[0]
				r := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "合作验证", "capacity": n}, 201)
				for _, c := range people[1:] {
					c.command(r, "join", nil, 200)
					r = current(a)
				}
				for _, c := range people {
					c.command(current(c), "ready", nil, 200)
				}
				a.command(current(a), "start", nil, 200)
				a.command(current(a), "action", map[string]any{}, 200)
				r = current(a)
				if r["status"] != "finished" {
					t.Fatal("not finished", r)
				}
				result := r["result"].(map[string]any)
				if result["rated"] != false || result["cooperative"] != true {
					t.Fatal("cooperative result policy", result)
				}
				for _, p := range result["players"].([]any) {
					v := p.(map[string]any)
					if v["won"] != win || v["ratingDelta"] != nil {
						t.Fatal("wrong winner/rating", v)
					}
				}
				if got := r["game"].(map[string]any)["winners"].([]any); len(got) != func() int {
					if win {
						return n
					}
					return 0
				}() {
					t.Fatal("wrong winners", got)
				}
				var count int
				if err = s.db.QueryRow("SELECT COUNT(*) FROM rating_ledger").Scan(&count); err != nil || count != 0 {
					t.Fatal("coop rating ledger", count, err)
				}
				trace := []any{r}
				ts.Close()
				s.Close()
				s, err = New(cfg, files)
				if err != nil {
					t.Fatal(err)
				}
				ts = httptest.NewServer(s.Handler())
				for _, c := range people {
					c.base = ts.URL
				}
				a.command(current(a), "rematch", nil, 200)
				if current(a)["game"] != nil {
					t.Fatal("rematch retained game")
				}
				for _, c := range people {
					uid := c.state()["user"].(map[string]any)["id"].(string)
					code, profile := c.request("GET", "/api/players/"+uid, nil)
					if code != 200 {
						t.Fatal(profile)
					}
					stats := profile["stats"].(map[string]any)["acceptance"].(map[string]any)
					wins := float64(0)
					if win {
						wins = 1
					}
					if stats["played"] != float64(1) || stats["wins"] != wins {
						t.Fatal("cooperative stats", stats)
					}
					trace = append(trace, profile)
				}
				out := filepath.Join("..", "..", ".local", "starfort", "http")
				if err = os.MkdirAll(out, 0755); err != nil {
					t.Fatal(err)
				}
				raw, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "test": t.Name(), "trace": trace}, "", "  ")
				if err = os.WriteFile(filepath.Join(out, fmt.Sprintf("cooperative-%t-%d.json", win, n)), raw, 0600); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
