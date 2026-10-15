package gomoku_test

// Failure cases specified before implementation: illegal/occupied/out-of-turn
// placement; stale round/ply or duplicate request; false edge/diagonal wins;
// long lines confused with exact-five; full-board win overwritten by draw;
// draw offers reset clocks or can be accepted after a move; surrender gives
// wrong winner; bots overlook an immediate win/block or place illegal stones;
// search mutates its input; autoplay/reset/restart corrupts the live board.
import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/gomoku/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func state(c *gametest.Client) game.State {
	raw, _ := json.Marshal(gametest.Data(c.Room()))
	var s game.State
	_ = json.Unmarshal(raw, &s)
	return s
}
func request(s game.State, kind string) map[string]any {
	return map[string]any{"type": kind, "round": s.Round, "ply": s.Ply}
}
func put(h *gametest.HTTP, x, y int) game.State {
	s := state(h.People[0])
	a := request(s, "place")
	a["x"], a["y"] = x, y
	h.People[s.Turn].Action(a, 200)
	return state(h.People[0])
}
func record(t *testing.T, kind string, value any) {
	t.Helper()
	p := filepath.Join("..", "..", "..", ".local", "gomoku", "http")
	if e := os.MkdirAll(p, 0755); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "kind": kind, "evidence": value}, "", "  ")
	if e := os.WriteFile(filepath.Join(p, kind+".json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
}

func TestGomokuHTTPRulesAndRestore(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "gomoku")
	h.AssertUnpublished()
	h.Start(map[string]any{})
	s := state(h.People[0])
	if len(s.Board) != 225 || s.Ply != 0 || s.Turn != s.Black || s.Options.Rules != "freestyle" {
		t.Fatal("bad natural setup", s)
	}
	a := request(s, "place")
	a["x"], a["y"] = 7, 7
	h.People[1-s.Turn].Action(a, 400)
	h.Spectator.Action(a, 400)
	for _, coord := range [][2]int{{-1, 0}, {15, 0}, {0, 15}} {
		bad := request(s, "place")
		bad["x"], bad["y"] = coord[0], coord[1]
		h.People[s.Turn].Action(bad, 400)
	}
	h.People[s.Turn].Action(request(s, "place"), 400)
	bad := request(s, "place")
	bad["x"], bad["y"], bad["round"] = 7, 7, "previous-round"
	h.People[s.Turn].Action(bad, 400)
	old := s
	payload := h.People[s.Turn].ActionRequest(a)
	room := h.People[0].Room()
	url := "/api/rooms/" + room["id"].(string)
	h.People[s.Turn].Request("POST", url, payload, 200)
	h.People[s.Turn].Request("POST", url, payload, 200)
	s = state(h.People[0])
	if s.Ply != 1 || s.Board[7*15+7] != 1 {
		t.Fatal("replay duplicated placement", s)
	}
	a = request(s, "place")
	a["x"], a["y"] = 7, 7
	h.People[s.Turn].Action(a, 400)
	a = request(old, "place")
	a["x"], a["y"] = 8, 8
	h.People[s.Turn].Action(a, 400)
	before := state(h.People[0])
	h.Restart()
	after := state(h.People[0])
	if !reflect.DeepEqual(before, after) {
		t.Fatal("restore reset board or deadline")
	}
	put(h, 8, 8)
	h.Record("legal placements, rejected errors and restart")
	record(t, "lifecycle", state(h.People[0]))
}

func TestGomokuHTTPWins(t *testing.T) {
	for _, d := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
		t.Run(fmt.Sprint(d), func(t *testing.T) {
			h := gametest.NewHTTP(t, 2, game.Module(), "gomoku")
			h.Start(map[string]any{})
			black := state(h.People[0]).Black
			for i := 0; i < 5; i++ {
				put(h, 4+i*d[0], 7+i*d[1])
				if i < 4 {
					put(h, 2*i, 14)
				}
			}
			s := state(h.People[0])
			if s.Phase != "finished" || s.Winner != black || s.Reason != "five" || len(s.Line) < 5 {
				t.Fatal("line not won", s)
			}
			r := h.People[0].Room()
			if r["status"] != "finished" || len(r["game"].(map[string]any)["winners"].([]any)) != 1 {
				t.Fatal(r)
			}
			h.Record("natural five-in-a-row win")
			record(t, fmt.Sprintf("line-%d-%d", d[0], d[1]), s)
		})
	}
}

func fixture(change func(*game.State)) gameapi.Module {
	m := game.Module()
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		match, e := start(c, o)
		if e != nil {
			return match, e
		}
		var s game.State
		_ = json.Unmarshal(match.Data, &s)
		change(&s)
		match.Data, _ = json.Marshal(s)
		match.Attention = []int{s.Turn}
		match.TimeoutActors = []int{s.Turn}
		return match, nil
	}
	return m
}

func TestGomokuHTTPLongLineAndBoardDraw(t *testing.T) {
	for _, rules := range []string{"freestyle", "exact-five"} {
		t.Run(rules, func(t *testing.T) {
			m := fixture(func(s *game.State) {
				s.Black, s.Turn, s.Ply = 0, 0, 10
				for _, x := range []int{0, 1, 2, 4, 5} {
					s.Board[7*15+x] = 1
				}
				for _, x := range []int{0, 2, 4, 6, 8} {
					s.Board[x] = 2
				}
				s.Last = game.Event{ID: 10, Seat: 1, X: 8, Y: 0}
			})
			h := gametest.NewHTTP(t, 2, m, "gomoku")
			h.Start(map[string]any{"rules": rules})
			s := put(h, 3, 7)
			if (s.Phase == "finished") != (rules == "freestyle") {
				t.Fatal("long-line rule mismatch", s)
			}
			record(t, "long-line-"+rules, s)
		})
	}
	m := fixture(func(s *game.State) {
		s.Black, s.Turn, s.Ply = 0, 0, 224
		for y := 0; y < 15; y++ {
			for x := 0; x < 15; x++ {
				v := 2
				if (x+2*y)%4 < 2 {
					v = 1
				}
				s.Board[y*15+x] = v
			}
		}
		s.Board[0] = 0
		s.Last = game.Event{ID: 224, Seat: 1, X: 14, Y: 14}
	})
	h := gametest.NewHTTP(t, 2, m, "gomoku")
	h.Start(map[string]any{})
	s := put(h, 0, 0)
	if s.Phase != "finished" || s.Reason != "board_full" || s.Winner != -1 || h.People[0].Room()["result"].(map[string]any)["draw"] != true {
		t.Fatal("full board not a draw", s)
	}
	h.Restart()
	record(t, "full-board-draw-fixture", state(h.People[0]))
}

func TestGomokuHTTPDrawOfferAndResign(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "gomoku")
	h.Start(map[string]any{})
	put(h, 7, 7)
	s := put(h, 8, 8)
	deadline := s.Deadline
	h.People[s.Turn].Action(request(s, "offer_draw"), 200)
	s = state(h.People[0])
	if s.Deadline != deadline {
		t.Fatal("offer renewed turn clock")
	}
	a := request(s, "respond_draw")
	a["accept"] = false
	h.People[1-s.Turn].Action(a, 200)
	s = state(h.People[0])
	h.People[s.Turn].Action(request(s, "offer_draw"), 400)
	s = put(h, 9, 9)
	h.People[s.Turn].Action(request(s, "offer_draw"), 200)
	s = state(h.People[0])
	a = request(s, "respond_draw")
	a["accept"] = true
	h.People[1-s.Turn].Action(a, 200)
	s = state(h.People[0])
	if s.Reason != "agreement" || s.Winner != -1 {
		t.Fatal(s)
	}
	record(t, "agreed-draw", s)
	h.People[0].Command("rematch", nil, 200)
	for _, c := range h.People {
		c.Command("ready", nil, 200)
	}
	h.People[0].Command("start", nil, 200)
	s = state(h.People[0])
	if s.Ply != 0 {
		t.Fatal("rematch reused stones")
	}
	h.People[1-s.Turn].Action(request(s, "resign"), 200)
	end := state(h.People[0])
	if end.Reason != "resign" || end.Winner != s.Turn {
		t.Fatal("wrong resignation winner", end)
	}
	record(t, "resign", end)
}

func TestGomokuBotTacticsHTTP(t *testing.T) {
	for _, kind := range []string{"win", "block"} {
		t.Run(kind, func(t *testing.T) {
			m := fixture(func(s *game.State) {
				s.Black, s.Turn, s.Ply = 0, 0, 8
				color := 1
				if kind == "block" {
					color = 2
				}
				for x := 1; x <= 4; x++ {
					s.Board[7*15+x] = color
				}
				s.Board[7*15] = 3 - color
				for _, x := range []int{0, 2, 4} {
					s.Board[x] = 3 - color
				}
				s.Last = game.Event{ID: 8, Seat: 1, X: 4, Y: 0}
			})
			h := gametest.NewHTTP(t, 2, m, "gomoku")
			h.Start(map[string]any{})
			r := h.People[0].Room()
			v := state(h.People[0])
			raw, _ := json.Marshal(v)
			match := gameapi.Match{Schema: 1, Data: raw, Actors: []int{0, 1}, TimeoutActors: []int{0}, Deadline: v.Deadline}
			before := append([]byte(nil), raw...)
			input, e := m.BotSearch.Input(gameapi.Context{Actor: 0, Players: 2, Now: time.Now()}, match)
			if e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			proposal, e := m.BotSearch.Think(ctx, input)
			cancel()
			if e != nil {
				t.Fatal(e)
			}
			var a map[string]any
			_ = json.Unmarshal(proposal, &a)
			if a["x"] != float64(5) || a["y"] != float64(7) {
				t.Fatal("missed immediate tactic", kind, a)
			}
			if !reflect.DeepEqual(before, []byte(match.Data)) {
				t.Fatal("search mutated authoritative state")
			}
			h.People[0].Action(a, 200)
			record(t, "bot-"+kind, map[string]any{"before": r, "proposal": a, "after": h.People[0].Room()})
		})
	}
}

func TestGomokuBotCompleteGamesHTTP(t *testing.T) {
	for _, rules := range []string{"freestyle", "exact-five"} {
		for _, difficulty := range []string{"easy", "standard"} {
			t.Run(rules+"-"+difficulty, func(t *testing.T) {
				m := game.Module()
				h := gametest.NewHTTP(t, 2, m, "gomoku")
				h.Start(map[string]any{"rules": rules, "difficulty": difficulty})
				trace := []any{}
				for steps := 0; steps < 225; steps++ {
					v := state(h.People[0])
					if v.Phase == "finished" {
						break
					}
					raw, _ := json.Marshal(v)
					match := gameapi.Match{Schema: 1, Data: raw, Actors: []int{0, 1}, Deadline: v.Deadline, TimeoutActors: []int{v.Turn}}
					input, e := m.BotSearch.Input(gameapi.Context{Actor: v.Turn, Players: 2, Now: time.Now()}, match)
					if e != nil {
						t.Fatal(e)
					}
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					proposal, e := m.BotSearch.Think(ctx, input)
					cancel()
					if e != nil {
						t.Fatal(e)
					}
					var action map[string]any
					if e = json.Unmarshal(proposal, &action); e != nil {
						t.Fatal(e)
					}
					h.People[v.Turn].Action(action, 200)
					trace = append(trace, action)
					if steps == 5 {
						h.Restart()
					}
				}
				end := state(h.People[0])
				if end.Phase != "finished" || end.Ply < 9 || end.Ply > 225 {
					t.Fatal("strategies did not complete a natural game", end)
				}
				record(t, "full-strategy-http-"+rules+"-"+difficulty, map[string]any{"scope": "Both production strategies submit through real human-session HTTP; native scheduling tested separately", "trace": trace, "final": end})
			})
		}
	}
}

func TestGomokuAutoPlayAndReclaimHTTP(t *testing.T) {
	m := fixture(func(s *game.State) { s.Deadline = time.Now().Add(100 * time.Millisecond).UnixMilli() })
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		v.Deadline = s.Deadline
		return v, e
	}
	h := gametest.NewHTTP(t, 2, m, "gomoku")
	h.Start(map[string]any{})
	first := state(h.People[0]).Turn
	for until := time.Now().Add(4 * time.Second); time.Now().Before(until) && state(h.People[0]).Ply == 0; {
		time.Sleep(20 * time.Millisecond)
	}
	v := state(h.People[0])
	if v.Ply != 1 {
		t.Fatal("timeout computer failed to move", v)
	}
	deadline := v.Deadline
	h.People[first].Command("autoplay", map[string]any{"enabled": false}, 200)
	after := state(h.People[0])
	if after.Deadline != deadline || !reflect.DeepEqual(v.Board, after.Board) {
		t.Fatal("reclaim changed another seat's clock or board")
	}
	h.Restart()
	h.Record("timeout autoplay, manual takeover and restore")
	record(t, "autoplay-reclaim", after)
}

// Natural production timing, real computer seat plus an autoplay human seat.
// No direct state writes, forced winner or test-only scheduler acceleration.
func TestGomokuAutoPlayCompleteHTTP(t *testing.T) {
	for _, rules := range []string{"freestyle", "exact-five"} {
		t.Run(rules, func(t *testing.T) {
			h := gametest.NewHTTP(t, 1, game.Module(), "gomoku")
			a := h.People[0]
			a.Request("POST", "/api/admin/games/gomoku", map[string]bool{"enabled": true}, 200)
			a.Request("POST", "/api/rooms", map[string]any{"kind": "gomoku", "name": "原生电脑完整局", "capacity": 2, "options": map[string]any{"rules": rules, "difficulty": "standard"}}, 201)
			for _, cmd := range []string{"add_bot", "ready", "start"} {
				a.Command(cmd, nil, 200)
			}
			a.Command("autoplay", map[string]any{"enabled": true}, 200)
			initial := state(a)
			started := time.Now()
			trace := []game.Event{}
			lastPly := 0
			restarted := false
			var end game.State
			for until := time.Now().Add(7 * time.Minute); time.Now().Before(until); {
				end = state(a)
				if end.Round != initial.Round || end.Ply < lastPly {
					t.Fatal("native play reset the match")
				}
				if end.Ply > lastPly {
					trace = append(trace, end.Last)
					lastPly = end.Ply
				}
				if end.Phase == "finished" {
					break
				}
				if !restarted && end.Ply >= 4 {
					h.Restart()
					restarted = true
				}
				time.Sleep(30 * time.Millisecond)
			}
			if end.Phase != "finished" || end.Ply < 9 || end.Ply > 225 {
				t.Fatal("native computers did not finish", end)
			}
			if a.Room()["result"].(map[string]any)["rated"] != false {
				t.Fatal("computer match awarded rating")
			}
			h.Record("native computer/autoplay complete game with restore")
			record(t, "native-complete-"+rules, map[string]any{"scope": "production bot scheduler, default think budget, real SQLite restore", "elapsedSeconds": time.Since(started).Seconds(), "trace": trace, "final": end})
		})
	}
}
