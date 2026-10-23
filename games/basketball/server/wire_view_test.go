package basketball_test

// Before implementation: ordinary views leak full runtime/long history;
// recovery metadata and state use different boundaries; compression drops
// authoritative input edges; view/checkpoint reads mutate or pause the game.
import (
	"encoding/json"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/games/basketball/syncwire"
	"reflect"
	"testing"
)

func TestBasketballCompactViewAndExplicitCheckpoint(t *testing.T) {
	mod, ctx, match := hostPlaying(t)
	for n := 0; n < 30; n++ {
		match = hostTick(t, mod, &ctx, match)
	}
	before := string(match.Data)
	value, e := mod.View(match, 0)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	var wire map[string]json.RawMessage
	if e = json.Unmarshal(raw, &wire); e != nil {
		t.Fatal(e)
	}
	if wire["runtime"] != nil || wire["videoInputs"] != nil {
		t.Fatal("regular view contains full simulation/history")
	}
	if len(raw) > 4096 {
		t.Fatalf("ordinary game view still too large: %d bytes", len(raw))
	}
	var protocol int
	json.Unmarshal(wire["protocol"], &protocol)
	if protocol != 1 {
		t.Fatal("missing input protocol version")
	}
	var window syncwire.InputWindow
	if e = json.Unmarshal(wire["replay"], &window); e != nil {
		t.Fatal(e)
	}
	inputs, e := syncwire.Unpack(window)
	if e != nil {
		t.Fatal(e)
	}
	s := hostState(t, match)
	if len(inputs) == 0 || len(inputs) > 24 || !reflect.DeepEqual(inputs, s.VideoInputs[len(s.VideoInputs)-len(inputs):]) {
		t.Fatal("compact window changed authority")
	}
	var digest string
	json.Unmarshal(wire["digest"], &digest)
	expected, e := syncwire.RuntimeDigest(s.Runtime)
	if e != nil || digest != expected {
		t.Fatal("wrong authority checksum", e)
	}
	if mod.SyncSnapshot == nil {
		t.Fatal("missing explicit checkpoint")
	}
	snapshot, e := mod.SyncSnapshot(match, 0)
	if e != nil {
		t.Fatal(e)
	}
	encoded, _ := json.Marshal(snapshot)
	var payload struct {
		View       json.RawMessage     `json:"view"`
		Checkpoint syncwire.Checkpoint `json:"checkpoint"`
	}
	if e = json.Unmarshal(encoded, &payload); e != nil {
		t.Fatal(e)
	}
	if payload.Checkpoint.Version != 1 || !reflect.DeepEqual(payload.Checkpoint.State, s.Runtime) {
		t.Fatal("checkpoint differs from authoritative state")
	}
	if string(payload.View) != string(raw) {
		t.Fatal("bootstrap metadata differs from the same view")
	}
	if string(match.Data) != before {
		t.Fatal("read advanced or altered game")
	}
	if _, e = mod.SyncSnapshot(match, -1); e != nil {
		t.Fatal("authorized spectators cannot bootstrap", e)
	}
	if _, e = mod.SyncSnapshot(match, 4); e == nil {
		t.Fatal("invalid seat accepted")
	}
	// Internal diagnostics remain available to native host tests, not on the wire.
	if value.(game.View).Runtime.VideoFrames != s.Runtime.VideoFrames {
		t.Fatal("native diagnostic state missing")
	}
}
