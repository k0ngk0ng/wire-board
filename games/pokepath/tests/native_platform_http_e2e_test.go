package pokepath_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	pokepath "github.com/k0ngk0ng/wire-board/games/pokepath/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

type nativeHTTPClient struct {
	client *http.Client
	base   string
	serial int
	name   string
}

func (client *nativeHTTPClient) exchange(method, path string, body any) (map[string]any, int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	request, err := http.NewRequest(method, client.base+path, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, response.StatusCode, err
	}
	return result, response.StatusCode, nil
}

func (client *nativeHTTPClient) request(t *testing.T, method, path string, body any, expected int) map[string]any {
	t.Helper()
	result, status, err := client.exchange(method, path, body)
	if err != nil || status != expected {
		t.Fatalf("%s %s status=%d expected=%d error=%v response=%v", method, path, status, expected, err, result)
	}
	return result
}

func nativeHTTPObject(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func (client *nativeHTTPClient) room(t *testing.T) map[string]any {
	t.Helper()
	return nativeHTTPObject(client.request(t, "GET", "/api/state", nil, 200)["room"])
}

func nativeHTTPData(room map[string]any) map[string]any {
	return nativeHTTPObject(nativeHTTPObject(room["game"])["data"])
}

func (client *nativeHTTPClient) command(t *testing.T, kind string, body map[string]any, expected int) map[string]any {
	t.Helper()
	room := client.room(t)
	client.serial++
	request := map[string]any{"type": kind, "version": room["version"], "nonce": fmt.Sprintf("native-%s-%012d", client.name, client.serial)}
	for key, value := range body {
		request[key] = value
	}
	return client.request(t, "POST", "/api/rooms/"+room["id"].(string), request, expected)
}

func (client *nativeHTTPClient) action(t *testing.T, action map[string]any, expected int) map[string]any {
	t.Helper()
	room := client.room(t)
	action["epoch"] = nativeHTTPData(room)["epoch"]
	return client.command(t, "action", map[string]any{"action": action}, expected)
}

func TestNativePlatformHTTPSharedActionsNonceAndRollback(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, ".local", "pokepath", "native-verification", "platform-http")
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	dataDirectory, err := os.MkdirTemp(output, "database-")
	if err != nil {
		t.Fatal(err)
	}
	configuration := server.Config{DataDir: dataDirectory, InviteCode: "native-pokepath-http", AdminUsername: "巡径Go管理员", AdminPassword: "native-pokepath-password", Modules: []gameapi.Module{pokepath.Module()}}
	application, err := server.New(configuration, fstest.MapFS{"index.html": {Data: []byte("Independent Go multiplayer acceptance")}})
	if err != nil {
		t.Fatal(err)
	}
	host := httptest.NewServer(application.Handler())
	t.Cleanup(func() {
		host.Close()
		application.Close()
		os.RemoveAll(dataDirectory)
	})
	clients := []*nativeHTTPClient{}
	for index := 0; index < 3; index++ {
		jar, _ := cookiejar.New(nil)
		client := &nativeHTTPClient{client: &http.Client{Jar: jar, Timeout: 10 * time.Second}, base: host.URL, name: fmt.Sprintf("player%d", index)}
		if index == 0 {
			client.request(t, "POST", "/api/login", map[string]any{"name": "巡径Go管理员", "password": "native-pokepath-password"}, 200)
		} else {
			client.request(t, "POST", "/api/register", map[string]any{"name": fmt.Sprintf("巡径Go队友%d", index), "password": "native-pokepath-password", "invite": "native-pokepath-http"}, 200)
		}
		clients = append(clients, client)
	}
	owner, partner, spectator := clients[0], clients[1], clients[2]
	created := owner.request(t, "POST", "/api/rooms", map[string]any{"kind": "pokepath", "name": "独立Go合作验收", "capacity": 2, "options": map[string]any{"starter": "charmander"}}, 201)
	roomID := created["id"].(string)
	partner.request(t, "POST", "/api/rooms/"+roomID, map[string]any{"type": "join", "version": owner.room(t)["version"], "nonce": "native-pokepath-join-000001"}, 200)
	owner.command(t, "ready", nil, 200)
	partner.command(t, "ready", nil, 200)
	owner.command(t, "start", nil, 200)
	spectator.request(t, "POST", "/api/rooms/"+roomID+"/watch", map[string]any{}, 200)
	room := owner.room(t)
	data := nativeHTTPData(room)
	pokemonID := nativeHTTPObject(data["team"].([]any)[0])["id"]
	spectator.request(t, "POST", "/api/rooms/"+roomID, map[string]any{"type": "action", "version": room["version"], "nonce": "native-pokepath-watch-denied-1", "action": map[string]any{"epoch": data["epoch"], "type": "upgrade", "pokemonID": pokemonID}}, 400)
	duplicate := map[string]any{"type": "action", "version": room["version"], "nonce": "native-pokepath-once-000001", "action": map[string]any{"epoch": data["epoch"], "type": "speed"}}
	owner.request(t, "POST", "/api/rooms/"+roomID, duplicate, 200)
	speed := nativeHTTPData(owner.room(t))["speed"]
	owner.request(t, "POST", "/api/rooms/"+roomID, duplicate, 200)
	if nativeHTTPData(owner.room(t))["speed"] != speed {
		t.Fatal("duplicate HTTP nonce executed the independent rule twice")
	}
	owner.action(t, map[string]any{"type": "mapOpen", "clientInputID": "map-open-shared-1"}, 200)
	mapRandom := nativeHTTPData(owner.room(t))["random"]
	owner.action(t, map[string]any{"type": "mapOpen", "clientInputID": "map-open-shared-1"}, 200)
	if nativeHTTPData(owner.room(t))["random"] != mapRandom {
		t.Fatal("duplicate map-open client input rerolled the shared island offer")
	}
	room = owner.room(t)
	data = nativeHTTPData(room)
	type outcome struct {
		status int
		result map[string]any
		err    error
	}
	results := make([]outcome, 2)
	var group sync.WaitGroup
	start := make(chan struct{})
	for index, client := range []*nativeHTTPClient{owner, partner} {
		group.Add(1)
		go func(index int, client *nativeHTTPClient) {
			defer group.Done()
			<-start
			body := map[string]any{"type": "action", "version": room["version"], "nonce": fmt.Sprintf("native-pokepath-race-%012d", index), "action": map[string]any{"epoch": data["epoch"], "type": "upgrade", "pokemonID": pokemonID, "clientInputID": fmt.Sprintf("upgrade-seat-%d", index)}}
			results[index].result, results[index].status, results[index].err = client.exchange("POST", "/api/rooms/"+roomID, body)
		}(index, client)
	}
	close(start)
	group.Wait()
	successes := 0
	for _, result := range results {
		if result.err != nil || result.status != 200 && result.status != 400 {
			t.Fatalf("unexpected concurrent result: %+v", result)
		}
		if result.status == 200 {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("shared balance admitted %d concurrent upgrades", successes)
	}
	afterRace := nativeHTTPData(owner.room(t))
	if afterRace["gold"] != float64(10) || nativeHTTPObject(afterRace["team"].([]any)[0])["level"] != float64(2) || nativeHTTPData(partner.room(t))["gold"] != float64(50) {
		t.Fatal("independent gold or stable owned-Pokémon identity diverged")
	}
	personalRoom := owner.room(t)
	personalResults := make([]outcome, 2)
	personalStart := make(chan struct{})
	for index, client := range []*nativeHTTPClient{owner, partner} {
		quote := nativeHTTPObject(nativeHTTPData(client.room(t))["progression"])["remainingEggs"]
		group.Add(1)
		go func(index int, client *nativeHTTPClient, quote any) {
			defer group.Done()
			<-personalStart
			body := map[string]any{"type": "action", "version": personalRoom["version"], "nonce": fmt.Sprintf("native-personal-shop-%012d", index), "action": map[string]any{"epoch": nativeHTTPData(personalRoom)["epoch"], "type": "egg", "quote": quote, "clientInputID": fmt.Sprintf("personal-shop-seat-%d", index)}}
			personalResults[index].result, personalResults[index].status, personalResults[index].err = client.exchange("POST", "/api/rooms/"+roomID, body)
		}(index, client, quote)
	}
	close(personalStart)
	group.Wait()
	for _, result := range personalResults {
		if result.err != nil || result.status != 200 {
			t.Fatalf("independent simultaneous purchase failed: status=%d error=%v", result.status, result.err)
		}
	}
	ownerData, partnerData := nativeHTTPData(owner.room(t)), nativeHTTPData(partner.room(t))
	if ownerData["gold"] != float64(0) || partnerData["gold"] != float64(40) || len(ownerData["team"].([]any)) != 4 || len(partnerData["team"].([]any)) != 4 {
		t.Fatal("simultaneous purchases shared a wallet or roster")
	}
	owner.action(t, map[string]any{"type": "pause"}, 200)
	frozen := nativeHTTPData(owner.room(t))["snapshot"]
	database, err := sql.Open("sqlite", filepath.Join(dataDirectory, "wire-board.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec("CREATE TRIGGER native_fail_resume BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'native-action-rollback'); END"); err != nil {
		t.Fatal(err)
	}
	owner.action(t, map[string]any{"type": "resume", "clientInputID": "resume-retry"}, 500)
	if _, err := database.Exec("DROP TRIGGER native_fail_resume"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(frozen, nativeHTTPData(owner.room(t))["snapshot"]) {
		t.Fatal("failed durable action changed the full Go State")
	}
	owner.action(t, map[string]any{"type": "resume", "clientInputID": "resume-retry"}, 200)
	owner.action(t, map[string]any{"type": "pause"}, 200)
	view := nativeHTTPData(owner.room(t))
	accepted, _ := view["acceptedInputs"].([]any)
	if len(accepted) == 0 || accepted[len(accepted)-1] != "resume-retry" {
		t.Fatal("a rolled-back input either leaked its receipt or could not be retried")
	}
	record := map[string]any{"command": "go test -race -count=1 ./games/pokepath/tests -run '^TestNativePlatformHTTP'", "passed": true, "independentGoRules": true, "players": 2, "spectatorDenied": true, "concurrentStatuses": []int{results[0].status, results[1].status}, "nonceAppliedOnce": true, "durableActionRollback": true, "fullStateFrozen": frozen, "sharedGold": afterRace["gold"], "sharedLevel": nativeHTTPObject(afterRace["team"].([]any)[0])["level"]}
	encoded, _ := json.MarshalIndent(record, "", "  ")
	if err := os.WriteFile(filepath.Join(output, "result.json"), append(encoded, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}
