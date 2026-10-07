package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Controlled pending effects on the official starting map. Full event draw
// remains private/unconnected; these test the production server response path.
func attackHTTPInheritedEvent(t *testing.T, n int, kind string) *game.State {
	t.Helper()
	s := attackHTTPEventFixture(t, n)
	g := s.Catan
	g.Attack.Knights = nil
	for p := range g.Players {
		for c, count := range g.Players[p].Resources {
			g.Bank[c] += count
			g.Players[p].Resources[c] = 0
		}
	}
	q := &game.CatanCardEvent{Kind: kind, Production: 6, Players: []int{}}
	for p := range n {
		q.Players = append(q.Players, p)
	}
	switch kind {
	case "good_neighbors":
		for p := range n {
			g.Players[p].Resources[p%5] = 1
			g.Bank[p%5]--
			q.Gifts = append(q.Gifts, game.CatanEventGift{From: p, To: (p + 1) % n, Color: -1})
		}
	case "helpful_neighbor":
		g.Attack.Prisoners[0] = 4
		g.Players[0].Score += 2
		g.Players[0].Resources[1] = 2
		g.Bank[1] -= 2
		q.Players = []int{0}
		for p := 1; p < n; p++ {
			q.Targets = append(q.Targets, p)
		}
	case "calm_seas":
		counts := make([]int, n)
		for _, v := range g.Vertices {
			if v.Owner < 0 || v.Level == 0 {
				continue
			}
			for _, port := range g.Ports {
				e := g.Edges[port.Edge]
				if e.A == v.ID || e.B == v.ID {
					counts[v.Owner]++
					break
				}
			}
		}
		best := slices.Max(counts)
		q.Players = nil
		for p, count := range counts {
			if count == best {
				q.Players = append(q.Players, p)
			}
		}
	case "trade_advantage":
		// Keep the official graph and construct five sides of one hex as roads.
		for i := range g.Edges {
			g.Edges[i].Owner = -1
		}
		for side := 0; side < 5; side++ {
			a, b := g.Tiles[0].Vertices[side], g.Tiles[0].Vertices[(side+1)%6]
			for i, e := range g.Edges {
				if e.A == a && e.B == b || e.A == b && e.B == a {
					g.Edges[i].Owner = 1
				}
			}
		}
		// Remove opposing intersections along the test path to preserve connectivity.
		for _, vertex := range g.Tiles[0].Vertices {
			v := &g.Vertices[vertex]
			if v.Level > 0 && v.Owner != 1 {
				g.Players[v.Owner].Score -= v.Level
				v.Owner, v.Level = -1, 0
			}
		}
		g.LongestOwner = 1
		g.Players[1].RoadLength = 5
		g.Players[1].Score += 2
		g.Players[2].Resources[1] = 2
		g.Bank[1] -= 2
		q.Players = []int{1}
	}
	g.CardEvent = q
	g.RevealedEvent.Kind = kind
	return s
}

func TestCatanAttackEventInheritedHTTPRecovery(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, kind := range []string{"earthquake", "plentiful_year", "calm_seas", "tournament", "good_neighbors", "helpful_neighbor", "trade_advantage"} {
			for _, mode := range []string{"manual", "autoplay", "timeout"} {
				t.Run(fmt.Sprintf("%d/%s/%s", n, kind, mode), func(t *testing.T) {
					s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
					state := attackHTTPInheritedEvent(t, n, kind)
					g := state.Catan
					expected := make([][]int, n)
					production := make([][]int, n)
					for p := range n {
						expected[p] = slices.Clone(g.Players[p].Resources)
						production[p] = make([]int, 5)
					}
					for _, tile := range g.Tiles {
						if tile.Number == 6 && tile.Resource >= 0 && tile.Resource < 5 {
							for _, id := range tile.Vertices {
								v := g.Vertices[id]
								if v.Owner >= 0 && v.Level > 0 {
									production[v.Owner][tile.Resource] += v.Level
								}
							}
						}
					}
					s.mu.Lock()
					r := s.rooms[id]
					r.Game = state
					now := time.Now()
					r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
					if !r.adjustCatanResponseClock("catan_roll", -1, g.SetupStep, now) || r.CatanTimeLeft != 45000 {
						t.Fatal("response clock did not pause")
					}
					if err := s.save(r); err != nil {
						t.Fatal(err)
					}
					s.mu.Unlock()
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					count := len(state.Catan.CardEvent.Players)
					damaged := []int{}
					for step := 0; step < count; step++ {
						r = s.rooms[id]
						g = r.Game.Catan
						actor := r.Game.CatanPendingActor()
						if actor < 0 {
							t.Fatal("queue ended too early")
						}
						for viewer, c := range clients {
							pub := current(c)["game"].(map[string]any)["catan"].(map[string]any)
							q := pub["cardEvent"].(map[string]any)
							if q["gifts"] != nil || q["canSkip"] == true {
								t.Fatal("private gifts exposed or optional action")
							}
							if own, ok := q["ownGift"].(map[string]any); ok && own["from"] != float64(viewer) {
								t.Fatal("another player's choice exposed")
							}
							for seat, raw := range pub["players"].([]any) {
								if (raw.(map[string]any)["resources"] != nil) != (seat == viewer) {
									t.Fatal("hidden hand exposed")
								}
							}
						}
						action := cardEventHTTPAction(t, r.Game)
						if mode != "manual" {
							var err error
							action, err = r.Game.BotAction(actor)
							if err != nil {
								t.Fatal(err)
							}
						}
						switch action.Type {
						case "catan_event_resource":
							for c, v := range action.Take {
								expected[actor][c] += v
							}
						case "catan_event_gift":
							target := action.Target
							if kind == "good_neighbors" {
								target = (actor + 1) % n
							}
							for c, v := range action.Give {
								expected[actor][c] -= v
								expected[target][c] += v
							}
						case "catan_event_steal":
							expected[actor][1]++
							expected[action.Target][1]--
						case "catan_earthquake":
							damaged = append(damaged, action.Edge)
						default:
							t.Fatal("unexpected response", action)
						}
						before, _ := json.Marshal(r)
						clients[n].command(current(clients[n]), "action", action, 400)
						clients[(actor+1)%n].command(current(clients[(actor+1)%n]), "action", action, 400)
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("invalid action mutated room")
						}
						resolvedAt := time.Now()
						var replay map[string]any
						switch mode {
						case "manual":
							replay = map[string]any{"type": "action", "version": r.Version, "nonce": fmt.Sprintf("inherit-event-%d", step), "action": action}
							clients[actor].post("/api/rooms/"+id, replay, 200)
						case "autoplay":
							setAutoPlay(clients[actor], current(clients[actor]), true, 200)
							s.mu.Lock()
							s.rooms[id].BotAt = 0
							s.runBots(resolvedAt)
							s.mu.Unlock()
							setAutoPlay(clients[actor], current(clients[actor]), false, 200)
						case "timeout":
							resolvedAt = time.UnixMilli(r.TurnDeadline)
							s.mu.Lock()
							s.expireSetups(resolvedAt)
							s.mu.Unlock()
						}
						r = s.rooms[id]
						want := int64(45000)
						if step < count-1 {
							want = 120000
						}
						if left := r.TurnDeadline - resolvedAt.UnixMilli(); left < want || left > want+1000 {
							t.Fatal("response/remaining clock", left, want)
						}
						before, _ = json.Marshal(r)
						if replay != nil {
							clients[actor].post("/api/rooms/"+id, replay, 200)
						}
						// Restore the partially selected private gifts and damaged roads, then
						// restore the completed state separately to rule out double production.
						if step == 0 || step == count-1 {
							s, ts = restartRiversHTTP(t, s, ts, clients, id)
							if replay != nil {
								clients[actor].post("/api/rooms/"+id, replay, 200)
							}
						}
						after, _ = json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("restart/replay repeated effect")
						}
					}
					r = s.rooms[id]
					g = r.Game.Catan
					if r.Game.Phase != "catan_turn" || r.Game.Turn != 0 || g.CardEvent != nil || !g.RevealedEvent.ProductionStarted || n == 6 && g.Paired.Second {
						t.Fatal("wrong final continuation")
					}
					for p := range n {
						for c, v := range production[p] {
							expected[p][c] += v
						}
						if !slices.Equal(g.Players[p].Resources, expected[p]) {
							t.Fatal("wrong final hand", p, g.Players[p].Resources, expected[p])
						}
					}
					for _, edge := range damaged {
						if !g.Edges[edge].Damaged {
							t.Fatal("lost earthquake damage")
						}
					}
					if g.Attack.Sequence != 0 || g.Attack.CardSequence != 0 {
						t.Fatal("event generated unrelated landing or development card")
					}
				})
			}
		}
	}
}
