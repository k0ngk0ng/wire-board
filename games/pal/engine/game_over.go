package engine

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/tetratelabs/wazero/api"
)

type GameOverState struct {
	Stage int `json:"stage"`
	Slot  int `json:"slot"`
}

func validateGameOver(v *GameOverState, event Event) error {
	if v == nil {
		if event.Kind == GameOver {
			return errors.New("native failure reload has no state")
		}
		return nil
	}
	if v.Stage < 1 || v.Stage > 2 || v.Slot < 0 || v.Slot > 5 || event.Kind == SceneComplete || (v.Stage == 2) != (event.Kind == GameOver) || event.Kind == GameOver && event.Value != v.Slot {
		return errors.New("invalid native failure continuation")
	}
	return nil
}

func observeGameOver(ctx context.Context, module api.Module, event Event) (*GameOverState, error) {
	if module.ExportedFunction("wb_game_over_info") == nil {
		return nil, validateGameOver(nil, event)
	}
	address, err := call(ctx, module, "wb_game_over_info")
	if err != nil {
		return nil, err
	}
	data, ok := module.Memory().Read(uint32(address), 12)
	if !ok || binary.LittleEndian.Uint32(data) != 1 {
		return nil, errors.New("invalid native failure observation")
	}
	stage := int(binary.LittleEndian.Uint32(data[4:]))
	slot := int(binary.LittleEndian.Uint32(data[8:]))
	if stage == 0 {
		if slot != 0 {
			return nil, errors.New("inactive native failure retained a file slot")
		}
		return nil, validateGameOver(nil, event)
	}
	v := &GameOverState{Stage: stage, Slot: slot}
	return v, validateGameOver(v, event)
}
