package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func cursorRequest(t *testing.T, client *testClient) *http.Request {
	t.Helper()
	request, err := http.NewRequest("GET", client.base+"/api/ws?stream=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range client.client.Jar.Cookies(request.URL) {
		request.AddCookie(cookie)
	}
	return request
}

func TestRealtimeCursorRegistrationRequiresRecovery(t *testing.T) {
	module := realtimeFixture(true, time.Hour, 0, "")
	module.StreamView = func(gameapi.Match, int, string) (gameapi.StreamViewResult, error) {
		return gameapi.StreamViewResult{}, nil
	}
	if module.Validate() == nil {
		t.Fatal("stream registration without recovery accepted")
	}
	module.SyncSnapshot = module.View
	if err := module.Validate(); err != nil {
		t.Fatal(err)
	}
	module.Realtime = false
	if module.Validate() == nil {
		t.Fatal("non-realtime stream registration accepted")
	}
}

func TestRealtimeCursorProjectionDetachedOutsideLockAndScoped(t *testing.T) {
	module := realtimeFixture(true, time.Hour, 0, "")
	module.SyncSnapshot = module.View
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	module.StreamView = func(match gameapi.Match, viewer int, cursor string) (gameapi.StreamViewResult, error) {
		once.Do(func() { close(entered); <-release })
		var data realtimePosition
		if err := json.Unmarshal(match.Data, &data); err != nil {
			return gameapi.StreamViewResult{}, err
		}
		match.Data[0] = '!'
		match.Actors[0] = 31
		return gameapi.StreamViewResult{Data: map[string]any{"viewer": viewer, "previous": cursor, "inputs": data.Inputs[0]}, Cursor: "next-token"}, nil
	}
	f := newRealtimeTable(t, module, false)
	request := cursorRequest(t, f.a)
	before := current(f.a)
	type answer struct {
		packet any
		cursor streamConnectionCursor
		ok     bool
	}
	done := make(chan answer, 1)
	go func() {
		packet, cursor, ok := f.s.streamGameCursor(request, streamConnectionCursor{RoomID: f.id, Viewer: 0, Value: "first-token"})
		done <- answer{packet, cursor, ok}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("stream callback not entered")
	}
	// A real authenticated command must finish while stream projection waits.
	commandDone := make(chan struct{})
	go func() {
		f.a.post("/api/rooms/"+f.id, map[string]any{"type": "action", "version": before["version"], "nonce": "outside-cursor-lock", "action": map[string]any{"drive": 1}}, 200)
		close(commandDone)
	}()
	select {
	case <-commandDone:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("stream projection held platform lock")
	}
	close(release)
	var result answer
	select {
	case result = <-done:
	case <-time.After(time.Second):
		t.Fatal("projection did not finish")
	}
	if !result.ok || result.cursor.Value != "next-token" {
		t.Fatal("cursor result unavailable", result)
	}
	packet := result.packet.(map[string]any)
	data := packet["game"].(map[string]any)["data"].(map[string]any)
	if packet["version"] != before["version"] || data["previous"] != "first-token" || data["inputs"] != 0 {
		t.Fatal("detached match and envelope mixed versions", packet)
	}
	if stored := realtimeData(t, current(f.a)); stored.Inputs[0] != 1 {
		t.Fatal("callback changed original room", stored)
	}
	for _, previous := range []streamConnectionCursor{{RoomID: "another-room", Viewer: 0, Value: "foreign"}, {RoomID: f.id, Viewer: -1, Value: "foreign"}, {}} {
		packet, next, ok := f.s.streamGameCursor(request, previous)
		if !ok || next.RoomID != f.id || next.Viewer != 0 {
			t.Fatal("authenticated cursor scope changed", next, ok)
		}
		if packet.(map[string]any)["game"].(map[string]any)["data"].(map[string]any)["previous"] != "" {
			t.Fatal("cursor crossed room/viewer/connection boundary", previous)
		}
	}
	// A callback-free module retains its original view and does not gain cursor state.
	f.s.mu.Lock()
	legacy := f.s.modules[module.ID]
	legacy.StreamView = nil
	f.s.modules[module.ID] = legacy
	f.s.mu.Unlock()
	packetLegacy, next, ok := f.s.streamGameCursor(request, result.cursor)
	if !ok || next != (streamConnectionCursor{}) {
		t.Fatal("legacy callback acquired cursor behavior", next)
	}
	if packetLegacy.(map[string]any)["game"].(map[string]any)["data"].(realtimePosition).Inputs[0] != 1 {
		t.Fatal("legacy view behavior changed")
	}
	streamEvidence(t, "cursor-detached-scope", map[string]any{"passed": true, "capturedVersion": before["version"], "projectionOutsideLock": true, "roomViewerReset": true, "legacyUnchanged": true})
}
