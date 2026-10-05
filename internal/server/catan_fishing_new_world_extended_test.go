package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func fishingWorldExtendedResponseState(t *testing.T, n, number int) *game.State {
	t.Helper()
	data, err := os.ReadFile("../game/testdata/catan-new-world-five-six.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Layout                               game.CatanNewWorldMap
		PortEdges, FishVertices, FishNumbers []int
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	state, err := game.NewCatanFishingNewWorld(n, game.CatanOptions{FiveSix: true}, &fixture.Layout)
	if err != nil {
		t.Fatal(err)
	}
	state.Catan.Fishing.WorldSetup.Numbers = slices.Clone(fixture.FishNumbers)
	for _, edge := range fixture.PortEdges {
		if err = state.Apply(state.Turn, game.Action{Type: "catan_world_port", Edge: edge}); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range fixture.FishVertices {
		if err = state.Apply(state.Turn, game.Action{Type: "catan_world_fish", Vertex: v}); err != nil {
			t.Fatal(err)
		}
	}
	g := state.Catan
	state.Turn, state.Phase = n-1, "catan_roll"
	g.StartPlayer, g.SetupStep, g.TurnSerial = n-1, g.SetupLimit(), 1
	g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = n-1, (n+2)%n, false
	for p := range g.Fishing.Started {
		g.Fishing.Started[p] = true
	}
	for _, ground := range g.Fishing.Map.Grounds {
		if ground.Number == number {
			p, level := 0, 1
			if ground.SeaTile != nil {
				p, level = 1, 2
			}
			v := ground.Vertices[0]
			g.Vertices[v].Owner, g.Vertices[v].Level = p, level
			g.Players[p].Score = level
		}
	}
	for _, e := range g.Edges {
		if g.Vertices[e.A].Level > 0 && g.Vertices[e.B].Level > 0 {
			t.Fatal("fixture distance")
		}
	}
	f := &g.Fishing.Tokens
	for p, ids := range map[int][]int{0: {0, 1, 2, 3, 4, 5, 6}, 1: {11, 12, 13, 14, 15, 16, 17}, 2: {30, 39}} {
		for _, id := range ids {
			at := slices.Index(f.DrawPile, id)
			f.DrawPile = slices.Delete(f.DrawPile, at, at+1)
			f.Hands[p] = append(f.Hands[p], id)
		}
	}
	// Both hands are full before production; the first replacement draws a boot.
	at := slices.Index(f.DrawPile, 29)
	f.DrawPile = slices.Delete(f.DrawPile, at, at+1)
	f.DrawPile = append(f.DrawPile, 29)
	saved, _ := json.Marshal(state)
	for attempt := 0; attempt < 1000; attempt++ {
		var next game.State
		if err = json.Unmarshal(saved, &next); err != nil {
			t.Fatal(err)
		}
		if err = next.Apply(n-1, game.Action{Type: "catan_roll"}); err != nil {
			t.Fatal(err)
		}
		if next.Catan.Dice[0]+next.Catan.Dice[1] == number {
			if next.Phase != "catan_fish_replace" || next.CatanPendingActor() != 0 {
				t.Fatal("duplicate fish response")
			}
			return &next
		}
	}
	t.Fatal("failed to roll fixture number")
	return nil
}

func TestCatanFishingNewWorldExtendedResponseHTTP(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, number := range []int{5, 9} {
			for _, mode := range []string{"manual", "autoplay", "timeout"} {
				t.Run(fmt.Sprintf("%d/roll%d/%s", n, number, mode), func(t *testing.T) {
					s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
					state := fishingWorldExtendedResponseState(t, n, number)
					now := time.Now()
					s.mu.Lock()
					r := s.rooms[id]
					r.Game = state
					r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
					if !r.adjustCatanResponseClock("catan_roll", -1, state.Catan.SetupStep, now) || r.CatanTimeLeft != 45000 {
						t.Fatal("response clock not paused")
					}
					if err := s.save(r); err != nil {
						t.Fatal(err)
					}
					s.mu.Unlock()
					restarts := 0
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
							t.Fatal("restart changed map, response, paired markers or clock")
						}
						restarts++
					}
					restart()
					step := 0
					for s.rooms[id].Game.CatanPendingActor() >= 0 {
						r = s.rooms[id]
						actor := r.Game.CatanPendingActor()
						if step < 2 && (actor != step || r.Game.Phase != "catan_fish_replace") {
							t.Fatal("clockwise responders")
						}
						if step >= 2 && r.Game.Phase != "catan_gold" {
							t.Fatal("unexpected followup phase")
						}
						if step > 4 {
							t.Fatal("response did not terminate")
						}
						action, err := r.Game.BotAction(actor)
						if err != nil {
							t.Fatal(err)
						}
						if step == 0 {
							action = game.Action{Type: "catan_fish_replace", Card: 0}
						} else if step == 1 {
							action = game.Action{Type: "catan_fish_keep"}
						}
						for viewer, c := range clients {
							v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
							fish := v["fishing"].(map[string]any)
							tokens := fish["tokens"].(map[string]any)
							if fish["pending"] != nil || tokens["drawPile"] != nil || fish["worldSetup"].(map[string]any)["numbers"] != nil || fish["canReplace"] != (step < 2 && viewer == actor) {
								t.Fatal("private response/permissions exposed")
							}
							for p, raw := range tokens["players"].([]any) {
								_, visible := raw.(map[string]any)["tokens"]
								if visible != (p == viewer && (p == 0 || p == 1 || p == 2)) {
									t.Fatal("fish faces leaked")
								}
							}
						}
						before, _ := json.Marshal(r)
						clients[(actor+1)%n].command(current(clients[(actor+1)%n]), "action", action, 400)
						clients[n].command(current(clients[n]), "action", action, 400)
						clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_roll"}, 400)
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("illegal response changed room")
						}
						at := time.Now()
						switch mode {
						case "manual":
							clients[actor].command(current(clients[actor]), "action", action, 200)
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
						left := r.TurnDeadline - at.UnixMilli()
						want := int64(120000)
						if r.Game.CatanPendingActor() < 0 {
							want = 45000
						}
						if left < want || left > want+1000 || r.Game.Turn != n-1 || r.Game.Catan.Paired.Second {
							t.Fatal("response clock/paired marker", left, want)
						}
						step++
						restart()
					}
					r = s.rooms[id]
					if step < 2 || r.Game.Phase != "catan_turn" || r.CatanTimeLeft != 45000 {
						t.Fatal("response failed to resume")
					}
					if mode == "manual" && (r.Game.Catan.Fishing.Tokens.BootOwner != 0 || len(r.Game.Catan.Fishing.Tokens.Hands[0]) != 6 || len(r.Game.Catan.Fishing.Tokens.Hands[1]) != 7) {
						t.Fatal("boot replacement/decline")
					}
					fishBefore, _ := json.Marshal(r.Game.Catan.Fishing.Tokens)
					rollID := r.Game.Catan.RollID
					at := time.Now()
					clients[n-1].command(current(clients[n-1]), "action", game.Action{Type: "catan_end"}, 200)
					r = s.rooms[id]
					second := (n + 2) % n
					left := r.TurnDeadline - at.UnixMilli()
					fishAfter, _ := json.Marshal(r.Game.Catan.Fishing.Tokens)
					if r.Game.Turn != second || r.Game.Phase != "catan_turn" || !r.Game.Catan.Paired.Second || left < 120000 || left > 121000 || r.Game.Catan.RollID != rollID || string(fishBefore) != string(fishAfter) {
						t.Fatal("second phase produced fish or lost clock")
					}
					restart()
					before, _ := json.Marshal(s.rooms[id])
					clients[second].command(current(clients[second]), "action", game.Action{Type: "catan_roll"}, 400)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("secondary roll changed state")
					}
					// Spend two expansion-only tokens (one and three fish) in
					// the secondary action, without refreshing its deadline.
					r = s.rooms[id]
					deadline, ore := r.TurnDeadline, r.Game.Catan.Players[second].Resources[4]
					pay := game.Action{Type: "catan_fish_resource", Tokens: []int{30, 39}, Color: 4}
					clients[n].command(current(clients[n]), "action", pay, 400)
					clients[n-1].command(current(clients[n-1]), "action", pay, 400)
					clients[second].command(current(clients[second]), "action", pay, 200)
					r = s.rooms[id]
					if r.TurnDeadline != deadline || r.Game.Catan.Players[second].Resources[4] != ore+1 || len(r.Game.Catan.Fishing.Tokens.Hands[second]) != 0 || !r.Game.Catan.Paired.Second {
						t.Fatal("secondary fish payment/effect/clock")
					}
					restart()
					// Returning to the next primary gives a fresh roll phase and turn clock.
					at = time.Now()
					clients[second].command(current(clients[second]), "action", game.Action{Type: "catan_end"}, 200)
					r = s.rooms[id]
					left = r.TurnDeadline - at.UnixMilli()
					if r.Game.Turn != 0 || r.Game.Phase != "catan_roll" || r.Game.Catan.Paired.Second || left < 120000 || left > 121000 {
						t.Fatal("new primary handoff")
					}
					for color, total := range r.Game.Catan.Bank {
						for _, p := range r.Game.Catan.Players {
							total += p.Resources[color]
						}
						if total != 24 {
							t.Fatal("extended resource supply")
						}
					}
					if !reflect.DeepEqual(r.Game.Catan.NewWorldMap(), state.Catan.NewWorldMap()) {
						t.Fatal("map changed")
					}
					restart()
					t.Logf("responses=%d restarts=%d", step, restarts)
				})
			}
		}
	}
}
