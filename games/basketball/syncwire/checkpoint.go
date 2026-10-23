package syncwire

import "github.com/k0ngk0ng/wire-board/games/basketball/engine"

// Checkpoint is transferred explicitly for initialization or recovery only.
type Checkpoint struct {
	Version int                 `json:"version"`
	State   engine.RuntimeState `json:"state"`
}
