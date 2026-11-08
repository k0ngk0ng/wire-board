package stoneage_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestStoneageAcceleratedHTTPClockRestoreE2E(t *testing.T) {
	h := gametest.NewHTTP(t, 2, acceleratedHTTPModule(), "stoneage")
	h.Start(map[string]any{})
	httpSelection(t, h)
	for _, client := range h.People {
		client.Action(httpSuggestion(t, client), 200)
	}
	before := httpView(t, h.People[0])
	if before.Phase != "resolving" {
		t.Fatal("no native timeline")
	}
	h.Restart()
	after := httpView(t, h.People[0])
	if !reflect.DeepEqual(before, after) {
		t.Fatal("scaled clock restore changed native events or RNG projection")
	}
	until := time.Now().Add(30 * time.Second)
	for time.Now().Before(until) && httpView(t, h.People[0]).Phase == "resolving" {
		time.Sleep(20 * time.Millisecond)
	}
	if httpView(t, h.People[0]).Phase == "resolving" {
		t.Fatal("real scheduler did not consume the accelerated deadline")
	}
	report(t, "accelerated-http-clock", map[string]any{"clockScale": 4, "scope": "real HTTP/SQLite/host scheduler, production rules under a monotonic 4x game clock; browser timing uses the unmodified module", "timeline": before})
}

func TestStoneageAcceleratedClockReclaimKeepsDeadlineE2E(t *testing.T) {
	h := gametest.NewHTTP(t, 2, acceleratedHTTPModule(), "stoneage")
	h.Start(map[string]any{})
	httpSelection(t, h)
	a := h.People[0]
	before := a.Room()["turnDeadline"]
	nativeHTTPCommand(t, a, "autoplay", map[string]any{"enabled": true})
	nativeHTTPCommand(t, a, "autoplay", map[string]any{"enabled": false})
	after := a.Room()["turnDeadline"]
	if before != after {
		t.Fatal("no-op reclaim scaled an already converted deadline twice", before, after)
	}
}
