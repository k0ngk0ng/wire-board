package server

// Failure inventory precedes implementation in docs/research/realtime-efficiency/PLAN.md.
// These use authenticated network sockets, public HTTP commands, real SQLite and
// the production clock. The fixture contains only host-contract counters.
import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func streamDial(t *testing.T, c *testClient, stream bool) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(c.base, "http") + "/api/ws"
	if stream {
		url += "?stream=1"
	}
	conn, _, err := websocket.Dial(context.Background(), url, &websocket.DialOptions{HTTPClient: c.client})
	if err != nil {
		t.Fatal(err)
	}
	conn.SetReadLimit(2 << 20)
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}
func streamRead(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, raw, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err = json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func streamUntil(t *testing.T, c *websocket.Conn, kind string) map[string]any {
	t.Helper()
	for i := 0; i < 100; i++ {
		m := streamRead(t, c)
		if m["type"] == kind {
			return m
		}
	}
	t.Fatalf("no %s packet", kind)
	return nil
}
func streamSend(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	raw, _ := json.Marshal(v)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatal(err)
	}
}
func streamEvidence(t *testing.T, name string, evidence any) {
	t.Helper()
	dir := filepath.Join("..", "..", ".local", "realtime-efficiency")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(evidence, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, name+".json"), raw, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRealtimeStreamBootstrapAndScopedPush(t *testing.T) {
	f := newRealtimeTable(t, realtimeFixture(true, 100*time.Millisecond, 0, ""), true)
	conn := streamDial(t, f.a, true)
	first := streamRead(t, conn)
	if first["type"] != "state" {
		t.Fatalf("stream bootstrap must contain state, got %v", first)
	}
	state := first["state"].(map[string]any)
	if state["room"].(map[string]any)["id"] != f.id {
		t.Fatal("wrong bootstrap room")
	}
	game := streamUntil(t, conn, "game")
	if game["roomId"] != f.id || game["game"] == nil || game["version"] == nil || game["serverNow"] == nil {
		t.Fatal("incomplete game packet", game)
	}
	for _, key := range []string{"rooms", "availableGames", "modules", "railMaps", "chat", "user", "state"} {
		if _, ok := game[key]; ok {
			t.Fatalf("high frequency packet includes %s", key)
		}
	}
	if game["turnDeadline"] != float64(0) {
		t.Fatal("realtime player clock should be absent", game)
	}
	streamEvidence(t, "stream-scoped-push", map[string]any{"passed": true, "bootstrap": first, "frame": game})
}

func TestRealtimeStreamInputAcknowledgedExactlyOnce(t *testing.T) {
	f := newRealtimeTable(t, realtimeFixture(true, time.Hour, 0, ""), false)
	c := streamDial(t, f.a, true)
	if first := streamRead(t, c); first["type"] != "state" {
		t.Fatal("missing state bootstrap", first)
	}
	room := current(f.a)
	req := map[string]any{"type": "input", "roomId": f.id, "version": room["version"], "nonce": "stream-exactly-once", "action": map[string]any{"drive": 1}}
	streamSend(t, c, req)
	ack := streamUntil(t, c, "ack")
	if ack["nonce"] != req["nonce"] || ack["status"] != float64(200) {
		t.Fatal("not acknowledged", ack)
	}
	streamSend(t, c, req)
	duplicate := streamUntil(t, c, "ack")
	if duplicate["version"] != ack["version"] || duplicate["status"] != float64(200) {
		t.Fatal("retry was not idempotent", duplicate)
	}
	got := realtimeData(t, current(f.a))
	if got.Inputs[0] != 1 {
		t.Fatal("duplicate committed", got)
	}
	// Reopen the real database and verify the acknowledged input survives.
	c.CloseNow()
	f.ts.Close()
	f.s.Close()
	var err error
	f.s, err = New(f.cfg, f.s.files)
	if err != nil {
		t.Fatal(err)
	}
	f.s.cancel()
	<-f.s.done
	if got := realtimeDataFromStored(t, f.s, f.id); got.Inputs[0] != 1 {
		t.Fatal("acknowledged input lost on restart", got)
	}
	streamEvidence(t, "stream-input", map[string]any{"passed": true, "ack": ack, "retry": duplicate, "data": got})
}
func realtimeDataFromStored(t *testing.T, s *Server, id string) realtimePosition {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	var p realtimePosition
	if err := json.Unmarshal(s.rooms[id].Module.Data, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRealtimeStreamRejectsSpectatorAndStaleRoom(t *testing.T) {
	f := newRealtimeTable(t, realtimeFixture(true, time.Hour, 0, ""), false)
	spectator := newClient(t, f.ts.URL)
	spectator.register("实时流观众")
	spectator.post("/api/rooms/"+f.id+"/watch", map[string]any{}, 200)
	conn := streamDial(t, spectator, true)
	if m := streamRead(t, conn); m["type"] != "state" {
		t.Fatal(m)
	}
	streamSend(t, conn, map[string]any{"type": "input", "roomId": f.id, "version": current(f.a)["version"], "nonce": "spectator-no-control", "action": map[string]any{"drive": 1}})
	ack := streamUntil(t, conn, "ack")
	if ack["status"] == float64(200) {
		t.Fatal("spectator input accepted", ack)
	}
	if realtimeData(t, current(f.a)).Inputs[0] != 0 {
		t.Fatal("spectator mutated game")
	}
	streamSend(t, conn, map[string]any{"type": "input", "roomId": "missing", "version": 1, "nonce": "missing-room-control", "action": map[string]any{"drive": 1}})
	ack = streamUntil(t, conn, "ack")
	if ack["status"] != float64(404) {
		t.Fatal("missing room not rejected", ack)
	}
}

func TestRealtimeDoesNotTimeoutToAutoplay(t *testing.T) {
	f := newRealtimeTable(t, realtimeFixture(true, 100*time.Millisecond, 0, ""), true)
	f.a.command(current(f.a), "action", map[string]any{"drive": 1, "timeout": true}, 200)
	time.Sleep(250 * time.Millisecond)
	room := current(f.a)
	seat := room["seats"].([]any)[0].(map[string]any)
	if seat["autoPlay"] == true || seat["timeoutAutoPlay"] == true || room["turnDeadline"] != float64(0) {
		t.Fatal("idle deadline still changes realtime ownership", seat, room["turnDeadline"])
	}
	setAutoPlay(f.a, room, true, 200)
	time.Sleep(150 * time.Millisecond)
	if current(f.a)["seats"].([]any)[0].(map[string]any)["autoPlay"] != true {
		t.Fatal("manual autoplay disabled")
	}
	setAutoPlay(f.a, current(f.a), false, 200)
	if current(f.a)["seats"].([]any)[0].(map[string]any)["autoPlay"] == true {
		t.Fatal("manual reclaim failed")
	}
	streamEvidence(t, "no-realtime-timeout", map[string]any{"passed": true, "room": current(f.a)})
}

// Baseline is opt-in and contains no pass threshold invented from a target.
// Compare the same fixture, interval, client and duration before/after changes.
func TestRealtimeTransportBaseline(t *testing.T) {
	if os.Getenv("WIRE_BOARD_REALTIME_BASELINE") == "" {
		t.Skip("opt-in measurement")
	}
	f := newRealtimeTable(t, realtimeFixture(true, 100*time.Millisecond, 0, ""), true)
	stream := os.Getenv("WIRE_BOARD_REALTIME_BASELINE") == "stream"
	c := streamDial(t, f.a, stream)
	snapshots, gets, wireBytes := 0, 0, 0
	start := time.Now()
	for time.Since(start) < 2*time.Second {
		m := streamRead(t, c)
		b, _ := json.Marshal(m)
		wireBytes += len(b)
		if stream {
			if m["type"] == "game" {
				snapshots++
			}
		} else {
			r := f.a.state()
			b, _ = json.Marshal(r)
			wireBytes += len(b)
			gets++
			snapshots++
		}
	}
	var actions int
	if err := f.s.db.QueryRow("SELECT COUNT(*) FROM actions WHERE room_id=?", f.id).Scan(&actions); err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("transport-%s", os.Getenv("WIRE_BOARD_REALTIME_BASELINE"))
	streamEvidence(t, name, map[string]any{"elapsedMS": time.Since(start).Milliseconds(), "snapshots": snapshots, "stateGETs": gets, "jsonBytes": wireBytes, "storedActions": actions, "scope": "one room, one viewer, three controlled seats, 100ms frame; compact JSON application bytes (not wire/TLS bytes)"})
}

func TestRealtimeLegacyTimeoutMigrationPreservesManualControl(t *testing.T) {
	f := newRealtimeTable(t, realtimeFixture(true, time.Hour, 0, ""), false)
	// A persisted legacy snapshot is the test's opening condition, not a live
	// control bypass. Seat 0 timed out; seat 1 deliberately enabled manual autoplay.
	f.s.mu.Lock()
	room := f.s.rooms[f.id]
	room.Seats[0].AutoPlay, room.Seats[0].TimeoutAutoPlay = true, true
	room.Seats[1].Bot = false
	room.Seats[1].AutoPlay, room.Seats[1].TimeoutAutoPlay = true, false
	room.Module.Deadline = time.Now().Add(time.Minute).UnixMilli()
	room.Module.TimeoutActors = []int{0}
	room.TurnDeadline = room.Module.Deadline
	originalSystem := room.Module.SystemDeadline
	err := f.s.save(room)
	f.s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	f.ts.Close()
	f.s.Close()
	f.s, err = New(f.cfg, f.s.files)
	if err != nil {
		t.Fatal(err)
	}
	f.s.cancel()
	<-f.s.done
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	restored := f.s.rooms[f.id]
	if restored.Seats[0].AutoPlay || restored.Seats[0].TimeoutAutoPlay || !restored.Seats[1].AutoPlay {
		t.Fatal("migration confused timeout and manual ownership", restored.Seats)
	}
	if restored.TurnDeadline != 0 || restored.Module.Deadline != 0 || len(restored.Module.TimeoutActors) != 0 {
		t.Fatal("legacy idle deadline retained")
	}
	if restored.Module.SystemDeadline != originalSystem {
		t.Fatal("migration changed game system time")
	}
}

func TestRealtimeFrequentInputDoesNotPostponeSystemFrames(t *testing.T) {
	module := realtimeFixture(true, 100*time.Millisecond, 0, "")
	apply := module.Apply
	module.Apply = func(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		next, err := apply(ctx, m, raw)
		// Same coupling as continuous games that advance their simulation on input.
		next.SystemDeadline = ctx.Now.Add(100 * time.Millisecond).UnixMilli()
		return next, err
	}
	module.AcceptStale = func(gameapi.Context, gameapi.Match, json.RawMessage) bool { return true }
	f := newRealtimeTable(t, module, true)
	start := time.Now()
	inputs := 0
	for time.Since(start) < 650*time.Millisecond {
		f.a.command(current(f.a), "action", map[string]any{"drive": 1}, 200)
		inputs++
		time.Sleep(10 * time.Millisecond)
	}
	data := realtimeData(t, current(f.a))
	streamEvidence(t, "input-clock", map[string]any{"inputs": inputs, "frames": data.Frames, "botInputs": data.Inputs, "passed": data.Frames >= 4})
	if data.Frames < 4 || data.Inputs[1] < 4 {
		t.Fatal("continuous human input postponed simulation/bot frames", inputs, data)
	}
}

func TestRealtimeStreamAuthenticationAndPrivateViews(t *testing.T) {
	module := realtimeFixture(true, time.Hour, 0, "")
	module.View = func(match gameapi.Match, viewer int) (any, error) {
		return map[string]any{"viewer": viewer, "fixturePrivateValue": fmt.Sprintf("seat-%d", viewer)}, nil
	}
	f := newRealtimeTable(t, module, false)
	url := "ws" + strings.TrimPrefix(f.ts.URL, "http") + "/api/ws?stream=1"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, res, err := websocket.Dial(ctx, url, nil)
	if err == nil {
		conn.CloseNow()
		t.Fatal("anonymous stream accepted")
	}
	if res == nil || res.StatusCode != 401 {
		t.Fatal("anonymous status", res, err)
	}
	conn, res, err = websocket.Dial(ctx, url, &websocket.DialOptions{HTTPClient: f.a.client, HTTPHeader: http.Header{"Origin": []string{"https://unrelated.invalid"}}})
	if err == nil {
		conn.CloseNow()
		t.Fatal("cross-origin stream accepted")
	}
	if res == nil || res.StatusCode != 403 {
		t.Fatal("origin status", res, err)
	}
	spectator := newClient(t, f.ts.URL)
	spectator.register("实时私有视图观众")
	spectator.post("/api/rooms/"+f.id+"/watch", map[string]any{}, 200)
	seatSocket, watchSocket := streamDial(t, f.a, true), streamDial(t, spectator, true)
	streamUntil(t, seatSocket, "state")
	streamUntil(t, watchSocket, "state")
	outsider := newClient(t, f.ts.URL)
	outsider.register("实时大厅观察者")
	lobby := streamDial(t, outsider, true)
	streamUntil(t, lobby, "state")
	f.frame(time.UnixMilli(f.s.rooms[f.id].Module.SystemDeadline))
	seatPacket, watchPacket := streamUntil(t, seatSocket, "game"), streamUntil(t, watchSocket, "game")
	seatData := seatPacket["game"].(map[string]any)["data"].(map[string]any)
	watchData := watchPacket["game"].(map[string]any)["data"].(map[string]any)
	if seatData["viewer"] != float64(0) || watchData["viewer"] != float64(-1) || seatData["fixturePrivateValue"] == watchData["fixturePrivateValue"] {
		t.Fatal("viewer privacy lost", seatData, watchData)
	}
	readCtx, stop := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer stop()
	if _, raw, err := lobby.Read(readCtx); err == nil {
		t.Fatal("realtime frame leaked to lobby", string(raw))
	}
	streamEvidence(t, "stream-privacy", map[string]any{"passed": true, "seat": seatData, "spectator": watchData, "anonymous": 401, "crossOrigin": 403})
}
