package pal_test

// Test first. Detached stream failures: no callback keeps full native JSON
// decoding under the shared host lock; projections lose per-seat ACKs, leak
// private native memory, share mutable slices, accept invalid saves, or race
// with original rule advancement. Stream cursors must not change visibility.
import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalDetachedStreamViews(t *testing.T) {
	m, _ := nativeModule(t)
	if m.StreamView == nil {
		t.Fatal("PAL still projects streams while holding the shared host lock")
	}
	c := gameapi.Context{Players: 1, Actor: 0, Now: time.Unix(1800000000, 0)}
	g := originalInn(t, m, &c)
	v := view(t, m, g, 0)
	g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: 1, Direction: 4, Keys: 4})
	before := bytes.Clone(g.Data)
	var group sync.WaitGroup
	for _, viewer := range []int{-1, 0} {
		group.Go(func() {
			for _, cursor := range []string{"", "stale-or-other-connection"} {
				streamed, e := m.StreamView(g, viewer, cursor)
				if e != nil {
					t.Error(e)
					return
				}
				want, e := m.View(g, viewer)
				if e != nil || !reflect.DeepEqual(streamed.Data, want) {
					t.Error("detached projection differs from authenticated public view", viewer, e)
					return
				}
				encoded, e := json.Marshal(streamed.Data)
				if e != nil || strings.Contains(string(encoded), `"native"`) || strings.Contains(string(encoded), `"memory"`) || strings.Contains(string(encoded), `"seed"`) {
					t.Error("stream exposed original private state", e)
					return
				}
				projection := streamed.Data.(pal.View)
				projection.Party[0].Name = "caller mutation"
				if len(projection.Draws) > 0 {
					projection.Draws[0].X++
				}
				again, e := m.StreamView(g, viewer, streamed.Cursor)
				if e != nil || !reflect.DeepEqual(again.Data, want) {
					t.Error("stream projections share mutable native presentation", viewer, e)
				}
			}
		})
	}
	// The source is immutable, and a detached native rule transaction proceeds
	// alongside both owner/spectator projections of that same source version.
	c.Now = time.UnixMilli(g.SystemDeadline + 200)
	next, e := m.OnTimer(c, g)
	if e != nil {
		t.Fatal(e)
	}
	group.Wait()
	if !bytes.Equal(before, g.Data) || bytes.Equal(next.Data, g.Data) {
		t.Fatal("detached projection changed its source or blocked original rules")
	}
	bad := g
	bad.Data = json.RawMessage(`{"invalid":true}`)
	if _, e = m.StreamView(bad, 0, ""); e == nil {
		t.Fatal("detached view accepted an invalid native match")
	}
	if !t.Failed() {
		report, _ := json.MarshalIndent(map[string]any{"passed": true, "test": t.Name(), "scope": "actual original inn, single owner and spectator; equivalent detached public views, independent ACKs, mutable output isolation, invalid input rejection and concurrent native advancement; not performance acceptance"}, "", "  ")
		if e = os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/detached-stream.json"), append(report, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
}
