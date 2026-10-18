package engine

import "fmt"

// The source held-ball sequence has a normal pose and two low-jump tails.
// A tail loops back to the normal sequence, while the owner can select its
// current tail cursor again on the following action call.
func animateOriginalHeldBall(b *Ball, owner Player) error {
	if b.Action == BallDribble {
		a, e := AdvanceAnimation(AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 127}, Ticks: 255}}, End: RepeatForever}, b.Animation, b.FacingLeft)
		if e != nil {
			return e
		}
		b.Animation, b.Pose = a.State, a.Pose
		return nil
	}
	if b.Action != BallHeld {
		return fmt.Errorf("held animation action %d outside source sequence", b.Action)
	}
	switch owner.Action {
	case 0x5a:
		b.Animation.Cursor = owner.Animation.Cursor + 2
	case 0x5c:
		b.Animation.Cursor = owner.Animation.Cursor + 9
	}
	offset := uint8(0)
	frames := []AnimationFrame{{Pose: Pose{Frame: 0}, Ticks: 255}}
	switch c := b.Animation.Cursor; {
	case c == 0:
	case c >= 2 && c < 8:
		offset = 2
		frames = []AnimationFrame{{Pose: Pose{Frame: 0, Mirrored: true}, Ticks: 1}, {Pose: Pose{Frame: 0}, Ticks: 1}, {Pose: Pose{Frame: 0}, Ticks: 1}, {Pose: Pose{Frame: 127}, Ticks: 1}, {Pose: Pose{Frame: 127}, Ticks: 1}, {Pose: Pose{Frame: 0, Mirrored: true}, Ticks: 1}}
	case c >= 9 && c < 13:
		offset = 9
		frames = []AnimationFrame{{Pose: Pose{Frame: 3}, Ticks: 1}, {Pose: Pose{Frame: 2}, Ticks: 1}, {Pose: Pose{Frame: 1}, Ticks: 1}, {Pose: Pose{Frame: 0}, Ticks: 1}}
	default:
		return fmt.Errorf("held animation cursor %d outside original sequences", c)
	}
	local := b.Animation
	local.Cursor -= offset
	a, e := AdvanceAnimation(AnimationSpec{Frames: frames, End: RepeatForever}, local, b.FacingLeft)
	if e != nil {
		return e
	}
	// Cursor0/elapsed0 can also mean the uint8 timer wrapped without advancing;
	// only a last-frame transition represents the source FF loop marker.
	looped := int(local.Cursor) == len(frames)-1 && a.State.Cursor == 0
	if offset != 0 && !looped {
		a.State.Cursor += offset
	}
	b.Animation, b.Pose = a.State, a.Pose
	return nil
}
