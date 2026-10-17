package engine

import "fmt"

// DisplayPaletteFade is the suspended original palette builder: four queues,
// each followed by two video interrupts. Frame is elapsed video commits, not
// main-loop updates or browser wall time. Pointers are replaced, never mutated,
// so copying a display state cannot mutate a saved/replay predecessor.
type DisplayPaletteFade struct {
	Out   bool
	Frame uint8
}

func displayPaletteLevel(level uint8) *uint8 { return &level }
func QueueDisplayPaletteLevel(original DisplayState, palette, level uint8) (DisplayState, error) {
	if palette == 0 || palette > 15 || level > 4 {
		return original, fmt.Errorf("invalid display palette or level")
	}
	if original.PaletteFade != nil {
		return original, fmt.Errorf("palette command during suspended fade")
	}
	s := original
	s.PendingPaletteID = palette
	s.PendingPaletteLevel = displayPaletteLevel(level)
	return s, nil
}
func QueueDisplayFade(original DisplayState, out bool) (DisplayState, error) {
	if original.PaletteFade != nil {
		return original, fmt.Errorf("display fade already active")
	}
	s := original
	s.PaletteFade = &DisplayPaletteFade{Out: out}
	level := uint8(1)
	if out {
		level = 3
	}
	s.PendingPaletteLevel = displayPaletteLevel(level)
	return s, nil
}
func commitDisplayPaletteFade(s *DisplayState) error {
	if e := validateDisplayPaletteFade(*s); e != nil {
		return e
	}
	if s.PendingPaletteLevel != nil {
		s.PaletteLevel = displayPaletteLevel(*s.PendingPaletteLevel)
		s.PendingPaletteLevel = nil
	}
	if s.PaletteFade == nil {
		return nil
	}
	f := *s.PaletteFade
	f.Frame++
	if f.Frame == 8 {
		s.PaletteFade = nil
		return nil
	}
	if f.Frame%2 == 0 {
		level := 1 + f.Frame/2
		if f.Out {
			level = 3 - f.Frame/2
		}
		s.PendingPaletteLevel = displayPaletteLevel(level)
	}
	s.PaletteFade = &f
	return nil
}
func validateDisplayPaletteFade(s DisplayState) error {
	for _, p := range []*uint8{s.PaletteLevel, s.PendingPaletteLevel, s.Visible.PaletteLevel, s.Submitted.PaletteLevel} {
		if p != nil && *p > 4 {
			return fmt.Errorf("invalid display palette level")
		}
	}
	if f := s.PaletteFade; f != nil {
		if f.Frame >= 8 {
			return fmt.Errorf("invalid display fade cursor")
		}
		if f.Frame%2 == 0 {
			next := 1 + f.Frame/2
			if f.Out {
				next = 3 - f.Frame/2
			}
			if s.PendingPaletteLevel == nil || *s.PendingPaletteLevel != next {
				return fmt.Errorf("inconsistent pending fade level")
			}
		} else if s.PendingPaletteLevel != nil {
			return fmt.Errorf("unexpected pending fade level")
		}
		if f.Frame > 0 {
			current := 1 + (f.Frame-1)/2
			if f.Out {
				current = 3 - (f.Frame-1)/2
			}
			if s.PaletteLevel == nil || *s.PaletteLevel != current {
				return fmt.Errorf("inconsistent committed fade level")
			}
		}
	}
	return nil
}
