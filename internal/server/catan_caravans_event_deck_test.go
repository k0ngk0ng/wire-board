package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanCaravansEventDeckHTTPProductionAndVote(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				var s *Server
				var ts *httptest.Server
				var clients []*testClient
				var id string
				if n == 2 {
					s, ts, clients, id = newTwoScenarioFullTable(t, "caravans")
				} else {
					s, ts, clients, id = newCaravansFullTable(t, n)
				}
				attachReferenceEventDeck(t, s.rooms[id].Game) // Explicit internal catalogue/order fixture, not a public option.
				for s.rooms[id].Game.Catan.SetupStep < s.rooms[id].Game.Catan.SetupLimit() {
					state := s.rooms[id].Game
					a, err := state.BotAction(state.Turn)
					if err != nil {
						t.Fatal(err)
					}
					clients[state.Turn].command(current(clients[state.Turn]), "action", a, 200)
				}
				r := s.rooms[id]
				r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				modes := map[string]int{}
				clockAt := time.Now()
				act := func(actor int, a game.Action, automatic bool) {
					t.Helper()
					r := s.rooms[id]
					before, _ := json.Marshal(r)
					clients[n].command(current(clients[n]), "action", a, 400)
					clients[(actor+1)%n].command(current(clients[(actor+1)%n]), "action", a, 400)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("unauthorized action changed room or time")
					}
					request := map[string]any{"type": "action", "action": a, "version": r.Version, "nonce": randomID(12)}
					selected := mode
					if !automatic {
						selected = "manual"
					}
					clockAt = time.Now()
					switch selected {
					case "manual":
						clients[actor].post("/api/rooms/"+id, request, 200)
					case "autoplay":
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
						s.mu.Lock()
						s.rooms[id].BotAt = 0
						s.runBots(time.Now())
						s.mu.Unlock()
					case "timeout":
						s.mu.Lock()
						clockAt = time.UnixMilli(s.rooms[id].TurnDeadline)
						s.expireSetups(clockAt)
						s.mu.Unlock()
						if !s.rooms[id].Seats[actor].TimeoutAutoPlay {
							t.Fatal("missing timeout takeover")
						}
					}
					if s.rooms[id].Version <= r.Version {
						t.Fatal("action stalled", a)
					}
					if s.rooms[id].Seats[actor].AutoPlay {
						setAutoPlay(clients[actor], current(clients[actor]), false, 200)
					}
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					if selected == "manual" {
						before, _ = json.Marshal(s.rooms[id])
						clients[actor].post("/api/rooms/"+id, request, 200)
						after, _ = json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("replay changed state")
						}
					}
					state := s.rooms[id].Game
					assertCaravansFullInventory(t, state)
					for viewer, c := range clients {
						v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
						deck := v["eventDeck"].(map[string]any)
						if deck["drawPile"] != nil || deck["deck"] != nil || deck["referenceOnly"] != true {
							t.Fatal("hidden or unlabelled reference deck")
						}
						for p, raw := range v["players"].([]any) {
							if (raw.(map[string]any)["resources"] != nil) != (viewer == p) {
								t.Fatal("private hand exposed")
							}
						}
					}
					modes[a.Type]++
				}
				owner := s.rooms[id].Game.Turn
				wantDraws := 1
				if n == 2 {
					wantDraws = 2
				}
				for draw := 1; draw <= wantDraws; draw++ {
					act(owner, game.Action{Type: "catan_roll"}, false)
					for step := 0; s.rooms[id].Game.CatanPendingActor() >= 0 && step < 20; step++ {
						state := s.rooms[id].Game
						actor := state.CatanPendingActor()
						a, err := state.BotAction(actor)
						if err != nil {
							t.Fatal(err)
						}
						if delta := s.rooms[id].TurnDeadline - clockAt.UnixMilli(); delta < 119000 || delta > 121000 {
							t.Fatal("response window", delta)
						}
						act(actor, a, true)
						if s.rooms[id].Game.Catan.RollID != draw {
							t.Fatal("event response drew again")
						}
					}
				}
				if s.rooms[id].Game.Phase != "catan_turn" || modes["catan_event_resource"] < n {
					t.Fatal("production not complete")
				}
				stages := 1
				if n == 6 {
					stages = 2
				}
				for stage := 0; stage < stages; stage++ {
					r = s.rooms[id]
					state, g := r.Game, r.Game.Catan
					owner = state.Turn
					// Explicit bank-conserving construction fixture after real setup/production.
					// The upgrade and all bids/placements themselves use production HTTP.
					for color, want := range []int{0, 0, 0, 2, 3} {
						g.Bank[color] += g.Players[owner].Resources[color] - want
						g.Players[owner].Resources[color] = want
					}
					vertex := -1
					for _, v := range g.Vertices {
						if v.Owner == owner && v.Level == 1 {
							vertex = v.ID
							break
						}
					}
					if vertex < 0 {
						t.Fatal("missing original village")
					}
					if err := s.save(r); err != nil {
						t.Fatal(err)
					}
					act(owner, game.Action{Type: "catan_city", Vertex: vertex}, false)
					act(owner, game.Action{Type: "catan_end"}, false)
					beforeWagons := len(s.rooms[id].Game.Catan.Caravans.Wagons)
					for step := 0; s.rooms[id].Game.Catan.Caravans.Pending != nil && step < 30; step++ {
						state = s.rooms[id].Game
						actor := state.CatanPendingActor()
						a, err := state.BotAction(actor)
						if err != nil {
							t.Fatal(err)
						}
						before, _ := json.Marshal(s.rooms[id])
						clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_roll"}, 400)
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("vote allowed a draw")
						}
						act(actor, a, true)
						if s.rooms[id].Game.Catan.RollID != wantDraws {
							t.Fatal("voting consumed another event")
						}
					}
					state = s.rooms[id].Game
					count := 1
					if n == 2 {
						count = 2
					}
					if state.Catan.Caravans.Pending != nil || len(state.Catan.Caravans.Wagons) != beforeWagons+count {
						t.Fatal("vote stalled")
					}
					if n == 6 && stage == 0 && (!state.Catan.Paired.Second || state.Phase != "catan_turn") {
						t.Fatal("secondary turn missing")
					}
				}
				if state := s.rooms[id].Game; state.Phase != "catan_roll" || state.Catan.RollID != wantDraws {
					t.Fatal("next production did not resume")
				}
				if modes["catan_caravan_bid"] == 0 || modes["catan_caravan_place"] == 0 {
					t.Fatal("missing real votes", modes)
				}
				t.Logf("%s actual response paths: %v", mode, modes)
			})
		}
	}
}
