package hotlinemiami_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	game "github.com/k0ngk0ng/wire-board/games/hotlinemiami/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

type httpPerson struct {
	t      *testing.T
	base   string
	http   *http.Client
	serial int
}

type httpRoom struct {
	host     *server.Server
	endpoint *httptest.Server
	config   server.Config
	people   []*httpPerson
	viewer   *httpPerson
}

func (person *httpPerson) request(method, path string, body any, status int) map[string]any {
	person.t.Helper()
	request, err := http.NewRequest(method, person.base+path, bytes.NewReader(encode(body)))
	if err != nil {
		person.t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := person.http.Do(request)
	if err != nil {
		person.t.Fatal(err)
	}
	defer response.Body.Close()
	var result map[string]any
	decoder := json.NewDecoder(response.Body)
	decoder.UseNumber()
	if err = decoder.Decode(&result); err != nil {
		person.t.Fatal(err)
	}
	if response.StatusCode != status {
		person.t.Fatalf("%s %s: %d expected %d %v", method, path, response.StatusCode, status, result)
	}
	return result
}

func (person *httpPerson) room() map[string]any {
	return person.request("GET", "/api/state", nil, 200)["room"].(map[string]any)
}

func (person *httpPerson) command(kind string, fields map[string]any, status int) map[string]any {
	room := person.room()
	person.serial++
	body := map[string]any{"type": kind, "version": room["version"], "nonce": fmt.Sprintf("hm-%d-%d", time.Now().UnixNano(), person.serial)}
	for key, value := range fields {
		body[key] = value
	}
	return person.request("POST", "/api/rooms/"+room["id"].(string), body, status)
}

func (room *httpRoom) open(t *testing.T) {
	var err error
	room.host, err = server.New(room.config, fstest.MapFS{"index.html": {Data: []byte("Hotline Miami HTTP acceptance")}})
	if err != nil {
		t.Fatal(err)
	}
	room.endpoint = httptest.NewServer(room.host.Handler())
	for _, person := range append(append([]*httpPerson{}, room.people...), room.viewer) {
		if person != nil {
			person.base = room.endpoint.URL
		}
	}
}

func newHTTPRoom(t *testing.T, count int) *httpRoom {
	t.Helper()
	module := game.Module()
	module.Controls = controls(t)
	room := &httpRoom{config: server.Config{DataDir: t.TempDir(), InviteCode: "hotline-http-test", AdminUsername: "热线验收管理员", AdminPassword: "hotline-local-test", Modules: []gameapi.Module{module}}}
	room.open(t)
	for i := 0; i <= count; i++ {
		jar, _ := cookiejar.New(nil)
		person := &httpPerson{t: t, base: room.endpoint.URL, http: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
		if i == 0 {
			person.request("POST", "/api/login", map[string]any{"name": "热线验收管理员", "password": "hotline-local-test"}, 200)
		} else {
			person.request("POST", "/api/register", map[string]any{"name": fmt.Sprintf("热线玩家%d", i), "password": "hotline-local-test", "invite": "hotline-http-test"}, 200)
		}
		if i < count {
			room.people = append(room.people, person)
		} else {
			room.viewer = person
		}
	}
	t.Cleanup(func() { room.endpoint.Close(); room.host.Close() })
	return room
}

func waitHTTP(t *testing.T, person *httpPerson, predicate func(map[string]any) bool) map[string]any {
	t.Helper()
	until := time.Now().Add(4 * time.Second)
	for time.Now().Before(until) {
		data := person.room()["game"].(map[string]any)["data"].(map[string]any)
		if predicate(data) {
			return data
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("authoritative state did not advance")
	return nil
}

func TestHTTPPreparationWebSocketAndRestore(t *testing.T) {
	harness := newHTTPRoom(t, 2)
	host, peer := harness.people[0], harness.people[1]
	available := peer.request("GET", "/api/state", nil, 200)["availableGames"].([]any)
	published := false
	for _, id := range available {
		if id == "hotlinemiami" {
			published = true
		}
	}
	if !published {
		t.Fatal("new game not published")
	}
	host.request("POST", "/api/rooms", map[string]any{"kind": "hotlinemiami", "name": "invalid", "capacity": 1}, 400)
	created := host.request("POST", "/api/rooms", map[string]any{"kind": "hotlinemiami", "name": "迈阿密混战验收", "capacity": 4}, 201)
	id := created["id"].(string)
	peer.request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": host.room()["version"], "nonce": "hm-peer-join"}, 200)
	peer.command("prepare", map[string]any{"action": map[string]any{"type": "character", "seat": 0, "character": "biker"}}, 400)
	peer.command("prepare", map[string]any{"action": map[string]any{"type": "character", "seat": 1, "character": "biker"}}, 200)
	host.command("add_bot", nil, 200)
	host.command("add_bot", nil, 200)
	host.command("prepare", map[string]any{"action": map[string]any{"type": "weapon", "seat": 2, "weapon": "uzi"}}, 200)
	host.command("ready", nil, 200)
	peer.command("ready", nil, 200)
	host.command("start", nil, 200)
	harness.viewer.request("POST", "/api/rooms/"+id+"/watch", map[string]any{}, 200)
	initial := waitHTTP(t, host, func(data map[string]any) bool { return integer(data["frame"]) > 2 })
	snapshot := host.room()
	player := initial["players"].([]any)[0].(map[string]any)
	input := map[string]any{"type": "input", "epoch": initial["epoch"], "seq": 1, "life": player["life"], "moveX": 1000, "moveY": 0, "aim": 0, "fire": false}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(host.base, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: host.http})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseNow()
	state := host.request("GET", "/api/state", nil, 200)
	message := map[string]any{"type": "input", "serverInstance": state["serverInstance"], "roomId": id, "version": snapshot["version"], "nonce": "hm-repeat-once", "action": input}
	for attempt := 0; attempt < 2; attempt++ {
		if err = connection.Write(ctx, websocket.MessageText, encode(message)); err != nil {
			t.Fatal(err)
		}
		for {
			_, raw, readErr := connection.Read(ctx)
			if readErr != nil {
				t.Fatal(readErr)
			}
			var answer map[string]any
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			if decoder.Decode(&answer) != nil {
				continue
			}
			if answer["type"] == "ack" && answer["nonce"] == "hm-repeat-once" {
				if integer(answer["status"]) != 200 {
					t.Fatal(answer)
				}
				break
			}
		}
	}
	confirmed := waitHTTP(t, host, func(data map[string]any) bool {
		return integer(data["players"].([]any)[0].(map[string]any)["appliedSeq"]) == 1
	})
	if confirmed["epoch"] != initial["epoch"] {
		t.Fatal("input reset epoch")
	}
	host.command("autoplay", map[string]any{"enabled": true}, 200)
	host.command("autoplay", map[string]any{"enabled": false}, 200)
	beforeRestart := host.room()["game"].(map[string]any)["data"].(map[string]any)
	harness.endpoint.Close()
	harness.host.Close()
	harness.open(t)
	afterRestart := host.room()["game"].(map[string]any)["data"].(map[string]any)
	if beforeRestart["epoch"] != afterRestart["epoch"] || beforeRestart["startAt"] != afterRestart["startAt"] {
		t.Fatal("restart reset match")
	}
	if host.room()["seats"].([]any)[0].(map[string]any)["autoPlay"] == true {
		t.Fatal("human automatically taken over")
	}
	artifact(t, map[string]any{"passed": true, "scope": "real authenticated HTTP/WS/SQLite, preparation, duplicate nonce, voluntary handoff and recovery", "before": beforeRestart, "after": afterRestart})
}
