package engine

import "fmt"

// ResultActorContext retains scene fields used by original menu/result scripts.
// It accompanies OpeningState; it is not gameplay animation or a second world.
type ResultActorContext struct {
	TextRequestFlags uint8    `json:"textRequestFlags"` // 06AB; consumed by the scene text processor
	TextID           uint8    `json:"textID"`           // 06AD
	PaletteIDs       [2]uint8 `json:"paletteIDs"`       // 0305/0306
	DisplayMask      uint8    `json:"displayMask"`      // 0304
}
type ResultActorStep struct {
	State    OpeningState
	Context  ResultActorContext
	SoundIDs []uint8
}

// StepOriginalResultActor restores C033/9598 for a result/menu actor, including
// initialization, contextual commands, 9769 motion and C00C projection. The
// caller owns draw-list reset and actor order. Failed calls preserve both inputs.
func StepOriginalResultActor(original OpeningState, context ResultActorContext, actor uint8) (ResultActorStep, error) {
	initial := ResultActorStep{State: original, Context: context}
	if actor >= 4 {
		return initial, fmt.Errorf("invalid result actor %d", actor)
	}
	r := initial
	t := &r.State.Transition
	o := &t.Objects[actor]
	if !o.Entered {
		o.Entered = true
		o.Animation.Cursor = 0
		o.Animation.Elapsed = 0
	}
	sounds, err := stepOriginalResultActorAnimation(&r.State, &r.Context, actor)
	if err != nil {
		return initial, err
	}
	b := &o.Body
	b.Position.X = signed24(b.Position.X + signed16(b.Velocity.X))
	b.Position.Depth = Fixed(uint16(b.Position.Depth + b.Velocity.Depth))
	b.Velocity.Height = signed16(b.Velocity.Height - b.Gravity)
	// 9769 leaves the high byte of height unchanged, including its sign.
	b.Position.Height = (b.Position.Height &^ 65535) | Fixed(uint16(b.Position.Height+b.Velocity.Height))
	pc := ProjectionContext{CameraX: t.CameraScroll[2], CameraY: t.CameraScroll[3], LayoutID: t.Scene}
	for i, obj := range t.Objects {
		pc.Positions[i] = obj.Body.Position
		pc.Poses[i] = obj.Pose.Frame
		if obj.Pose.Mirrored {
			pc.Poses[i] |= 128
		}
	}
	r.State.Projection, err = ProjectOriginalObject(r.State.Projection, actor, pc)
	if err != nil {
		return initial, err
	}
	r.SoundIDs = sounds
	return r, nil
}

// These scripts use countdowns and two-byte cursor slots, independently of
// gameplay animation. F8 reloads the program immediately; F9 does not reload
// this call's program, even when its target happens to be the current actor.
func stepOriginalResultActorAnimation(s *OpeningState, c *ResultActorContext, actor uint8) ([]uint8, error) {
	o := &s.Transition.Objects[actor]
	if o.Animation.Elapsed > 0 {
		o.Animation.Elapsed--
		return nil, nil
	}
	program, ok := originalResultActorPrograms[o.Action]
	if !ok {
		return nil, fmt.Errorf("unrestored result animation action %d", o.Action)
	}
	o.Pose.Frame = 0
	var sounds []uint8
	for n := 0; n < 256; n++ {
		command, ok := program[o.Animation.Cursor]
		if !ok {
			return nil, fmt.Errorf("invalid result animation action %d cursor %d", o.Action, o.Animation.Cursor)
		}
		o.Animation.Cursor++
		if command.Kind == resultPose {
			pose := command.Args[0]
			if o.Pose.Mirrored {
				pose ^= 128
			}
			o.Pose = Pose{Frame: pose & 127, Mirrored: pose&128 != 0}
			o.Animation.Elapsed = command.Args[1]
			return sounds, nil
		}
		reload, emitted, err := applyOriginalResultActorCommand(s, c, actor, command)
		if err != nil {
			return nil, err
		}
		sounds = append(sounds, emitted...)
		if reload {
			program, ok = originalResultActorPrograms[o.Action]
			if !ok {
				return nil, fmt.Errorf("unrestored result animation action %d", o.Action)
			}
			o.Pose.Frame = 0
		}
	}
	return nil, fmt.Errorf("result animation command cycle")
}
