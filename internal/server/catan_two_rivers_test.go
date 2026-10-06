package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"slices"
	"testing"
	"time"
)

// Explicit bridge-response fixtures; full combination setup/history and a
// browser game remain separate acceptance work, not implied by this matrix.
func TestCatanTwoRiversHTTPBridgeResponseAndRestart(t *testing.T) {
	for _, mode := range []string{"manual", "autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := newTwoFullTable(t)
			base, e := game.NewCatanTwoRivers(2, game.CatanOptions{})
			if e != nil {
				t.Fatal(e)
			}
			base.Turn = 0
			base.Catan.StartPlayer = 0
			if e = base.Apply(0, game.Action{Type: "catan_rivers_start", Tile: base.Catan.Rivers.Map.Swamps[0]}); e != nil {
				t.Fatal(e)
			}
			base.Catan.SetupStep = 4
			base.Phase = "catan_turn"
			base.Catan.Two.Rolls = []int{2, 12}
			for c, n := range []int{1, 2, 0, 0, 0} {
				base.Catan.Players[0].Resources[c] = n
				base.Catan.Bank[c] -= n
			}
			var state *game.State
			edge := -1
			// A currently empty, distance-legal bank of a bridge receives the fixture's
			// real settlement; the paid bridge and compulsory neutral bridge use Apply.
			for _, candidate := range base.Catan.Rivers.Map.Bridges {
				for _, v := range []int{base.Catan.Edges[candidate].A, base.Catan.Edges[candidate].B} {
					legal := base.Catan.Vertices[v].Level == 0
					for _, road := range base.Catan.Edges {
						if road.A == v && base.Catan.Vertices[road.B].Level > 0 || road.B == v && base.Catan.Vertices[road.A].Level > 0 {
							legal = false
						}
					}
					if !legal {
						continue
					}
					raw, _ := json.Marshal(base)
					var trial game.State
					json.Unmarshal(raw, &trial)
					trial.Catan.Vertices[v].Owner = 0
					trial.Catan.Vertices[v].Level = 1
					before, _ := json.Marshal(trial)
					if trial.Apply(0, game.Action{Type: "catan_bridge", Edge: candidate}) != nil || trial.Catan.Two.Pending == nil {
						continue
					}
					view := trial.View(0)["catan"].(map[string]any)["two"]
					raw, _ = json.Marshal(view)
					var response struct {
						Choices []struct {
							Edge int `json:"edge"`
						} `json:"choices"`
					}
					json.Unmarshal(raw, &response)
					if len(response.Choices) == 0 || !slices.Contains(trial.Catan.Rivers.Map.Bridges, response.Choices[0].Edge) {
						continue
					}
					state = &game.State{}
					json.Unmarshal(before, state)
					edge = candidate
					break
				}
				if state != nil {
					break
				}
			}
			if state == nil {
				t.Fatal("no legal neutral bridge fixture")
			}
			s.mu.Lock()
			r := s.rooms[id]
			r.Game = state
			r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
			if e = s.save(r); e != nil {
				t.Fatal(e)
			}
			s.mu.Unlock()
			clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_bridge", Edge: edge}, 200)
			r = s.rooms[id]
			if r.Game.Phase != "catan_two_build" || r.Game.Catan.Two.Pending.Kind != "bridge" || r.CatanTimeLeft < 44000 || r.CatanTimeLeft > 45000 || r.TurnDeadline-time.Now().UnixMilli() < 119000 {
				t.Fatal("bridge response clock")
			}
			gold, tokens, bank := slices.Clone(r.Game.Catan.Rivers.Gold), slices.Clone(r.Game.Catan.Two.Tokens), slices.Clone(r.Game.Catan.Bank)
			remaining := r.CatanTimeLeft
			assertTwoHTTPPrivacy(t, clients, r.Game)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			r = s.rooms[id]
			a, e := r.Game.BotAction(0)
			if e != nil {
				t.Fatal(e)
			}
			if a.Type != "catan_two_build" || !slices.Contains(r.Game.Catan.Rivers.Map.Bridges, a.Edge) {
				t.Fatal("bot failed to choose bridge", a)
			}
			before, _ := json.Marshal(r)
			for _, wrong := range []int{1, 2} {
				clients[wrong].command(current(clients[wrong]), "action", a, 400)
			}
			clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_end"}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid response mutated state/time")
			}
			at := time.Now()
			request := map[string]any{"type": "action", "action": a, "version": r.Version, "nonce": randomID(12)}
			switch mode {
			case "manual":
				clients[0].post("/api/rooms/"+id, request, 200)
			case "autoplay":
				setAutoPlay(clients[0], current(clients[0]), true, 200)
				s.mu.Lock()
				s.rooms[id].BotAt = 0
				s.runBots(at)
				s.mu.Unlock()
				setAutoPlay(clients[0], current(clients[0]), false, 200)
			case "timeout":
				at = time.UnixMilli(r.TurnDeadline)
				s.mu.Lock()
				s.expireSetups(at)
				s.mu.Unlock()
			}
			r = s.rooms[id]
			g := r.Game.Catan
			if r.Game.Phase != "catan_turn" || g.Two.Pending != nil || !g.Edges[a.Edge].Bridge || g.Edges[a.Edge].Owner != -a.Target-2 || !slices.Equal(g.Rivers.Gold, gold) || !slices.Equal(g.Two.Tokens, tokens) || !slices.Equal(g.Bank, bank) || r.TurnDeadline-at.UnixMilli() < remaining-100 || r.TurnDeadline-at.UnixMilli() > remaining+1000 {
				t.Fatal("neutral bridge accounting/clock", mode)
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			if mode == "manual" {
				clients[0].post("/api/rooms/"+id, request, 200)
			}
			clients[0].command(current(clients[0]), "action", a, 400)
			assertTwoHTTPPrivacy(t, clients, s.rooms[id].Game)
		})
	}
}
