package server

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanExplorerIntroPublicConfiguration(t *testing.T) {
	testPublicExplorerMissionConfiguration(t, "land-ho")
}

func TestCatanExplorerIntroNaturalHTTPGames(t *testing.T) {
	for _, tc := range []struct {
		n                  int
		city, fish, events bool
	}{
		{5, false, false, false}, {6, false, true, true}, {3, true, false, true}, {6, true, true, false},
	} {
		t.Run(fmt.Sprintf("%d/city%t/fish%t/events%t", tc.n, tc.city, tc.fish, tc.events), func(t *testing.T) {
			s, ts, clients, id := newPublicExplorerRecipeHTTP(t, tc.n, "land-ho", tc.city, tc.events, tc.fish, tc.fish)
			restored, timedout := false, false
			for step := 0; step < 10000 && !s.rooms[id].Game.Finished; step++ {
				r := s.rooms[id]
				p := r.Game.CatanPendingActor()
				if p < 0 {
					p = explorerHTTPActor(r.Game)
				}
				if r.Game.Phase == "catan_roll" && r.Game.Catan.RollID > 0 && !timedout {
					s.mu.Lock()
					s.expireSetups(time.UnixMilli(r.TurnDeadline))
					s.mu.Unlock()
					if !s.rooms[id].Seats[p].AutoPlay {
						t.Fatal("timeout failed")
					}
					reclaimTimeoutHumans(t, s, clients, id)
					timedout = true
					continue
				}
				a, err := r.Game.BotAction(p)
				if err != nil {
					t.Fatal(step, r.Game.Phase, err)
				}
				req := map[string]any{"type": "action", "action": a, "version": r.Version, "nonce": randomID(12)}
				clients[p].post("/api/rooms/"+id, req, 200)
				if !restored && a.Type == "catan_explorer_begin_move" {
					before, _ := json.Marshal(s.rooms[id])
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					clients[p].post("/api/rooms/"+id, req, 200)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("restore replay changed state")
					}
					restored = true
				}
				if step%131 == 0 {
					view := current(clients[tc.n])["game"].(map[string]any)["catan"].(map[string]any)
					board := view["explorer"].(map[string]any)["board"].(map[string]any)
					if board["introRules"] != game.CatanExplorerIntroRules || board["hidden"] != nil || board["numbers"] != nil {
						t.Fatal("recipe missing or fog exposed")
					}
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || !restored || !timedout {
				t.Fatal("incomplete natural game")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			assertPublicExplorerHistory(t, s, clients, id)
			_, profile := clients[tc.n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
			rules := profile["history"].([]any)[0].(map[string]any)["catanExpansionRules"].(map[string]any)
			if rules["land_ho"] != game.CatanExplorerIntroRules {
				t.Fatal("site recipe missing from history")
			}
			t.Log("natural game finished", r.Game.Round, "rounds")
		})
	}
}
