package contra

import (
	"encoding/json"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"strings"
	"testing"
)

// Failure inventory: ordinary data carries the map/presentation/audio history;
// streams repeat the full ring; one viewer advances another cursor; gaps and
// epoch changes replay invalid inputs instead of explicitly requesting sync.
func TestContraCompactViewAndConnectionCursors(t *testing.T) {
	m, c, g := initial(t, 2)
	g = input(t, m, c, g, engine.ButtonStart)
	g = ticks(t, m, &c, g, 50)
	g = input(t, m, c, g, 0)
	for range 200 {
		g = ticks(t, m, &c, g, 50)
	}
	check := func(value any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var wire map[string]json.RawMessage
		json.Unmarshal(raw, &wire)
		var snapshot, bundle map[string]json.RawMessage
		json.Unmarshal(wire["snapshot"], &snapshot)
		json.Unmarshal(wire["bundle"], &bundle)
		for _, key := range []string{"presentation", "audio"} {
			if _, ok := snapshot[key]; ok {
				t.Fatalf("ordinary packet contains %s", key)
			}
		}
		if _, ok := bundle["stages"]; ok {
			t.Fatal("ordinary packet repeats all stages")
		}
		if len(raw) > 5000 {
			t.Fatalf("oversized ordinary view: %d", len(raw))
		}
	}
	v, err := m.View(g, 0)
	if err != nil {
		t.Fatal(err)
	}
	check(v)
	if m.StreamView == nil {
		t.Fatal("missing cursor transport")
	}
	first, err := m.StreamView(g, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	check(first.Data)
	before := wire(t, m, g, 0).Revision
	g = ticks(t, m, &c, g, 50)
	fast, err := m.StreamView(g, 0, first.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	check(fast.Data)
	view := fast.Data.(View)
	count := 0
	for _, frame := range view.Frames {
		if frame.Revision > before {
			count++
		}
	}
	if count != 3 || len(view.Frames) > 9 || view.Resync {
		t.Fatalf("bad delta count=%d retained=%d resync=%v", count, len(view.Frames), view.Resync)
	}
	for _, cursor := range []string{"bad", strings.Repeat("f", 32) + ":1", view.Epoch + ":1"} {
		late, err := m.StreamView(g, 1, cursor)
		if err != nil {
			t.Fatal(err)
		}
		if !late.Data.(View).Resync {
			t.Fatal("missing recovery flag", cursor)
		}
	}
	unchanged, err := m.StreamView(g, 0, fast.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(unchanged.Data.(View).Frames) != 0 || unchanged.Cursor != fast.Cursor {
		t.Fatal("another connection advanced fast cursor")
	}
	sync, err := m.SyncSnapshot(g, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(sync)
	var full struct {
		View       struct{ Snapshot, Bundle map[string]json.RawMessage }
		Checkpoint json.RawMessage
	}
	json.Unmarshal(raw, &full)
	if len(full.View.Snapshot["presentation"]) == 0 || len(full.View.Snapshot["audio"]) == 0 || len(full.Checkpoint) == 0 {
		t.Fatal("explicit recovery lacks complete state")
	}
	if _, ok := full.View.Bundle["stages"]; ok {
		t.Fatal("recovery duplicates the static stage manifest")
	}
}
