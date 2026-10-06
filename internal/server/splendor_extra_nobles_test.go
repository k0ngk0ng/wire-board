package server

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestSplendorExtraNoblesHTTPNaturalMatches(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		for n := 2; n <= 4; n++ {
			t.Run(fmt.Sprintf("mask%d/players%d", mask, n), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				clients := make([]*testClient, n+1)
				for i := range clients {
					clients[i] = newClient(t, ts.URL)
					clients[i].register(fmt.Sprintf("贵族玩家%d", i))
				}
				o := game.SplendorOptions{ExtraNobles: true, TradingPosts: mask&1 != 0, Strongholds: mask&2 != 0}
				r := clients[0].post("/api/rooms", map[string]any{"name": "附赠贵族验收", "kind": "splendor", "capacity": n, "splendorOptions": o}, 201)
				for _, c := range clients[1:n] {
					c.command(current(clients[0]), "join", nil, 200)
				}
				for _, c := range clients[:n] {
					c.command(current(c), "ready", nil, 200)
				}
				clients[0].command(current(clients[0]), "start", nil, 200)
				id := r["id"].(string)
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				state := s.rooms[id].Game
				if !state.Splendor.Options.ExtraNobles || state.Splendor.Options.Rules != game.SplendorExpansionRules || len(state.Splendor.Nobles) != n+1 {
					t.Fatal("public start lost bonus noble setup")
				}
				initial := slices.Clone(state.Splendor.Bank)
				nobles := map[int]game.Noble{}
				for _, noble := range state.Splendor.Nobles {
					nobles[noble.ID] = noble
				}
				steps := 0
				for ; !s.rooms[id].Game.Finished && steps < 2500; steps++ {
					room := s.rooms[id]
					state = room.Game
					actor := state.Turn
					if steps%31 == 0 {
						assertModernPrivacy(t, clients, state)
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						room, state = s.rooms[id], s.rooms[id].Game
					}
					if steps%2 == 0 {
						a, err := state.BotAction(actor)
						if err != nil {
							t.Fatal(err)
						}
						clients[actor].command(current(clients[actor]), "action", a, 200)
					} else {
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
						s.mu.Lock()
						s.rooms[id].BotAt = 0
						s.runBots(time.Now())
						s.mu.Unlock()
						if !s.rooms[id].Game.Finished {
							setAutoPlay(clients[actor], current(clients[actor]), false, 200)
						}
					}
					state = s.rooms[id].Game
					assertModernComponents(t, state, initial)
					found := map[int]bool{}
					check := func(list []game.Noble) {
						for _, x := range list {
							if found[x.ID] || !reflect.DeepEqual(x, nobles[x.ID]) {
								t.Fatal("noble changed or duplicated", x)
							}
							found[x.ID] = true
						}
					}
					check(state.Splendor.Nobles)
					for _, p := range state.Splendor.Players {
						check(p.Nobles)
					}
					if len(found) != n+1 {
						t.Fatal("lost noble")
					}
				}
				room := s.rooms[id]
				if !room.Game.Finished || len(room.Game.Winners) == 0 {
					t.Fatal("natural game stalled", steps)
				}
				var raw string
				if err := s.db.QueryRow("SELECT snapshot FROM match_history WHERE id=?", room.MatchID).Scan(&raw); err != nil {
					t.Fatal(err)
				}
				var record MatchRecord
				if err := json.Unmarshal([]byte(raw), &record); err != nil {
					t.Fatal(err)
				}
				if !record.SplendorOptions.ExtraNobles || record.SplendorOptions != room.Game.Splendor.Options || record.Status != "finished" {
					t.Fatal("history lost bonus noble identity")
				}
				for i, p := range record.Players {
					if p.Score == nil || *p.Score != room.Game.Splendor.Players[i].Score {
						t.Fatal("history lost noble score")
					}
				}
				t.Logf("%d legal actions; initial nobles %v; winners %v", steps, nobles, room.Game.Winners)
			})
		}
	}
}
