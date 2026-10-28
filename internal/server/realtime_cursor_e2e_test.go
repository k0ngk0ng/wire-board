package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Real parallel sockets: a spectator stops consuming a 4MiB stream while the
// player continues receiving compact game frames and input acknowledgements.
// A deliberately mutated callback input must remain detached from room state.
func TestRealtimeCursorStreamIsolatesSlowConnection(t *testing.T) {
	m := realtimeFixture(true, time.Hour, 0, "")
	m.SyncSnapshot = m.View
	m.StreamView = func(match gameapi.Match, actor int, cursor string) (gameapi.StreamViewResult, error) {
		var state realtimePosition
		if err := json.Unmarshal(match.Data, &state); err != nil {
			return gameapi.StreamViewResult{}, err
		}
		match.Data[0] = '!'
		data := map[string]any{"viewer": actor, "previous": cursor, "inputs": state.Inputs[0]}
		if actor < 0 {
			data["padding"] = strings.Repeat("s", 4<<20)
		}
		return gameapi.StreamViewResult{Data: data, Cursor: strconv.Itoa(state.Inputs[0])}, nil
	}
	f := newRealtimeTable(t, m, false)
	slow := newClient(t, f.ts.URL)
	slow.register("游标慢连接")
	slow.post("/api/rooms/"+f.id+"/watch", map[string]any{}, 200)
	f.ts.Close()
	f.ts = httptest.NewUnstartedServer(f.s.Handler())
	f.ts.Config.ConnState = func(c net.Conn, state http.ConnState) {
		if state == http.StateNew {
			if tcp, ok := c.(*net.TCPConn); ok {
				_ = tcp.SetWriteBuffer(1024)
			}
		}
	}
	f.ts.Start()
	f.a.base = f.ts.URL
	slow.base = f.ts.URL
	fastSocket := streamDial(t, f.a, true)
	slowSocket := streamDial(t, slow, true)
	streamUntil(t, fastSocket, "state")
	streamUntil(t, slowSocket, "state")
	room := current(f.a)
	f.a.post("/api/rooms/"+f.id, map[string]any{"type": "action", "version": room["version"], "nonce": "cursor-first", "action": map[string]any{"drive": 1}}, 200)
	first := streamUntil(t, fastSocket, "game")
	data := first["game"].(map[string]any)["data"].(map[string]any)
	if data["previous"] != "" || data["viewer"] != float64(0) {
		t.Fatal("cursor callback not used or viewer mixed", data)
	}
	version := first["version"]
	maximum := time.Duration(0)
	for i := 0; i < 12; i++ {
		start := time.Now()
		streamSend(t, fastSocket, map[string]any{"type": "input", "roomId": f.id, "version": version, "nonce": fmt.Sprintf("cursor-fast-%d", i), "action": map[string]any{"drive": 1}})
		ack := streamUntil(t, fastSocket, "ack")
		elapsed := time.Since(start)
		if elapsed > maximum {
			maximum = elapsed
		}
		if ack["status"] != float64(200) {
			t.Fatal("fast input rejected", ack)
		}
		version = ack["version"]
	}
	if maximum > 500*time.Millisecond {
		t.Fatal("slow stream delayed fast acknowledgements", maximum)
	}
	next := streamUntil(t, fastSocket, "game")
	data = next["game"].(map[string]any)["data"].(map[string]any)
	if data["previous"] == "" || data["inputs"] != float64(13) {
		t.Fatal("fast connection cursor lost", data)
	}
	slowSocket.CloseNow()
	if got := realtimeData(t, current(f.a)); got.Inputs[0] != 13 {
		t.Fatal("stream callback changed stored match", got)
	}
	streamEvidence(t, "cursor-slow-connection", map[string]any{"passed": true, "blockedPayloadBytes": 4 << 20, "fastActions": 12, "maximumAckMS": float64(maximum.Microseconds()) / 1000, "lastFastPacket": next})
}
