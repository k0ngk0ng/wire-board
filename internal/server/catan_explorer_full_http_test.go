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

func TestCatanExplorerFullNaturalManualHTTPMatch(t *testing.T) {
	s, ts, clients, id := newExplorerFullHTTP(t)
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
		response := a.Type == "catan_explorer_resolve" || a.Type == "catan_explorer_battle"
		mission := response || a.Type == "catan_explorer_land" || a.Type == "catan_explorer_pickup" || strings.HasPrefix(a.Type, "catan_explorer_fish_") || strings.HasPrefix(a.Type, "catan_explorer_spice_")
		if mission && !restored[a.Type] {
			before, _ := json.Marshal(r)
			clients[(actor+1)%3].command(current(clients[(actor+1)%3]), "action", a, 400)
			clients[3].command(current(clients[3]), "action", a, 400)
			bad := a
			bad.Prompt--
			clients[actor].command(current(clients[actor]), "action", bad, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("rejected full mission request changed room")
			}
			assertExplorerSpiceHTTPPrivacy(t, clients, r.Game)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			after, _ = json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("restart changed full mission action or clock")
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
			if !response && !r.Game.Finished && (r.TurnDeadline != deadline || r.Game.Catan.TurnSerial != serial || r.Game.Phase != phase) {
				t.Fatal("mission operation refreshed or consumed turn clock", a.Type)
			}
			if a.Type != "catan_explorer_spice_gold" && (r.Game.Catan.Explorer.Motion == nil || r.Game.Catan.Explorer.Motion.Kind != a.Type) {
				t.Fatal("mission HTTP update omitted animation")
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
				// already-collected piece or deliver a returned piece. Gold may have
				// a second farm use; tied hero dice may legitimately be rolled again.
				if a.Type != "catan_explorer_spice_gold" && a.Type != "catan_explorer_battle" {
					clients[actor].command(current(clients[actor]), "action", a, 400)
				}
				after, _ := json.Marshal(s.rooms[id])
				if string(snapshot) != string(after) {
					t.Fatal("repeated full mission request changed room")
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
	if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || r.Game.Catan.Players[r.Game.Winners[0]].Score < 17 || r.TurnDeadline != 0 {
		t.Fatal("manual full HTTP game did not finish", steps)
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
	t.Log("actions", steps, "fish rolls", seen["catan_explorer_fish_roll"], "loads", seen["catan_explorer_fish_load"], "deliveries", seen["catan_explorer_fish_deliver"], "spice land", seen["catan_explorer_spice_land"], "spice delivery", seen["catan_explorer_spice_deliver"], "spice gold", seen["catan_explorer_spice_gold"], "lair land", seen["catan_explorer_land"], "lair pickup", seen["catan_explorer_pickup"], "lair resolve", seen["catan_explorer_resolve"], "hero rolls", seen["catan_explorer_battle"])
}

func TestCatanExplorerFullHTTPExpiredMovementTakeoverAndRemoval(t *testing.T) {
	explorerHTTPExpiredMovementTakeoverAndRemoval(t, newExplorerFullHTTP)
}

func TestCatanExplorerFullHTTPSetupTimeoutAndRestart(t *testing.T) {
	explorerHTTPSetupTimeoutAndRestart(t, newExplorerFullHTTP)
}

func fullHTTPResponse(t *testing.T) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts, clients, id := newExplorerFullHTTP(t)
	// Independently validated controlled boundary, not a naturally reached board.
	raw, err := os.ReadFile("testdata/catan_explorer_full_resolve.json")
	if err != nil {
		t.Fatal(err)
	}
	var state game.State
	if err = json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s.mu.Lock()
	r := s.rooms[id]
	r.Game = &state
	r.TurnDeadline = now.Add(37 * time.Second).UnixMilli()
	r.CatanPendingVersion = r.Version
	if !r.adjustCatanResponseClock("catan_explorer_move", -1, r.Game.Catan.SetupStep, now) {
		t.Fatal("full response not recognized")
	}
	if r.TurnDeadline != now.Add(120*time.Second).UnixMilli() {
		t.Fatal("response lacks 120 seconds")
	}
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	return s, ts, clients, id
}

func TestCatanExplorerFullHTTPResponseClocksAndReplay(t *testing.T) {
	for _, mode := range []string{"manual", "autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := fullHTTPResponse(t)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			if mode == "autoplay" {
				for p := 0; p < 3; p++ {
					setAutoPlay(clients[p], current(clients[p]), true, 200)
				}
			}
			r := s.rooms[id]
			actor, serial, originalDeadline := r.Game.Turn, r.Game.Catan.TurnSerial, r.TurnDeadline
			before := r.Game.Catan.Explorer
			seen := map[string]int{}
			for step := 0; s.rooms[id].Game.Phase != "catan_roll"; step++ {
				if step >= 60 {
					t.Fatal("full response stalled")
				}
				r = s.rooms[id]
				phase := r.Game.Phase
				if phase != "catan_explorer_resolve" && phase != "catan_explorer_battle" {
					t.Fatal("unexpected response", phase)
				}
				seen[phase]++
				if r.Game.CatanPendingActor() != actor || r.Game.Catan.TurnSerial != serial || r.TurnDeadline != originalDeadline {
					t.Fatal("response actor/serial/deadline changed")
				}
				assertExplorerSpiceHTTPPrivacy(t, clients, r.Game)
				a, err := r.Game.BotAction(actor)
				if err != nil {
					t.Fatal(err)
				}
				a = fishHTTPPreview(t, current(clients[actor]), a)
				now := time.Now()
				if mode == "manual" {
					saved, _ := json.Marshal(r)
					clients[(actor+1)%3].command(current(clients[(actor+1)%3]), "action", a, 400)
					clients[3].command(current(clients[3]), "action", a, 400)
					bad := a
					bad.Prompt--
					clients[actor].command(current(clients[actor]), "action", bad, 400)
					after, _ := json.Marshal(s.rooms[id])
					if string(saved) != string(after) {
						t.Fatal("rejected response mutated room")
					}
					view := current(clients[actor])
					request := map[string]any{"type": "action", "action": a, "version": view["version"], "nonce": randomID(12)}
					clients[actor].post("/api/rooms/"+id, request, 200)
					now = time.Now()
					saved, _ = json.Marshal(s.rooms[id])
					clients[actor].post("/api/rooms/"+id, request, 200)
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					clients[actor].post("/api/rooms/"+id, request, 200)
					clients[actor].command(view, "action", a, 409)
					// A tied hero roll permits a new roll with a new nonce/version; resolve
					// must never pay contributors twice even when using the latest version.
					if a.Type == "catan_explorer_resolve" {
						clients[actor].command(current(clients[actor]), "action", a, 400)
					}
					after, _ = json.Marshal(s.rooms[id])
					if string(saved) != string(after) {
						t.Fatal("response replay mutated room")
					}
				} else {
					s.mu.Lock()
					if mode == "autoplay" {
						r.BotAt = 0
						s.runBots(now)
					} else {
						now = time.UnixMilli(max(originalDeadline, s.rooms[id].BotAt))
						s.expireSetups(now)
					}
					s.mu.Unlock()
				}
				r = s.rooms[id]
				if r.Game.Phase == "catan_roll" {
					remaining := r.TurnDeadline - now.UnixMilli()
					if remaining < 119000 || remaining > 120000 {
						t.Fatal("next turn lacks fresh 120 seconds", remaining)
					}
				} else if r.TurnDeadline != originalDeadline {
					t.Fatal("hero roll refreshed shared response deadline")
				}
				saved, _ := json.Marshal(r)
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				after, _ := json.Marshal(s.rooms[id])
				if string(saved) != string(after) {
					t.Fatal("restart changed full response or clock")
				}
			}
			r = s.rooms[id]
			x := r.Game.Catan.Explorer
			if r.Game.Turn != (actor+1)%3 || r.Game.Catan.TurnSerial != serial+1 || r.CatanTimeLeft != 0 || seen["catan_explorer_resolve"] != 1 || mode != "timeout" && seen["catan_explorer_battle"] == 0 {
				t.Fatal("response failed to finish once", seen)
			}
			if !reflect.DeepEqual(before.Cargo.Spice, x.Cargo.Spice) || !reflect.DeepEqual(before.Cargo.Fish, x.Cargo.Fish) || !reflect.DeepEqual(before.Spice, x.Spice) || !reflect.DeepEqual(before.Fish, x.Fish) {
				t.Fatal("battle changed fish or spice state")
			}
			for p := 0; p < 3; p++ {
				want := before.Economy.Gold[p]
				if p == actor || p == (actor+1)%3 {
					want += 2
				}
				if x.Economy.Gold[p] != want {
					t.Fatal("incorrect/repeated contribution reward", p)
				}
			}
			resolved := 0
			for _, site := range x.Lairs.Sites {
				if site.Resolved > 0 {
					resolved++
				}
			}
			if resolved != 1 || x.Lairs.Battle != nil {
				t.Fatal("lair not fully resolved")
			}
			assertExplorerSpiceHTTPPrivacy(t, clients, r.Game)
			t.Log(seen)
		})
	}
}

func TestCatanExplorerFullHTTPKickCannotInterruptResponse(t *testing.T) {
	for _, phase := range []string{"resolve", "battle"} {
		t.Run(phase, func(t *testing.T) {
			s, ts, clients, id := fullHTTPResponse(t)
			r := s.rooms[id]
			actor := r.Game.Turn
			if phase == "battle" {
				a, err := r.Game.BotAction(actor)
				if err != nil {
					t.Fatal(err)
				}
				clients[actor].command(current(clients[actor]), "action", fishHTTPPreview(t, current(clients[actor]), a), 200)
			}
			r = s.rooms[id]
			kick(clients[(actor+1)%3], current(clients[(actor+1)%3]), r.Seats[actor].ID, 400)
			expireTurn(t, s, id)
			before, _ := json.Marshal(s.rooms[id].Game)
			// Timeout advances the mandatory chain before applying the kick command,
			// so its old room version must lose the race without removing any seat.
			kick(clients[(actor+1)%3], current(clients[(actor+1)%3]), r.Seats[actor].ID, 409)
			r = s.rooms[id]
			for p, seat := range r.Seats {
				if seat.Left || r.Game.Catan.Players[p].Eliminated {
					t.Fatal("response race removed a player")
				}
			}
			after, _ := json.Marshal(r.Game)
			if string(before) == string(after) {
				t.Fatal("mandatory timeout did not advance")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			assertExplorerSpiceHTTPPrivacy(t, clients, s.rooms[id].Game)
		})
	}
}
