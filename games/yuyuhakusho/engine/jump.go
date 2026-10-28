package engine

// FighterBodyState extends recovered action state with world motion and the
// source fields needed by movement effects. VerticalVelocity has one owner in
// GroundActionState, including its preparatory directional-byte representation.
type FighterBodyState struct {
	GroundActionState
	X               int32  `json:"x"`
	Y               int32  `json:"y"`
	VelocityX       int32  `json:"velocity_x"`
	Gravity         int32  `json:"gravity"`
	Ground          int32  `json:"ground"`
	ExtraJump       uint8  `json:"extra_jump"`
	Attributes      uint16 `json:"attributes"`
	Depth           uint16 `json:"depth"`
	Layer           uint8  `json:"layer"`
	Health          int16  `json:"health"`
	Auxiliary7E     int16  `json:"auxiliary_7e"`
	Auxiliary80     int16  `json:"auxiliary_80"`
	TransitionDelay uint8  `json:"transition_delay"`
}

type JumpParameters struct {
	VerticalVelocity   int32 `json:"vertical_velocity"`
	Gravity            int32 `json:"gravity"`
	HorizontalVelocity int32 `json:"horizontal_velocity"`
}

func (state FighterBodyState) MovementDustSource() EffectSource {
	return EffectSource{X: state.X, Y: state.Y, Attributes: state.Attributes, Layer: state.Layer, Depth: state.Depth}
}

// BeginJump recovers 0x18580 through its actual return. Dust is created before
// action reset or velocity replacement. Noncanonical directions preserve the
// old horizontal velocity, matching the original fallthrough path.
func BeginJump(state FighterBodyState, pool EffectPool, parameters JumpParameters) (FighterBodyState, EffectPool, [2]int, error) {
	next, selected, err := SpawnMovementDust(pool, state.MovementDustSource())
	if err != nil {
		return state, pool, [2]int{-1, -1}, err
	}
	state.GroundActionState = ResetActionState(state.GroundActionState)
	direction := uint8(uint32(state.VerticalVelocity) >> 24)
	state.VerticalVelocity = parameters.VerticalVelocity
	state.Gravity = parameters.Gravity
	state.ActionKind, state.Pose, state.EnabledStages = 2, 6, 0x14
	switch direction {
	case 9:
		state.VelocityX = parameters.HorizontalVelocity
		if state.Facing == 1 {
			state.Pose = 7
		}
	case 5:
		state.VelocityX = -parameters.HorizontalVelocity
		if state.Facing == 0 {
			state.Pose = 7
		}
	case 1:
		state.VelocityX = 0
		state.Pose = 8
	}
	return state, next, selected, nil
}

// BeginLanding recovers 0x18564, including both independent effect allocations
// and the tail-called common action reset. It does not clamp world Y itself;
// the air action performs that before invoking this constructor.
func BeginLanding(state FighterBodyState, pool EffectPool) (FighterBodyState, EffectPool, [2]int, error) {
	next, selected, err := SpawnMovementDust(pool, state.MovementDustSource())
	if err != nil {
		return state, pool, [2]int{-1, -1}, err
	}
	state.ActionKind, state.Pose, state.EnabledStages = 0, 0x0e, 0x0b
	state.GroundActionState = ResetActionState(state.GroundActionState)
	return state, next, selected, nil
}

// StepAirAction recovers 0x19dc6 up to entry into 0x15afc. The shared action
// tail still has to process input, animation/shape data and random updates.
func StepAirAction(state FighterBodyState, pool EffectPool) (FighterBodyState, EffectPool, [2]int, bool, error) {
	before := state
	motion, landed := IntegrateAir(MotionState{X: state.X, Y: state.Y, VX: state.VelocityX, VY: state.VerticalVelocity, Gravity: state.Gravity, Ground: state.Ground, Facing: state.Facing})
	state.X, state.Y, state.VerticalVelocity = motion.X, motion.Y, motion.VY
	if !landed {
		return state, pool, [2]int{-1, -1}, false, nil
	}
	state.Y = state.Ground
	state, next, selected, err := BeginLanding(state, pool)
	if err != nil {
		return before, pool, [2]int{-1, -1}, false, err
	}
	state.ExtraJump = 0
	return state, next, selected, true, nil
}
