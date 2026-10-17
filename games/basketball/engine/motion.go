// Package engine owns the basketball simulation. It has no browser, network,
// database, or emulator dependency and can be compiled natively or to Wasm.
package engine

// Fixed is a distance or per-update velocity with 8 fractional bits.
// One pixel is 256 units, preserving the original subpixel movement exactly.
type Fixed int32

const Pixel Fixed = 256

// Position keeps floor depth separate from height above the floor. Drawing uses
// both to project the player; jumping must not move its ground collision point.
type Position struct {
	X      Fixed `json:"x"`
	Depth  Fixed `json:"depth"`
	Height Fixed `json:"height"`
}

type Velocity struct {
	X      Fixed `json:"x"`
	Depth  Fixed `json:"depth"`
	Height Fixed `json:"height"`
}

type Body struct {
	Position Position `json:"position"`
	Velocity Velocity `json:"velocity"`
	Gravity  Fixed    `json:"gravity"`
}

// IntegrateMotion advances one original simulation update, before collisions.
// Vertical acceleration is applied before integrating height. Position and
// velocity widths preserve the original arithmetic, including negative heights
// that the later landing/collision phase will resolve.
//
// Evidence: PRG bank 15, $EF5F..$EFD4; testdata/motion.csv contains independent
// FCEUX observations. This is a semantic physics operation, not instruction
// translation or CPU emulation.
func IntegrateMotion(body Body) Body {
	body.Position.X = signed24(body.Position.X + signed16(body.Velocity.X))
	body.Position.Depth = Fixed(uint16(body.Position.Depth + signed16(body.Velocity.Depth)))
	body.Velocity.Height = signed16(body.Velocity.Height - body.Gravity)
	body.Position.Height = signed24(body.Position.Height + body.Velocity.Height)
	return body
}

func signed16(value Fixed) Fixed { return Fixed(int16(value)) }
func signed24(value Fixed) Fixed { return value << 8 >> 8 }
