package runtime

// Failure inventory before the bridge: new play starts from an oracle/ROM
// snapshot; client-controlled seed/state/outcome replaces authority; epoch/frame
// mismatch advances the wrong match; duplicate pads create another shot; intro
// is skipped without input; native/Wasm diverge; failed requests leak state;
// checkpoints change epoch/controls; results publish before terminal flush.
// These drive the actual development HTTP transport and semantic Session.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeHTTPStartsIntroAndAcceptsRealPads(t *testing.T) {
	raw, err := os.ReadFile("../engine/leveldata/stage-1.json")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewDevelopmentHandler(raw, testRulesBuild(t))
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(handler)
	defer s.Close()
	trace := []json.RawMessage{}
	request := func(path string, body any, want int) map[string]any {
		t.Helper()
		b, _ := json.Marshal(body)
		response, e := http.Post(s.URL+path, "application/json", bytes.NewReader(b))
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		var out map[string]any
		if e = json.NewDecoder(response.Body).Decode(&out); e != nil {
			t.Fatal(e)
		}
		if response.StatusCode != want {
			t.Fatalf("%s got %d want %d: %+v", path, response.StatusCode, want, out)
		}
		proof, _ := json.Marshal(out)
		trace = append(trace, proof)
		return out
	}
	for _, players := range []int{1, 2} {
		created := request("/new", map[string]any{"players": players}, 200)
		id, epoch := created["session"], created["epoch"]
		request("/checkpoint", map[string]any{"session": id, "epoch": epoch}, 200)
		if created["frame"] != "0" || created["phase"] != "initializing" || created["finalized"] != false {
			t.Fatal("non-standard start", created)
		}
		playerViews := created["players"].([]any)
		if playerViews[0].(map[string]any)["lives"] != float64(2) {
			t.Fatal("wrong original lives")
		}
		view := created
		intro := false
		shot := false
		startX := float64(0)
		for frame := 1; frame <= 160; frame++ {
			pads := []int{0, 0}
			if frame == 24 {
				pads[0] = 16
			}
			if frame >= 70 {
				pads[0] = 1
				if frame%8 == 0 {
					pads[0] |= 64
				}
				if players == 2 && frame%9 == 0 {
					pads[1] = 64
				}
			}
			if frame == 70 {
				startX = view["players"].([]any)[0].(map[string]any)["x"].(float64)
			}
			input := map[string]any{"session": id, "epoch": epoch, "frame": fmt.Sprint(frame), "pads": pads}
			view = request("/frame", input, 200)
			intro = intro || view["phase"] == "intro"
			shot = shot || view["projectiles"].(float64) > 0
			if frame == 2 {
				request("/frame", map[string]any{"session": id, "epoch": "wrong", "frame": "3", "pads": pads}, 400)
				request("/frame", map[string]any{"session": id, "epoch": epoch, "frame": "4", "pads": pads}, 400)
				request("/frame", map[string]any{"session": id, "epoch": epoch, "frame": "3", "pads": []int{256, 0}}, 400)
				request("/frame", map[string]any{"session": id, "epoch": epoch, "frame": "3", "pads": pads, "state": map[string]any{"win": true}}, 400)
				request("/restore", map[string]any{"session": id, "epoch": epoch, "state": map[string]any{"win": true}}, 404)
			}
			if frame%31 == 0 {
				again := request("/frame", input, 200)
				if again["digest"] != view["digest"] || again["frame"] != view["frame"] {
					t.Fatal("duplicate advanced simulation")
				}
			}
		}
		if !intro || !shot || view["phase"] != "playing" || view["players"].([]any)[0].(map[string]any)["x"].(float64) == startX {
			t.Fatal("new-session input did not reach actual movement and shooting", view)
		}
	}
	out := filepath.Join("..", "..", "..", ".local", "contra-browser", "protocol")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	proof, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "real local HTTP -> independent Session; one/two-player stage-one entry, no ROM or injected after-state", "trace": trace}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "result.json"), proof, 0600); err != nil {
		t.Fatal(err)
	}
}
