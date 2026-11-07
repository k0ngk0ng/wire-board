package games

import (
	"embed"
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"io/fs"
)

// Input metadata is owned by each game, independently of its rule engine.
//
//go:embed */controls.json
var controllerFiles embed.FS

func ControllerDefinitionFor(id string) *gameapi.ControllerDefinition {
	raw, err := controllerFiles.ReadFile(id + "/controls.json")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		panic(err)
	}
	var definition gameapi.ControllerDefinition
	if err = json.Unmarshal(raw, &definition); err != nil {
		panic(err)
	}
	return &definition
}
