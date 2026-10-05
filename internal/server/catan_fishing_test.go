package server

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func fishingResponseFixture(t *testing.T, r *Room, now time.Time) {
	t.Helper()
	fishingResponseVariant(t, r, now, false)
}

func fishingResponseVariant(t *testing.T, r *Room, now time.Time, city bool) {
	t.Helper()
	state, err := game.NewCatanFishing(3, game.CatanOptions{})
	if city {
		state, err = game.NewCatanFishingCitiesKnights(3, game.CatanOptions{})
	}
	if err != nil {
		t.Fatal(err)
	}
	g := state.Catan
	if city {
		g.CitiesKnights.Players[1].Improvements[game.CatanScience] = 3
		for i := range g.Tiles {
			if g.Tiles[i].Resource < 5 {
				g.Tiles[i].Number = 6
			}
		}
	}
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	state.Turn, state.Phase = 0, "catan_roll"
	f := g.Fishing
	vertices := g.Tiles[f.Map.Lakes[0].Tile].Vertices
	g.Vertices[vertices[0]].Owner, g.Vertices[vertices[0]].Level = 1, 2
	g.Vertices[vertices[3]].Owner, g.Vertices[vertices[3]].Level = 2, 1
	for player, ids := range map[int][]int{1: {0, 1, 2, 3, 4, 5, 6}, 2: {11, 12, 13, 14, 15, 16, 17}} {
		for _, id := range ids {
			at := slices.Index(f.Tokens.DrawPile, id)
			f.Tokens.DrawPile = slices.Delete(f.Tokens.DrawPile, at, at+1)
			f.Tokens.Hands[player] = append(f.Tokens.Hands[player], id)
		}
	}
	// The first blind replacement is the boot: it must consume the sole
	// replacement and never restart this city's remaining claim.
	at := slices.Index(f.Tokens.DrawPile, 29)
	f.Tokens.DrawPile = slices.Delete(f.Tokens.DrawPile, at, at+1)
	f.Tokens.DrawPile = append(f.Tokens.DrawPile, 29)
	initial, _ := json.Marshal(state)
	// Enter through the real roll action. Restore the initial fixture until
	// its lake produces; no arbitrary client-controlled dice/production API.
	for attempts := 0; attempts < 1000; attempts++ {
		var next game.State
		if err := json.Unmarshal(initial, &next); err != nil {
			t.Fatal(err)
		}
		if err := next.Apply(0, game.Action{Type: "catan_roll"}); err != nil {
			t.Fatal(err)
		}
		if next.CatanPendingActor() == 1 && next.Phase == "catan_fish_replace" {
			state = &next
			break
		}
	}
	if state.CatanPendingActor() != 1 {
		t.Fatal("fixture did not roll lake production")
	}
	r.Game = state
	r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
	if !r.adjustCatanResponseClock("catan_roll", -1, g.SetupStep, now) || r.CatanTimeLeft != 45000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("fishing response did not pause action clock")
	}
}

func TestCatanFishingHTTPRestartManualAutoplayTimeoutAndPrivacy(t *testing.T) {
	for _, mode := range []string{"manual", "autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := newCatanTable(t)
			clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			s.mu.Lock()
			fishingResponseFixture(t, s.rooms[id], time.Now())
			if err := s.save(s.rooms[id]); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			resources, _ := json.Marshal(s.rooms[id].Game.Catan.Players)
			checkPrivacy := func() {
				t.Helper()
				for viewer, c := range clients {
					v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					f := v["fishing"].(map[string]any)
					tokens := f["tokens"].(map[string]any)
					if tokens["drawPile"] != nil || tokens["hands"] != nil || f["pending"] != nil || f["lastRollId"] != nil || f["started"] != nil {
						t.Fatal("private fishing state exposed")
					}
					for p, raw := range tokens["players"].([]any) {
						_, visible := raw.(map[string]any)["tokens"]
						if visible != (p == viewer && len(s.rooms[id].Game.Catan.Fishing.Tokens.Hands[p]) > 0) {
							t.Fatal("fish face privacy", viewer, p)
						}
					}
					if f["canReplace"] != (viewer == s.rooms[id].Game.CatanPendingActor()) {
						t.Fatal("wrong replace permissions")
					}
				}
			}
			restart := func() {
				t.Helper()
				before, _ := json.Marshal(s.rooms[id])
				ts.Close()
				s.Close()
				next, err := New(s.cfg, s.files)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = next.Close() })
				stopBotTicker(next)
				after, _ := json.Marshal(next.rooms[id])
				if string(before) != string(after) {
					t.Fatal("fishing room changed on restart")
				}
				nextHTTP := httptest.NewServer(next.Handler())
				t.Cleanup(nextHTTP.Close)
				for _, c := range clients {
					c.base = nextHTTP.URL
				}
				s, ts = next, nextHTTP
				checkPrivacy()
			}
			before, _ := json.Marshal(s.rooms[id])
			for _, p := range []int{0, 2, 3} {
				clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_fish_replace", Card: 0}, 400)
			}
			clients[1].command(current(clients[1]), "action", game.Action{Type: "catan_fish_replace", Card: 29}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid response changed game or timer")
			}
			checkPrivacy()
			restart()
			for step := 0; step < 2; step++ {
				r := s.rooms[id]
				actor := r.Game.CatanPendingActor()
				if actor != step+1 {
					t.Fatal("wrong responder order")
				}
				resolved := time.Now()
				if mode == "manual" {
					a := game.Action{Type: "catan_fish_keep"}
					if actor == 1 {
						a = game.Action{Type: "catan_fish_replace", Card: 0}
					}
					clients[actor].command(current(clients[actor]), "action", a, 200)
				} else if mode == "autoplay" {
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(resolved)
					s.mu.Unlock()
				} else {
					resolved = time.UnixMilli(r.TurnDeadline)
					s.mu.Lock()
					s.expireSetups(resolved)
					s.mu.Unlock()
				}
				r = s.rooms[id]
				want := int64(45000)
				if step == 0 {
					want = 120000
				}
				remaining := r.TurnDeadline - resolved.UnixMilli()
				if remaining < want || remaining > want+1000 || r.CatanTimeLeft != 45000 {
					t.Fatal("fishing response clock", remaining, want)
				}
				if r.Game.Turn != 0 || r.Game.Catan.Fishing.Tokens.BootOwner != 1 || len(r.Game.Catan.Fishing.Tokens.Hands[1]) != 6 {
					t.Fatal("replacement boot incorrectly redrew/changed turn")
				}
				checkPrivacy()
				if step == 0 {
					restart()
				}
			}
			r := s.rooms[id]
			g := r.Game.Catan
			if r.Game.Phase != "catan_turn" || g.Fishing.Pending != nil || r.Game.CatanPendingActor() != -1 || g.RollID != 1 || g.Fishing.LastRollID != 1 {
				t.Fatal("production failed to finish once")
			}
			got, _ := json.Marshal(g.Players)
			if string(got) != string(resources) {
				t.Fatal("resources were repeated after fish response")
			}
			if !reflect.DeepEqual(g.Fishing.Tokens.Discard, []int{0}) {
				t.Fatal("wrong faceup discard")
			}
			restart()
		})
	}
}
