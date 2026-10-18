// Replay a retained runtime independently; output complete logical worlds.
// This diagnostic does not alter source state, clock, controls or game outcomes.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	e "github.com/k0ngk0ng/wire-board/games/basketball/engine"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--cold-origin" {
		data, err := os.ReadFile(os.Args[2])
		if err != nil {
			panic(err)
		}
		var retained e.RuntimeState
		if err = json.Unmarshal(data, &retained); err != nil {
			panic(err)
		}
		s, err := e.NewRuntime(retained.Config)
		if err != nil {
			panic(err)
		}
		for s.Playing == nil && s.VideoFrames < 3000 {
			step, err := e.StepVideo(s, [4]e.Buttons{})
			if err != nil {
				panic(err)
			}
			s = step.State
		}
		if s.Playing == nil {
			panic("cold initialization did not reach playing")
		}
		if err = json.NewEncoder(os.Stdout).Encode(s); err != nil {
			panic(err)
		}
		return
	}
	if len(os.Args) != 3 {
		panic("usage: replay_world_segment origin.json world-count")
	}
	count, err := strconv.Atoi(os.Args[2])
	if err != nil || count < 1 {
		panic("world-count must be positive")
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var s e.RuntimeState
	if err = json.Unmarshal(data, &s); err != nil {
		panic(err)
	}
	if s.Playing == nil {
		panic("origin is not a playing world")
	}
	if err = e.ValidateRuntimeState(s); err != nil {
		panic(err)
	}
	output := json.NewEncoder(os.Stdout)
	if err = output.Encode(s.Playing.World); err != nil {
		panic(err)
	}
	written := 1
	for frames := 0; written < count && frames < count*8; frames++ {
		step, err := e.StepVideo(s, [4]e.Buttons{})
		if err != nil {
			panic(err)
		}
		s = step.State
		if step.LogicAdvanced && s.Playing != nil {
			if err = output.Encode(s.Playing.World); err != nil {
				panic(err)
			}
			written++
		}
	}
	if written != count {
		panic(fmt.Sprintf("segment left playing phase: %d of %d worlds", written, count))
	}
}
