package cs16_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	game "github.com/k0ngk0ng/wire-board/games/cs16/server"
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

type inputPeer struct {
	seat       int
	connection *websocket.Conn
	roomID     string
	epoch      string
	instance   string
	life       int64
	seq        int64
	frame      int64
	version    any
}

type inputTrace struct {
	Seat                   int     `json:"seat"`
	Inputs                 int     `json:"inputs"`
	LastSeq                int64   `json:"lastSeq"`
	AckMicroseconds        []int64 `json:"ackMicroseconds"`
	StreamApplicationBytes []int   `json:"streamApplicationBytes"`
	Error                  string  `json:"error,omitempty"`
}

type inputClock struct {
	At    int64 `json:"at"`
	Frame int64 `json:"frame"`
	LagMS int64 `json:"lagMS"`
}

func tenConcurrentInputs(t *testing.T, harness *httpRoom) ([]inputTrace, []inputClock, highRateReport) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	peers := make([]inputPeer, len(harness.people))
	for seat, person := range harness.people {
		state := person.request("GET", "/api/state", nil, 200)
		room := state["room"].(map[string]any)
		data := room["game"].(map[string]any)["data"].(map[string]any)
		own := data["players"].([]any)[seat].(map[string]any)
		connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(person.base, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: person.http})
		if err != nil {
			t.Fatal(err)
		}
		defer connection.CloseNow()
		peers[seat] = inputPeer{seat: seat, connection: connection, roomID: room["id"].(string), epoch: data["epoch"].(string), instance: state["serverInstance"].(string), life: integer(own["life"]), seq: integer(own["receivedSeq"]), frame: integer(data["frame"]), version: room["version"]}
	}
	results := make(chan inputTrace, len(peers))
	clocks := []inputClock{}
	sample := func() {
		data := harness.people[0].room()["game"].(map[string]any)["data"].(map[string]any)
		at := time.Now().UnixMilli()
		clocks = append(clocks, inputClock{At: at, Frame: integer(data["frame"]), LagMS: max(0, at-integer(data["serverAt"]))})
	}
	sample()
	var workers sync.WaitGroup
	for _, peer := range peers {
		workers.Add(1)
		go func(peer inputPeer) {
			defer workers.Done()
			trace := inputTrace{Seat: peer.seat, AckMicroseconds: []int64{}}
			defer func() { results <- trace }()
			for ordinal := 1; ordinal <= 40; ordinal++ {
				nonce := fmt.Sprintf("cs16-ten-input-%d-%d", peer.seat, ordinal)
				seq := peer.seq + int64(ordinal)
				action := map[string]any{"type": "input", "epoch": peer.epoch, "seq": seq, "life": peer.life, "viewFrame": peer.frame, "moveX": 0, "moveY": 250, "yaw": (ordinal*500 + peer.seat*1000) & 65535, "pitch": 0, "fire": false}
				message := map[string]any{"type": "input", "serverInstance": peer.instance, "roomId": peer.roomID, "version": peer.version, "nonce": nonce, "action": action}
				started := time.Now()
				if err := peer.connection.Write(ctx, websocket.MessageText, encode(message)); err != nil {
					trace.Error = err.Error()
					return
				}
				for {
					_, raw, err := peer.connection.Read(ctx)
					if err != nil {
						trace.Error = err.Error()
						return
					}
					var answer map[string]any
					decoder := json.NewDecoder(bytes.NewReader(raw))
					decoder.UseNumber()
					if decoder.Decode(&answer) != nil {
						continue
					}
					if answer["type"] == "game" {
						if game, ok := answer["game"].(map[string]any); ok {
							if data, ok := game["data"].(map[string]any); ok {
								if integer(data["schema"]) == 2 {
									trace.StreamApplicationBytes = append(trace.StreamApplicationBytes, len(raw))
								}
								if frame, ok := data["frame"].(json.Number); ok {
									peer.frame = max(peer.frame, integer(frame))
								}
							}
						}
					}
					if answer["type"] == "ack" && answer["nonce"] == nonce {
						if integer(answer["status"]) != 200 {
							trace.Error = string(raw)
							return
						}
						trace.AckMicroseconds = append(trace.AckMicroseconds, time.Since(started).Microseconds())
						trace.Inputs++
						trace.LastSeq = seq
						break
					}
				}
				time.Sleep(32 * time.Millisecond)
			}
		}(peer)
	}
	complete := make(chan struct{})
	go func() { workers.Wait(); close(results); close(complete) }()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	monitoring := true
	for monitoring {
		select {
		case <-ticker.C:
			sample()
		case <-complete:
			monitoring = false
		}
	}
	sample()
	elapsed := clocks[len(clocks)-1].At - clocks[0].At
	advanced := (clocks[len(clocks)-1].Frame - clocks[0].Frame) * 10
	if elapsed < 1000 || advanced+200 < elapsed {
		t.Fatal("sustained input starved the 100 Hz rule clock", elapsed, advanced)
	}
	for _, clock := range clocks {
		if clock.LagMS > 200 {
			t.Fatal("rule clock fell outside bounded input window while clients were sending", clock.LagMS)
		}
	}
	traces := make([]inputTrace, len(peers))
	applicationBytes := []int{}
	for trace := range results {
		traces[trace.Seat] = trace
		applicationBytes = append(applicationBytes, trace.StreamApplicationBytes...)
		if trace.Error != "" || trace.Inputs != 40 {
			t.Fatalf("seat %d concurrent stream failed: %s (%d)", trace.Seat, trace.Error, trace.Inputs)
		}
		waitHTTP(t, harness.people[trace.Seat], func(data map[string]any) bool {
			return integer(data["players"].([]any)[trace.Seat].(map[string]any)["appliedSeq"]) >= trace.LastSeq
		})
		peers[trace.Seat].seq = trace.LastSeq
		peers[trace.Seat].frame = clocks[len(clocks)-1].Frame
	}
	sort.Ints(applicationBytes)
	if len(applicationBytes) < 100 || applicationBytes[(len(applicationBytes)*95+99)/100-1] > 2048 {
		t.Fatal("ordinary ten-seat application packet P95 exceeds 2KiB or has insufficient samples", len(applicationBytes), applicationBytes)
	}
	highRate := tenHighRateInputs(t, harness, peers)
	return traces, clocks, highRate
}

func controls(t *testing.T) *gameapi.ControllerDefinition {
	t.Helper()
	raw, err := os.ReadFile("../controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var result gameapi.ControllerDefinition
	if json.Unmarshal(raw, &result) != nil {
		t.Fatal("invalid controller definition")
	}
	return &result
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
	if decoder.Decode(&result) != nil {
		person.t.Fatal("response not JSON", method, path, response.StatusCode)
	}
	if response.StatusCode != status {
		person.t.Fatalf("%s %s: got %d want %d %v", method, path, response.StatusCode, status, result)
	}
	return result
}

func (person *httpPerson) room() map[string]any {
	return person.request("GET", "/api/state", nil, 200)["room"].(map[string]any)
}

func (person *httpPerson) command(kind string, fields map[string]any, status int) map[string]any {
	current := person.room()
	person.serial++
	body := map[string]any{"type": kind, "version": current["version"], "nonce": fmt.Sprintf("cs16-%d-%d", time.Now().UnixNano(), person.serial)}
	for key, value := range fields {
		body[key] = value
	}
	return person.request("POST", "/api/rooms/"+current["id"].(string), body, status)
}

func (room *httpRoom) open(t *testing.T) {
	var err error
	room.host, err = server.New(room.config, fstest.MapFS{"index.html": {Data: []byte("CS 1.6 real HTTP acceptance")}})
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
	root := filepath.Join("..", "..", "..", ".local", "cs16", "tmp")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(root, "server-http-")
	if err != nil {
		t.Fatal(err)
	}
	module := game.Module()
	module.Controls = controls(t)
	room := &httpRoom{config: server.Config{DataDir: directory, InviteCode: "cs16-local-invite", AdminUsername: "CS验收管理员", AdminPassword: "cs16-local-password", Modules: []gameapi.Module{module}}}
	room.open(t)
	for index := 0; index <= count; index++ {
		jar, _ := cookiejar.New(nil)
		person := &httpPerson{t: t, base: room.endpoint.URL, http: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
		if index == 0 {
			person.request("POST", "/api/login", map[string]any{"name": "CS验收管理员", "password": "cs16-local-password"}, 200)
		} else {
			person.request("POST", "/api/register", map[string]any{"name": fmt.Sprintf("CS验收玩家%d", index), "password": "cs16-local-password", "invite": "cs16-local-invite"}, 200)
		}
		if index < count {
			room.people = append(room.people, person)
		} else {
			room.viewer = person
		}
	}
	t.Cleanup(func() { room.endpoint.Close(); room.host.Close(); os.RemoveAll(directory) })
	return room
}

func waitHTTP(t *testing.T, person *httpPerson, predicate func(map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		data := person.room()["game"].(map[string]any)["data"].(map[string]any)
		if predicate(data) {
			return data
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("authoritative state did not advance")
	return nil
}

func TestTenHumansHTTPWebSocketAndSQLiteRecovery(t *testing.T) {
	harness := newHTTPRoom(t, 10)
	host := harness.people[0]
	available := host.request("GET", "/api/state", nil, 200)["availableGames"].([]any)
	published := false
	for _, id := range available {
		published = published || id == "cs16"
	}
	if !published {
		t.Fatal("CS 1.6 unavailable")
	}
	host.request("POST", "/api/rooms", map[string]any{"kind": "cs16", "name": "超员房间", "capacity": 11}, 400)
	created := host.request("POST", "/api/rooms", map[string]any{"kind": "cs16", "name": "十人5对5验收", "capacity": 10}, 201)
	id := created["id"].(string)
	for index := 1; index < 10; index++ {
		harness.people[index].request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": host.room()["version"], "nonce": fmt.Sprintf("cs16-join-%d", index)}, 200)
	}
	harness.viewer.request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": host.room()["version"], "nonce": "eleventh-player"}, 400)
	harness.people[1].command("prepare", map[string]any{"action": map[string]any{"type": "map", "map": "cs_assault"}}, 400)
	harness.people[1].command("prepare", map[string]any{"action": map[string]any{"type": "character", "character": "sas"}}, 200)
	for _, person := range harness.people {
		person.command("ready", nil, 200)
	}
	host.command("start", nil, 200)
	harness.viewer.request("POST", "/api/rooms/"+id+"/watch", map[string]any{}, 200)
	initial := waitHTTP(t, host, func(data map[string]any) bool { return integer(data["frame"]) > 2 })
	if len(initial["players"].([]any)) != 10 {
		t.Fatal("ten players not active")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(host.base, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: host.http})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseNow()
	state := host.request("GET", "/api/state", nil, 200)
	player := initial["players"].([]any)[0].(map[string]any)
	action := map[string]any{"type": "input", "epoch": initial["epoch"], "seq": 1, "life": player["life"], "viewFrame": initial["frame"], "moveX": 1000, "moveY": 0, "yaw": 0, "pitch": 0, "fire": false}
	message := map[string]any{"type": "input", "serverInstance": state["serverInstance"], "roomId": id, "version": host.room()["version"], "nonce": "cs16-deduplicate", "action": action}
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
			if answer["type"] == "ack" && answer["nonce"] == "cs16-deduplicate" {
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
	waitHTTP(t, host, func(data map[string]any) bool { return data["phase"] == "live" })
	traffic, inputClocks, highRate := tenConcurrentInputs(t, harness)
	host.command("autoplay", map[string]any{"enabled": true}, 200)
	autoView := host.room()["game"].(map[string]any)["data"].(map[string]any)
	autoPlayer := autoView["players"].([]any)[0].(map[string]any)
	cancelSeq := integer(autoPlayer["receivedSeq"]) + 100
	cancelAction := map[string]any{"type": "input", "epoch": autoView["epoch"], "seq": cancelSeq, "life": autoPlayer["life"], "viewFrame": autoView["frame"], "moveX": 0, "moveY": 0, "yaw": 0, "pitch": 0, "fire": false, "cancel": true}
	host.command("action", map[string]any{"action": cancelAction}, 200)
	cancelAction["seq"] = cancelSeq + 1
	cancelAction["moveY"] = 1000
	host.command("action", map[string]any{"action": cancelAction}, 400)
	cancelAction["moveY"] = 0
	delete(cancelAction, "cancel")
	host.command("action", map[string]any{"action": cancelAction}, 400)
	host.command("autoplay", map[string]any{"enabled": false}, 200)
	before := host.room()["game"].(map[string]any)["data"].(map[string]any)
	harness.endpoint.Close()
	harness.host.Close()
	harness.open(t)
	after := host.room()["game"].(map[string]any)["data"].(map[string]any)
	if before["epoch"] != after["epoch"] || before["startAt"] != after["startAt"] {
		t.Fatal("restart changed match identity")
	}
	for _, seat := range host.room()["seats"].([]any) {
		if seat.(map[string]any)["autoPlay"] == true {
			t.Fatal("human automatically taken over")
		}
	}
	artifact(t, map[string]any{"passed": true, "scope": "10 authenticated humans each stream 40 ordinary commands then 100Hz yaw on the same connections; clock sampled during traffic; real HTTP/WS/SQLite, duplicate nonce, autoplay cleanup-only permission, handoff and restart", "concurrentInputs": traffic, "highRate": highRate, "inputClock": inputClocks, "autoplayCancelOnly": true, "confirmed": confirmed, "before": before, "after": after})
}

func TestControllerDefinitionHTTPPersistence(t *testing.T) {
	definition := controls(t)
	if definition.Layout != "game" || len(definition.Actions) < 20 {
		t.Fatal("incomplete FPS controls")
	}
	for id, key := range map[string]string{"lastWeapon": "KeyQ", "buyPrimaryAmmo": "Comma", "buySecondaryAmmo": "Period"} {
		found := false
		for _, action := range definition.Actions {
			if action.ID == id {
				for _, binding := range action.Keys {
					found = found || binding == key
				}
			}
		}
		if !found {
			t.Fatal("classic PC action missing", id, key)
		}
	}
	harness := newHTTPRoom(t, 2)
	person := harness.people[1]
	settings := person.request("GET", "/api/controllers?game=cs16", nil, 200)
	found := false
	for _, value := range settings["models"].([]any) {
		found = found || value.(map[string]any)["id"] == "game:cs16"
	}
	if !found {
		t.Fatal("CS controls unavailable")
	}
	profile := settings["profiles"].(map[string]any)["game:cs16"].(map[string]any)
	person.request("POST", "/api/controllers/game:cs16", profile, 200)
	artifact(t, map[string]any{"passed": true, "scope": "real control GET and profile save HTTP", "actions": definition.Actions})
}
