package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestChatIsolationPersistenceAndGameIndependence(t *testing.T) {
	s, ts := setupServer(t)
	a, b, outsider := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	outsider.register("Carol")
	room := startRoom(t, a, b, "splendor")
	id := room["id"].(string)
	path := "/api/rooms/" + id + "/chat"
	body := map[string]string{"text": "<img src=x onerror=alert(1)> 你好\n第二行", "nonce": "chat-test-message"}
	outsider.post(path, body, 403)
	newClient(t, ts.URL).post(path, body, 401)
	watched := make(chan struct{}, 1)
	s.mu.Lock()
	s.watchers[watched] = true
	s.mu.Unlock()
	first := a.post(path, body, 201)
	select {
	case <-watched:
	default:
		t.Fatal("chat did not notify connected players")
	}
	s.mu.Lock()
	delete(s.watchers, watched)
	s.mu.Unlock()
	// Concurrent retries, including a lost response, must not duplicate a message.
	var wg sync.WaitGroup
	codes := make(chan int, 6)
	for range 6 {
		wg.Add(1)
		go func() { defer wg.Done(); code, _ := a.request("POST", path, body); codes <- code }()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 200 {
			t.Fatalf("retry status %d", code)
		}
	}
	after := current(b)
	messages := after["chat"].([]any)
	if len(messages) != 1 || messages[0].(map[string]any)["text"] != body["text"] || first["sender"].(map[string]any)["name"] != "Alice" {
		t.Fatal("message not delivered faithfully", messages)
	}
	if after["version"] != room["version"] || after["turnDeadline"] != room["turnDeadline"] {
		t.Fatal("chat altered game version or clock")
	}
	beforeGame, _ := json.Marshal(room["game"])
	afterGame, _ := json.Marshal(current(a)["game"])
	if string(beforeGame) != string(afterGame) {
		t.Fatal("chat changed the game")
	}
	for _, summary := range outsider.state()["rooms"].([]any) {
		if _, ok := summary.(map[string]any)["chat"]; ok {
			t.Fatal("private room chat leaked in lobby")
		}
	}
	if _, ok := outsider.state()["chat"]; ok {
		t.Fatal("chat leaked to outsider")
	}
	a.command(room, "action", map[string]any{"type": "reserve", "tier": 1}, 200)
	if len(current(b)["chat"].([]any)) != 1 {
		t.Fatal("game action erased chat")
	}
	ts.Close()
	s.Close()
	resumed, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	restarted := httptest.NewServer(resumed.Handler())
	defer restarted.Close()
	a.base = restarted.URL
	b.base = restarted.URL
	a.post(path, body, 200)
	if len(current(a)["chat"].([]any)) != 1 {
		t.Fatal("restart lost history or deduplication")
	}
	a.command(current(a), "close", nil, 200)
	b.command(current(b), "leave", nil, 200)
	b.post(path, map[string]string{"text": "left", "nonce": "left-chat-test"}, 403)
}

func TestChatValidationRateAndRetention(t *testing.T) {
	s, ts := setupServer(t)
	a := newClient(t, ts.URL)
	a.register("Alice")
	room := a.post("/api/rooms", map[string]any{"name": "Chat", "kind": "rail", "capacity": 2}, 201)
	id := room["id"].(string)
	path := "/api/rooms/" + id + "/chat"
	for _, text := range []string{" \n\t", strings.Repeat("宝", 501), "a\x00b"} {
		a.post(path, map[string]string{"text": text, "nonce": randomID(12)}, 400)
	}
	a.post(path, map[string]any{"text": "hello", "nonce": randomID(12), "sender": map[string]string{"name": "Spoof"}}, 400)
	a.post(path, map[string]string{"text": strings.Repeat("💎", 500), "nonce": "unicode-500"}, 201)
	a.post(path, map[string]string{"text": "changed", "nonce": "unicode-500"}, 409)
	for i := 1; i < 20; i++ {
		a.post(path, map[string]string{"text": fmt.Sprint(i), "nonce": randomID(12)}, 201)
	}
	a.post(path, map[string]string{"text": "too fast", "nonce": randomID(12)}, 429)
	s.mu.Lock()
	r := s.rooms[id]
	for key := range s.limits {
		if strings.HasPrefix(key, "chat:") {
			delete(s.limits, key)
		}
	}
	for i := range r.Chat {
		r.Chat[i].SentAt = time.Now().Add(-2 * time.Minute).UnixMilli()
	}
	for len(r.Chat) < 100 {
		r.Chat = append(r.Chat, ChatMessage{ID: randomID(12), Sender: User{ID: "old-player", Name: "Old"}, Text: "older", Nonce: randomID(12), SentAt: 1})
	}
	oldest := r.Chat[0].ID
	s.mu.Unlock()
	last := a.post(path, map[string]string{"text": "after cooldown", "nonce": "after-cooldown"}, 201)
	messages := current(a)["chat"].([]any)
	if len(messages) != 100 || messages[0].(map[string]any)["id"] == oldest || messages[99].(map[string]any)["id"] != last["id"] {
		t.Fatal("history limit or ordering", len(messages))
	}
}
