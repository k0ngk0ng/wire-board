package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

func TestCatanFishingDesertDualProductionHTTP(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_roll", "ship")
				state, err := game.NewCatanFishingSeafarers(n, game.CatanOptions{}, game.CatanSeafarersSetup{Scenario: "desert", Layout: "fixed"}, nil)
				if err != nil {
					t.Fatal(err)
				}
				g := state.Catan
				state.Turn, state.Phase = 0, "catan_roll"
				g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
				g.Robber = -1
				extra := g.Fishing.Map.ExtraNumbers[0]
				recipient := g.Tiles[extra.Tile]
				for i := range g.Tiles {
					if i != extra.Tile && g.Tiles[i].Resource < 5 {
						g.Tiles[i].Number = 5
					}
				}
				g.Vertices[recipient.Vertices[0]].Owner, g.Vertices[recipient.Vertices[0]].Level = 1, 2
				lake := g.Tiles[g.Fishing.Map.Lakes[0].Tile]
				g.Vertices[lake.Vertices[0]].Owner, g.Vertices[lake.Vertices[0]].Level = 2, 1
				f := g.Fishing
				for _, token := range []int{0, 1, 2, 3, 4, 5, 6} {
					at := slices.Index(f.Tokens.DrawPile, token)
					f.Tokens.DrawPile = slices.Delete(f.Tokens.DrawPile, at, at+1)
					f.Tokens.Hands[2] = append(f.Tokens.Hands[2], token)
				}
				// Real Apply roll, using saved initial state for independent attempts.
				initial, _ := json.Marshal(state)
				for attempt := 0; attempt < 1000; attempt++ {
					var next game.State
					if err = json.Unmarshal(initial, &next); err != nil {
						t.Fatal(err)
					}
					if err = next.Apply(0, game.Action{Type: "catan_roll"}); err != nil {
						t.Fatal(err)
					}
					if next.Catan.Dice[0]+next.Catan.Dice[1] == extra.Number {
						state = &next
						break
					}
				}
				if state.Phase != "catan_fish_replace" || state.CatanPendingActor() != 2 || state.Catan.Players[1].Resources[recipient.Resource] != 2 {
					t.Fatal("dual production fixture failed")
				}
				now := time.Now()
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = state
				r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
				if !r.adjustCatanResponseClock("catan_roll", -1, g.SetupStep, now) {
					t.Fatal("missing response clock")
				}
				if err = s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				restart := func() {
					t.Helper()
					before, _ := json.Marshal(s.rooms[id])
					ts.Close()
					s.Close()
					next, e := New(s.cfg, s.files)
					if e != nil {
						t.Fatal(e)
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
						t.Fatal("room/dual discs lost on restart")
					}
				}
				restart()
				for viewer, c := range clients {
					raw := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					public := raw["fishing"].(map[string]any)
					m := public["map"].(map[string]any)
					x := m["extraNumbers"].([]any)[0].(map[string]any)
					if int(x["tile"].(float64)) != extra.Tile || int(x["number"].(float64)) != extra.Number {
						t.Fatal("public disc missing")
					}
					tokens := public["tokens"].(map[string]any)
					if public["pending"] != nil || tokens["drawPile"] != nil {
						t.Fatal("private continuation/pile leaked")
					}
					for seat, v := range tokens["players"].([]any) {
						_, visible := v.(map[string]any)["tokens"]
						if visible != (viewer == seat && seat == 2) {
							t.Fatal("private fish leaked")
						}
					}
				}
				before, _ := json.Marshal(s.rooms[id])
				a := game.Action{Type: "catan_fish_keep"}
				clients[0].command(current(clients[0]), "action", a, 400)
				clients[n].command(current(clients[n]), "action", a, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("illegal response mutated room")
				}
				at := time.Now()
				switch mode {
				case "manual":
					clients[2].command(current(clients[2]), "action", a, 200)
				case "autoplay":
					setAutoPlay(clients[2], current(clients[2]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(at)
					s.mu.Unlock()
				case "timeout":
					at = time.UnixMilli(s.rooms[id].TurnDeadline)
					s.mu.Lock()
					s.expireSetups(at)
					s.mu.Unlock()
				}
				r = s.rooms[id]
				remaining := r.TurnDeadline - at.UnixMilli()
				if r.Game.Phase != "catan_turn" || r.Game.Turn != 0 || r.Game.Catan.Players[1].Resources[recipient.Resource] != 2 || remaining < 45000 || remaining > 46000 {
					t.Fatal("duplicate production/lost turn", r.Game.Phase, remaining)
				}
				restart()
			})
		}
	}
}
