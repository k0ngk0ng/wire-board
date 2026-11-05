package engine

import "errors"

type ChuActionParameters struct {
	ChuTailParameters
	WalkSpeed       int32                          `json:"walk_speed"`
	MovementSources map[uint32]FighterMovementSpec `json:"-"`
}

// ChuActionState is the union of state owned by the complete action routine.
// The tail keeps the energy high word; EnergyFraction retains its low word for
// continuous charging and special motion. Children are not stepped here.
type ChuActionState struct {
	ChuTailState
	EnergyFraction         uint16
	HitStopTicks, Shake    uint8
	ScreenOffsetX          int16
	StatusDelay            uint8
	Residual               int32
	StunTicks              uint16
	TargetBit              uint8
	Power                  int16
	AlternateVelocity      int32
	MovementSource         uint32
	DragonBankCursor       uint16
	Children               [3]ChuSpecialChild
	ChildExtras            [3]Fighter4ChildExtra
	Fighter5ChildExtras    [3]Fighter5ChildExtra
	EffectExtras           [16]ReactionEffectExtra
	GlobalF568, GlobalF56A uint16
	Left, Right            int16
}

// StepChuAction recovers 0x15a18 through its actual return at 0x15b82. Input
// preparation still runs during hit-stop. Each dispatched action consumes the
// newly advanced clock exactly once, then enters the complete shared tail.
// Input latching, AI, child objects and other world scheduling are separate.
func StepChuAction(state ChuActionState, parameters ChuActionParameters, commands CommandLibrary, animations AnimationLibrary) (ChuActionState, error) {
	return StepFighterAction(state, 9, parameters, commands, animations)
}

// StepFighterAction keeps one copy of the complete action driver for the
// restored sources0/1/2/3/4/5/6/8/9/10. AlternateVelocity remains the sole +48 long owner.
func StepFighterAction(state ChuActionState, characterID uint8, parameters ChuActionParameters, commands CommandLibrary, animations AnimationLibrary) (ChuActionState, error) {
	original := state
	if characterID != 0 && characterID != 1 && characterID != 2 && characterID != 3 && characterID != 4 && characterID != 5 && characterID != 6 && characterID != 8 && characterID != 9 && characterID != 10 {
		return original, errors.New("unsupported complete fighter action source")
	}
	var err error
	parameters, err = actionMovementParameters(parameters, state.MovementSource)
	if err != nil {
		return original, err
	}
	command := CommandState{RawHeld: state.Controls.RawHeld, Facing: state.Body.Facing, Request: state.Body.Request}
	copy(command.Timeouts[:], state.Body.Commands[:8])
	copy(command.Indices[:], state.Body.Commands[8:])
	command, err = PrepareCharacterCommands(characterID, command, commands)
	if err != nil {
		return original, err
	}
	state.Body.Request = command.Request
	copy(state.Body.Commands[:8], command.Timeouts[:])
	copy(state.Body.Commands[8:], command.Indices[:])
	preparation := PrepareFighterInput(FighterPreparationState{Energy: state.fullActionEnergy(), Held: uint8(state.Controls.Held),
		Pressed: uint8(state.Controls.Pressed), ExtendedPressed: uint8(state.Controls.Pressed >> 8), Facing: state.Body.Facing,
		ChargeTicks: state.ChargeTicks, Request: state.Body.Request})
	state.Body.Request, state.ChargeTicks = preparation.Request, preparation.ChargeTicks
	prelude, dispatch := PrepareActionStep(ActionPreludeState{Body: state.Body, HitStopTicks: state.HitStopTicks, Shake: state.Shake,
		ScreenOffsetX: state.ScreenOffsetX, StatusDelay: state.StatusDelay, Residual: state.Residual, Occupied: state.OccupiedLanes})
	state.Body, state.HitStopTicks, state.Shake = prelude.Body, prelude.HitStopTicks, prelude.Shake
	state.ScreenOffsetX, state.StatusDelay, state.Residual, state.OccupiedLanes = prelude.ScreenOffsetX, prelude.StatusDelay, prelude.Residual, prelude.Occupied
	mode := ChuTailHitStop
	if dispatch {
		mode = ChuTailNormal
		switch state.Body.ActionKind {
		case 0:
			body, pool, _, actionErr := StepGroundIdle(state.Body, state.Pool, parameters.JumpParameters, state.Controls.Held)
			if actionErr != nil {
				return original, actionErr
			}
			state.Body, state.Pool = body, pool
		case 1:
			motion := StepWalk(MotionState{X: state.Body.X, Y: state.Body.Y, Ground: state.Body.Ground, Facing: state.Body.Facing}, parameters.WalkSpeed)
			state.Body.X, state.Body.Y = motion.X, motion.Y
		case 2:
			body, pool, _, _, actionErr := StepAirAction(state.Body, state.Pool)
			if actionErr != nil {
				return original, actionErr
			}
			state.Body, state.Pool = body, pool
		case 3, 4, 5, 6, 7, 8, 9, 12, 13:
			reaction, actionErr := StepReactionMotion(ReactionMotionState{ActorID: state.ObjectID, Body: state.Body, Controls: state.Controls,
				GuardMask: state.GuardMask, StunTicks: state.StunTicks, Charge: state.fullActionEnergy(), Pool: state.Pool,
				EffectExtras: state.EffectExtras, OccupiedLanes: state.OccupiedLanes, PendingSound: state.PendingSound,
				GlobalF568: state.GlobalF568, GlobalF56A: state.GlobalF56A, Left: state.Left, Right: state.Right}, parameters.JumpParameters)
			if actionErr != nil {
				return original, actionErr
			}
			state.Body, state.GuardMask, state.StunTicks = reaction.Body, reaction.GuardMask, reaction.StunTicks
			state.setActionEnergy(reaction.Charge)
			state.Pool, state.EffectExtras, state.OccupiedLanes = reaction.Pool, reaction.EffectExtras, reaction.OccupiedLanes
			state.PendingSound, state.GlobalF568, state.GlobalF56A = reaction.PendingSound, reaction.GlobalF568, reaction.GlobalF56A
		case 10, 11:
			body, pool, occupied, _, _, actionErr := StepLaneAction(state.Body, state.Pool, state.OccupiedLanes)
			if actionErr != nil {
				return original, actionErr
			}
			state.Body, state.Pool, state.OccupiedLanes = body, pool, occupied
		case 0x0e, 0x12, 0x16, 0x17, 0x18, 0x19:
			if characterID == 4 && (state.Body.ActionKind == 0x0e || state.Body.ActionKind == 0x12) {
				if actionErr := stepFighter4ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if characterID == 6 && state.Body.ActionKind == 0x0e {
				if actionErr := stepFighter6ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if characterID == 3 && state.Body.ActionKind == 0x17 {
				if actionErr := stepFighter3ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if characterID != 0 {
				return original, errors.New("unsupported fighter action source/slot")
			}
			if actionErr := stepFighter0ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
				return original, actionErr
			}
		case 0x13, 0x21, 0x23, 0x24:
			if characterID == 6 && state.Body.ActionKind == 0x13 {
				if actionErr := stepFighter6ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if characterID != 3 {
				return original, errors.New("unsupported fighter action source/slot")
			}
			if actionErr := stepFighter3ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
				return original, actionErr
			}
		case 0x22:
			if characterID != 3 {
				return original, errors.New("unsupported fighter action source/slot")
			}
			next, actionErr := StepFighter3AirMotion(state.specialMotionView(), parameters.JumpParameters)
			if actionErr != nil {
				return original, actionErr
			}
			state.acceptSpecialMotion(next)
		case 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c:
			if characterID != 4 {
				return original, errors.New("unsupported fighter action source/slot")
			}
			if actionErr := stepFighter4ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
				return original, actionErr
			}
		case 0x2e, 0x2f, 0x30, 0x31, 0x32:
			if characterID != 5 {
				return original, errors.New("unsupported fighter action source/slot")
			}
			if actionErr := stepFighter5ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
				return original, actionErr
			}
		case 0x1e, 0x1f, 0x20:
			if characterID != 2 {
				return original, errors.New("unsupported fighter action source/slot")
			}
			if actionErr := stepFighter2ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
				return original, actionErr
			}
		case 0x1a, 0x1b, 0x1c, 0x1d:
			if characterID != 1 {
				return original, errors.New("unsupported fighter action source/slot")
			}
			if actionErr := stepFighter1ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
				return original, actionErr
			}
		case 0x3e, 0x3f, 0x40:
			if characterID != 8 {
				return original, errors.New("unsupported fighter action source/slot")
			}
			if actionErr := stepFighter8ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
				return original, actionErr
			}
		case 0x33, 0x34, 0x35, 0x36, 0x37, 0x38:
			if characterID != 6 {
				return original, errors.New("unsupported fighter action source/slot")
			}
			if actionErr := stepFighter6ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
				return original, actionErr
			}
		case 15, 20, 21, 65, 66, 67, 68, 69, 70:
			if characterID == 6 && state.Body.ActionKind == 15 {
				if actionErr := stepFighter6ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if characterID == 1 && state.Body.ActionKind == 15 {
				if actionErr := stepFighter1ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if characterID == 8 && state.Body.ActionKind == 15 {
				if actionErr := stepFighter8ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if characterID == 2 && state.Body.ActionKind == 15 {
				if actionErr := stepFighter2ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			special, actionErr := StepChuSpecialMotion(ChuSpecialMotionState{ObjectID: state.ObjectID, Body: state.Body, Controls: state.Controls,
				Energy: state.fullActionEnergy(), Power: state.Power, AlternateVelocity: state.AlternateVelocity, ContactFlags: state.ContactFlags,
				TargetBit: state.TargetBit, Children: state.Children, Pool: state.Pool}, parameters.JumpParameters)
			if actionErr != nil {
				return original, actionErr
			}
			state.Body, state.Pool, state.Children = special.Body, special.Pool, special.Children
			state.setActionEnergy(special.Energy)
			state.Power, state.AlternateVelocity, state.ContactFlags = special.Power, special.AlternateVelocity, special.ContactFlags
		case 0x11, 0x2d, 0x47, 0x48, 0x49:
			if characterID == 5 && (state.Body.ActionKind == 0x11 || state.Body.ActionKind == 0x2d) {
				if actionErr := stepFighter5ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if characterID == 6 && state.Body.ActionKind == 0x11 {
				if actionErr := stepFighter6ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if characterID == 1 && state.Body.ActionKind == 0x11 {
				if actionErr := stepFighter1ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if characterID == 8 && state.Body.ActionKind == 0x11 {
				if actionErr := stepFighter8ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if characterID == 2 && state.Body.ActionKind == 0x11 {
				if actionErr := stepFighter2ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if (characterID == 0 || characterID == 3) && state.Body.ActionKind == 0x11 {
				if actionErr := stepFighter0ActionMotion(&state, parameters.JumpParameters); actionErr != nil {
					return original, actionErr
				}
				break
			}
			if characterID != 10 {
				return original, errors.New("unsupported Chu action")
			}
			motion, actionErr := StepFighter10Motion(Fighter10MotionState{ObjectID: state.ObjectID, Body: state.Body,
				Controls: state.Controls, Aux48: state.AlternateVelocity, ContactFlags: state.ContactFlags, Pool: state.Pool}, parameters.JumpParameters)
			if actionErr != nil {
				return original, actionErr
			}
			state.Body, state.Pool, state.AlternateVelocity, state.ContactFlags = motion.Body, motion.Pool, motion.Aux48, motion.ContactFlags
		default:
			return original, errors.New("unsupported Chu action")
		}
	}
	// Child creation earlier in this same call changes the later availability
	// gate. The child's size is authoritative, rather than its old input copy.
	state.ChildSize = state.Children[0].Size
	parameters, err = actionMovementParameters(parameters, state.MovementSource)
	if err != nil {
		return original, err
	}
	tail, aux48, power, err := stepFighterActionTail(state.ChuTailState, characterID, state.AlternateVelocity, state.Power, state.EnergyFraction, parameters.ChuTailParameters, animations, mode)
	if err != nil {
		return original, err
	}
	state.ChuTailState, state.AlternateVelocity, state.Power = tail, aux48, power
	return state, nil
}

func actionMovementParameters(parameters ChuActionParameters, source uint32) (ChuActionParameters, error) {
	if parameters.MovementSources == nil {
		return parameters, nil
	}
	movement, found := parameters.MovementSources[source]
	if !found {
		return parameters, errors.New("current fighter movement source is unavailable")
	}
	parameters.WalkSpeed = movement.WalkSpeed
	parameters.ChuTailParameters = ChuTailParameters{JumpParameters: movement.JumpParameters, BurstSpeed: movement.BurstSpeed, EnergyDrain: movement.EnergyDrain}
	return parameters, nil
}

func stepFighter4ActionMotion(state *ChuActionState, parameters JumpParameters) error {
	next, err := StepFighter4Motion(Fighter4MotionState{ChuSpecialMotionState: state.specialMotionView(), MovementSource: state.MovementSource,
		DragonBankCursor: state.DragonBankCursor, ChildExtras: state.ChildExtras, EffectExtras: state.EffectExtras}, parameters)
	if err != nil {
		return err
	}
	state.acceptSpecialMotion(next.ChuSpecialMotionState)
	state.MovementSource, state.DragonBankCursor, state.ChildExtras, state.EffectExtras = next.MovementSource, next.DragonBankCursor, next.ChildExtras, next.EffectExtras
	return nil
}

func stepFighter5ActionMotion(state *ChuActionState, parameters JumpParameters) error {
	next, err := StepFighter5Motion(Fighter5MotionState{ChuSpecialMotionState: state.specialMotionView(), ChildExtras: state.Fighter5ChildExtras}, parameters)
	if err != nil {
		return err
	}
	state.acceptSpecialMotion(next.ChuSpecialMotionState)
	state.Fighter5ChildExtras = next.ChildExtras
	return nil
}

func stepFighter0ActionMotion(state *ChuActionState, parameters JumpParameters) error {
	next, err := StepFighter0Motion(Fighter0MotionState{ChuSpecialMotionState: state.specialMotionView(), PendingSound: state.PendingSound, EffectExtras: state.EffectExtras}, parameters)
	if err != nil {
		return err
	}
	state.acceptSpecialMotion(next.ChuSpecialMotionState)
	state.PendingSound, state.EffectExtras = next.PendingSound, next.EffectExtras
	return nil
}

func stepFighter3ActionMotion(state *ChuActionState, parameters JumpParameters) error {
	next, err := StepFighter3Motion(Fighter3MotionState{ChuSpecialMotionState: state.specialMotionView(), PendingSound: state.PendingSound, EffectExtras: state.EffectExtras}, parameters)
	if err != nil {
		return err
	}
	state.acceptSpecialMotion(next.ChuSpecialMotionState)
	state.PendingSound, state.EffectExtras = next.PendingSound, next.EffectExtras
	return nil
}

func stepFighter2ActionMotion(state *ChuActionState, parameters JumpParameters) error {
	next, err := StepFighter2Motion(state.specialMotionView(), parameters)
	if err != nil {
		return err
	}
	state.acceptSpecialMotion(next)
	return nil
}

func stepFighter8ActionMotion(state *ChuActionState, parameters JumpParameters) error {
	next, err := StepFighter8Motion(state.specialMotionView(), parameters)
	if err != nil {
		return err
	}
	state.acceptSpecialMotion(next)
	return nil
}

func stepFighter1ActionMotion(state *ChuActionState, parameters JumpParameters) error {
	next, err := StepFighter1Motion(Fighter1MotionState{ChuSpecialMotionState: state.specialMotionView(), EffectExtras: state.EffectExtras}, parameters)
	if err != nil {
		return err
	}
	state.acceptSpecialMotion(next.ChuSpecialMotionState)
	state.EffectExtras = next.EffectExtras
	return nil
}

func stepFighter6ActionMotion(state *ChuActionState, parameters JumpParameters) error {
	next, err := StepFighter6Motion(Fighter6MotionState{ChuSpecialMotionState: state.specialMotionView(), StunTicks: state.StunTicks}, parameters)
	if err != nil {
		return err
	}
	state.acceptSpecialMotion(next.ChuSpecialMotionState)
	state.StunTicks = next.StunTicks
	return nil
}

func (state ChuActionState) specialMotionView() ChuSpecialMotionState {
	return ChuSpecialMotionState{ObjectID: state.ObjectID, Body: state.Body, Controls: state.Controls, Energy: state.fullActionEnergy(), Power: state.Power,
		AlternateVelocity: state.AlternateVelocity, ContactFlags: state.ContactFlags, TargetBit: state.TargetBit, Children: state.Children, Pool: state.Pool}
}

func (state *ChuActionState) acceptSpecialMotion(next ChuSpecialMotionState) {
	state.Body, state.Controls, state.Pool, state.Children = next.Body, next.Controls, next.Pool, next.Children
	state.setActionEnergy(next.Energy)
	state.Power, state.AlternateVelocity, state.ContactFlags = next.Power, next.AlternateVelocity, next.ContactFlags
}

func (state ChuActionState) fullActionEnergy() int32 {
	return int32(uint32(uint16(state.Energy))<<16 | uint32(state.EnergyFraction))
}

func (state *ChuActionState) setActionEnergy(value int32) {
	state.Energy, state.EnergyFraction = int16(value>>16), uint16(value)
}
