package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Public Explorer configuration is still gated. Provision an exact private
// constructor snapshot after normal registration/join/ready, without running
// base-game setup or injecting resources/points into the natural matches.
func newExplorerHTTP(t *testing.T, n int, discard bool) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("探险玩家%d", p))
	}
	raw := clients[0].post("/api/rooms", map[string]any{"name": "初航网络验证", "kind": "catan", "capacity": max(3, n)}, 201)
	id := raw["id"].(string)
	for p := 1; p < n; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for p := 0; p < n; p++ {
		clients[p].command(current(clients[p]), "ready", nil, 200)
	}
	path := fmt.Sprintf("testdata/catan_explorer_initial_%d.json", n)
	if discard {
		path = "testdata/catan_explorer_discard.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state game.State
	if err = json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if state.Catan.Explorer == nil || len(state.Catan.Players) != n || (!discard && (state.Phase != "catan_roll" || state.Catan.TurnSerial != 1 || state.Catan.RollID != 0)) {
		t.Fatal("invalid initial fixture")
	}
	s.mu.Lock()
	r := s.rooms[id]
	r.Game, r.Status, r.Capacity = &state, "playing", n
	r.startTurnClock(time.Now())
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	return s, ts, clients, id
}

func explorerHTTPActor(g *game.State) int {
	if g.Phase == "catan_discard" {
		for p, due := range g.Catan.DiscardDue {
			if due > 0 {
				return p
			}
		}
	}
	return g.Turn
}

func assertExplorerHTTPPrivacy(t *testing.T, clients []*testClient) {
	t.Helper()
	for viewer, c := range clients {
		v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
		x := v["explorer"].(map[string]any)
		if viewer != int(current(c)["game"].(map[string]any)["turn"].(float64)) && len(x["choices"].([]any)) != 0 {
			t.Fatal("actor choices leaked to another viewer")
		}
		if lairs, ok := x["lairs"].(map[string]any); ok {
			if lairs["deck"] != nil || lairs["inventory"] != nil {
				t.Fatal("secret lair inventory leaked")
			}
			for _, raw := range lairs["sites"].([]any) {
				site := raw.(map[string]any)
				if (site["resolved"] == nil || site["resolved"] == float64(0)) && site["number"] != nil && site["number"] != float64(0) {
					t.Fatal("unliberated lair number leaked")
				}
			}
		}
		board := x["board"].(map[string]any)
		if board["hidden"] != nil || board["numbers"] != nil || x["economy"].(map[string]any)["turn"] != nil {
			t.Fatal("private explorer state leaked")
		}
		for owner, raw := range v["players"].([]any) {
			p := raw.(map[string]any)
			if owner != viewer && (p["resources"] != nil || p["dev"] != nil) {
				t.Fatal("opponent resources leaked")
			}
			if owner == viewer && p["resources"] == nil {
				t.Fatal("own hand missing")
			}
		}
	}
}

func TestCatanExplorerNaturalHTTPMatches(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newExplorerHTTP(t, n, false)
			restored, moves, steps := false, 0, 0
			for ; steps < 4000 && !s.rooms[id].Game.Finished; steps++ {
				r := s.rooms[id]
				g := r.Game
				actor := explorerHTTPActor(g)
				a, err := g.BotAction(actor)
				if err != nil {
					t.Fatal(steps, g.Phase, err)
				}
				// Play the actual wire preview, using BotAction only to choose an
				// intent. A shortest advertised voyage may differ from the bot's path.
				if g.Phase != "catan_discard" {
					view := current(clients[actor])["game"].(map[string]any)["catan"].(map[string]any)["explorer"].(map[string]any)
					found := false
					for _, raw := range view["choices"].([]any) {
						data, _ := json.Marshal(raw)
						var candidate game.Action
						if err := json.Unmarshal(data, &candidate); err != nil {
							t.Fatal(err)
						}
						match := reflect.DeepEqual(a, candidate)
						if a.Type == "catan_explorer_sail" && candidate.Type == a.Type && candidate.Slot == a.Slot && len(candidate.Targets) > 0 && len(a.Targets) > 0 {
							match = candidate.Targets[len(candidate.Targets)-1] == a.Targets[len(a.Targets)-1]
						}
						if match {
							a, found = candidate, true
							break
						}
					}
					if !found {
						t.Fatal("bot intent missing from network choices", a)
					}
				}
				phase, serial, deadline := g.Phase, g.Catan.TurnSerial, r.TurnDeadline
				if phase == "catan_explorer_move" {
					moves++
					if !restored {
						assertExplorerHTTPPrivacy(t, clients)
						before, _ := json.Marshal(r)
						clients[(actor+1)%n].command(current(clients[(actor+1)%n]), "action", a, 400)
						clients[n].command(current(clients[n]), "action", a, 400)
						bad := a
						bad.Prompt--
						clients[actor].command(current(clients[actor]), "action", bad, 400)
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("rejected action mutated room")
						}
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						restored = true
					}
				}
				clients[actor].command(current(clients[actor]), "action", a, 200)
				r = s.rooms[id]
				if !r.Game.Finished && r.Game.Catan.TurnSerial == serial && phase != "catan_discard" && r.Game.Phase != "catan_discard" && r.TurnDeadline != deadline {
					t.Fatal("ordinary action refreshed turn deadline", a.Type)
				}
				if !r.Game.Finished && r.Game.Catan.TurnSerial != serial && r.TurnDeadline-time.Now().UnixMilli() < 119000 {
					t.Fatal("new actor lacks full turn")
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || r.Game.Catan.Players[r.Game.Winners[0]].Score < 8 || !restored || moves == 0 || r.TurnDeadline != 0 {
				t.Fatal("incomplete natural game", steps, moves, r.Status)
			}
			t.Logf("%dp natural HTTP match: %d actions, %d movement actions", n, steps, moves)
		})
	}
}

func TestCatanExplorerHTTPDiscardClock(t *testing.T) {
	for _, mode := range []string{"manual", "autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			// Controlled response fixture: two hands of 8/9 resources, conserved bank,
			// real private roll [3,4]. Clock entry is isolated here, not natural HTTP play.
			s, ts, clients, id := newExplorerHTTP(t, 3, true)
			now := time.Now()
			s.mu.Lock()
			r := s.rooms[id]
			r.TurnDeadline = now.Add(37 * time.Second).UnixMilli()
			r.CatanPendingVersion = r.Version
			if !r.adjustCatanResponseClock("catan_roll", -1, r.Game.Catan.SetupStep, now) || r.CatanTimeLeft != 37000 {
				t.Fatal("discard did not pause original turn")
			}
			deadline := r.TurnDeadline
			if deadline != now.Add(120*time.Second).UnixMilli() {
				t.Fatal("missing shared response window")
			}
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			assertExplorerHTTPPrivacy(t, clients)
			pending := []int{}
			for p, due := range s.rooms[id].Game.Catan.DiscardDue {
				if due > 0 {
					pending = append(pending, p)
				}
			}
			if len(pending) != 2 {
				t.Fatal("must have two responders")
			}
			staleView := current(clients[pending[1]])
			if mode == "autoplay" {
				for _, p := range pending {
					setAutoPlay(clients[p], current(clients[p]), true, 200)
				}
			}
			for step := 0; s.rooms[id].Game.Phase == "catan_discard"; step++ {
				if step > 3 {
					t.Fatal("discard stalled")
				}
				r = s.rooms[id]
				if mode == "manual" {
					p := explorerHTTPActor(r.Game)
					a, err := r.Game.BotAction(p)
					if err != nil {
						t.Fatal(err)
					}
					view := current(clients[p])
					if step == 1 {
						view = staleView
					}
					clients[p].command(view, "action", a, 200)
					now = time.Now()
				} else {
					s.mu.Lock()
					if mode == "autoplay" {
						now = time.Now()
						r.BotAt = 0
						s.runBots(now)
					} else {
						now = time.UnixMilli(deadline)
						s.expireSetups(now)
					}
					s.mu.Unlock()
				}
				r = s.rooms[id]
				if r.Game.Phase == "catan_discard" && (r.TurnDeadline != deadline || r.CatanTimeLeft != 37000) {
					t.Fatal("first response reset shared deadline")
				}
			}
			r = s.rooms[id]
			remaining := r.TurnDeadline - now.UnixMilli()
			if r.Game.Phase != "catan_turn" || r.CatanTimeLeft != 0 || remaining < 36000 || remaining > 37000 {
				t.Fatal("original time not restored", remaining, r.Game.Phase)
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
		})
	}
}

func TestCatanExplorerHTTPAutoplayMovement(t *testing.T) {
	s, ts, clients, id := newExplorerHTTP(t, 3, false)
	// Reach movement using actual HTTP actions, never change the game phase.
	for step := 0; s.rooms[id].Game.Phase != "catan_explorer_move"; step++ {
		if step > 40 {
			t.Fatal("movement unreachable")
		}
		g := s.rooms[id].Game
		p := explorerHTTPActor(g)
		a, err := g.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		clients[p].command(current(clients[p]), "action", a, 200)
	}
	actor := s.rooms[id].Game.Turn
	setAutoPlay(clients[actor], current(clients[actor]), true, 200)
	expireTurn(t, s, id)
	kick(clients[(actor+1)%3], current(clients[(actor+1)%3]), s.rooms[id].Seats[actor].ID, 400)
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	deadline := s.rooms[id].TurnDeadline
	for step := 0; s.rooms[id].Game.Turn == actor; step++ {
		if step > 30 {
			t.Fatal("autoplay movement stalled")
		}
		s.mu.Lock()
		s.rooms[id].BotAt = 0
		s.runBots(time.Now())
		s.mu.Unlock()
		r := s.rooms[id]
		if r.Game.Turn == actor && r.TurnDeadline != deadline {
			t.Fatal("autoplay refreshed clock")
		}
	}
	r := s.rooms[id]
	if r.Game.Phase != "catan_roll" || r.TurnDeadline-time.Now().UnixMilli() < 119000 {
		t.Fatal("autoplay failed to hand off")
	}
}

func TestCatanExplorerHTTPTimeoutDeparture(t *testing.T) {
	for _, n := range []int{2, 3} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newExplorerHTTP(t, n, false)
			r := s.rooms[id]
			actor := r.Game.Turn
			other := (actor + 1) % n
			// Make the initial actor host to exercise host transfer regardless of the
			// private constructor's randomized first player; no game state is altered.
			s.mu.Lock()
			r.Host = r.Seats[actor].ID
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			target := r.Seats[actor].ID
			kick(clients[other], current(clients[other]), target, 400)
			expireTurn(t, s, id)
			before, _ := json.Marshal(s.rooms[id])
			kick(clients[n], current(clients[n]), target, 400)
			kick(clients[actor], current(clients[actor]), target, 400)
			kick(clients[other], current(clients[other]), r.Seats[other].ID, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid kick mutated room")
			}
			old := r.Game.Catan
			kick(clients[other], current(clients[other]), target, 200)
			r = s.rooms[id]
			g := r.Game.Catan
			x := g.Explorer
			if !r.Seats[actor].Left || !g.Players[actor].Eliminated || r.Host == target || x.Economy.Gold[actor] != 0 {
				t.Fatal("departure metadata incorrect")
			}
			if !reflect.DeepEqual(old.Vertices, g.Vertices) || !reflect.DeepEqual(old.Edges, g.Edges) {
				t.Fatal("fixed board changed")
			}
			for resource, held := range old.Players[actor].Resources {
				if g.Players[actor].Resources[resource] != 0 || g.Bank[resource] != old.Bank[resource]+held {
					t.Fatal("resources not returned")
				}
			}
			if x.Economy.GoldBank != old.Explorer.Economy.GoldBank+old.Explorer.Economy.Gold[actor] {
				t.Fatal("gold not returned")
			}
			for ship := actor * 3; ship < (actor+1)*3; ship++ {
				if x.Fleet.Positions[ship] != -1 {
					t.Fatal("departed ship still deployed")
				}
			}
			for unit := actor * 11; unit < (actor+1)*11; unit++ {
				if x.Cargo.Units[unit].Kind != "supply" {
					t.Fatal("departed cargo still deployed")
				}
			}
			if clients[actor].state()["room"] != nil {
				t.Fatal("departed user retained seat")
			}
			clients[actor].command(current(clients[other]), "action", game.Action{Type: "catan_roll", Prompt: int(g.TurnSerial)}, 400)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			if n == 2 {
				r = s.rooms[id]
				if r.Status != "finished" || !r.Game.Finished || len(r.Game.Winners) != 1 || r.Game.Winners[0] != other || r.Game.Catan.Players[other].Score >= 8 || r.TurnDeadline != 0 {
					t.Fatal("survivor not awarded")
				}
			} else {
				for step := 0; step < 4000 && !s.rooms[id].Game.Finished; step++ {
					g := s.rooms[id].Game
					p := explorerHTTPActor(g)
					if p == actor {
						t.Fatal("departed seat got another turn")
					}
					a, err := g.BotAction(p)
					if err != nil {
						t.Fatal(err)
					}
					clients[p].command(current(clients[p]), "action", a, 200)
				}
				if !s.rooms[id].Game.Finished {
					t.Fatal("survivors could not finish")
				}
			}
		})
	}
}
