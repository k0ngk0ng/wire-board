package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanTransportSeaHTTP(t *testing.T) {
	for _, scenario := range []string{"transport-shores", "transport-desert"} {
		for n := 2; n <= 6; n++ {
			t.Run(fmt.Sprintf("%s/%d", scenario, n), func(t *testing.T) {
				s, ts, clients, id := newPublicFishingScenarioVariants(t, n, scenario, false, false, true)
				movement, timeout := false, false
				for step := 0; step < 14000 && !s.rooms[id].Game.Finished; step++ {
					r := s.rooms[id]
					g := r.Game
					actor := twoHTTPActor(g)
					if g.Phase == "catan_transport_move" && !movement {
						deadline := r.TurnDeadline
						if left := deadline - time.Now().UnixMilli(); left < 118000 || left > 120000 {
							t.Fatal("movement clock", left)
						}
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						if s.rooms[id].TurnDeadline != deadline {
							t.Fatal("restart reset clock")
						}
						s.mu.Lock()
						s.expireSetups(time.UnixMilli(deadline + 1))
						s.mu.Unlock()
						if !s.rooms[id].Seats[actor].AutoPlay || !s.rooms[id].Seats[actor].TimeoutAutoPlay {
							t.Fatal("timeout did not take over")
						}
						setAutoPlay(clients[actor], current(clients[actor]), false, 200)
						movement, timeout = true, true
						continue
					}
					a, err := g.BotAction(actor)
					if err != nil {
						t.Fatal(step, g.Phase, err)
					}
					clients[actor].command(current(clients[actor]), "action", a, 200)
				}
				r := s.rooms[id]
				if !r.Game.Finished || !movement || !timeout {
					t.Fatal("incomplete match", r.Game.Phase)
				}
				assertTransportInventory(t, r.Game)
				_, profile := clients[n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
				record := profile["history"].([]any)[0].(map[string]any)
				rules := record["catanExpansionRules"].(map[string]any)
				if record["catanScenario"] != scenario || rules["transport_seafarers"] != game.CatanTransportSeafarersRules || rules["event_cards"] != game.CatanEventCatalogue {
					t.Fatal("lost combination history", record)
				}
				t.Logf("completed round %d", r.Game.Round)
			})
		}
	}
}
