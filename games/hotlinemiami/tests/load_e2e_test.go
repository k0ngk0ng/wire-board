package hotlinemiami_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestEightLiveRoomsAndSlowReceiver(t *testing.T) {
	harness := newHTTPRoom(t, 8)
	for index, person := range harness.people {
		person.request("POST", "/api/rooms", map[string]any{"kind": "hotlinemiami", "name": fmt.Sprintf("并行验收%d", index), "capacity": 4, "options": map[string]any{"map": []string{"no-talk", "full-house", "push-it"}[index%3]}}, 201)
		for range 3 {
			person.command("add_bot", nil, 200)
		}
		person.command("ready", nil, 200)
		person.command("start", nil, 200)
		person.command("autoplay", map[string]any{"enabled": true}, 200)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	slow, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(harness.people[0].base, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: harness.people[0].http})
	if err != nil {
		t.Fatal(err)
	}
	defer slow.CloseNow()
	initial := make([]int64, len(harness.people))
	for index, person := range harness.people {
		initial[index] = integer(person.room()["game"].(map[string]any)["data"].(map[string]any)["frame"])
	}
	ages := make([]int64, 0, 320)
	views := 0
	for range 40 {
		for _, person := range harness.people {
			data := person.room()["game"].(map[string]any)["data"].(map[string]any)
			ages = append(ages, max(0, time.Now().UnixMilli()-integer(data["serverAt"])))
			views++
		}
		time.Sleep(100 * time.Millisecond)
	}
	slices.Sort(ages)
	p95 := ages[len(ages)*95/100]
	final := make([]int64, len(harness.people))
	for index, person := range harness.people {
		data := person.room()["game"].(map[string]any)["data"].(map[string]any)
		final[index] = integer(data["frame"])
		if final[index]-initial[index] < 200 {
			t.Fatal("a room stopped advancing", index, initial[index], final[index])
		}
	}
	if p95 > 67 {
		t.Fatalf("eight-room authoritative clock P95 lag exceeds 67ms: %d", p95)
	}
	artifact(t, map[string]any{"passed": true, "scope": "real production host and SQLite, eight authenticated four-seat rooms with platform autoplay, one non-reading WebSocket", "rooms": 8, "playersPerRoom": 4, "snapshots": views, "initialFrames": initial, "finalFrames": final, "ruleClockLagP95MS": p95, "ruleClockLagMaxMS": ages[len(ages)-1]})
}
