package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCatanFishingCitiesKnightsProgressHTTPAndRestart(t *testing.T) {
	for _, v := range []struct {
		n     int
		phase string
	}{{3, "catan_roll"}, {3, "catan_turn"}, {6, "catan_turn"}} {
		t.Run(fmt.Sprintf("%d/%s", v.n, v.phase), func(t *testing.T) {
			s, ts, clients, id, p, a := newFishingActionTable(t, v.n, v.phase, "progress")
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
					t.Fatal("combined progress save changed on restart")
				}
			}
			restart()
			before, _ := json.Marshal(s.rooms[id])
			for _, seat := range []int{(p + 1) % v.n, v.n} {
				clients[seat].command(current(clients[seat]), "action", a, 400)
			}
			invalid := a
			invalid.Color = 3
			clients[p].command(current(clients[p]), "action", invalid, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid progress buy changed table")
			}
			clients[p].command(current(clients[p]), "action", a, 200)
			r := s.rooms[id]
			g := r.Game.Catan
			if r.TurnDeadline != deadline || r.Game.Phase != v.phase || len(g.Fishing.Tokens.Hands[p]) != 4 || len(g.CitiesKnights.Players[p].Progress) != 1 || g.CitiesKnights.Players[p].Progress[0] != 16 {
				t.Fatal("progress effect or timer")
			}
			restart()
			for viewer, c := range clients {
				view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
				k := view["citiesKnights"].(map[string]any)
				if k["progressDecks"] != nil {
					t.Fatal("deck leaked")
				}
				for seat, raw := range k["players"].([]any) {
					_, visible := raw.(map[string]any)["progress"]
					if visible != (viewer == seat) {
						t.Fatal("progress leaked")
					}
				}
				fish := view["fishing"].(map[string]any)["tokens"].(map[string]any)
				for seat, raw := range fish["players"].([]any) {
					_, visible := raw.(map[string]any)["tokens"]
					if visible != (viewer == seat && seat == p) {
						t.Fatal("fish leaked")
					}
				}
			}
		})
	}
}
func TestCatanFishingCitiesKnightsFishToAqueductHTTP(t *testing.T) {
	for _, mode := range []string{"manual", "autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := newCatanTable(t)
			clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			s.mu.Lock()
			fishingResponseVariant(t, s.rooms[id], time.Now(), true)
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
					t.Fatal("combined response lost on restart")
				}
			}
			restart()
			for step, actor := range []int{1, 2, 1} {
				r := s.rooms[id]
				if r.Game.CatanPendingActor() != actor {
					t.Fatal("wrong response order", step, r.Game.Phase)
				}
				a := game.Action{Type: "catan_fish_keep"}
				if step == 2 {
					a = game.Action{Type: "catan_aqueduct", Color: 0}
				}
				before, _ := json.Marshal(r)
				clients[0].command(current(clients[0]), "action", a, 400)
				clients[3].command(current(clients[3]), "action", a, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("wrong responder changed state")
				}
				at := time.Now()
				if mode == "manual" {
					clients[actor].command(current(clients[actor]), "action", a, 200)
				} else if mode == "autoplay" {
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(at)
					s.mu.Unlock()
				} else {
					at = time.UnixMilli(r.TurnDeadline)
					s.mu.Lock()
					s.expireSetups(at)
					s.mu.Unlock()
				}
				r = s.rooms[id]
				want := int64(120000)
				if step == 2 {
					want = 45000
				}
				remaining := r.TurnDeadline - at.UnixMilli()
				if remaining < want || remaining > want+1000 || r.CatanTimeLeft != 45000 || r.Game.Turn != 0 {
					t.Fatal("combined response clock", step, remaining, want)
				}
				if step == 1 && (r.Game.Phase != "catan_aqueduct" || r.Game.Catan.Fishing.Pending != nil) {
					t.Fatal("aqueduct not after fish")
				}
				restart()
			}
			r := s.rooms[id]
			g := r.Game.Catan
			total := 0
			for _, n := range g.Players[1].Resources {
				total += n
			}
			if r.Game.Phase != "catan_turn" || r.Game.CatanPendingActor() != -1 || total != 1 || g.RollID != 1 || g.Fishing.LastRollID != 1 {
				t.Fatal("duplicate production or missing aqueduct")
			}
		})
	}
}
