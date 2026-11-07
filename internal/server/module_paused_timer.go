package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Freeze the control envelope before invoking the callback. Serializing first
// also detects in-place mutation of aliased actor slices or other metadata.
// The caller already owns a detached room, so failure cannot alter live state.
func runPausedPresentationTimer(module gameapi.Module, ctx gameapi.Context, before gameapi.Match) (after gameapi.Match, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			after = gameapi.Match{}
			err = fmt.Errorf("paused presentation timer panicked: %v", recovered)
		}
	}()
	if !module.Realtime || module.OnPausedTimer == nil || !before.Paused || before.Result != nil {
		return before, errors.New("missing paused presentation clock capability")
	}
	control := func(match gameapi.Match) ([]byte, error) {
		match.Data = nil
		match.SystemDeadline = 0
		match.CheckpointKey = ""
		return json.Marshal(match)
	}
	frozen, err := control(before)
	if err != nil {
		return before, err
	}
	after, err = module.OnPausedTimer(ctx, before)
	if err != nil {
		return before, err
	}
	updated, err := control(after)
	if err != nil {
		return before, err
	}
	if !bytes.Equal(frozen, updated) {
		return before, errors.New("paused presentation timer changed control metadata")
	}
	return after, nil
}
