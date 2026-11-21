package hotlinemiami_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type loadCallbackSample struct {
	Kind       string `json:"kind"`
	DurationNS int64  `json:"durationNS"`
	AtMS       int64  `json:"atMS"`
}

type loadCallbackMeter struct {
	Lock    sync.Mutex
	Samples []loadCallbackSample
}

func (meter *loadCallbackMeter) record(kind string, start time.Time) {
	sample := loadCallbackSample{Kind: kind, DurationNS: time.Since(start).Nanoseconds(), AtMS: time.Now().UnixMilli()}
	meter.Lock.Lock()
	meter.Samples = append(meter.Samples, sample)
	meter.Lock.Unlock()
}

func (meter *loadCallbackMeter) wrap(module gameapi.Module) gameapi.Module {
	bot, apply, timer, view, stream, snapshot := module.BotAction, module.Apply, module.OnTimer, module.View, module.StreamView, module.SyncSnapshot
	module.BotAction = func(ctx gameapi.Context, match gameapi.Match) (json.RawMessage, error) {
		start := time.Now()
		value, err := bot(ctx, match)
		meter.record("BotAction", start)
		return value, err
	}
	module.Apply = func(ctx gameapi.Context, match gameapi.Match, action json.RawMessage) (gameapi.Match, error) {
		start := time.Now()
		value, err := apply(ctx, match, action)
		meter.record("Apply", start)
		return value, err
	}
	module.OnTimer = func(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		start := time.Now()
		value, err := timer(ctx, match)
		meter.record("OnTimer", start)
		return value, err
	}
	module.View = func(match gameapi.Match, seat int) (any, error) {
		start := time.Now()
		value, err := view(match, seat)
		meter.record("View", start)
		return value, err
	}
	module.StreamView = func(match gameapi.Match, seat int, cursor string) (gameapi.StreamViewResult, error) {
		start := time.Now()
		value, err := stream(match, seat, cursor)
		meter.record("StreamView", start)
		return value, err
	}
	module.SyncSnapshot = func(match gameapi.Match, seat int) (any, error) {
		start := time.Now()
		value, err := snapshot(match, seat)
		meter.record("SyncSnapshot", start)
		return value, err
	}
	return module
}

func (meter *loadCallbackMeter) results() ([]loadCallbackSample, map[string]map[string]int64) {
	meter.Lock.Lock()
	raw := append([]loadCallbackSample{}, meter.Samples...)
	meter.Lock.Unlock()
	groups := map[string][]int64{}
	for _, sample := range raw {
		groups[sample.Kind] = append(groups[sample.Kind], sample.DurationNS)
	}
	stats := map[string]map[string]int64{}
	for kind, values := range groups {
		slices.Sort(values)
		stats[kind] = map[string]int64{"samples": int64(len(values)), "p95NS": values[len(values)*95/100], "maxNS": values[len(values)-1]}
	}
	return raw, stats
}

func TestEightLiveRoomsAndSlowReceiver(t *testing.T) {
	temporary, err := filepath.Abs(filepath.Join("..", "..", "..", ".local", "hotlinemiami", "tmp", "live-load"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(temporary, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", temporary)
	harness := newHTTPRoom(t, 8)
	meter := &loadCallbackMeter{}
	harness.endpoint.Close()
	harness.host.Close()
	harness.config.Modules[0] = meter.wrap(harness.config.Modules[0])
	harness.open(t)
	for index, person := range harness.people {
		person.request("POST", "/api/rooms", map[string]any{"kind": "hotlinemiami", "name": fmt.Sprintf("并行验收%d", index), "capacity": 4, "options": map[string]any{"map": liveRoomMaps()[index]}}, 201)
		for range 3 {
			person.command("add_bot", nil, 200)
		}
		person.command("ready", nil, 200)
		person.command("start", nil, 200)
		person.command("autoplay", map[string]any{"enabled": true}, 200)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		connection, dialErr := (&net.Dialer{}).DialContext(ctx, network, address)
		if dialErr != nil {
			return nil, dialErr
		}
		if tcp, ok := connection.(*net.TCPConn); ok {
			if dialErr = tcp.SetReadBuffer(1024); dialErr != nil {
				connection.Close()
				return nil, dialErr
			}
		}
		return connection, nil
	}
	defer transport.CloseIdleConnections()
	slowClient := &http.Client{Jar: harness.people[0].http.Jar, Transport: transport, Timeout: 10 * time.Second}
	slow, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(harness.people[0].base, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: slowClient})
	if err != nil {
		t.Fatal(err)
	}
	defer slow.CloseNow()
	initial := make([]int64, len(harness.people))
	for index, person := range harness.people {
		room := person.room()
		data := room["game"].(map[string]any)["data"].(map[string]any)
		if integer(room["capacity"]) != 4 || len(data["players"].([]any)) != 4 || data["mapId"] != liveRoomMaps()[index] {
			t.Fatal("load room does not match its four-seat scene plan", index)
		}
		initial[index] = integer(data["frame"])
	}
	ages := make([]int64, 0, 320)
	clockSamples := make([]map[string]any, 0, 320)
	views := 0
	for range 40 {
		for index, person := range harness.people {
			data := person.room()["game"].(map[string]any)["data"].(map[string]any)
			now := time.Now().UnixMilli()
			lag := max(0, now-integer(data["serverAt"]))
			ages = append(ages, lag)
			clockSamples = append(clockSamples, map[string]any{"roomIndex": index, "map": liveRoomMaps()[index], "atMS": now, "serverAtMS": integer(data["serverAt"]), "frame": integer(data["frame"]), "lagMS": lag})
			views++
		}
		time.Sleep(100 * time.Millisecond)
	}
	slices.Sort(ages)
	p95 := ages[len(ages)*95/100]
	final := make([]int64, len(harness.people))
	violations := []string{}
	for index, person := range harness.people {
		data := person.room()["game"].(map[string]any)["data"].(map[string]any)
		final[index] = integer(data["frame"])
		if final[index]-initial[index] < 200 {
			violations = append(violations, fmt.Sprintf("room %d stopped advancing: %d to %d", index, initial[index], final[index]))
		}
	}
	if p95 > 67 {
		violations = append(violations, fmt.Sprintf("eight-room authoritative clock P95 lag exceeds 67ms: %d", p95))
	}
	rawCallbacks, callbackStats := meter.results()
	for _, kind := range []string{"BotAction", "Apply", "OnTimer", "View"} {
		if callbackStats[kind]["samples"] == 0 {
			violations = append(violations, "missing measured callback "+kind)
		}
	}
	for kind, stats := range callbackStats {
		if stats["p95NS"] > 8000000 {
			violations = append(violations, fmt.Sprintf("%s callback P95 exceeds 8ms: %dns", kind, stats["p95NS"]))
		}
	}
	artifact(t, map[string]any{"passed": len(violations) == 0, "scope": "real production host and SQLite, eight authenticated four-seat rooms including three Exposed arenas, platform autoplay, one constrained non-reading Exposed WebSocket", "maps": liveRoomMaps(), "rooms": 8, "playersPerRoom": 4, "slowReceiverRoomIndex": 0, "slowReceiverMap": "exposed", "slowReceiverRequestedReadBufferBytes": 1024, "snapshots": views, "initialFrames": initial, "finalFrames": final, "ruleClockLagP95MS": p95, "ruleClockLagMaxMS": ages[len(ages)-1], "clockSamples": clockSamples, "callbackP95BudgetNS": 8000000, "callbackStats": callbackStats, "callbackSamples": rawCallbacks, "violations": violations})
	if len(violations) > 0 {
		t.Fatal(strings.Join(violations, "; "))
	}
}

func liveRoomMaps() []string {
	return []string{"exposed", "exposed", "exposed", "highball", "hot-heavy", "no-talk", "full-house", "tension"}
}

func TestEightLiveRoomPlanIncludesLargestScene(t *testing.T) {
	plan := liveRoomMaps()
	count := 0
	for _, id := range plan {
		if id == "exposed" {
			count++
		}
	}
	if len(plan) != 8 || plan[0] != "exposed" || count < 3 {
		t.Fatalf("eight-room load plan omits largest-scene slow receiver: %v", plan)
	}
	for _, id := range []string{"highball", "hot-heavy"} {
		if !slices.Contains(plan, id) {
			t.Fatalf("eight-room load plan omits %s", id)
		}
	}
	artifact(t, map[string]any{"passed": true, "maps": plan, "playersPerRoom": 4, "slowReceiverRoomIndex": 0, "slowReceiverMap": "exposed", "callbackP95BudgetNS": 8000000, "clockLagP95BudgetMS": 67})
}
