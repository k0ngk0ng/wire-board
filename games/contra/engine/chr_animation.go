package contra

import "errors"

// PatternPages separates simulation's desired pages from the currently visible
// pages. The world commits at the display boundary before advancing new rules.
type PatternPages struct {
	Desired [8]uint8
	Applied [8]uint8
}

func (p *PatternPages) Commit() { p.Applied = p.Desired }

// CHRAnimation owns only the original animation timer/phase/pace. Scene flags,
// the mutable alternate marker and the shared random word have other owners.
type CHRAnimation struct{ Timer, Phase, Pace uint8 }
type CHRAnimationContext struct {
	Level, SceneType, Alternate, Random, Screen, Counter uint8
}

type chrAnimationRow struct {
	slot  uint8
	pages [4]uint8
}

// Japanese C8DF: eight level rows plus the lair screen-12 presentation row.
var chrAnimationRows = [...]chrAnimationRow{
	{5, [4]uint8{34, 33, 32, 69}},
	{7, [4]uint8{105, 106, 105, 106}},
	{7, [4]uint8{37, 36, 35, 99}},
	{7, [4]uint8{105, 106, 105, 106}},
	{5, [4]uint8{110, 109, 108, 81}},
	{4, [4]uint8{64, 65, 66, 67}},
	{4, [4]uint8{0, 0, 0, 0}},
	{4, [4]uint8{118, 117, 116, 92}},
	{6, [4]uint8{102, 101, 100, 66}},
}

// Step follows original C83B and its per-stage dispatch. It changes only desired
// pages; neither a just-written page nor a transition is presented early.
func (s *CHRAnimation) Step(c CHRAnimationContext, transition *uint8, desired *[8]uint8) error {
	if c.Level >= 8 || transition == nil || desired == nil {
		return errors.New("invalid pattern animation context")
	}
	next, pages, changing := *s, *desired, *transition
	if changing != 0 {
		changing, next.Timer, pages[7] = 0x80, 0, 75
		*s, *desired, *transition = next, pages, changing
		return nil
	}
	next.Timer--
	if next.Timer&0x80 == 0 {
		*s = next
		return nil
	}
	row := c.Level
	draw := true
	switch c.Level {
	case 0:
		next.Timer = (c.Random & 7) + 4
	case 1, 3:
		if c.SceneType&0x80 != 0 {
			draw = false
		} else {
			next.Timer = 4
		}
	case 2:
		if c.Alternate&0x80 != 0 {
			draw = false
		} else {
			next.Timer = (c.Random & 1) + 6
		}
	case 4:
		next.Timer = (c.Random & 3) + 1 + next.Pace
	case 5, 6:
		draw = false
	case 7:
		oldPace := next.Pace
		if oldPace != 0 {
			next.Pace++
		}
		if oldPace != 0 && next.Pace != 0 {
			// INC changes memory but retains the old accumulator value used
			// by C8AB to reload the timer.
			next.Timer = oldPace
		} else if distance := uint8(12 - c.Screen); distance != 0 {
			next.Timer = distance
		} else if c.Counter&7 != 0 {
			draw = false
		} else {
			row = 8 // C8AF bypasses the timer reload in this branch.
		}
	}
	if draw {
		next.Phase--
		if next.Phase&0x80 != 0 {
			next.Phase = 3
		}
		if next.Phase > 3 {
			return errors.New("invalid animated pattern phase")
		}
		pattern := chrAnimationRows[row]
		pages[pattern.slot] = pattern.pages[next.Phase]
		if c.Level == 7 && c.Alternate&0x80 == 0 {
			pages[7] = [...]uint8{54, 53, 52, 95}[next.Phase]
		}
	}
	*s, *desired, *transition = next, pages, changing
	return nil
}
