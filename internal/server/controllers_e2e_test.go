package server

// Failure scenarios, before implementation: anonymous access, preferences shared
// across accounts/models, stale writes replacing newer settings, invalid key
// codes/duplicates/incomplete mappings, loss after restart, and public leakage.

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func controllerState(t *testing.T, c *testClient) map[string]any {
	t.Helper()
	code, v := c.request("GET", "/api/controllers", nil)
	if code != 200 {
		t.Fatalf("controller state %d: %v", code, v)
	}
	return v
}

func controllerProfileHTTP(t *testing.T, c *testClient, model string) map[string]any {
	t.Helper()
	return controllerState(t, c)["profiles"].(map[string]any)[model].(map[string]any)
}

func TestControllerProfilesE2E(t *testing.T) {
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite"}
	files := fstest.MapFS{"index.html": {Data: []byte("controller acceptance")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	a, b, guest := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	if code, _ := guest.request("GET", "/api/controllers", nil); code != 401 {
		t.Fatalf("anonymous status %d", code)
	}
	guest.post("/api/controllers/classic-2", map[string]any{}, 401)
	a.register("ControllerAlice")
	b.register("ControllerBob")
	profile := controllerProfileHTTP(t, a, "classic-2")
	want := map[string]any{"up": "KeyW", "down": "KeyS", "left": "KeyA", "right": "KeyD", "select": "KeyF", "start": "Space", "b": "KeyJ", "a": "KeyK", "turboB": "KeyU", "turboA": "KeyI"}
	if !reflect.DeepEqual(profile["keyboard"], want) {
		t.Fatalf("defaults: %v", profile)
	}
	profile["gamepads"] = map[string]any{"standard:Restart Pad": padConfig(1)}
	profile["gamepadDevice"] = "standard:Restart Pad"
	profile["keyboard"].(map[string]any)["right"] = "ArrowRight"
	saved := a.post("/api/controllers/classic-2", profile, 200)
	if saved["revision"] != float64(1) {
		t.Fatal(saved)
	}
	models := controllerState(t, a)["models"].([]any)
	if models[0].(map[string]any)["defaults"].(map[string]any)["keyboard"].(map[string]any)["right"] != "KeyD" {
		t.Fatal("saved preferences mutated reset-to-default values")
	}
	a.post("/api/controllers/classic-2", profile, 409)
	if !reflect.DeepEqual(controllerProfileHTTP(t, b, "classic-2")["keyboard"], want) {
		t.Fatal("other account changed")
	}
	if controllerProfileHTTP(t, a, "classic-4")["keyboard"].(map[string]any)["right"] != "KeyD" {
		t.Fatal("other controller model changed")
	}
	other := controllerProfileHTTP(t, a, "classic-4")
	other["keyboard"].(map[string]any)["x"] = "KeyO"
	a.post("/api/controllers/classic-4", other, 200)
	if controllerProfileHTTP(t, a, "classic-2")["keyboard"].(map[string]any)["right"] != "ArrowRight" {
		t.Fatal("model settings overwritten")
	}
	uid := a.state()["user"].(map[string]any)["id"].(string)
	_, public := b.request("GET", "/api/players/"+uid, nil)
	encoded, _ := json.Marshal(public)
	if strings.Contains(string(encoded), "ArrowRight") {
		t.Fatal("private preferences leaked through profile")
	}
	ts.Close()
	s.Close()
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	a.base = ts.URL
	b.base = ts.URL
	if got := controllerProfileHTTP(t, a, "classic-2"); !reflect.DeepEqual(got, saved) {
		t.Fatalf("restart changed preferences: %v", got)
	}
	if !reflect.DeepEqual(controllerProfileHTTP(t, b, "classic-2")["keyboard"], want) {
		t.Fatal("defaults changed after restart")
	}
}

func TestControllerProfileValidationE2E(t *testing.T) {
	_, ts := setupServer(t)
	c := newClient(t, ts.URL)
	c.register("ControllerInvalid")
	original := controllerProfileHTTP(t, c, "classic-2")
	c.post("/api/controllers/unknown", original, 404)
	for _, change := range []func(map[string]any){
		func(p map[string]any) { p["keyboard"].(map[string]any)["right"] = "KeyW" },
		func(p map[string]any) { p["keyboard"].(map[string]any)["right"] = "BadKey" },
		func(p map[string]any) { delete(p["keyboard"].(map[string]any), "a") },
		func(p map[string]any) { p["keyboard"].(map[string]any)["unknown"] = "KeyZ" },
		func(p map[string]any) { p["turboHz"] = 0 },
		func(p map[string]any) { p["turboHz"] = 99 },
		func(p map[string]any) { p["revision"] = -1 },
		func(p map[string]any) { p["userId"] = "another-user" },
	} {
		copy := controllerProfileHTTP(t, c, "classic-2")
		change(copy)
		c.post("/api/controllers/classic-2", copy, 400)
		if got := controllerProfileHTTP(t, c, "classic-2"); !reflect.DeepEqual(got, original) {
			t.Fatal("rejected write changed settings")
		}
	}
}
