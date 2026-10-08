package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"os"
	"testing"
	"time"
)

func transportHTTPFixture(t *testing.T, n int) *game.State {
	t.Helper()
	data, err := os.ReadFile(fmt.Sprintf("testdata/catan_transport_%d.json", n))
	if err != nil {
		t.Fatal(err)
	}
	var s game.State
	if err = json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_setup_settlement" || s.Catan.SetupStep != 0 || s.Catan.Transport == nil {
		t.Fatal("must start from fresh constructor fixture")
	}
	return &s
}
func TestCatanTransportNaturalHTTPMatches(t *testing.T) {
	testCatanTransportFullHTTP(t, false)
}

func TestCatanTransportEventsNaturalHTTP(t *testing.T) {
	testCatanTransportFullHTTP(t, true)
}

func testCatanTransportFullHTTP(t *testing.T, events bool) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newPublicFishingScenarioVariants(t, n, "transport", false, false, events)
			r := s.rooms[id]
			if r.Game.Catan.Transport == nil || (r.Game.Catan.Two != nil) != (n == 2) {
				t.Fatal("wrong public transport opening")
			}
			restored := false
			twoRestored := map[string]bool{}
			steps := 0
			moves := 0
			pairedMoves := map[bool]bool{}
			eventRestored := false
			for ; steps < 4000 && !s.rooms[id].Game.Finished; steps++ {
				r = s.rooms[id]
				g := r.Game
				assertTransportInventory(t, g)
				actor := g.CatanPendingActor()
				if actor < 0 {
					actor = g.Turn
				}
				if g.Phase == "catan_discard" {
					for p, n := range g.Catan.DiscardDue {
						if n > 0 {
							actor = p
							break
						}
					}
				}
				action, err := g.BotAction(actor)
				if err != nil {
					t.Fatal(err)
				}
				phase, deadline := g.Phase, r.TurnDeadline
				if events && phase == "catan_card_event" && !eventRestored {
					assertPublicEventsPrivacy(t, clients)
					before, _ := json.Marshal(r)
					clients[(actor+1)%n].command(current(clients[(actor+1)%n]), "action", action, 400)
					clients[n].command(current(clients[n]), "action", action, 400)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("event rejection mutated room")
					}
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					eventRestored = true
				}
				if phase == "catan_transport_move" {
					moves++
					if g.Catan.Paired != nil {
						pairedMoves[g.Catan.Paired.Second] = true
					}
					// Probe the actual network view at the first move, then resume the same
					// persisted response and deadline through a real server restart.
					if !restored {
						for viewer, c := range clients {
							v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
							x := v["transport"].(map[string]any)
							if x["stacks"] != nil || x["gold"] != nil {
								t.Fatal("raw transport leaked")
							}
							if viewer != actor && len(x["choices"].(map[string]any)) != 0 {
								t.Fatal("resource-dependent choices leaked")
							}
							if x["canAct"] != (viewer == actor) {
								t.Fatal("wrong controller")
							}
							for owner, raw := range v["players"].([]any) {
								if owner != viewer && (raw.(map[string]any)["resources"] != nil || raw.(map[string]any)["dev"] != nil) {
									t.Fatal("hidden cards leaked")
								}
							}
						}
						before, _ := json.Marshal(r)
						clients[(actor+1)%n].command(current(clients[(actor+1)%n]), "action", action, 400)
						clients[n].command(current(clients[n]), "action", action, 400)
						bad := action
						bad.Offer--
						clients[actor].command(current(clients[actor]), "action", bad, 400)
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("rejection mutated response")
						}
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						restored = true
					}
				}
				if n == 2 && (phase == "catan_two_trade" || phase == "catan_two_build") && !twoRestored[phase] {
					assertTwoHTTPPrivacy(t, clients, g)
					before, _ := json.Marshal(r)
					clients[1-actor].command(current(clients[1-actor]), "action", action, 400)
					clients[2].command(current(clients[2]), "action", action, 400)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("two-player response rejection mutated state")
					}
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					twoRestored[phase] = true
				}
				clients[actor].command(current(clients[actor]), "action", action, 200)
				after := s.rooms[id]
				if phase == "catan_transport_move" && after.Game.Phase == phase && after.TurnDeadline != deadline {
					t.Fatal("movement refreshed 120s window")
				}
			}
			r = s.rooms[id]
			if !r.Game.Finished || len(r.Game.Winners) != 1 || r.Game.Catan.Players[r.Game.Winners[0]].Score < 13 || !restored || moves == 0 {
				t.Fatal("incomplete transport match", steps, moves, r.Status)
			}
			if n == 2 && (!twoRestored["catan_two_build"] || !twoRestored["catan_two_trade"]) {
				t.Fatal("natural game did not cover both two-player responses", twoRestored)
			}
			assertTransportInventory(t, r.Game)
			if n > 4 && (!pairedMoves[false] || !pairedMoves[true]) {
				t.Fatal("both paired players must move their wagons", pairedMoves)
			}
			assertTransportHistory(t, s, clients, id)
			if events {
				if !eventRestored || r.Game.Catan.EventDeck == nil {
					t.Fatal("missing event response coverage", r.Game.Catan.RollID)
				}
				_, profile := clients[n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
				record := profile["history"].([]any)[0].(map[string]any)
				if record["catanExpansionRules"].(map[string]any)["event_cards"] != game.CatanEventCatalogue {
					t.Fatal("history lost transport events")
				}
			}
			t.Logf("%dp full HTTP match: %d actions, %d movement actions", n, steps, moves)
		})
	}
}

func TestCatanTransportHTTPAutomaticMovement(t *testing.T) {
	for _, mode := range []string{"autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id, _, _ := newFishingActionTable(t, 3, "catan_turn", "resource")
			state := transportHTTPFixture(t, 3)
			for state.Phase != "catan_roll" {
				a, e := state.BotAction(state.Turn)
				if e != nil {
					t.Fatal(e)
				}
				if e = state.Apply(state.Turn, a); e != nil {
					t.Fatal(e)
				}
			}
			// Response fixture: setup was played legally; skip only the production roll
			// to isolate automatic end movement. Full HTTP games above do not skip it.
			state.Phase = "catan_turn"
			actor := state.Turn
			s.mu.Lock()
			r := s.rooms[id]
			r.Game = state
			r.TurnDeadline = time.Now().Add(41 * time.Second).UnixMilli()
			r.CatanTimeLeft = 0
			if e := s.save(r); e != nil {
				t.Fatal(e)
			}
			s.mu.Unlock()
			clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_end"}, 200)
			r = s.rooms[id]
			deadline := r.TurnDeadline
			if r.Game.Phase != "catan_transport_move" || deadline-time.Now().UnixMilli() < 119000 {
				t.Fatal("movement window missing")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			if mode == "autoplay" {
				setAutoPlay(clients[actor], current(clients[actor]), true, 200)
			}
			now := time.Now()
			for i := 0; s.rooms[id].Game.Phase == "catan_transport_move"; i++ {
				if i > 20 {
					t.Fatal("movement automation stuck")
				}
				s.mu.Lock()
				if mode == "autoplay" {
					now = time.Now()
					s.rooms[id].BotAt = 0
					s.runBots(now)
				} else {
					now = time.UnixMilli(max(deadline, s.rooms[id].BotAt))
					s.expireSetups(now)
				}
				s.mu.Unlock()
				if s.rooms[id].Game.Phase == "catan_transport_move" && s.rooms[id].TurnDeadline != deadline {
					t.Fatal("automation refreshed movement timeout")
				}
			}
			r = s.rooms[id]
			if r.Game.Turn == actor || r.Game.Phase != "catan_roll" || r.Game.CatanPendingActor() != -1 || r.TurnDeadline-now.UnixMilli() < 119000 {
				t.Fatal("automatic movement did not hand off", r.Game.Phase)
			}
		})
	}
}

// Reach the response from a fresh constructor through legal actions. Return
// its preceding state so the HTTP command must actually start the 120s window.
func transportTwoBeforeResponse(t *testing.T, phase string) (*game.State, game.Action) {
	t.Helper()
	state := transportHTTPFixture(t, 2)
	for step := 0; step < 1500 && !state.Finished; step++ {
		actor := twoHTTPActor(state)
		a, err := state.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		if err = state.Apply(actor, a); err != nil {
			t.Fatal(step, state.Phase, a, err)
		}
		if state.Phase == phase {
			var before game.State
			if err = json.Unmarshal(data, &before); err != nil {
				t.Fatal(err)
			}
			return &before, a
		}
	}
	t.Fatal("natural transport game did not reach response", phase)
	return nil, game.Action{}
}

func TestCatanTransportTwoHTTPResponseAutomation(t *testing.T) {
	for _, phase := range []string{"catan_two_build", "catan_two_trade"} {
		before, action := transportTwoBeforeResponse(t, phase)
		data, err := json.Marshal(before)
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(phase+"/"+mode, func(t *testing.T) {
				s, ts, clients, id := newTwoFullTable(t)
				var state game.State
				if err := json.Unmarshal(data, &state); err != nil {
					t.Fatal(err)
				}
				actor := state.Turn
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = &state
				r.TurnDeadline = time.Now().Add(43 * time.Second).UnixMilli()
				r.CatanTimeLeft = 0
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				clients[actor].command(current(clients[actor]), "action", action, 200)
				r = s.rooms[id]
				if r.Game.Phase != phase || r.TurnDeadline-time.Now().UnixMilli() < 119000 || r.CatanTimeLeft < 42000 || r.CatanTimeLeft > 43000 {
					t.Fatal("response did not pause 43s action", r.Game.Phase, r.CatanTimeLeft)
				}
				saved := r.CatanTimeLeft
				deadline := r.TurnDeadline
				resume := ""
				if r.Game.Catan.Two.Pending != nil {
					resume = r.Game.Catan.Two.Pending.Resume
				} else {
					resume = r.Game.Catan.Two.Trade.Resume
				}
				assertTwoHTTPPrivacy(t, clients, r.Game)
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				if s.rooms[id].TurnDeadline != deadline || s.rooms[id].CatanTimeLeft != saved {
					t.Fatal("restart reset response clock")
				}
				now := time.Now()
				switch mode {
				case "manual":
					a, e := s.rooms[id].Game.BotAction(actor)
					if e != nil {
						t.Fatal(e)
					}
					clients[actor].command(current(clients[actor]), "action", a, 200)
					now = time.Now()
				case "autoplay":
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					now = time.Now()
					s.runBots(now)
					s.mu.Unlock()
				case "timeout":
					now = time.UnixMilli(max(deadline, s.rooms[id].BotAt))
					s.mu.Lock()
					s.expireSetups(now)
					s.mu.Unlock()
				}
				r = s.rooms[id]
				if r.Game.Phase != resume || r.Game.Turn != actor || r.Game.Catan.Two.Pending != nil || r.Game.Catan.Two.Trade != nil {
					t.Fatal("response did not resume", mode, r.Game.Phase, resume)
				}
				remaining := r.TurnDeadline - now.UnixMilli()
				if remaining < saved-1000 || remaining > saved+1000 {
					t.Fatal("response lost action time", mode, remaining, saved)
				}
			})
		}
	}
}

// Reach each marker's movement via real public HTTP actions, then verify that
// timeout and explicit autoplay hand off to the correct player with a new clock.
func TestCatanTransportPairedHTTPAutomaticMovement(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, second := range []bool{false, true} {
			for _, mode := range []string{"autoplay", "timeout"} {
				t.Run(fmt.Sprintf("%d/second-%t/%s", n, second, mode), func(t *testing.T) {
					s, ts, clients, id := newPublicScenarioTable(t, n, "transport")
					reached := false
					for step := 0; step < 600; step++ {
						g := s.rooms[id].Game
						if g.Phase == "catan_transport_move" && g.Catan.Paired.Second == second {
							reached = true
							break
						}
						actor := g.CatanPendingActor()
						if actor < 0 {
							actor = g.Turn
						}
						if g.Phase == "catan_discard" {
							for p, due := range g.Catan.DiscardDue {
								if due > 0 {
									actor = p
									break
								}
							}
						}
						a, err := g.BotAction(actor)
						if err != nil {
							t.Fatal(err)
						}
						clients[actor].command(current(clients[actor]), "action", a, 200)
					}
					if !reached {
						t.Fatal("paired movement unreachable")
					}
					r := s.rooms[id]
					actor := r.Game.Turn
					pair := *r.Game.Catan.Paired
					deadline := r.TurnDeadline
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					if s.rooms[id].TurnDeadline != deadline {
						t.Fatal("restart refreshed clock")
					}
					if mode == "autoplay" {
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					}
					now := time.Now()
					for step := 0; s.rooms[id].Game.Turn == actor; step++ {
						if step > 30 {
							t.Fatal("paired movement stuck")
						}
						s.mu.Lock()
						if mode == "autoplay" {
							now = time.Now()
							s.rooms[id].BotAt = 0
							s.runBots(now)
						} else {
							now = time.UnixMilli(max(deadline, s.rooms[id].BotAt))
							s.expireSetups(now)
						}
						s.mu.Unlock()
						if s.rooms[id].Game.Turn == actor && s.rooms[id].TurnDeadline != deadline {
							t.Fatal("movement refreshed clock")
						}
					}
					r = s.rooms[id]
					next, phase := pair.Secondary, "catan_turn"
					if second {
						next, phase = (pair.Primary+1)%n, "catan_roll"
					}
					if r.Game.Turn != next || r.Game.Phase != phase || r.TurnDeadline-now.UnixMilli() < 119000 {
						t.Fatal("wrong paired handoff", r.Game.Turn, r.Game.Phase)
					}
					if mode == "timeout" && !r.Seats[actor].AutoPlay {
						t.Fatal("timeout did not persist autoplay")
					}
					assertTransportInventory(t, r.Game)
				})
			}
		}
	}
}
