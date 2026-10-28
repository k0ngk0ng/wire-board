package contra

import "errors"

// SnowControlState owns CE59's visibility value and byte cursor into the four
// original screen thresholds. CHR pace remains in the single CHRAnimation.
type SnowControlState struct{ Visibility, Cursor uint8 }

var snowVisibilitySteps = [...]struct{ screen, scroll, value uint8 }{
	{0, 0, 1}, {2, 0, 0}, {3, 128, 1}, {17, 0, 0},
}

// Advance follows CE59. Even after a large camera jump it consumes just one
// threshold per call. The terminal FF marker is valid for ordinary stage maps.
func (s *SnowControlState) Advance(level, screen, scroll uint8, animation *CHRAnimation) (bool, error) {
	if level != 4 {
		return false, nil
	}
	if animation == nil || s.Cursor%3 != 0 || s.Cursor > 12 || s.Cursor == 12 && screen == 255 {
		return false, errors.New("invalid snow visibility context")
	}
	if s.Cursor == 12 {
		return false, nil
	}
	step := snowVisibilitySteps[s.Cursor/3]
	if screen < step.screen || screen == step.screen && scroll < step.scroll {
		return false, nil
	}
	next := *s
	next.Visibility = step.value
	next.Cursor += 3
	pace := uint8(3)
	if step.value&1 != 0 {
		pace = 1
	}
	*s, animation.Pace = next, pace
	return true, nil
}
