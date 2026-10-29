package runtime

// Failure inventory: prediction changes authority through shared buffers;
// reconciliation leaks native sessions; self/cross-epoch forks overwrite an
// unrelated session; malformed targets mutate either source or destination.
import (
	"encoding/json"
	"os"
	"testing"
)

func TestRuntimePredictionForkIsolationAndReuse(t *testing.T) {
	b := New(testRulesBuild(t))
	call := func(op map[string]any, valid bool) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(op)
		var r map[string]any
		if e := json.Unmarshal([]byte(b.JSON(string(raw))), &r); e != nil {
			t.Fatal(e)
		}
		if r["ok"] != valid {
			t.Fatalf("%v: %v", op, r)
		}
		if !valid {
			return nil
		}
		return r["data"].(map[string]any)
	}
	raw, e := os.ReadFile("../engine/leveldata/stage-1.json")
	if e != nil {
		t.Fatal(e)
	}
	var resources any
	json.Unmarshal(raw, &resources)
	call(map[string]any{"op": "resources", "resources": resources}, true)
	source := call(map[string]any{"op": "new", "seed": "42", "players": 2, "epoch": "match"}, true)
	id := source["session"]
	fork := call(map[string]any{"op": "fork", "session": id, "epoch": "match"}, true)
	if fork["session"] == id || fork["digest"] != source["digest"] {
		t.Fatal("invalid independent fork")
	}
	target := fork["session"]
	for n := 0; n < 100; n++ {
		call(map[string]any{"op": "pads", "session": target, "epoch": "match", "frame": "1", "pads": []int{65, 0}}, true)
		unchanged := call(map[string]any{"op": "snapshot", "session": id, "epoch": "match"}, true)
		if unchanged["digest"] != source["digest"] {
			t.Fatal("prediction mutated authority")
		}
		reset := call(map[string]any{"op": "fork", "session": id, "target": target, "epoch": "match"}, true)
		if reset["session"] != target || reset["digest"] != source["digest"] {
			t.Fatal("fork did not reuse/reset target")
		}
	}
	other := call(map[string]any{"op": "new", "seed": "42", "players": 1, "epoch": "other"}, true)
	for _, bad := range []map[string]any{
		{"op": "fork", "session": id, "target": id, "epoch": "match"},
		{"op": "fork", "session": id, "target": other["session"], "epoch": "match"},
		{"op": "fork", "session": id, "target": 999, "epoch": "match"},
		{"op": "fork", "session": id, "epoch": "wrong"},
	} {
		call(bad, false)
	}
	if len(b.sessions) != 3 {
		t.Fatal("fork leaked sessions", len(b.sessions))
	}
	call(map[string]any{"op": "dispose", "session": target, "epoch": "match"}, true)
	if len(b.sessions) != 2 {
		t.Fatal("prediction session not released")
	}
}
