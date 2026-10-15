package chess_test

// Failure inventory before further strategy changes: incomplete games, illegal
// proposals, search mutates input, cancellation ignored, clock/control lost on
// restore, reclaim touches another seat, computer results accidentally rated.
import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/chess/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestChessBotCompleteGamesHTTP(t *testing.T) {
	for _, difficulty := range []string{"easy", "standard"} {
		t.Run(difficulty, func(t *testing.T) {
			m := game.Module()
			h := gametest.NewHTTP(t, 2, m, "chess")
			h.Start(map[string]any{"difficulty": difficulty})
			trace := []any{}
			for steps := 0; steps < 700; steps++ {
				v := state(h.People[0])
				if v.Phase == "finished" {
					break
				}
				raw, _ := json.Marshal(v.State)
				match := gameapi.Match{Schema: 1, Data: raw, Actors: []int{0, 1}, Deadline: v.Deadline, TimeoutActors: []int{v.Turn}}
				input, e := m.BotSearch.Input(gameapi.Context{Actor: v.Turn, Players: 2, Now: time.Now()}, match)
				if e != nil {
					t.Fatal(e)
				}
				before := append([]byte(nil), input...)
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				proposal, e := m.BotSearch.Think(ctx, input)
				cancel()
				if e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(before, []byte(input)) {
					t.Fatal("search mutated input")
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
			record(t, "full-strategy-http-"+difficulty, map[string]any{"scope": "production strategies through real human-session HTTP; native scheduler tested separately", "trace": trace, "final": end})
			if end.Phase != "finished" || end.Ply < 4 {
				t.Fatal("strategies did not finish", end.FEN, end.Ply)
			}
		})
	}
}

func TestChessBotCancellation(t *testing.T) {
	m := game.Module()
	match, e := m.Start(gameapi.Context{Players: 2, Now: time.Now()}, nil)
	if e != nil {
		t.Fatal(e)
	}
	var s game.State
	_ = json.Unmarshal(match.Data, &s)
	in, e := m.BotSearch.Input(gameapi.Context{Actor: s.Turn, Players: 2}, match)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	proposal, e := m.BotSearch.Think(ctx, in)
	if e == nil || len(proposal) > 0 || time.Since(started) > 100*time.Millisecond {
		t.Fatal("cancelled search did work", e)
	}
	record(t, "cancelled-search", map[string]any{"elapsed": time.Since(started).String(), "error": e.Error()})
}

func TestChessAutoPlayAndReclaimHTTP(t *testing.T) {
	m := game.Module()
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		s.Deadline = time.Now().Add(100 * time.Millisecond).UnixMilli()
		v.Deadline = s.Deadline
		v.Data, _ = json.Marshal(s)
		return v, e
	}
	h := gametest.NewHTTP(t, 2, m, "chess")
	h.Start(map[string]any{})
	first := state(h.People[0]).Turn
	for until := time.Now().Add(5 * time.Second); time.Now().Before(until) && state(h.People[0]).Ply == 0; {
		time.Sleep(20 * time.Millisecond)
	}
	v := state(h.People[0])
	if v.Ply != 1 {
		t.Fatal("timeout failed to move", v.Ply)
	}
	h.People[first].Command("autoplay", map[string]any{"enabled": false}, 200)
	after := state(h.People[0])
	if after.Deadline != v.Deadline || after.FEN != v.FEN {
		t.Fatal("reclaim changed other turn")
	}
	h.Restart()
	if !reflect.DeepEqual(after, state(h.People[0])) {
		t.Fatal("restore changed reclaimed match")
	}
	record(t, "autoplay-reclaim", after)
}

func TestChessAutoPlayCompleteHTTP(t *testing.T) {
	for _, difficulty := range []string{"easy", "standard"} {
		t.Run(difficulty, func(t *testing.T) {
			h := gametest.NewHTTP(t, 1, game.Module(), "chess")
			a := h.People[0]
			a.Request("POST", "/api/admin/games/chess", map[string]bool{"enabled": true}, 200)
			a.Request("POST", "/api/rooms", map[string]any{"kind": "chess", "name": "原生电脑完整局", "capacity": 2, "options": map[string]any{"difficulty": difficulty}}, 201)
			for _, cmd := range []string{"add_bot", "ready", "start"} {
				a.Command(cmd, nil, 200)
			}
			a.Command("autoplay", map[string]any{"enabled": true}, 200)
			initial := state(a)
			started := time.Now()
			trace := []game.Event{}
			lastPly := 0
			restarted := false
			var end game.View
			for until := time.Now().Add(20 * time.Minute); time.Now().Before(until); {
				end = state(a)
				if end.Round != initial.Round || end.Ply < lastPly {
					t.Fatal("native match reset")
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
				time.Sleep(100 * time.Millisecond)
			}
			record(t, "native-complete-"+difficulty, map[string]any{"scope": "production scheduler and think budget; real computer plus autoplay human; SQLite restart", "elapsedSeconds": time.Since(started).Seconds(), "trace": trace, "final": end})
			if end.Phase != "finished" || end.Ply < 4 {
				t.Fatal("native computers did not finish", end.FEN, end.Ply)
			}
			if a.Room()["result"].(map[string]any)["rated"] != false {
				t.Fatal("computer game rated")
			}
		})
	}
}

// Concurrent detached searches must not share the engine's mutable positions
// or return a proposal for another room; each result still goes through HTTP.
func TestChessConcurrentSearchHTTP(t *testing.T) {
	m := game.Module()
	type work struct {
		h   *gametest.HTTP
		v   game.View
		raw json.RawMessage
	}
	jobs := make([]work, 3)
	for i := range jobs {
		h := gametest.NewHTTP(t, 2, m, "chess")
		h.Start(map[string]any{"difficulty": "standard"})
		v := state(h.People[0])
		raw, _ := json.Marshal(v.State)
		jobs[i] = work{h: h, v: v, raw: raw}
	}
	type answer struct {
		index    int
		proposal json.RawMessage
		err      error
	}
	out := make(chan answer, len(jobs))
	for i, j := range jobs {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			proposal, e := m.BotSearch.Think(ctx, j.raw)
			out <- answer{i, proposal, e}
		}()
	}
	for range jobs {
		a := <-out
		if a.err != nil {
			t.Fatal(a.err)
		}
		j := jobs[a.index]
		var action map[string]any
		if e := json.Unmarshal(a.proposal, &action); e != nil {
			t.Fatal(e)
		}
		if action["round"] != j.v.Round {
			t.Fatal("cross-room proposal")
		}
		j.h.People[j.v.Turn].Action(action, 200)
		end := state(j.h.People[0])
		if end.Ply != 1 {
			t.Fatal("concurrent proposal not applied")
		}
		original, _ := json.Marshal(j.v.State)
		if !reflect.DeepEqual(original, []byte(j.raw)) {
			t.Fatal("shared snapshot mutation")
		}
		record(t, fmt.Sprintf("concurrent-room-%d", a.index), map[string]any{"action": action, "after": end})
	}
}
