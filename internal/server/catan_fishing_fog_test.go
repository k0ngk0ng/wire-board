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

func fogFishingResponseFixture(t *testing.T, r *Room, n int, now time.Time) {
	t.Helper()
	state, err := game.NewCatanFishingSeafarers(n, game.CatanOptions{FiveSix: n > 4}, game.CatanSeafarersSetup{Scenario: "fog"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g := state.Catan
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	state.Turn, state.Phase = 0, "catan_roll"
	if n > 4 {
		g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 0, 3, false
	}
	g.Seafarers.Pirate = -1
	for i := range g.Tiles {
		if g.Tiles[i].Resource < 5 {
			g.Tiles[i].Number = 2
		}
	}
	ground := g.Fishing.Map.Grounds[0]
	g.Vertices[ground.Vertices[0]].Owner, g.Vertices[ground.Vertices[0]].Level = 1, 2
	g.Vertices[ground.Vertices[2]].Owner, g.Vertices[ground.Vertices[2]].Level = 2, 1
	// Represent an already explored gold hex, preserving both hidden stacks.
	fog := g.Seafarers.Fog
	goldTile := slices.IndexFunc(g.Tiles, func(tile game.CatanTile) bool { return tile.Resource == game.CatanFog })
	atGold := slices.Index(fog.Terrain, game.CatanGold)
	fog.Terrain = slices.Delete(fog.Terrain, atGold, atGold+1)
	atNumber := slices.Index(fog.Numbers, ground.Number)
	fog.Numbers = slices.Delete(fog.Numbers, atNumber, atNumber+1)
	g.Tiles[goldTile].Resource, g.Tiles[goldTile].Number = game.CatanGold, ground.Number
	g.Vertices[g.Tiles[goldTile].Vertices[0]].Owner, g.Vertices[g.Tiles[goldTile].Vertices[0]].Level = 1, 2
	g.Vertices[g.Tiles[goldTile].Vertices[3]].Owner, g.Vertices[g.Tiles[goldTile].Vertices[3]].Level = 2, 1
	ordinary := fog.StartTiles[0]
	g.Tiles[ordinary].Number = ground.Number
	for _, vertex := range g.Tiles[ordinary].Vertices {
		if g.Vertices[vertex].Owner < 0 {
			g.Vertices[vertex].Owner, g.Vertices[vertex].Level = 0, 1
			break
		}
	}
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
func TestCatanFishingFogGoldResponseHTTP(t *testing.T) {
	for _, n := range []int{3, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) { testFishingFogGoldResponseHTTP(t, n) })
	}
}
func testFishingFogGoldResponseHTTP(t *testing.T, n int) {
	for _, mode := range []string{"manual", "autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
			s.mu.Lock()
			fogFishingResponseFixture(t, s.rooms[id], n, time.Now())
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
			ordinary := make([]int, n)
			for p, seat := range s.rooms[id].Game.Catan.Players {
				for _, count := range seat.Resources {
					ordinary[p] += count
				}
			}
			for step, actor := range []int{1, 2, 1, 2} {
				r := s.rooms[id]
				phase := "catan_fish_replace"
				if step >= 2 {
					phase = "catan_gold"
				}
				if r.Game.Phase != phase || r.Game.CatanPendingActor() != actor {
					t.Fatal("responder sequence")
				}
				a := game.Action{Type: "catan_fish_replace", Card: 0}
				if actor == 2 {
					a = game.Action{Type: "catan_fish_keep"}
				}
				if step >= 2 {
					var err error
					a, err = r.Game.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
				}
				for viewer, c := range clients {
					v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					fish := v["fishing"].(map[string]any)
					if fish["pending"] != nil {
						t.Fatal("saved continuation leaked")
					}
					tokens := fish["tokens"].(map[string]any)
					if tokens["drawPile"] != nil {
						t.Fatal("fish pile leaked")
					}
					for seat, raw := range tokens["players"].([]any) {
						_, visible := raw.(map[string]any)["tokens"]
						if visible != (viewer == seat && (seat == 1 || seat == 2)) {
							t.Fatal("fish faces leaked")
						}
					}
					fog := v["seafarers"].(map[string]any)["fog"].(map[string]any)
					if fog["terrain"] != nil || fog["numbers"] != nil {
						t.Fatal("exploration stack leaked")
					}
				}
				before, _ := json.Marshal(r)
				clients[0].command(current(clients[0]), "action", a, 400)
				clients[n].command(current(clients[n]), "action", a, 400)
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
				if step == 3 {
					want = 45000
				}
				remaining := r.TurnDeadline - at.UnixMilli()
				if remaining < want || remaining > want+1000 || r.Game.Turn != 0 || r.CatanTimeLeft != 45000 {
					t.Fatal("response timer", step, remaining)
				}
				restart()
			}
			g := s.rooms[id].Game.Catan
			if s.rooms[id].Game.Phase != "catan_turn" || g.Fishing.LastRollID != 1 || g.RollID != 1 || g.Fishing.Pending != nil || g.GoldPending != nil || s.rooms[id].Game.CatanPendingActor() != -1 {
				t.Fatal("duplicate production or lost turn")
			}
			for p, seat := range g.Players {
				count := 0
				for _, n := range seat.Resources {
					count += n
				}
				gold := 0
				if p == 1 || p == 2 {
					gold = 3 - p
				}
				if count != ordinary[p]+gold {
					t.Fatal("lost or duplicate payout", p, count, ordinary[p], gold)
				}
			}
			if n > 4 {
				before, _ := json.Marshal(g.Fishing.Tokens)
				at := time.Now()
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_end"}, 200)
				r := s.rooms[id]
				after, _ := json.Marshal(r.Game.Catan.Fishing.Tokens)
				if string(before) != string(after) || r.Game.Catan.RollID != 1 || r.Game.Turn != 3 || r.Game.Phase != "catan_turn" || !r.Game.Catan.Paired.Second {
					t.Fatal("secondary action lost or repeated production")
				}
				remaining := r.TurnDeadline - at.UnixMilli()
				if remaining < 120000 || remaining > 121000 {
					t.Fatal("secondary clock", remaining)
				}
				clients[3].command(current(clients[3]), "action", game.Action{Type: "catan_roll"}, 400)
				restart()
			}
		})
	}
}
