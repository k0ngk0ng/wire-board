package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Create the public two-player recipe and use real ready/start commands.
// No internal draft, running state, cards, resources, dice or winners are installed.
func newTwoFullTable(t *testing.T) (*Server, *httptest.Server, []*testClient, string) {
	return newTwoScenarioFullTable(t, "")
}
func newTwoScenarioFullTable(t *testing.T, scenario string, events ...bool) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, 3)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("双人验收%d", p))
	}
	clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "未知双人剧本", "capacity": 2, "catanTwoScenario": "unknown"}, 400)
	clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "禁止版本注入", "capacity": 2, "catanTwoRules": game.CatanTwoRules}, 400)
	recipe := map[string]any{"kind": "catan", "name": "双人完整对局", "capacity": 2, "catanTwoScenario": scenario}
	if len(events) > 0 && events[0] {
		recipe["catanEvents"] = game.CatanEventCatalogue
	}
	raw := clients[0].post("/api/rooms", recipe, 201)
	id := raw["id"].(string)
	clients[1].command(current(clients[0]), "join", nil, 200)
	for p := range 2 {
		clients[p].command(current(clients[p]), "ready", nil, 200)
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	clients[0].command(current(clients[0]), "start", nil, 200)
	clients[2].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	r := s.rooms[id]
	phase := "catan_setup_settlement"
	if scenario == "rivers" {
		phase = "catan_rivers_start"
		if r.Game.Catan.Rivers == nil || r.Game.Catan.Rivers.Rules != game.CatanRiversRules {
			t.Fatal("formal start omitted river rules")
		}
	}
	if scenario == "caravans" && (r.Game.Catan.Caravans == nil || r.Game.Catan.Caravans.Rules != game.CatanCaravansRules) {
		t.Fatal("formal start omitted merchant train rules")
	}
	wantBank := 10
	if scenario == "fishing" {
		wantBank = 0
	}
	if len(r.Seats) != 2 || len(r.Game.Catan.Players) != 2 || r.Game.Phase != phase || r.Game.Catan.Two == nil || r.Game.Catan.Two.Bank != wantBank {
		t.Fatal("formal start did not construct two-player setup")
	}
	return s, ts, clients, id
}

func twoHTTPActor(s *game.State) int {
	if actor := s.CatanPendingActor(); actor >= 0 {
		return actor
	}
	if s.Phase == "catan_discard" {
		for p, due := range s.Catan.DiscardDue {
			if due > 0 {
				return p
			}
		}
	}
	return s.Turn
}

func assertTwoHTTPInventory(t *testing.T, s *game.State) {
	t.Helper()
	g, q := s.Catan, s.Catan.Two
	wantTokens := 20 + q.TokensIssued
	if g.Fishing != nil {
		wantTokens = 0
		if g.Fishing.Two != game.CatanTwoFishingRules || q.TokensIssued != 0 || q.Spent || q.KnightExchanged || q.Trade != nil {
			t.Fatal("fishing token rules")
		}
		seen := map[int]bool{}
		all := append(append([]int{}, g.Fishing.Tokens.DrawPile...), g.Fishing.Tokens.Discard...)
		for _, h := range g.Fishing.Tokens.Hands {
			if len(h) > 7 {
				t.Fatal("fish hand limit")
			}
			all = append(all, h...)
		}
		if g.Fishing.Tokens.BootOwner >= 0 {
			all = append(all, 29)
		}
		for _, id := range all {
			if id < 0 || id >= 30 || seen[id] {
				t.Fatal("fish inventory", id)
			}
			seen[id] = true
		}
		if len(seen) != 30 {
			t.Fatal("fish total")
		}
	}
	if len(g.Players) != 2 || q.Rules != game.CatanTwoRules || q.Bank < 0 || len(q.Tokens) != 2 || q.Tokens[0] < 0 || q.Tokens[1] < 0 || q.Bank+q.Tokens[0]+q.Tokens[1] != wantTokens {
		t.Fatal("two-player token/player inventory")
	}
	for color, total := range g.Bank {
		if total < 0 {
			t.Fatal("negative bank")
		}
		for _, seat := range g.Players {
			if seat.Resources[color] < 0 {
				t.Fatal("negative hand")
			}
			total += seat.Resources[color]
		}
		if g.Caravans != nil && g.Caravans.Pending != nil {
			for _, bid := range g.Caravans.Pending.Bids {
				if bid != nil {
					total += bid[color]
				}
			}
		}
		want := 19
		if color >= 5 && g.CitiesKnights != nil {
			want = 12
		}
		if total != want {
			t.Fatal("resource conservation", color, total)
		}
	}
	if g.Rivers != nil {
		if g.Rivers.Rules != game.CatanRiversRules || len(g.Rivers.Gold) != 2 || g.Rivers.Bank < 0 {
			t.Fatal("river rule/player inventory")
		}
		total := g.Rivers.Bank
		for _, amount := range g.Rivers.Gold {
			if amount < 0 {
				t.Fatal("negative gold")
			}
			total += amount
		}
		if total != 100+g.Rivers.GoldIssued {
			t.Fatal("gold conservation")
		}
	}
	if g.Caravans != nil {
		if g.Caravans.Rules != game.CatanCaravansRules {
			t.Fatal("missing merchant train rules")
		}
		assertCaravansFullInventory(t, s)
	}
	dev := len(g.DevDeck) + len(g.DevDiscard)
	for p, seat := range g.Players {
		if seat.Eliminated {
			t.Fatal("natural game eliminated player")
		}
		score := seat.Dev[4]
		for _, n := range seat.Dev {
			dev += n
		}
		for _, v := range g.Vertices {
			if v.Owner == p {
				score += v.Level
				if g.Caravans != nil {
					adjacent := 0
					for _, w := range g.Caravans.Wagons {
						e := g.Edges[w.Edge]
						if e.A == v.ID || e.B == v.ID {
							adjacent++
						}
					}
					if adjacent >= 2 {
						score++
					}
				}
			}
		}
		if g.LongestOwner == p {
			score += 2
		}
		if g.ArmyOwner == p {
			score += 2
		}
		if g.Rivers != nil {
			if g.Rivers.Gold[p] > g.Rivers.Gold[1-p] {
				score++
			}
			if g.Rivers.Gold[p] <= g.Rivers.Gold[1-p] {
				score -= 2
			}
		}
		if k := g.CitiesKnights; k != nil {
			score += k.Players[p].DefenderPoints + k.Players[p].ProgressPoints
			for _, vertex := range k.Metropolises {
				if vertex >= 0 && g.Vertices[vertex].Owner == p {
					score += 2
				}
			}
			if k.Merchant != nil && k.Merchant.Owner == p {
				score++
			}
		}
		if score != seat.Score {
			t.Fatal("incorrect real score", p, score, seat.Score)
		}
	}
	wantDev := 25
	if g.CitiesKnights != nil {
		wantDev = 0
	}
	if dev != wantDev {
		t.Fatal("development inventory", dev)
	}
	for _, owner := range []int{0, 1, -2, -3} {
		roads, villages, cities, bridges := 0, 0, 0, 0
		for _, e := range g.Edges {
			if e.Owner == owner {
				if e.Bridge {
					bridges++
				} else {
					roads++
				}
			}
			if g.Vertices[e.A].Level > 0 && g.Vertices[e.B].Level > 0 {
				t.Fatal("adjacent settlements")
			}
		}
		for _, v := range g.Vertices {
			if v.Owner == owner {
				if v.Level == 1 {
					villages++
				} else if v.Level == 2 {
					cities++
				}
			}
		}
		if k := g.CitiesKnights; k != nil {
			for _, vertex := range k.FallenCities {
				if g.Vertices[vertex].Owner == owner {
					villages--
					cities++
				}
			}
		}
		if bridges > 3 || roads > 15 || villages > 5 || cities > 4 || owner < 0 && (villages < 1 || cities != 0) {
			t.Fatal("piece inventory", owner, roads, villages, cities)
		}
	}
}

func assertTwoHTTPPrivacy(t *testing.T, clients []*testClient, s *game.State) {
	t.Helper()
	if s.Catan.Caravans != nil {
		assertCaravansFullPrivacy(t, clients, s)
	}
	for viewer, client := range clients {
		v := current(client)["game"].(map[string]any)["catan"].(map[string]any)
		if v["devDeck"] != nil || v["devDiscard"] != nil {
			t.Fatal("private deck exposed")
		}
		for p, raw := range v["players"].([]any) {
			seat := raw.(map[string]any)
			if (seat["resources"] != nil) != (viewer == p) || (seat["dev"] != nil) != (viewer == p) {
				t.Fatal("private hand exposed", viewer, p)
			}
		}
		if s.Catan.Fishing != nil {
			f := v["fishing"].(map[string]any)
			tokens := f["tokens"].(map[string]any)
			if tokens["drawPile"] != nil || tokens["hands"] != nil {
				t.Fatal("fish deck leaked")
			}
			for p, raw := range tokens["players"].([]any) {
				h := raw.(map[string]any)
				if p != viewer && h["tokens"] != nil {
					t.Fatal("opponent fish exposed")
				}
			}
		}
		if k := s.Catan.CitiesKnights; k != nil {
			public := v["citiesKnights"].(map[string]any)
			if public["progressDecks"] != nil || public["event"] != nil {
				t.Fatal("city deck/queue exposed")
			}
			for p, raw := range public["players"].([]any) {
				seat := raw.(map[string]any)
				if (seat["progress"] != nil) != (viewer == p) {
					t.Fatal("private progress hand exposed", viewer, p)
				}
			}
			if q := k.Pending; q != nil && (q.Kind == "espionage" || q.Kind == "guild_dues") {
				pending := public["pending"].(map[string]any)
				if viewer != q.Players[0] && (pending["progress"] != nil || pending["resources"] != nil) {
					t.Fatal("private response exposed")
				}
			}
		}
		q := v["two"].(map[string]any)
		if s.Catan.Two.Trade != nil {
			if (q["trade"].(map[string]any)["drawn"] != nil) != (viewer == s.Turn) {
				t.Fatal("forced draw privacy")
			}
		}
		if q["canAct"] != (!s.Finished && viewer == s.Turn && (s.Catan.Two.Pending != nil || s.Catan.Two.Trade != nil)) {
			t.Fatal("incorrect response permission")
		}
	}
}

func TestCatanTwoCompleteHTTPGames(t *testing.T)         { runTwoCompleteHTTPGames(t, "") }
func TestCatanTwoRiversCompleteHTTPGames(t *testing.T)   { runTwoCompleteHTTPGames(t, "rivers") }
func TestCatanTwoCaravansCompleteHTTPGames(t *testing.T) { runTwoCompleteHTTPGames(t, "caravans") }
func TestCatanTwoFishingCompleteHTTPGames(t *testing.T)  { runTwoCompleteHTTPGames(t, "fishing") }
func TestCatanTwoFishingEventsCompleteHTTPGames(t *testing.T) {
	runTwoCompleteHTTPGames(t, "fishing", true)
}
func runTwoCompleteHTTPGames(t *testing.T, scenario string, events ...bool) {
	coverage := map[string]int{}
	for sample := range 3 {
		t.Run(fmt.Sprint(sample), func(t *testing.T) {
			s, ts, clients, id := newTwoScenarioFullTable(t, scenario, events...)
			restored, modes := map[string]bool{}, map[string]int{}
			restart := func(label string) { s, ts = restartRiversHTTP(t, s, ts, clients, id); restored[label] = true }
			restart("initial")
			steps, automatic, timeouts := 0, 0, 0
			for ; steps < 5000 && !s.rooms[id].Game.Finished; steps++ {
				r := s.rooms[id]
				state := r.Game
				assertTwoHTTPInventory(t, state)
				label := fmt.Sprintf("%s/rolls=%d", state.Phase, len(state.Catan.Two.Rolls))
				if state.Catan.Two.Pending != nil {
					label += "/" + state.Catan.Two.Pending.Kind
				}
				if q := state.Catan.Caravans; q != nil && q.Pending != nil {
					label += fmt.Sprintf("/cursor=%d/second=%v", q.Pending.Cursor, q.Pending.Two.First != nil)
				}
				if !restored[label] {
					restart(label)
					r = s.rooms[id]
					state = r.Game
				}
				actor := twoHTTPActor(state)
				pending := state.CatanPendingActor() >= 0
				if steps%37 == 0 || pending {
					assertTwoHTTPPrivacy(t, clients, state)
				}
				action, err := state.BotAction(actor)
				if err != nil {
					t.Fatal(steps, state.Phase, err)
				}
				coverage["action/"+action.Type]++
				if state.Catan.Two.Pending != nil {
					coverage["build/"+state.Catan.Two.Pending.Kind]++
				}
				mode := "manual"
				if pending {
					count := modes[state.Phase]
					if count%3 == 1 {
						mode = "autoplay"
					}
					if count%3 == 2 {
						mode = "timeout"
					}
					if count == 0 {
						before, _ := json.Marshal(r)
						for _, wrong := range []int{1 - actor, 2} {
							clients[wrong].command(current(clients[wrong]), "action", action, 400)
						}
						clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_end"}, 400)
						after, _ := json.Marshal(s.rooms[id])
						if string(before) != string(after) {
							t.Fatal("invalid response mutated room/clock")
						}
					}
					modes[state.Phase]++
					coverage[state.Phase+"/"+mode]++
				} else if steps%19 == 0 {
					mode = "autoplay"
				}
				version := r.Version
				switch mode {
				case "manual":
					clients[actor].command(current(clients[actor]), "action", action, 200)
				case "autoplay":
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					version = s.rooms[id].Version
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Version <= version {
						t.Fatal("autoplay stalled", state.Phase)
					}
					if !s.rooms[id].Game.Finished {
						setAutoPlay(clients[actor], current(clients[actor]), false, 200)
					}
					automatic++
				case "timeout":
					s.mu.Lock()
					s.rooms[id].TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
					s.expireSetups(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Version <= version {
						t.Fatal("timeout stalled", state.Phase)
					}
					timeouts++
					reclaimTimeoutHumans(t, s, clients, id)
				}
				wagonAdvanced := state.Catan.Caravans != nil && len(s.rooms[id].Game.Catan.Caravans.Wagons) == len(state.Catan.Caravans.Wagons)+1
				neutralAdvanced := state.Catan.Two.Sequence != s.rooms[id].Game.Catan.Two.Sequence
				if pending && !wagonAdvanced && !neutralAdvanced && s.rooms[id].Game.Phase == state.Phase && s.rooms[id].Game.CatanPendingActor() == actor {
					t.Fatal("response did not advance", state.Phase, mode)
				}
			}
			r := s.rooms[id]
			assertTwoHTTPInventory(t, r.Game)
			if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || automatic == 0 || timeouts == 0 {
				t.Fatal("full game incomplete", steps, automatic, timeouts)
			}
			winner := r.Game.Winners[0]
			target := 10
			if scenario == "cities-knights" {
				target = 13
			}
			if scenario == "caravans" {
				target = 12
			}
			if r.Game.Catan.Fishing != nil && r.Game.Catan.Fishing.Tokens.BootOwner == winner {
				target++
			}
			if winner != r.Game.Turn || r.Game.Catan.Players[winner].Score < target {
				t.Fatal("wrong victory")
			}
			restart("finished")
			checkHistory := func() {
				for p := range 2 {
					code, profile := clients[2].request("GET", "/api/players/"+s.rooms[id].Seats[p].ID, nil)
					if code != 200 {
						t.Fatal("history unavailable")
					}
					history := profile["history"].([]any)
					if len(history) != 1 {
						t.Fatal("duplicate or absent history")
					}
					record := history[0].(map[string]any)
					if scenario == "cities-knights" && (record["catanScenario"] != "cities-knights" || record["catanExpansionRules"].(map[string]any)["two_knights"] != game.CatanTwoKnightsRules) {
						t.Fatal("two city history missing", record)
					}
					if scenario == "fishing" && (record["catanScenario"] != "fishing" || record["catanExpansionRules"].(map[string]any)["two_fishing"] != game.CatanTwoFishingRules) {
						t.Fatal("fishing history missing", record)
					}
					if scenario == "rivers" && (record["catanScenario"] != "rivers" || record["catanExpansionRules"].(map[string]any)["rivers"] != game.CatanRiversRules) {
						t.Fatal("river combination history missing", record)
					}
					if scenario == "caravans" && (record["catanScenario"] != "caravans" || record["catanExpansionRules"].(map[string]any)["caravans"] != game.CatanCaravansRules) {
						t.Fatal("merchant train combination history missing", record)
					}
					stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
					want := float64(0)
					if p == winner {
						want = 1
					}
					if stats["wins"] != want || stats["played"] != float64(1) || record["catanRules"] != game.CatanTwoRules || record["catanLayout"] != "variable" || record["catanExpansionRules"].(map[string]any)["two_player"] != game.CatanTwoRules || len(record["players"].([]any)) != 2 {
						t.Fatal("incorrect two-player history", record)
					}
				}
			}
			checkHistory()
			clients[winner].command(map[string]any{"id": id, "version": s.rooms[id].Version}, "action", game.Action{Type: "catan_end"}, 400)
			checkHistory()
			t.Logf("steps=%d autoplay=%d timeouts=%d restarts=%d phases=%v", steps, automatic, timeouts, len(restored)+1, modes)
		})
	}
	phases := []string{"catan_two_build", "catan_two_trade"}
	if scenario == "fishing" {
		phases = []string{"catan_two_build"}
	}
	if scenario == "caravans" {
		phases = append(phases, "catan_caravan_bid", "catan_caravan_place")
	}
	for _, phase := range phases {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			if coverage[phase+"/"+mode] == 0 {
				t.Fatal("missing full-game response path", phase, mode)
			}
		}
	}
	if scenario == "rivers" {
		for _, key := range []string{"action/catan_bridge", "build/bridge", "action/catan_coin_buy", "action/catan_two_robber"} {
			if coverage[key] == 0 {
				t.Fatal("missing natural river combination path", key, coverage)
			}
		}
	}
	t.Logf("scenario=%s coverage=%v", scenario, coverage)

}
