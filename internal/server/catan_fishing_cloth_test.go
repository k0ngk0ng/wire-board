package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func fishingClothResponseState(t *testing.T, n int, layout string) (*game.State, int) {
	t.Helper()
	state, err := game.NewCatanFishingSeafarers(n, game.CatanOptions{}, game.CatanSeafarersSetup{Scenario: "cloth", Layout: layout}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g := state.Catan
	state.Turn, state.Phase = 0, "catan_roll"
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	g.Robber, g.Seafarers.Pirate = -1, -1
	ground := g.Fishing.Map.Grounds[0]
	g.Vertices[ground.Vertices[0]].Owner, g.Vertices[ground.Vertices[0]].Level = 0, 1
	g.Vertices[ground.Vertices[2]].Owner, g.Vertices[ground.Vertices[2]].Level = 1, 2
	cloth := g.Seafarers.Cloth
	village := slices.IndexFunc(cloth.Villages, func(v game.CatanClothVillage) bool { return v.Number == ground.Number })
	if village < 0 {
		t.Fatal("missing matching cloth village")
	}
	cloth.Villages[village].Traders = []int{0, 1}
	// Model the cloth already received when each trade route was established.
	cloth.Held[0], cloth.Held[1], cloth.Villages[village].Stock = 1, 1, 3
	// Ensure this roll also produces ordinary resources, independently of layout.
	for _, tile := range cloth.HomeTiles {
		if g.Tiles[tile].Resource >= 5 {
			continue
		}
		g.Tiles[tile].Number = ground.Number
		for _, vertex := range g.Tiles[tile].Vertices {
			if g.Vertices[vertex].Owner < 0 {
				g.Vertices[vertex].Owner, g.Vertices[vertex].Level = 2, 1
				break
			}
		}
		break
	}
	f := &g.Fishing.Tokens
	for player, ids := range map[int][]int{0: {0, 1, 2, 3, 4, 5, 6}, 1: {11, 12, 13, 14, 15, 16, 17}} {
		for _, token := range ids {
			at := slices.Index(f.DrawPile, token)
			f.DrawPile = slices.Delete(f.DrawPile, at, at+1)
			f.Hands[player] = append(f.Hands[player], token)
		}
	}
	// Fix only the fish sequence; roll through the real action entry point.
	for _, token := range []int{23, 22, 21} {
		at := slices.Index(f.DrawPile, token)
		f.DrawPile = slices.Delete(f.DrawPile, at, at+1)
		f.DrawPile = append(f.DrawPile, token)
	}
	initial, _ := json.Marshal(state)
	for attempt := 0; attempt < 1000; attempt++ {
		var next game.State
		if err = json.Unmarshal(initial, &next); err != nil {
			t.Fatal(err)
		}
		if err = next.Apply(0, game.Action{Type: "catan_roll"}); err != nil {
			t.Fatal(err)
		}
		if next.Catan.Dice[0]+next.Catan.Dice[1] == ground.Number {
			if next.Phase != "catan_fish_replace" || next.CatanPendingActor() != 0 {
				t.Fatal("missing first fish response")
			}
			return &next, village
		}
	}
	t.Fatal("did not roll matching production")
	return nil, -1
}

func TestCatanFishingClothProductionHTTPRestartAndResponses(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"fixed", "variable"} {
			for _, mode := range []string{"manual", "autoplay", "timeout"} {
				t.Run(fmt.Sprintf("%d/%s/%s", n, layout, mode), func(t *testing.T) {
					s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_roll", "ship")
					state, village := fishingClothResponseState(t, n, layout)
					now := time.Now()
					s.mu.Lock()
					r := s.rooms[id]
					r.Game = state
					r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
					if !r.adjustCatanResponseClock("catan_roll", -1, state.Catan.SetupStep, now) || r.CatanTimeLeft != 45000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
						t.Fatal("response did not pause turn for 120 seconds")
					}
					if err := s.save(r); err != nil {
						t.Fatal(err)
					}
					s.mu.Unlock()
					g := state.Catan
					cloth := g.Seafarers.Cloth
					if cloth.Held[0] != 2 || cloth.Held[1] != 2 || cloth.Villages[village].Stock != 1 {
						t.Fatal("cloth was not paid before fish response")
					}
					resources := make([][]int, n)
					for p, player := range g.Players {
						resources[p] = slices.Clone(player.Resources)
					}
					if slices.Equal(resources[2], []int{0, 0, 0, 0, 0}) {
						t.Fatal("ordinary production fixture missing")
					}
					clothBefore, _ := json.Marshal(cloth)
					mapBefore, _ := json.Marshal(g.Fishing.Map)
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
							t.Fatal("restart changed fish/cloth/resources/timer")
						}
					}
					restart()
					for _, actor := range []int{0, 1} {
						r = s.rooms[id]
						if r.Game.Phase != "catan_fish_replace" || r.Game.CatanPendingActor() != actor {
							t.Fatal("wrong response sequence")
						}
						for viewer, c := range clients {
							v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
							fish := v["fishing"].(map[string]any)
							tokens := fish["tokens"].(map[string]any)
							if fish["pending"] != nil || tokens["drawPile"] != nil || fish["canReplace"] != (viewer == actor) {
								t.Fatal("private continuation/pile or response permission")
							}
							for p, raw := range tokens["players"].([]any) {
								_, shown := raw.(map[string]any)["tokens"]
								if shown != (p < 2 && p == viewer) {
									t.Fatal("fish faces leaked")
								}
							}
						}
						a := game.Action{Type: "catan_fish_keep"}
						if actor == 0 {
							a = game.Action{Type: "catan_fish_replace", Card: 0}
						}
						before, _ := json.Marshal(r)
						clients[2].command(current(clients[2]), "action", a, 400)
						clients[n].command(current(clients[n]), "action", a, 400)
						clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_fish_replace", Card: 999}, 400)
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("invalid action mutated state/timer")
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
							at = time.UnixMilli(s.rooms[id].TurnDeadline)
							s.mu.Lock()
							s.expireSetups(at)
							s.mu.Unlock()
						}
						r = s.rooms[id]
						want := int64(120000)
						if actor == 1 {
							want = 45000
						}
						left := r.TurnDeadline - at.UnixMilli()
						if left < want || left > want+1000 || r.Game.Turn != 0 || r.CatanTimeLeft != 45000 {
							t.Fatal("response timer lost", actor, left)
						}
						restart()
					}
					r = s.rooms[id]
					g = r.Game.Catan
					clothAfter, _ := json.Marshal(g.Seafarers.Cloth)
					mapAfter, _ := json.Marshal(g.Fishing.Map)
					if r.Game.Phase != "catan_turn" || r.Game.CatanPendingActor() != -1 || g.Fishing.Pending != nil || g.RollID != 1 || g.Fishing.LastRollID != 1 || string(clothBefore) != string(clothAfter) || string(mapBefore) != string(mapAfter) {
						t.Fatal("lost turn/map or duplicate cloth production")
					}
					for p, player := range g.Players {
						if !reflect.DeepEqual(resources[p], player.Resources) {
							t.Fatal("ordinary resources awarded twice")
						}
					}
					if len(g.Fishing.Tokens.Hands[0]) != 7 || len(g.Fishing.Tokens.Hands[1]) != 7 || !slices.Contains(g.Fishing.Tokens.Hands[0], 21) || slices.Contains(g.Fishing.Tokens.Hands[0], 0) {
						t.Fatal("fish replacement lost or hand limit exceeded")
					}
				})
			}
		}
	}
}
