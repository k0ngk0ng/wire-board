package engine_test

// Regression first against an unedited original tournament checkpoint.
// Failures: after two-party rescue combat, Li's solo duel retains the previous
// UI cursor (slot 1) while waiting, causing capture/save to fail; save changes
// that original cursor or advances the fade; the first real command uses the
// departed role; negative/out-of-capacity cursors or an unavailable role in an
// interactive battle menu are accepted as valid saved metadata.
import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestOriginalSoloDuelAfterPartyShrinks(t *testing.T) {
	ctx := context.Background()
	r := nativeRuntime(t)
	b, err := os.ReadFile(filepath.Join("..", "..", "..", ".local/pal/verification/tournament-duel-entry-state-1.json"))
	if err != nil {
		t.Fatal("run the original tournament E2E first", err)
	}
	var state struct {
		Native []byte `json:"native"`
	}
	if err = json.Unmarshal(b, &state); err != nil {
		t.Fatal(err)
	}
	s, err := r.Decode(state.Native)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; !s.View.Battle.Active; i++ {
		if i > 128 {
			t.Fatal("original tournament duel did not start")
		}
		ack := uint32(0)
		if s.Event.Kind == engine.Dialogue {
			ack = 1
		}
		s, err = r.Step(ctx, s, ack)
		if err != nil {
			t.Fatal(err)
		}
	}
	if s.View.Scene != 33 || len(s.View.Party) != 1 || s.View.Party[0] != 0 || s.View.Battle.State != 0 || s.View.Battle.Current != 1 {
		t.Fatal("not the original wait boundary after party shrink", s.View.Battle)
	}
	encoded, err := r.Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	fresh := nativeRuntime(t)
	after, err := fresh.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	after, err = fresh.ReleaseInputs(ctx, after)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.View, after.View) {
		t.Fatal("saving the original duel wait changed its native presentation")
	}
	for _, tc := range []struct {
		name           string
		state, current int
	}{
		{"negative wait cursor", 0, -1}, {"cursor beyond native capacity", 0, 3},
		{"departed role command", 1, 1}, {"departed role enemy target", 2, 1},
		{"departed role player target", 3, 1}, {"departed role all-enemy target", 4, 1}, {"departed role all-player target", 5, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := s
			bad.View.Battle.State = tc.state
			bad.View.Battle.Current = tc.current
			if _, err := r.Encode(bad); err == nil {
				t.Fatal("invalid saved battle cursor accepted")
			}
		})
	}
	for i := 0; s.View.Battle.State == 0; i++ {
		if i > 256 {
			t.Fatal("solo duel never offered Li's command")
		}
		ack := uint32(0)
		if s.Event.Kind == engine.Dialogue {
			ack = 1
		}
		s, err = r.Step(ctx, s, ack)
		if err != nil {
			t.Fatal(err)
		}
		after, err = fresh.Step(ctx, after, ack)
		if err != nil {
			t.Fatal(err)
		}
		if s.Event != after.Event || !reflect.DeepEqual(s.View, after.View) {
			t.Fatal("saved solo duel initialization diverged")
		}
	}
	if s.View.Battle.State != 1 || s.View.Battle.Current != 0 || len(s.View.Battle.Players) != 1 || s.View.Battle.Players[0].Role != 0 {
		t.Fatal("solo duel offers a departed role", s.View.Battle)
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original two-party rescue to Li-only tournament initialization; stale wait cursor preserved, fresh-runtime save and normal menu reset; forged interactive/out-of-capacity cursors rejected", "waitCursor": 1, "firstCommandCursor": 0, "negativeCases": 7, "scene": s.View.Scene}, "", "  ")
	if err = os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/solo-duel-party-change.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
