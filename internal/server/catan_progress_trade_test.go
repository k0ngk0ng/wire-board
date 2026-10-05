package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanProgressTradeHTTPResponsesPrivacyRestartAndClock(t *testing.T) {
	for _, kind := range []string{"guild_dues", "commercial_harbor"} {
		for _, mode := range []string{"manual", "timeout", "autoplay"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				s, ts, clients, id := newCatanTable(t)
				clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				state, err := game.NewCatanCitiesKnights(3, game.CatanOptions{})
				if err != nil {
					t.Fatal(err)
				}
				g := state.Catan
				g.SetupStep = 6
				g.TurnSerial = 1
				state.Turn, state.Phase = 0, "catan_turn"
				g.Vertices[0].Owner, g.Vertices[0].Level = 1, 2
				g.Players[1].Score = 2
				g.Players[0].Resources[0] = 3
				g.Bank[0] -= 3
				g.Players[1].Resources[5] = 3
				g.Bank[5] -= 3
				card, actor := 11, 0
				response := map[string]any{"type": "catan_guild_dues", "take": []int{0, 0, 0, 0, 0, 2, 0, 0}}
				if kind == "commercial_harbor" {
					card, actor = 10, 1
					response = map[string]any{"type": "catan_commercial_harbor", "color": 5}
				}
				cityProgressGive(t, g, 0, card)
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = state
				deadline := time.Now().Add(45 * time.Second).UnixMilli()
				r.TurnDeadline = deadline
				if err = s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_progress", "card": card, "target": 1}, 200)
				if kind == "commercial_harbor" {
					if s.rooms[id].TurnDeadline != deadline {
						t.Fatal("activating optional harbor reset ordinary clock")
					}
					clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_commercial_offer", "card": 0, "target": 1, "color": 0}, 200)
				}
				r = s.rooms[id]
				if r.Game.CatanPendingActor() != actor || r.Game.Turn != 0 || r.Game.Phase != "catan_"+kind || r.CatanTimeLeft < 44000 || r.CatanTimeLeft > 45000 {
					t.Fatal("trade response failed to pause original turn")
				}
				if remaining := r.TurnDeadline - time.Now().UnixMilli(); remaining < 119000 || remaining > 120000 {
					t.Fatal("response must receive 120 seconds")
				}
				savedTime := r.CatanTimeLeft
				before, _ := json.Marshal(r)
				for _, p := range []int{0, 1, 2, 3} {
					if p != actor {
						clients[p].command(current(clients[p]), "action", response, 400)
					}
				}
				clients[actor].command(current(clients[actor]), "action", map[string]any{"type": "catan_" + kind, "color": 0, "take": []int{2, 0, 0, 0, 0, 0, 0, 0}}, 400)
				clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_end"}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid response mutated resources or deadline")
				}
				for viewer, c := range clients {
					view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					k := view["citiesKnights"].(map[string]any)
					if _, ok := k["progressDecks"]; ok {
						t.Fatal("progress deck exposed")
					}
					_, revealed := k["pending"].(map[string]any)["resources"]
					if revealed != (kind == "guild_dues" && viewer == 0) {
						t.Fatal("Guild Dues hand reveal for wrong viewer")
					}
					if len(view["progressPlayable"].([]any)) != 0 {
						t.Fatal("progress may be played during mandatory response")
					}
					for p, raw := range view["players"].([]any) {
						_, ok := raw.(map[string]any)["resources"]
						if ok != (p == viewer) {
							t.Fatal("ordinary hand leaked")
						}
					}
				}
				ts.Close()
				s.Close()
				next, err := New(s.cfg, s.files)
				if err != nil {
					t.Fatal(err)
				}
				defer next.Close()
				stopBotTicker(next)
				after, _ = json.Marshal(next.rooms[id])
				if string(before) != string(after) {
					t.Fatal("trade response or power lost on restart")
				}
				ts2 := httptest.NewServer(next.Handler())
				defer ts2.Close()
				for _, c := range clients {
					c.base = ts2.URL
				}
				var resolvedAt time.Time
				switch mode {
				case "manual":
					clients[actor].command(current(clients[actor]), "action", response, 200)
					resolvedAt = time.Now()
				case "timeout":
					next.mu.Lock()
					resolvedAt = time.UnixMilli(next.rooms[id].TurnDeadline)
					next.expireSetups(resolvedAt)
					next.mu.Unlock()
				case "autoplay":
					next.mu.Lock()
					next.rooms[id].Seats[actor].AutoPlay = true
					next.rooms[id].BotAt = 0
					resolvedAt = time.Now()
					next.runBots(resolvedAt)
					next.mu.Unlock()
				}
				r = next.rooms[id]
				g = r.Game.Catan
				if r.Game.Phase != "catan_turn" || r.Game.Turn != 0 || r.Game.CatanPendingActor() != -1 {
					t.Fatal("trade response did not return to current player")
				}
				want := 2
				if kind == "commercial_harbor" {
					want = 1
				}
				if g.Players[0].Resources[5] != want || g.Players[1].Resources[5] != 3-want {
					t.Fatal("wrong goods exchange")
				}
				if kind == "commercial_harbor" && (g.Players[0].Resources[0] != 2 || g.Players[1].Resources[0] != 1) {
					t.Fatal("harbor resource not transferred")
				}
				remaining := r.TurnDeadline - resolvedAt.UnixMilli()
				if remaining > savedTime || remaining < savedTime-1000 {
					t.Fatal("wrong resumed clock", remaining, savedTime)
				}
				if kind == "commercial_harbor" {
					before, _ = json.Marshal(r)
					clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_commercial_offer", "card": 0, "target": 1, "color": 0}, 400)
					after, _ = json.Marshal(next.rooms[id])
					if string(before) != string(after) {
						t.Fatal("repeated harbor offer mutated state")
					}
				}
				for _, c := range clients {
					k := current(c)["game"].(map[string]any)["catan"].(map[string]any)["citiesKnights"].(map[string]any)
					if _, ok := k["pending"]; ok {
						t.Fatal("private reveal retained after resolution")
					}
				}
			})
		}
	}
}
