package server

// Failure inventory, before implementation:
// - Adding actions makes selected-game and all-profile GET return 500.
// - Read-time completion overwrites custom primary/alternate keys, cleared
//   bindings, device choices, deadzones, turbo rate or the stored revision.
// - A new default steals an existing primary/alternate key or physical input.
// - Nonstandard devices receive guessed standard button numbers.
// - GET silently rewrites SQLite, stale saves succeed, or complete profiles
//   receive new defaults on deliberately omitted physical bindings.
// - Unknown actions, invalid/duplicate bindings or malformed stored JSON are
//   accidentally repaired into an accepted profile.

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/k0ngk0ng/wire-board/games"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestControllerModelEvolutionHTTPAndSQLite(t *testing.T) {
	for _, kind := range []string{"generic", "racing"} {
		t.Run(kind, func(t *testing.T) {
			module := acceptanceModule()
			module.ID = "evolving-controls"
			button := func(index int) []gameapi.ControllerPhysicalBinding {
				return []gameapi.ControllerPhysicalBinding{{Type: "button", Index: index}}
			}
			current := &gameapi.ControllerDefinition{Layout: "game", Actions: []gameapi.ControllerAction{
				{ID: "left", Keys: []string{"KeyA", "ArrowLeft"}, Gamepad: button(0)},
				{ID: "right", Keys: []string{"KeyD", "ArrowRight"}, Gamepad: button(1)},
				{ID: "camera", Keys: []string{"KeyC", "KeyV"}, Gamepad: button(4)},
				{ID: "horn", Keys: []string{"KeyH"}, Gamepad: button(5)},
				{ID: "mirror", Keys: []string{"F7"}, Gamepad: button(11)},
			}}
			oldCount := 2
			if kind == "racing" {
				module.ID, current, oldCount = "racing", games.ControllerDefinitionFor("racing"), 11
			}
			old := *current
			old.Actions = append([]gameapi.ControllerAction(nil), current.Actions[:oldCount]...)
			module.Controls = &old
			cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{module}}
			files := fstest.MapFS{"index.html": {Data: []byte("controller evolution")}}
			s, err := New(cfg, files)
			if err != nil {
				t.Fatal(err)
			}
			ts := httptest.NewServer(s.Handler())
			t.Cleanup(func() { ts.Close(); s.Close() })
			c := newClient(t, ts.URL)
			c.register("ControlEvolution")
			id := "game:" + module.ID
			profile := controllerProfileHTTP(t, c, id)
			keys := profile["keyboard"].(map[string]any)
			keys["left"], keys["right"] = "KeyC", ""
			profile["keyboardAlt"] = map[string]any{"left": []string{"KeyH", "ArrowLeft"}, "right": []string{}}
			profile["turboHz"] = 7
			profile["gamepadDevice"] = "standard:Custom Pad"
			profile["gamepads"] = map[string]any{
				"standard:Custom Pad": map[string]any{"deadzone": .37, "bindings": map[string]any{"left": []any{map[string]any{"type": "button", "index": 4}}, "right": []any{}}},
				"raw:Custom Pad":      map[string]any{"deadzone": .42, "bindings": map[string]any{"left": []any{map[string]any{"type": "button", "index": 23}}, "right": []any{}}},
			}
			saved := c.post("/api/controllers/"+id, profile, 200)
			user := c.state()["user"].(map[string]any)["id"].(string)
			var before []byte
			var beforeRevision int64
			if err := s.db.QueryRow("SELECT settings,revision FROM controller_profiles WHERE user_id=? AND model=?", user, id).Scan(&before, &beforeRevision); err != nil {
				t.Fatal(err)
			}
			ts.Close()
			s.Close()
			module.Controls = current
			cfg.Modules = []gameapi.Module{module}
			s, err = New(cfg, files)
			if err != nil {
				t.Fatal(err)
			}
			ts = httptest.NewServer(s.Handler())
			c.base = ts.URL
			code, selected := c.request("GET", "/api/controllers?game="+module.ID, nil)
			if code != 200 {
				t.Fatalf("old saved profile blocked updated model: HTTP %d: %v", code, selected)
			}
			p := selected["profiles"].(map[string]any)[id].(map[string]any)
			if !reflect.DeepEqual(p, controllerProfileHTTP(t, c, id)) {
				t.Fatal("all-profile GET differs from the selected-game GET")
			}
			k := p["keyboard"].(map[string]any)
			if len(k) != len(current.Actions) || k["left"] != "KeyC" || k["right"] != "" || k["camera"] != "" || k["horn"] != "" || k["mirror"] != "F7" {
				t.Fatal("completion lost custom/cleared keys or stole a primary/alternate binding", p)
			}
			if !reflect.DeepEqual(p["keyboardAlt"].(map[string]any)["left"], saved["keyboardAlt"].(map[string]any)["left"]) || p["revision"] != saved["revision"] || p["turboHz"] != float64(7) || p["gamepadDevice"] != saved["gamepadDevice"] {
				t.Fatal("completion replaced saved preferences or revision", p)
			}
			if kind == "generic" && !reflect.DeepEqual(p["keyboardAlt"].(map[string]any)["camera"], []any{"KeyV"}) {
				t.Fatal("unoccupied new alternate key was not completed", p)
			}
			pads := p["gamepads"].(map[string]any)
			for _, device := range []string{"standard:Custom Pad", "raw:Custom Pad"} {
				pad := pads[device].(map[string]any)
				oldPad := saved["gamepads"].(map[string]any)[device].(map[string]any)
				bindings := pad["bindings"].(map[string]any)
				oldBindings := oldPad["bindings"].(map[string]any)
				if pad["deadzone"] != oldPad["deadzone"] || !reflect.DeepEqual(bindings["left"], oldBindings["left"]) || !reflect.DeepEqual(bindings["right"], []any{}) {
					t.Fatal("completion changed existing physical preferences", device, pad)
				}
				if len(bindings["camera"].([]any)) != 0 || device[:4] == "raw:" && len(bindings["horn"].([]any)) != 0 || device[:9] == "standard:" && len(bindings["horn"].([]any)) != 1 {
					t.Fatal("physical completion stole an input or guessed raw device bindings", device, pad)
				}
			}
			var after []byte
			var afterRevision int64
			if err := s.db.QueryRow("SELECT settings,revision FROM controller_profiles WHERE user_id=? AND model=?", user, id).Scan(&after, &afterRevision); err != nil || !bytes.Equal(before, after) || beforeRevision != afterRevision {
				t.Fatal("GET rewrote saved SQLite settings", err)
			}
			updated := c.post("/api/controllers/"+id, p, 200)
			if updated["revision"] != saved["revision"].(float64)+1 {
				t.Fatal("normalized save did not preserve optimistic revision", updated)
			}
			c.post("/api/controllers/"+id, p, 409)
			// A fully current profile with deliberately cleared/missing pad
			// bindings is not an old model and must remain unchanged.
			updated["gamepads"] = saved["gamepads"]
			complete := c.post("/api/controllers/"+id, updated, 200)
			if got := controllerProfileHTTP(t, c, id); !reflect.DeepEqual(got, complete) {
				t.Fatal("full-key profile was unexpectedly completed", got, complete)
			}
			path := filepath.Join("..", "..", ".local", "racing-nfs-backend", "controller-evolution-"+kind+".json")
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			data, _ := json.MarshalIndent(map[string]any{"passed": true, "oldActions": oldCount, "newActions": len(current.Actions), "saved": saved, "normalized": p, "savedAfterEvolution": complete, "readDidNotWriteSQLite": true}, "", "  ")
			if err := os.WriteFile(path, data, 0644); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestControllerEvolutionRejectsInvalidStoredProfilesHTTP(t *testing.T) {
	module := acceptanceModule()
	module.ID = "corrupt-controls"
	module.Controls = &gameapi.ControllerDefinition{Layout: "game", Actions: []gameapi.ControllerAction{{ID: "left", Keys: []string{"KeyA"}}, {ID: "camera", Keys: []string{"KeyC"}}}}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{module}}, fstest.MapFS{"index.html": {Data: []byte("controls")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	c := newClient(t, ts.URL)
	c.register("ControlBadStored")
	user := c.state()["user"].(map[string]any)["id"].(string)
	for _, bad := range []string{
		`{"keyboard":{"unknown":"KeyA"},"turboHz":10}`,
		`{"keyboard":{"left":"BadKey"},"turboHz":10}`,
		`{"keyboard":{"left":"KeyA"},"keyboardAlt":{"left":["KeyA"]},"turboHz":10}`,
		`{"keyboard":{"left":"KeyA"},"keyboardAlt":{"unknown":["KeyB"]},"turboHz":10}`,
		`{"keyboard":{"left":"KeyA"},"gamepads":{"raw:Bad":{"deadzone":0.3,"bindings":{"left":[{"type":"button","index":-1}]}}},"turboHz":10}`,
		`{"keyboard":{"left":"KeyA"},"gamepads":{"raw:Bad":{"deadzone":0.3,"bindings":{"unknown":[]}}},"turboHz":10}`,
		`{"keyboard":null,"turboHz":10}`,
		`{"keyboard":{"left":42},"turboHz":10}`,
		`{"keyboard":{"left":"KeyA","camera":"KeyC"},"turboHz":10,"unknown":true}`,
		`{"keyboard":`,
	} {
		if _, err := s.db.Exec(`INSERT INTO controller_profiles(user_id,model,settings,revision) VALUES(?,?,?,3) ON CONFLICT(user_id,model) DO UPDATE SET settings=excluded.settings`, user, "game:"+module.ID, []byte(bad)); err != nil {
			t.Fatal(err)
		}
		if code, body := c.request("GET", "/api/controllers?game="+module.ID, nil); code != 500 {
			t.Errorf("invalid stored configuration accepted: HTTP %d %v; %s", code, body, bad)
		}
	}
}
