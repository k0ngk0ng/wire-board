package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// End-to-end acceptance for every third-module sea nesting added in this
// batch: each configuration creates a real room over HTTP, plays a complete
// game through the public action endpoint, restarts the server mid-game and
// checks the spectator projection. The run writes a repeatable artifact under
// .local so the accepted configurations can be re-checked later.
type seaNestingCase struct {
	family   string
	scenario string
	players  int
	knights  bool
	fishing  bool
	events   bool
}

type seaNestingResult struct {
	Family   string `json:"family"`
	Scenario string `json:"scenario"`
	Players  int    `json:"players"`
	Knights  bool   `json:"knights"`
	Fishing  bool   `json:"fishing"`
	Events   bool   `json:"events"`
	Rounds   int    `json:"rounds"`
	Steps    int    `json:"steps"`
	Winner   int    `json:"winner"`
	Seconds  string `json:"seconds"`
}

func TestCatanSeaNestingsE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("end-to-end nesting acceptance plays full games")
	}
	// Fifteen full games need far more than the shard budget on CI runners
	// (about half an hour there), so the dedicated acceptance job asks for it
	// explicitly instead of crowding a rules shard.
	if os.Getenv("WIRE_BOARD_NESTING_ACCEPTANCE") != "1" {
		t.Skip("run by the dedicated nesting acceptance job")
	}
	cases := []seaNestingCase{
		{"rivers-seafarers", "rivers-shores", 3, true, false, false},
		{"rivers-seafarers", "rivers-fog", 6, true, false, true},
		{"rivers-seafarers", "rivers-desert", 3, false, true, false},
		{"rivers-seafarers", "rivers-tribe", 4, true, true, true},
		{"caravans-seafarers", "caravans-shores", 3, true, false, false},
		{"caravans-seafarers", "caravans-islands", 4, false, true, true},
		{"caravans-seafarers", "caravans-desert", 5, true, true, true},
		{"attack-seafarers", "attack-shores", 3, true, false, false},
		{"attack-seafarers", "attack-desert", 4, true, false, true},
		{"attack-seafarers", "attack-tribe", 6, true, false, false},
		{"attack-seafarers", "attack-wonders", 2, true, false, true},
		{"attack-seafarers", "attack-shores", 3, false, true, true},
		{"attack-seafarers", "attack-desert", 2, true, true, true},
		{"attack-seafarers", "attack-pirates", 2, true, false, true},
		{"attack-seafarers", "attack-pirates", 4, true, true, false},
	}
	results := []seaNestingResult{}
	for _, tc := range cases {
		name := fmt.Sprintf("%s/%s/%d/knights%t/fishing%t/events%t", tc.family, tc.scenario, tc.players, tc.knights, tc.fishing, tc.events)
		t.Run(name, func(t *testing.T) {
			results = append(results, runSeaNestingGame(t, tc))
		})
	}
	writeSeaNestingArtifact(t, results)
}

func runSeaNestingGame(t *testing.T, tc seaNestingCase) seaNestingResult {
	t.Helper()
	started := time.Now()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, tc.players+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("海图嵌套%d", p))
	}
	recipe := map[string]any{"kind": "catan", "name": "海图三模块嵌套", "capacity": tc.players, "catanScenario": tc.scenario}
	if tc.knights {
		recipe["catanCitiesKnights"] = game.CatanCitiesKnightsSetup{}
	}
	if tc.fishing {
		recipe["catanFishing"] = true
	}
	if tc.events {
		recipe["catanEvents"] = game.CatanEventCatalogue
	}
	raw := clients[0].post("/api/rooms", recipe, 201)
	id := raw["id"].(string)
	for p := 1; p < tc.players; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for p := 0; p < tc.players; p++ {
		clients[p].command(current(clients[p]), "ready", nil, 200)
	}
	clients[0].command(current(clients[0]), "start", nil, 200)
	ordered := make([]*testClient, tc.players+1)
	for _, c := range clients[:tc.players] {
		ordered[int(current(c)["you"].(float64))] = c
	}
	ordered[tc.players] = clients[tc.players]
	clients = ordered
	clients[tc.players].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	steps, restored := 0, false
	for ; steps < 24000 && !s.rooms[id].Game.Finished; steps++ {
		g := s.rooms[id].Game
		p := twoHTTPActor(g)
		a, err := g.BotAction(p)
		if err != nil {
			t.Fatalf("step %d phase %s: %v", steps, g.Phase, err)
		}
		clients[p].command(current(clients[p]), "action", a, 200)
		if steps == 83 {
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			restored = true
		}
		if steps%131 == 0 {
			view := current(clients[tc.players])["game"].(map[string]any)["catan"].(map[string]any)
			if view["seafarers"] == nil {
				t.Fatal("spectator view lost the sea components")
			}
			if tc.knights && view["citiesKnights"] == nil {
				t.Fatal("spectator view lost the knight components")
			}
			if tc.fishing && view["fishing"] == nil {
				t.Fatal("spectator view lost the fishing components")
			}
		}
	}
	if !s.rooms[id].Game.Finished || !restored {
		t.Fatalf("incomplete game: round %d phase %s", s.rooms[id].Game.Round, s.rooms[id].Game.Phase)
	}
	winner := -1
	if len(s.rooms[id].Game.Winners) > 0 {
		winner = s.rooms[id].Game.Winners[0]
	}
	result := seaNestingResult{
		Family: tc.family, Scenario: tc.scenario, Players: tc.players,
		Knights: tc.knights, Fishing: tc.fishing, Events: tc.events,
		Rounds: s.rooms[id].Game.Round, Steps: steps, Winner: winner,
		Seconds: time.Since(started).Truncate(time.Millisecond).String(),
	}
	t.Logf("accepted %s/%d: round %d steps %d winner %d in %s", tc.scenario, tc.players, result.Rounds, result.Steps, winner, result.Seconds)
	return result
}

func writeSeaNestingArtifact(t *testing.T, results []seaNestingResult) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", ".local"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(struct {
		Test      string             `json:"test"`
		Recorded  string             `json:"recordedAt"`
		Results   []seaNestingResult `json:"results"`
		TotalRuns int                `json:"totalRuns"`
	}{t.Name(), time.Now().Format(time.RFC3339), results, len(results)}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "catan-sea-nestings-e2e.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var table strings.Builder
	table.WriteString("| 组合 | 海图 | 人数 | 骑士 | 渔夫 | 事件 | 回合 | 步数 | 胜者 | 用时 |\n")
	table.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range results {
		fmt.Fprintf(&table, "| %s | %s | %d | %t | %t | %t | %d | %d | %d | %s |\n",
			r.Family, r.Scenario, r.Players, r.Knights, r.Fishing, r.Events, r.Rounds, r.Steps, r.Winner, r.Seconds)
	}
	if err = os.WriteFile(filepath.Join(dir, "catan-sea-nestings-e2e.md"), []byte(table.String()), 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("artifact written: %s (%d accepted configurations)", path, len(results))
}
