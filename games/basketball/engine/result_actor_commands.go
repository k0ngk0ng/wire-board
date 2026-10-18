package engine

import "fmt"

type resultActorCommandKind uint8

const resultPose resultActorCommandKind = 0
const (
	resultPosition resultActorCommandKind = 0xf0 + iota
	resultVelocity
	resultHeight
	resultHeightVelocity
	resultCharacter
	resultText
	resultSound
	resultPalettes
	resultAction
	resultOtherAction
	resultRepeat
	resultCamera
	resultPhase
	resultDisplayMask
)

type resultActorCommand struct {
	Kind resultActorCommandKind
	Args [5]uint8
}

// The script fetch has already advanced one cursor slot. Padding operands in
// four/six-byte commands consume the additional original slots, even when the
// handler ignores their values. No instruction bytes execute in the Go engine.
func applyOriginalResultActorCommand(s *OpeningState, c *ResultActorContext, actor uint8, command resultActorCommand) (bool, []uint8, error) {
	if actor >= 4 {
		return false, nil, fmt.Errorf("invalid result actor %d", actor)
	}
	t := &s.Transition
	o := &t.Objects[actor]
	a := command.Args
	signedWord := func(high, low uint8) Fixed { return Fixed(int16(uint16(high)<<8 | uint16(low))) }
	switch command.Kind {
	case resultPosition:
		o.Body.Position.X = signed24(Fixed(a[0])<<16 | Fixed(a[1])<<8 | (o.Body.Position.X & 255))
		o.Body.Position.Depth = Fixed(a[2])<<8 | (o.Body.Position.Depth & 255)
		o.Animation.Cursor++
	case resultVelocity:
		o.Body.Velocity.X = signedWord(a[0], a[1])
		o.Body.Velocity.Depth = signedWord(a[2], a[3])
		o.Animation.Cursor += 2
	case resultHeight:
		o.Body.Position.Height = signed24(Fixed(a[0])<<16 | Fixed(a[1])<<8 | (o.Body.Position.Height & 255))
		o.Animation.Cursor++
	case resultHeightVelocity:
		o.Body.Velocity.Height = signedWord(a[0], a[1])
		o.Body.Gravity = Fixed(a[2])
		o.Animation.Cursor++
	case resultCharacter:
		t.Characters[actor] = a[0]
	case resultText:
		c.TextRequestFlags = 128
		c.TextID = a[0]
	case resultSound:
		return false, []uint8{a[0]}, nil
	case resultPalettes:
		c.PaletteIDs = [2]uint8{a[0], a[1]}
		o.Animation.Cursor++
	case resultAction:
		o.Action, o.Entered = a[0]&127, a[0]&128 != 0
		o.Animation.Cursor = 0
		return true, nil, nil
	case resultOtherAction:
		if a[0] >= 4 {
			return false, nil, fmt.Errorf("invalid result actor target %d", a[0])
		}
		target := &t.Objects[a[0]]
		target.Action, target.Entered = a[1]&127, a[1]&128 != 0
		target.Animation.Cursor, target.Animation.Elapsed = 0, 0
		o.Animation.Cursor++
	case resultRepeat:
		repeat := &t.EndActorFlags[actor]
		if *repeat&128 == 0 {
			*repeat |= 128 | a[1]
			o.Animation.Cursor = a[0]
		} else {
			*repeat++
			if *repeat != 0 {
				o.Animation.Cursor = a[0]
			} else {
				o.Animation.Cursor++
			}
		}
	case resultCamera:
		// FB adds an unsigned X byte and carry to the high byte. Its vertical
		// F0-page retry adds 16, reloads the same delta, and adds it again.
		t.CameraScroll[2] += uint16(a[0])
		y := uint8(t.CameraScroll[3])
		for attempts := 0; ; attempts++ {
			if attempts == 256 {
				return false, nil, fmt.Errorf("result camera command cycle")
			}
			y += a[1]
			if y&0xf0 != 0xf0 {
				break
			}
			y += 0x10
		}
		t.CameraScroll[3] = t.CameraScroll[3]&0xff00 | uint16(y)
		o.Animation.Cursor++
	case resultPhase:
		t.Match.Phase, t.Match.Entered = MatchPhase(a[0]&127), a[0]&128 != 0
	case resultDisplayMask:
		c.DisplayMask = a[0]
	default:
		return false, nil, fmt.Errorf("unknown result actor command %x", command.Kind)
	}
	return false, nil, nil
}
