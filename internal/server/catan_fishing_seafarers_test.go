package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

func TestCatanFishingSeafarersHTTPAndRestart(t *testing.T) {
	for _, phase := range []string{"catan_roll", "catan_turn"} {
		for _, kind := range []string{"ship", "pirate"} {
			t.Run(phase+"/"+kind, func(t *testing.T) {
				s, ts, clients, id, p, a := newFishingActionTable(t, 3, phase, kind)
				deadline := s.rooms[id].TurnDeadline
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
						t.Fatal("sea fishing room changed on restart")
					}
				}
				restart()
				before, _ := json.Marshal(s.rooms[id])
				clients[1].command(current(clients[1]), "action", a, 400)
				clients[3].command(current(clients[3]), "action", a, 400)
				bad := a
				bad.Tokens = []int{999}
				clients[p].command(current(clients[p]), "action", bad, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid request changed room")
				}
				g := s.rooms[id].Game.Catan
				robber := g.Robber
				bank := slices.Clone(g.Bank)
				clients[p].command(current(clients[p]), "action", a, 200)
				r := s.rooms[id]
				g = r.Game.Catan
				if r.TurnDeadline != deadline || r.Game.Phase != phase || r.Game.Turn != p || g.Robber != robber || !slices.Equal(bank, g.Bank) || !slices.Equal(g.Fishing.Tokens.Discard, a.Tokens) {
					t.Fatal("payment reset clock/phase or ordinary resources")
				}
				if kind == "ship" {
					if !g.Edges[a.Edge].Ship || g.Edges[a.Edge].Owner != p || !slices.Contains(g.Seafarers.BuiltShips, a.Edge) {
						t.Fatal("ship action")
					}
				} else if g.Seafarers.Pirate != -1 {
					t.Fatal("pirate action")
				}
				restart()
				for viewer, c := range clients {
					f := current(c)["game"].(map[string]any)["catan"].(map[string]any)["fishing"].(map[string]any)
					tokens := f["tokens"].(map[string]any)
					if tokens["drawPile"] != nil {
						t.Fatal("pile leaked")
					}
					for seat, raw := range tokens["players"].([]any) {
						_, visible := raw.(map[string]any)["tokens"]
						if visible != (viewer == seat && seat == p) {
							t.Fatal("fish faces leaked")
						}
					}
					legal := f["legal"].(map[string]any)
					if viewer != p && (len(legal["actions"].([]any)) != 0 || len(legal["ships"].([]any)) != 0) {
						t.Fatal("private legal actions leaked")
					}
				}
			})
		}
	}
}

func seaFishingResponseFixture(t *testing.T, r *Room, now time.Time) {
	t.Helper()
	state, err := game.NewCatanFishingSeafarers(3, game.CatanOptions{}, game.CatanSeafarersSetup{Scenario: "islands"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g := state.Catan
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	state.Turn, state.Phase = 0, "catan_roll"
	g.Seafarers.Pirate = -1
	for i := range g.Tiles {
		if g.Tiles[i].Resource < 5 {
			g.Tiles[i].Number = 2
		}
	}
	ground := g.Fishing.Map.Grounds[0]
	g.Vertices[ground.Vertices[0]].Owner, g.Vertices[ground.Vertices[0]].Level = 1, 2
	g.Vertices[ground.Vertices[2]].Owner, g.Vertices[ground.Vertices[2]].Level = 2, 1
	f := g.Fishing
	for player, ids := range map[int][]int{1: {0, 1, 2, 3, 4, 5, 6}, 2: {11, 12, 13, 14, 15, 16, 17}} {
		for _, id := range ids {
			at := slices.Index(f.Tokens.DrawPile, id)
			f.Tokens.DrawPile = slices.Delete(f.Tokens.DrawPile, at, at+1)
			f.Tokens.Hands[player] = append(f.Tokens.Hands[player], id)
		}
	}
	at := slices.Index(f.Tokens.DrawPile, 29)
	f.Tokens.DrawPile = slices.Delete(f.Tokens.DrawPile, at, at+1)
	f.Tokens.DrawPile = append(f.Tokens.DrawPile, 29)
	initial, _ := json.Marshal(state)
	for attempt := 0; attempt < 1000; attempt++ {
		var next game.State
		if err = json.Unmarshal(initial, &next); err != nil {
			t.Fatal(err)
		}
		if err = next.Apply(0, game.Action{Type: "catan_roll"}); err != nil {
			t.Fatal(err)
		}
		if next.Phase == "catan_fish_replace" && next.CatanPendingActor() == 1 {
			state = &next
			break
		}
	}
	if state.CatanPendingActor() != 1 {
		t.Fatal("fixture did not roll fishing production")
	}
	r.Game = state
	r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
	if !r.adjustCatanResponseClock("catan_roll", -1, g.SetupStep, now) || r.CatanTimeLeft != 45000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("response clock")
	}
}
func TestCatanFishingSeafarersResponseHTTP(t *testing.T) {
	for _, mode := range []string{"manual", "autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := newCatanTable(t)
			clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			s.mu.Lock()
			seaFishingResponseFixture(t, s.rooms[id], time.Now())
			if err := s.save(s.rooms[id]); err != nil {
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
					t.Fatal("response restore changed room")
				}
			}
			restart()
			for step, actor := range []int{1, 2} {
				r := s.rooms[id]
				if r.Game.Phase != "catan_fish_replace" || r.Game.CatanPendingActor() != actor {
					t.Fatal("responder sequence")
				}
				a := game.Action{Type: "catan_fish_replace", Card: 0}
				if actor == 2 {
					a = game.Action{Type: "catan_fish_keep"}
				}
				before, _ := json.Marshal(r)
				clients[0].command(current(clients[0]), "action", a, 400)
				clients[3].command(current(clients[3]), "action", a, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("wrong actor changed response")
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
				case "timeout":
					at = time.UnixMilli(r.TurnDeadline)
					s.mu.Lock()
					s.expireSetups(at)
					s.mu.Unlock()
				}
				r = s.rooms[id]
				want := int64(120000)
				if step == 1 {
					want = 45000
				}
				remaining := r.TurnDeadline - at.UnixMilli()
				if remaining < want || remaining > want+1000 || r.Game.Turn != 0 || r.CatanTimeLeft != 45000 {
					t.Fatal("response timer", step, remaining)
				}
				restart()
			}
			g := s.rooms[id].Game.Catan
			if s.rooms[id].Game.Phase != "catan_turn" || g.Fishing.LastRollID != 1 || g.RollID != 1 || g.Fishing.Pending != nil || s.rooms[id].Game.CatanPendingActor() != -1 {
				t.Fatal("duplicate production or lost turn")
			}
			for _, p := range g.Players {
				for _, count := range p.Resources {
					if count != 0 {
						t.Fatal("response repeated ordinary production")
					}
				}
			}
		})
	}
}
