package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanFishingNewWorldLayoutHTTPAndRestart(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "ship")
				layout, err := game.GenerateCatanNewWorldMap(n)
				if err != nil {
					t.Fatal(err)
				}
				state, err := game.NewCatanFishingNewWorld(n, game.CatanOptions{}, layout)
				if err != nil {
					t.Fatal(err)
				}
				first := n - 1
				state.Turn, state.Catan.StartPlayer = first, first
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = state
				r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
				if err = s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				restart := func() {
					t.Helper()
					before, _ := json.Marshal(s.rooms[id])
					ts.Close()
					s.Close()
					next, err := New(s.cfg, s.files)
					if err != nil {
						t.Fatal(err)
					}
					stopBotTicker(next)
					t.Cleanup(func() { next.Close() })
					ts = httptest.NewServer(next.Handler())
					t.Cleanup(ts.Close)
					for _, c := range clients {
						c.base = ts.URL
					}
					s = next
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("restart changed layout/hidden decks/actor/clock")
					}
				}
				for step := 0; step < 16; step++ {
					if step == 0 || step == 9 || step == 10 || step == 15 {
						restart()
					}
					r = s.rooms[id]
					actor := r.Game.Turn
					a, err := r.Game.BotAction(actor)
					if err != nil {
						t.Fatal(step, err)
					}
					for viewer, c := range clients {
						view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
						f := view["fishing"].(map[string]any)
						q := f["worldSetup"].(map[string]any)
						_, shown := q["current"]
						if q["numbers"] != nil || shown != (step >= 10) || f["tokens"].(map[string]any)["drawPile"] != nil {
							t.Fatal("unrevealed layout/fish deck exposed")
						}
						legal := view["legal"].(map[string]any)
						grounds, _ := legal["fishGrounds"].([]any)
						if (len(grounds) > 0) != (step >= 10 && viewer == actor) {
							t.Fatal("ground placement permission")
						}
					}
					before, _ := json.Marshal(r)
					clients[(actor+1)%n].command(current(clients[(actor+1)%n]), "action", a, 400)
					clients[n].command(current(clients[n]), "action", a, 400)
					clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_world_fish", Vertex: -1}, 400)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("invalid request changed game/deadline")
					}
					at := time.Now()
					switch mode {
					case "manual":
						clients[actor].command(current(clients[actor]), "action", a, 200)
					case "autoplay":
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
						s.mu.Lock()
						s.rooms[id].BotAt = 0
						s.runBots(at)
						s.mu.Unlock()
						setAutoPlay(clients[actor], current(clients[actor]), false, 200)
					case "timeout":
						at = time.UnixMilli(r.TurnDeadline)
						s.mu.Lock()
						s.expireSetups(at)
						s.mu.Unlock()
					}
					r = s.rooms[id]
					g := r.Game.Catan
					left := r.TurnDeadline - at.UnixMilli()
					wantTurn, wantPhase := (first+step+1)%n, "catan_world_ports"
					if step >= 9 {
						wantTurn, wantPhase = (first+step-9)%n, "catan_world_fish"
					}
					if step == 15 {
						wantTurn, wantPhase = first, "catan_setup_settlement"
					}
					if g.SetupStep != 0 || len(g.Ports) != min(10, step+1) || len(g.Fishing.Map.Grounds) != max(0, step-9) ||
						r.Game.Turn != wantTurn || r.Game.Phase != wantPhase || left < 120000 || left > 121000 {
						t.Fatal("one-piece handoff/clock", step, r.Game.Turn, r.Game.Phase, left)
					}
					for _, v := range g.Vertices {
						if v.Level != 0 {
							t.Fatal("layout action also built a settlement")
						}
					}
				}
				restart()
			})
		}
	}
}
