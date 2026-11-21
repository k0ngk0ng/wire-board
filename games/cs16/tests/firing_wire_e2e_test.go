package cs16_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type firingWireTrace struct {
	Seat                 int     `json:"seat"`
	Weapon               string  `json:"weapon"`
	Sent                 int64   `json:"sent"`
	Acknowledged         int64   `json:"acknowledged"`
	OwnShots             int     `json:"ownShots"`
	NormalDeltaBytes     []int64 `json:"normalDeltaApplicationBytes"`
	FireDeltaBytes       []int64 `json:"fireDeltaApplicationBytes"`
	FullGameBytes        []int64 `json:"fullGameApplicationBytes"`
	RecoveryBytes        []int64 `json:"recoveryApplicationBytes"`
	RoundTransitionBytes []int64 `json:"roundTransitionApplicationBytes"`
	StateBytes           []int64 `json:"stateApplicationBytes"`
	Error                string  `json:"error,omitempty"`
}

type firingWirePeer struct {
	inputPeer
	weapon       string
	latestFrame  atomic.Int64
	sent         atomic.Int64
	acknowledged atomic.Int64
}

func collectFiringWire(ctx context.Context, peer *firingWirePeer, initialEventID int64, reports chan<- firingWireTrace, ready chan<- struct{}) {
	trace := firingWireTrace{Seat: peer.seat, Weapon: peer.weapon}
	defer func() {
		trace.Sent, trace.Acknowledged = peer.sent.Load(), peer.acknowledged.Load()
		reports <- trace
	}()
	lastEventID, round := initialEventID, int64(0)
	prepared := false
	for {
		_, raw, err := peer.connection.Read(ctx)
		if err != nil {
			if ctx.Err() == nil {
				trace.Error = err.Error()
			}
			return
		}
		var packet map[string]any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if decoder.Decode(&packet) != nil {
			trace.Error = "WebSocket packet is not JSON"
			return
		}
		if packet["type"] == "ack" {
			if integer(packet["status"]) != 200 {
				trace.Error = string(raw)
				return
			}
			peer.acknowledged.Add(1)
			continue
		}
		if packet["type"] == "state" {
			trace.StateBytes = append(trace.StateBytes, int64(len(raw)))
			continue
		}
		if packet["type"] != "game" {
			continue
		}
		data := packet["game"].(map[string]any)["data"].(map[string]any)
		peer.latestFrame.Store(max(peer.latestFrame.Load(), integer(data["frame"])))
		shots := 0
		for _, event := range data["events"].([]any) {
			value := event.(map[string]any)
			if integer(value["id"]) > lastEventID && value["type"] == "shot" {
				shots++
				if integer(value["seat"]) == int64(peer.seat) {
					trace.OwnShots++
				}
			}
		}
		lastEventID = max(lastEventID, integer(data["eventId"]))
		currentRound := round
		if value, exists := data["round"]; exists {
			currentRound = integer(value)
		} else if global, ok := data["global"].(map[string]any); ok {
			if values, ok := global["values"].(map[string]any); ok {
				if value, exists := values["round"]; exists {
					currentRound = integer(value)
				}
			}
		}
		size := int64(len(raw))
		switch {
		case data["recover"] == true:
			trace.RecoveryBytes = append(trace.RecoveryBytes, size)
		case round != 0 && currentRound != round:
			trace.RoundTransitionBytes = append(trace.RoundTransitionBytes, size)
		case integer(data["schema"]) == 1:
			trace.FullGameBytes = append(trace.FullGameBytes, size)
		default:
			trace.NormalDeltaBytes = append(trace.NormalDeltaBytes, size)
			if shots > 0 {
				trace.FireDeltaBytes = append(trace.FireDeltaBytes, size)
			}
		}
		round = currentRound
		if !prepared {
			prepared = true
			ready <- struct{}{}
		}
	}
}

func TestSixMapFiringApplicationPackets(t *testing.T) {
	for _, mapID := range []string{"de_dust2", "de_inferno", "de_aztec", "cs_assault", "fy_iceworld", "cs_bloodstrike"} {
		t.Run(mapID, func(t *testing.T) {
			harness := newHTTPRoom(t, 10)
			host := harness.people[0]
			created := host.request("POST", "/api/rooms", map[string]any{"kind": "cs16", "name": "十真人密集枪击", "capacity": 10, "options": map[string]any{"map": mapID, "roundSeconds": 60, "maxRounds": 2}}, 201)
			id := created["id"].(string)
			for seat := 1; seat < 10; seat++ {
				person := harness.people[seat]
				person.request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": host.room()["version"], "nonce": fmt.Sprintf("firing-join-%d", seat)}, 200)
			}
			for _, person := range harness.people {
				person.command("ready", nil, 200)
			}
			host.command("start", nil, 200)
			waitHTTP(t, host, func(data map[string]any) bool { return data["phase"] == "live" })
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			peers := make([]*firingWirePeer, 10)
			reports := make(chan firingWireTrace, 10)
			ready := make(chan struct{}, 10)
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
				connection.SetReadLimit(1024 * 1024)
				peer := &firingWirePeer{inputPeer: inputPeer{seat: seat, connection: connection, roomID: id, epoch: data["epoch"].(string), instance: state["serverInstance"].(string), life: integer(own["life"]), seq: integer(own["receivedSeq"]), version: room["version"]}}
				peer.weapon = "glock18"
				if own["team"] == "ct" {
					peer.weapon = "usp"
				}
				peer.latestFrame.Store(integer(data["frame"]))
				peers[seat] = peer
				go collectFiringWire(ctx, peer, integer(data["eventId"]), reports, ready)
			}
			for range peers {
				select {
				case <-ready:
				case <-ctx.Done():
					t.Fatal("dense stream did not initialize", ctx.Err())
				}
			}
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			for ordinal := 1; ordinal <= 40; ordinal++ {
				select {
				case <-ticker.C:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				for _, peer := range peers {
					if peer.sent.Load()-peer.acknowledged.Load() >= 8 {
						t.Fatal("dense stream exceeded existing eight-input window", peer.seat)
					}
					action := map[string]any{"type": "input", "epoch": peer.epoch, "seq": peer.seq + int64(ordinal), "life": peer.life, "viewFrame": peer.latestFrame.Load(), "moveX": 0, "moveY": 0, "yaw": 0, "pitch": 16000, "fire": ordinal%2 == 1}
					if ordinal == 1 {
						action["select"] = peer.weapon
					}
					message := map[string]any{"type": "input", "serverInstance": peer.instance, "roomId": id, "version": peer.version, "nonce": fmt.Sprintf("firing-%d-%d", peer.seat, ordinal), "action": action}
					peer.sent.Add(1)
					if err := peer.connection.Write(ctx, websocket.MessageText, encode(message)); err != nil {
						t.Fatal(err)
					}
				}
			}
			for seat, peer := range peers {
				waitHTTP(t, harness.people[seat], func(data map[string]any) bool {
					return integer(data["players"].([]any)[seat].(map[string]any)["appliedSeq"]) >= peer.seq+40 && peer.acknowledged.Load() == 40
				})
			}
			cancel()
			traces := make([]firingWireTrace, 10)
			fireBytes, normalBytes := []int64{}, []int64{}
			for range peers {
				trace := <-reports
				if trace.Error != "" || trace.Sent != 40 || trace.Acknowledged != 40 || trace.OwnShots < 2 || len(trace.FullGameBytes) == 0 {
					t.Fatal("dense firing stream did not execute real input edges", trace)
				}
				traces[trace.Seat] = trace
				fireBytes = append(fireBytes, trace.FireDeltaBytes...)
				normalBytes = append(normalBytes, trace.NormalDeltaBytes...)
			}
			if len(fireBytes) == 0 {
				t.Fatal("dense firing measurement contains no ordinary firing deltas")
			}
			artifact(t, map[string]any{"passed": true, "scope": "10 authenticated humans select their original spawned pistol and alternate actual downward fire press/release through WebSocket; every input acknowledged and applied; application JSON includes platform envelope, excludes WebSocket/TCP/TLS framing; report-only dense samples have no substituted ordinary budget; natural combat is verified separately", "map": mapID, "humans": 10, "inputs": 400, "fireDeltaApplicationP95Bytes": percentile(fireBytes, 95), "normalDeltaApplicationP95Bytes": percentile(normalBytes, 95), "traces": traces})
		})
	}
}
