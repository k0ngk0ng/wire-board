package pokepath_test

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	pokepath "github.com/k0ngk0ng/wire-board/games/pokepath/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestNativeControllerHTTPDefaultsRemapAndIsolation(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, ".local/pokepath/native-verification/controller-http")
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(output, "database-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(directory)
	configuration := server.Config{DataDir: directory, InviteCode: "controller-pokepath", AdminUsername: "巡径控制管理员", AdminPassword: "controller-pokepath-password", Modules: []gameapi.Module{pokepath.Module()}}
	application, err := server.New(configuration, fstest.MapFS{"index.html": {Data: []byte("Controller acceptance")}})
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	host := httptest.NewServer(application.Handler())
	defer host.Close()
	clients := make([]*nativeHTTPClient, 2)
	for index := range clients {
		jar, _ := cookiejar.New(nil)
		clients[index] = &nativeHTTPClient{client: &http.Client{Jar: jar, Timeout: 10 * time.Second}, base: host.URL}
	}
	clients[0].request(t, "POST", "/api/login", map[string]any{"name": "巡径控制管理员", "password": "controller-pokepath-password"}, 200)
	clients[1].request(t, "POST", "/api/register", map[string]any{"name": "巡径控制伙伴", "password": "controller-pokepath-password", "invite": "controller-pokepath"}, 200)
	settings := clients[0].request(t, "GET", "/api/controllers?game=pokepath", nil, 200)
	var model map[string]any
	for _, value := range settings["models"].([]any) {
		candidate := nativeHTTPObject(value)
		if candidate["id"] == "game:pokepath" {
			model = candidate
		}
	}
	if model == nil || model["layout"] != "navigation" {
		t.Fatalf("game navigation is not registered: %v", model)
	}
	profile := nativeHTTPObject(nativeHTTPObject(settings["profiles"])["game:pokepath"])
	keys := nativeHTTPObject(profile["keyboard"])
	for action, key := range map[string]string{"profile": "KeyQ", "routes": "KeyW", "box": "KeyE", "shop": "KeyS", "damage": "KeyZ", "ranges": "KeyX", "speed": "ShiftLeft", "select0": "Digit1", "select9": "Digit0", "load0": "F1", "load4": "F5", "navUp": "ArrowUp", "confirm": "Enter"} {
		if keys[action] != key {
			t.Fatalf("default %s: got %v expected %s", action, keys[action], key)
		}
	}
	profile["gamepads"] = map[string]any{"standard:Pokepath verification": map[string]any{"deadzone": 0.25, "bindings": model["standardGamepad"]}}
	profile["gamepadDevice"] = "standard:Pokepath verification"
	saved := clients[0].request(t, "POST", "/api/controllers/game:pokepath", profile, 200)
	keys = nativeHTTPObject(saved["keyboard"])
	keys["profile"] = "F6"
	remapped := clients[0].request(t, "POST", "/api/controllers/game:pokepath", saved, 200)
	clients[0].request(t, "POST", "/api/controllers/game:pokepath", profile, 409)
	other := clients[1].request(t, "GET", "/api/controllers?game=pokepath", nil, 200)
	otherKeys := nativeHTTPObject(nativeHTTPObject(nativeHTTPObject(other["profiles"])["game:pokepath"])["keyboard"])
	if otherKeys["profile"] != "KeyQ" || nativeHTTPObject(remapped["keyboard"])["profile"] != "F6" {
		t.Fatal("controller remap leaked between accounts or did not persist")
	}
	defaults := nativeHTTPObject(nativeHTTPObject(model["defaults"])["keyboard"])
	if defaults["profile"] != "KeyQ" || !reflect.DeepEqual(saved["keyboardAlt"], remapped["keyboardAlt"]) || !reflect.DeepEqual(saved["gamepads"], remapped["gamepads"]) {
		t.Fatal("saving the defaults changed their metadata or removed alternate bindings")
	}
	report := map[string]any{"passed": true, "command": "go test -race -count=1 ./games/pokepath/tests -run '^TestNativeController'", "defaultRoundTrip": true, "remap": true, "staleDenied": true, "accountIsolation": true, "model": model, "profile": remapped}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "result.json"), append(encoded, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}
