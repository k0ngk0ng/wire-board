package server

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func baselineFixture() gameapi.Module {
	module := unchangedFixture(false)
	module.View = func(match gameapi.Match, viewer int) (any, error) {
		var position realtimePosition
		if err := json.Unmarshal(match.Data, &position); err != nil {
			return nil, err
		}
		return map[string]any{"schema": 1, "frame": position.Frames, "viewer": viewer, "source": "view"}, nil
	}
	module.SyncSnapshot = module.View
	module.StreamView = func(match gameapi.Match, viewer int, previous string) (gameapi.StreamViewResult, error) {
		var position realtimePosition
		if err := json.Unmarshal(match.Data, &position); err != nil {
			return gameapi.StreamViewResult{}, err
		}
		schema := 1
		if previous != "" {
			schema = 2
		}
		return gameapi.StreamViewResult{Data: map[string]any{"schema": schema, "frame": position.Frames, "viewer": viewer, "previous": previous, "source": "stream"}, Cursor: strconv.Itoa(position.Frames)}, nil
	}
	return module
}

func baselineEvidence(t *testing.T, name string, value any) {
	t.Helper()
	directory := filepath.Join("..", "..", ".local", "cs16", "verification", "stream-baseline")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, name+".json"), body, 0600); err != nil {
		t.Fatal(err)
	}
}

func baselineData(t *testing.T, packet map[string]any, expectedFrame int, previous string, legacy bool) {
	t.Helper()
	data := packet["game"].(map[string]any)["data"].(map[string]any)
	schema := 1
	if previous != "" {
		schema = 2
	}
	if unchangedInteger(t, data["frame"]) != expectedFrame || unchangedInteger(t, data["schema"]) != schema {
		t.Fatal("full or delta projection has the wrong frame or schema", data, expectedFrame, schema)
	}
	if legacy {
		if data["source"] != "view" {
			t.Fatal("legacy module stopped using its ordinary view", data)
		}
		return
	}
	if data["previous"] != previous || data["source"] != "stream" {
		t.Fatal("stream baseline did not match the last successfully sent projection", data, previous)
	}
}

func baselineInput(t *testing.T, fixture *realtimeTable, conn *websocket.Conn, packets <-chan unchangedPacket, stats *unchangedStats, nonce string, frame int) map[string]any {
	t.Helper()
	streamSend(t, conn, map[string]any{"type": "input", "roomId": fixture.id, "version": current(fixture.a)["version"], "nonce": nonce, "action": map[string]any{"drive": 1, "advance": true}})
	_, game := unchangedAckAndFrame(t, packets, stats, nonce, frame)
	return game
}

func TestRealtimeStreamBaselineResetsAfterFullState(t *testing.T) {
	for _, mode := range []string{"changed", "packet-nil", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			module := baselineFixture()
			legacy := mode == "legacy"
			if legacy {
				module.StreamView = nil
				module.RealtimeDeadlineFrames = false
			}
			fixture := newUnchangedTable(t, module)
			conn := streamDial(t, fixture.a, true)
			packets, stats := unchangedReader(t, conn), &unchangedStats{}
			unchangedKind(t, packets, stats, "state")
			baselineData(t, baselineInput(t, fixture, conn, packets, stats, "baseline-first", 1), 1, "", legacy)
			previous := "1"
			if legacy {
				previous = ""
			}
			baselineData(t, baselineInput(t, fixture, conn, packets, stats, "baseline-incremental", 2), 2, previous, legacy)
			if mode == "packet-nil" {
				fixture.s.mu.Lock()
				fixture.s.rooms[fixture.id].Status = "waiting"
				for _, frames := range fixture.s.streamFrames {
					frames <- streamFrameNotice{immediate: true}
				}
				fixture.s.mu.Unlock()
				unchangedKind(t, packets, stats, "state")
				fixture.s.mu.Lock()
				fixture.s.rooms[fixture.id].Status = "playing"
				fixture.s.mu.Unlock()
			} else {
				for _, enabled := range []bool{true, false} {
					fixture.a.post("/api/rooms/"+fixture.id, map[string]any{"type": "autoplay", "version": current(fixture.a)["version"], "nonce": "baseline-autoplay-" + strconv.FormatBool(enabled), "enabled": enabled}, 200)
					packet := unchangedKind(t, packets, stats, "state")
					room := packet["state"].(map[string]any)["room"].(map[string]any)
					automated := room["seats"].([]any)[0].(map[string]any)["autoPlay"] == true
					if automated != enabled {
						t.Fatal("full state lost the current control permissions", room)
					}
				}
			}
			baselineData(t, baselineInput(t, fixture, conn, packets, stats, "baseline-after-state", 3), 3, "", legacy)
			previous = "3"
			if legacy {
				previous = ""
			}
			baselineData(t, baselineInput(t, fixture, conn, packets, stats, "baseline-after-full", 4), 4, previous, legacy)
			baselineEvidence(t, mode, map[string]any{"passed": true, "mode": mode, "initialFull": true, "normalDelta": !legacy, "httpPreservedWSBaseline": true, "stateResetsCursor": true, "nextGameFull": true, "followingGameDelta": !legacy, "legacyCompatible": legacy})
		})
	}
}

func TestRealtimeStreamBaselineFailedStateDoesNotContaminatePeers(t *testing.T) {
	module := baselineFixture()
	var largeState atomic.Bool
	var spectatorProjections atomic.Int64
	prepared := make(chan struct{}, 1)
	view, projection := module.View, module.StreamView
	module.View = func(match gameapi.Match, viewer int) (any, error) {
		data, err := view(match, viewer)
		if err == nil && viewer < 0 && largeState.Load() {
			data.(map[string]any)["padding"] = strings.Repeat("b", 8<<20)
			select {
			case prepared <- struct{}{}:
			default:
			}
		}
		return data, err
	}
	module.StreamView = func(match gameapi.Match, viewer int, previous string) (gameapi.StreamViewResult, error) {
		if viewer < 0 {
			spectatorProjections.Add(1)
		}
		return projection(match, viewer, previous)
	}
	fixture := newUnchangedTable(t, module)
	spectator := newClient(t, fixture.ts.URL)
	spectator.register("完整状态写入失败观众")
	spectator.post("/api/rooms/"+fixture.id+"/watch", map[string]any{}, 200)
	spectatorID := spectator.state()["user"].(map[string]any)["id"].(string)
	fixture.ts.Close()
	fixture.ts = httptest.NewUnstartedServer(fixture.s.Handler())
	fixture.ts.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		if state == http.StateNew {
			if tcp, ok := conn.(*net.TCPConn); ok {
				_ = tcp.SetWriteBuffer(1024)
			}
		}
	}
	fixture.ts.Start()
	fixture.a.base, spectator.base = fixture.ts.URL, fixture.ts.URL
	fast, slow := streamDial(t, fixture.a, true), streamDial(t, spectator, true)
	packets, stats := unchangedReader(t, fast), &unchangedStats{}
	unchangedKind(t, packets, stats, "state")
	streamUntil(t, slow, "state")
	baselineData(t, baselineInput(t, fixture, fast, packets, stats, "baseline-peers-first", 1), 1, "", false)
	if streamUntil(t, slow, "game")["game"].(map[string]any)["data"].(map[string]any)["previous"] != "" {
		t.Fatal("initial spectator cursor was not independent")
	}
	before := spectatorProjections.Load()
	largeState.Store(true)
	fixture.s.mu.Lock()
	for changed, userID := range fixture.s.connections {
		if userID == spectatorID {
			changed <- struct{}{}
		}
	}
	fixture.s.mu.Unlock()
	select {
	case <-prepared:
	case <-time.After(3 * time.Second):
		t.Fatal("slow spectator did not start its full state write")
	}
	began := time.Now()
	baselineData(t, baselineInput(t, fixture, fast, packets, stats, "baseline-fast-during-state", 2), 2, "1", false)
	if time.Since(began) > 500*time.Millisecond {
		t.Fatal("slow full-state write blocked the fast stream")
	}
	slow.CloseNow()
	detached := false
	for until := time.Now().Add(3 * time.Second); time.Now().Before(until); {
		fixture.s.mu.Lock()
		detached = true
		for _, userID := range fixture.s.connections {
			if userID == spectatorID {
				detached = false
			}
		}
		fixture.s.mu.Unlock()
		if detached {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !detached || spectatorProjections.Load() != before {
		t.Fatal("failed full-state connection continued projecting pending frames", detached, before, spectatorProjections.Load())
	}
	largeState.Store(false)
	fresh := streamDial(t, spectator, true)
	freshPackets, freshStats := unchangedReader(t, fresh), &unchangedStats{}
	unchangedKind(t, freshPackets, freshStats, "state")
	baselineData(t, baselineInput(t, fixture, fast, packets, stats, "baseline-fast-after-failure", 3), 3, "2", false)
	baselineData(t, unchangedKind(t, freshPackets, freshStats, "game"), 3, "", false)
	baselineEvidence(t, "failed-state", map[string]any{"passed": true, "blockedStateBytes": 8 << 20, "failedConnectionDetached": true, "noProjectionAfterFailure": true, "fastCursorPreserved": true, "reconnectedCursorEmpty": true})
}
