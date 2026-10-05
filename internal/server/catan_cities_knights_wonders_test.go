package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCatanCitiesKnightsSeaHTTPPreludeRestart(t *testing.T) {
	for _, scenario := range []string{"wonders", "cloth"} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(scenario+"/"+mode, func(t *testing.T) {
				s, ts, clients, id := newCatanTable(t)
				state, e := game.NewCatanCitiesKnightsSeafarers(3, game.CatanOptions{}, game.CatanSeafarersSetup{Scenario: scenario, Layout: "variable"}, nil)
				if e != nil {
					t.Fatal(e)
				}
				state.Turn = 2
				state.Catan.StartPlayer = 2
				choice := state.Catan.CitiesKnights.RobberStart
				for _, tile := range state.Catan.Tiles {
					if (scenario == "wonders" && tile.Resource == game.CatanDesert) || (scenario == "cloth" && tile.Number == 12) {
						choice = tile.ID
					}
				}
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = state
				r.startTurnClock(time.Now())
				e = s.save(r)
				s.mu.Unlock()
				if e != nil {
					t.Fatal(e)
				}
				before, _ := json.Marshal(r)
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_" + scenario + "_start", Tile: choice}, 400)
				clients[2].command(current(clients[2]), "action", game.Action{Type: "catan_" + scenario + "_start", Tile: -1}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid start changed state/clock")
				}
				ts.Close()
				s.Close()
				next, e := New(s.cfg, s.files)
				if e != nil {
					t.Fatal(e)
				}
				defer next.Close()
				stopBotTicker(next)
				ts2 := httptest.NewServer(next.Handler())
				defer ts2.Close()
				for _, c := range clients {
					c.base = ts2.URL
				}
				after, _ = json.Marshal(next.rooms[id])
				if string(before) != string(after) {
					t.Fatal("prelude lost on restart")
				}
				switch mode {
				case "manual":
					clients[2].command(current(clients[2]), "action", game.Action{Type: "catan_" + scenario + "_start", Tile: choice}, 200)
				case "autoplay":
					choice = state.Catan.CitiesKnights.RobberStart
					setAutoPlay(clients[2], current(clients[2]), true, 200)
					next.mu.Lock()
					next.rooms[id].BotAt = 0
					next.runBots(time.Now())
					next.mu.Unlock()
				case "timeout":
					choice = state.Catan.CitiesKnights.RobberStart
					next.mu.Lock()
					next.rooms[id].TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
					next.expireSetups(time.Now())
					next.mu.Unlock()
				}
				room := next.rooms[id]
				g := room.Game.Catan
				if room.Game.Phase != "catan_setup_settlement" || g.Robber != -1 || g.CitiesKnights.RobberStart != choice || g.Seafarers.Pirate != -1 {
					t.Fatal("origin not resolved while dormant", mode, room.Game.Phase)
				}
				if left := room.TurnDeadline - time.Now().UnixMilli(); left < 118000 || left > 120000 {
					t.Fatal("opening player did not get fresh 120s", left)
				}
				v := current(clients[2])["game"].(map[string]any)["catan"].(map[string]any)
				legal := v["legal"].(map[string]any)
				if len(legal["pirate"].([]any)) != 0 {
					t.Fatal("pirate offered")
				}
			})
		}
	}
}
