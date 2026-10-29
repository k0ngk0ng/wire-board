package server

// Per-game profile failure inventory before implementation: games share mutable
// bindings; old layout preferences disappear; later layout edits overwrite game
// overrides; key aliases collide; selected-game reads expose another profile.
import (
	"github.com/k0ngk0ng/wire-board/games"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"net/http/httptest"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestControllerGameProfilesE2E(t *testing.T) {
	first, second := acceptanceModule(), acceptanceModule()
	first.ID = "pad-game-one"
	second.ID = "pad-game-two"
	first.Controls = &gameapi.ControllerDefinition{Layout: "classic-2", LegacyProfile: "classic-2"}
	second.Controls = &gameapi.ControllerDefinition{Layout: "classic-2", LegacyProfile: "classic-2"}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{first, second}}, fstest.MapFS{"index.html": {Data: []byte("controls")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("GamePadAlice")
	b.register("GamePadBob")
	legacy := controllerProfileHTTP(t, a, "classic-2")
	legacy["keyboard"].(map[string]any)["right"] = "ArrowRight"
	legacy["gamepads"] = map[string]any{"standard:Pad": padConfig(1)}
	legacy = a.post("/api/controllers/classic-2", legacy, 200)
	one := controllerProfileHTTP(t, a, "game:pad-game-one")
	two := controllerProfileHTTP(t, a, "game:pad-game-two")
	if !reflect.DeepEqual(one["keyboard"], legacy["keyboard"]) || !reflect.DeepEqual(one["gamepads"], legacy["gamepads"]) {
		t.Fatal("legacy preference was not migrated", one)
	}
	one["keyboard"].(map[string]any)["right"] = "KeyO"
	a.post("/api/controllers/game:pad-game-one", one, 200)
	if !reflect.DeepEqual(two, controllerProfileHTTP(t, a, "game:pad-game-two")) {
		t.Fatal("another game's settings changed")
	}
	legacy["keyboard"].(map[string]any)["right"] = "KeyP"
	a.post("/api/controllers/classic-2", legacy, 200)
	if !reflect.DeepEqual(two, controllerProfileHTTP(t, a, "game:pad-game-two")) {
		t.Fatal("layout update overwrote the migrated game")
	}
	if controllerProfileHTTP(t, b, "game:pad-game-one")["keyboard"].(map[string]any)["right"] != "KeyD" {
		t.Fatal("another user's game mapping changed")
	}
	code, v := a.request("GET", "/api/controllers?game=pad-game-one", nil)
	if code != 200 || len(v["profiles"].(map[string]any)) != 1 {
		t.Fatal("game query not isolated", code, v)
	}
}
func TestControllerGameAliasesE2E(t *testing.T) {
	_, ts := setupServer(t)
	c := newClient(t, ts.URL)
	c.register("ControllerAliases")
	p := controllerProfileHTTP(t, c, "classic-2")
	p["keyboardAlt"] = map[string]any{"up": []string{"ArrowUp"}}
	saved := c.post("/api/controllers/classic-2", p, 200)
	invalid := controllerProfileHTTP(t, c, "classic-2")
	invalid["keyboardAlt"] = map[string]any{"up": []string{"KeyD"}}
	c.post("/api/controllers/classic-2", invalid, 400)
	if !reflect.DeepEqual(saved, controllerProfileHTTP(t, c, "classic-2")) {
		t.Fatal("invalid alias changed preferences")
	}
}

func TestControllerDeclaredModelsE2E(t *testing.T) {
	// Legacy room protocols still use their game's declared input metadata;
	// games without an adapter must not expose a nonfunctional navigation preset.
	_, ts := setupServer(t)
	c := newClient(t, ts.URL)
	c.register("DeclaredController")
	code, data := c.request("GET", "/api/controllers?game=splendor", nil)
	if code != 200 {
		t.Fatal(code, data)
	}
	profiles := data["profiles"].(map[string]any)
	if len(profiles) != 1 || profiles["game:splendor"] == nil {
		t.Fatal(data)
	}
}

// Navigation extensions must retain common defaults and expose the game's
// discrete action for per-user remapping without fabricating keyboard events.
func TestControllerNavigationExtensionE2E(t *testing.T) {
	m := acceptanceModule()
	m.ID = "pad-map-extension"
	m.Controls = &gameapi.ControllerDefinition{Layout: "navigation", Actions: []gameapi.ControllerAction{
		{ID: "buildTown", Label: "直接建城镇", Keys: []string{"KeyT"}, Gamepad: []gameapi.ControllerPhysicalBinding{{Type: "button", Index: 3}}},
	}}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("controls")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	c := newClient(t, ts.URL)
	c.register("MapExtendedControls")
	p := controllerProfileHTTP(t, c, "game:pad-map-extension")
	keys := p["keyboard"].(map[string]any)
	if keys["navRight"] != "ArrowRight" || keys["confirm"] != "Enter" || keys["buildTown"] != "KeyT" || len(keys) != 9 {
		t.Fatal(p)
	}
	keys["buildTown"] = "KeyH"
	c.post("/api/controllers/game:pad-map-extension", p, 200)
	if controllerProfileHTTP(t, c, "game:pad-map-extension")["keyboard"].(map[string]any)["buildTown"] != "KeyH" {
		t.Fatal("extension remap not saved")
	}
}

// Failure inventory: a game's declared defaults may contain reserved keys or
// duplicate bindings, so opening a valid-looking profile cannot later be saved.
func TestControllerDeclaredDefaultsRoundTripE2E(t *testing.T) {
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: games.Modules()}, fstest.MapFS{"index.html": {Data: []byte("controls")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	c := newClient(t, ts.URL)
	c.register("ControllerDefaults")
	code, data := c.request("GET", "/api/controllers", nil)
	if code != 200 {
		t.Fatal(code, data)
	}
	profiles := data["profiles"].(map[string]any)
	if profiles["game:snooker"] == nil || len(profiles) < len(games.Modules())+2 {
		t.Fatal("registered defaults missing", data)
	}
	for id := range profiles {
		t.Run(id, func(t *testing.T) {
			client := *c
			client.t = t
			// Saving a legacy layout can change an unmigrated game's source
			// revision. Do not reuse a snapshot from before that legacy write.
			profile := controllerProfileHTTP(t, &client, id)
			client.post("/api/controllers/"+id, profile, 200)
		})
	}
}
