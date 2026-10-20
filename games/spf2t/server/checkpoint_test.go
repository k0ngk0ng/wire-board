package spf2t

// Failure inventory: phase changes are not durable; ordinary physics produces
// a new checkpoint key every frame; round changes reuse a previous key.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func TestCheckpointPhaseContract(t *testing.T) {
	mod := Module()
	ctx := gameapi.Context{Players: 2, Actor: -1, Now: time.Unix(1000, 0)}
	options, err := mod.Configure(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := mod.Start(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	previousPhase, previousKey := "", ""
	changed, stable := 0, 0
	for i := 0; i < 1000; i++ {
		var d struct {
			Phase string `json:"phase"`
			Round int    `json:"round"`
		}
		if err = json.Unmarshal(m.Data, &d); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(d)
		phase := string(raw)
		if m.CheckpointKey == "" {
			t.Fatal("missing critical phase key")
		}
		if previousPhase != "" {
			if phase != previousPhase {
				changed++
				if m.CheckpointKey == previousKey {
					t.Fatal("phase boundary not marked")
				}
			} else {
				stable++
				if m.CheckpointKey != previousKey {
					t.Fatal("ordinary simulation changes checkpoint key")
				}
			}
		}
		if d.Phase == "playing" && changed > 0 && stable > 0 {
			return
		}
		previousPhase, previousKey = phase, m.CheckpointKey
		if m.SystemDeadline == 0 {
			break
		}
		ctx.Now = time.UnixMilli(m.SystemDeadline)
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("fixture did not cover both critical and ordinary frames", changed, stable)
}
