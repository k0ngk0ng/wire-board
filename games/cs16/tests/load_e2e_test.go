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
	"sync/atomic"
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

func expiredLoadInput(answer map[string]any, performance bool) bool {
	nonce, ok := answer["nonce"].(string)
	return !performance && ok && nonce != "" && answer["type"] == "ack" && integer(answer["status"]) == 400 && answer["error"] == "操作排队已过期" && answer["durable"] == false
}

func TestFourRoomConcurrentInputsWithSlowPeer(t *testing.T) {
	runFourRoomIsolation(t, false)
}

func TestFourTenSeatRoomsWithSlowSpectator(t *testing.T) {
	runFourRoomIsolation(t, true)
}

func TestSmallMapFourRoomConcurrentInputsWithSlowPeer(t *testing.T) {
	runFourRoomMapIsolation(t, false, []string{"fy_iceworld", "cs_bloodstrike", "fy_iceworld", "cs_bloodstrike"})
}

func TestSmallMapFourTenSeatRoomsWithSlowSpectator(t *testing.T) {
	runFourRoomMapIsolation(t, true, []string{"fy_iceworld", "cs_bloodstrike", "fy_iceworld", "cs_bloodstrike"})
}

func runFourRoomIsolation(t *testing.T, performance bool) {
	t.Helper()
	runFourRoomMapIsolation(t, performance, []string{"de_dust2", "de_inferno", "de_aztec", "cs_assault"})
}

func runFourRoomMapIsolation(t *testing.T, performance bool, maps []string) {
	t.Helper()
	if len(maps) != 4 {
		t.Fatal("isolation acceptance requires exactly four rooms")
	}
	var harness *httpRoom
	if performance {
		harness = newHTTPRoom(t, 4)
	} else {
		harness = newFortyHumanHTTPRoom(t)
	}
	hosts := make([]*httpPerson, 4)
	for index := range hosts {
		seat := index
		if !performance {
			seat = index * 10
		}
		hosts[index] = harness.people[seat]
	}
	ids := make([]string, 4)
	for index, person := range hosts {
		created := person.request("POST", "/api/rooms", map[string]any{"kind": "cs16", "name": fmt.Sprintf("十席并行%d", index), "capacity": 10, "options": map[string]any{"map": maps[index], "roundSeconds": 60, "maxRounds": 2}}, 201)
		ids[index] = created["id"].(string)
		if performance {
			for range 9 {
				person.command("add_bot", nil, 200)
			}
		} else {
			for seat := 1; seat < 10; seat++ {
				member := harness.people[index*10+seat]
				member.request("POST", "/api/rooms/"+ids[index], map[string]any{"type": "join", "version": person.room()["version"], "nonce": fmt.Sprintf("race-room-%d-seat-%d", index, seat)}, 200)
				member.command("ready", nil, 200)
			}
		}
		person.command("ready", nil, 200)
		person.command("start", nil, 200)
		if !performance {
			for _, value := range person.room()["seats"].([]any) {
				if value.(map[string]any)["bot"] == true {
					t.Fatal("race isolation fixture must use real authenticated human seats")
				}
			}
		}
	}
	waitHTTP(t, hosts[len(hosts)-1], func(data map[string]any) bool { return data["phase"] == "live" })
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
	person := hosts[0]
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
	applicationBytes, firingApplicationBytes := []int64{}, []int64{}
	lastWireEventID := integer(initial["eventId"])
	shotEvents := 0
	frames := make([]int64, 4)
	var latestFrame atomic.Int64
	latestFrame.Store(viewFrame)
	streamAcks := make(chan map[string]any, 1)
	streamErrors := make(chan error, 1)
	streamReady := make(chan struct{})
	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		for {
			_, raw, readErr := fast.Read(ctx)
			if readErr != nil {
				streamErrors <- readErr
				return
			}
			var answer map[string]any
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			if decodeErr := decoder.Decode(&answer); decodeErr != nil {
				streamErrors <- decodeErr
				return
			}
			if answer["type"] == "game" {
				if game, ok := answer["game"].(map[string]any); ok {
					if data, ok := game["data"].(map[string]any); ok {
						applicationBytes = append(applicationBytes, int64(len(raw)))
						shots := 0
						if events, ok := data["events"].([]any); ok {
							for _, event := range events {
								value := event.(map[string]any)
								if integer(value["id"]) > lastWireEventID && value["type"] == "shot" {
									shots++
								}
							}
						}
						lastWireEventID = max(lastWireEventID, integer(data["eventId"]))
						if shots > 0 {
							shotEvents += shots
							firingApplicationBytes = append(firingApplicationBytes, int64(len(raw)))
						}
						if frame, ok := data["frame"].(json.Number); ok {
							latestFrame.Store(max(latestFrame.Load(), integer(frame)))
							select {
							case <-streamReady:
							default:
								close(streamReady)
							}
						}
					}
				}
			}
			if answer["type"] == "ack" {
				select {
				case streamAcks <- answer:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	defer func() { fast.CloseNow(); <-streamDone }()
	select {
	case <-streamReady:
	case readErr := <-streamErrors:
		t.Fatal(readErr)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	for index, host := range hosts {
		frames[index] = integer(host.room()["game"].(map[string]any)["data"].(map[string]any)["frame"])
	}
	inputCount, acceptedInputs, expiredQueuedInputs := 40, 0, 0
	for seq := 1; seq <= inputCount; seq++ {
		nonce := fmt.Sprintf("cs16-load-input-%d", seq)
		action := map[string]any{"type": "input", "epoch": initial["epoch"], "seq": seq, "life": player["life"], "viewFrame": latestFrame.Load(), "moveX": 0, "moveY": 0, "yaw": 0, "pitch": 0, "fire": false}
		message := map[string]any{"type": "input", "serverInstance": state["serverInstance"], "roomId": ids[0], "version": room["version"], "nonce": nonce, "action": action}
		started := time.Now()
		if err = fast.Write(ctx, websocket.MessageText, encode(message)); err != nil {
			t.Fatal(err)
		}
		select {
		case readErr := <-streamErrors:
			t.Fatal(readErr)
		case answer := <-streamAcks:
			if answer["nonce"] != nonce {
				t.Fatal("unexpected input acknowledgement", answer)
			}
			if integer(answer["status"]) == 200 {
				acceptedInputs++
			} else if expiredLoadInput(answer, performance || seq > 40) {
				expiredQueuedInputs++
			} else {
				t.Fatal(answer)
			}
			acks = append(acks, time.Since(started).Microseconds())
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		if seq == 40 && expiredQueuedInputs > 0 {
			inputCount = 41
		}
		if seq%4 == 0 {
			for _, host := range hosts {
				data := host.room()["game"].(map[string]any)["data"].(map[string]any)
				lags = append(lags, max(0, time.Now().UnixMilli()-integer(data["serverAt"])))
			}
		}
		time.Sleep(30 * time.Millisecond)
		if seq == 8 {
			time.Sleep(650 * time.Millisecond)
		}
	}
	fast.CloseNow()
	<-streamDone
	for index, host := range hosts {
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
	humansPerRoom, botsPerRoom := 10, 0
	if performance {
		humansPerRoom, botsPerRoom = 1, 9
	}
	artifact(t, map[string]any{"passed": !performance || budgetPassed, "performanceAcceptance": performance, "performanceBudgetPassed": budgetPassed, "scope": "4 authenticated ten-seat rooms, one active input stream and non-reading 1024-byte-buffer spectator; timing is acceptance only in the ordinary performance case; application bytes include platform JSON envelope but exclude WebSocket/TCP/TLS framing", "rooms": 4, "maps": maps, "wireObservedMap": maps[0], "playersPerRoom": 10, "humansPerRoom": humansPerRoom, "botsPerRoom": botsPerRoom, "authenticatedSpectators": 1, "independentSourceIPs": !performance, "inputCount": inputCount, "acceptedInputs": acceptedInputs, "expiredQueuedInputs": expiredQueuedInputs, "freshRecoveryAcknowledged": inputCount == 41, "senderPauseMS": 650, "ackMicroseconds": acks, "ruleLagMS": lags, "ackP95Microseconds": ackP95, "ruleLagP95MS": lagP95, "applicationPacketBytes": applicationBytes, "applicationP95Bytes": percentile(applicationBytes, 95), "firingApplicationPacketBytes": firingApplicationBytes, "firingApplicationP95Bytes": percentile(firingApplicationBytes, 95), "shotEvents": shotEvents})
	if performance && !budgetPassed {
		t.Fatal("parallel room performance budget exceeded", ackP95, lagP95)
	}
}
