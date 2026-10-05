package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"
)

func fishingWorldGoldResponseState(t *testing.T, n int, scarce bool) *game.State {
	t.Helper()
	data, err := os.ReadFile("../game/testdata/catan-new-world-gold.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Layout                                          game.CatanNewWorldMap
		GoldTiles, PortEdges, FishVertices, FishNumbers []int
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	state, err := game.NewCatanFishingNewWorld(n, game.CatanOptions{}, &fixture.Layout)
	if err != nil {
		t.Fatal(err)
	}
	state.Catan.Fishing.WorldSetup.Numbers = slices.Clone(fixture.FishNumbers)
	for _, edge := range fixture.PortEdges {
		if err = state.Apply(state.Turn, game.Action{Type: "catan_world_port", Edge: edge}); err != nil {
			t.Fatal(err)
		}
	}
	for _, vertex := range fixture.FishVertices {
		if err = state.Apply(state.Turn, game.Action{Type: "catan_world_fish", Vertex: vertex}); err != nil {
			t.Fatal(err)
		}
	}
	g := state.Catan
	state.Turn, state.Phase = 0, "catan_roll"
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	for p := range g.Players {
		g.Fishing.Started[p] = true
	}
	ground := g.Fishing.Map.Grounds[0]
	g.Vertices[ground.Vertices[0]].Owner, g.Vertices[ground.Vertices[0]].Level = 1, 2
	g.Vertices[ground.Vertices[2]].Owner, g.Vertices[ground.Vertices[2]].Level = 2, 1
	g.Players[1].Score, g.Players[2].Score = 2, 1
	placed := false
	for _, vertex := range g.Tiles[14].Vertices {
		ok := g.Vertices[vertex].Owner < 0
		for _, e := range g.Edges {
			if e.A == vertex && g.Vertices[e.B].Level > 0 || e.B == vertex && g.Vertices[e.A].Level > 0 {
				ok = false
			}
		}
		if ok {
			g.Vertices[vertex].Owner, g.Vertices[vertex].Level = 0, 1
			g.Players[0].Score = 1
			placed = true
			break
		}
	}
	if !placed {
		t.Fatal("no ordinary grain producer")
	}
	for _, e := range g.Edges {
		if g.Vertices[e.A].Level > 0 && g.Vertices[e.B].Level > 0 {
			t.Fatal("fixture violates building distance")
		}
	}
	f := &g.Fishing.Tokens
	for p, ids := range map[int][]int{1: {0, 1, 2, 3, 4, 5, 6}, 2: {11, 12, 13, 14, 15, 16, 17}} {
		for _, id := range ids {
			at := slices.Index(f.DrawPile, id)
			f.DrawPile = slices.Delete(f.DrawPile, at, at+1)
			f.Hands[p] = append(f.Hands[p], id)
		}
	}
	at := slices.Index(f.DrawPile, 29)
	f.DrawPile = slices.Delete(f.DrawPile, at, at+1)
	f.DrawPile = append(f.DrawPile, 29)
	if scarce {
		for c := range g.Bank {
			left := 0
			if c == 4 {
				left = 1
			}
			g.Players[0].Resources[c] += g.Bank[c] - left
			g.Bank[c] = left
		}
	}
	saved, _ := json.Marshal(state)
	for attempt := 0; attempt < 1000; attempt++ {
		var next game.State
		if err = json.Unmarshal(saved, &next); err != nil {
			t.Fatal(err)
		}
		if err = next.Apply(0, game.Action{Type: "catan_roll"}); err != nil {
			t.Fatal(err)
		}
		if next.Catan.Dice[0]+next.Catan.Dice[1] == 9 {
			if next.Phase != "catan_fish_replace" || next.CatanPendingActor() != 1 || !reflect.DeepEqual(next.Catan.NewWorldMap(), &fixture.Layout) {
				t.Fatal("gold+fish roll/map")
			}
			return &next
		}
	}
	t.Fatal("failed to roll nine")
	return nil
}

func TestCatanFishingNewWorldGoldHTTPClockAndRestart(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, scarce := range []bool{false, true} {
			for _, mode := range []string{"manual", "autoplay", "timeout"} {
				t.Run(fmt.Sprintf("%d/scarce=%v/%s", n, scarce, mode), func(t *testing.T) {
					s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_roll", "ship")
					state := fishingWorldGoldResponseState(t, n, scarce)
					now := time.Now()
					s.mu.Lock()
					r := s.rooms[id]
					r.Game = state
					r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
					if !r.adjustCatanResponseClock("catan_roll", -1, state.Catan.SetupStep, now) || r.CatanTimeLeft != 45000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
						t.Fatal("fish did not pause roll clock")
					}
					if err := s.save(r); err != nil {
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
							t.Fatal("restart changed map/gold/fish continuation/clock")
						}
					}
					originalMap, _ := json.Marshal(state.Catan.NewWorldMap())
					resources := make([]int, n)
					for p, seat := range state.Catan.Players {
						for _, count := range seat.Resources {
							resources[p] += count
						}
					}
					actors := []int{1, 2, 1, 2}
					if scarce {
						actors = actors[:3]
					}
					restart()
					for step, actor := range actors {
						r = s.rooms[id]
						phase := "catan_fish_replace"
						if step >= 2 {
							phase = "catan_gold"
						}
						if r.Game.Phase != phase || r.Game.CatanPendingActor() != actor || r.Game.Turn != 0 {
							t.Fatal("gold/fish responder order", step)
						}
						a := game.Action{Type: "catan_fish_keep"}
						if step == 0 {
							a = game.Action{Type: "catan_fish_replace", Card: 0}
						}
						count := 0
						if step >= 2 {
							count = 3 - actor
							if scarce {
								count = 1
							}
							a = game.Action{Type: "catan_gold", Take: []int{0, 0, 0, 0, count}}
						}
						for viewer, c := range clients {
							v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
							f := v["fishing"].(map[string]any)
							tokens := f["tokens"].(map[string]any)
							if f["pending"] != nil || tokens["drawPile"] != nil || f["worldSetup"].(map[string]any)["numbers"] != nil || f["canReplace"] != (step < 2 && viewer == actor) {
								t.Fatal("private continuation or permissions exposed")
							}
							for p, raw := range tokens["players"].([]any) {
								_, visible := raw.(map[string]any)["tokens"]
								if visible != (p == viewer && (p == 1 || p == 2)) {
									t.Fatal("private fish face exposed")
								}
							}
							if step >= 2 {
								gold := v["goldPending"].(map[string]any)
								claims := gold["claims"].([]any)
								claim := claims[0].(map[string]any)
								if claim["player"] != float64(actor) || claim["count"] != float64(3-actor) {
									t.Fatal("public gold claim lost")
								}
							}
						}
						before, _ := json.Marshal(r)
						clients[0].command(current(clients[0]), "action", a, 400)
						clients[n].command(current(clients[n]), "action", a, 400)
						bad := game.Action{Type: "catan_fish_replace", Card: 999}
						if step >= 2 {
							bad = game.Action{Type: "catan_gold", Take: []int{0, 0, 0, 0, count + 1}}
						}
						clients[actor].command(current(clients[actor]), "action", bad, 400)
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("invalid request changed state/clock")
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
						want := int64(120000)
						if step == len(actors)-1 {
							want = 45000
						}
						left := r.TurnDeadline - at.UnixMilli()
						if left < want || left > want+1000 || r.CatanTimeLeft != 45000 || r.Game.Turn != 0 {
							t.Fatal("response clock", step, left)
						}
						resources[actor] += count
						for p, seat := range r.Game.Catan.Players {
							total := 0
							for _, value := range seat.Resources {
								total += value
							}
							if total != resources[p] {
								t.Fatal("duplicate/absent ordinary or gold resources", step, p, total, resources[p])
							}
						}
						restart()
					}
					g := s.rooms[id].Game.Catan
					if s.rooms[id].Game.Phase != "catan_turn" || g.GoldPending != nil || g.Fishing.Pending != nil || g.RollID != 1 || g.Fishing.LastRollID != 1 || g.Fishing.Tokens.BootOwner != 1 || len(g.Fishing.Tokens.Hands[1]) != 6 || len(g.Fishing.Tokens.Hands[2]) != 7 {
						t.Fatal("final continuation/boot/fish state")
					}
					afterMap, _ := json.Marshal(g.NewWorldMap())
					if string(afterMap) != string(originalMap) {
						t.Fatal("approved gold map changed")
					}
					for color, bank := range g.Bank {
						total := bank
						for _, seat := range g.Players {
							total += seat.Resources[color]
						}
						if total != 19 {
							t.Fatal("resource supply lost")
						}
					}
				})
			}
		}
	}
}
