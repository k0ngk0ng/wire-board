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

func TestCatanExplorerSpiceHTTPGoldAndMixedTransfer(t *testing.T) {
	for _, name := range []string{"gold", "transfer"} {
		t.Run(name, func(t *testing.T) {
			s, ts, clients, id := newExplorerSpiceHTTP(t)
			// Controlled legal boundary snapshots, independently validated in game
			// tests; natural gameplay is covered by the complete HTTP match below.
			raw, err := os.ReadFile("testdata/catan_explorer_spice_actions.json")
			if err != nil {
				t.Fatal(err)
			}
			var states map[string]*game.State
			if err = json.Unmarshal(raw, &states); err != nil {
				t.Fatal(err)
			}
			s.mu.Lock()
			r := s.rooms[id]
			r.Game = states[name]
			r.startTurnClock(time.Now())
			r.TurnDeadline = time.Now().Add(37 * time.Second).UnixMilli()
			err = s.save(r)
			s.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			actor, deadline := r.Game.Turn, r.TurnDeadline
			g, x := r.Game.Catan, r.Game.Catan.Explorer
			gold, goldBank, woodBank := x.Economy.Gold[actor], x.Economy.GoldBank, g.Bank[0]
			a := game.Action{Type: "catan_explorer_spice_gold", Card: 0, Prompt: int(g.TurnSerial)}
			if name == "transfer" {
				sacks := []int{}
				harbor := -1
				for i, sack := range x.Cargo.Spice {
					if sack.Owner == actor && sack.At.Kind == "harbor" {
						sacks = append(sacks, i)
						harbor = sack.At.Index
					}
				}
				if len(sacks) != 2 {
					t.Fatal("missing stored sacks")
				}
				a = game.Action{Type: "catan_explorer_transfer", Slot: actor * 3, Vertex: harbor, Take: []int{actor * 11}, SpiceLoad: sacks, Prompt: int(g.TurnSerial)}
			}
			a = fishHTTPPreview(t, current(clients[actor]), a)
			before, _ := json.Marshal(r)
			clients[(actor+1)%3].command(current(clients[(actor+1)%3]), "action", a, 400)
			clients[3].command(current(clients[3]), "action", a, 400)
			bad := a
			bad.Prompt--
			clients[actor].command(current(clients[actor]), "action", bad, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid boundary request changed state")
			}
			request := map[string]any{"type": "action", "action": a, "version": current(clients[actor])["version"], "nonce": randomID(12)}
			clients[actor].post("/api/rooms/"+id, request, 200)
			snapshot, _ := json.Marshal(s.rooms[id])
			clients[actor].post("/api/rooms/"+id, request, 200)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			clients[actor].post("/api/rooms/"+id, request, 200)
			stale := map[string]any{"type": "action", "action": a, "version": request["version"], "nonce": randomID(12)}
			clients[actor].post("/api/rooms/"+id, stale, 409)
			after, _ = json.Marshal(s.rooms[id])
			if string(snapshot) != string(after) {
				t.Fatal("replayed request changed room")
			}
			r = s.rooms[id]
			g, x = r.Game.Catan, r.Game.Catan.Explorer
			if r.TurnDeadline != deadline {
				t.Fatal("boundary action reset clock")
			}
			if name == "gold" {
				if g.Players[actor].Resources[0] != 2 || g.Bank[0] != woodBank+1 || x.Economy.Gold[actor] != gold+1 || x.Economy.GoldBank != goldBank-1 || x.Spice.GoldUse.Count != 1 {
					t.Fatal("first exchange or replay charged wrong amount")
				}
				// A new request is legitimate: the second owned farm has its own use.
				a = fishHTTPPreview(t, current(clients[actor]), a)
				clients[actor].command(current(clients[actor]), "action", a, 200)
				r = s.rooms[id]
				g, x = r.Game.Catan, r.Game.Catan.Explorer
				if g.Players[actor].Resources[0] != 1 || g.Bank[0] != woodBank+2 || x.Economy.Gold[actor] != gold+2 || x.Economy.GoldBank != goldBank-2 || x.Spice.GoldUse.Count != 2 || r.TurnDeadline != deadline {
					t.Fatal("second farm exchange failed")
				}
				snapshot, _ = json.Marshal(r)
				clients[actor].command(current(clients[actor]), "action", a, 400)
			} else {
				settler := x.Cargo.Units[actor*11]
				if settler.Kind != "harbor" || settler.Index != a.Vertex {
					t.Fatal("settler did not unload")
				}
				m := x.Motion
				if m == nil || m.Kind != a.Type || m.Player != actor || m.Ship != a.Slot || m.Vertex != a.Vertex || len(m.Spice) != 2 || len(m.Cargo) != 1 {
					t.Fatal("missing mixed cargo animation")
				}
				for _, id := range a.SpiceLoad {
					sack := x.Cargo.Spice[id]
					if sack.At.Kind != "ship" || sack.At.Index != a.Slot || sack.Owner != actor {
						t.Fatal("spice did not load")
					}
					found := false
					for _, motion := range m.Spice {
						if motion.Sack == id {
							found = true
							if motion.From.Kind != "harbor" || motion.From.Index != a.Vertex || motion.To != sack.At {
								t.Fatal("wrong spice animation endpoints")
							}
						}
					}
					if !found {
						t.Fatal("missing sack animation")
					}
				}
				if m.Cargo[0].Unit != actor*11 || m.Cargo[0].From.Kind != "ship" || m.Cargo[0].From.Index != a.Slot || m.Cargo[0].To != settler {
					t.Fatal("wrong settler animation endpoints")
				}
				for _, client := range clients {
					public := current(client)["game"].(map[string]any)["catan"].(map[string]any)["explorer"].(map[string]any)["motion"]
					want, _ := json.Marshal(m)
					got, _ := json.Marshal(public)
					var decoded any
					if err := json.Unmarshal(want, &decoded); err != nil {
						t.Fatal(err)
					}
					want, _ = json.Marshal(decoded)
					if string(got) != string(want) {
						t.Fatal("player or watcher lost cargo motion")
					}
				}
				clients[actor].command(current(clients[actor]), "action", a, 400)
			}
			after, _ = json.Marshal(s.rooms[id])
			if string(snapshot) != string(after) {
				t.Fatal("illegal repeat changed room")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			after, _ = json.Marshal(s.rooms[id])
			if string(snapshot) != string(after) {
				t.Fatal("restart lost boundary state")
			}
			assertExplorerSpiceHTTPPrivacy(t, clients, s.rooms[id].Game)
		})
	}
}

func TestCatanExplorerSpiceNaturalManualHTTPMatch(t *testing.T) {
	s, ts, clients, id := newExplorerSpiceHTTP(t)
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
		mission := strings.HasPrefix(a.Type, "catan_explorer_fish_") || strings.HasPrefix(a.Type, "catan_explorer_spice_")
		if mission && !restored[a.Type] {
			before, _ := json.Marshal(r)
			clients[(actor+1)%3].command(current(clients[(actor+1)%3]), "action", a, 400)
			clients[3].command(current(clients[3]), "action", a, 400)
			bad := a
			bad.Prompt--
			clients[actor].command(current(clients[actor]), "action", bad, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("rejected spice request changed room")
			}
			assertExplorerSpiceHTTPPrivacy(t, clients, r.Game)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			after, _ = json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("restart changed spice action or clock")
			}
			restored[a.Type] = true
		}
		r = s.rooms[id]
		phase, serial, deadline, pending := r.Game.Phase, r.Game.Catan.TurnSerial, r.TurnDeadline, r.Game.CatanPendingActor()
		request := map[string]any{"type": "action", "action": a, "version": current(clients[actor])["version"], "nonce": randomID(12)}
		clients[actor].post("/api/rooms/"+id, request, 200)
		seen[a.Type]++
		r = s.rooms[id]
		if mission {
			if !r.Game.Finished && (r.TurnDeadline != deadline || r.Game.Catan.TurnSerial != serial || r.Game.Phase != phase) {
				t.Fatal("spice operation refreshed or consumed turn clock", a.Type)
			}
			if a.Type != "catan_explorer_spice_gold" && (r.Game.Catan.Explorer.Motion == nil || r.Game.Catan.Explorer.Motion.Kind != a.Type) {
				t.Fatal("spice HTTP update omitted animation")
			}
			if seen[a.Type] == 1 {
				snapshot, _ := json.Marshal(r)
				clients[actor].post("/api/rooms/"+id, request, 200)
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				clients[actor].post("/api/rooms/"+id, request, 200)
				r = s.rooms[id]
				// The same old room version is rejected even with a fresh request nonce.
				clients[actor].command(view, "action", a, 409)
				// A second invocation with a fresh version cannot roll twice, collect an
				// already-collected piece or deliver a returned piece.
				if a.Type != "catan_explorer_spice_gold" {
					clients[actor].command(current(clients[actor]), "action", a, 400)
				}
				after, _ := json.Marshal(s.rooms[id])
				if string(snapshot) != string(after) {
					t.Fatal("repeated spice request changed room")
				}
			}
		} else if !r.Game.Finished && pending < 0 && r.Game.CatanPendingActor() < 0 && r.Game.Phase != "catan_discard" && phase != "catan_discard" && r.Game.Catan.Explorer.Setup == nil && phase != "catan_explorer_setup" && r.Game.Catan.TurnSerial == serial && r.TurnDeadline != deadline {
			t.Fatal("ordinary movement or building renewed time", a.Type)
		}
		if steps%131 == 0 {
			assertExplorerSpiceHTTPPrivacy(t, clients, r.Game)
		}
	}
	r := s.rooms[id]
	if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || r.Game.Catan.Players[r.Game.Winners[0]].Score < 15 || r.TurnDeadline != 0 {
		t.Fatal("manual spice HTTP game did not finish", steps)
	}
	for _, kind := range []string{"catan_explorer_fish_roll", "catan_explorer_fish_load", "catan_explorer_fish_deliver", "catan_explorer_spice_land", "catan_explorer_spice_deliver"} {
		if seen[kind] == 0 {
			t.Fatal("natural HTTP game omitted mission operation", kind)
		}
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	assertExplorerSpiceHTTPPrivacy(t, clients, s.rooms[id].Game)
	if s.rooms[id].Status != "finished" || s.rooms[id].TurnDeadline != 0 {
		t.Fatal("result lost on restart")
	}
	t.Log("actions", steps, "fish rolls", seen["catan_explorer_fish_roll"], "loads", seen["catan_explorer_fish_load"], "deliveries", seen["catan_explorer_fish_deliver"], "spice land", seen["catan_explorer_spice_land"], "spice delivery", seen["catan_explorer_spice_deliver"], "spice gold", seen["catan_explorer_spice_gold"])
}

// Reach a loaded delivery berth by normal legal actions, without adjusting
// resources, boats, fog or score. Only the selected boundary is exercised over
// HTTP below; the full manual-HTTP match above covers the complete wire path.
func spiceHTTPDeliveryBoundary(t *testing.T, s *Server, id string) game.Action {
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
		if a.Type == "catan_explorer_spice_deliver" {
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
	t.Fatal("normal actions did not reach a spice delivery")
	return game.Action{}
}

func TestCatanExplorerSpiceHTTPExpiredMovementTakeoverAndRemoval(t *testing.T) {
	explorerHTTPExpiredMovementTakeoverAndRemoval(t, newExplorerSpiceHTTP)
}

func explorerHTTPExpiredMovementTakeoverAndRemoval(t *testing.T, create func(*testing.T) (*Server, *httptest.Server, []*testClient, string)) {
	t.Helper()
	for _, mode := range []string{"takeover", "legacy_departure"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := create(t)
			a := spiceHTTPDeliveryBoundary(t, s, id)
			r := s.rooms[id]
			actor := r.Game.Turn
			other := (actor + 1) % 3
			a = fishHTTPPreview(t, current(clients[actor]), a)
			target := r.Seats[actor].ID
			kick(clients[other], current(clients[other]), target, 400)
			if mode == "takeover" {
				deliveries := len(r.Game.Catan.Explorer.Spice.Deliveries)
				expireTurn(t, s, id)
				deadline := s.rooms[id].TurnDeadline
				s.mu.Lock()
				s.expireSetups(time.Now())
				s.mu.Unlock()
				r = s.rooms[id]
				if !r.Seats[actor].AutoPlay || !r.Seats[actor].TimeoutAutoPlay || len(r.Game.Catan.Explorer.Spice.Deliveries) != deliveries+1 || r.Game.Catan.Explorer.Cargo.Spice[a.Card].At.Kind != "supply" || r.TurnDeadline != deadline {
					t.Fatal("timeout failed to deliver cargo using the computer strategy")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				stale := current(clients[actor])
				kick(clients[other], current(clients[other]), target, 400)
				clients[actor].command(current(clients[actor]), "action", a, 400)
				setAutoPlay(clients[actor], stale, false, 200)
				if s.rooms[id].Seats[actor].AutoPlay || time.Until(time.UnixMilli(s.rooms[id].TurnDeadline)) < 119*time.Second {
					t.Fatal("reclaim did not provide a usable manual window")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				if s.rooms[id].Seats[actor].AutoPlay || len(s.rooms[id].Game.Catan.Explorer.Spice.Deliveries) != deliveries+1 {
					t.Fatal("restart lost delivery or manual control")
				}
			} else {
				kick(clients[3], current(clients[3]), target, 400)
				old := s.rooms[id].Game.Catan.Explorer
				legacyCatanDeparture(t, s, id, actor)
				r = s.rooms[id]
				x := r.Game.Catan.Explorer
				if !r.Seats[actor].Left || !x.Fish.Retired[actor] || x.Lairs != nil && !x.Lairs.Retired[actor] || r.Game.Turn == actor {
					t.Fatal("loaded player was not retired")
				}
				for i, sack := range old.Cargo.Spice {
					owned := sack.Owner == actor
					if owned {
						if x.Cargo.Spice[i].At.Kind != "supply" || x.Cargo.Spice[i].Owner != sack.Owner || x.Cargo.Spice[i].Origin != sack.Origin {
							t.Fatal("departing player's spice remained on board")
						}
					} else if x.Cargo.Spice[i] != sack {
						t.Fatal("departure changed other player's or unclaimed spice")
					}
				}
				for unit := actor * 11; unit < (actor+1)*11; unit++ {
					if x.Cargo.Units[unit].Kind != "supply" {
						t.Fatal("departed permanent crew not returned")
					}
				}
				for ship, position := range old.Fleet.Positions {
					if ship/3 == actor {
						if x.Fleet.Positions[ship] != -1 {
							t.Fatal("departed ship remains")
						}
					} else if x.Fleet.Positions[ship] != position {
						t.Fatal("other ship changed")
					}
				}
				for unit, loc := range old.Cargo.Units {
					if unit/11 != actor && x.Cargo.Units[unit] != loc {
						t.Fatal("other crew changed")
					}
				}
				for id, loc := range old.Cargo.Fish {
					owned := loc.Kind == "ship" && loc.Index/3 == actor || loc.Kind == "harbor" && r.Game.Catan.Vertices[loc.Index].Owner == actor
					if owned {
						if x.Cargo.Fish[id].Kind != "supply" {
							t.Fatal("departed fish cargo remains")
						}
					} else if x.Cargo.Fish[id] != loc {
						t.Fatal("other fish cargo changed")
					}
				}
				if !reflect.DeepEqual(old.Fish.Deliveries, x.Fish.Deliveries) {
					t.Fatal("departure erased fish history")
				}
				if x.Lairs != nil {
					if !reflect.DeepEqual(old.Lairs.Progress, x.Lairs.Progress) {
						t.Fatal("departure erased lair progress")
					}
					for i, site := range old.Lairs.Sites {
						if site.Resolved > 0 && !reflect.DeepEqual(site, x.Lairs.Sites[i]) {
							t.Fatal("departure changed resolved lair history")
						}
					}
				}
				if !reflect.DeepEqual(old.Spice.Deliveries, x.Spice.Deliveries) {
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

func TestCatanExplorerSpiceHTTPSetupTimeoutAndRestart(t *testing.T) {
	explorerHTTPSetupTimeoutAndRestart(t, newExplorerSpiceHTTP)
}

func explorerHTTPSetupTimeoutAndRestart(t *testing.T, create func(*testing.T) (*Server, *httptest.Server, []*testClient, string)) {
	t.Helper()
	s, ts, clients, id := create(t)
	first := s.rooms[id].Game.Turn
	for step := 0; s.rooms[id].Game.Catan.Explorer.Setup != nil; step++ {
		if step >= 12 {
			t.Fatal("spice opening timeout stalled")
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
		t.Fatal("spice opening did not enter first ordinary turn")
	}
	assertExplorerSpiceHTTPPrivacy(t, clients, r.Game)
}
