package server

// HTTP/SQLite boundary, authored before the minimal FC/MD physical input port.
// Virtual bindings are not evidence for any physical hardware brand.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func physicalPortPad(index int) map[string]any {
	return map[string]any{"deadzone": .25, "bindings": map[string]any{"a": []any{map[string]any{"type": "button", "index": index}}, "left": []any{map[string]any{"type": "axis", "index": 0, "direction": -1}}}}
}
func TestPhysicalControllerPortHTTPPersistenceAndLegacy(t *testing.T) {
	_, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("MDPhysicalA")
	b.register("MDPhysicalB")
	p := controllerProfileHTTP(t, a, "md-6")
	keyboard := p["keyboard"]
	p["gamepads"] = map[string]any{"standard:Virtual MD": physicalPortPad(2), "raw:Virtual Adapter": physicalPortPad(5)}
	p["gamepadDevice"] = "standard:Virtual MD"
	saved := a.post("/api/controllers/md-6", p, 200)
	if !reflect.DeepEqual(saved["keyboard"], keyboard) || saved["gamepads"] == nil {
		t.Fatal("physical settings changed keyboard or disappeared", saved)
	}
	a.post("/api/controllers/md-6", p, 409)
	if controllerProfileHTTP(t, b, "md-6")["gamepads"] != nil || controllerProfileHTTP(t, a, "classic-2")["gamepads"] != nil {
		t.Fatal("device profile crossed user/layout")
	}
	legacy := map[string]any{"keyboard": saved["keyboard"], "revision": saved["revision"], "turboHz": saved["turboHz"]}
	again := a.post("/api/controllers/md-6", legacy, 200)
	if !reflect.DeepEqual(again["gamepads"], saved["gamepads"]) || again["gamepadDevice"] != saved["gamepadDevice"] {
		t.Fatal("old keyboard client erased device profile")
	}
	again["gamepads"] = map[string]any{}
	again["gamepadDevice"] = "none"
	reset := a.post("/api/controllers/md-6", again, 200)
	if reset["gamepads"] != nil || reset["gamepadDevice"] != "none" {
		t.Fatal("explicit physical reset ignored")
	}
	for _, model := range controllerState(t, a)["models"].([]any) {
		if strings.Contains(strings.ToLower(model.(map[string]any)["name"].(string)), "sfc") || model.(map[string]any)["id"] == "classic-4" {
			t.Fatal("removed SFC configuration returned")
		}
	}
}
func TestPhysicalControllerPortHTTPInvalidAndIsolation(t *testing.T) {
	_, ts := setupServer(t)
	c := newClient(t, ts.URL)
	c.register("MDPhysicalInvalid")
	original := controllerProfileHTTP(t, c, "md-6")
	cases := []any{
		map[string]any{"deadzone": 0, "bindings": map[string]any{}}, map[string]any{"deadzone": 1, "bindings": map[string]any{}},
		map[string]any{"deadzone": .25, "bindings": nil}, map[string]any{"deadzone": .25, "bindings": map[string]any{"bad": []any{map[string]any{"type": "button", "index": 0}}}},
		map[string]any{"deadzone": .25, "bindings": map[string]any{"a": []any{map[string]any{"type": "button", "index": 64}}}}, map[string]any{"deadzone": .25, "bindings": map[string]any{"a": []any{map[string]any{"type": "button", "index": -1}}}},
		map[string]any{"deadzone": .25, "bindings": map[string]any{"a": []any{map[string]any{"type": "button"}}}}, map[string]any{"deadzone": .25, "bindings": map[string]any{"a": []any{map[string]any{"type": "axis", "index": 0, "direction": 0}}}},
		map[string]any{"deadzone": .25, "bindings": map[string]any{"a": []any{map[string]any{"type": "axis", "index": 16, "direction": 1}}}}, map[string]any{"deadzone": .25, "bindings": map[string]any{"a": []any{map[string]any{"type": "button", "index": 0}}, "b": []any{map[string]any{"type": "button", "index": 0}}}},
	}
	for _, bad := range cases {
		p := controllerProfileHTTP(t, c, "md-6")
		p["gamepads"] = map[string]any{"raw:Virtual": bad}
		c.post("/api/controllers/md-6", p, 400)
		if !reflect.DeepEqual(original, controllerProfileHTTP(t, c, "md-6")) {
			t.Fatal("invalid physical profile committed")
		}
	}
	for _, key := range []string{"raw: ", "standard:", "raw:bad\nname", "bad-prefix:Virtual", "raw:" + strings.Repeat("x", 1025)} {
		p := controllerProfileHTTP(t, c, "md-6")
		p["gamepads"] = map[string]any{key: physicalPortPad(0)}
		c.post("/api/controllers/md-6", p, 400)
	}
	p := controllerProfileHTTP(t, c, "md-6")
	p["keyboardAlt"] = map[string]any{"up": []string{"ArrowUp"}}
	saved := c.post("/api/controllers/md-6", p, 200)
	p = controllerProfileHTTP(t, c, "md-6")
	p["keyboardAlt"] = map[string]any{"up": []string{"KeyD"}}
	c.post("/api/controllers/md-6", p, 400)
	if !reflect.DeepEqual(saved, controllerProfileHTTP(t, c, "md-6")) {
		t.Fatal("alias conflict committed")
	}
	raw, _ := json.Marshal(saved)
	if strings.Contains(string(raw), "classic-4") {
		t.Fatal("unexpected SFC alias")
	}
}

func TestPhysicalControllerPortPerGameHTTPIsolation(t *testing.T) {
	one, two, none := acceptanceModule(), acceptanceModule(), acceptanceModule()
	one.ID, one.Name = "physical-md-one", "Physical MD one"
	two.ID, two.Name = "physical-md-two", "Physical MD two"
	none.ID, none.Name = "physical-unmapped", "Unmapped"
	one.Controls = &gameapi.ControllerDefinition{Layout: "md-6", LegacyProfile: "md-6"}
	two.Controls = &gameapi.ControllerDefinition{Layout: "md-6", LegacyProfile: "md-6"}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{one, two, none}}, fstest.MapFS{"index.html": {Data: []byte("controller HTTP isolation")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("PhysicalGameA")
	b.register("PhysicalGameB")
	legacy := controllerProfileHTTP(t, a, "md-6")
	legacy["keyboard"].(map[string]any)["x"] = "KeyP"
	legacy["gamepads"] = map[string]any{"standard:Virtual MD": physicalPortPad(2)}
	legacy = a.post("/api/controllers/md-6", legacy, 200)
	first := controllerProfileHTTP(t, a, "game:physical-md-one")
	second := controllerProfileHTTP(t, a, "game:physical-md-two")
	if !reflect.DeepEqual(first["keyboard"], legacy["keyboard"]) || !reflect.DeepEqual(first["gamepads"], legacy["gamepads"]) {
		t.Fatal("legacy MD preferences not copied to game")
	}
	first["keyboard"].(map[string]any)["x"] = "KeyH"
	first["gamepads"].(map[string]any)["raw:Other Virtual"] = physicalPortPad(4)
	a.post("/api/controllers/game:physical-md-one", first, 200)
	if !reflect.DeepEqual(second, controllerProfileHTTP(t, a, "game:physical-md-two")) {
		t.Fatal("game profiles share mutable key")
	}
	legacy["keyboard"].(map[string]any)["x"] = "KeyQ"
	a.post("/api/controllers/md-6", legacy, 200)
	if !reflect.DeepEqual(second, controllerProfileHTTP(t, a, "game:physical-md-two")) {
		t.Fatal("layout overwrote migrated game")
	}
	if controllerProfileHTTP(t, b, "game:physical-md-one")["keyboard"].(map[string]any)["x"] != "KeyU" {
		t.Fatal("game profile crossed user")
	}
	code, data := a.request("GET", "/api/controllers?game=physical-md-one", nil)
	if code != 200 || len(data["models"].([]any)) != 1 || len(data["profiles"].(map[string]any)) != 1 {
		t.Fatal("game query not isolated", code, data)
	}
	model := data["models"].([]any)[0].(map[string]any)
	if model["layout"] != "md-6" || model["gameId"] != "physical-md-one" {
		t.Fatal("wrong game model identity", model)
	}
	standard := model["standardGamepad"].(map[string]any)
	for logical, index := range map[string]float64{"a": 2, "b": 0, "c": 1, "x": 4, "y": 3, "z": 5, "mode": 8, "start": 9} {
		bindings := standard[logical].([]any)
		if bindings[0].(map[string]any)["index"] != index {
			t.Fatal("MDstandard preset differs", logical, bindings)
		}
	}
	if code, _ = a.request("GET", "/api/controllers?game=physical-unmapped", nil); code != 404 {
		t.Fatal("unwired game exposes saveable controller", code)
	}
	a.post("/api/controllers/classic-4", map[string]any{}, 404)
}

// Legacy clients omit aliases, so their new primary keys must be checked after
// the stored aliases are restored. Clearing keeps every logical key present.
func TestPhysicalControllerPortLegacyAliasMergeHTTP(t *testing.T) {
	_, ts := setupServer(t)
	c := newClient(t, ts.URL)
	c.register("MDPhysicalAliasMerge")
	p := controllerProfileHTTP(t, c, "md-6")
	p["keyboardAlt"] = map[string]any{"up": []string{"ArrowUp"}}
	saved := c.post("/api/controllers/md-6", p, 200)
	primary := saved["keyboard"].(map[string]any)
	primary["right"] = "ArrowUp"
	c.post("/api/controllers/md-6", map[string]any{"revision": saved["revision"], "turboHz": saved["turboHz"], "keyboard": primary}, 400)
	unchanged := controllerProfileHTTP(t, c, "md-6")
	if unchanged["keyboard"].(map[string]any)["right"] != "KeyD" {
		t.Fatal("invalid merged alias profile persisted")
	}
	primary = unchanged["keyboard"].(map[string]any)
	primary["x"] = ""
	cleared := c.post("/api/controllers/md-6", map[string]any{"revision": unchanged["revision"], "turboHz": unchanged["turboHz"], "keyboard": primary}, 200)
	if cleared["keyboard"].(map[string]any)["x"] != "" || !reflect.DeepEqual(cleared["keyboardAlt"], unchanged["keyboardAlt"]) {
		t.Fatal("clear omitted logical action or erased saved aliases")
	}
	delete(primary, "x")
	c.post("/api/controllers/md-6", map[string]any{"revision": cleared["revision"], "turboHz": cleared["turboHz"], "keyboard": primary}, 400)
}
