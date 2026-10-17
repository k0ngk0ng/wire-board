// verify-player replays raw logical button samples through input interpretation
// and action updates. Go carries requests, tap windows, positions, velocities
// and animations forward; original action requests are no longer injected.
// This is an offline acceptance executable, not a playable game frontend.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
)

type rawPlayer struct {
	Action, Request, Facing, Pose, Clock, Cursor, RepeatCount, NextAction int32
	Air, Heading, X, Depth, Height, VX, VY, VZ, Gravity                   int32
}

type rawInputState struct {
	Request, Heading, TapTimer, LastTap, AttackTapTimer, LastAttackTap, ButtonDelay, PendingButtons uint8
}
type rawInput struct {
	Scenario                         string
	Frame                            int
	Held, Pressed, Action, Pose, Air uint8
	Before, After                    rawInputState
}

func memoryOf(r rawInputState) engine.InputMemory {
	return engine.InputMemory{TapWindow: r.TapTimer, LastTap: engine.Buttons(r.LastTap), AttackTapWindow: r.AttackTapTimer, LastAttackTap: engine.Buttons(r.LastAttackTap), ButtonDelay: r.ButtonDelay, PendingButtons: engine.Buttons(r.PendingButtons)}
}

func project(r rawPlayer) engine.Player {
	heading := engine.HeadingNone
	if r.Heading != 255 {
		heading = engine.Heading(r.Heading / 32)
	}
	next := engine.ActionIdle
	if r.NextAction < 128 {
		next = engine.Action(r.NextAction)
	}
	return engine.Player{
		Body:   engine.Body{Position: engine.Position{X: engine.Fixed(r.X << 8 >> 8), Depth: engine.Fixed(r.Depth), Height: engine.Fixed(r.Height << 8 >> 8)}, Velocity: engine.Velocity{X: engine.Fixed(int16(r.VX)), Depth: engine.Fixed(int16(r.VY)), Height: engine.Fixed(int16(r.VZ))}, Gravity: engine.Fixed(int16(r.Gravity))},
		Action: engine.Action(r.Action & 127), Entered: r.Action&128 != 0,
		Request:    engine.ActionRequest{Action: engine.Action(r.Request & 127), Pending: r.Request&128 == 0},
		FacingLeft: r.Facing == 128, Pose: engine.Pose{Frame: uint8(r.Pose & 127), Mirrored: r.Pose&128 != 0},
		Animation: engine.AnimationState{Cursor: uint8(r.Cursor), Elapsed: uint8(r.Clock), Loops: uint8(r.RepeatCount), NextAction: next, HasNextAction: r.NextAction < 128},
		Airborne:  r.Air&128 != 0, Heading: heading,
	}
}

func run() error {
	file := flag.String("fixture", "games/basketball/engine/testdata/free-player.json", "original action-phase fixture")
	inputFile := flag.String("inputs", "games/basketball/engine/testdata/input.json", "original raw controller samples")
	flag.Parse()
	data, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	var rows []struct {
		Scenario      string
		Frame         int
		Before, After rawPlayer
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("empty replay")
	}
	inputData, err := os.ReadFile(*inputFile)
	if err != nil {
		return err
	}
	var inputRows []rawInput
	if err = json.Unmarshal(inputData, &inputRows); err != nil {
		return err
	}
	inputs := map[string][]rawInput{}
	for _, r := range inputRows {
		inputs[r.Scenario] = append(inputs[r.Scenario], r)
	}
	indices := map[string]int{}
	var current engine.ControlledPlayer
	last := ""
	scenarios := 0
	h := sha256.New()
	inputHash := sha256.New()
	profile, court := engine.KunioProfile(), engine.NewYorkGroundCourt()
	for _, r := range rows {
		before := project(r.Before)
		i := indices[r.Scenario]
		if i >= len(inputs[r.Scenario]) {
			return fmt.Errorf("missing controller sample for %s update %d", r.Scenario, i)
		}
		input := inputs[r.Scenario][i]
		indices[r.Scenario]++
		if r.Scenario != last {
			current.Player = before
			current.Player.Request = engine.ActionRequest{Action: engine.Action(input.Before.Request & 127), Pending: input.Before.Request&128 == 0}
			current.Player.Heading = engine.HeadingNone
			if input.Before.Heading != 255 {
				current.Player.Heading = engine.Heading(input.Before.Heading / 32)
			}
			current.Input = memoryOf(input.Before)
			last = r.Scenario
			scenarios++
		}
		if current.Player.Action != engine.Action(input.Action&127) || current.Player.Pose.Mirrored != (input.Pose&128 != 0) || current.Player.Airborne != (input.Air&128 != 0) {
			return fmt.Errorf("%s frame %d: input context diverged", r.Scenario, r.Frame)
		}
		current, err = engine.StepControlledFreePlayer(current, engine.ControllerFrame{Held: engine.Buttons(input.Held), Pressed: engine.Buttons(input.Pressed)}, profile, court)
		if err != nil {
			return err
		}
		if current.Input != memoryOf(input.After) {
			return fmt.Errorf("%s frame %d: controller memory diverged: got %+v want %+v", r.Scenario, r.Frame, current.Input, memoryOf(input.After))
		}
		if want := project(r.After); current.Player != want {
			return fmt.Errorf("%s frame %d: replay diverged\nGo: %+v\nOriginal: %+v", r.Scenario, r.Frame, current, want)
		}
		encoded, err := json.Marshal(current.Player)
		if err != nil {
			return err
		}
		h.Write(encoded)
		encoded, err = json.Marshal(current.Input)
		if err != nil {
			return err
		}
		inputHash.Write(encoded)
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "passed", "scenarios": scenarios, "updates": len(rows), "state_sha256": hex.EncodeToString(h.Sum(nil)), "controller_sha256": hex.EncodeToString(inputHash.Sum(nil)), "position_from_go": true, "animation_from_go": true, "requests_from_go": true, "scope": "unpossessed player; only held/pressed logical buttons come from the original recording"})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
