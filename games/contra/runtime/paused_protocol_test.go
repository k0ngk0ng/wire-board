package runtime

// New protocol failure cases before implementing paused/resume operations:
// paused presentation is replayed as duplicate pads; revision retries advance
// audio twice; skipped/old revisions mutate authority; a pending resume accepts
// another paused tick; restore loses independent presentation and rule clocks.
import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

func TestRuntimePausedRevisionProtocol(t *testing.T) {
	rules, err := os.ReadFile("../engine/leveldata/stage-1.json")
	if err != nil {
		t.Fatal(err)
	}
	bridge := New(testRulesBuild(t))
	call := func(request map[string]any, want bool) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(request)
		var response map[string]any
		if err := json.Unmarshal([]byte(bridge.JSON(string(raw))), &response); err != nil {
			t.Fatal(err)
		}
		if response["ok"] != want {
			t.Fatalf("request %+v: %+v", request, response)
		}
		if !want {
			return response
		}
		return response["data"].(map[string]any)
	}
	var resources any
	_ = json.Unmarshal(rules, &resources)
	call(map[string]any{"op": "resources", "resources": resources}, true)
	view := call(map[string]any{"op": "new", "seed": "42", "players": 1, "epoch": "paused-wire"}, true)
	id := view["session"]
	revision := func() uint64 {
		t.Helper()
		text, ok := view["revision"].(string)
		if !ok {
			t.Fatal("missing exact revision", view)
		}
		v, e := strconv.ParseUint(text, 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for frame := 1; frame <= 60; frame++ {
		pads := []int{0, 0}
		if frame == 2 || frame == 60 {
			pads[0] = 16
		}
		view = call(map[string]any{"op": "pads", "session": id, "epoch": "paused-wire", "frame": strconv.Itoa(frame), "pads": pads}, true)
	}
	if view["paused"] != true {
		t.Fatal("real Start did not pause")
	}
	prior := view
	next := strconv.FormatUint(revision()+1, 10)
	request := map[string]any{"op": "paused", "session": id, "epoch": "paused-wire", "revision": next}
	view = call(request, true)
	if view["frame"] != prior["frame"] || view["audioTick"] == prior["audioTick"] || view["revision"] == prior["revision"] || view["paused"] != true {
		t.Fatal("paused clocks merged", view)
	}
	repeated := call(request, true)
	if repeated["digest"] != view["digest"] {
		t.Fatal("paused retry repeated")
	}
	call(map[string]any{"op": "paused", "session": id, "epoch": "paused-wire", "revision": strconv.FormatUint(revision()+2, 10)}, false)
	call(map[string]any{"op": "paused", "session": id, "epoch": "wrong", "revision": strconv.FormatUint(revision()+1, 10)}, false)
	view = call(map[string]any{"op": "resume", "session": id, "epoch": "paused-wire", "revision": strconv.FormatUint(revision()+1, 10)}, true)
	call(map[string]any{"op": "paused", "session": id, "epoch": "paused-wire", "revision": strconv.FormatUint(revision()+1, 10)}, false)
	view = call(map[string]any{"op": "pads", "session": id, "epoch": "paused-wire", "frame": "61", "pads": []int{0, 0}, "revision": strconv.FormatUint(revision()+1, 10)}, true)
	if view["paused"] != false || view["frame"] != "61" {
		t.Fatal("resume did not enter next rule step", view)
	}
	checkpoint := call(map[string]any{"op": "checkpoint", "session": id, "epoch": "paused-wire"}, true)
	restored := call(map[string]any{"op": "restore", "epoch": "paused-wire", "checkpoint": checkpoint}, true)
	if restored["digest"] != view["digest"] || restored["revision"] != view["revision"] {
		t.Fatal("checkpoint lost paused clock history")
	}
}
