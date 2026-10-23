// Package syncwire contains only this game's input encoding and consistency
// checks. Rules and original AI remain in engine; no game state is inferred.
package syncwire

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
)

const MaxFrames = 120
const maxFrame = uint64(1<<53 - 1)

// InputWindow runs are [count, held4, priorHeld4, pendingPressed4, automation].
// Four byte-sized controller masks occupy one exact uint32 JSON number.
// A run is lossless: pending short taps and controller handoffs split it.
type InputWindow struct {
	From uint64     `json:"from"`
	Runs [][]uint64 `json:"runs"`
}

func buttons(b [4]engine.Buttons) uint64 {
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24
}
func masks(n uint64) (b [4]engine.Buttons) {
	for i := range b {
		b[i] = engine.Buttons(n >> (i * 8))
	}
	return
}
func Pack(inputs []engine.RecordedVideoInput) (InputWindow, error) {
	w := InputWindow{Runs: [][]uint64{}}
	if len(inputs) > MaxFrames {
		return w, fmt.Errorf("too many input frames")
	}
	for i, input := range inputs {
		if input.Frame == 0 || input.Frame > maxFrame || i > 0 && input.Frame != inputs[i-1].Frame+1 {
			return InputWindow{}, fmt.Errorf("input frame sequence is invalid")
		}
		if i == 0 {
			w.From = input.Frame
		}
		auto := uint64(0)
		for actor, enabled := range input.Automated {
			if enabled {
				auto |= 1 << actor
			}
		}
		next := []uint64{1, buttons(input.Held), buttons(input.PriorHeld), buttons(input.PendingPressed), auto}
		if n := len(w.Runs); n > 0 {
			old := w.Runs[n-1]
			if old[1] == next[1] && old[2] == next[2] && old[3] == next[3] && old[4] == next[4] {
				old[0]++
				continue
			}
		}
		w.Runs = append(w.Runs, next)
	}
	return w, nil
}
func Unpack(w InputWindow) ([]engine.RecordedVideoInput, error) {
	if len(w.Runs) == 0 {
		if w.From != 0 {
			return nil, fmt.Errorf("empty input window has a frame")
		}
		return []engine.RecordedVideoInput{}, nil
	}
	if w.From == 0 || w.From > maxFrame || len(w.Runs) > MaxFrames {
		return nil, fmt.Errorf("invalid input window")
	}
	total := uint64(0)
	for _, run := range w.Runs {
		if len(run) != 5 || run[0] == 0 || run[0] > MaxFrames || run[1] > 1<<32-1 || run[2] > 1<<32-1 || run[3] > 1<<32-1 || run[4] > 15 {
			return nil, fmt.Errorf("invalid packed input")
		}
		total += run[0]
		if total > MaxFrames || w.From+total-1 > maxFrame {
			return nil, fmt.Errorf("input window exceeds limit")
		}
	}
	out := make([]engine.RecordedVideoInput, 0, int(total))
	frame := w.From
	for _, run := range w.Runs {
		entry := engine.RecordedVideoInput{Held: masks(run[1]), PriorHeld: masks(run[2]), PendingPressed: masks(run[3])}
		for i := range entry.Automated {
			entry.Automated[i] = run[4]&(1<<i) != 0
		}
		for n := uint64(0); n < run[0]; n++ {
			entry.Frame = frame
			out = append(out, entry)
			frame++
		}
	}
	return out, nil
}

// Physical latches may be cleared between video frames. The next recorded
// input supplies both arrays explicitly, so excluding ONLY these transport
// latches avoids a false divergence after every device cancellation.
func RuntimeDigest(state engine.RuntimeState) (string, error) {
	state.Held = [4]engine.Buttons{}
	state.PendingPressed = [4]engine.Buttons{}
	data, e := json.Marshal(state)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Latches cover explicit cancellations accepted between video boundaries.
// They are input state, not a second transfer of world/AI/display data.
func PackLatches(state engine.RuntimeState) [2]uint32 {
	return [2]uint32{uint32(buttons(state.Held)), uint32(buttons(state.PendingPressed))}
}
func ApplyLatches(state engine.RuntimeState, packed [2]uint32) engine.RuntimeState {
	state.Held = masks(uint64(packed[0]))
	state.PendingPressed = masks(uint64(packed[1]))
	return state
}
