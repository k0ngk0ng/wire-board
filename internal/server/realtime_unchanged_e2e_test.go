package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type unchangedPacket struct {
	value map[string]any
	bytes int
	err   error
}

type unchangedStats struct {
	Games int `json:"games"`
	State int `json:"state"`
	Acks  int `json:"acks"`
	Bytes int `json:"bytes"`
}

func unchangedFixture(enabled bool) gameapi.Module {
	module := realtimeFixture(true, time.Hour, 0, "")
	module.RealtimeDeadlineFrames = true
	module.SyncSnapshot = module.View
	apply := module.Apply
	module.Apply = func(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		next, err := apply(ctx, match, raw)
		if err != nil {
			return next, err
		}
		var action struct {
			Advance bool `json:"advance"`
			Epoch   bool `json:"epoch"`
		}
		if err := json.Unmarshal(raw, &action); err != nil {
			return next, err
		}
		if action.Advance {
			var position realtimePosition
			if err := json.Unmarshal(next.Data, &position); err != nil {
				return next, err
			}
			position.Frames++
			next.Data, err = json.Marshal(position)
			next.SystemDeadline += 10
		}
		if action.Epoch {
			next.Schema++
		}
		return next, err
	}
	module.StreamView = func(match gameapi.Match, viewer int, cursor string) (gameapi.StreamViewResult, error) {
		var position realtimePosition
		if err := json.Unmarshal(match.Data, &position); err != nil {
			return gameapi.StreamViewResult{}, err
		}
		token := strconv.Itoa(match.Schema) + ":" + strconv.Itoa(position.Frames)
		if enabled && cursor == token {
			return gameapi.StreamViewResult{Unchanged: true, Cursor: "not-the-sent-cursor"}, nil
		}
		data := map[string]any{"frame": position.Frames, "epoch": match.Schema, "viewer": viewer, "previous": cursor, "padding": strings.Repeat("u", 4096)}
		return gameapi.StreamViewResult{Data: data, Cursor: token}, nil
	}
	return module
}

func newUnchangedTable(t *testing.T, module gameapi.Module) *realtimeTable {
	t.Helper()
	root := filepath.Join("..", "..", ".local", "cs16", "tmp")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(root, "stream-unchanged-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	fixture := &realtimeTable{cfg: Config{DataDir: directory, InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{module}}}
	fixture.s, err = New(fixture.cfg, fstest.MapFS{"index.html": {Data: []byte("unchanged realtime E2E")}})
	if err != nil {
		t.Fatal(err)
	}
	fixture.s.cancel()
	<-fixture.s.done
	fixture.ts = httptest.NewServer(fixture.s.Handler())
	t.Cleanup(func() { fixture.ts.Close(); fixture.s.Close() })
	publishModuleForTest(t, fixture.ts.URL, module.ID)
	fixture.a = newClient(t, fixture.ts.URL)
	fixture.a.register("未变化投影验收玩家")
	room := fixture.a.post("/api/rooms", map[string]any{"kind": module.ID, "name": "未变化投影验收", "capacity": 4}, 201)
	fixture.id = room["id"].(string)
	for range 3 {
		fixture.a.command(current(fixture.a), "add_bot", nil, 200)
	}
	fixture.a.command(current(fixture.a), "ready", nil, 200)
	fixture.a.command(current(fixture.a), "start", nil, 200)
	return fixture
}

func unchangedReader(t *testing.T, conn *websocket.Conn) <-chan unchangedPacket {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	packets := make(chan unchangedPacket, 256)
	go func() {
		defer close(packets)
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				if ctx.Err() == nil {
					select {
					case packets <- unchangedPacket{err: err}:
					case <-ctx.Done():
					}
				}
				return
			}
			var value map[string]any
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			err = decoder.Decode(&value)
			select {
			case packets <- unchangedPacket{value: value, bytes: len(raw), err: err}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return packets
}

func unchangedTake(t *testing.T, packets <-chan unchangedPacket, stats *unchangedStats) map[string]any {
	t.Helper()
	select {
	case packet, open := <-packets:
		if !open || packet.err != nil {
			t.Fatal("stream closed", packet.err)
		}
		stats.Bytes += packet.bytes
		switch packet.value["type"] {
		case "game":
			stats.Games++
		case "state":
			stats.State++
		case "ack":
			stats.Acks++
		}
		return packet.value
	case <-time.After(3 * time.Second):
		t.Fatal("stream response deadline exceeded")
		return nil
	}
}

func unchangedKind(t *testing.T, packets <-chan unchangedPacket, stats *unchangedStats, kind string) map[string]any {
	t.Helper()
	for range 100 {
		packet := unchangedTake(t, packets, stats)
		if packet["type"] == kind {
			return packet
		}
	}
	t.Fatal("missing stream kind", kind)
	return nil
}

func unchangedInteger(t *testing.T, value any) int {
	t.Helper()
	number, ok := value.(json.Number)
	if !ok {
		t.Fatal("expected JSON integer", value)
	}
	result, err := strconv.Atoi(number.String())
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func unchangedAckAndFrame(t *testing.T, packets <-chan unchangedPacket, stats *unchangedStats, nonce string, frame int) (map[string]any, map[string]any) {
	t.Helper()
	var ack, game map[string]any
	for ack == nil || game == nil {
		packet := unchangedTake(t, packets, stats)
		if packet["type"] == "ack" && packet["nonce"] == nonce {
			ack = packet
		}
		if packet["type"] == "game" {
			data := packet["game"].(map[string]any)["data"].(map[string]any)
			if unchangedInteger(t, data["frame"]) == frame {
				game = packet
			}
		}
	}
	if unchangedInteger(t, ack["status"]) != 200 {
		t.Fatal("input rejected", ack)
	}
	return ack, game
}

func unchangedEvidence(t *testing.T, name string, data any) {
	t.Helper()
	directory := filepath.Join("..", "..", ".local", "cs16", "verification", "stream-unchanged")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, name+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRealtimeUnchangedSkipsOnlyGamePacketsAndKeepsAcks(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(strconv.FormatBool(enabled), func(t *testing.T) {
			fixture := newUnchangedTable(t, unchangedFixture(enabled))
			conn := streamDial(t, fixture.a, true)
			packets := unchangedReader(t, conn)
			stats := &unchangedStats{}
			unchangedKind(t, packets, stats, "state")
			version := current(fixture.a)["version"]
			streamSend(t, conn, map[string]any{"type": "input", "roomId": fixture.id, "version": version, "nonce": "unchanged-bootstrap", "action": map[string]any{"drive": 1}})
			ack, _ := unchangedAckAndFrame(t, packets, stats, "unchanged-bootstrap", 0)
			version = ack["version"]
			baseline := *stats
			maximum := time.Duration(0)
			for index := range 40 {
				nonce := fmt.Sprintf("unchanged-duplicate-%d", index)
				start := time.Now()
				streamSend(t, conn, map[string]any{"type": "input", "roomId": fixture.id, "version": version, "nonce": nonce, "action": map[string]any{"drive": 1}})
				ack := unchangedKind(t, packets, stats, "ack")
				if ack["nonce"] != nonce || unchangedInteger(t, ack["status"]) != 200 {
					t.Fatal("duplicate input not acknowledged", ack)
				}
				maximum = max(maximum, time.Since(start))
				version = ack["version"]
			}
			streamSend(t, conn, map[string]any{"type": "input", "roomId": fixture.id, "version": version, "nonce": "unchanged-advance", "action": map[string]any{"drive": 1, "advance": true}})
			ack, game := unchangedAckAndFrame(t, packets, stats, "unchanged-advance", 1)
			if game["game"].(map[string]any)["data"].(map[string]any)["previous"] == "" {
				t.Fatal("new-frame deadline discarded the valid incremental cursor")
			}
			duplicates := stats.Games - baseline.Games - 1
			if enabled && (duplicates != 0 || stats.State != baseline.State) {
				t.Fatal("unchanged game produced duplicate game or fallback state packets", *stats, baseline)
			}
			if !enabled && duplicates == 0 {
				t.Fatal("ordinary modules lost their original push behavior")
			}
			if maximum > 500*time.Millisecond || stats.Acks-baseline.Acks != 41 {
				t.Fatal("skipping delayed or lost input acknowledgements", maximum, *stats)
			}
			streamSend(t, conn, map[string]any{"type": "input", "roomId": fixture.id, "version": ack["version"], "nonce": "unchanged-epoch", "action": map[string]any{"drive": 1, "epoch": true}})
			ack, game = unchangedAckAndFrame(t, packets, stats, "unchanged-epoch", 1)
			if unchangedInteger(t, game["game"].(map[string]any)["data"].(map[string]any)["epoch"]) != 2 {
				t.Fatal("epoch recovery was skipped", game)
			}
			fresh := streamDial(t, fixture.a, true)
			freshPackets := unchangedReader(t, fresh)
			freshStats := &unchangedStats{}
			unchangedKind(t, freshPackets, freshStats, "state")
			streamSend(t, fresh, map[string]any{"type": "input", "roomId": fixture.id, "version": ack["version"], "nonce": "unchanged-reconnect", "action": map[string]any{"drive": 1}})
			_, game = unchangedAckAndFrame(t, freshPackets, freshStats, "unchanged-reconnect", 1)
			if game["game"].(map[string]any)["data"].(map[string]any)["previous"] != "" {
				t.Fatal("reconnected socket reused another connection cursor")
			}
			unchangedEvidence(t, "input-"+strconv.FormatBool(enabled), map[string]any{"passed": true, "optedIn": enabled, "duplicateInputs": 40, "duplicateGamePackets": duplicates, "maximumAckMS": maximum.Milliseconds(), "stats": stats, "reconnect": true, "epochRecovery": true})
		})
	}
}

func TestRealtimeUnchangedPreservesPublicEnvelopeChanges(t *testing.T) {
	fixture := newUnchangedTable(t, unchangedFixture(true))
	conn := streamDial(t, fixture.a, true)
	packets := unchangedReader(t, conn)
	stats := &unchangedStats{}
	unchangedKind(t, packets, stats, "state")
	streamSend(t, conn, map[string]any{"type": "input", "roomId": fixture.id, "version": current(fixture.a)["version"], "nonce": "metadata-bootstrap", "action": map[string]any{"drive": 1}})
	unchangedAckAndFrame(t, packets, stats, "metadata-bootstrap", 0)
	changes := []struct {
		name   string
		mutate func(*Room)
	}{
		{"setup", func(room *Room) { room.SetupVersion++ }},
		{"clock-offset", func(room *Room) { room.ClockOffsetMS += 7 }},
		{"turn-deadline", func(room *Room) { room.TurnDeadline += 11 }},
		{"actors", func(room *Room) { room.Module.Actors = []int{0}; room.Module.Attention = nil }},
		{"attention", func(room *Room) { room.Module.Attention = []int{} }},
		{"eliminated", func(room *Room) { room.Module.Eliminated = []int{3} }},
		{"restartable", func(room *Room) { room.Module.Restartable = true }},
		{"paused", func(room *Room) { room.Module.Paused = true; room.Module.SystemDeadline = 0 }},
		{"loading", func(room *Room) {
			room.Loading = &ClientLoadBarrier{Token: "gate", Build: "build", Ready: []bool{false, true, true, true}}
		}},
		{"loading-ready", func(room *Room) { room.Loading.Ready[0] = true }},
		{"loading-token", func(room *Room) { room.Loading.Token = "next-gate" }},
		{"loading-released", func(room *Room) { room.Loading = nil; room.ClockOffsetMS += 13 }},
		{"log", func(room *Room) { room.Game.Log = append(room.Game.Log, "public notice") }},
		{"turn", func(room *Room) { room.Game.Turn++ }},
		{"finished", func(room *Room) {
			room.Module.Result = &gameapi.Result{Ranks: []int{1, 2, 3, 4}, Winners: []int{0}}
			room.Game.Winners = []int{0}
		}},
	}
	proof := []string{}
	for _, change := range changes {
		fixture.s.mu.Lock()
		room := fixture.s.rooms[fixture.id]
		change.mutate(room)
		room.Version++
		fixture.s.broadcastGameChange(room)
		fixture.s.mu.Unlock()
		packet := unchangedKind(t, packets, stats, "game")
		data := packet["game"].(map[string]any)["data"].(map[string]any)
		if data["previous"] != "" || unchangedInteger(t, data["frame"]) != 0 {
			t.Fatal("public envelope change did not force a complete same-frame projection", change.name, data)
		}
		proof = append(proof, change.name)
	}
	fixture.s.mu.Lock()
	fixture.s.rooms[fixture.id].Status = "closed"
	fixture.s.broadcast()
	fixture.s.mu.Unlock()
	unchangedKind(t, packets, stats, "state")
	unchangedEvidence(t, "metadata", map[string]any{"passed": true, "changes": proof, "statusNotification": true, "stats": stats})
}

func TestRealtimeUnchangedCannotHideInitialProjection(t *testing.T) {
	module := unchangedFixture(true)
	module.StreamView = func(gameapi.Match, int, string) (gameapi.StreamViewResult, error) {
		return gameapi.StreamViewResult{Unchanged: true, Cursor: "invalid-initial"}, nil
	}
	fixture := newUnchangedTable(t, module)
	conn := streamDial(t, fixture.a, true)
	packets := unchangedReader(t, conn)
	stats := &unchangedStats{}
	unchangedKind(t, packets, stats, "state")
	streamSend(t, conn, map[string]any{"type": "input", "roomId": fixture.id, "version": current(fixture.a)["version"], "nonce": "invalid-initial", "action": map[string]any{"drive": 1}})
	packet := unchangedKind(t, packets, stats, "game")
	data, ok := packet["game"].(map[string]any)["data"].(map[string]any)
	if !ok || data["inputs"].([]any)[0] != json.Number("1") {
		t.Fatal("initial Unchanged suppressed or nulled the authenticated full projection", packet)
	}
	unchangedEvidence(t, "initial", map[string]any{"passed": true, "fallbackToAuthenticatedView": true, "stats": stats})
}

func TestRealtimeUnchangedPreservesSpectatorPermissionsAndRoomState(t *testing.T) {
	fixture := newUnchangedTable(t, unchangedFixture(true))
	spectator := newClient(t, fixture.ts.URL)
	spectator.register("未变化流权限观众")
	spectator.post("/api/rooms/"+fixture.id+"/watch", map[string]any{}, 200)
	playerSocket := streamDial(t, fixture.a, true)
	spectatorSocket := streamDial(t, spectator, true)
	playerPackets := unchangedReader(t, playerSocket)
	spectatorPackets := unchangedReader(t, spectatorSocket)
	playerStats, spectatorStats := &unchangedStats{}, &unchangedStats{}
	unchangedKind(t, playerPackets, playerStats, "state")
	unchangedKind(t, spectatorPackets, spectatorStats, "state")
	streamSend(t, playerSocket, map[string]any{"type": "input", "roomId": fixture.id, "version": current(fixture.a)["version"], "nonce": "permission-bootstrap", "action": map[string]any{"drive": 1}})
	ack, _ := unchangedAckAndFrame(t, playerPackets, playerStats, "permission-bootstrap", 0)
	watch := unchangedKind(t, spectatorPackets, spectatorStats, "game")
	data := watch["game"].(map[string]any)["data"].(map[string]any)
	if unchangedInteger(t, data["viewer"]) != -1 || data["previous"] != "" {
		t.Fatal("spectator reused another viewer cursor", data)
	}
	streamSend(t, spectatorSocket, map[string]any{"type": "input", "roomId": fixture.id, "version": ack["version"], "nonce": "spectator-forbidden", "action": map[string]any{"drive": 1}})
	denied := unchangedKind(t, spectatorPackets, spectatorStats, "ack")
	if unchangedInteger(t, denied["status"]) == 200 {
		t.Fatal("spectator acquired input permission", denied)
	}
	fixture.a.post("/api/rooms/"+fixture.id, map[string]any{"type": "autoplay", "version": current(fixture.a)["version"], "nonce": "unchanged-autoplay", "enabled": true}, 200)
	state := unchangedKind(t, playerPackets, playerStats, "state")
	room := state["state"].(map[string]any)["room"].(map[string]any)
	if room["seats"].([]any)[0].(map[string]any)["autoPlay"] != true {
		t.Fatal("ordinary room permission notification was dropped", room)
	}
	streamSend(t, playerSocket, map[string]any{"type": "input", "roomId": fixture.id, "version": room["version"], "nonce": "autoplay-no-manual", "action": map[string]any{"drive": 1}})
	denied = unchangedKind(t, playerPackets, playerStats, "ack")
	if unchangedInteger(t, denied["status"]) == 200 {
		t.Fatal("autoplay retained manual input permission", denied)
	}
	unchangedEvidence(t, "permissions", map[string]any{"passed": true, "spectatorRejected": true, "viewerScoped": true, "autoplayStateReceived": true, "manualInputRejected": true})
}
