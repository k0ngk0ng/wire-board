package server

// Failure inventory is recorded before implementation in
// scripts/controllers/gamepad/SCENARIOS.md. These use the real HTTP and SQLite
// boundary, including legacy clients, account/model isolation and validation.
import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func padConfig(button int) map[string]any {
	return map[string]any{"deadzone": 0.25, "bindings": map[string]any{"a": []any{map[string]any{"type": "button", "index": button}}, "left": []any{map[string]any{"type": "axis", "index": 0, "direction": -1}}}}
}
func TestControllerGamepadsE2E(t *testing.T) {
	_, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("PadAlice")
	b.register("PadBob")
	p := controllerProfileHTTP(t, a, "classic-2")
	keyboard := p["keyboard"]
	p["gamepads"] = map[string]any{"standard:Pad A": padConfig(1), "raw:Pad B": padConfig(5)}
	p["gamepadDevice"] = "standard:Pad A"
	wire, _ := json.Marshal(p["gamepads"])
	var want any
	_ = json.Unmarshal(wire, &want)
	saved := a.post("/api/controllers/classic-2", p, 200)
	if !reflect.DeepEqual(saved["keyboard"], keyboard) || !reflect.DeepEqual(saved["gamepads"], want) {
		t.Fatal(saved)
	}
	a.post("/api/controllers/classic-2", p, 409)
	if controllerProfileHTTP(t, b, "classic-2")["gamepads"] != nil || controllerProfileHTTP(t, a, "classic-4")["gamepads"] != nil {
		t.Fatal("gamepad profile leaked across account/model")
	}
	// An older keyboard-only client must not erase device preferences.
	legacy := map[string]any{"keyboard": saved["keyboard"], "revision": saved["revision"], "turboHz": saved["turboHz"]}
	newer := a.post("/api/controllers/classic-2", legacy, 200)
	if !reflect.DeepEqual(newer["gamepads"], saved["gamepads"]) || newer["gamepadDevice"] != saved["gamepadDevice"] {
		t.Fatal("legacy save erased gamepads", newer)
	}
	// Device-only changes preserve the saved maps, including explicit auto mode.
	selection := map[string]any{"keyboard": newer["keyboard"], "revision": newer["revision"], "turboHz": newer["turboHz"], "gamepadDevice": "none"}
	newer = a.post("/api/controllers/classic-2", selection, 200)
	if newer["gamepadDevice"] != "none" || !reflect.DeepEqual(newer["gamepads"], saved["gamepads"]) {
		t.Fatal("device-only selection was ignored", newer)
	}
	selection["revision"] = newer["revision"]
	selection["gamepadDevice"] = ""
	newer = a.post("/api/controllers/classic-2", selection, 200)
	if newer["gamepadDevice"] != "" || !reflect.DeepEqual(newer["gamepads"], saved["gamepads"]) {
		t.Fatal("explicit auto selection was ignored", newer)
	}
	// Explicit reset remains possible and does not affect the keyboard map.
	newer["gamepads"] = map[string]any{}
	newer["gamepadDevice"] = "none"
	reset := a.post("/api/controllers/classic-2", newer, 200)
	if reset["gamepads"] != nil || reset["gamepadDevice"] != "none" {
		t.Fatal(reset)
	}
}
func TestControllerGamepadValidationE2E(t *testing.T) {
	_, ts := setupServer(t)
	c := newClient(t, ts.URL)
	c.register("PadInvalid")
	cases := []map[string]any{
		{"deadzone": 0, "bindings": map[string]any{}},
		{"deadzone": 1, "bindings": map[string]any{}},
		{"deadzone": 0.25, "bindings": map[string]any{"unknown": []any{map[string]any{"type": "button", "index": 0}}}},
		{"deadzone": 0.25, "bindings": map[string]any{"a": []any{map[string]any{"type": "button", "index": 64}}}},
		{"deadzone": 0.25, "bindings": map[string]any{"a": []any{map[string]any{"type": "button", "index": -1}}}},
		{"deadzone": 0.25, "bindings": map[string]any{"a": []any{map[string]any{"type": "button"}}}},
		{"deadzone": 0.25, "bindings": map[string]any{"a": []any{map[string]any{"type": "axis", "index": 0, "direction": 0}}}},
		{"deadzone": 0.25, "bindings": map[string]any{"a": []any{map[string]any{"type": "axis", "index": 16, "direction": 1}}}},
		{"deadzone": 0.25, "bindings": map[string]any{"a": []any{map[string]any{"type": "button", "index": 0}}, "b": []any{map[string]any{"type": "button", "index": 0}}}},
		{"deadzone": 0.25, "bindings": nil},
	}
	original := controllerProfileHTTP(t, c, "classic-2")
	for _, bad := range cases {
		p := controllerProfileHTTP(t, c, "classic-2")
		p["gamepads"] = map[string]any{"raw:test": bad}
		c.post("/api/controllers/classic-2", p, 400)
		if !reflect.DeepEqual(original, controllerProfileHTTP(t, c, "classic-2")) {
			t.Fatal("invalid write changed profile")
		}
	}
	p := controllerProfileHTTP(t, c, "classic-2")
	p["gamepads"] = map[string]any{"bad-device-key": padConfig(0)}
	c.post("/api/controllers/classic-2", p, 400)
}

func TestControllerGamepadBoundsE2E(t *testing.T) {
	_, ts := setupServer(t)
	c := newClient(t, ts.URL)
	c.register("PadBounds")
	original := controllerProfileHTTP(t, c, "classic-2")
	for _, name := range []string{"raw: ", "standard:", "raw:bad\nname", "raw:" + strings.Repeat("x", 1025)} {
		p := controllerProfileHTTP(t, c, "classic-2")
		p["gamepads"] = map[string]any{name: padConfig(0)}
		c.post("/api/controllers/classic-2", p, 400)
	}
	p := controllerProfileHTTP(t, c, "classic-2")
	many := map[string]any{}
	for i := 0; i < 17; i++ {
		many[fmt.Sprintf("raw:device-%d", i)] = padConfig(0)
	}
	p["gamepads"] = many
	c.post("/api/controllers/classic-2", p, 400)
	p = controllerProfileHTTP(t, c, "classic-2")
	p["gamepadDevice"] = "invalid"
	c.post("/api/controllers/classic-2", p, 400)
	bindings := []any{}
	for i := 0; i < 5; i++ {
		bindings = append(bindings, map[string]any{"type": "button", "index": i})
	}
	p = controllerProfileHTTP(t, c, "classic-2")
	p["gamepads"] = map[string]any{"raw:test": map[string]any{"deadzone": .25, "bindings": map[string]any{"a": bindings}}}
	c.post("/api/controllers/classic-2", p, 400)
	p = controllerProfileHTTP(t, c, "classic-2")
	p["gamepads"] = map[string]any{"raw:test": map[string]any{"deadzone": .25, "bindings": map[string]any{"a": []any{map[string]any{"type": "button", "index": 0, "extra": true}}}}}
	c.post("/api/controllers/classic-2", p, 400)
	p = controllerProfileHTTP(t, c, "classic-2")
	p["gamepadDevice"] = "raw:" + strings.Repeat("x", 140000)
	c.post("/api/controllers/classic-2", p, 400)
	if !reflect.DeepEqual(original, controllerProfileHTTP(t, c, "classic-2")) {
		t.Fatal("invalid device payload changed settings")
	}
}
