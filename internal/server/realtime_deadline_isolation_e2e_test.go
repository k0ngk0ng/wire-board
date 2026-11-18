package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestDeadlineRoomBlockedRulesDoNotStopOtherRoomOrOverwriteClose(t *testing.T) {
	type position struct {
		Room   int `json:"room"`
		Frames int `json:"frames"`
	}
	entered := make(chan struct{}, 1)
	returning := make(chan struct{}, 1)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var created, slowCalls atomic.Int32
	decode := func(match gameapi.Match) (position, error) {
		var view position
		err := json.Unmarshal(match.Data, &view)
		return view, err
	}
	module := realtimeFixture(true, time.Hour, 0, "")
	module.MinPlayers, module.MaxPlayers = 1, 1
	module.BotAction, module.Reclaim = nil, nil
	module.RealtimeDeadlineFrames = true
	module.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		data, err := json.Marshal(position{Room: int(created.Add(1))})
		return gameapi.Match{Schema: 1, Data: data, Actors: []int{0}, Attention: []int{}, SystemDeadline: c.Now.Add(time.Hour).UnixMilli()}, err
	}
	module.Apply = func(c gameapi.Context, match gameapi.Match, _ json.RawMessage) (gameapi.Match, error) {
		match.SystemDeadline = c.Now.Add(10 * time.Millisecond).UnixMilli()
		return match, nil
	}
	module.View = func(match gameapi.Match, _ int) (any, error) { return decode(match) }
	module.SyncSnapshot = module.View
	module.StreamView = func(match gameapi.Match, _ int, _ string) (gameapi.StreamViewResult, error) {
		view, err := decode(match)
		return gameapi.StreamViewResult{Data: view}, err
	}
	module.OnTimer = func(c gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		view, err := decode(match)
		if err != nil {
			return match, err
		}
		if view.Room == 2 {
			slowCalls.Add(1)
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
			select {
			case returning <- struct{}{}:
			default:
			}
		}
		view.Frames++
		match.Data, err = json.Marshal(view)
		match.SystemDeadline = c.Now.Add(30 * time.Millisecond).UnixMilli()
		return match, err
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{module}}, fstest.MapFS{"index.html": {Data: []byte("two deadline rooms")}})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { unblock(); ts.Close(); s.Close() })
	start := func(name string) (*testClient, string) {
		client := newClient(t, ts.URL)
		client.register(name)
		room := client.post("/api/rooms", map[string]any{"kind": module.ID, "name": name, "capacity": 1}, 201)
		client.command(current(client), "ready", nil, 200)
		client.command(current(client), "start", nil, 200)
		return client, room["id"].(string)
	}
	fast, fastID := start("截止帧快速房间")
	slow, slowID := start("截止帧阻塞房间")
	socket := streamDial(t, fast, true)
	streamUntil(t, socket, "state")
	arm := func(client *testClient, id string) {
		client.post("/api/rooms/"+id, map[string]any{"type": "action", "version": current(client)["version"], "nonce": "deadline-arm-" + id, "action": map[string]int{"drive": 1}}, 200)
	}
	arm(fast, fastID)
	arm(slow, slowID)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("slow rule callback did not enter")
	}
	readLimit, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	first, last, packets := -1, -1, 0
	for packets < 5 {
		kind, data, err := socket.Read(readLimit)
		if err != nil || kind != websocket.MessageText {
			t.Fatalf("blocked room stalled the fast socket after %d frames: %v", packets, err)
		}
		var packet struct {
			Type, RoomID string
			Game         struct{ Data position }
		}
		if json.Unmarshal(data, &packet) != nil {
			t.Fatal("invalid websocket frame")
		}
		if packet.Type != "game" {
			continue
		}
		if packet.RoomID != fastID || packet.Game.Data.Room != 1 {
			t.Fatal("other room escaped onto the fast socket", packet)
		}
		frames := packet.Game.Data.Frames
		if frames <= last {
			continue
		}
		if first < 0 {
			first = frames
		}
		last = frames
		packets++
	}
	if last-first < 4 || slowCalls.Load() != 1 {
		t.Fatal("fast room did not progress or slow rules were started twice", first, last, slowCalls.Load())
	}
	slow.command(current(slow), "close", nil, 200)
	unblock()
	select {
	case <-returning:
	case <-time.After(2 * time.Second):
		t.Fatal("slow frame did not return")
	}
	time.Sleep(200 * time.Millisecond)
	s.mu.Lock()
	closed := s.rooms[slowID]
	view, err := decode(*closed.Module)
	status, version := closed.Status, closed.Version
	s.mu.Unlock()
	if err != nil || view.Frames != 0 || status != "closed" {
		t.Fatal("stale computed frame overwrote close", view, status, err)
	}
	var stored []byte
	if err = s.db.QueryRow("SELECT snapshot FROM rooms WHERE id=?", slowID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var saved Room
	if json.Unmarshal(stored, &saved) != nil || saved.Status != "closed" || saved.Version != version {
		t.Fatal("closed transaction changed in storage")
	}
	out, err := json.MarshalIndent(map[string]any{"passed": true, "fastPacketsWhileOtherRuleBlocked": packets, "fastFirstFrame": first, "fastLastFrame": last, "parallelSlowCalls": slowCalls.Load(), "durableClosedVersion": version, "staleCommitRejected": true}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join("..", "..", ".local/pal/verification/mixed-room-isolation.json"), append(out, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
