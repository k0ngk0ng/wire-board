package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Provision only a verified private Explorer snapshot, after ordinary room
// registration/join/ready. This does not open unfinished public configuration.
func newExplorerPairedHTTP(t *testing.T, n int, fixture string) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("配对探险%d", p))
	}
	raw := clients[0].post("/api/rooms", map[string]any{"name": "配对探险联机", "kind": "catan", "capacity": n, "catanOptions": game.CatanOptions{FiveSix: true}}, 201)
	id := raw["id"].(string)
	for p := 1; p < n; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for p := 0; p < n; p++ {
		clients[p].command(current(clients[p]), "ready", nil, 200)
	}
	data, err := os.ReadFile("testdata/catan_explorer_paired_" + fixture + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var state game.State
	if err = json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Catan.Players) != n || state.Catan.Explorer == nil || state.Catan.Paired == nil {
		t.Fatal("wrong paired snapshot")
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

func assertExplorerPairedHTTPPrivacy(t *testing.T, clients []*testClient, state *game.State) {
	t.Helper()
	assertExplorerSpiceHTTPPrivacy(t, clients, state)
	for _, c := range clients {
		g := current(c)["game"].(map[string]any)["catan"].(map[string]any)
		p := g["paired"].(map[string]any)
		if p["primary"] != float64(state.Catan.Paired.Primary) || p["secondary"] != float64(state.Catan.Paired.Secondary) || p["second"] != state.Catan.Paired.Second {
			t.Fatal("public markers disagree")
		}
	}
}

func TestCatanExplorerPairedHTTPSetupTimeoutAndRestart(t *testing.T) {
	for _, n := range []int{5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newExplorerPairedHTTP(t, n, fmt.Sprintf("setup_%d", n))
			first := s.rooms[id].Game.Turn
			steps := 0
			for s.rooms[id].Game.Catan.Explorer.Setup != nil {
				if steps >= 4*n {
					t.Fatal("opening timeout stalled")
				}
				now := time.UnixMilli(s.rooms[id].TurnDeadline)
				s.mu.Lock()
				s.expireSetups(now)
				s.mu.Unlock()
				if s.rooms[id].TurnDeadline != now.Add(120*time.Second).UnixMilli() {
					t.Fatal("piece lacks fresh window")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				steps++
			}
			r := s.rooms[id]
			if steps != 4*n || r.Game.Phase != "catan_roll" || r.Game.Turn != first || r.Game.Catan.Paired.Second {
				t.Fatal("wrong first portion")
			}
			assertExplorerPairedHTTPPrivacy(t, clients, r.Game)
		})
	}
}

func TestCatanExplorerPairedHTTPResponseHandoff(t *testing.T) {
	for _, second := range []bool{false, true} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%t/%s", second, mode), func(t *testing.T) {
				s, ts, clients, id := newExplorerPairedHTTP(t, 6, fmt.Sprintf("resolve_%t", second))
				r := s.rooms[id]
				actor, serial, rolls := r.Game.Turn, r.Game.Catan.TurnSerial, r.Game.Catan.RollID
				primary, secondary := r.Game.Catan.Paired.Primary, r.Game.Catan.Paired.Secondary
				deadline := r.TurnDeadline
				if mode == "autoplay" {
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
				}
				seen := map[string]int{}
				for steps := 0; s.rooms[id].Game.Catan.TurnSerial == serial; steps++ {
					if steps >= 60 {
						t.Fatal("shared response stalled")
					}
					r = s.rooms[id]
					phase := r.Game.Phase
					seen[phase]++
					if r.TurnDeadline != deadline || r.Game.CatanPendingActor() != actor {
						t.Fatal("response clock reset")
					}
					a, err := r.Game.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					a = fishHTTPPreview(t, current(clients[actor]), a)
					now := time.Now()
					if mode == "manual" {
						before, _ := json.Marshal(r)
						clients[(actor+1)%6].command(current(clients[(actor+1)%6]), "action", a, 400)
						clients[6].command(current(clients[6]), "action", a, 400)
						bad := a
						bad.Prompt--
						clients[actor].command(current(clients[actor]), "action", bad, 400)
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("invalid response changed room")
						}
						view := current(clients[actor])
						req := map[string]any{"type": "action", "action": a, "version": view["version"], "nonce": randomID(12)}
						clients[actor].post("/api/rooms/"+id, req, 200)
						now = time.Now()
						before, _ = json.Marshal(s.rooms[id])
						clients[actor].post("/api/rooms/"+id, req, 200)
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						clients[actor].post("/api/rooms/"+id, req, 200)
						after, _ = json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("response replay paid twice")
						}
					} else {
						s.mu.Lock()
						if mode == "autoplay" {
							r.BotAt = 0
							s.runBots(now)
						} else {
							now = time.UnixMilli(deadline)
							s.expireSetups(now)
						}
						s.mu.Unlock()
					}
					r = s.rooms[id]
					if r.Game.Catan.TurnSerial != serial {
						left := r.TurnDeadline - now.UnixMilli()
						if left < 119000 || left > 120000 || r.CatanTimeLeft != 0 {
							t.Fatal("next portion lacks own two minutes", left)
						}
					} else if r.TurnDeadline != deadline {
						t.Fatal("hero reroll renewed time")
					}
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
				}
				r = s.rooms[id]
				wantActor, wantPhase := secondary, "catan_turn"
				if second {
					wantActor, wantPhase = (primary+1)%6, "catan_roll"
				}
				if r.Game.Turn != wantActor || r.Game.Phase != wantPhase || r.Game.Catan.RollID != rolls || r.Game.Catan.TurnSerial != serial+1 || seen["catan_explorer_resolve"] == 0 || mode != "timeout" && seen["catan_explorer_battle"] == 0 {
					t.Fatal("response advanced wrong portion", seen)
				}
				assertExplorerPairedHTTPPrivacy(t, clients, r.Game)
			})
		}
	}
}

func TestCatanExplorerPairedHTTPNaturalMatches(t *testing.T) {
	for _, n := range []int{5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newExplorerPairedHTTP(t, n, fmt.Sprintf("setup_%d", n))
			auto := n == 6
			if auto {
				for p := 0; p < n; p++ {
					setAutoPlay(clients[p], current(clients[p]), true, 200)
				}
			}
			secondActions, handoffs, steps := 0, 0, 0
			restored := map[string]bool{}
			for ; steps < 12000 && !s.rooms[id].Game.Finished; steps++ {
				r := s.rooms[id]
				actor := explorerHTTPActor(r.Game)
				serial, phase, pending, deadline := r.Game.Catan.TurnSerial, r.Game.Phase, r.Game.CatanPendingActor(), r.TurnDeadline
				if r.Game.Catan.Paired.Second {
					secondActions++
				}
				if auto {
					before := r.Version
					s.mu.Lock()
					r.BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Version <= before {
						t.Fatal("paired autoplay stalled", phase)
					}
				} else {
					a, err := r.Game.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					a = fishHTTPPreview(t, current(clients[actor]), a)
					clients[actor].command(current(clients[actor]), "action", a, 200)
				}
				r = s.rooms[id]
				if !r.Game.Finished && r.Game.Catan.TurnSerial != serial && phase != "catan_explorer_setup" {
					handoffs++
					left := r.TurnDeadline - time.Now().UnixMilli()
					if left < 118000 || left > 120000 || r.CatanTimeLeft != 0 {
						t.Fatal("handoff clock", left)
					}
				} else if !r.Game.Finished && phase != "catan_explorer_setup" && pending < 0 && r.Game.CatanPendingActor() < 0 && phase != "catan_discard" && r.Game.Phase != "catan_discard" && r.TurnDeadline != deadline {
					t.Fatal("action extended deadline")
				}
				key := fmt.Sprintf("%t/%s", r.Game.Catan.Paired.Second, r.Game.Phase)
				if !restored[key] || steps%173 == 0 {
					assertExplorerPairedHTTPPrivacy(t, clients, r.Game)
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					restored[key] = true
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || r.Status != "finished" || r.TurnDeadline != 0 || len(r.Game.Winners) != 1 || r.Game.Catan.Players[r.Game.Winners[0]].Score < 17 || secondActions == 0 || handoffs < 2 {
				t.Fatal("paired network game incomplete", steps)
			}
			if len(r.Game.Catan.Explorer.Fish.Deliveries) == 0 || len(r.Game.Catan.Explorer.Spice.Deliveries) == 0 {
				t.Fatal("mission deliveries missing")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			assertExplorerPairedHTTPPrivacy(t, clients, s.rooms[id].Game)
			t.Log("steps", steps, "secondary actions", secondActions, "handoffs", handoffs, "round", r.Game.Round)
		})
	}
}

func TestCatanExplorerPairedHTTPReclaimAndTimeoutRemoval(t *testing.T) {
	for _, second := range []bool{false, true} {
		for _, mode := range []string{"takeover", "remove"} {
			t.Run(fmt.Sprintf("%t/%s", second, mode), func(t *testing.T) {
				// Finishing the opposite portion's natural lair gives an ordinary first
				// or second portion without editing the game's phase, cards or markers.
				s, ts, clients, id := newExplorerPairedHTTP(t, 6, fmt.Sprintf("resolve_%t", !second))
				serial := s.rooms[id].Game.Catan.TurnSerial
				for steps := 0; s.rooms[id].Game.Catan.TurnSerial == serial; steps++ {
					if steps > 60 {
						t.Fatal("response stalled")
					}
					r := s.rooms[id]
					a, err := r.Game.BotAction(r.Game.Turn)
					if err != nil {
						t.Fatal(err)
					}
					clients[r.Game.Turn].command(current(clients[r.Game.Turn]), "action", fishHTTPPreview(t, current(clients[r.Game.Turn]), a), 200)
				}
				r := s.rooms[id]
				actor, primary, secondary := r.Game.Turn, r.Game.Catan.Paired.Primary, r.Game.Catan.Paired.Secondary
				if r.Game.Catan.Paired.Second != second {
					t.Fatal("wrong ordinary portion")
				}
				serial, rolls := r.Game.Catan.TurnSerial, r.Game.Catan.RollID
				// A valid bank/resource hand does not authorize a secondary production roll
				// or a domestic trade. Other users and spectators cannot act for this seat.
				if second {
					before, _ := json.Marshal(r)
					clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_roll", Prompt: int(serial)}, 400)
					clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_trade_offer", Prompt: int(serial), GoldGive: 1, Give: make([]int, 5), Take: []int{1, 0, 0, 0, 0}}, 400)
					a := game.Action{Type: "catan_explorer_begin_move", Prompt: int(serial)}
					clients[(actor+1)%6].command(current(clients[(actor+1)%6]), "action", a, 400)
					clients[6].command(current(clients[6]), "action", a, 400)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("invalid secondary requests changed room")
					}
				}
				target := r.Seats[actor].ID
				other := (actor + 1) % 6
				kick(clients[other], current(clients[other]), target, 400)
				expireTurn(t, s, id)
				before, _ := json.Marshal(s.rooms[id])
				s.mu.Lock()
				s.expireSetups(time.Now())
				s.mu.Unlock()
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("ordinary expired portion auto-executed")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				deadline := s.rooms[id].TurnDeadline
				if mode == "takeover" {
					stale := current(clients[actor])
					setAutoPlay(clients[actor], stale, true, 200)
					kick(clients[other], current(clients[other]), target, 400)
					r = s.rooms[id]
					a, err := r.Game.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					clients[actor].command(current(clients[actor]), "action", a, 400)
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					version := s.rooms[id].Version
					botTick(s, id)
					if s.rooms[id].Version <= version {
						t.Fatal("takeover stalled")
					}
					setAutoPlay(clients[actor], stale, false, 200)
					if s.rooms[id].Seats[actor].AutoPlay {
						t.Fatal("reclaim failed")
					}
					// A first-player roll can enter mandatory seven responses. For a second
					// action there is no production, so takeover must keep the expired clock.
					if second && s.rooms[id].TurnDeadline != deadline {
						t.Fatal("takeover/reclaim renewed secondary time")
					}
				} else {
					kick(clients[6], current(clients[6]), target, 400)
					kick(clients[other], current(clients[other]), target, 200)
					r = s.rooms[id]
					want, phase := secondary, "catan_turn"
					if second {
						want, phase = (primary+1)%6, "catan_roll"
					}
					if !r.Seats[actor].Left || !r.Game.Catan.Players[actor].Eliminated || r.Game.Turn != want || r.Game.Phase != phase || r.Game.Catan.RollID != rolls || r.Game.Catan.TurnSerial != serial+1 {
						t.Fatal("removal advanced wrong marker")
					}
					left := r.TurnDeadline - time.Now().UnixMilli()
					if left < 119000 || left > 120000 {
						t.Fatal("removal next seat clock")
					}
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				// A removed client can no longer read private state; verify remaining
				// players and observer through the ordinary expanded privacy checks above.
				if mode == "takeover" {
					assertExplorerPairedHTTPPrivacy(t, clients, s.rooms[id].Game)
				}
			})
		}
	}
}

func TestCatanExplorerPairedHTTPKickCannotInterruptResponse(t *testing.T) {
	for _, second := range []bool{false, true} {
		for _, battle := range []bool{false, true} {
			t.Run(fmt.Sprintf("%t/%t", second, battle), func(t *testing.T) {
				s, ts, clients, id := newExplorerPairedHTTP(t, 6, fmt.Sprintf("resolve_%t", second))
				actor := s.rooms[id].Game.Turn
				if battle {
					r := s.rooms[id]
					a, err := r.Game.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					clients[actor].command(current(clients[actor]), "action", fishHTTPPreview(t, current(clients[actor]), a), 200)
					if s.rooms[id].Game.Phase != "catan_explorer_battle" {
						t.Fatal("missing shared hero response")
					}
				}
				target := s.rooms[id].Seats[actor].ID
				other := (actor + 1) % 6
				kick(clients[other], current(clients[other]), target, 400)
				expireTurn(t, s, id)
				before, _ := json.Marshal(s.rooms[id].Game)
				// The handler processes mandatory timeouts first; the stale kick must lose
				// that version race, irrespective of which paired marker is currently acting.
				kick(clients[other], current(clients[other]), target, 409)
				r := s.rooms[id]
				after, _ := json.Marshal(r.Game)
				if string(before) == string(after) {
					t.Fatal("mandatory response did not advance")
				}
				for p, seat := range r.Seats {
					if seat.Left || r.Game.Catan.Players[p].Eliminated {
						t.Fatal("kick interrupted mandatory response")
					}
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				assertExplorerPairedHTTPPrivacy(t, clients, s.rooms[id].Game)
			})
		}
	}
}
