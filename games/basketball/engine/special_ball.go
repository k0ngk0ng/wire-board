package engine

import "fmt"

type OriginalShotBallContext struct {
	// Original $055A belongs to owner six, not any player SpecialShot.
	UnownedSpecialShot     uint8
	Holder                 int8
	ShooterHeight          Fixed
	ShooterAction          Action
	Team, Character        uint8
	EnhancementEnabled     bool
	Equipment, SpecialShot uint8
	BallInputButtons       Buttons
	MountFlags             [6]uint8
	SelectedHoop           int8
	Target                 Position
	ArcHeight              Fixed
	Landing                Position
}
type OriginalShotBallStep struct {
	ClearUnownedSpecialShot bool
	Ball                    Ball
	ReleaseHolder           bool
	ChangeSpecialShot       bool
	SpecialShot             uint8
	SelectedHoop            int8
	Target                  Position
	ArcHeight               Fixed
	Landing                 Position
	SoundIDs                []uint8
}

func originalShotLaunchPlan(ball Ball, ctx OriginalShotBallContext) (ShotPlan, bool, error) {
	if ctx.Holder == 6 {
		// DFF8 reads the current owner's height. A loose ball relaunch owns
		// itself; E078 skips player equipment and only consumes its own flag.
		plan, err := ChooseOrdinaryShot(ball.Body.Position, ball.Body.Position.Height, ball.FacingLeft, ctx.MountFlags)
		consume := ctx.UnownedSpecialShot != 0
		if consume {
			plan.Target.Height = 272 * Pixel
		}
		return plan, consume, err
	}
	if ctx.Holder < 0 || ctx.Holder >= 4 {
		return ShotPlan{}, false, fmt.Errorf("shot launch owner %d is not a player", ctx.Holder)
	}
	plan, err := ChooseOrdinaryShot(ball.Body.Position, ctx.ShooterHeight, ball.FacingLeft, ctx.MountFlags)
	if err != nil {
		return plan, false, err
	}
	consume := false
	if !ctx.EnhancementEnabled || ctx.Team != 0 {
		consume = ctx.SpecialShot != 0
	} else {
		if ctx.Character >= 4 {
			return plan, false, fmt.Errorf("invalid shot character")
		}
		if ctx.ShooterAction != ActionShoot && ctx.ShooterAction != ActionAirShoot {
			return plan, false, fmt.Errorf("non-release equipment targeting is not reconstructed")
		}
		if ctx.Equipment != 255 && ctx.Equipment >= 32 {
			return plan, false, fmt.Errorf("shot equipment outside original catalog")
		}
		if ctx.Equipment < 32 && originalEquipmentShotModes[ctx.Equipment] == ctx.Equipment {
			heights := [4]Fixed{272, 176, 104, 160}
			index := ctx.Equipment >> 2
			if index >= 4 {
				return plan, false, fmt.Errorf("equipment target height index outside recovered table")
			}
			plan.Target.Height = heights[index] * Pixel
		} else if ctx.Character == 0 {
			consume = ctx.SpecialShot != 0
		} else if ctx.BallInputButtons&ButtonUp != 0 {
			// $E107 aliases the ball's input slot to controller-two press memory.
			// Its extra-object flag write is not part of the ordinary shooter state.
			return plan, false, fmt.Errorf("extra-object shot target override is not reconstructed")
		}
	}
	if consume {
		plan.Target.Height = 272 * Pixel
	}
	return plan, consume, nil
}

func selectOriginalBallHoop(ball Ball) int8 {
	object := int8(7)
	if !ball.FacingLeft {
		object++
	}
	if ball.Body.Position.Height >= 96*Pixel {
		object += 2
	}
	if ball.Body.Position.Height >= 192*Pixel {
		object += 2
	}
	return object
}

func animateOriginalSpecialBall(ball *Ball, action BallAction) error {
	frame := func(p, t uint8) AnimationFrame { return AnimationFrame{Pose: Pose{Frame: p}, Ticks: t} }
	var primary, secondary []AnimationFrame
	start := uint8(0)
	ending := RepeatForever
	switch action {
	case 13:
		primary = []AnimationFrame{frame(5, 1), frame(4, 1), frame(5, 1)}
		secondary = []AnimationFrame{frame(38, 1), frame(39, 1), frame(39, 1)}
		start = 4
	case 23:
		primary = []AnimationFrame{frame(33, 1), frame(34, 1), frame(35, 1)}
		secondary = []AnimationFrame{frame(36, 1), frame(37, 1), frame(37, 1)}
		start = 4
	case 20:
		primary = []AnimationFrame{frame(23, 2), frame(25, 2), frame(26, 2), frame(24, 2)}
		secondary = []AnimationFrame{frame(20, 2), frame(21, 2), frame(22, 255)}
		start = 5
	case 21:
		primary = []AnimationFrame{frame(28, 1), frame(29, 2)}
		secondary = []AnimationFrame{frame(27, 2)}
		start = 3
	default:
		return fmt.Errorf("special ball animation %d not reconstructed", action)
	}
	state := ball.Animation
	lead := state.Cursor >= start
	frames := primary
	if lead {
		state.Cursor -= start
		frames = secondary
		if action == 20 {
			ending = RepeatRemaining
		}
	}
	step, err := AdvanceAnimation(AnimationSpec{Frames: frames, End: ending}, state, ball.FacingLeft)
	if err != nil {
		return err
	}
	if lead && (step.State.Cursor != 0 || step.State.Elapsed != 0) {
		step.State.Cursor += start
	}
	ball.Animation, ball.Pose = step.State, step.Pose
	if step.ChangeAction {
		ball.Action, ball.Entered = BallAction(step.Action), false
	}
	return nil
}

func specialBallPassedTurnLine(ball *Ball, selected int8, offset Fixed) bool {
	target := Fixed(463) - offset
	if uint8(selected)&1 != 0 {
		target = 64 + offset
	}
	x := ball.Body.Position.X >> 8
	crossed := x < target && ball.Body.Velocity.X < 0 || x >= target && ball.Body.Velocity.X>>8 > 0
	if crossed {
		ball.Body.Velocity.Depth = 0
	}
	return crossed
}

func redirectOriginalSpecialDrop(ball *Ball) error {
	side := ball.Body.Position.X >> 16
	if side < 0 || side > 1 {
		return fmt.Errorf("special ball outside original court halves")
	}
	ball.Body.Velocity.X, ball.Body.Velocity.Depth = 0, 0
	x := uint8(64)
	if side != 0 {
		x = 207
	}
	ball.Body.Position.X = setCourtHalfPixel(ball.Body.Position.X, x)
	ball.Body.Position.Depth = 45*Pixel + (ball.Body.Position.Depth & 255)
	ball.Body.Velocity.Height = -11 * Pixel / 4
	ball.Body.Gravity = Pixel
	return nil
}

func originalSpecialDropBand(ball Ball) bool {
	side := ball.Body.Position.X >> 16
	x := ball.Body.Position.X >> 8
	if side == 0 && x > 80 || side != 0 && x <= 447 {
		return false
	}
	low, high := uint8(ball.Body.Position.Height>>8), uint8(ball.Body.Position.Height>>16)
	if high != 0 {
		if low >= 72 {
			return true
		}
	} else if low < 88 {
		return false
	}
	// The first high-byte subtraction in $BA4D uses the low height again.
	// Preserve that original comparison, rather than correcting its bands.
	for _, band := range [3][2]uint16{{328, 304}, {232, 208}, {136, 112}} {
		borrow := 0
		if uint8(band[0]) < low {
			borrow = 1
		}
		if int(band[0]>>8)-int(low)-borrow >= 0 {
			return false
		}
		borrow = 0
		if uint8(band[1]) < low {
			borrow = 1
		}
		if int(band[1]>>8)-int(high)-borrow >= 0 {
			return true
		}
	}
	return false
}

func clampOriginalSpecialDescent(ball *Ball) {
	if ball.Body.Velocity.Height>>8 < -8 {
		ball.Body.Velocity.Height = -7*Pixel + (ball.Body.Velocity.Height & 255)
	}
}

// StepOriginalShotBall restores ordinary/high-target shots and the original
// New York/Dallas special flight handlers. It never creates a score or a hit.
func StepOriginalShotBall(original Ball, ctx OriginalShotBallContext, court FlatCourt) (OriginalShotBallStep, error) {
	initial := OriginalShotBallStep{Ball: original, SpecialShot: ctx.SpecialShot, SelectedHoop: ctx.SelectedHoop, Target: ctx.Target, ArcHeight: ctx.ArcHeight, Landing: ctx.Landing}
	result := initial
	resolved, resolveErr := ResolveBallRequest(original)
	if resolveErr != nil {
		return initial, resolveErr
	}
	result.Ball = resolved
	if resolved.Action == BallDeflected {
		next, err := StepDeflectedBall(resolved, court)
		if err != nil {
			return initial, err
		}
		result.Ball = next
		return result, nil
	}
	b := &result.Ball
	action := b.Action
	if action != BallShot && action != 13 && action != 20 && action != 21 && action != 23 {
		return initial, fmt.Errorf("special ball action %d not reconstructed", action)
	}
	initializing := !b.Entered
	plan := ShotPlan{Target: ctx.Target, ArcHeight: ctx.ArcHeight}
	if initializing {
		result.SelectedHoop = selectOriginalBallHoop(*b)
		if action != 21 {
			var clear bool
			var err error
			plan, clear, err = originalShotLaunchPlan(*b, ctx)
			if err != nil {
				return initial, err
			}
			if clear {
				if ctx.Holder == 6 {
					result.ClearUnownedSpecialShot = true
				} else {
					result.ChangeSpecialShot = true
					result.SpecialShot = 0
				}
			}
			result.Target, result.ArcHeight = plan.Target, plan.ArcHeight
		}
		result.ReleaseHolder = true
	}
	if action == BallShot {
		next, err := StepShotBall(*b, plan, court)
		if err != nil {
			return initial, err
		}
		result.Ball = next
		if initializing {
			result.Landing = Position{X: plan.Target.X, Depth: plan.Target.Depth}
		}
		return result, nil
	}
	if initializing {
		b.Entered = true
		b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 0, 0, 0
		result.SoundIDs = append(result.SoundIDs, 30)
		if action == 21 {
			b.Animation.NextAction, b.Animation.HasNextAction = Action(action), true
			b.Body.Velocity.X = 8 * Pixel
			if b.FacingLeft {
				b.Body.Velocity.X = -8 * Pixel
			}
			b.Animation.Cursor = 3
			b.Body.Gravity = -160
		} else {
			body, err := court.launchPass(b.Body.Position, plan.Target, plan.ArcHeight)
			if err != nil {
				return initial, err
			}
			b.Body = body
			if action == 13 || action == 23 {
				b.Animation.NextAction, b.Animation.HasNextAction = Action(action), true
				b.Body.Velocity.Height, b.Body.Gravity = 0, 0
				if b.Body.Velocity.X>>8 >= 9 {
					b.Body.Velocity.X = 8*Pixel + (b.Body.Velocity.X & 255)
				} else if b.Body.Velocity.X>>8 < -8 {
					b.Body.Velocity.X = -8*Pixel + (b.Body.Velocity.X & 255)
				}
			}
		}
	}
	if err := animateOriginalSpecialBall(b, action); err != nil {
		return initial, err
	}
	if err := advanceBallOnCourt(b, court); err != nil {
		return initial, err
	}
	switch action {
	case 21:
		if b.Body.Velocity.Height >= 0 && specialBallPassedTurnLine(b, result.SelectedHoop, 16) {
			b.Action, b.Entered = BallAction(b.Animation.NextAction), true
			if err := redirectOriginalSpecialDrop(b); err != nil {
				return initial, err
			}
		}
		clampOriginalSpecialDescent(b)
	case 13, 23:
		if b.Animation.Cursor == 6 {
			b.Animation.Cursor, b.Animation.Elapsed = 4, 0
		} else if b.Animation.Cursor < 4 {
			if b.Body.Gravity == 0 {
				if specialBallPassedTurnLine(b, result.SelectedHoop, 160) {
					b.Action, b.Entered = BallAction(b.Animation.NextAction), true
					b.Body.Gravity = -96
				}
			} else if originalSpecialDropBand(*b) {
				if err := redirectOriginalSpecialDrop(b); err != nil {
					return initial, err
				}
				b.Animation.Cursor, b.Animation.Elapsed = 4, 0
			}
		}
		clampOriginalSpecialDescent(b)
	case 20:
		if b.Animation.Cursor < 5 {
			x := b.Body.Position.X >> 8
			if b.Body.Velocity.X >= 0 && x >= 464 || b.Body.Velocity.X < 0 && x < 64 {
				target := Fixed(464)
				if b.Body.Velocity.X < 0 {
					target = 64
				}
				b.Animation.Cursor = 5
				b.Body.Velocity = Velocity{}
				b.Body.Gravity = Pixel / 4
				b.Body.Position.X = target*Pixel + (b.Body.Position.X & 255)
				b.Body.Position.Depth = 45*Pixel + (b.Body.Position.Depth & 255)
			}
		}
	}
	if !b.Airborne {
		if action == 20 {
			b.BouncesRemaining = 1
		}
		b.Action, b.Entered = BallBounce, false
	}
	return result, nil
}
