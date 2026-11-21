package cs16_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func percentile(values []int64, percent int) int64 {
	ordered := append([]int64(nil), values...)
	slices.Sort(ordered)
	if len(ordered) == 0 {
		return 0
	}
	return ordered[min(len(ordered)-1, (len(ordered)*percent+99)/100-1)]
}

func TestFourRoomConcurrentInputsWithSlowPeer(t *testing.T) {
	runFourRoomIsolation(t, false)
}

func TestFourTenSeatRoomsWithSlowSpectator(t *testing.T) {
	runFourRoomIsolation(t, true)
}

func runFourRoomIsolation(t *testing.T, performance bool) {
	t.Helper()
	harness := newHTTPRoom(t, 4)
	maps := []string{"de_dust2", "de_inferno", "de_aztec", "cs_assault"}
	ids := make([]string, 4)
	for index, person := range harness.people {
		created := person.request("POST", "/api/rooms", map[string]any{"kind": "cs16", "name": fmt.Sprintf("十席并行%d", index), "capacity": 10, "options": map[string]any{"map": maps[index], "roundSeconds": 60, "maxRounds": 2}}, 201)
		ids[index] = created["id"].(string)
		for range 9 {
			person.command("add_bot", nil, 200)
		}
		person.command("ready", nil, 200)
		person.command("start", nil, 200)
	}
	waitHTTP(t, harness.people[len(harness.people)-1], func(data map[string]any) bool { return data["phase"] == "live" })
	harness.viewer.request("POST", "/api/rooms/"+ids[0]+"/watch", map[string]any{}, 200)
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		connection, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		if tcp, ok := connection.(*net.TCPConn); ok {
			if err = tcp.SetReadBuffer(1024); err != nil {
				connection.Close()
				return nil, err
			}
		}
		return connection, nil
	}}
	defer transport.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	slow, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(harness.viewer.base, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: &http.Client{Jar: harness.viewer.http.Jar, Transport: transport}})
	if err != nil {
		t.Fatal(err)
	}
	defer slow.CloseNow()
	person := harness.people[0]
	fast, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(person.base, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: person.http})
	if err != nil {
		t.Fatal(err)
	}
	defer fast.CloseNow()
	state := person.request("GET", "/api/state", nil, 200)
	room := state["room"].(map[string]any)
	initial := room["game"].(map[string]any)["data"].(map[string]any)
	player := initial["players"].([]any)[0].(map[string]any)
	viewFrame := integer(initial["frame"])
	acks := []int64{}
	lags := []int64{}
	frames := make([]int64, 4)
	for index, host := range harness.people {
		frames[index] = integer(host.room()["game"].(map[string]any)["data"].(map[string]any)["frame"])
	}
	for seq := 1; seq <= 40; seq++ {
		nonce := fmt.Sprintf("cs16-load-input-%d", seq)
		action := map[string]any{"type": "input", "epoch": initial["epoch"], "seq": seq, "life": player["life"], "viewFrame": viewFrame, "moveX": 0, "moveY": 0, "yaw": 0, "pitch": 0, "fire": false}
		message := map[string]any{"type": "input", "serverInstance": state["serverInstance"], "roomId": ids[0], "version": room["version"], "nonce": nonce, "action": action}
		started := time.Now()
		if err = fast.Write(ctx, websocket.MessageText, encode(message)); err != nil {
			t.Fatal(err)
		}
		for {
			_, raw, readErr := fast.Read(ctx)
			if readErr != nil {
				t.Fatal(readErr)
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
						if frame, ok := data["frame"].(json.Number); ok {
							viewFrame = max(viewFrame, integer(frame))
						}
					}
				}
			}
			if answer["type"] == "ack" && answer["nonce"] == nonce {
				if integer(answer["status"]) != 200 {
					t.Fatal(answer)
				}
				acks = append(acks, time.Since(started).Microseconds())
				break
			}
		}
		if seq%4 == 0 {
			for _, host := range harness.people {
				data := host.room()["game"].(map[string]any)["data"].(map[string]any)
				lags = append(lags, max(0, time.Now().UnixMilli()-integer(data["serverAt"])))
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	for index, host := range harness.people {
		current := host.room()
		data := current["game"].(map[string]any)["data"].(map[string]any)
		if len(data["players"].([]any)) != 10 || integer(data["frame"]) <= frames[index]+50 {
			t.Fatal("parallel room stopped", index, data["frame"], frames[index])
		}
		if current["seats"].([]any)[0].(map[string]any)["autoPlay"] == true {
			t.Fatal("slow connection automatically took over human")
		}
	}
	ackP95, lagP95 := percentile(acks, 95), percentile(lags, 95)
	budgetPassed := ackP95 <= 100000 && lagP95 <= 100
	artifact(t, map[string]any{"passed": !performance || budgetPassed, "performanceAcceptance": performance, "performanceBudgetPassed": budgetPassed, "scope": "4 authenticated ten-seat rooms and non-reading 1024-byte-buffer spectator; timing is acceptance only in the ordinary performance case", "rooms": 4, "playersPerRoom": 10, "ackMicroseconds": acks, "ruleLagMS": lags, "ackP95Microseconds": ackP95, "ruleLagP95MS": lagP95})
	if performance && !budgetPassed {
		t.Fatal("parallel room performance budget exceeded", ackP95, lagP95)
	}
}
