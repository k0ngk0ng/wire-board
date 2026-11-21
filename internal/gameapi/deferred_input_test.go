package gameapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRealtimeDeferredInputCapability(t *testing.T) {
	base := pausedClockModule()
	base.OnPausedTimer = nil
	base.SyncSnapshot = base.View
	base.StreamView = func(Match, int, string) (StreamViewResult, error) { return StreamViewResult{}, nil }
	base.RealtimeDeferredInput = func(json.RawMessage) bool { return true }
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Module){
		"ordinary":  func(module *Module) { module.Realtime = false },
		"no-timer":  func(module *Module) { module.OnTimer = nil },
		"no-stream": func(module *Module) { module.StreamView = nil },
	} {
		module := base
		change(&module)
		if module.Validate() == nil {
			t.Fatal("deferred input capability accepted incompatible module", name)
		}
	}
	raw, err := json.Marshal(base)
	if err != nil || strings.Contains(string(raw), "RealtimeDeferredInput") {
		t.Fatal("private predicate leaked into module metadata", string(raw), err)
	}
}
