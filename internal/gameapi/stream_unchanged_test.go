package gameapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStreamViewUnchangedIsOptionalAndPrivate(t *testing.T) {
	ordinary := StreamViewResult{Data: map[string]int{"frame": 1}, Cursor: "first"}
	if ordinary.Unchanged {
		t.Fatal("existing stream result gained skip behavior")
	}
	module := pausedClockModule()
	module.OnPausedTimer = nil
	module.SyncSnapshot = func(Match, int) (any, error) { return nil, nil }
	module.StreamView = func(Match, int, string) (StreamViewResult, error) {
		return StreamViewResult{Unchanged: true}, nil
	}
	if err := module.Validate(); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(module)
	if err != nil || strings.Contains(string(raw), "Unchanged") || strings.Contains(string(raw), "StreamView") {
		t.Fatal("connection-private stream metadata entered public module registration", string(raw), err)
	}
}
