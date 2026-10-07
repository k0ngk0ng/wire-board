package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Public room recipes remain gated. Start normal authenticated rooms, then
// install a validated private combination snapshot, keeping real HTTP/storage.
func newExplorerCityHTTP(t *testing.T, n int, phase string) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("城市探险%d", p))
	}
	raw := clients[0].post("/api/rooms", map[string]any{"name": "城市探险联机验收", "kind": "catan", "capacity": n, "catanOptions": game.CatanOptions{FiveSix: n > 4}}, 201)
	id := raw["id"].(string)
	for p := 1; p < n; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for p := 0; p < n; p++ {
		clients[p].command(current(clients[p]), "ready", nil, 200)
	}
	data, err := os.ReadFile(fmt.Sprintf("testdata/catan_explorer_city_%s_%d.json", phase, n))
	if err != nil {
		t.Fatal(err)
	}
	var state game.State
	if err = json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if state.Catan == nil || state.Catan.Explorer == nil || state.Catan.CitiesKnights == nil || len(state.Catan.Players) != n {
		t.Fatal("wrong combined fixture")
	}
	s.mu.Lock()
	r := s.rooms[id]
	r.Game, r.Status = &state, "playing"
	r.startTurnClock(time.Now())
	r.CatanPendingVersion = r.Version
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	return s, ts, clients, id
}

func assertExplorerCityHTTPPrivacy(t *testing.T, clients []*testClient, state *game.State) {
	t.Helper()
	for viewer, c := range clients {
		room, ok := c.state()["room"].(map[string]any)
		if !ok {
			if viewer >= len(state.Catan.Players) || !state.Catan.Players[viewer].Eliminated {
				t.Fatal("active player or spectator lost room", viewer)
			}
			// A kicked participant is returned to the lobby, with no private view.
			continue
		}
		v := room["game"].(map[string]any)["catan"].(map[string]any)
		x := v["explorer"].(map[string]any)
		k := v["citiesKnights"].(map[string]any)
		if k["progressDecks"] != nil || k["event"] != nil || x["board"].(map[string]any)["hidden"] != nil || x["board"].(map[string]any)["numbers"] != nil || x["lairs"].(map[string]any)["deck"] != nil {
			t.Fatal("hidden deck/event/world leaked", viewer)
		}
		for p, raw := range v["players"].([]any) {
			hand, ok := raw.(map[string]any)["resources"]
			if ok != (p == viewer) || ok && len(hand.([]any)) != 8 {
				t.Fatal("wrong eight-card hand visibility", viewer, p)
			}
		}
		for p, raw := range k["players"].([]any) {
			hand, ok := raw.(map[string]any)["progress"]
			if ok != (p == viewer) {
				t.Fatal("private progress hand leaked/omitted", viewer, p)
			}
			if ok && len(hand.([]any)) != len(state.Catan.CitiesKnights.Players[p].Progress) {
				t.Fatal("own progress count mismatch")
			}
		}
		actor := state.CatanPendingActor()
		if actor < 0 {
			actor = state.Turn
		}
		can := viewer == actor
		if state.Phase == "catan_discard" {
			can = viewer < len(state.Catan.Players) && state.Catan.DiscardDue[viewer] > 0
		}
		if x["canRespond"] != can {
			t.Fatal("wrong response owner", viewer, state.Phase, x["canRespond"], actor)
		}
		if !can && (len(x["choices"].([]any)) > 0 || x["response"] != nil) {
			t.Fatal("response/choices leaked to nonactor", viewer)
		}
		for _, raw := range x["choices"].([]any) {
			if raw.(map[string]any)["prompt"] != float64(state.Catan.TurnSerial) {
				t.Fatal("choice lost turn serial")
			}
		}
		if pending, ok := k["pending"].(map[string]any); ok {
			q := state.Catan.CitiesKnights.Pending
			if pending["resources"] != nil && (viewer != actor || q.Kind != "guild_dues") {
				t.Fatal("private resource reveal leaked")
			}
			revealed, hasReveal := pending["progress"]
			if hasReveal != (viewer == actor && q.Kind == "espionage") {
				t.Fatal("private espionage reveal leaked/omitted")
			}
			if hasReveal && len(revealed.([]any)) != len(state.Catan.CitiesKnights.Players[q.Target].Progress) {
				t.Fatal("authorized espionage reveal incomplete")
			}
			if q.Kind == "commercial_harbor" && viewer != actor && viewer != q.Target && pending["color"] != nil {
				t.Fatal("private commercial offer leaked")
			}
		}
	}
}

func explorerCityHTTPAction(t *testing.T, clients []*testClient, s *Server, id string, p int, a game.Action) {
	t.Helper()
	a.Prompt = int(s.rooms[id].Game.Catan.TurnSerial)
	clients[p].command(current(clients[p]), "action", a, 200)
}

// Exercise actual manual requests, the production timeout runner and autoplay
// across saved off-turn responses; do not construct synthetic Pending structs.
func TestCatanExplorerCityHTTPResponsesRestoreClockAndPrivacy(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, kind := range []string{"commercial_harbor", "espionage", "sabotage"} {
			for _, mode := range []string{"manual", "timeout", "autoplay"} {
				t.Run(fmt.Sprintf("%d/%s/%s", n, kind, mode), func(t *testing.T) {
					s, ts, clients, id := newExplorerCityHTTP(t, n, "action")
					p := s.rooms[id].Game.Turn
					target := (p + 1) % n
					beforeHands := make([][]int, n)
					for i, h := range s.rooms[id].Game.Catan.Players {
						beforeHands[i] = append([]int(nil), h.Resources...)
					}
					s.mu.Lock()
					s.rooms[id].TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
					if err := s.save(s.rooms[id]); err != nil {
						t.Fatal(err)
					}
					s.mu.Unlock()
					deadline := s.rooms[id].TurnDeadline
					card := map[string]int{"commercial_harbor": 10, "espionage": 18, "sabotage": 20}[kind]
					explorerCityHTTPAction(t, clients, s, id, p, game.Action{Type: "catan_progress", Card: card, Target: target})
					if kind == "commercial_harbor" {
						if s.rooms[id].TurnDeadline != deadline {
							t.Fatal("optional power reset action clock")
						}
						explorerCityHTTPAction(t, clients, s, id, p, game.Action{Type: "catan_commercial_offer", Card: 0, Target: target, Color: 0})
					}
					r := s.rooms[id]
					if r.Game.Phase != "catan_"+kind || r.CatanTimeLeft < 40000 || r.CatanTimeLeft > 45000 {
						t.Fatal("response did not save action time", r.Game.Phase, r.CatanTimeLeft)
					}
					left := r.CatanTimeLeft
					if dt := r.TurnDeadline - time.Now().UnixMilli(); dt < 115000 || dt > 120000 {
						t.Fatal("response did not get two minutes", dt)
					}
					replies := 0
					for s.rooms[id].Game.CatanPendingActor() >= 0 {
						r = s.rooms[id]
						actor := r.Game.CatanPendingActor()
						if replies >= n {
							t.Fatal("response chain did not finish")
						}
						assertExplorerCityHTTPPrivacy(t, clients, r.Game)
						a, err := r.Game.BotAction(actor)
						if err != nil {
							t.Fatal(err)
						}
						if a.Type != "catan_"+kind {
							t.Fatal("unexpected response", a)
						}
						before, _ := json.Marshal(r)
						clients[(actor+1)%n].command(current(clients[(actor+1)%n]), "action", a, 400)
						clients[n].command(current(clients[n]), "action", a, 400)
						bad := a
						bad.Prompt--
						clients[actor].command(current(clients[actor]), "action", bad, 400)
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("invalid response mutated room")
						}
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						r = s.rooms[id]
						var at time.Time
						switch mode {
						case "manual":
							request := map[string]any{"type": "action", "version": r.Version, "nonce": randomID(12), "action": a}
							old := current(clients[actor])
							clients[actor].post("/api/rooms/"+id, request, 200)
							at = time.Now()
							saved, _ := json.Marshal(s.rooms[id])
							clients[actor].post("/api/rooms/"+id, request, 200)
							s, ts = restartRiversHTTP(t, s, ts, clients, id)
							clients[actor].post("/api/rooms/"+id, request, 200)
							clients[actor].command(old, "action", a, 409)
							now, _ := json.Marshal(s.rooms[id])
							if string(saved) != string(now) {
								t.Fatal("replay changed progress/cards/time")
							}
						case "timeout":
							at = time.UnixMilli(r.TurnDeadline)
							s.mu.Lock()
							s.expireSetups(at)
							s.mu.Unlock()
						case "autoplay":
							setAutoPlay(clients[actor], current(clients[actor]), true, 200)
							at = time.Now()
							s.mu.Lock()
							s.rooms[id].BotAt = 0
							s.runBots(at)
							s.mu.Unlock()
							setAutoPlay(clients[actor], current(clients[actor]), false, 200)
						}
						r = s.rooms[id]
						replies++
						if r.Game.CatanPendingActor() >= 0 {
							remaining := r.TurnDeadline - at.UnixMilli()
							if r.CatanTimeLeft != left || remaining > turnLimit.Milliseconds() || remaining < turnLimit.Milliseconds()-5000 || r.TurnDeadline != at.Add(turnLimit).UnixMilli() && mode != "manual" {
								t.Fatal("next responder lost time", mode)
							}
						} else if r.Game.Phase != "catan_turn" || r.Game.Turn != p || r.CatanTimeLeft != 0 || r.TurnDeadline-at.UnixMilli() > left || r.TurnDeadline-at.UnixMilli() < left-5000 {
							t.Fatal("action budget did not resume", mode, r.Game.Phase, r.TurnDeadline-at.UnixMilli(), left)
						}
					}
					if kind == "sabotage" && replies != n-1 || kind != "sabotage" && replies != 1 {
						t.Fatal("wrong number of responses", replies)
					}
					g := s.rooms[id].Game.Catan
					if kind == "commercial_harbor" {
						if g.Players[p].Resources[0] != beforeHands[p][0]-1 || g.Players[target].Resources[0] != beforeHands[target][0]+1 {
							t.Fatal("commercial resource exchange missing")
						}
						changes := 0
						for c := 5; c < 8; c++ {
							changes += g.Players[p].Resources[c] - beforeHands[p][c]
						}
						if changes != 1 {
							t.Fatal("commodity not transferred")
						}
					}
					if kind == "espionage" && (len(g.CitiesKnights.Players[p].Progress) != 4 || len(g.CitiesKnights.Players[target].Progress) != 1) {
						t.Fatal("espionage did not transfer one card")
					}
					if kind == "sabotage" {
						for seat, hand := range beforeHands {
							if seat == p {
								continue
							}
							beforeCount, afterCount := 0, 0
							for card, amount := range hand {
								beforeCount += amount
								afterCount += g.Players[seat].Resources[card]
							}
							if afterCount != beforeCount-beforeCount/2 {
								t.Fatal("sabotage did not discard half of all eight cards", seat)
							}
						}
					}
					assertExplorerCityHTTPPrivacy(t, clients, s.rooms[id].Game)
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
				})
			}
		}
	}
}

func TestCatanExplorerCityHTTPFreeRoadsReplayAndWebsocket(t *testing.T) {
	s, ts, clients, id := newExplorerCityHTTP(t, 3, "action")
	p := s.rooms[id].Game.Turn
	deadline := s.rooms[id].TurnDeadline
	explorerCityHTTPAction(t, clients, s, id, p, game.Action{Type: "catan_progress", Card: 7})
	if s.rooms[id].Game.Phase != "catan_roads" {
		t.Fatal("road building did not open")
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/ws", &websocket.DialOptions{HTTPClient: clients[3].client})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if _, _, err = conn.Read(ctx); err != nil {
		t.Fatal(err)
	}
	placed := 0
	for s.rooms[id].Game.Phase == "catan_roads" {
		r := s.rooms[id]
		before := append([]int(nil), r.Game.Catan.Players[p].Resources...)
		v := current(clients[p])
		choices := v["game"].(map[string]any)["catan"].(map[string]any)["explorer"].(map[string]any)["choices"].([]any)
		if len(choices) == 0 || placed >= 2 {
			t.Fatal("no free road choices")
		}
		clients[p].command(v, "action", choices[0], 200)
		if !reflect.DeepEqual(before, s.rooms[id].Game.Catan.Players[p].Resources) || s.rooms[id].TurnDeadline != deadline {
			t.Fatal("free road spent cards/time")
		}
		if _, raw, e := conn.Read(ctx); e != nil || string(raw) != `{"type":"changed"}` {
			t.Fatal("private data in websocket notification", e, string(raw))
		}
		placed++
	}
	if placed != 2 {
		t.Fatal("expected two legal roads", placed)
	}
	assertExplorerCityHTTPPrivacy(t, clients, s.rooms[id].Game)
	conn.CloseNow()
	_, _ = restartRiversHTTP(t, s, ts, clients, id)
}

// No resource, score or progress injection: play the committed post-opening
// snapshots through real production HTTP until the combined victory target.
func TestCatanExplorerCityNaturalHTTPMatches(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newExplorerCityHTTP(t, n, "roll")
			seen := map[string]int{}
			steps := 0
			restored := false
			for ; steps < 14000 && !s.rooms[id].Game.Finished; steps++ {
				r := s.rooms[id]
				state := r.Game
				actor := state.CatanPendingActor()
				if actor < 0 {
					actor = explorerHTTPActor(state)
				}
				a, err := state.BotAction(actor)
				if err != nil {
					t.Fatal("bot", steps, state.Phase, err)
				}
				if a.Prompt != int(state.Catan.TurnSerial) {
					t.Fatal("bot omitted serial", a)
				}
				code, result := clients[actor].request("POST", "/api/rooms/"+id, map[string]any{"type": "action", "version": r.Version, "nonce": randomID(12), "action": a})
				if code != 200 {
					raw, _ := json.Marshal(state)
					t.Log("failed natural state", string(raw))
					t.Fatal("natural HTTP action", steps, state.Phase, a, code, result)
				}
				seen[a.Type]++
				if !s.rooms[id].Game.Finished && steps%151 == 0 {
					assertExplorerCityHTTPPrivacy(t, clients, s.rooms[id].Game)
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					restored = true
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || r.TurnDeadline != 0 || !restored {
				t.Fatal("natural combined match did not finish", steps, r.Game.Round, seen)
			}
			winner := r.Game.Winners[0]
			if r.Game.Catan.Players[winner].Score < r.Game.Catan.Explorer.Board.Target {
				t.Fatal("won below combined target")
			}
			if seen["catan_roll"] == 0 || seen["catan_explorer_sail"] == 0 || seen["catan_explorer_begin_move"] == 0 {
				t.Fatal("match omitted production/movement", seen)
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			t.Log("natural combined actions", steps, "round", r.Game.Round, "seen", seen)
		})
	}
}

func TestCatanExplorerCityHTTPTimeoutRemovalAndContinue(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, phase := range []string{"roll", "movement"} {
			t.Run(fmt.Sprintf("%d/%s", n, phase), func(t *testing.T) {
				fixture := phase
				if fixture == "movement" {
					fixture = "action"
				}
				s, ts, clients, id := newExplorerCityHTTP(t, n, fixture)
				p := s.rooms[id].Game.Turn
				if phase == "movement" {
					explorerCityHTTPAction(t, clients, s, id, p, game.Action{Type: "catan_explorer_begin_move"})
				}
				kick := func(c *testClient, want int) {
					r := current(c)
					c.post("/api/rooms/"+id, map[string]any{"type": "kick_timeout", "target": s.rooms[id].Seats[p].ID, "version": r["version"], "nonce": randomID(12)}, want)
				}
				actor := (p + 1) % n
				before, _ := json.Marshal(s.rooms[id])
				kick(clients[actor], 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("early kick changed room")
				}
				s.mu.Lock()
				r := s.rooms[id]
				r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				before, _ = json.Marshal(s.rooms[id])
				kick(clients[p], 400)
				kick(clients[n], 400)
				after, _ = json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("unauthorized kick changed room")
				}
				setAutoPlay(clients[p], current(clients[p]), true, 200)
				kick(clients[actor], 400)
				setAutoPlay(clients[p], current(clients[p]), false, 200)
				kick(clients[actor], 200)
				if clients[p].state()["room"] != nil {
					t.Fatal("kicked player did not return to lobby")
				}
				r = s.rooms[id]
				g := r.Game.Catan
				if !r.Seats[p].Left || !g.Players[p].Eliminated || r.Game.Turn == p || r.Status != "playing" || len(g.CitiesKnights.Players[p].Progress) != 0 || g.Explorer.Economy.Gold[p] != 0 || r.CatanTimeLeft != 0 {
					t.Fatal("timeout removal incomplete")
				}
				for _, v := range g.Players[p].Resources {
					if v != 0 {
						t.Fatal("departed hand retained")
					}
				}
				if left := r.TurnDeadline - time.Now().UnixMilli(); left < 115000 || left > 120000 {
					t.Fatal("remaining player lacks full window", left)
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				assertExplorerCityHTTPPrivacy(t, clients, s.rooms[id].Game)
				serial := s.rooms[id].Game.Catan.TurnSerial
				for steps := 0; s.rooms[id].Game.Catan.TurnSerial < serial+3; steps++ {
					if steps > 250 {
						t.Fatal("remaining HTTP game stalled")
					}
					state := s.rooms[id].Game
					next := state.CatanPendingActor()
					if next < 0 {
						next = explorerHTTPActor(state)
					}
					if next == p {
						t.Fatal("remaining game requested departed player")
					}
					a, err := state.BotAction(next)
					if err != nil {
						t.Fatal(err)
					}
					clients[next].command(current(clients[next]), "action", a, 200)
				}
				assertExplorerCityHTTPPrivacy(t, clients, s.rooms[id].Game)
				_, _ = restartRiversHTTP(t, s, ts, clients, id)
			})
		}
	}
}

func TestCatanExplorerCityHTTPSimultaneousEightCardDiscard(t *testing.T) {
	s, ts, clients, id := newExplorerCityHTTP(t, 6, "action")
	// Complete the primary and secondary portions normally, then use a real
	// Alchemist card to force seven. Only this owned card is added to the fixture.
	for s.rooms[id].Game.Phase != "catan_roll" {
		p := s.rooms[id].Game.Turn
		explorerCityHTTPAction(t, clients, s, id, p, game.Action{Type: "catan_explorer_begin_move"})
		explorerCityHTTPAction(t, clients, s, id, p, game.Action{Type: "catan_end"})
	}
	s.mu.Lock()
	r := s.rooms[id]
	p := r.Game.Turn
	cityProgressGive(t, r.Game.Catan, p, 0)
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	explorerCityHTTPAction(t, clients, s, id, p, game.Action{Type: "catan_progress", Card: 0, Tokens: []int{3, 4}})
	r = s.rooms[id]
	if r.Game.Phase != "catan_discard" {
		t.Fatal("Alchemist did not enter seven discards", r.Game.Phase)
	}
	deadline := r.TurnDeadline
	budget := r.CatanTimeLeft
	views := make([]map[string]any, 6)
	requests := make([]game.Action, 6)
	for seat := 0; seat < 6; seat++ {
		if r.Game.Catan.DiscardDue[seat] == 0 {
			t.Fatal("fixture missing simultaneous discarder", seat)
		}
		views[seat] = current(clients[seat])
		var err error
		requests[seat], err = r.Game.BotAction(seat)
		if err != nil {
			t.Fatal(err)
		}
		if len(requests[seat].Tokens) != 8 {
			t.Fatal("discard lost commodities")
		}
	}
	assertExplorerCityHTTPPrivacy(t, clients, r.Game)
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	for seat := 5; seat >= 0; seat-- {
		// The initial shared version stays valid for other outstanding discarders.
		clients[seat].command(views[seat], "action", requests[seat], 200)
		clients[seat].command(current(clients[seat]), "action", requests[seat], 400)
		if seat > 0 && (s.rooms[id].TurnDeadline != deadline || s.rooms[id].CatanTimeLeft != budget) {
			t.Fatal("one discard reset simultaneous clock")
		}
	}
	if s.rooms[id].Game.Phase != "catan_explorer_pirate_place" || s.rooms[id].CatanTimeLeft != budget {
		t.Fatal("discard did not continue to pirate")
	}
	assertExplorerCityHTTPPrivacy(t, clients, s.rooms[id].Game)
	_, _ = restartRiversHTTP(t, s, ts, clients, id)
}
