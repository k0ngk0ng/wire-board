package engine

import "errors"

// Original direction quantization, in 1/256 units. These are ordinary game
// parameters, not instructions: changing them changes the pass aiming curve.
var passRatioAngle = [...]uint8{0, 1, 2, 4, 5, 6, 8, 9, 10, 11, 12, 13, 14, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 28, 29, 30, 31, 31, 32}
var passSine = [...]int32{0, 7, 15, 20, 25, 33, 38, 48, 51, 56, 64, 69, 76, 81, 87, 94, 99, 107, 110, 115, 122, 128, 133, 138, 140, 148, 153, 158, 163, 166, 174, 176, 179}
var passCosine = [...]int32{255, 255, 255, 254, 254, 253, 253, 251, 250, 249, 248, 246, 245, 243, 240, 238, 235, 232, 230, 227, 225, 222, 220, 215, 212, 207, 204, 202, 197, 194, 189, 184, 179}
var passDistanceExtra = [...]int32{0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 7, 8, 10, 15, 16, 20, 21, 24, 26, 28, 33, 36, 38, 45, 49, 56, 60, 64, 73, 77, 88, 99, 105}

// passFallUpdates counts the original discrete half-pixel gravity steps. The
// extra update is intentional: replacing this with a continuous square root
// changes release velocity and the time at which the receiver meets the ball.
func passFallUpdates(height int32) int32 {
	var distance, velocity, updates int32
	for distance>>8 <= height {
		updates++
		distance += velocity
		velocity += 128
	}
	return updates + 1
}

// PassLaunchContext supplies the original equal-diagonal quirk: bank5 $97C9
// loads $0069 (player seat 1's X fraction), whereas other quadrants load the
// immediate constant $69. Read it at launch time, after earlier actor updates.
type PassLaunchContext struct {
	SecondPlayerX Fixed
}

// LaunchPass chooses the original flight velocity from whole-pixel source and
// receiver positions, retaining the source fractions for subsequent physics.
// This does not resolve catches, contact, baskets or the rest of a world step.
func LaunchPass(position, target Position, arcHeight Fixed, context ...PassLaunchContext) (Body, error) {
	original := Body{Position: position}
	if position.Height < 0 || target.Height < 0 || arcHeight < 0 || position.Height > 511*Pixel || target.Height > 511*Pixel || arcHeight > 255*Pixel {
		return original, errors.New("pass height lies outside reconstructed court range")
	}
	dx := int32(target.X/Pixel - position.X/Pixel)
	dy := int32(target.Depth/Pixel - position.Depth/Pixel)
	ax, ay := absPass(dx), absPass(dy)
	if ax > 1023 || ay > 255 {
		return original, errors.New("pass distance lies outside reconstructed court range")
	}
	major, minor := max(ax, ay), min(ax, ay)
	var nx, ny, extra int32
	if ax == ay {
		nx, ny, extra = 179, 179, 105
		if dx >= 0 && dy < 0 {
			if len(context) == 0 {
				return original, errors.New("positive-X negative-depth diagonal pass requires second-player X context")
			}
			extra = int32(uint8(context[0].SecondPlayerX))
		}
	} else {
		ratio := minor * 256 / major
		angle := passRatioAngle[min(255, ratio+3)/8]
		if angle == 0 {
			nx, ny, extra = 256, 0, 0
		} else {
			nx, ny, extra = passCosine[angle], passSine[angle], passDistanceExtra[angle]
		}
		if ay > ax {
			nx, ny = ny, nx
		}
	}
	if dx < 0 {
		nx = -nx
	}
	if dy < 0 {
		ny = -ny
	}
	travelDistance := major * (256 + extra) >> 8
	rise := int32((target.Height / Pixel) - (position.Height / Pixel) + (arcHeight / Pixel))
	drop := int32(arcHeight / Pixel)
	for rise < 0 {
		drop = int32(position.Height / Pixel)
		rise += drop
	}
	ascent, descent := passFallUpdates(rise), passFallUpdates(drop)
	speed := travelDistance * 256 / (ascent + descent)
	vx, vy := nx*speed>>8, ny*speed>>8
	// The original saturates the signed integer byte, retaining fractions.
	if vx >= 13*256 {
		vx = 12*256 + (vx & 255)
	}
	if vx < -12*256 {
		vx = -12*256 + (vx & 255)
	}
	return Body{Position: position, Velocity: Velocity{X: Fixed(vx), Depth: Fixed(vy), Height: Fixed(ascent * 128)}, Gravity: 128}, nil
}

func absPass(value int32) int32 {
	if value < 0 {
		return -value
	}
	return value
}

func (c FlatCourt) launchPass(position, target Position, arcHeight Fixed) (Body, error) {
	if c.passLaunchContext != nil {
		return LaunchPass(position, target, arcHeight, c.passLaunchContext())
	}
	return LaunchPass(position, target, arcHeight)
}
