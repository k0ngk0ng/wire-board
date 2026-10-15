package weiqi_test

// Before strategy implementation: illegal moves, true-eye filling, failure to
// capture/save, cancelled or mutating search, surrendering live stones during
// scoring, stalled natural games and restart/hand-over corruption.
import (
	"context"
	"encoding/json"
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/weiqi/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestWeiqiBotTacticsAndCancellation(t *testing.T) {
	for _, difficulty := range []string{"easy", "standard"} {
		m := fixture(func(s *game.State) {
			for _, p := range []int{0, 2, 12, 18, 20, 28} {
				s.Board[p] = 1
			}
			for _, p := range []int{1, 9, 11, 19} {
				s.Board[p] = 2
			}
		})
		v, e := m.Start(gameapi.Context{Players: 2, Now: time.Now()}, json.RawMessage(fmt.Sprintf(`{"size":9,"difficulty":%q}`, difficulty)))
		if e != nil {
			t.Fatal(e)
		}
		before := append([]byte(nil), v.Data...)
		c := gameapi.Context{Players: 2, Actor: 0, Now: time.Now()}
		raw, e := m.BotAction(c, v)
		if e != nil {
			t.Fatal(e)
		}
		var a game.Action
		_ = json.Unmarshal(raw, &a)
		if a.Type != "place" || a.Pos == nil || *a.Pos != 10 {
			t.Fatal("missed capture", string(raw))
		}
		if !reflect.DeepEqual(before, []byte(v.Data)) {
			t.Fatal("bot mutated input")
		}
		input, e := m.BotSearch.Input(c, v)
		if e != nil {
			t.Fatal(e)
		}
		saved := append([]byte(nil), input...)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		begin := time.Now()
		_, _ = m.BotSearch.Think(ctx, input)
		if time.Since(begin) > 100*time.Millisecond {
			t.Fatal("cancellation too slow")
		}
		ctx, cancel = context.WithTimeout(context.Background(), m.BotSearch.Budget)
		raw, e = m.BotSearch.Think(ctx, input)
		cancel()
		if e != nil {
			t.Fatal(e)
		}
		_ = json.Unmarshal(raw, &a)
		if a.Type != "place" || a.Pos == nil || *a.Pos != 10 {
			t.Fatal("search missed capture", string(raw))
		}
		if !reflect.DeepEqual(saved, []byte(input)) {
			t.Fatal("search mutated projection")
		}
		record(t, "bot-tactics-"+difficulty, map[string]any{"proposal": json.RawMessage(raw), "budget": m.BotSearch.Budget.String()})
	}
}
func TestWeiqiBotScoringDisputeHTTP(t *testing.T) {
	m := game.Module()
	var last gameapi.Match
	var mu sync.Mutex
	start, apply := m.Start, m.Apply
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		mu.Lock()
		last = v
		mu.Unlock()
		return v, e
	}
	m.Apply = func(c gameapi.Context, v gameapi.Match, a json.RawMessage) (gameapi.Match, error) {
		next, e := apply(c, v, a)
		if e == nil {
			mu.Lock()
			last = next
			mu.Unlock()
		}
		return next, e
	}
	h := gametest.NewHTTP(t, 2, m, "weiqi")
	h.Start(map[string]any{"size": 9})
	s := put(h, 4, 4)
	black := s.Black
	pass(h)
	s = pass(h)
	s = mark(h, 1-black, 40)
	mu.Lock()
	snapshot := last
	mu.Unlock()
	raw, e := m.BotAction(gameapi.Context{Actor: black, Players: 2, Now: time.Now()}, snapshot)
	if e != nil {
		t.Fatal(e)
	}
	var a map[string]any
	_ = json.Unmarshal(raw, &a)
	if a["type"] != "resume" {
		t.Fatal("bot accepted live own stones marked dead", a)
	}
	h.People[black].Action(a, 200)
	s = state(h.People[0])
	if s.Phase != "playing" || s.Board[40] != 1 {
		t.Fatal(s)
	}
	record(t, "bot-score-dispute", s)
}
func TestWeiqiBotCompleteGamesHTTP(t *testing.T)  { completeHTTP(t, 7.5) }
func TestWeiqiBotNoKomiCompleteHTTP(t *testing.T) { completeHTTP(t, 0) }
func completeHTTP(t *testing.T, komi float64) {
	for _, n := range []int{9, 13, 19} {
		for _, difficulty := range []string{"easy", "standard"} {
			t.Run(fmt.Sprintf("%d-%s", n, difficulty), func(t *testing.T) {
				m := game.Module()
				var last gameapi.Match
				var mu sync.Mutex
				start, apply := m.Start, m.Apply
				m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
					v, e := start(c, o)
					mu.Lock()
					last = v
					mu.Unlock()
					return v, e
				}
				m.Apply = func(c gameapi.Context, v gameapi.Match, a json.RawMessage) (gameapi.Match, error) {
					next, e := apply(c, v, a)
					if e == nil {
						mu.Lock()
						last = next
						mu.Unlock()
					}
					return next, e
				}
				h := gametest.NewHTTP(t, 2, m, "weiqi")
				h.Start(map[string]any{"size": n, "difficulty": difficulty, "komi": komi})
				trace := []any{}
				restarted := false
				for steps := 0; steps < 6*n*n; steps++ {
					s := state(h.People[0])
					if s.Phase == "finished" {
						break
					}
					actor := s.Turn
					if s.Phase == "scoring" {
						actor = 0
						if s.Confirmed[0] {
							actor = 1
						}
					}
					mu.Lock()
					snapshot := last
					mu.Unlock()
					c := gameapi.Context{Actor: actor, Players: 2, Now: time.Now()}
					input, e := m.BotSearch.Input(c, snapshot)
					if e != nil {
						t.Fatal(e)
					}
					ctx, cancel := context.WithTimeout(context.Background(), m.BotSearch.Budget)
					raw, e := m.BotSearch.Think(ctx, input)
					cancel()
					if e != nil {
						t.Fatal(e)
					}
					var a map[string]any
					if e = json.Unmarshal(raw, &a); e != nil {
						t.Fatal("idle bot", e)
					}
					h.People[actor].Action(a, 200)
					trace = append(trace, a)
					if steps == 10 {
						before := state(h.People[0])
						h.Restart()
						if !reflect.DeepEqual(before, state(h.People[0])) {
							t.Fatal("restore changed board/history")
						}
						restarted = true
					}
				}
				s := state(h.People[0])
				name := fmt.Sprintf("complete-%d-%s", n, difficulty)
				if komi == 0 {
					name += "-komi0"
				}
				record(t, name, map[string]any{"trace": trace, "final": s, "nativeScheduler": false, "restarted": restarted})
				if s.Phase != "finished" || s.Reason != "score" || s.Ply < n || !restarted {
					t.Fatal("natural game failed", s.Phase, s.Reason, s.Ply)
				}
				h.Record("natural board, production strategy via HTTP, restart, double pass and confirmed score")
			})
		}
	}
}
func TestWeiqiAutoPlayCompleteHTTP(t *testing.T) {
	for _, n := range []int{9, 13, 19} {
		for _, difficulty := range []string{"easy", "standard"} {
			t.Run(fmt.Sprintf("%d-%s", n, difficulty), func(t *testing.T) {
				t.Parallel()
				h := gametest.NewHTTP(t, 1, game.Module(), "weiqi")
				a := h.People[0]
				a.Request("POST", "/api/admin/games/weiqi", map[string]bool{"enabled": true}, 200)
				a.Request("POST", "/api/rooms", map[string]any{"kind": "weiqi", "capacity": 2, "name": "原生围棋电脑验收", "options": map[string]any{"size": n, "difficulty": difficulty}}, 201)
				a.Command("add_bot", nil, 200)
				a.Command("ready", nil, 200)
				a.Command("start", nil, 200)
				a.Command("autoplay", map[string]any{"enabled": true}, 200)
				initial := state(a)
				started := time.Now()
				last := 0
				restarted, reclaimed := false, false
				trace := []game.Event{}
				var end game.State
				for until := time.Now().Add(20 * time.Minute); time.Now().Before(until); {
					end = state(a)
					if end.Round != initial.Round || end.Ply < last {
						t.Fatal("match reset")
					}
					if end.Last.ID > last {
						trace = append(trace, end.Last)
						last = end.Last.ID
					}
					if end.Phase == "finished" {
						break
					}
					if end.Ply >= 4 && !reclaimed {
						a.Command("autoplay", map[string]any{"enabled": false}, 200)
						for end = state(a); end.Turn != 0 && end.Phase == "playing"; end = state(a) {
							time.Sleep(30 * time.Millisecond)
						}
						if end.Phase != "playing" {
							t.Fatal("finished before handover")
						}
						before := end
						time.Sleep(250 * time.Millisecond)
						end = state(a)
						if !reflect.DeepEqual(before, end) {
							t.Fatal("cancelled own control still acted")
						}
						data := gametest.Data(a.Room())
						legal := data["legal"].([]any)
						if len(legal) == 0 {
							t.Fatal("no opening handover move")
						}
						request := action(end, "place")
						request["pos"] = legal[0]
						a.Action(request, 200)
						a.Command("autoplay", map[string]any{"enabled": true}, 200)
						reclaimed = true
					}
					if end.Ply >= 12 && !restarted {
						h.Restart()
						restarted = true
					}
					time.Sleep(70 * time.Millisecond)
				}
				record(t, fmt.Sprintf("native-complete-%d-%s", n, difficulty), map[string]any{"scope": "production scheduler, real computer and human autoplay, manual handover and SQLite restart", "elapsedSeconds": time.Since(started).Seconds(), "trace": trace, "final": end, "reclaimed": reclaimed, "restarted": restarted})
				if end.Phase != "finished" || end.Reason != "score" || !restarted || !reclaimed {
					t.Fatal("native game incomplete", end.Phase, end.Ply)
				}
				if a.Room()["result"].(map[string]any)["rated"] != false {
					t.Fatal("computer game rated")
				}
			})
		}
	}
}
func scoringFixture() gameapi.Module {
	m := game.Module()
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		s.Phase = "scoring"
		s.Passes = 2
		s.Ply = 2
		s.Revision = 1
		s.Deadline = 0
		s.Deadlines = [2]int64{c.Now.Add(500 * time.Millisecond).UnixMilli(), c.Now.Add(time.Minute).UnixMilli()}
		v.Data, _ = json.Marshal(s)
		v.Actors = []int{0, 1}
		v.Attention = []int{0, 1}
		v.TimeoutActors = []int{0}
		v.Deadline = s.Deadlines[0]
		return v, e
	}
	return m
}
func TestWeiqiTimeoutScoringHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 2, scoringFixture(), "weiqi")
	h.Start(map[string]any{"size": 9})
	before := state(h.People[0])
	for until := time.Now().Add(5 * time.Second); time.Now().Before(until) && !state(h.People[0]).Confirmed[0]; {
		time.Sleep(30 * time.Millisecond)
	}
	after := state(h.People[0])
	if !after.Confirmed[0] || after.Confirmed[1] || after.Deadlines != before.Deadlines || after.Phase != "scoring" {
		t.Fatal("timeout changed other confirmer", after)
	}
	h.People[0].Command("autoplay", map[string]any{"enabled": false}, 200)
	if !reflect.DeepEqual(after, state(h.People[0])) {
		t.Fatal("confirmed seat reclaim changed proposal")
	}
	h.People[1].Action(action(after, "confirm_score"), 200)
	record(t, "scoring-timeout-midgame", state(h.People[0]))
}
func TestWeiqiAutoPlayScoringReclaimHTTP(t *testing.T) {
	m := scoringFixture()
	m.BotSearch = nil
	m.BotAction = func(gameapi.Context, gameapi.Match) (json.RawMessage, error) { return nil, nil }
	h := gametest.NewHTTP(t, 2, m, "weiqi")
	h.Start(map[string]any{"size": 9})
	before := state(h.People[0])
	h.People[0].Command("autoplay", map[string]any{"enabled": true}, 200)
	for time.Now().UnixMilli() <= before.Deadlines[0] {
		time.Sleep(20 * time.Millisecond)
	}
	h.People[0].Command("autoplay", map[string]any{"enabled": false}, 200)
	after := state(h.People[0])
	if after.Deadlines[0] <= before.Deadlines[0] || after.Deadlines[1] != before.Deadlines[1] || !reflect.DeepEqual(before.Board, after.Board) || before.Revision != after.Revision {
		t.Fatal("reclaim changed other seat or position", after)
	}
	h.Restart()
	if !reflect.DeepEqual(after, state(h.People[0])) {
		t.Fatal("reclaim not restored")
	}
	record(t, "scoring-http-reclaim", after)
}
