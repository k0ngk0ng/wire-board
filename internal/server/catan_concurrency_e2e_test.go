package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanConcurrentHTTPActionsAndSnapshots(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, 4)
	for i := range clients {
		clients[i] = newClient(t, ts.URL)
		clients[i].register(fmt.Sprintf("并发验证%d", i))
	}
	host := clients[0]
	room := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "并发快照", "capacity": 3}, 201)
	id := room["id"].(string)
	for _, client := range clients[1:3] {
		client.command(current(host), "join", nil, 200)
	}
	for _, client := range clients[:3] {
		client.command(current(client), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	ordered := make([]*testClient, 3)
	for _, client := range clients[:3] {
		uid := client.state()["user"].(map[string]any)["id"].(string)
		s.mu.Lock()
		for seat, player := range s.rooms[id].Seats {
			if player.ID == uid {
				ordered[seat] = client
			}
		}
		s.mu.Unlock()
	}
	s.mu.Lock()
	initialVersion := s.rooms[id].Version
	s.mu.Unlock()
	type observation struct {
		requests int
		err      error
	}
	results := make(chan observation, len(clients))
	start := make(chan struct{})
	var readers sync.WaitGroup
	for _, client := range clients {
		readers.Add(1)
		go func(client *testClient) {
			defer readers.Done()
			<-start
			for count := 0; count < 30; count++ {
				response, err := client.client.Get(client.base + "/api/state")
				if err != nil {
					results <- observation{count, err}
					return
				}
				var snapshot map[string]any
				decoder := json.NewDecoder(response.Body)
				decoder.UseNumber()
				err = decoder.Decode(&snapshot)
				response.Body.Close()
				if err != nil || response.StatusCode != 200 {
					results <- observation{count, fmt.Errorf("snapshot status=%d error=%v", response.StatusCode, err)}
					return
				}
			}
			results <- observation{requests: 30}
		}(client)
	}
	defer readers.Wait()
	close(start)
	actions := 0
	for step := 0; step < 32; step++ {
		s.mu.Lock()
		state := s.rooms[id].Game
		if state.Finished {
			s.mu.Unlock()
			break
		}
		actor := twoHTTPActor(state)
		action, err := state.BotAction(actor)
		s.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		ordered[actor].command(current(ordered[actor]), "action", action, 200)
		actions++
	}
	readers.Wait()
	requests := 0
	for range clients {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		requests += result.requests
	}
	s.mu.Lock()
	beforeSpectator := s.rooms[id].Version
	s.mu.Unlock()
	denied := clients[3].command(current(host), "action", game.Action{Type: "roll"}, 400)
	if denied["error"] != "只有牌桌玩家可以操作游戏" {
		t.Fatal("spectator action did not fail at the seat permission check", denied)
	}
	s.mu.Lock()
	finalVersion := s.rooms[id].Version
	s.mu.Unlock()
	if requests != 120 || actions < 12 || finalVersion <= initialVersion || finalVersion != beforeSpectator {
		t.Fatalf("requests=%d actions=%d version=%d->%d", requests, actions, initialVersion, finalVersion)
	}
	directory := filepath.Join("..", "..", ".local/catan/verification/concurrency-http")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	report, err := json.Marshal(map[string]any{"passed": true, "requests": requests, "actions": actions, "initialVersion": initialVersion, "finalVersion": finalVersion, "spectatorActionStatus": 400})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "result.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
