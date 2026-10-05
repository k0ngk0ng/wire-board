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

func fishingTribeRewardState(t *testing.T, n int, phase string) (*game.State, game.Action) {
	t.Helper()
	state, err := game.NewCatanFishingSeafarers(n, game.CatanOptions{}, game.CatanSeafarersSetup{Scenario: "tribe", Layout: "fixed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g := state.Catan
	state.Turn, state.Phase = 0, phase
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	g.Seafarers.Pirate = -1
	edge := g.Seafarers.Tribe.Ports[0].Edge
	root := g.Fishing.Map.Grounds[0].Vertices[0]
	g.Vertices[root].Owner, g.Vertices[root].Level = 0, 1
	sea := func(edge int) bool {
		e := g.Edges[edge]
		count := 0
		for _, tile := range g.Tiles {
			if slices.Contains(tile.Vertices, e.A) && slices.Contains(tile.Vertices, e.B) {
				count++
				if tile.Resource == game.CatanSea {
					return true
				}
			}
		}
		return count == 1
	}
	previous := map[int]int{root: -1}
	via := map[int]int{}
	queue := []int{root}
	end := -1
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		if v == g.Edges[edge].A || v == g.Edges[edge].B {
			end = v
			break
		}
		for _, e := range g.Edges {
			if e.ID == edge || !sea(e.ID) {
				continue
			}
			next := -1
			if e.A == v {
				next = e.B
			} else if e.B == v {
				next = e.A
			}
			if next < 0 {
				continue
			}
			if _, ok := previous[next]; ok {
				continue
			}
			previous[next], via[next] = v, e.ID
			queue = append(queue, next)
		}
	}
	if end < 0 {
		t.Fatal("no water path")
	}
	for v := end; v != root; v = previous[v] {
		e := via[v]
		g.Edges[e].Owner, g.Edges[e].Ship = 0, true
	}
	used := map[int]bool{}
	for _, ground := range g.Fishing.Map.Grounds {
		for _, e := range ground.Edges {
			used[e] = true
		}
	}
	// Another mainland coastal settlement provides a free destination port edge.
	for _, e := range g.Edges {
		if used[e.ID] || !sea(e.ID) {
			continue
		}
		main := false
		for _, tile := range g.Tiles {
			if tile.Number > 0 && slices.Contains(tile.Vertices, e.A) && slices.Contains(tile.Vertices, e.B) {
				main = true
			}
		}
		if !main {
			continue
		}
		g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 0, 1
		break
	}
	for _, token := range []int{0, 11, 21} {
		at := slices.Index(g.Fishing.Tokens.DrawPile, token)
		g.Fishing.Tokens.DrawPile = slices.Delete(g.Fishing.Tokens.DrawPile, at, at+1)
		g.Fishing.Tokens.Hands[0] = append(g.Fishing.Tokens.Hands[0], token)
	}
	return state, game.Action{Type: "catan_fish_ship", Edge: edge, Tokens: []int{11, 21}}
}

func TestCatanFishingTribeRewardHTTPRestartAndResponses(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, phase := range []string{"catan_roll", "catan_turn"} {
			for _, mode := range []string{"manual", "autoplay", "timeout"} {
				t.Run(fmt.Sprintf("%d/%s/%s", n, phase, mode), func(t *testing.T) {
					s, ts, clients, id, _, _ := newFishingActionTable(t, n, phase, "ship")
					state, a := fishingTribeRewardState(t, n, phase)
					s.mu.Lock()
					r := s.rooms[id]
					r.Game = state
					r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
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
							t.Fatal("room/map/payment/continuation changed on restart")
						}
					}
					clients[0].command(current(clients[0]), "action", a, 200)
					r = s.rooms[id]
					g := r.Game.Catan
					if r.Game.Phase != "catan_port" || g.Seafarers.Tribe.Pending.Resume != phase || r.CatanTimeLeft < 44000 || r.CatanTimeLeft > 45000 || r.TurnDeadline-time.Now().UnixMilli() < 119000 {
						t.Fatal("port response clock/phase")
					}
					if !slices.Equal(g.Fishing.Tokens.Discard, []int{11, 21}) || !g.Edges[a.Edge].Ship || g.Edges[a.Edge].Owner != 0 {
						t.Fatal("fish payment or reward ship missing")
					}
					fishMap, _ := json.Marshal(g.Fishing.Map)
					left := r.CatanTimeLeft
					restart()
					for viewer, c := range clients {
						v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
						f := v["fishing"].(map[string]any)
						tokens := f["tokens"].(map[string]any)
						if f["pending"] != nil || tokens["drawPile"] != nil {
							t.Fatal("fish privacy")
						}
						for p, raw := range tokens["players"].([]any) {
							_, shown := raw.(map[string]any)["tokens"]
							if shown != (p == 0 && viewer == 0) {
								t.Fatal("private fish faces")
							}
						}
						tr := v["seafarers"].(map[string]any)["tribe"].(map[string]any)
						for _, raw := range tr["development"].([]any) {
							if raw.(map[string]any)["card"] != nil {
								t.Fatal("private map development")
							}
						}
						ports := v["legal"].(map[string]any)["ports"].([]any)
						if (len(ports) > 0) != (viewer == 0) {
							t.Fatal("port actor hints")
						}
					}
					raw := current(clients[0])
					v := raw["game"].(map[string]any)["catan"].(map[string]any)
					ports := v["legal"].(map[string]any)["ports"].([]any)
					if len(ports) == 0 {
						t.Fatal("no legal port")
					}
					portAction := game.Action{Type: "catan_port", Edge: int(ports[0].(float64))}
					before, _ := json.Marshal(s.rooms[id])
					clients[1].command(current(clients[1]), "action", portAction, 400)
					clients[n].command(current(clients[n]), "action", portAction, 400)
					bad := portAction
					bad.Edge = s.rooms[id].Game.Catan.Fishing.Map.Grounds[0].Edges[0]
					clients[0].command(current(clients[0]), "action", bad, 400)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("invalid action mutated room")
					}
					at := time.Now()
					switch mode {
					case "manual":
						clients[0].command(current(clients[0]), "action", portAction, 200)
					case "autoplay":
						setAutoPlay(clients[0], current(clients[0]), true, 200)
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
					g = r.Game.Catan
					remaining := r.TurnDeadline - at.UnixMilli()
					if r.Game.Phase != phase || r.Game.Turn != 0 || g.Seafarers.Tribe.Pending != nil || remaining < left || remaining > left+1000 {
						t.Fatal("port response did not restore paused turn", r.Game.Phase, remaining, left)
					}
					currentMap, _ := json.Marshal(g.Fishing.Map)
					if string(currentMap) != string(fishMap) || !slices.Equal(g.Fishing.Tokens.Discard, []int{11, 21}) || len(g.Ports) != 1 || len(g.Seafarers.Tribe.HeldPorts[0]) != 0 {
						t.Fatal("port changed grounds/repaid fish")
					}
					for _, ground := range g.Fishing.Map.Grounds {
						if slices.Contains(ground.Edges[:], g.Ports[0].Edge) {
							t.Fatal("port overlaps fishing ground")
						}
					}
					restart()
				})
			}
		}
	}
}
