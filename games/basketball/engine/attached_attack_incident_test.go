package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestAttachedAttackFamilySupportsRecoveredMotion(t *testing.T) {
	raw, err := os.ReadFile("testdata/attached-attack-incident.json")
	if err != nil {
		t.Fatal(err)
	}
	var incident RuntimeState
	if err = json.Unmarshal(raw, &incident); err != nil {
		t.Fatal(err)
	}
	for action := Action(0x60); action <= 0x68; action++ {
		for team := uint8(0); team < 8; team++ {
			for character := uint8(0); character < 4; character++ {
				for _, computer := range []bool{false, true} {
					t.Run(fmt.Sprintf("%02x-%d-%d-%t", action, team, character, computer), func(t *testing.T) {
						result := WorldActionStep{State: incident.Playing.World.Preparation.Actions}
						ctx := incident.Playing.Context.Actions
						ctx.ActorPressedButtons = &[4]Buttons{}
						ctx.Teams[0], ctx.Characters[0] = team, character
						ctx.Controls[0] = 0
						if computer {
							ctx.Controls[0] = 128
						}
						player := &result.State.Actors[0].Player
						player.Action, player.Entered, player.Request.Pending = action, false, false
						if err = advanceWorldPlayer(&result, 0, ctx, NewYorkGroundCourt()); err != nil {
							t.Fatal(err)
						}
						if result.State.PlayerLinks[0]&128 != 0 {
							support := result.State.Auxiliary[1].Body.Position
							if player.Body.Position.X&^255 != support.X&^255 || player.Body.Position.Depth&^255 != support.Depth&^255 {
								t.Fatal("attached attack did not follow its original support", player.Body.Position, support)
							}
						}
					})
				}
			}
		}
	}
}

func TestAttachedFreeActionsKeepRecoveredSupport(t *testing.T) {
	raw, err := os.ReadFile("testdata/attached-attack-incident.json")
	if err != nil {
		t.Fatal(err)
	}
	var incident RuntimeState
	if err = json.Unmarshal(raw, &incident); err != nil {
		t.Fatal(err)
	}
	for _, action := range []Action{ActionIdle, ActionWalk, ActionRun, ActionBrake, ActionLand, ActionFalling} {
		t.Run(fmt.Sprintf("%02x", action), func(t *testing.T) {
			result := WorldActionStep{State: incident.Playing.World.Preparation.Actions}
			player := &result.State.Actors[0].Player
			player.Action, player.Entered, player.Request.Pending = action, false, false
			if err = advanceWorldPlayer(&result, 0, incident.Playing.Context.Actions, NewYorkGroundCourt()); err != nil {
				t.Fatal(err)
			}
			support := result.State.Auxiliary[1].Body.Position
			if player.Body.Position.X&^255 != support.X&^255 || player.Body.Position.Depth&^255 != support.Depth&^255 {
				t.Fatal("free action lost its original attached support", player.Body.Position, support)
			}
		})
	}
}

func TestFreeActionsPreserveAlreadyRejectedOriginalRequests(t *testing.T) {
	for _, action := range []Action{ActionIdle, ActionWalk, ActionRun, ActionBrake, ActionLand, ActionFalling} {
		for request := Action(0); request < 106; request++ {
			player := Player{Action: action, Heading: HeadingNone, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}, Request: ActionRequest{Action: request, Pending: true}}
			resolved, err := ResolvePlayerRequest(player)
			if err != nil {
				t.Fatal(err)
			}
			if !resolved.Request.Pending {
				continue
			}
			step, err := StepFreePlayerAction(resolved, FreePlayerContext{RequestResolved: true}, KunioProfile(), NewYorkGroundCourt())
			if err != nil {
				t.Fatalf("rejected %02x -> %02x stopped a legal current action: %v", action, request, err)
			}
			if step.Player.Request != player.Request {
				t.Fatalf("rejected %02x -> %02x was consumed again", action, request)
			}
		}
	}
}

func TestAttachedAttackIncidentContinues(t *testing.T) {
	raw, err := os.ReadFile("testdata/attached-attack-incident.json")
	if err != nil {
		t.Fatal(err)
	}
	var state RuntimeState
	if err = json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	start := state.VideoFrames
	for frame := 0; frame < 600; frame++ {
		step, stepErr := StepVideo(state, [4]Buttons{})
		if stepErr != nil {
			t.Fatalf("video %d: %v", state.VideoFrames+1, stepErr)
		}
		state = step.State
		if frame%37 == 0 {
			encoded, marshalErr := json.Marshal(state)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if err = json.Unmarshal(encoded, &state); err != nil {
				t.Fatal(err)
			}
		}
	}
	if state.VideoFrames != start+600 {
		t.Fatal("incident did not advance", start, state.VideoFrames)
	}
}
