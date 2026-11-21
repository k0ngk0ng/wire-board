package cs16_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type delayedWriteConnection struct {
	net.Conn
	delay *atomic.Int64
}

func (connection *delayedWriteConnection) Write(data []byte) (int, error) {
	if delay := connection.delay.Swap(0); delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	return connection.Conn.Write(data)
}

func TestDelayedReliableInputDoesNotResurrectAttack(t *testing.T) {
	harness := newHTTPRoom(t, 2)
	host, peer := harness.people[0], harness.people[1]
	created := host.request("POST", "/api/rooms", map[string]any{"kind": "cs16", "name": "传输过期边沿验收", "capacity": 2}, 201)
	id := created["id"].(string)
	peer.request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": host.room()["version"], "nonce": "cs16-late-peer-join"}, 200)
	host.command("ready", nil, 200)
	peer.command("ready", nil, 200)
	host.command("start", nil, 200)
	initial := waitHTTP(t, host, func(data map[string]any) bool { return data["phase"] == "live" })
	var delay atomic.Int64
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		connection, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return &delayedWriteConnection{Conn: connection, delay: &delay}, nil
	}}
	defer transport.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(host.base, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: &http.Client{Jar: host.http.Jar, Transport: transport}})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.CloseNow()
	state := host.request("GET", "/api/state", nil, 200)
	room := state["room"].(map[string]any)
	initial = room["game"].(map[string]any)["data"].(map[string]any)
	player := initial["players"].([]any)[0].(map[string]any)
	statuses := []int64{}
	for ordinal := 1; ordinal <= 2; ordinal++ {
		nonce := fmt.Sprintf("cs16-old-edge-%d", ordinal)
		action := map[string]any{"type": "input", "epoch": initial["epoch"], "seq": ordinal, "life": player["life"], "viewFrame": initial["frame"], "moveX": 0, "moveY": 0, "yaw": 0, "pitch": 0, "fire": ordinal == 1}
		message := map[string]any{"type": "input", "serverInstance": state["serverInstance"], "roomId": id, "version": room["version"], "nonce": nonce, "action": action}
		if ordinal == 1 {
			delay.Store(650)
		}
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
			if answer["type"] == "ack" && answer["nonce"] == nonce {
				statuses = append(statuses, integer(answer["status"]))
				break
			}
		}
	}
	after := host.room()["game"].(map[string]any)["data"].(map[string]any)
	observedFrame := integer(after["frame"])
	after = waitHTTP(t, host, func(data map[string]any) bool { return integer(data["frame"]) >= observedFrame+6 })
	oldShots := 0
	for _, item := range after["events"].([]any) {
		event := item.(map[string]any)
		if event["type"] == "shot" && integer(event["seat"]) == 0 {
			oldShots++
		}
	}
	passed := len(statuses) == 2 && statuses[0] != 200 && statuses[1] != 200 && integer(after["players"].([]any)[0].(map[string]any)["receivedSeq"]) == 0 && oldShots == 0
	artifact(t, map[string]any{"passed": passed, "scope": "real reliable WebSocket outbound write held 650ms while server kept advancing, stale press and release in same life", "initialFrame": initial["frame"], "afterFrame": after["frame"], "statuses": statuses, "oldShots": oldShots, "after": after})
	if !passed {
		t.Fatal("transport-delayed edges were accepted or replayed", statuses, oldShots)
	}
	freshPlayer := after["players"].([]any)[0].(map[string]any)
	host.command("action", map[string]any{"action": map[string]any{"type": "input", "epoch": after["epoch"], "seq": 3, "life": freshPlayer["life"], "viewFrame": after["frame"], "moveX": 0, "moveY": 0, "yaw": 0, "pitch": 0, "fire": true}}, 200)
	waitHTTP(t, host, func(data map[string]any) bool {
		return integer(data["players"].([]any)[0].(map[string]any)["appliedSeq"]) == 3
	})
}
