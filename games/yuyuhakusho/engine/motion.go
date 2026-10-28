package engine

// MotionState holds signed 16.16 world positions, velocities and acceleration.
// Screen projection is separate. Fixed-width overflow follows the original
// state arithmetic and is identical in Go native and Wasm.
type MotionState struct {
	X       int32 `json:"x"`
	Y       int32 `json:"y"`
	VX      int32 `json:"vx"`
	VY      int32 `json:"vy"`
	Gravity int32 `json:"gravity"`
	Ground  int32 `json:"ground"`
	Facing  uint8 `json:"facing"`
}

// StepWalk recovers the movement stage at 0x19da2. The speed comes from the
// current fighter's movement parameters. The shared action tail runs afterward.
func StepWalk(state MotionState, speed int32) MotionState {
	if state.Facing == 0 {
		state.X += speed
	} else {
		state.X -= speed
	}
	state.Y = state.Ground
	return state
}

// IntegrateAir recovers 0x19dc6 up to the landing branch. A true result asks
// the caller to enter the landing constructor, including its effects; it does
// not silently substitute a position clamp for that still-separate transition.
func IntegrateAir(state MotionState) (MotionState, bool) {
	state.X += state.VX
	state.VY += state.Gravity
	state.Y += state.VY
	landing := state.VY >= 0 && int16(state.Y>>16) >= int16(state.Ground>>16)
	return state, landing
}

// ProjectionState keeps camera and one-frame offsets in their original word
// units. Offsets are consumed and cleared by projection, rather than accumulated
// into world position or velocity.
type ProjectionState struct {
	WorldX  int32 `json:"world_x"`
	WorldY  int32 `json:"world_y"`
	ScreenX int16 `json:"screen_x"`
	ScreenY int16 `json:"screen_y"`
	OffsetX int16 `json:"offset_x"`
	OffsetY int16 `json:"offset_y"`
	CameraX int16 `json:"camera_x"`
	CameraY int16 `json:"camera_y"`
	Shake   int16 `json:"shake"`
}

// ProjectFighter recovers 0x156f0. Each intermediate addition is a word
// operation; widening before clipping would change extreme camera positions.
func ProjectFighter(state ProjectionState) ProjectionState {
	x := int16(state.WorldX>>16) - state.CameraX
	x += 128
	x += state.OffsetX
	if x <= 64 {
		x = 64
	}
	if x >= 511 {
		x = 511
	}
	y := int16(state.WorldY>>16) - state.CameraY
	y += state.Shake
	y += 32
	y += state.OffsetY
	if y <= 64 {
		y = 64
	}
	if y >= 448 {
		y = 448
	}
	state.ScreenX, state.ScreenY = x, y-8
	state.OffsetX, state.OffsetY = 0, 0
	return state
}
