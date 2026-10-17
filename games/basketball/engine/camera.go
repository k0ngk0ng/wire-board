package engine

import "fmt"

// ScreenPosition is the original signed integer-pixel projection. Keep it apart
// from Position: depth and height still participate independently in physics.
type ScreenPosition struct {
	X int16 `json:"x"`
	Y int16 `json:"y"`
}

// CameraState retains displayed horizontal scroll separately from the next
// scroll. Original $D66C reads $0397/398 and writes $039B/39C; the display commit
// happens outside this routine. Y preserves the original two-page scroll
// encoding, including the 16-pixel gap between the pages.
type CameraState struct {
	DisplayedX uint16 `json:"displayedX"`
	X          uint16 `json:"x"`
	Y          uint16 `json:"y"`
	TileColumn uint8  `json:"tileColumn"`
	Shake      uint8  `json:"shake"`
}

type CameraContext struct {
	TrackedObject uint8              `json:"trackedObject"`
	BallHolder    uint8              `json:"ballHolder"`
	Positions     [13]Position       `json:"positions"`
	Projected     [13]ScreenPosition `json:"projected"`
}

type CameraStep struct {
	State CameraState
	// Original bank1:$8000 streams a background tile column here. Canvas can
	// draw the complete court instead; this signal is retained for verification.
	StreamColumn bool
}

// StepOriginalCamera restores fixed-bank $D66C. It does not move the players,
// commit the display scroll, advance NMI clocks or create background tile data.
func StepOriginalCamera(s CameraState, c CameraContext) (CameraStep, error) {
	r := CameraStep{State: s}
	if c.TrackedObject >= 13 || c.BallHolder > 6 {
		return r, fmt.Errorf("invalid camera tracked object or ball holder")
	}
	x := uint16(c.Positions[c.TrackedObject].X >> 8)
	left, right := uint16(uint8(s.DisplayedX))+112, uint16(uint8(s.DisplayedX))+144
	var amount uint16
	if x < left {
		amount = left - x
		if amount > 8 {
			amount = 8
		}
		r.State.X = s.DisplayedX - amount
		if r.State.X&0x8000 != 0 || uint8(r.State.X) < 6 {
			r.State.X = 6
		}
		r.State.TileColumn = uint8(s.DisplayedX)>>3 + uint8(s.DisplayedX>>8)*16
		r.StreamColumn = true
	} else if x > right {
		amount = x - right
		if amount > 8 {
			amount = 8
		}
		r.State.X = s.DisplayedX + amount
		if r.State.X>>8 != 0 {
			r.State.X = 255
		}
		r.State.TileColumn = (uint8(s.DisplayedX) >> 3) + uint8(s.DisplayedX>>8)*16 + 32
		r.StreamColumn = true
	}
	target := c.BallHolder
	if target == 6 {
		target = c.TrackedObject
	}
	y := uint16(c.Projected[target].Y)
	step := uint16(16)
	up, down := false, false
	if y>>8 == 0 {
		if y < 16 {
			step = 16 - y
			up = true
		} else if y >= 32 {
			step = y - 32
			down = true
		}
	} else if y&0x8000 != 0 {
		up = true
	} else {
		down = true
	}
	if down {
		r.State.Y += step
		lo, hi := uint8(r.State.Y), uint8(r.State.Y>>8)
		if hi == 0 {
			if lo&0xf0 == 0xf0 {
				lo += 16
				hi = 1
			}
		} else if lo >= 42 {
			lo = 41
		}
		r.State.Y = uint16(lo) | uint16(hi)<<8
	} else if up {
		r.State.Y -= step
		lo, hi := uint8(r.State.Y), uint8(r.State.Y>>8)
		if hi == 0 {
			if lo&0xf0 == 0xf0 {
				lo -= 16
			} else if lo < 48 {
				lo = 48
			}
		}
		r.State.Y = uint16(lo) | uint16(hi)<<8
	}
	if r.State.Shake != 0 {
		r.State.Shake--
		if r.State.Shake&1 != 0 {
			r.State.Y -= 2
		} else {
			r.State.Y += 2
		}
	}
	return r, nil
}

// OriginalCrowdingFlags restores bank3:$BF42. It counts later players in a
// vertical sprite band, ignoring horizontal distance once both projections
// are in the unsigned on-screen page. Only bit 7 is owned by this routine.
func OriginalCrowdingFlags(flags uint8, projected [4]ScreenPosition) uint8 {
	for i := 0; i < 3; i++ {
		p := projected[i]
		if uint16(p.X)>>8 != 0 || uint16(p.Y)>>8 != 0 {
			continue
		}
		count := 0
		for j := i + 1; j < 4; j++ {
			q := projected[j]
			if uint16(q.X)>>8 != 0 || uint16(q.Y)>>8 != 0 || uint8(q.Y) >= 192 {
				continue
			}
			lower, upper, candidate := uint8(p.Y), uint8(p.Y)+48, uint8(q.Y)+24
			if lower < candidate && candidate <= upper {
				count++
				if count == 2 {
					return flags | 0x80
				}
			}
		}
	}
	return flags & 0x7f
}
