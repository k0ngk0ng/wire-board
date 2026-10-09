package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

type catanTraceAction struct {
	Player int         `json:"player"`
	Action game.Action `json:"action"`
}

// Store game data only: no room users, cookies, or server configuration.
// Keep the initial state and submitted actions so a long bot game is inspectable
// Timeout-driven actions are not in the trace; Current is authoritative.
// even if the Go test alarm terminates the process before cleanup callbacks.
func saveCatanLongGame(t *testing.T, initial, current *game.State, actions []catanTraceAction) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", ".local", "catan-long-games"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(struct {
		Test    string             `json:"test"`
		Initial *game.State        `json:"initial"`
		Current *game.State        `json:"current"`
		Actions []catanTraceAction `json:"actions"`
	}{t.Name(), initial, current, actions})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.CreateTemp(dir, "caravan-*.json")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	if _, err = f.Write(raw); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	t.Log("long-game diagnostic saved", path)
	return path
}

func TestCatanLongGameArtifact(t *testing.T) {
	s, err := game.NewCatanCaravansShoresSeafarers(2)
	if err != nil {
		t.Fatal(err)
	}
	action, err := s.BotAction(s.Turn)
	if err != nil {
		t.Fatal(err)
	}
	path := saveCatanLongGame(t, s, s, []catanTraceAction{{Player: s.Turn, Action: action}})
	defer func() {
		if err := os.Remove(path); err != nil {
			t.Error(err)
		}
	}()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		Initial, Current *game.State
		Actions          []catanTraceAction
	}
	if err = json.Unmarshal(raw, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Initial == nil || artifact.Current == nil || len(artifact.Actions) != 1 {
		t.Fatal("missing diagnostic state")
	}
	if err = artifact.Current.Apply(artifact.Actions[0].Player, artifact.Actions[0].Action); err != nil {
		t.Fatal("snapshot cannot continue", err)
	}
}
