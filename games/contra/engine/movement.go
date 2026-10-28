package contra

// PlayerMotion is a semantic projection of the original player's vertical
// state, not an emulated address space. Position is signed 16.8 (24-bit), and
// Velocity signed 8.8 (16-bit); positive Y points down. Coordinates wrap at
// their original widths before collision/viewport rules act on them.
type PlayerMotion struct {
	Position       int32 `json:"position"`
	Velocity       int16 `json:"velocity"`
	Jump           uint8 `json:"jump"`
	AnimationTimer uint8 `json:"animationTimer"`
	AnimationFrame uint8 `json:"animationFrame"`
	Aim            uint8 `json:"aim"`
}

// BeginJump reproduces the guarded $D5FD..D625 transition. The caller decides
// whether jumping is legal for the current water/platform/indoor phase. The
// existing position fraction deliberately survives, as in the original.
func (m *PlayerMotion) BeginJump(indoor bool) {
	if m.Jump != 0 {
		return
	}
	m.Jump = 0x11
	if m.Aim >= 5 {
		m.Jump = 0x91
	}
	m.AnimationTimer, m.AnimationFrame = 0, 0
	m.Velocity = -1040 // $FBF0: -4.0625 pixels/tick, outdoor.
	if indoor {
		m.Velocity = -880 // $FC90: -3.4375 pixels/tick, indoor.
	}
}

// ApplyGravity reproduces $D922..D92F, including 16-bit wrap. The routine does
// not clamp terminal velocity; phase/collision code controls when it is used.
func (m *PlayerMotion) ApplyGravity() {
	m.Velocity += 0x23
}

// AdvanceY reproduces $D901..D921: add a sign-extended velocity to the complete
// position, retaining fractional carry and the offscreen high byte.
func (m *PlayerMotion) AdvanceY() {
	position := (m.Position + int32(m.Velocity)) & 0xffffff
	if position&0x800000 != 0 {
		position -= 0x1000000
	}
	m.Position = position
}
