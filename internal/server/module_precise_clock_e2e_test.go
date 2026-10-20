package server

// Failure inventory before implementation: a non-realtime system animation
// waits for the 1s player-timeout ticker; two short nodes collapse into one;
// faster delivery changes player deadlines/ownership or commits an event twice.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

func TestModuleSystemDeadlinesUsePreciseLiveClock(t *testing.T) {
	m := eventModule()
	m.PreciseTimers = true
	m.Start = func(ctx gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Schema: 1, Data: json.RawMessage(`{"times":[]}`), Actors: []int{0, 1, 2}, Attention: []int{}, TimeoutActors: []int{0}, Deadline: ctx.Now.Add(30 * time.Second).UnixMilli(), SystemDeadline: ctx.Now.Add(50 * time.Millisecond).UnixMilli()}, nil
	}
	m.OnTimer = func(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		var data struct {
			Times []int64 `json:"times"`
		}
		if err := json.Unmarshal(match.Data, &data); err != nil {
			return match, err
		}
		data.Times = append(data.Times, ctx.Now.UnixMilli())
		match.Data, _ = json.Marshal(data)
		match.SystemDeadline = 0
		if len(data.Times) < 2 {
			match.SystemDeadline = ctx.Now.Add(50 * time.Millisecond).UnixMilli()
		}
		return match, nil
	}
	m.View = func(match gameapi.Match, _ int) (any, error) {
		var x any
		err := json.Unmarshal(match.Data, &x)
		return x, err
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("ok")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	publishModuleForTest(t, ts.URL, "acceptance")
	a, b, c := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("精确时钟甲")
	b.register("精确时钟乙")
	c.register("精确时钟丙")
	r := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "精确系统时钟", "capacity": 3}, 201)
	b.command(r, "join", nil, 200)
	c.command(current(a), "join", nil, 200)
	for _, client := range []*testClient{a, b, c} {
		client.command(current(client), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	before := current(a)
	deadline := before["turnDeadline"]
	var times []any
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end); {
		r = current(a)
		times = r["game"].(map[string]any)["data"].(map[string]any)["times"].([]any)
		if len(times) >= 2 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	if len(times) != 2 {
		t.Fatalf("expected two live nodes, got %v", times)
	}
	gap := times[1].(float64) - times[0].(float64)
	dir := filepath.Join("..", "..", ".local", "fortressblue-refactor", "host-clock")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	record, _ := json.MarshalIndent(map[string]any{"times": times, "gapMS": gap, "deadline": deadline}, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "live-system-clock.json"), record, 0644); err != nil {
		t.Fatal(err)
	}
	if gap > 250 {
		t.Fatalf("50ms system nodes delivered %.0fms apart; player-timeout ticker must not drive animation", gap)
	}
	if r["turnDeadline"] != deadline {
		t.Fatal("system nodes changed the player's deadline")
	}
	for _, seat := range r["seats"].([]any) {
		if seat.(map[string]any)["autoPlay"] == true {
			t.Fatal("system event enabled autoplay")
		}
	}
	time.Sleep(1100 * time.Millisecond)
	r = current(a)
	times = r["game"].(map[string]any)["data"].(map[string]any)["times"].([]any)
	if len(times) != 2 {
		t.Fatalf("slow ticker duplicated an event: %v", times)
	}
}
