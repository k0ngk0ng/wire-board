package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// An explicit midgame fixture on unchanged printed terrain/discs. The two
// ground endpoints are nonadjacent; extended maps add a second gold claimant
// on their separate 6-point gold islet. No placement is claimed as natural play.
func fishingWondersGoldResponseState(t *testing.T, n, bankLimit int) (*game.State, []int) {
	t.Helper()
	options := game.CatanOptions{FiveSix: n > 4}
	setup := game.CatanSeafarersSetup{Scenario: "wonders"}
	state, err := game.NewCatanFishingSeafarers(n, options, setup, nil)
	if err != nil {
		t.Fatal(err)
	}
	coasts := state.Catan.FishingCoasts()
	var placements []game.CatanFishingGroundPlacement
	goldID := -1
	for _, coast := range coasts {
		for _, tile := range state.Catan.Tiles {
			if tile.Resource != game.CatanGold || tile.Number != 6 || !slices.Contains(tile.Vertices, coast.Vertices[1]) {
				continue
			}
			left := []int{4, 5, 8, 9, 10}
			if n > 4 {
				left = []int{4, 5, 5, 8, 9, 9, 10}
			}
			placements = []game.CatanFishingGroundPlacement{{Number: 6, Edges: coast.Edges}}
			used := map[int]bool{coast.Edges[0]: true, coast.Edges[1]: true}
			for _, other := range coasts {
				if len(placements) == len(left)+1 {
					break
				}
				if used[other.Edges[0]] || used[other.Edges[1]] {
					continue
				}
				placements = append(placements, game.CatanFishingGroundPlacement{Number: left[len(placements)-1], Edges: other.Edges})
				used[other.Edges[0]], used[other.Edges[1]] = true, true
			}
			if len(placements) == len(left)+1 {
				goldID = tile.ID
				break
			}
		}
		if goldID >= 0 {
			break
		}
	}
	if goldID < 0 {
		t.Fatal("missing gold coast")
	}
	state, err = game.NewCatanFishingSeafarers(n, options, setup, placements)
	if err != nil {
		t.Fatal(err)
	}
	g := state.Catan
	state.Turn, state.Phase = 0, "catan_roll"
	g.SetupStep, g.TurnSerial, g.Robber = g.SetupLimit(), 1, -1
	if n > 4 {
		g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 0, 3, false
	}
	for p := range g.Fishing.Started {
		g.Fishing.Started[p] = true
	}
	ground := g.Fishing.Map.Grounds[0]
	a, b := ground.Vertices[0], ground.Vertices[2]
	if !slices.Contains(g.Tiles[goldID].Vertices, a) {
		a, b = b, a
	}
	if !slices.Contains(g.Tiles[goldID].Vertices, a) || slices.Contains(g.Tiles[goldID].Vertices, b) {
		t.Fatal("gold coast endpoints")
	}
	g.Vertices[a].Owner, g.Vertices[a].Level = 1, 2
	g.Vertices[b].Owner, g.Vertices[b].Level = 2, 1
	claims := make([]int, n)
	claims[1] = 2
	place := func(player int, tile game.CatanTile) bool {
		for _, v := range tile.Vertices {
			ok := g.Vertices[v].Level == 0
			for _, e := range g.Edges {
				if e.A == v && g.Vertices[e.B].Level > 0 || e.B == v && g.Vertices[e.A].Level > 0 {
					ok = false
				}
			}
			if ok {
				g.Vertices[v].Owner, g.Vertices[v].Level = player, 1
				return true
			}
		}
		return false
	}
	if n > 4 {
		for _, tile := range g.Tiles {
			if tile.ID != goldID && tile.Resource == game.CatanGold && tile.Number == 6 && place(2, tile) {
				claims[2] = 1
				break
			}
		}
		if claims[2] != 1 {
			t.Fatal("missing second gold islet")
		}
	}
	ordinary := false
	for _, tile := range g.Tiles {
		if tile.Resource < 5 && tile.Number == 6 && place(0, tile) {
			ordinary = true
			break
		}
	}
	if !ordinary {
		t.Fatal("missing ordinary producer")
	}
	for _, edge := range g.Edges {
		if g.Vertices[edge.A].Level > 0 && g.Vertices[edge.B].Level > 0 {
			t.Fatal("building distance")
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
	if bankLimit >= 0 {
		for c := range g.Bank {
			left := 0
			if c == 4 {
				left = bankLimit
			}
			g.Players[0].Resources[c] += g.Bank[c] - left
			g.Bank[c] = left
		}
	}
	saved, _ := json.Marshal(state)
	for attempt := 0; attempt < 1000; attempt++ {
		var next game.State
		if err := json.Unmarshal(saved, &next); err != nil {
			t.Fatal(err)
		}
		if err := next.Apply(0, game.Action{Type: "catan_roll"}); err != nil {
			t.Fatal(err)
		}
		if next.Catan.Dice[0]+next.Catan.Dice[1] == 6 {
			if next.Phase != "catan_fish_replace" || next.CatanPendingActor() != 1 {
				t.Fatal("fish production queue")
			}
			return &next, claims
		}
	}
	t.Fatal("could not roll six")
	return nil, nil
}

func TestCatanFishingWondersGoldHTTPClockAndRestart(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, bank := range []int{-1, 1, 0} {
			for _, mode := range []string{"manual", "autoplay", "timeout"} {
				t.Run(fmt.Sprintf("%d/bank%d/%s", n, bank, mode), func(t *testing.T) {
					s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_roll", "resource")
					state, claims := fishingWondersGoldResponseState(t, n, bank)
					now := time.Now()
					s.mu.Lock()
					r := s.rooms[id]
					r.Game = state
					r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
					if !r.adjustCatanResponseClock("catan_roll", -1, state.Catan.SetupStep, now) || r.CatanTimeLeft != 45000 {
						t.Fatal("response clock entry")
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
							t.Fatal("response restart changed room")
						}
					}
					mapState := func(g *game.Catan) string {
						v, _ := json.Marshal([]any{g.Tiles, g.Ports, g.Fishing.Map, g.Seafarers.Wonders})
						return string(v)
					}
					originalMap := mapState(state.Catan)
					resources := make([]int, n)
					for p, seat := range state.Catan.Players {
						for _, c := range seat.Resources {
							resources[p] += c
						}
					}
					actors := []int{1, 2}
					if bank != 0 {
						actors = append(actors, 1)
						if bank < 0 && claims[2] > 0 {
							actors = append(actors, 2)
						}
					}
					restart()
					for step, actor := range actors {
						r = s.rooms[id]
						phase := "catan_fish_replace"
						count := 0
						a := game.Action{Type: "catan_fish_keep"}
						if step == 0 {
							a = game.Action{Type: "catan_fish_replace", Card: 0}
						}
						if step >= 2 {
							phase = "catan_gold"
							count = claims[actor]
							if bank == 1 {
								count = 1
							}
							a = game.Action{Type: "catan_gold", Take: []int{0, 0, 0, 0, count}}
						}
						if r.Game.Phase != phase || r.Game.CatanPendingActor() != actor || r.Game.Turn != 0 {
							t.Fatal("response order", step, r.Game.Phase, r.Game.CatanPendingActor())
						}
						for viewer, c := range clients {
							v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
							f := v["fishing"].(map[string]any)
							tokens := f["tokens"].(map[string]any)
							if f["pending"] != nil || tokens["drawPile"] != nil || f["canReplace"] != (step < 2 && viewer == actor) {
								t.Fatal("private queue or wrong response permission")
							}
							for p, raw := range tokens["players"].([]any) {
								_, visible := raw.(map[string]any)["tokens"]
								if visible != (p == viewer && (p == 1 || p == 2)) {
									t.Fatal("fish privacy")
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
							t.Fatal("invalid response mutated room")
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
							t.Fatal("response deadline", step, left)
						}
						resources[actor] += count
						for p, seat := range r.Game.Catan.Players {
							total := 0
							for _, v := range seat.Resources {
								total += v
							}
							if total != resources[p] {
								t.Fatal("resource payout repeated/missing", step, p, total, resources[p])
							}
						}
						restart()
					}
					g := s.rooms[id].Game.Catan
					if s.rooms[id].Game.Phase != "catan_turn" || g.GoldPending != nil || g.Fishing.Pending != nil || g.RollID != 1 || g.Fishing.LastRollID != 1 || g.Fishing.Tokens.BootOwner != 1 || len(g.Fishing.Tokens.Hands[1]) != 6 || len(g.Fishing.Tokens.Hands[2]) != 7 || mapState(g) != originalMap {
						t.Fatal("final map/boot/response state")
					}
					for c, stock := range g.Bank {
						total := stock
						for _, seat := range g.Players {
							total += seat.Resources[c]
						}
						want := 19
						if n > 4 {
							want = 24
						}
						if total != want {
							t.Fatal("resource inventory")
						}
					}
					if n > 4 {
						before, _ := json.Marshal(g.Fishing.Tokens)
						at := time.Now()
						clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_end"}, 200)
						r = s.rooms[id]
						after, _ := json.Marshal(r.Game.Catan.Fishing.Tokens)
						if string(before) != string(after) || r.Game.Turn != 3 || !r.Game.Catan.Paired.Second || r.Game.Phase != "catan_turn" || r.Game.Catan.RollID != 1 {
							t.Fatal("secondary repeated production")
						}
						left := r.TurnDeadline - at.UnixMilli()
						if left < 120000 || left > 121000 {
							t.Fatal("secondary clock", left)
						}
						clients[3].command(current(clients[3]), "action", game.Action{Type: "catan_roll"}, 400)
						restart()
					}
				})
			}
		}
	}
}
