package junqi_test

// Failure cases: hidden enemy ranks affect proposals, computers cannot deploy
// or finish, restore loses state, pending deployment seats share deadlines.
import (
	"encoding/json"
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/junqi/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestJunqiBotHiddenRanks(t *testing.T) {
	m := game.Module()
	v, e := m.Start(gameapi.Context{Players: 4, Now: time.Now()}, nil)
	if e != nil {
		t.Fatal(e)
	}
	var s game.State
	_ = json.Unmarshal(v.Data, &s)
	s.Phase = "playing"
	for i := range s.Ready {
		s.Ready[i] = true
	}
	s.Turn = 0
	s.Keys = []string{game.PositionKey(&s)}
	v.Data, _ = json.Marshal(s)
	ctx := gameapi.Context{Players: 4, Actor: 0, Now: time.Now()}
	a, e := m.BotAction(ctx, v)
	if e != nil {
		t.Fatal(e)
	}
	viewA, e := m.View(v, 0)
	if e != nil {
		t.Fatal(e)
	}
	first, second := -1, -1
	for i, p := range s.Pieces {
		if p.Seat == 1 && p.Kind == "commander" {
			first = i
		}
		if p.Seat == 1 && p.Kind == "company" {
			second = i
		}
	}
	s.Pieces[first].Kind, s.Pieces[second].Kind = s.Pieces[second].Kind, s.Pieces[first].Kind
	s.Keys = []string{game.PositionKey(&s)}
	v.Data, _ = json.Marshal(s)
	b, e := m.BotAction(ctx, v)
	if e != nil {
		t.Fatal(e)
	}
	viewB, e := m.View(v, 0)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(viewA, viewB) || string(a) != string(b) {
		t.Fatal("hidden ranks affected seat-visible state or strategy", string(a), string(b))
	}
	record(t, "bot-hidden-rank-invariance", map[string]any{"view": viewA, "proposal": json.RawMessage(a)})
}
func TestJunqiBotCompleteGamesHTTP(t *testing.T) {
	for _, n := range []int{2, 4} {
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
				h := gametest.NewHTTP(t, n, m, "junqi")
				h.Start(map[string]any{"difficulty": difficulty})
				trace := []any{}
				for steps := 0; steps < 1800; steps++ {
					v := state(h.People[0])
					if v.Phase == "finished" {
						break
					}
					actor := v.Turn
					if v.Phase == "deploy" {
						actor = 0
						for i, p := range v.Players {
							if !p.Ready {
								actor = i
								break
							}
						}
					}
					mu.Lock()
					snapshot := last
					mu.Unlock()
					proposal, e := m.BotAction(gameapi.Context{Actor: actor, Players: n, Now: time.Now()}, snapshot)
					if e != nil {
						t.Fatal(e)
					}
					var a map[string]any
					if e = json.Unmarshal(proposal, &a); e != nil {
						t.Fatal("idle strategy", e)
					}
					h.People[actor].Action(a, 200)
					trace = append(trace, a)
					if steps == n+5 {
						h.Restart()
					}
				}
				end := state(h.People[0])
				record(t, fmt.Sprintf("full-strategy-%d-%s", n, difficulty), map[string]any{"scope": "production seat-projected strategy through human HTTP sessions; native scheduler separate", "trace": trace, "final": end})
				if end.Phase != "finished" || end.Ply < 1 {
					t.Fatal("strategy did not finish", end.Ply, end.Phase)
				}
			})
		}
	}
}

func TestJunqiBotDefendsVisibleFlagThreatHTTP(t *testing.T) {
	p := append(flags(2), piece("defender", 0, "general", game.Home(0, 4, 0)), piece("raider", 0, "commander", game.Home(1, 0, 0)), piece("threat", 1, "engineer", game.Home(0, 4, 1)), piece("bait", 1, "platoon", game.Home(1, 0, 1)))
	for i := range p {
		if p[i].ID == "threat" {
			p[i].Revealed = true
		}
	}
	m := battle(2, p)
	var snapshot gameapi.Match
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		snapshot = v
		return v, e
	}
	h := gametest.NewHTTP(t, 2, m, "junqi")
	h.Start(map[string]any{"difficulty": "standard"})
	proposal, e := m.BotAction(gameapi.Context{Actor: 0, Players: 2, Now: time.Now()}, snapshot)
	if e != nil {
		t.Fatal(e)
	}
	var a map[string]any
	_ = json.Unmarshal(proposal, &a)
	if a["unit"] != "defender" || a["to"] != float64(game.Home(0, 4, 1)) {
		t.Fatal("ignored immediate flag threat for distant material", a)
	}
	h.People[0].Action(a, 200)
	v := state(h.People[0])
	for _, u := range v.Pieces {
		if u.ID == "threat" {
			t.Fatal("threat survived")
		}
	}
	record(t, "bot-defends-flag", v)
}

func TestJunqiAutoPlayDeploymentReclaimHTTP(t *testing.T) {
	m := game.Module()
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		s.Deadlines[0] = c.Now.Add(100 * time.Millisecond).UnixMilli()
		s.Deadlines[1] = c.Now.Add(60 * time.Second).UnixMilli()
		v.Deadline = s.Deadlines[0]
		v.TimeoutActors = []int{0}
		v.Data, _ = json.Marshal(s)
		return v, e
	}
	// Keep the overdue decision pending to inspect reclaim of only this seat.
	m.BotAction = func(gameapi.Context, gameapi.Match) (json.RawMessage, error) { return nil, nil }
	h := gametest.NewHTTP(t, 2, m, "junqi")
	h.Start(map[string]any{})
	before := state(h.People[0])
	h.People[0].Command("autoplay", map[string]any{"enabled": true}, 200)
	// Explicitly controlled but idle fixture: keep the overdue setup pending.
	for time.Now().UnixMilli() <= before.Deadlines[0] {
		time.Sleep(20 * time.Millisecond)
	}
	h.People[0].Command("autoplay", map[string]any{"enabled": false}, 200)
	v := state(h.People[0])
	if v.Deadlines[0] <= before.Deadlines[0] || v.Deadlines[1] != before.Deadlines[1] || v.Phase != "deploy" || v.Revision != before.Revision {
		t.Fatal("reclaim reset another deployment deadline or formation")
	}
	h.Restart()
	if !reflect.DeepEqual(v, state(h.People[0])) {
		t.Fatal("reclaim not preserved")
	}
	record(t, "deployment-reclaim", v)
}
func TestJunqiAutoPlayCompleteHTTP(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			h := gametest.NewHTTP(t, 1, game.Module(), "junqi")
			a := h.People[0]
			a.Request("POST", "/api/admin/games/junqi", map[string]bool{"enabled": true}, 200)
			a.Request("POST", "/api/rooms", map[string]any{"kind": "junqi", "capacity": n, "name": "原生电脑军棋完整局"}, 201)
			for i := 1; i < n; i++ {
				a.Command("add_bot", nil, 200)
			}
			a.Command("ready", nil, 200)
			a.Command("start", nil, 200)
			a.Command("autoplay", map[string]any{"enabled": true}, 200)
			initial := state(a)
			started := time.Now()
			last := 0
			restarted := false
			trace := []game.Event{}
			var end game.View
			for until := time.Now().Add(20 * time.Minute); time.Now().Before(until); {
				end = state(a)
				if end.Round != initial.Round || end.Ply < last {
					t.Fatal("native match reset")
				}
				if end.Ply > last {
					trace = append(trace, end.Last)
					last = end.Ply
				}
				if end.Phase == "finished" {
					break
				}
				if end.Ply >= 4 && !restarted {
					h.Restart()
					restarted = true
				}
				time.Sleep(60 * time.Millisecond)
			}
			record(t, fmt.Sprintf("native-complete-%d", n), map[string]any{"scope": "production scheduler, one autoplay human with real computer seats, real deployment and SQLite restore", "elapsedSeconds": time.Since(started).Seconds(), "trace": trace, "final": end})
			if end.Phase != "finished" || end.Ply < 4 || !restarted {
				t.Fatal("native computers did not finish through restore", end.Ply, end.Phase)
			}
			if a.Room()["result"].(map[string]any)["rated"] != false {
				t.Fatal("computer match rated")
			}
		})
	}
}

func TestJunqiTimeoutDeploymentHTTP(t *testing.T) {
	m := game.Module()
	start := m.Start
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		v, e := start(c, o)
		var s game.State
		_ = json.Unmarshal(v.Data, &s)
		s.Deadlines[0] = c.Now.Add(100 * time.Millisecond).UnixMilli()
		s.Deadlines[1] = c.Now.Add(60 * time.Second).UnixMilli()
		v.Deadline = s.Deadlines[0]
		v.TimeoutActors = []int{0}
		v.Data, _ = json.Marshal(s)
		return v, e
	}
	h := gametest.NewHTTP(t, 2, m, "junqi")
	h.Start(map[string]any{})
	otherDeadline := state(h.People[1]).Deadlines[1]
	for until := time.Now().Add(5 * time.Second); time.Now().Before(until) && !state(h.People[0]).Players[0].Ready; {
		time.Sleep(20 * time.Millisecond)
	}
	v := state(h.People[0])
	if !v.Players[0].Ready || v.Players[1].Ready || v.Phase != "deploy" || v.Deadlines[1] != otherDeadline {
		t.Fatal("timeout did not confirm only overdue formation", v.Players)
	}
	h.People[0].Command("autoplay", map[string]any{"enabled": false}, 200)
	if after := state(h.People[0]); !reflect.DeepEqual(v, after) {
		t.Fatal("reclaiming confirmed setup changed another seat")
	}
	record(t, "native-timeout-deployment", v)
}
