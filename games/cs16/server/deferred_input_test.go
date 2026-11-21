package cs16

import (
	"encoding/json"
	"testing"
)

func TestDeferredInputOnlyQueuesNormalCommands(t *testing.T) {
	for _, raw := range []string{`{"type":"input","fire":true}`, `{"type":"input","jump":true}`, `{"type":"input","use":true}`, `{"type":"input","select":"awp"}`, `{"type":"input","cancel":false}`} {
		if !deferredInput(json.RawMessage(raw)) {
			t.Fatal("ordinary queued edge was not deferred", raw)
		}
	}
	for _, raw := range []string{`{"type":"input","cancel":true}`, `{"type":"buy","item":"ak47"}`, `{"type":"unknown"}`, `null`, `{`, ``} {
		if deferredInput(json.RawMessage(raw)) {
			t.Fatal("immediate or invalid command was deferred", raw)
		}
	}
	module := Module()
	if module.RealtimeDeferredInput == nil || !module.Realtime || module.OnTimer == nil || module.StreamView == nil {
		t.Fatal("queued input declaration incomplete")
	}
}
