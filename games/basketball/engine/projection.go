package engine

import "fmt"

type ProjectionState struct {
	Projected [13]ScreenPosition `json:"projected"`
	DrawOrder [13]uint8          `json:"drawOrder"`
	DrawCount uint8              `json:"drawCount"`
}

type ProjectionContext struct {
	CameraX uint16 `json:"cameraX"`
	CameraY uint16 `json:"cameraY"`
	// LayoutID is original $0588, not the terrain/AI variant at $058B.
	LayoutID  uint8        `json:"layoutID"`
	Poses     [13]uint8    `json:"poses"`
	Positions [13]Position `json:"positions"`
}

func validateProjection(s ProjectionState, c ProjectionContext) error {
	if c.LayoutID >= 8 || c.CameraY>>8 > 1 {
		return fmt.Errorf("unsupported court or camera page in original projection")
	}
	for _, i := range s.DrawOrder {
		if i != 255 && i >= 13 {
			return fmt.Errorf("invalid original draw-order object %d", i)
		}
	}
	return nil
}

// StepOriginalProjection restores the complete $D348..D352 object loop. The
// caller retains the original draw-list reset boundary; this does not clear
// existing entries or cached off-screen coordinates on its own.
func StepOriginalProjection(s ProjectionState, c ProjectionContext) (ProjectionState, error) {
	if err := validateProjection(s, c); err != nil {
		return s, err
	}
	result := s
	for i := uint8(0); i < 13; i++ {
		var err error
		result, err = projectOriginalObject(result, i, c)
		if err != nil {
			return s, err
		}
	}
	return result, nil
}

// ProjectOriginalObject restores fixed $CD3C and its $D0D9 draw-list insertion.
// Hidden poses leave both cached axes unchanged; horizontal culling updates X
// but deliberately preserves Y. Depth ordering is stable for equal depths.
func ProjectOriginalObject(s ProjectionState, object uint8, c ProjectionContext) (ProjectionState, error) {
	if object >= 13 {
		return s, fmt.Errorf("invalid projection object %d", object)
	}
	if err := validateProjection(s, c); err != nil {
		return s, err
	}
	return projectOriginalObject(s, object, c)
}

func projectOriginalObject(s ProjectionState, object uint8, c ProjectionContext) (ProjectionState, error) {
	if c.Poses[object]&0x7f == 0x7f {
		return s, nil
	}
	p := c.Positions[object]
	x := int16(uint16(p.X>>8) - c.CameraX)
	s.Projected[object].X = x
	if x < -48 || x > 303 {
		return s, nil
	}
	// The original low-byte table addition discards carry before subtracting
	// depth. Preserve that behavior instead of flattening the page encoding.
	pageBase := [2]uint8{208, 224}
	courtHigh := [8]uint8{1, 0, 1, 1, 1, 1, 1, 1}
	courtLow := [8]uint8{32, 0, 32, 32, 32, 32, 32, 32}
	baseLow := pageBase[c.CameraY>>8] + courtLow[c.LayoutID]
	base := uint16(baseLow) | uint16(1-courtHigh[c.LayoutID])<<8
	y := base - uint16(uint8(p.Depth>>8)) - c.CameraY - uint16(p.Height>>8)
	s.Projected[object].Y = int16(y)
	if (y+16)>>8 != 0 {
		return s, nil
	}
	if c.LayoutID == 2 {
		if s.DrawCount >= 13 {
			return s, fmt.Errorf("original court2 draw list overflow")
		}
		s.DrawOrder[s.DrawCount] = object
		s.DrawCount++
		return s, nil
	}
	depth := uint8(p.Depth >> 8)
	for index, other := range s.DrawOrder {
		if other == 255 {
			s.DrawOrder[index] = object
			break
		}
		if depth < uint8(c.Positions[other].Depth>>8) {
			pending := object
			for next := index; next < len(s.DrawOrder); next++ {
				displaced := s.DrawOrder[next]
				s.DrawOrder[next] = pending
				if displaced == 255 {
					break
				}
				pending = displaced
			}
			break
		}
	}
	return s, nil
}
