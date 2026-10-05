package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Public scenario selection remains gated. Only the pristine constructor is
// installed after the formal lobby flow; no dice, resources or pieces are granted.
func newCaravansFullTable(t *testing.T, n int) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("商队玩家%d", p))
	}
	opts := game.CatanOptions{FiveSix: n > 4}
	raw := clients[0].post("/api/rooms", map[string]any{"name": "商队完整对局", "kind": "catan", "capacity": n, "catanOptions": opts}, 201)
	id := raw["id"].(string)
	for p := 1; p < n; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for p := range n {
		clients[p].command(current(clients[0]), "ready", nil, 200)
	}
	clients[0].command(current(clients[0]), "start", nil, 200)
	clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	initial, err := game.NewCatanCaravans(n, opts)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[id]
	r.Game = initial
	r.startTurnClock(time.Now())
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	return s, ts, clients, id
}

func assertCaravansFullInventory(t *testing.T, state *game.State) {
	t.Helper()
	g, c := state.Catan, state.Catan.Caravans
	resources, development, wagons := 19, 25, 22
	if len(g.Players) > 4 {
		resources, development, wagons = 24, 34, 33
	}
	if c.Map.Supply != wagons || len(c.Wagons) > wagons {
		t.Fatal("wagon supply")
	}
	occupied, outgoing, heads := map[int]bool{}, map[int]bool{}, map[int]bool{}
	incidents := make([]int, len(g.Vertices))
	for _, w := range c.Wagons {
		if w.Edge < 0 || w.Edge >= len(g.Edges) {
			t.Fatal("invalid wagon edge")
		}
		e := g.Edges[w.Edge]
		start := false
		for _, origin := range c.Map.Starts {
			start = start || origin == w
		}
		if (w.From != e.A && w.From != e.B) || occupied[w.Edge] || outgoing[w.From] || (!start && !heads[w.From]) {
			t.Fatal("broken directed wagon network", w)
		}
		occupied[w.Edge], outgoing[w.From] = true, true
		heads[e.A+e.B-w.From] = true
		incidents[e.A]++
		incidents[e.B]++
	}
	for color, total := range g.Bank {
		if total < 0 {
			t.Fatal("negative bank")
		}
		for p, seat := range g.Players {
			if seat.Resources[color] < 0 {
				t.Fatal("negative hand")
			}
			total += seat.Resources[color]
			if q := c.Pending; q != nil && q.Bids[p] != nil {
				if q.Bids[p][color] < 0 || (color != 2 && color != 3 && q.Bids[p][color] != 0) {
					t.Fatal("invalid public escrow")
				}
				total += q.Bids[p][color]
			}
		}
		if total != resources {
			t.Fatal("resource conservation including escrow", color, total)
		}
	}
	dev := len(g.DevDeck) + len(g.DevDiscard)
	for p, seat := range g.Players {
		if seat.Eliminated {
			t.Fatal("natural game eliminated a player")
		}
		for _, count := range seat.Dev {
			dev += count
		}
		roads, villages, cities, score := 0, 0, 0, seat.Dev[4]
		for _, e := range g.Edges {
			if e.Owner == p {
				roads++
			}
		}
		for _, v := range g.Vertices {
			if v.Owner != p {
				continue
			}
			if v.Level == 1 {
				villages++
			} else if v.Level == 2 {
				cities++
			} else {
				t.Fatal("invalid building")
			}
			score += v.Level
			if incidents[v.ID] >= 2 {
				score++
			}
		}
		if roads > 15 || villages > 5 || cities > 4 {
			t.Fatal("pieces exhausted")
		}
		if g.LongestOwner == p {
			score += 2
		}
		if g.ArmyOwner == p {
			score += 2
		}
		if score != seat.Score {
			t.Fatal("independent building/caravan/victory score", p, score, seat.Score)
		}
	}
	if dev != development {
		t.Fatal("development conservation", dev)
	}
}

func assertCaravansFullPrivacy(t *testing.T, clients []*testClient, state *game.State) {
	t.Helper()
	n := len(state.Catan.Players)
	actor := state.CatanPendingActor()
	viewer := state.Turn
	if actor >= 0 {
		viewer = actor
	}
	for _, p := range []int{viewer, (viewer + 1) % n, n} {
		v := current(clients[p])["game"].(map[string]any)["catan"].(map[string]any)
		if v["devDeck"] != nil || v["victoryTarget"] != float64(12) {
			t.Fatal("hidden deck or wrong victory target")
		}
		for other, raw := range v["players"].([]any) {
			seat := raw.(map[string]any)
			_, hand := seat["resources"]
			_, dev := seat["dev"]
			if hand != (p == other) || dev != (p == other) {
				t.Fatal("private cards exposed", p, other)
			}
		}
		c := v["caravans"].(map[string]any)
		if c["actor"] != float64(actor) || c["canAct"] != (p == actor) || c["remaining"] != float64(state.Catan.Caravans.Map.Supply-len(state.Catan.Caravans.Wagons)) {
			t.Fatal("public responder or supply")
		}
		if q := state.Catan.Caravans.Pending; q != nil {
			got, _ := json.Marshal(c["pending"])
			want, _ := json.Marshal(q)
			// Compare decoded JSON: the map and struct serialize key order differently.
			var expected map[string]any
			if err := json.Unmarshal(want, &expected); err != nil {
				t.Fatal(err)
			}
			want, _ = json.Marshal(expected)
			if string(got) != string(want) {
				t.Fatal("public bids/votes changed")
			}
		}
	}
}

func TestCatanCaravansCompleteHTTPGames(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newCaravansFullTable(t, n)
			restored, phases, modes := map[string]bool{}, map[string]int{}, map[string]int{}
			restart := func(label string) {
				t.Helper()
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				restored[label] = true
			}
			restart("start")
			steps, automatic, timeouts := 0, 0, 0
			for ; steps < 6000 && !s.rooms[id].Game.Finished; steps++ {
				r := s.rooms[id]
				state, g := r.Game, r.Game.Catan
				assertCaravansFullInventory(t, state)
				label := ""
				switch {
				case g.SetupStep == 1:
					label = "first-setup-pair"
				case g.Caravans.Pending != nil:
					label = state.Phase
					if g.Caravans.Pending.Cursor > 0 {
						label += "-partial"
					}
				case len(g.Caravans.Wagons) == 1:
					label = "first-wagon"
				case g.SetupStep >= g.SetupLimit() && state.Phase == "catan_roll":
					label = "normal-turn"
				case g.Paired != nil && g.Paired.Second:
					label = "secondary"
				}
				if label != "" && !restored[label] {
					restart(label)
					r = s.rooms[id]
					state, g = r.Game, r.Game.Catan
				}
				if steps%31 == 0 || g.Caravans.Pending != nil {
					assertCaravansFullPrivacy(t, clients, state)
				}
				actor := state.Turn
				if pending := state.CatanPendingActor(); pending >= 0 {
					actor = pending
				} else if state.Phase == "catan_discard" {
					for p, due := range g.DiscardDue {
						if due > 0 {
							actor = p
							break
						}
					}
				}
				pending := g.Caravans.Pending != nil
				if pending {
					if phases[state.Phase] == 0 {
						before, _ := json.Marshal(r)
						for _, wrong := range []int{(actor + 1) % n, n} {
							clients[wrong].command(current(clients[wrong]), "action", game.Action{Type: state.Phase, Tokens: []int{0, 0, 0, 0, 0}}, 400)
						}
						clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_end"}, 400)
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("invalid vote changed room/clock")
						}
					}
					phases[state.Phase]++
				}
				version := r.Version
				if pending && modes["timeout"] == 0 || steps%29 == 0 && (g.SetupStep < g.SetupLimit() || state.CatanPendingActor() >= 0 || state.Phase == "catan_discard") {
					s.mu.Lock()
					r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
					s.expireSetups(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Version <= version {
						t.Fatal("timeout stalled", steps, state.Phase)
					}
					timeouts++
					if pending {
						modes["timeout"]++
					}
					continue
				}
				if pending && modes["autoplay"] == 0 || steps%17 == 0 {
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					version = s.rooms[id].Version
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Version <= version {
						t.Fatal("autoplay stalled", steps, state.Phase)
					}
					if !s.rooms[id].Game.Finished {
						setAutoPlay(clients[actor], current(clients[actor]), false, 200)
					}
					automatic++
					if pending {
						modes["autoplay"]++
					}
					continue
				}
				action, err := state.BotAction(actor)
				if err != nil {
					t.Fatal(steps, state.Phase, err)
				}
				// Legal human bidding policy: offer one wool/grain if held. This
				// exercises negotiation without granting cards or forcing dice.
				if action.Type == "catan_caravan_bid" {
					action.Tokens = make([]int, 5)
					for _, color := range []int{2, 3} {
						if g.Players[actor].Resources[color] > 0 {
							action.Tokens[color] = 1
							break
						}
					}
				}
				clients[actor].command(current(clients[actor]), "action", action, 200)
				if pending {
					modes["manual"]++
				}
			}
			r := s.rooms[id]
			assertCaravansFullInventory(t, r.Game)
			if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || !restored["first-setup-pair"] || !restored["normal-turn"] || n > 4 && !restored["secondary"] || automatic == 0 || timeouts == 0 || len(r.Game.Catan.Caravans.Wagons) == 0 || len(modes) != 3 {
				t.Fatal("incomplete full game", steps, automatic, timeouts, restored, modes)
			}
			winner := r.Game.Winners[0]
			if winner != r.Game.Turn || r.Game.Catan.Players[winner].Score < 12 {
				t.Fatal("victory outside own action or below target")
			}
			restart("finished")
			checkHistory := func() {
				t.Helper()
				for p := range n {
					code, profile := clients[n].request("GET", "/api/players/"+s.rooms[id].Seats[p].ID, nil)
					if code != 200 {
						t.Fatal("history unavailable")
					}
					wins := profile["stats"].(map[string]any)["catan"].(map[string]any)["wins"]
					want := float64(0)
					if p == winner {
						want = 1
					}
					if wins != want {
						t.Fatal("wrong history winner", p, wins)
					}
				}
			}
			checkHistory()
			before, _ := json.Marshal(s.rooms[id])
			clients[winner].command(map[string]any{"id": id, "version": s.rooms[id].Version}, "action", game.Action{Type: "catan_end"}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("finished action mutated room")
			}
			checkHistory()
			t.Logf("steps=%d autoplay=%d timeouts=%d wagons=%d phases=%v modes=%v winner=%d score=%d restarts=%v", steps, automatic, timeouts, len(r.Game.Catan.Caravans.Wagons), phases, modes, winner, r.Game.Catan.Players[winner].Score, restored)
		})
	}
}
