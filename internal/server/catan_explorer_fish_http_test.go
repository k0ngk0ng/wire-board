package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func newExplorerFishHTTP(t *testing.T) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts, clients, id := newExplorerHTTP(t, 3, false)
	// Exact normal constructor; artificial lair tokens remain a private test
	// input, never a verified production inventory. No resources are injected.
	raw, err := os.ReadFile("testdata/catan_explorer_fish_setup.json")
	if err != nil {
		t.Fatal(err)
	}
	var state game.State
	if err = json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	r := s.rooms[id]
	r.Game = &state
	r.startTurnClock(time.Now())
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	return s, ts, clients, id
}

func fishHTTPPreview(t *testing.T, view map[string]any, intent game.Action) game.Action {
	t.Helper()
	if intent.Type == "catan_discard" {
		return intent
	}
	x := view["game"].(map[string]any)["catan"].(map[string]any)["explorer"].(map[string]any)
	for _, raw := range x["choices"].([]any) {
		data, _ := json.Marshal(raw)
		var candidate game.Action
		if err := json.Unmarshal(data, &candidate); err != nil {
			t.Fatal(err)
		}
		// JSON previews may encode an empty transfer side as [] instead of
		// nil. Both are the same intent; still submit the actual wire choice.
		normalize := func(a game.Action) game.Action {
			for _, field := range []*[]int{&a.Cards, &a.Targets, &a.Give, &a.Take, &a.Tokens, &a.Keep} {
				if len(*field) == 0 {
					*field = nil
				}
			}
			return a
		}
		match := reflect.DeepEqual(normalize(intent), normalize(candidate))
		if intent.Type == "catan_explorer_sail" && candidate.Type == intent.Type && candidate.Slot == intent.Slot && len(candidate.Targets) > 0 && len(intent.Targets) > 0 {
			match = candidate.Targets[len(candidate.Targets)-1] == intent.Targets[len(intent.Targets)-1]
		}
		if match {
			return candidate
		}
	}
	t.Fatal("intent has no executable public preview", intent)
	return game.Action{}
}

func TestCatanExplorerFishNaturalManualHTTPMatch(t *testing.T) {
	s, ts, clients, id := newExplorerFishHTTP(t)
	seen := map[string]int{}
	restored := map[string]bool{}
	steps := 0
	for ; steps < 9000 && !s.rooms[id].Game.Finished; steps++ {
		r := s.rooms[id]
		actor := explorerHTTPActor(r.Game)
		intent, err := r.Game.BotAction(actor)
		if err != nil {
			t.Fatal(steps, err)
		}
		view := current(clients[actor])
		a := fishHTTPPreview(t, view, intent)
		fish := strings.HasPrefix(a.Type, "catan_explorer_fish_")
		if fish && !restored[a.Type] {
			before, _ := json.Marshal(r)
			clients[(actor+1)%3].command(current(clients[(actor+1)%3]), "action", a, 400)
			clients[3].command(current(clients[3]), "action", a, 400)
			bad := a
			bad.Prompt--
			clients[actor].command(current(clients[actor]), "action", bad, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("rejected fish request changed room")
			}
			assertExplorerFishHTTPPrivacy(t, clients, r.Game)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			after, _ = json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("restart changed fish action or clock")
			}
			restored[a.Type] = true
		}
		r = s.rooms[id]
		phase, serial, deadline, pending := r.Game.Phase, r.Game.Catan.TurnSerial, r.TurnDeadline, r.Game.CatanPendingActor()
		clients[actor].command(current(clients[actor]), "action", a, 200)
		seen[a.Type]++
		r = s.rooms[id]
		if fish {
			if !r.Game.Finished && (r.TurnDeadline != deadline || r.Game.Catan.TurnSerial != serial || r.Game.Phase != phase) {
				t.Fatal("fish operation refreshed or consumed turn clock", a.Type)
			}
			if r.Game.Catan.Explorer.Motion == nil || r.Game.Catan.Explorer.Motion.Kind != a.Type {
				t.Fatal("fish HTTP update omitted animation")
			}
			if seen[a.Type] == 1 {
				snapshot, _ := json.Marshal(r)
				// The same old room version is rejected even with a fresh request nonce.
				clients[actor].command(view, "action", a, 409)
				// A second invocation with a fresh version cannot roll twice, collect an
				// already-collected piece or deliver a returned piece.
				clients[actor].command(current(clients[actor]), "action", a, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(snapshot) != string(after) {
					t.Fatal("repeated fish request changed room")
				}
			}
		} else if !r.Game.Finished && pending < 0 && r.Game.CatanPendingActor() < 0 && r.Game.Phase != "catan_discard" && phase != "catan_discard" && r.Game.Catan.Explorer.Setup == nil && phase != "catan_explorer_setup" && r.Game.Catan.TurnSerial == serial && r.TurnDeadline != deadline {
			t.Fatal("ordinary movement or building renewed time", a.Type)
		}
		if steps%131 == 0 {
			assertExplorerFishHTTPPrivacy(t, clients, r.Game)
		}
	}
	r := s.rooms[id]
	if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || r.Game.Catan.Players[r.Game.Winners[0]].Score < 15 || r.TurnDeadline != 0 {
		t.Fatal("manual fish HTTP game did not finish", steps)
	}
	for _, kind := range []string{"catan_explorer_fish_roll", "catan_explorer_fish_load", "catan_explorer_fish_deliver", "catan_explorer_resolve"} {
		if seen[kind] == 0 {
			t.Fatal("natural HTTP game omitted mission operation", kind)
		}
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	assertExplorerFishHTTPPrivacy(t, clients, s.rooms[id].Game)
	if s.rooms[id].Status != "finished" || s.rooms[id].TurnDeadline != 0 {
		t.Fatal("result lost on restart")
	}
	t.Log("actions", steps, "fish rolls", seen["catan_explorer_fish_roll"], "loads", seen["catan_explorer_fish_load"], "deliveries", seen["catan_explorer_fish_deliver"])
}

// Reach a loaded delivery berth by normal legal actions, without adjusting
// resources, boats, fog or score. Only the selected boundary is exercised over
// HTTP below; the full manual-HTTP match above covers the complete wire path.
func fishHTTPDeliveryBoundary(t *testing.T, s *Server, id string) game.Action {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[id]
	for step := 0; step < 9000 && !r.Game.Finished; step++ {
		actor := explorerHTTPActor(r.Game)
		a, err := r.Game.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		if a.Type == "catan_explorer_fish_deliver" {
			r.TurnDeadline = time.Now().Add(37 * time.Second).UnixMilli()
			if err = s.save(r); err != nil {
				t.Fatal(err)
			}
			return a
		}
		if err = r.applyGameAction(actor, a, time.Now()); err != nil {
			t.Fatal(err)
		}
		r.Version++
	}
	t.Fatal("normal actions did not reach a fish delivery")
	return game.Action{}
}

func TestCatanExplorerFishHTTPExpiredMovementTakeoverAndRemoval(t *testing.T) {
	for _, mode := range []string{"takeover", "remove"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := newExplorerFishHTTP(t)
			a := fishHTTPDeliveryBoundary(t, s, id)
			r := s.rooms[id]
			actor := r.Game.Turn
			other := (actor + 1) % 3
			a = fishHTTPPreview(t, current(clients[actor]), a)
			target := r.Seats[actor].ID
			kick(clients[other], current(clients[other]), target, 400)
			expireTurn(t, s, id)
			before, _ := json.Marshal(s.rooms[id])
			s.mu.Lock()
			s.expireSetups(time.Now())
			s.mu.Unlock()
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("optional fishing was auto-executed on timeout")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			r = s.rooms[id]
			deadline := r.TurnDeadline
			if mode == "takeover" {
				stale := current(clients[actor])
				setAutoPlay(clients[actor], stale, true, 200)
				kick(clients[other], current(clients[other]), target, 400)
				clients[actor].command(current(clients[actor]), "action", a, 400)
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				r = s.rooms[id]
				deliveries := len(r.Game.Catan.Explorer.Fish.Deliveries)
				botTick(s, id)
				r = s.rooms[id]
				if len(r.Game.Catan.Explorer.Fish.Deliveries) != deliveries+1 || r.Game.Catan.Explorer.Cargo.Fish[a.Card].Kind != "supply" || r.TurnDeadline != deadline {
					t.Fatal("takeover lost cargo, failed delivery or renewed time")
				}
				setAutoPlay(clients[actor], stale, false, 200)
				if s.rooms[id].Seats[actor].AutoPlay || s.rooms[id].TurnDeadline != deadline {
					t.Fatal("reclaim changed deadline")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				if s.rooms[id].Seats[actor].AutoPlay || len(s.rooms[id].Game.Catan.Explorer.Fish.Deliveries) != deliveries+1 {
					t.Fatal("restart lost delivery or manual control")
				}
			} else {
				kick(clients[3], current(clients[3]), target, 400)
				old := s.rooms[id].Game.Catan.Explorer
				kick(clients[other], current(clients[other]), target, 200)
				r = s.rooms[id]
				x := r.Game.Catan.Explorer
				if !r.Seats[actor].Left || !x.Fish.Retired[actor] || r.Game.Turn == actor {
					t.Fatal("loaded player was not retired")
				}
				for i, loc := range old.Cargo.Fish {
					owned := loc.Kind == "ship" && loc.Index/3 == actor || loc.Kind == "harbor" && r.Game.Catan.Vertices[loc.Index].Owner == actor
					if owned {
						if x.Cargo.Fish[i].Kind != "supply" {
							t.Fatal("departing player's fish remained on board")
						}
					} else if x.Cargo.Fish[i] != loc {
						t.Fatal("departure changed other player's or public fish")
					}
				}
				if !reflect.DeepEqual(old.Fish.Deliveries, x.Fish.Deliveries) {
					t.Fatal("departure erased delivery history")
				}
				serial := r.Game.Catan.TurnSerial
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				for step := 0; step < 300 && s.rooms[id].Game.Catan.TurnSerial < serial+3; step++ {
					r = s.rooms[id]
					p := explorerHTTPActor(r.Game)
					if p == actor {
						t.Fatal("departed player was selected again")
					}
					a, err := r.Game.BotAction(p)
					if err != nil {
						t.Fatal(err)
					}
					if r.Game.Phase == "catan_turn" {
						a = game.Action{Type: "catan_explorer_begin_move", Prompt: int(r.Game.Catan.TurnSerial)}
					}
					if r.Game.Phase == "catan_explorer_move" {
						a = game.Action{Type: "catan_end", Prompt: int(r.Game.Catan.TurnSerial)}
					}
					clients[p].command(current(clients[p]), "action", a, 200)
				}
				if s.rooms[id].Game.Catan.TurnSerial < serial+3 {
					t.Fatal("remaining players could not continue")
				}
			}
		})
	}
}

func TestCatanExplorerFishHTTPSetupTimeoutAndRestart(t *testing.T) {
	s, ts, clients, id := newExplorerFishHTTP(t)
	first := s.rooms[id].Game.Turn
	for step := 0; s.rooms[id].Game.Catan.Explorer.Setup != nil; step++ {
		if step >= 12 {
			t.Fatal("fish opening timeout stalled")
		}
		r := s.rooms[id]
		deadline := r.TurnDeadline
		now := time.UnixMilli(deadline)
		s.mu.Lock()
		s.expireSetups(now)
		s.mu.Unlock()
		r = s.rooms[id]
		if r.TurnDeadline != now.Add(120*time.Second).UnixMilli() || r.Version < 1 {
			t.Fatal("next setup step did not receive 120 seconds")
		}
		saved, _ := json.Marshal(r)
		s, ts = restartRiversHTTP(t, s, ts, clients, id)
		restored, _ := json.Marshal(s.rooms[id])
		if string(saved) != string(restored) {
			t.Fatal("restart lost setup pieces or clock")
		}
	}
	r := s.rooms[id]
	if r.Game.Phase != "catan_roll" || r.Game.Turn != first || r.Game.Catan.TurnSerial != 1 || len(r.Game.Catan.Explorer.Cargo.Fish) != 6 {
		t.Fatal("fish opening did not enter first ordinary turn")
	}
	assertExplorerFishHTTPPrivacy(t, clients, r.Game)
}
