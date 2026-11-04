package engine

import "errors"

type ChuTailMode uint8

const (
	ChuTailNormal ChuTailMode = iota
	ChuTailHitStop
)

type ChuTailParameters struct {
	JumpParameters
	BurstSpeed  int32 `json:"burst_speed"`
	EnergyDrain int16 `json:"energy_drain"`
}

type TailFighterEnergy struct {
	Size   uint16
	Team   uint8
	Health int16
	Energy int16
}

type ChuTailState struct {
	Body                           FighterBodyState
	Controls                       FighterControls
	ObjectID                       uint16
	Energy                         int16
	ChildSize                      uint16
	Pool                           EffectPool
	Throws                         ThrowContactBuffer
	OtherFighters                  map[uint16]TailFighterEnergy
	ChargeTicks                    uint8
	OccupiedLanes                  uint8
	BattleState                    int16
	AnimationSet, ShapeSet         uint32
	GraphicOffsetX, GraphicOffsetY int16
	GraphicParameter               uint16
	GuardMask, ContactFlags        uint8
	TargetMask                     uint32
	AttackValue, StunDamage        uint8
	EffectKind, Reaction           uint8
	AttackScale                    uint16
	RandomSeed                     uint32
	Buffers                        [4]AnimationShapeBuffer
	PendingSound                   uint8
	SoundQueue                     [4]uint8
}

// StepChuActionTail recovers the complete normal/hit-stop shared tail. Stage
// flags are reread after each call: earlier transitions can enable or bypass
// later handlers. World input preparation, movement and audio-consumer timing
// are separate caller stages, not simulated inside this atomic rule function.
func StepChuActionTail(state ChuTailState, parameters ChuTailParameters, library AnimationLibrary, mode ChuTailMode) (ChuTailState, error) {
	state, _, err := stepFighterActionTail(state, 9, 0, 0, parameters, library, mode)
	return state, err
}

func stepFighterActionTail(state ChuTailState, characterID uint8, aux48 int32, energyFraction uint16, parameters ChuTailParameters, library AnimationLibrary, mode ChuTailMode) (ChuTailState, int32, error) {
	if mode != ChuTailNormal && mode != ChuTailHitStop {
		return state, aux48, errors.New("unsupported shared-tail entry")
	}
	original := state
	originalAux48 := aux48
	state.OtherFighters = make(map[uint16]TailFighterEnergy, len(original.OtherFighters))
	for id, fighter := range original.OtherFighters {
		state.OtherFighters[id] = fighter
	}
	if mode == ChuTailNormal {
		if state.Body.Pose != 0x11 && state.BattleState >= 0x100 && state.Body.Ground>>16 == state.Body.Y>>16 && state.Body.Health >= 0 {
			state.Body.ActionKind, state.Body.Pose, state.Body.EnabledStages = 0, 0x11, 0
			state.Body.GroundActionState = ResetActionState(state.Body.GroundActionState)
		}
		if state.Body.EnabledStages&1 != 0 {
			body, pool, occupied, _, err := StepGroundControl(state.Body, state.Pool, state.OccupiedLanes, state.Controls)
			if err != nil {
				return original, originalAux48, err
			}
			state.Body, state.Pool, state.OccupiedLanes = body, pool, occupied
		}
		for _, stage := range []struct {
			flag uint8
			mode ChuAttackMode
		}{{8, ChuGroundSpecial}, {16, ChuAirSpecial}, {2, ChuGroundAttack}, {4, ChuAirAttack}} {
			if state.Body.EnabledStages&stage.flag == 0 {
				continue
			}
			if stage.mode == ChuGroundAttack {
				if state.Controls.Changed&0x70 == 0x70 {
					if err := drainChuOpponents(&state, parameters.EnergyDrain); err != nil {
						return original, originalAux48, err
					}
					state.Body.ActionKind, state.Body.Pose, state.Body.EnabledStages = 0, 0x14, 0
					state.Body.GroundActionState = ResetActionState(state.Body.GroundActionState)
					continue
				}
				if state.ChargeTicks > 24 {
					state.Body.ActionKind, state.Body.Pose, state.Body.EnabledStages = 13, 0x13, 0
					state.Body.VelocityX = parameters.BurstSpeed
					state.Body.GroundActionState = ResetActionState(state.Body.GroundActionState)
					continue
				}
			}
			if stage.mode == ChuAirAttack && state.Body.ExtraJump == 0 && state.Controls.Pressed&ButtonUp != 0 {
				beginChuExtraJump(&state, parameters.JumpParameters)
			}
			selection := ChuAttackState{Body: state.Body, Controls: state.Controls, ObjectID: state.ObjectID,
				Energy: state.Energy, ChildSize: state.ChildSize, GravityParameter: parameters.Gravity, Pool: state.Pool, Throws: state.Throws}
			var err error
			if characterID == 0 {
				selection, err = SelectFighter0Attack(selection, stage.mode)
			} else if characterID == 3 {
				var selected Fighter3AttackState
				selected, err = SelectFighter3Attack(Fighter3AttackState{ChuAttackState: selection, AlternateVelocity: aux48}, stage.mode)
				selection, aux48 = selected.ChuAttackState, selected.AlternateVelocity
			} else if characterID == 9 {
				selection, err = SelectChuAttack(selection, stage.mode)
			} else {
				var selected Fighter10AttackState
				selected, err = SelectFighter10Attack(Fighter10AttackState{ChuAttackState: selection, Aux48High: uint8(uint32(aux48) >> 24),
					EnergyFraction: energyFraction, RandomSeed: state.RandomSeed}, stage.mode)
				selection = selected.ChuAttackState
				aux48 = int32(uint32(aux48)&0x00ffffff | uint32(selected.Aux48High)<<24)
				state.RandomSeed = selected.RandomSeed
			}
			if err != nil {
				return original, originalAux48, err
			}
			state.Body, state.Pool, state.Throws, state.Energy = selection.Body, selection.Pool, selection.Throws, selection.Energy
		}
		state.ContactFlags = 0
	}
	switch state.Body.Pose {
	case 9, 0x2c:
		state.GuardMask = 4
	case 10, 0x2d:
		state.GuardMask = 8
	default:
		state.GuardMask = 0
	}
	animation, err := SubmitAnimationRecord(AnimationSubmissionState{Actor: AnimationActorState{
		ObjectID: state.ObjectID, AnimationSet: state.AnimationSet, ShapeSet: state.ShapeSet, Pose: state.Body.Pose, Clock: state.Body.Clock,
		Facing: state.Body.Facing, ActionFlags: state.Body.ActionFlags, EnabledStages: state.Body.EnabledStages, Attributes: state.Body.Attributes,
		GraphicOffsetX: state.GraphicOffsetX, GraphicOffsetY: state.GraphicOffsetY, WorldX: state.Body.X, WorldY: state.Body.Y,
		GraphicParameter: state.GraphicParameter, Auxiliary94: state.TargetMask, Auxiliary64: state.Body.Auxiliary64,
		Value65: state.AttackValue, Value66: state.StunDamage, Value67: state.EffectKind, Value68: state.Reaction},
		Buffers: state.Buffers, PendingSound: state.PendingSound, SoundQueue: state.SoundQueue}, library)
	if err != nil {
		return original, originalAux48, err
	}
	a := animation.Actor
	state.Body.Clock, state.Body.EnabledStages, state.Body.Attributes, state.Body.Auxiliary64 = a.Clock, a.EnabledStages, a.Attributes, a.Auxiliary64
	state.GraphicOffsetX, state.GraphicOffsetY, state.GraphicParameter, state.TargetMask = a.GraphicOffsetX, a.GraphicOffsetY, a.GraphicParameter, a.Auxiliary94
	state.AttackValue, state.StunDamage, state.EffectKind, state.Reaction = a.Value65, a.Value66, a.Value67, a.Value68
	state.Buffers, state.PendingSound, state.SoundQueue = animation.Buffers, animation.PendingSound, animation.SoundQueue
	randomized := RandomizeAttackValue(AttackValueState{Seed: state.RandomSeed, Scale: state.AttackScale, Value: state.AttackValue})
	state.RandomSeed, state.AttackValue = randomized.Seed, randomized.Value
	return state, aux48, nil
}

func drainChuOpponents(state *ChuTailState, amount int16) error {
	for _, id := range [...]uint16{0x8400, 0x8800, 0x8c00, 0x9000} {
		if id == state.ObjectID {
			continue // The source's own current team always matches itself.
		}
		other, ok := state.OtherFighters[id]
		if !ok {
			return errors.New("energy-drain fighter is unavailable")
		}
		if other.Size == 0 || other.Team == state.Body.Team || other.Health < 0 {
			continue
		}
		other.Energy -= amount // SUB.W wraps before TST.W clamps negative values.
		if other.Energy < 0 {
			other.Energy = 0
		}
		state.OtherFighters[id] = other
	}
	return nil
}

func beginChuExtraJump(state *ChuTailState, parameters JumpParameters) {
	state.Body.ExtraJump++
	state.Body.GroundActionState = ResetActionState(state.Body.GroundActionState)
	state.Body.Gravity = parameters.Gravity
	state.Body.VerticalVelocity = (parameters.VerticalVelocity >> 1) + (parameters.VerticalVelocity >> 2)
	state.Body.ActionKind, state.Body.Pose, state.Body.EnabledStages = 2, 6, 0x14
	switch state.Controls.Held & 15 {
	case 9:
		state.Body.VelocityX = parameters.HorizontalVelocity
		if state.Body.Facing == 1 {
			state.Body.Pose = 7
		}
	case 5:
		state.Body.VelocityX = -parameters.HorizontalVelocity
		if state.Body.Facing == 0 {
			state.Body.Pose = 7
		}
	case 1:
		state.Body.VelocityX, state.Body.Pose = 0, 8
	}
}
