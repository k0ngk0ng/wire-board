package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Targeted response fixtures complement the untouched full-game tests. Only
// these fixtures set production phase and resource inventories explicitly.
func TestCatanTwoHTTPResponseClocksPrivacyReplayAndRestart(t *testing.T) {
	for _, scenario := range []string{"trade_roll", "trade_between", "trade_turn", "build_turn", "fish_build_roll", "fish_build_between", "fish_build_turn", "city_trade_roll", "city_trade_between", "city_trade_turn", "city_mixed_roll", "city_mixed_between", "city_mixed_turn", "city_knight_turn"} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(scenario+"/"+mode, func(t *testing.T) {
				scenario := scenario
				recipe := ""
				city := strings.HasPrefix(scenario, "city_")
				mixed := strings.HasPrefix(scenario, "city_mixed_")
				if city {
					recipe = "cities-knights"
					scenario = strings.TrimPrefix(scenario, "city_")
					scenario = strings.Replace(scenario, "mixed_", "trade_", 1)
				}
				if strings.HasPrefix(scenario, "fish_") {
					recipe = "fishing"
				}
				s, ts, clients, id := newTwoScenarioFullTable(t, recipe)
				state := s.rooms[id].Game
				for state.Catan.SetupStep < state.Catan.SetupLimit() {
					a, err := state.BotAction(state.Turn)
					if err != nil {
						t.Fatal(err)
					}
					if err = state.Apply(state.Turn, a); err != nil {
						t.Fatal(err)
					}
				}
				p := state.Turn
				resume := "catan_roll"
				if scenario == "trade_between" || scenario == "fish_build_between" {
					state.Catan.Two.Rolls = []int{2}
					state.Catan.RollID = 1
				}
				if scenario == "trade_turn" || scenario == "build_turn" || scenario == "fish_build_turn" || scenario == "knight_turn" {
					resume = "catan_turn"
					state.Phase = resume
					state.Catan.Two.Rolls = []int{2, 12}
				}
				// One card is taken from the opponent, then two must be returned.
				for player, hand := range [][]int{{1, 1, 0, 0, 0}, {0, 1, 0, 0, 0}} {
					seat := p
					if player == 1 {
						seat = 1 - p
					}
					for c, n := range hand {
						state.Catan.Bank[c] += state.Catan.Players[seat].Resources[c] - n
						state.Catan.Players[seat].Resources[c] = n
					}
				}
				if city {
					state.Catan.RollID = len(state.Catan.Two.Rolls)
					if state.Catan.RollID > 0 {
						state.Catan.CitiesKnights.EventDie = 0
						state.Catan.Dice = []int{1, 1}
					}
					if state.Catan.RollID == 2 {
						state.Catan.Dice = []int{6, 6}
					}
				}
				trigger := game.Action{Type: "catan_two_trade"}
				if mixed {
					trigger.Choice = "mixed"
				}
				if scenario == "knight_turn" {
					for _, color := range []int{2, 4} {
						state.Catan.Bank[color]--
						state.Catan.Players[p].Resources[color]++
					}
					legal := state.View(p)["catan"].(map[string]any)["legal"].(map[string][]int)["knightRecruit"]
					if len(legal) == 0 {
						t.Fatal("no recruitable site")
					}
					trigger = game.Action{Type: "catan_knight_recruit", Vertex: legal[0]}
				}
				if scenario == "build_turn" || strings.HasPrefix(scenario, "fish_") {
					v := state.View(p)["catan"].(map[string]any)
					roads := v["legal"].(map[string][]int)["roads"]
					if recipe == "fishing" {
						roads = v["fishing"].(map[string]any)["legal"].(map[string]any)["roads"].([]int)
					}
					if len(roads) == 0 {
						t.Fatal("no legal fixture road")
					}
					trigger = game.Action{Type: "catan_road", Edge: roads[0]}
					if recipe == "fishing" {
						trigger.Type = "catan_fish_road"
						trigger.Tokens = append([]int{}, state.Catan.Fishing.Tokens.Hands[p]...)
					}
				}
				s.mu.Lock()
				r := s.rooms[id]
				r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				request := map[string]any{"type": "action", "action": trigger, "version": r.Version, "nonce": randomID(12)}
				clients[p].post("/api/rooms/"+id, request, 200)
				r = s.rooms[id]
				phase := r.Game.Phase
				left := r.TurnDeadline - time.Now().UnixMilli()
				if r.Game.CatanPendingActor() != p || left < 119000 || left > 120000 || r.CatanTimeLeft < 44000 || r.CatanTimeLeft > 45000 {
					t.Fatal("response did not pause 45 seconds and grant 120", left, r.CatanTimeLeft)
				}
				before, _ := json.Marshal(r)
				clients[p].post("/api/rooms/"+id, request, 200)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("duplicate trigger redrew or spent twice")
				}
				assertTwoHTTPPrivacy(t, clients, r.Game)
				remaining := r.CatanTimeLeft
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				r = s.rooms[id]
				clients[p].post("/api/rooms/"+id, request, 200)
				after, _ = json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("persisted nonce failed")
				}
				a, err := r.Game.BotAction(p)
				if err != nil {
					t.Fatal(err)
				}
				for _, wrong := range []int{1 - p, 2} {
					clients[wrong].command(current(clients[wrong]), "action", a, 400)
				}
				clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_end"}, 400)
				after, _ = json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid response renewed deadline")
				}
				at := time.Now()
				response := map[string]any{"type": "action", "action": a, "version": r.Version, "nonce": randomID(12)}
				switch mode {
				case "manual":
					clients[p].post("/api/rooms/"+id, response, 200)
				case "autoplay":
					setAutoPlay(clients[p], current(clients[p]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(at)
					s.mu.Unlock()
					setAutoPlay(clients[p], current(clients[p]), false, 200)
				case "timeout":
					at = time.UnixMilli(r.TurnDeadline)
					s.mu.Lock()
					s.expireSetups(at)
					s.mu.Unlock()
				}
				r = s.rooms[id]
				if r.Game.Phase != resume || r.Game.CatanPendingActor() != -1 || r.TurnDeadline < at.UnixMilli()+remaining-100 || r.TurnDeadline > at.UnixMilli()+remaining+1000 {
					t.Fatal("response did not restore original time", phase, mode, r.Game.Phase, r.TurnDeadline-at.UnixMilli(), remaining)
				}
				if (scenario == "trade_between" || scenario == "fish_build_between") && (len(r.Game.Catan.Two.Rolls) != 1 || r.Game.Catan.Two.Rolls[0] != 2 || r.Game.Catan.RollID != 1) {
					t.Fatal("between-roll response repeated or lost production")
				}
				assertTwoHTTPInventory(t, r.Game)
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				before, _ = json.Marshal(s.rooms[id])
				if mode == "manual" {
					clients[p].post("/api/rooms/"+id, response, 200)
				}
				clients[p].command(current(clients[p]), "action", a, 400)
				after, _ = json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("completed response replayed")
				}
			})
		}
	}
}

func TestCatanTwoPrivateConfigurationAndReadiness(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("双人配置房主")
	guest.register("双人配置朋友")
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "双人配置", "capacity": 3}, 201)
	id := raw["id"].(string)
	host.command(current(host), "catan_two", nil, 400)
	guest.command(current(host), "join", nil, 200)
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	s.mu.Lock()
	if err := s.rooms[id].setCatanTwo(); err != nil {
		t.Fatal(err)
	}
	if err := s.save(s.rooms[id]); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready {
			t.Fatal("configuration did not clear readiness")
		}
	}
	host.command(current(host), "start", nil, 400)
	for _, o := range []game.CatanOptions{{Helpers: true}, {FiveSix: true}} {
		r := s.rooms[id]
		before, _ := json.Marshal(r)
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "version": r.Version, "nonce": randomID(12), "catanOptions": o}, 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("invalid combo changed waiting room")
		}
	}
	for _, bad := range []func(*Room){
		func(r *Room) { r.CatanTwoRules = "unknown" },
		func(r *Room) { r.Capacity = 3 },
		func(r *Room) { r.CatanOptions.Helpers = true },
		func(r *Room) { r.CatanSeafarers = &game.CatanSeafarersSetup{} },
		func(r *Room) { r.CatanBaseConfiguration = &game.CatanBaseConfiguration{} },
	} {
		r := *s.rooms[id]
		bad(&r)
		if err := r.validateCatanTwoSetup(); err == nil {
			t.Fatal("invalid private recipe accepted", fmt.Sprint(r.CatanTwoRules, r.Capacity))
		}
	}
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
	guest.command(current(guest), "start", nil, 400)
	host.command(current(host), "start", nil, 200)
	// Archive rules must come from the active game, not an obsolete draft.
	s.mu.Lock()
	s.rooms[id].CatanTwoRules = "stale"
	s.mu.Unlock()
	host.command(current(host), "close", nil, 200)
	code, profile := host.request("GET", "/api/players/"+s.rooms[id].Seats[0].ID, nil)
	if code != 200 || profile["history"].([]any)[0].(map[string]any)["catanRules"] != game.CatanTwoRules {
		t.Fatal("archive used stale draft")
	}
}
