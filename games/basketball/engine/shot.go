package engine

import "errors"

// Hoop indexes are bottom-left, bottom-right, middle-left, middle-right,
// top-left, top-right. They are game objects, not original RAM indexes.
type ShotPlan struct {
	Target    Position `json:"target"`
	ArcHeight Fixed    `json:"arcHeight"`
	Hoop      uint8    `json:"hoop"`
}

// ChooseOrdinaryShot restores target selection without equipment/special-shot
// overrides. Original ball height selects the hoop state to inspect; shooter
// height separately selects the target elevation. These thresholds differ.
func ChooseOrdinaryShot(ball Position, shooterHeight Fixed, facingLeft bool, rimFlags [6]uint8) (ShotPlan, error) {
	if ball.Height < 0 || shooterHeight < 0 {
		return ShotPlan{}, errors.New("negative shot height")
	}
	index := uint8(0)
	if !facingLeft {
		index++
	}
	if ball.Height >= 96*Pixel {
		index += 2
	}
	if ball.Height >= 192*Pixel {
		index += 2
	}
	targetHeight := Fixed(88) * Pixel
	if shooterHeight >= 56*Pixel {
		targetHeight = 184 * Pixel
	}
	if shooterHeight >= 152*Pixel {
		targetHeight = 280 * Pixel
	}
	x := Fixed(460) * Pixel
	if facingLeft {
		x = 68 * Pixel
	}
	arc := Fixed(24) * Pixel
	if rimFlags[index]&2 != 0 {
		arc = 48 * Pixel
	}
	return ShotPlan{Target: Position{X: x, Depth: 47 * Pixel, Height: targetHeight}, ArcHeight: arc, Hoop: index}, nil
}

func LaunchShot(position Position, plan ShotPlan) (Body, error) {
	return LaunchPass(position, plan.Target, plan.ArcHeight)
}

func NewYorkBallCourt() FlatCourt {
	c := NewYorkGroundCourt()
	c.MinX, c.MaxX, c.LeftReset, c.RightReset = 47*Pixel, 480*Pixel, 48*Pixel, 479*Pixel
	return c
}

func advanceBallSpin(b *Ball) error {
	frames := []AnimationFrame{{Pose: Pose{Frame: 1}, Ticks: 2}, {Pose: Pose{Frame: 2}, Ticks: 2}, {Pose: Pose{Frame: 3}, Ticks: 2}, {Pose: Pose{Frame: 0}, Ticks: 2}}
	state := b.Animation
	lead := state.Cursor >= 5
	if lead {
		frames = frames[2:]
		state.Cursor -= 5
	}
	step, err := AdvanceAnimation(AnimationSpec{Frames: frames, End: RepeatForever}, state, b.FacingLeft)
	if err != nil {
		return err
	}
	if lead && !(step.State.Cursor == 0 && step.State.Elapsed == 0) {
		step.State.Cursor += 5
	}
	b.Animation, b.Pose = step.State, step.Pose
	return nil
}

// advanceBallOnCourt restores the ball's floor handling, which differs from a
// player's landing: horizontal movement is retained. It also handles the
// identified New York platform surfaces; basket contacts remain a world phase.
func advanceBallOnCourt(b *Ball, court FlatCourt) error {
	if court.originalMotion != nil {
		return court.originalMotion.ball(b)
	}
	court.recordMotionSurface(b.Body.Position)
	previous := b.Body.Position
	b.Body = IntegrateMotion(b.Body)
	p, v := &b.Body.Position, &b.Body.Velocity
	if p.X < court.MinX {
		p.X = court.LeftReset + (p.X & 255)
		v.X, v.Depth = 0, 0
	}
	if p.X >= court.MaxX {
		p.X = court.RightReset + (p.X & 255)
		v.X, v.Depth = 0, 0
	}
	if p.Depth < court.MinDepth {
		p.Depth = court.MinDepth + (p.Depth & 255)
	}
	if p.Depth >= court.MaxDepth {
		p.Depth = court.MaxDepth - Pixel + (p.Depth & 255)
	}
	surface := func(x, depth Fixed) Fixed {
		if court.RaisedDepth > 0 && depth >= court.RaisedDepth && (x < court.RaisedLeftEnd || x >= court.RaisedRightStart) {
			return court.RaisedFloor
		}
		return court.Floor
	}
	floorPosition := surface(p.X, p.Depth)
	if p.Height>>8 < floorPosition>>8 && !(v.Height < 0 && previous.Height>>8 >= floorPosition>>8) {
		b.Airborne = v.Height != 0 || b.Body.Gravity != 0
		blocked := false
		if surface(p.X, previous.Depth)>>8 > Fixed(uint8(p.Height>>8)) {
			v.X = 0
			if previous.X>>8 < p.X>>8 {
				p.X = (previous.X &^ 2047) + 7*Pixel + (previous.X & 255)
			} else if previous.X>>8 > p.X>>8 {
				p.X = (previous.X &^ 2047) + (previous.X & 255)
			}
			b.Request = BallRequest{Action: BallDeflected, Pending: true}
			b.Deflection = DeflectX
			blocked = true
		}
		if surface(previous.X, p.Depth)>>8 > Fixed(uint8(p.Height>>8)) {
			v.Depth = 0
			p.Depth = (previous.Depth &^ 2047) + 7*Pixel + (previous.Depth & 255)
			blocked = true
		}
		if !blocked && floorPosition > court.Floor {
			return errors.New("diagonal raised-platform contact not reconstructed")
		}
		floorPosition = surface(p.X, p.Depth)
	}
	height, floor := p.Height>>8, floorPosition>>8
	if height < floor && v.Height < 0 {
		p.Height = floorPosition
		v.Height, b.Body.Gravity = 0, 0
		b.Airborne = false
	} else if height == floor {
		if v.Height < Pixel {
			p.Height = floorPosition
		}
		if v.Height == 0 && b.Body.Gravity == 0 {
			b.Airborne = false
		}
	} else if height > floor {
		b.Airborne = true
	}
	// Bank4 8249 releases a resting ball on a raised floor back toward the
	// court before its shared depth-surface correction. It does not enter the
	// bounce action until its next action dispatch.
	if height == floor && !b.Airborne && b.Action == BallRest && uint8(p.Height>>8) != 0 {
		b.Action, b.Entered = BallBounce, false
		v.Depth = -Pixel / 2
	}
	if !b.Airborne && floorPosition > court.Floor && b.Action != BallHeld && b.Action != BallDribble {
		if p.Depth >= 80*Pixel {
			p.Depth = ((p.Depth>>8)&240)*Pixel - 8*Pixel + (p.Depth & 255)
		} else {
			p.Depth = (p.Depth &^ 2047) + (p.Depth & 255)
		}
	}
	if b.Action == BallBounce && surface(previous.X, previous.Depth) > court.Floor && v.Depth >= 0 {
		v.Depth = 0
	}
	return nil
}

func StepShotBall(original Ball, plan ShotPlan, court FlatCourt) (Ball, error) {
	b := original
	if b.Action != BallShot {
		return original, errors.New("ball is not in shot flight")
	}
	if !b.Entered {
		body, err := court.launchPass(b.Body.Position, plan.Target, plan.ArcHeight)
		if err != nil {
			return original, err
		}
		b.Body = body
		b.Entered = true
		b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 5, 0, 0
	}
	if err := advanceBallSpin(&b); err != nil {
		return original, err
	}
	if err := advanceBallOnCourt(&b, court); err != nil {
		return original, err
	}
	if !b.Airborne {
		b.Action, b.Entered = BallBounce, false
	}
	return b, nil
}

func StepBouncingBall(original Ball, court FlatCourt) (Ball, error) {
	b := original
	if b.Action != BallBounce {
		return original, errors.New("ball is not bouncing")
	}
	if !b.Entered {
		b.Entered = true
		b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 0, 0, 0
		b.Body.Velocity.Height, b.Body.Gravity = 0, 0
		b.BouncesRemaining = 3
		b.Airborne = false
	}
	if err := advanceBallSpin(&b); err != nil {
		return original, err
	}
	if !b.Airborne {
		if b.BouncesRemaining == 0 {
			return original, errors.New("bounce counter is exhausted")
		}
		b.Body.Velocity.Height, b.Body.Gravity = 0, 0
		b.BouncesRemaining--
		if b.BouncesRemaining == 0 {
			b.Action, b.Entered = BallRolling, false
		} else {
			b.Body.Velocity.Height = Fixed(b.BouncesRemaining) * 2 * Pixel
			b.Body.Gravity = Fixed(b.BouncesRemaining) * 64
		}
	}
	if err := advanceBallOnCourt(&b, court); err != nil {
		return original, err
	}
	return b, nil
}
