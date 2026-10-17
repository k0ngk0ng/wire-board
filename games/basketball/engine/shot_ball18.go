package engine

import "fmt"

// Ball18TailContext is read after EEFF, when devices may have changed the
// shared latch, random register or hoop. It is not a second saved state.
type Ball18TailContext struct {
	ContactLatch uint8
	Random       RandomState
	SelectedHoop int8
}
type OriginalBall18Step struct {
	OriginalShotBallStep
	ChangeContactLatch bool
	ContactLatch       uint8
	ChangeRandom       bool
	Random             RandomState
}

// PrepareOriginalBall18 restores BCAA..BCC2. World commits the entry commands
// before court physics: sound 30, launch metadata, latch 80 and released owner.
func PrepareOriginalBall18(original Ball, ctx OriginalShotBallContext, launch PassLaunchContext) (OriginalBall18Step, error) {
	initial := OriginalBall18Step{OriginalShotBallStep: OriginalShotBallStep{Ball: original, SpecialShot: ctx.SpecialShot, SelectedHoop: ctx.SelectedHoop, Target: ctx.Target, ArcHeight: ctx.ArcHeight, Landing: ctx.Landing}}
	result := initial
	b := &result.Ball
	if b.Action != 18 {
		return initial, fmt.Errorf("ball18 preparation action %d", b.Action)
	}
	if b.Entered {
		return result, nil
	}
	b.Entered = true
	b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 0, 0, 0
	result.SoundIDs = []uint8{30}
	result.SelectedHoop = selectOriginalBallHoop(*b)
	plan, clear, e := originalShotLaunchPlan(*b, ctx)
	if e != nil {
		return initial, e
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
	b.Body, e = LaunchPass(b.Body.Position, plan.Target, plan.ArcHeight, launch)
	if e != nil {
		return initial, e
	}
	b.BouncesRemaining = 6
	result.ChangeContactLatch, result.ContactLatch = true, 128
	result.ReleaseHolder = true
	return result, nil
}

func animateOriginalBall18(b *Ball) error {
	frame := func(p, t uint8) AnimationFrame { return AnimationFrame{Pose: Pose{Frame: p}, Ticks: t} }
	state := b.Animation
	tail := state.Cursor >= 7
	frames := []AnimationFrame{frame(0, 2), frame(127, 1), frame(0, 1), frame(15, 1), frame(16, 1), frame(127, 255)}
	if tail {
		state.Cursor -= 7
		frames = []AnimationFrame{frame(15, 1), frame(16, 1), frame(0, 255)}
	}
	out, e := AdvanceAnimation(AnimationSpec{Frames: frames, End: RepeatForever}, state, b.FacingLeft)
	if e != nil {
		return e
	}
	// FF loops to the beginning of the whole source sequence, not the tail.
	if tail && (out.State.Cursor != 0 || out.State.Elapsed != 0) {
		out.State.Cursor += 7
	}
	b.Animation, b.Pose = out.State, out.Pose
	return nil
}

// CompleteOriginalBall18 restores BCC2..BD13. loadTail must observe the world
// after motion. Entry commands are cleared and are never replayed over device
// changes. Only changed latch/random fields are committed by the caller.
func CompleteOriginalBall18(prepared OriginalBall18Step, court FlatCourt, loadTail func() Ball18TailContext) (OriginalBall18Step, error) {
	if prepared.Ball.Action != 18 || loadTail == nil {
		return prepared, fmt.Errorf("invalid ball18 completion context")
	}
	result := prepared
	result.SoundIDs = nil
	result.ReleaseHolder, result.ChangeSpecialShot, result.ClearUnownedSpecialShot = false, false, false
	result.ChangeContactLatch, result.ChangeRandom = false, false
	b := &result.Ball
	if e := animateOriginalBall18(b); e != nil {
		return prepared, e
	}
	if e := advanceBallOnCourt(b, court); e != nil {
		return prepared, e
	}
	ctx := loadTail()
	if int8(ctx.ContactLatch) < 0 {
		if b.Action != 18 {
			return prepared, fmt.Errorf("special ball18 post-motion turn selector %d not reconstructed", b.Action)
		}
		if !specialBallPassedTurnLine(b, ctx.SelectedHoop, 16) {
			if !b.Airborne {
				b.Action, b.Entered = 6, false
			}
			return result, nil
		}
		b.Action, b.Entered = 18, true
		b.Animation.Cursor, b.Animation.Elapsed = 7, 0
		result.ChangeContactLatch, result.ContactLatch = true, 0
		if e := redirectOriginalSpecialDrop(b); e != nil {
			return prepared, e
		}
		// EC59 immediately clears BAAB's temporary vertical speed and gravity.
		b.Body.Velocity.Height, b.Body.Gravity = 0, 0
		next, value := NextRandom(ctx.Random)
		result.ChangeRandom, result.Random = true, next
		heights := [4]Fixed{328, 232, 136, 136}
		b.Body.Position.Height = heights[value&3]*Pixel + (b.Body.Position.Height & 255)
	}
	b.BouncesRemaining--
	if int8(b.BouncesRemaining) < 0 {
		b.Action, b.Entered = 10, false
	}
	if !b.Airborne {
		b.Action, b.Entered = 6, false
	}
	return result, nil
}
