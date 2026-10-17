package engine

import "fmt"

// RiderLinkObject is the subset read by bank 5 $809A. Props occupy slots 4/5;
// ordinary rider candidates are only slots 0..3. Coordinates retain fractions
// because the descending height test carries the low byte into its prediction.
type RiderLinkObject struct {
	Position          Position
	HeightVelocity    Fixed
	Kind              uint8
	Action            Action
	Entered, Airborne bool
	Control, AIMode   uint8
}
type RiderLinkState struct {
	Objects [6]RiderLinkObject
	Links   [7]uint8
}

func canAttemptOriginalRiding(action Action) bool {
	switch action {
	case 0x04, 0x05, 0x14, 0x15, 0x24, 0x25, 0x34:
		return true
	}
	return false
}
func originalSupportHeight(kind uint8) (uint16, error) {
	heights := [16]uint16{29, 29, 29, 29, 29, 29, 29, 29, 0, 0, 16, 40, 16, 16, 12, 16}
	if int(kind) >= len(heights) {
		return 0, fmt.Errorf("rider support kind %d is not reconstructed", kind)
	}
	return heights[kind], nil
}
func canSupportOriginalRider(kind uint8, action Action) (bool, error) {
	// $9301 points kinds 0..7/15 at $9321, kind 8 at $938B,
	// and kinds 9..14 at $93A3. Only bit 0x40 participates here.
	if kind <= 7 || kind == 15 {
		if action >= 106 {
			return false, fmt.Errorf("rider player action %d is outside original table", action)
		}
		switch action {
		case 0, 1, 2, 3, 0x10, 0x11, 0x12, 0x13, 0x3d, 0x40, 0x41, 0x42, 0x43:
			return true, nil
		}
		return false, nil
	}
	if kind == 8 {
		if action >= 24 {
			return false, fmt.Errorf("rider ball action %d is outside original table", action)
		}
		return false, nil
	}
	if kind >= 9 && kind <= 14 {
		if action >= 21 {
			return false, fmt.Errorf("rider prop action %d is outside original table", action)
		}
		switch action {
		case 0, 1, 4, 5, 6, 9, 10:
			return true, nil
		}
		return false, nil
	}
	return false, fmt.Errorf("rider support kind %d is not reconstructed", kind)
}
func crossesOriginalSupport(rider, support RiderLinkObject) (bool, error) {
	depth := uint8(rider.Position.Depth >> 8)
	targetDepth := uint8(support.Position.Depth >> 8)
	if targetDepth >= depth+8 || targetDepth < depth-7 {
		return false, nil
	}
	x := uint16(rider.Position.X >> 8)
	targetX := uint16(support.Position.X >> 8)
	if targetX > x+8 || targetX <= x-9 {
		return false, nil
	}
	height, err := originalSupportHeight(support.Kind)
	if err != nil {
		return false, err
	}
	top := uint16(support.Position.Height>>8) + height
	// $815A adds the signed downward velocity before discarding the fraction;
	// negative predicted heights clamp to zero for ground-level props.
	next := uint16(signed24(rider.Position.Height+rider.HeightVelocity) >> 8)
	if next&0x8000 != 0 {
		next = 0
	}
	return next <= top && uint16(rider.Position.Height>>8) >= top, nil
}

// StepOriginalRiderLinks restores the phase after contact search and before
// combat. It changes only action/entered and link fields. Input/AI, motion and
// attached-player action execution are separate phases, not simulated here.
func StepOriginalRiderLinks(original RiderLinkState) (RiderLinkState, error) {
	s := original
	groups := [6]uint8{0x41, 0x41, 0x82, 0x82, 0xc8, 0xd0}
	for source := 0; source < 4; source++ {
		rider := s.Objects[source]
		if !canAttemptOriginalRiding(rider.Action) {
			continue
		}
		if rider.Control&128 != 0 && rider.AIMode&127 != 0x1d && rider.AIMode&127 != 0x22 {
			continue
		}
		if !rider.Airborne {
			s.Links[source] = 0
			continue
		}
		if s.Links[source] != 0 || uint16(rider.HeightVelocity)&0x8000 == 0 {
			continue
		}
		found := uint8(0)
		for target := 0; target < 6; target++ {
			if source == target || groups[source]&groups[target]&0xc0 == 0 {
				continue
			}
			if rider.Control&128 != 0 && target >= 4 {
				continue
			}
			support := s.Objects[target]
			if support.Kind == 13 {
				continue
			}
			if support.Kind == 12 && (rider.Action == 0x24 || rider.Action == 0x25) {
				continue
			}
			enabled, err := canSupportOriginalRider(support.Kind, support.Action)
			if err != nil {
				return original, err
			}
			if !enabled {
				continue
			}
			crosses, err := crossesOriginalSupport(rider, support)
			if err != nil {
				return original, err
			}
			if !crosses {
				continue
			}
			link := 0x80 | uint8(target)
			occupied := false
			for i, existing := range s.Links {
				if i != 4 && existing == link {
					occupied = true
					break
				}
			}
			if occupied {
				continue
			}
			if s.Links[target]&128 != 0 && s.Links[target]&15 == uint8(source) {
				continue
			}
			s.Objects[source].Action = 0x3d
			if support.Kind == 12 {
				s.Objects[source].Action = 0x40
			}
			s.Objects[source].Entered = false
			found = link
			break
		}
		s.Links[source] = found
	}
	return s, nil
}
