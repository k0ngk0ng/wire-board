package basketball_test

// Failure cases specified before implementing network presentation history:
// host-injected short taps disappear in local replay; cancellation or autoplay
// changes produce a divergent world; snapshot batching drops intermediate
// animation frames; replaying a gap partially commits; history grows forever.
import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/games/basketball/runtimebridge"
)

func TestHostVideoHistoryReplaysExactInputAndHandoff(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	initial := hostState(t, m).Runtime
	firstView, err := module.View(m, 0)
	if err != nil {
		t.Fatal(err)
	}
	views := []any{firstView}
	b := runtimebridge.New()
	request := func(value any) map[string]json.RawMessage {
		t.Helper()
		raw, _ := json.Marshal(value)
		var envelope struct {
			OK    bool                       `json:"ok"`
			Data  map[string]json.RawMessage `json:"data"`
			Error any                        `json:"error"`
		}
		if err := json.Unmarshal([]byte(b.JSON(string(raw))), &envelope); err != nil || !envelope.OK {
			t.Fatal("bridge", err, envelope.Error)
		}
		return envelope.Data
	}
	restored := request(map[string]any{"op": "restore", "checkpoint": runtimebridge.Checkpoint{Version: 1, State: initial}})
	var session uint64
	_ = json.Unmarshal(restored["session"], &session)
	lastFrame := initial.VideoFrames
	for index := 0; index < 8; index++ {
		ctx.Actor = 0
		if index == 0 {
			m = hostButtons(t, module, ctx, m, 0, "buttons", engine.ButtonB)
			m = hostButtons(t, module, ctx, m, 0, "buttons", 0)
		}
		if index == 1 {
			m = hostButtons(t, module, ctx, m, 0, "release", 0)
		}
		if index == 2 {
			ctx.Automated[2] = true
		}
		if index == 4 {
			ctx.Automated[2] = false
		}
		if index == 5 {
			m = hostButtons(t, module, ctx, m, 0, "buttons", engine.ButtonRight)
		}
		if index == 6 {
			m = hostButtons(t, module, ctx, m, 0, "release", 0)
		}
		m = hostTick(t, module, &ctx, m)
		s := hostState(t, m)
		var inputs []engine.RecordedVideoInput
		for _, input := range s.VideoInputs {
			if input.Frame > lastFrame {
				inputs = append(inputs, input)
			}
		}
		if len(inputs) != int(s.Runtime.VideoFrames-lastFrame) {
			t.Fatal("missing authoritative intermediate video input")
		}
		replayed := request(map[string]any{"op": "replay", "session": session, "inputs": inputs})
		var actual engine.RuntimeState
		if err := json.Unmarshal(replayed["state"], &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, s.Runtime) {
			t.Fatalf("host replay diverged after frame %d", s.Runtime.VideoFrames)
		}
		var frames []struct {
			Frame   uint64              `json:"frame"`
			Display engine.DisplayState `json:"display"`
		}
		if err := json.Unmarshal(replayed["frames"], &frames); err != nil {
			t.Fatal(err)
		}
		if len(frames) != len(inputs) {
			t.Fatal("batched replay discarded intermediate display frames")
		}
		for n, frame := range frames {
			if frame.Frame != lastFrame+uint64(n)+1 {
				t.Fatal("out-of-order display frame")
			}
		}
		lastFrame = s.Runtime.VideoFrames
		view, err := module.View(m, 0)
		if err != nil {
			t.Fatal(err)
		}
		views = append(views, view)
	}
	// A missing frame must fail without consuming any part of a replay batch.
	s := hostState(t, m)
	first := s.VideoInputs[len(s.VideoInputs)-1]
	first.Frame = lastFrame + 1
	gap := first
	gap.Frame += 2
	invalid := []engine.RecordedVideoInput{first, gap}
	raw, _ := json.Marshal(map[string]any{"op": "replay", "session": session, "inputs": invalid})
	var rejected struct {
		OK bool `json:"ok"`
	}
	_ = json.Unmarshal([]byte(b.JSON(string(raw))), &rejected)
	if rejected.OK {
		t.Fatal("old video input replay was accepted")
	}
	after := request(map[string]any{"op": "snapshot", "session": session})
	var saved runtimebridge.Checkpoint
	_ = json.Unmarshal(after["checkpoint"], &saved)
	if !reflect.DeepEqual(saved.State, s.Runtime) {
		t.Fatal("invalid replay changed state")
	}
	writeVideoFixture(t, "host-views.json", map[string]any{"scope": "actual game module snapshots, input and handoff; not a ROM trace", "views": views})
}

func writeVideoFixture(t *testing.T, name string, value any) {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".local/basketball/network-scene")
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestHostVideoHistoryIsBoundedAndRestorable(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	for n := 0; n < 30; n++ {
		m = hostTick(t, module, &ctx, m)
	}
	s := hostState(t, m)
	if len(s.VideoInputs) != 120 || s.VideoInputs[0].Frame+119 != s.Runtime.VideoFrames {
		t.Fatal("video history is not a bounded consecutive suffix")
	}
	restored, err := module.Restore(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(hostState(t, restored).VideoInputs, s.VideoInputs) {
		t.Fatal("restore replaced confirmed video history")
	}
}
