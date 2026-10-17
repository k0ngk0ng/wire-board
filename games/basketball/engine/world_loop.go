package engine

import "fmt"

// WorldLoopMemory contains only input state; the authoritative actors and AI
// strategy live in Preparation, not a duplicate embedded WorldInputState.
type WorldLoopMemory struct {
	Memory        [4]InputMemory
	Held, Pressed [4]Buttons
	Human         [2]HumanCommandState
}

type WorldLoopState struct {
	// Last GENERATED column, not Camera.TileColumn's initialization loop-end.
	// Phase is the BCFF category: 0 initializes, 1 is the nonzero world phase.
	// Immutable pointer: a new generation replaces it, never mutates input.
	LaunchColumn     *CourtColumnCommit `json:"launchColumn,omitempty"`
	Preparation      WorldPreparationState
	Input            WorldLoopMemory
	Camera           CameraState
	Projection       ProjectionState
	LogicFrame       uint8
	Started          bool
	Updates          uint64
	DisplayedCameraY uint16
}

type WorldLoopContext struct {
	PursuitReserved                                        *PursuitReservedBytes
	PursuitAlias74                                         *PursuitAlias74
	Actions                                                WorldActionContext
	ObjectKinds                                            [13]uint8
	HoopLinks                                              [6]uint8
	AttackGrades, JumpEnhancements                         [4]uint8
	HumanTactics                                           [2]uint8
	Difficulty, BackboardStrength, LayoutID, TrackedObject uint8
}

type WorldLoopInput struct {
	Frames [4]ControllerFrame
	// PhysicalControllers preserves ports before the actor assignment map.
	// Nil is unknown; static geometry does not require this optional source.
	PhysicalControllers *[4]ControllerFrame
}

// HUDWorldSnapshot is taken at D326, before the current actor inputs. A
// separate video-clock driver supplies its own interrupt ticks and caches;
// logical updates here are never treated as video frames or elapsed seconds.
type HUDWorldSnapshot struct {
	LogicFrame uint8
	Scores     [2]uint16
	Tactics    [4]uint8
}

type WorldLoopStep struct {
	MotionSurfaceWrites []MotionSurfaceWrite
	State               WorldLoopState
	ActionCalls         []uint8
	SoundIDs            []uint8
	HUD                 *HUDWorldSnapshot
	StreamColumn        bool
}

func worldLoopPositions(s WorldActionState) [13]Position {
	var positions [13]Position
	for i, a := range s.Actors {
		positions[i] = a.Player.Body.Position
	}
	for i, a := range s.Auxiliary {
		positions[i+4] = a.Body.Position
	}
	positions[6] = s.Ball.Body.Position
	for i, h := range s.Hoops {
		positions[i+7] = h.Position
	}
	return positions
}
func worldLoopPoses(s WorldActionState) [13]uint8 {
	var result [13]uint8
	encode := func(p Pose) uint8 {
		value := p.Frame
		if p.Mirrored {
			value |= 128
		}
		return value
	}
	for i, a := range s.Actors {
		result[i] = encode(a.Player.Pose)
	}
	for i, a := range s.Auxiliary {
		result[i+4] = encode(a.Pose)
	}
	result[6] = encode(s.Ball.Pose)
	for i, h := range s.Hoops {
		result[i+7] = encode(h.Pose)
	}
	return result
}

func resetOriginalWorldDrawList(s ProjectionState) ProjectionState {
	// Renderer finalization D063 leaves slot thirteen untouched.
	for i := 0; i < 12; i++ {
		s.DrawOrder[i] = 255
	}
	s.DrawCount = 0
	return s
}

// StepWorldLoop advances the restored playing-world core from one persistent
// state. The initial checkpoint is D32C, so its first update starts at inputs.
// Later iterations restore the main-loop RNG/scroll boundary and all recovered
// preparation/input/action/projection phases. Match timing remains external.
func StepWorldLoop(original WorldLoopState, input WorldLoopInput, ctx WorldLoopContext) (WorldLoopStep, error) {
	ctx.ObjectKinds = worldObjectKinds(original.Preparation.Actions, ctx.ObjectKinds)
	initial := WorldLoopStep{State: original}
	result := initial
	s := &result.State
	if ctx.Actions.CourtID == 5 && (s.LaunchColumn == nil || s.LaunchColumn.Column >= 64 || s.LaunchColumn.Phase > 1) {
		return initial, fmt.Errorf("court5 requires its last generated background column")
	}
	if ctx.TrackedObject >= 13 || ctx.LayoutID >= 8 {
		return initial, fmt.Errorf("invalid original world presentation context")
	}
	if s.Started {
		s.LogicFrame++
		s.Camera.DisplayedX = s.Camera.X
		s.DisplayedCameraY = s.Camera.Y
		s.Projection = resetOriginalWorldDrawList(s.Projection)
		// CAA4 consumes this shared register once, independently of all AI
		// and physical outcomes. No RNG value is read from expected snapshots.
		s.Preparation.Actions.Random, _ = NextRandom(s.Preparation.Actions.Random)
		camera, err := StepOriginalCamera(s.Camera, CameraContext{TrackedObject: ctx.TrackedObject, BallHolder: uint8(s.Preparation.Actions.Holder), Positions: worldLoopPositions(s.Preparation.Actions), Projected: s.Projection.Projected})
		if err != nil {
			return initial, fmt.Errorf("camera: %w", err)
		}
		s.Camera, result.StreamColumn = camera.State, camera.StreamColumn
		if camera.StreamColumn && s.LaunchColumn != nil {
			s.LaunchColumn = &CourtColumnCommit{Column: camera.State.TileColumn, Phase: 1}
		}
		pc := WorldPreparationContext{Actions: ctx.Actions, AttackGrades: ctx.AttackGrades, HumanTactics: ctx.HumanTactics, BackboardStrength: ctx.BackboardStrength, PhysicalControllers: input.PhysicalControllers}
		copy(pc.Kinds[:], ctx.ObjectKinds[:7])
		for i := range pc.Suppressed {
			pc.Suppressed[i] = uint16(s.Projection.Projected[i].X)>>8 != 0
		}
		for i, h := range s.Input.Human {
			pc.HumanCommands[i], pc.HumanStatus[i] = h.Request, h.Status
		}
		prepared, err := StepWorldPreparation(s.Preparation, pc)
		if err != nil {
			return initial, fmt.Errorf("preparation: %w", err)
		}
		s.Preparation = prepared.State
		result.SoundIDs = append(result.SoundIDs, prepared.SoundIDs...)
		result.HUD = &HUDWorldSnapshot{LogicFrame: s.LogicFrame, Scores: s.Preparation.Scores}
		for team, h := range s.Input.Human {
			result.HUD.Tactics[team*2], result.HUD.Tactics[team*2+1] = h.Messages[0], h.Messages[1]
		}
		var projected [4]ScreenPosition
		copy(projected[:], s.Projection.Projected[:4])
		s.Preparation.DisplayFlags = OriginalCrowdingFlags(s.Preparation.DisplayFlags, projected)
	}
	is := WorldInputState{World: s.Preparation.Actions, Team: s.Preparation.TeamAI, Memory: s.Input.Memory, Held: s.Input.Held, Pressed: s.Input.Pressed, Human: s.Input.Human}
	ic := WorldInputContext{PursuitReserved: ctx.PursuitReserved, PursuitAlias74: ctx.PursuitAlias74, World: ctx.Actions, Frames: input.Frames, Difficulty: ctx.Difficulty, ScoreDifference: s.Preparation.ScoreDifference, CachedZones: s.Preparation.Zones, ObjectKinds: ctx.ObjectKinds, HoopLinks: ctx.HoopLinks, HumanTactics: ctx.HumanTactics, JumpEnhancements: ctx.JumpEnhancements}
	copy(ic.PickupFlags[:], s.Preparation.Contacts.Marks[:4])
	inputs, err := StepWorldInputs(is, ic)
	if err != nil {
		return initial, fmt.Errorf("inputs: %w", err)
	}
	s.Preparation.Actions, s.Preparation.TeamAI = inputs.State.World, inputs.State.Team
	s.Input = WorldLoopMemory{Memory: inputs.State.Memory, Held: inputs.State.Held, Pressed: inputs.State.Pressed, Human: inputs.State.Human}
	for i, change := range inputs.ChangePickupFlags {
		if change {
			s.Preparation.Contacts.Marks[i] = inputs.PickupFlags[i]
		}
	}
	actionContext := ctx.Actions
	if s.LaunchColumn != nil {
		height, err := OriginalBackgroundLaunchHeight(ctx.Actions.CourtID, s.LaunchColumn.Column, s.LaunchColumn.Phase)
		if err != nil {
			return initial, err
		}
		actionContext.BackgroundLaunchHeight = &height
	}
	var pickupFlags [4]uint8
	copy(pickupFlags[:], s.Preparation.Contacts.Marks[:4])
	actionContext.PickupFlags = &pickupFlags
	jumpEnhancements := ctx.JumpEnhancements
	actionContext.JumpEnhancements = &jumpEnhancements
	actionContext.ActorHeldButtons = s.Input.Held
	actionContext.ActorPressedButtons = &s.Input.Pressed
	actionContext.CachedZones = s.Preparation.Zones
	actionContext.BallShotInputButtons = s.Input.Pressed[2]
	actionContext.CameraShake = s.Camera.Shake
	actions, err := StepWorldActions(s.Preparation.Actions, actionContext)
	if err != nil {
		return initial, fmt.Errorf("actions: %w", err)
	}
	s.Preparation.Actions = actions.State
	if actions.ChangeCameraShake {
		s.Camera.Shake = actions.CameraShake
	}
	result.ActionCalls = actions.Calls
	result.MotionSurfaceWrites = actions.MotionSurfaceWrites
	result.SoundIDs = append(result.SoundIDs, actions.SoundIDs...)
	if actions.ChangeSelectedBoard {
		s.Preparation.SelectedBoard = actions.SelectedBoard
	}
	projection, err := StepOriginalProjection(s.Projection, ProjectionContext{CameraX: s.Camera.X, CameraY: s.Camera.Y, LayoutID: ctx.LayoutID, Poses: worldLoopPoses(s.Preparation.Actions), Positions: worldLoopPositions(s.Preparation.Actions)})
	if err != nil {
		return initial, fmt.Errorf("projection: %w", err)
	}
	s.Projection = projection
	s.Started = true
	s.Updates++
	return result, nil
}
