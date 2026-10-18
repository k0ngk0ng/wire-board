package engine

import "errors"

type BallAction uint8

const (
	BallRest BallAction = iota
	BallDribble
	BallHeld
	BallPass
	BallShot
	BallRolling
	BallBounce
	BallKnockback
	BallDeflected
	BallStuck
)

type BallRequest struct {
	Action  BallAction `json:"action"`
	Pending bool       `json:"pending"`
}

type DeflectionAxes uint8

const (
	DeflectX      DeflectionAxes = 128
	DeflectHeight DeflectionAxes = 64
	DeflectDepth  DeflectionAxes = 32
)

type Ball struct {
	Body             Body           `json:"body"`
	Action           BallAction     `json:"action"`
	Entered          bool           `json:"entered"`
	Pose             Pose           `json:"pose"`
	Animation        AnimationState `json:"animation"`
	Airborne         bool           `json:"airborne"`
	FacingLeft       bool           `json:"facingLeft"`
	BouncesRemaining uint8          `json:"bouncesRemaining"`
	Request          BallRequest    `json:"request"`
	Deflection       DeflectionAxes `json:"deflection"`
}

// StepHeldBall preserves original rendering semantics: dribbling is drawn in
// the carrier's frames, so its separate ball sprite is hidden. An elevated ball
// follows integer pose offsets while retaining its own fractional coordinates.
// Free flight, catches, basket collision and scoring belong to later ball phases.
func StepHeldBall(original Ball, carrier Player) (Ball, error) {
	return StepHeldBallOnCourt(original, carrier, NewYorkBallCourt())
}

// StepHeldBallOnCourt runs the ball's own terrain phase before applying its
// carrier-pose offset. The terrain may change airborne/reflection state even
// though the following pose replaces its integer position.
func StepHeldBallOnCourt(original Ball, carrier Player, court FlatCourt) (Ball, error) {
	b := original
	if b.Action != BallDribble && b.Action != BallHeld {
		return original, errors.New("ball is not in a held action")
	}
	if !b.Entered {
		b.Entered = true
		b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 0, 0, 0
		b.Body.Velocity.X, b.Body.Velocity.Depth = 0, 0
		if b.Action == BallHeld {
			b.Body.Velocity.Height, b.Body.Gravity = 0, 0
		}
	}
	if err := animateOriginalHeldBall(&b, carrier); err != nil {
		return original, err
	}
	if b.Action == BallDribble {
		b.Body.Position = carrier.Body.Position
		return b, nil
	}
	if err := advanceBallOnCourt(&b, court); err != nil {
		return original, err
	}
	if int(carrier.Action) >= len(originalHeldBallActionOffsets) {
		return original, errors.New("held ball action is outside original pose table")
	}
	offset := originalHeldBallOffsets[originalHeldBallActionOffsets[carrier.Action]]
	dx, height, depth := Fixed(offset.X)*Pixel, Fixed(offset.Height)*Pixel, -Fixed(offset.SubtractDepth)*Pixel
	// $B5CC overrides the action offset for the overhead pose before $EC7B.
	if carrier.Pose.Frame == 30 {
		dx, height, depth = 4*Pixel, 6*Pixel, -Pixel
	}
	if carrier.Pose.Mirrored {
		dx = -dx
	}
	b.Body.Position.X = signed24((carrier.Body.Position.X &^ 255) + dx + (b.Body.Position.X & 255))
	b.Body.Position.Depth = Fixed(uint8((carrier.Body.Position.Depth>>8)+(depth>>8)))*Pixel + (b.Body.Position.Depth & 255)
	b.Body.Position.Height = signed24((carrier.Body.Position.Height &^ 255) + height + (b.Body.Position.Height & 255))
	if carrier.Pose.Frame >= 59 && carrier.Pose.Frame < 63 {
		b.Pose = Pose{Frame: 127}
	}
	return b, nil
}

// StepPassedBall advances a pass through its original launch, spin and court
// motion. Ground contact selects the bounce action for the next dispatch.
// Player and basket contacts are resolved by the surrounding world stages.
func StepPassedBall(original Ball, receiver Position) (Ball, error) {
	return stepPassedBallOnCourt(original, receiver, NewYorkBallCourt())
}

func stepPassedBallOnCourt(original Ball, receiver Position, court FlatCourt) (Ball, error) {
	b := original
	if b.Action != BallPass {
		return original, errors.New("ball is not passing")
	}
	if !b.Entered {
		distance := absPass(int32(receiver.X/Pixel - b.Body.Position.X/Pixel))
		arcs := [...]Fixed{0, 4, 8, 16, 48, 64, 80, 96}
		if distance/64 >= int32(len(arcs)) {
			return original, errors.New("pass arc distance not reconstructed")
		}
		launched, err := court.launchPass(b.Body.Position, receiver, arcs[distance/64]*Pixel)
		if err != nil {
			return original, err
		}
		b.Body = launched
		b.Entered = true
		b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 5, 0, 0
	}
	frames := []AnimationFrame{{Pose: Pose{Frame: 1}, Ticks: 2}, {Pose: Pose{Frame: 2}, Ticks: 2}, {Pose: Pose{Frame: 3}, Ticks: 2}, {Pose: Pose{Frame: 0}, Ticks: 2}}
	state := b.Animation
	launchLead := state.Cursor >= 5
	if launchLead {
		frames = frames[2:]
		state.Cursor -= 5
	}
	step, err := AdvanceAnimation(AnimationSpec{Frames: frames, End: RepeatForever}, state, b.FacingLeft)
	if err != nil {
		return original, err
	}
	if launchLead && !(step.State.Cursor == 0 && step.State.Elapsed == 0) {
		step.State.Cursor += 5
	}
	b.Animation, b.Pose = step.State, step.Pose
	if err := advanceBallOnCourt(&b, court); err != nil {
		return original, err
	}
	if !b.Airborne {
		b.Action, b.Entered = BallBounce, false
	}
	return b, nil
}
