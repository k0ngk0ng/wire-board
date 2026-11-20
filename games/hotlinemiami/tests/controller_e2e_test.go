package hotlinemiami_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func controls(t *testing.T) *gameapi.ControllerDefinition {
	t.Helper()
	raw, err := os.ReadFile("../controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var definition gameapi.ControllerDefinition
	if err = json.Unmarshal(raw, &definition); err != nil {
		t.Fatal(err)
	}
	return &definition
}

func TestControllerDefinitionHTTPPersistence(t *testing.T) {
	definition := controls(t)
	if definition.Layout != "game" || len(definition.Actions) != 13 {
		t.Fatal("incomplete twin-stick control definition")
	}
	continuous := []string{"up", "down", "left", "right", "aimUp", "aimDown", "aimLeft", "aimRight"}
	for _, action := range definition.Actions {
		if slices.Contains(continuous, action.ID) && !action.Continuous {
			t.Fatal("axis lost analog value", action.ID)
		}
	}
	harness := newHTTPRoom(t, 2)
	person := harness.people[1]
	settings := person.request("GET", "/api/controllers?game=hotlinemiami", nil, 200)
	found := false
	for _, value := range settings["models"].([]any) {
		model := value.(map[string]any)
		if model["id"] == "game:hotlinemiami" {
			found = true
		}
	}
	if !found {
		t.Fatal("game controller not exposed through real platform API")
	}
	profile := settings["profiles"].(map[string]any)["game:hotlinemiami"].(map[string]any)
	person.request("POST", "/api/controllers/game:hotlinemiami", profile, 200)
	artifact(t, map[string]any{"passed": true, "scope": "real controller metadata GET and profile save HTTP", "actions": definition.Actions})
}
