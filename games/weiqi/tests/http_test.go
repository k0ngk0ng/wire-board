package weiqi_test

// Authored before implementation. Failure matrix is frozen in
// docs/research/chess-family/weiqi-implementation.md. These are real HTTP/SQLite
// tests; explicit midgame fixtures are never reported as natural complete games.
import (
	"encoding/json"
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/weiqi/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func state(c *gametest.Client) game.State {
	raw, _ := json.Marshal(gametest.Data(c.Room()))
	var s game.State
	_ = json.Unmarshal(raw, &s)
	return s
}
func action(s game.State, kind string) map[string]any {
	return map[string]any{"type": kind, "round": s.Round, "ply": s.Ply, "revision": s.Revision}
}
func put(h *gametest.HTTP, x, y int) game.State {
	s := state(h.People[0])
	a := action(s, "place")
	a["pos"] = y*s.Options.Size + x
	h.People[s.Turn].Action(a, 200)
	return state(h.People[0])
}
func pass(h *gametest.HTTP) game.State {
	s := state(h.People[0])
	h.People[s.Turn].Action(action(s, "pass"), 200)
	return state(h.People[0])
}
func mark(h *gametest.HTTP, seat, pos int) game.State {
	s := state(h.People[0])
	a := action(s, "mark_dead")
	a["pos"] = pos
	h.People[seat].Action(a, 200)
	return state(h.People[0])
}
func record(t *testing.T, name string, v any) {
	t.Helper()
	p := filepath.Join("..", "..", "..", ".local", "weiqi", "http")
	if e := os.MkdirAll(p, 0755); e != nil {
		t.Fatal(e)
	}
	b, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "test": t.Name(), "evidence": v}, "", "  ")
	if e := os.WriteFile(filepath.Join(p, name+".json"), b, 0600); e != nil {
		t.Fatal(e)
	}
}
func fixture(change func(*game.State)) gameapi.Module {
	m := game.Module()
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		if e != nil {
			return v, e
		}
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		s.Black = 0
		s.Turn = 0
		change(&s)
		s.Keys = []string{game.PositionKey(s.Board)}
		v.Data, _ = json.Marshal(s)
		v.Attention = []int{s.Turn}
		v.TimeoutActors = []int{s.Turn}
		return v, nil
	}
	return m
}
func TestWeiqiHTTPSetupAndRestore(t *testing.T) {
	for _, n := range []int{9, 13, 19} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			h := gametest.NewHTTP(t, 2, game.Module(), "weiqi")
			h.AssertUnpublished()
			h.Start(map[string]any{"size": n})
			s := state(h.People[0])
			if len(s.Board) != n*n || s.Turn != s.Black || s.Options.Komi != 7.5 || len(s.Keys) != 1 {
				t.Fatal(s)
			}
			a := action(s, "place")
			a["pos"] = n * n / 2
			h.People[1-s.Turn].Action(a, 400)
			h.Spectator.Action(a, 400)
			for _, pos := range []int{-1, n * n} {
				b := action(s, "place")
				b["pos"] = pos
				h.People[s.Turn].Action(b, 400)
			}
			h.People[s.Turn].Action(action(s, "place"), 400)
			old := s
			payload := h.People[s.Turn].ActionRequest(a)
			url := "/api/rooms/" + h.People[0].Room()["id"].(string)
			h.People[s.Turn].Request("POST", url, payload, 200)
			h.People[s.Turn].Request("POST", url, payload, 200)
			s = state(h.People[0])
			if s.Ply != 1 || s.Board[n*n/2] != 1 {
				t.Fatal("duplicate placement")
			}
			a = action(old, "place")
			a["pos"] = 0
			h.People[s.Turn].Action(a, 400)
			a = action(s, "place")
			a["pos"] = n * n / 2
			h.People[s.Turn].Action(a, 400)
			before := state(h.People[0])
			h.Restart()
			if !reflect.DeepEqual(before, state(h.People[0])) {
				t.Fatal("restore differs")
			}
			put(h, 0, 0)
			record(t, fmt.Sprintf("setup-%d", n), state(h.People[0]))
			h.Record("natural opening, duplicate/illegal actions and restored SQLite")
		})
	}
}
func TestWeiqiHTTPOptions(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "weiqi")
	c := h.People[0]
	c.Request("POST", "/api/admin/games/weiqi", map[string]bool{"enabled": true}, 200)
	for _, o := range []any{map[string]any{"size": 10}, map[string]any{"komi": 6.5}, map[string]any{"difficulty": "pro"}, map[string]any{"turnSeconds": 0}, map[string]any{"handicap": 2}} {
		c.Request("POST", "/api/rooms", map[string]any{"kind": "weiqi", "name": "invalid", "capacity": 2, "options": o}, 400)
	}
	c.Request("POST", "/api/rooms", map[string]any{"kind": "weiqi", "name": "invalid", "capacity": 3}, 400)
	record(t, "invalid-options", true)
}
func TestWeiqiHTTPMultiCaptureAndSuicide(t *testing.T) {
	h := gametest.NewHTTP(t, 2, fixture(func(s *game.State) {
		for _, p := range []int{0, 2, 12, 18, 20, 28} {
			s.Board[p] = 1
		}
		for _, p := range []int{1, 9, 11, 19} {
			s.Board[p] = 2
		}
	}), "weiqi")
	h.Start(map[string]any{"size": 9})
	s := put(h, 1, 1)
	if s.Captured[0] != 4 || len(s.Last.Removed) != 4 || s.Board[10] != 1 {
		t.Fatal("multi capture", s)
	}
	for _, p := range []int{1, 9, 11, 19} {
		if s.Board[p] != 0 {
			t.Fatal("uncaptured", p)
		}
	}
	a := action(s, "place")
	a["pos"] = 1
	h.People[s.Turn].Action(a, 400)
	if !reflect.DeepEqual(s, state(h.People[0])) {
		t.Fatal("rejected suicide mutated state")
	}
	record(t, "multi-capture-midgame", s)
	h2 := gametest.NewHTTP(t, 2, fixture(func(s *game.State) { s.Board[1] = 2; s.Board[9] = 2; s.Board[10] = 1 }), "weiqi")
	h2.Start(map[string]any{"size": 9})
	s = state(h2.People[0])
	a = action(s, "place")
	a["pos"] = 0
	h2.People[0].Action(a, 400)
	put(h2, 2, 2)
	record(t, "diagonal-not-connected-midgame", state(h2.People[0]))
}
func koFixture() gameapi.Module {
	return fixture(func(s *game.State) {
		for _, p := range []int{1, 9, 19} {
			s.Board[p] = 1
		}
		for _, p := range []int{10, 2, 12, 20} {
			s.Board[p] = 2
		}
	})
}
func TestWeiqiHTTPKoAndHistory(t *testing.T) {
	h := gametest.NewHTTP(t, 2, koFixture(), "weiqi")
	h.Start(map[string]any{"size": 9})
	initial := state(h.People[0])
	s := put(h, 2, 1)
	if s.Captured[0] != 1 {
		t.Fatal(s)
	}
	a := action(s, "place")
	a["pos"] = 10
	h.People[s.Turn].Action(a, 400)
	pass(h)
	pass(h)
	s = state(h.People[0])
	h.People[0].Action(action(s, "resume"), 200)
	s = state(h.People[0])
	a = action(s, "place")
	a["pos"] = 10
	h.People[s.Turn].Action(a, 400)
	before := state(h.People[0])
	h.Restart()
	if !reflect.DeepEqual(before, state(h.People[0])) {
		t.Fatal("ko history lost")
	}
	put(h, 8, 8)
	put(h, 7, 8)
	s = put(h, 1, 1)
	if s.Board[11] != 0 || s.Captured[1] != 1 || len(s.Keys) != 5 || s.Keys[0] != game.PositionKey(initial.Board) {
		t.Fatal("ko threat recapture failed", s)
	}
	record(t, "ko-midgame-and-restore", s)
}
func TestWeiqiHTTPScoringProposal(t *testing.T) {
	h := gametest.NewHTTP(t, 2, fixture(func(s *game.State) { s.Board[0] = 1; s.Board[1] = 1; s.Board[80] = 2 }), "weiqi")
	h.Start(map[string]any{"size": 9})
	pass(h)
	s := pass(h)
	if s.Phase != "scoring" || s.Turn != 0 {
		t.Fatal(s)
	}
	deadline := s.Deadlines
	old := s
	h.People[0].Action(action(s, "confirm_score"), 200)
	s = state(h.People[0])
	if !s.Confirmed[0] {
		t.Fatal(s)
	}
	s = mark(h, 1, 0)
	if len(s.Dead) != 2 || s.Confirmed[0] || s.Confirmed[1] || s.Deadlines != deadline {
		t.Fatal("group marking or clocks", s)
	}
	h.People[1].Action(action(old, "confirm_score"), 400)
	h.Spectator.Action(action(s, "confirm_score"), 400)
	before := s
	h.Restart()
	if !reflect.DeepEqual(before, state(h.People[0])) {
		t.Fatal("score restore lost")
	}
	h.People[0].Action(action(s, "resume"), 200)
	s = state(h.People[0])
	if s.Phase != "playing" || s.Turn != 0 || len(s.Dead) != 0 || s.Board[0] != 1 || s.Passes != 0 || !reflect.DeepEqual(s.Keys, before.Keys) {
		t.Fatal("resume changed position", s)
	}
	pass(h)
	pass(h)
	s = mark(h, 1, 0)
	h.People[0].Action(action(s, "confirm_score"), 200)
	h.People[1].Action(action(s, "confirm_score"), 200)
	s = state(h.People[0])
	if s.Phase != "finished" || s.Winner != 1 || s.Score.Black != 0 || s.Score.White != 88.5 || s.Board[0] != 0 || s.Board[1] != 0 {
		t.Fatal("dead removal score", s)
	}
	h.Record("midgame dead-group proposal, stale rejection, restore, resume and score")
	record(t, "score-proposal-midgame", s)
}
func TestWeiqiHTTPAreaAndDraw(t *testing.T) {
	for _, komi := range []float64{0, 7.5} {
		t.Run(fmt.Sprint(komi), func(t *testing.T) {
			h := gametest.NewHTTP(t, 2, fixture(func(s *game.State) { s.Board[0] = 1; s.Board[80] = 2; s.Captured = [2]int{13, 7} }), "weiqi")
			h.Start(map[string]any{"size": 9, "komi": komi})
			pass(h)
			s := pass(h)
			if s.Score.Black != 1 || s.Score.White != 1+komi || s.Score.Neutral != 79 {
				t.Fatal("neutral or prisoners counted", s.Score)
			}
			h.People[0].Action(action(s, "confirm_score"), 200)
			h.People[1].Action(action(s, "confirm_score"), 200)
			s = state(h.People[0])
			result := h.People[0].Room()["result"].(map[string]any)
			if (s.Winner < 0) != (komi == 0) {
				t.Fatal(s)
			}
			if komi == 0 && result["draw"] != true {
				t.Fatal(result)
			}
			record(t, fmt.Sprintf("area-neutral-komi-%v", komi), map[string]any{"state": s, "result": result})
			h.Record("area accounting excludes prisoners; neutral empty region")
		})
	}
}
func TestWeiqiHTTPPassAndResign(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "weiqi")
	h.Start(map[string]any{"size": 9})
	s := pass(h)
	if s.Passes != 1 || s.Phase != "playing" {
		t.Fatal(s)
	}
	s = put(h, 4, 4)
	if s.Passes != 0 {
		t.Fatal(s)
	}
	a := action(s, "pass")
	a["pos"] = 0
	h.People[s.Turn].Action(a, 400)
	h.People[1-s.Turn].Action(action(s, "resign"), 200)
	s = state(h.People[0])
	if s.Phase != "finished" || s.Reason != "resign" {
		t.Fatal(s)
	}
	result := h.People[0].Room()["result"].(map[string]any)
	if detail, ok := result["details"].(map[string]any); ok {
		if _, scored := detail["score"]; scored {
			t.Fatal("resignation contains an unconfirmed score", detail)
		}
	}
	record(t, "pass-resign", s)
}
func TestWeiqiHTTPConcurrentConfirmation(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "weiqi")
	h.Start(map[string]any{"size": 9})
	pass(h)
	s := pass(h)
	requests := []map[string]any{h.People[0].ActionRequest(action(s, "confirm_score")), h.People[1].ActionRequest(action(s, "confirm_score"))}
	url := "/api/rooms/" + h.People[0].Room()["id"].(string)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); h.People[i].Request("POST", url, requests[i], 200) }(i)
	}
	wg.Wait()
	s = state(h.People[0])
	if s.Phase != "finished" {
		t.Fatal(s)
	}
	record(t, "concurrent-score", s)
}
func TestWeiqiReclaimDeadlineIsolation(t *testing.T) {
	m := game.Module()
	v, e := m.Start(gameapi.Context{Players: 2, Now: time.Now()}, nil)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		a, _ := json.Marshal(action(s, "pass"))
		v, e = m.Apply(gameapi.Context{Players: 2, Actor: s.Turn, Now: time.Now()}, v, a)
		if e != nil {
			t.Fatal(e)
		}
	}
	var s game.State
	_ = json.Unmarshal(v.Data, &s)
	now := time.Now()
	s.Deadlines = [2]int64{now.Add(-time.Second).UnixMilli(), now.Add(-2 * time.Second).UnixMilli()}
	v.Data, _ = json.Marshal(s)
	v, e = m.Reclaim(gameapi.Context{Players: 2, Actor: 0, Now: now}, v)
	if e != nil {
		t.Fatal(e)
	}
	var after game.State
	_ = json.Unmarshal(v.Data, &after)
	if after.Deadlines[0] <= now.UnixMilli() || after.Deadlines[1] != s.Deadlines[1] {
		t.Fatal("reclaim changed other seat deadline", after)
	}
	record(t, "scoring-reclaim", after)
}
func TestWeiqiHTTPConnectedCaptureOnce(t *testing.T) {
	h := gametest.NewHTTP(t, 2, fixture(func(s *game.State) {
		for _, p := range []int{1, 9, 10} {
			s.Board[p] = 2
		}
		for _, p := range []int{2, 11, 18, 19} {
			s.Board[p] = 1
		}
	}), "weiqi")
	h.Start(map[string]any{"size": 9})
	s := put(h, 0, 0)
	if s.Captured[0] != 3 || len(s.Last.Removed) != 3 {
		t.Fatal("same group counted twice", s)
	}
	record(t, "connected-capture-midgame", s)
}
func TestWeiqiHTTPSharedLibertiesArea(t *testing.T) {
	h := gametest.NewHTTP(t, 2, fixture(func(s *game.State) {
		for y := 0; y < 9; y++ {
			for x := 0; x < 9; x++ {
				if x < 4 {
					s.Board[y*9+x] = 1
				}
				if x > 4 {
					s.Board[y*9+x] = 2
				}
			}
		}
		s.Board[10] = 0
		s.Board[70] = 0
	}), "weiqi")
	h.Start(map[string]any{"size": 9})
	pass(h)
	s := pass(h)
	if s.Score.Black != 36 || s.Score.White != 43.5 || s.Score.Neutral != 9 || s.Score.Territory != [2]int{1, 1} {
		t.Fatal("shared liberties or single-color eye wrong", s.Score)
	}
	h.People[0].Action(action(s, "confirm_score"), 200)
	h.People[1].Action(action(s, "confirm_score"), 200)
	s = state(h.People[0])
	if s.Score.Stones != [2]int{35, 35} {
		t.Fatal("automatic dead removal", s.Score)
	}
	record(t, "shared-liberties-area-midgame", s)
}
func TestWeiqiHTTPOlderPositionHistory(t *testing.T) {
	m := fixture(func(s *game.State) { s.Board[0] = 1; s.Board[80] = 2 })
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		older := append([]int(nil), s.Board...)
		older[40] = 1
		s.Keys = append([]string{game.PositionKey(older)}, s.Keys...)
		v.Data, _ = json.Marshal(s)
		return v, e
	}
	h := gametest.NewHTTP(t, 2, m, "weiqi")
	h.Start(map[string]any{"size": 9})
	s := state(h.People[0])
	a := action(s, "place")
	a["pos"] = 40
	h.People[0].Action(a, 400)
	h.Restart()
	h.People[0].Action(a, 400)
	if !reflect.DeepEqual(s, state(h.People[0])) {
		t.Fatal("rejected superko changed board")
	}
	put(h, 4, 5)
	record(t, "older-position-history-midgame", state(h.People[0]))
}
