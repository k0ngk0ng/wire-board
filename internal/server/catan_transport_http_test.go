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
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
			state := transportHTTPFixture(t, n)
			s.mu.Lock()
			r := s.rooms[id]
			r.Game = state
			r.TurnDeadline = time.Now().Add(120 * time.Second).UnixMilli()
			r.CatanTimeLeft = 0
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			restored := false
			steps := 0
			moves := 0
			for ; steps < 4000 && !s.rooms[id].Game.Finished; steps++ {
				r = s.rooms[id]
				g := r.Game
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
				if phase == "catan_transport_move" {
					moves++
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
					now = time.UnixMilli(deadline)
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
