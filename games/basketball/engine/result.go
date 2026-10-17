package engine

import "fmt"

// OriginalResultStep includes BA40's result decision and its four BC96 actor
// calls. A changed scene/phase describes a boundary, not a completed menu.
type OriginalResultStep struct {
	State      OpeningState
	Transition MatchTransitionStep
}

func StepOriginalResultPresentation(original OpeningState, buttons uint8) (OriginalResultStep, error) {
	core, err := StepOriginalMatchResult(original.Transition, buttons)
	if err != nil {
		return OriginalResultStep{State: original}, err
	}
	r := OriginalResultStep{State: original, Transition: core}
	r.State.Transition = core.State
	for i := uint8(0); i < 4; i++ {
		sounds, err := stepOriginalResultActor(&r.State, i)
		if err != nil {
			return OriginalResultStep{State: original}, err
		}
		r.Transition.SoundIDs = append(r.Transition.SoundIDs, sounds...)
	}
	r.Transition.State = r.State.Transition
	return r, nil
}

func stepOriginalResultActor(s *OpeningState, actor uint8) ([]uint8, error) {
	t := &s.Transition
	o := &t.Objects[actor]
	if !o.Entered {
		o.Entered = true
		o.Animation.Cursor = 0
		o.Animation.Elapsed = 0
	}
	sounds, err := stepOriginalResultAnimation(o, &t.EndActorFlags[actor])
	if err != nil {
		return nil, err
	}
	b := &o.Body
	b.Position.X = signed24(b.Position.X + signed16(b.Velocity.X))
	b.Position.Depth = Fixed(uint16(b.Position.Depth + b.Velocity.Depth))
	b.Velocity.Height = signed16(b.Velocity.Height - b.Gravity)
	// 9769 deliberately leaves the high byte of height unchanged.
	b.Position.Height = (b.Position.Height &^ 65535) | Fixed(uint16(b.Position.Height+b.Velocity.Height))
	pc := ProjectionContext{CameraX: t.CameraScroll[2], CameraY: t.CameraScroll[3], LayoutID: t.Scene}
	for i, obj := range t.Objects {
		pc.Positions[i] = obj.Body.Position
		pc.Poses[i] = obj.Pose.Frame
		if obj.Pose.Mirrored {
			pc.Poses[i] |= 128
		}
	}
	s.Projection, err = ProjectOriginalObject(s.Projection, actor, pc)
	return sounds, err
}

// These are the semantic victory/defeat sequences at 9FF2/A004, not the
// gameplay animation interpreter. Cursor is the original two-byte slot;
// Elapsed is a countdown, and repetition belongs to the result actor flag.
func stepOriginalResultAnimation(o *PeriodObject, repeat *uint8) ([]uint8, error) {
	if o.Animation.Elapsed > 0 {
		o.Animation.Elapsed--
		return nil, nil
	}
	var sounds []uint8
	frame := func(p, ticks uint8) { o.Pose.Frame = p; o.Animation.Elapsed = ticks; o.Animation.Cursor++ }
	change := func(action uint8) { o.Action = action; o.Entered = false; o.Animation.Cursor = 0 }
	loop := func(target, initial uint8) {
		o.Animation.Cursor++
		if *repeat&128 == 0 {
			*repeat |= 128 | initial
			o.Animation.Cursor = target
		} else {
			*repeat++
			if *repeat != 0 {
				o.Animation.Cursor = target
			} else {
				o.Animation.Cursor++
			}
		}
	}
	for commands := 0; commands < 16; commands++ {
		switch o.Action {
		case 0:
			if o.Animation.Cursor == 0 {
				frame(127, 48)
				return sounds, nil
			}
			if o.Animation.Cursor == 1 {
				change(0)
				continue
			}
		case 0x5b, 0x60:
			switch o.Animation.Cursor {
			case 0:
				o.Body.Position.Height &= 255
				o.Animation.Cursor = 2
				continue
			case 2:
				if o.Action == 0x5b {
					frame(28, 1)
				} else {
					frame(28, 8)
				}
				return sounds, nil
			case 3:
				if o.Action == 0x5b {
					change(0x60)
					continue
				}
				frame(29, 8)
				return sounds, nil
			case 4:
				if o.Action == 0x60 {
					change(0x60)
					continue
				}
			}
		case 0x61:
			switch o.Animation.Cursor {
			case 0, 3, 7, 9:
				frame(25, 3)
				return sounds, nil
			case 2, 8:
				frame(26, 3)
				return sounds, nil
			case 4, 10:
				frame(27, 3)
				return sounds, nil
			case 1:
				sounds = append(sounds, 8)
				o.Animation.Cursor++
				continue
			case 5:
				loop(0, 0xfe)
				continue
			case 11:
				loop(7, 0x80)
				continue
			}
		case 0x62, 0x63, 0x64:
			// BA40 calls the first reward pose on its exit. Continuing phase 7 is
			// intentionally unsupported until the independent reward stage is restored.
			if o.Animation.Cursor == 0 {
				o.Body.Position.Height &= 255
				o.Animation.Cursor = 2
				continue
			}
			if o.Animation.Cursor == 2 {
				poses := [3]uint8{9, 10, 7}
				ticks := [3]uint8{8, 24, 24}
				frame(poses[o.Action-0x62], ticks[o.Action-0x62])
				return sounds, nil
			}
		}
		return nil, fmt.Errorf("unrestored result animation action %d cursor %d", o.Action, o.Animation.Cursor)
	}
	return nil, fmt.Errorf("result animation command cycle")
}
