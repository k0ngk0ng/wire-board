package server

import (
	"fmt"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanSeafarersEventDeckHTTPDrawRecoveryReplayAndTimeout(t *testing.T) {
	for _, scenario := range []string{"shores", "islands", "fog", "desert"} {
		for _, n := range []int{3, 6} {
			for _, mode := range []string{"manual", "autoplay", "timeout"} {
				t.Run(fmt.Sprintf("%s/%d/%s", scenario, n, mode), func(t *testing.T) {
					s, ts := setupServer(t)
					stopBotTicker(s)
					clients := make([]*testClient, n+1)
					for i := range clients {
						clients[i] = newClient(t, ts.URL)
						clients[i].register(fmt.Sprintf("海图事件%d", i))
					}
					raw := clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "海图事件参考", "capacity": n, "catanOptions": game.CatanOptions{FiveSix: n > 4}}, 201)
					id := raw["id"].(string)
					for i := 1; i < n; i++ {
						clients[i].command(current(clients[0]), "join", nil, 200)
					}
					// Explicit internal waiting recipe; no public expansion option.
					provisionSeafarers(t, s, id, game.CatanSeafarersSetup{Scenario: scenario})
					for i := 0; i < n; i++ {
						clients[i].command(current(clients[i]), "ready", nil, 200)
					}
					clients[0].command(current(clients[0]), "start", nil, 200)
					clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
					for s.rooms[id].Game.Phase != "catan_roll" {
						state := s.rooms[id].Game
						actor := state.Turn
						if p := state.CatanPendingActor(); p >= 0 {
							actor = p
						}
						a, err := state.BotAction(actor)
						if err != nil {
							t.Fatal(err)
						}
						clients[actor].command(current(clients[actor]), "action", a, 200)
					}
					// A pinned reference face order makes every seat respond. The
					// common harness checks private hands/pile, nonce, persistence,
					// production, 120-second responses and restored turn budget.
					referenceEventDrawHTTP(t, s, ts, clients, id, mode)
				})
			}
		}
	}
}
