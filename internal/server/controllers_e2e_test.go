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
	if controllerProfileHTTP(t, a, "md-6")["keyboard"].(map[string]any)["right"] != "KeyD" {
		t.Fatal("other controller model changed")
	}
	other := controllerProfileHTTP(t, a, "md-6")
	other["keyboard"].(map[string]any)["x"] = "KeyP"
	mdSaved := a.post("/api/controllers/md-6", other, 200)
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
	if got := controllerProfileHTTP(t, a, "md-6"); !reflect.DeepEqual(got, mdSaved) {
		t.Fatalf("restart changed MD preferences: %v", got)
	}
}

func TestControllerMDProfilesE2E(t *testing.T) {
	_, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("MDControllerAlice")
	b.register("MDControllerBob")
	state := controllerState(t, a)
	models := state["models"].([]any)
	ids := []string{}
	for _, raw := range models {
		ids = append(ids, raw.(map[string]any)["id"].(string))
	}
	if !reflect.DeepEqual(ids, []string{"classic-2", "md-6"}) {
		t.Fatalf("controller models: %v", ids)
	}
	want := map[string]any{"up": "KeyW", "down": "KeyS", "left": "KeyA", "right": "KeyD", "mode": "KeyF", "start": "Space", "b": "KeyJ", "a": "KeyK", "c": "KeyL", "x": "KeyU", "y": "KeyI", "z": "KeyO"}
	p := controllerProfileHTTP(t, a, "md-6")
	if !reflect.DeepEqual(p["keyboard"], want) {
		t.Fatalf("MD defaults: %v", p)
	}
	a.post("/api/controllers/classic-4", p, 404)
	for _, change := range []func(map[string]any){
		func(p map[string]any) { p["keyboard"].(map[string]any)["c"] = "KeyO" },
		func(p map[string]any) { delete(p["keyboard"].(map[string]any), "z") },
		func(p map[string]any) { p["keyboard"].(map[string]any)["turboA"] = "KeyP" },
	} {
		invalid := controllerProfileHTTP(t, a, "md-6")
		change(invalid)
		a.post("/api/controllers/md-6", invalid, 400)
	}
	p["keyboard"].(map[string]any)["c"] = "KeyP"
	saved := a.post("/api/controllers/md-6", p, 200)
	a.post("/api/controllers/md-6", p, 409)
	if !reflect.DeepEqual(controllerProfileHTTP(t, a, "md-6"), saved) {
		t.Fatal("MD profile did not persist")
	}
	if !reflect.DeepEqual(controllerProfileHTTP(t, b, "md-6")["keyboard"], want) {
		t.Fatal("MD settings leaked to another user")
	}
	fc := controllerProfileHTTP(t, a, "classic-2")["keyboard"].(map[string]any)
	if fc["turboB"] != "KeyU" || fc["turboA"] != "KeyI" || fc["b"] != "KeyJ" || fc["a"] != "KeyK" {
		t.Fatal("MD settings changed FC defaults")
	}
	for _, raw := range controllerState(t, a)["models"].([]any) {
		m := raw.(map[string]any)
		if m["id"] == "md-6" && !reflect.DeepEqual(m["defaults"].(map[string]any)["keyboard"], want) {
			t.Fatal("MD reset defaults mutated")
		}
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
