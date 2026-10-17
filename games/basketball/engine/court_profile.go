package engine

import "fmt"

// CourtSurface retains both the original floor height and the independent
// depth-travel attributes. A floor-only approximation loses raised edges.
type CourtSurface uint8

func (s CourtSurface) Height() Fixed { return Fixed(uint8(s)&248) * Pixel }
func (s CourtSurface) Travel() uint8 { return uint8(s) & 7 }

type CourtTerrainProfile struct {
	Court        uint8
	TerrainRows  uint8
	MaximumDepth uint8 // Exclusive travel limit; row-table extent can be larger.
}

// OriginalCourtProfile describes $058B. It is not the $0588 scene/layout and
// does not imply that the court's complete gameplay or artwork is enabled.
func OriginalCourtProfile(court uint8) (CourtTerrainProfile, error) {
	if court >= 8 {
		return CourtTerrainProfile{}, fmt.Errorf("unknown original court %d", court)
	}
	limits := [8]uint8{73, 88, 73, 88, 80, 81, 72, 81}
	return CourtTerrainProfile{Court: court, TerrainRows: uint8(len(originalCourtSurfaceCells[court])), MaximumDepth: limits[court]}, nil
}

// OriginalCourtSurface restores the 8B1F/page/row/cell lookup without reading
// ROM at runtime. Reject positions beyond actual source rows rather than
// interpreting neighboring pointer tables as terrain.
func OriginalCourtSurface(court uint8, x, depth Fixed) (CourtSurface, error) {
	profile, e := OriginalCourtProfile(court)
	if e != nil {
		return 0, e
	}
	if x < 0 || x >= 512*Pixel || depth < 0 || depth >= Fixed(profile.TerrainRows)*8*Pixel {
		return 0, fmt.Errorf("court %d terrain point outside source table: x=%d depth=%d", court, x, depth)
	}
	return CourtSurface(originalCourtSurfaceCells[court][depth/(8*Pixel)][x/(8*Pixel)]), nil
}

type CourtTerrainProbe struct {
	Position      Position `json:"position"`
	Candidate     Position `json:"candidate"`
	Flags         uint8    `json:"flags"` // Original $05F7, including air (0x80) and edge (0x40).
	Action        uint8    `json:"action"`
	VelocityDepth Fixed    `json:"velocityDepth"`
}
type CourtTerrainProbeResult struct {
	State CourtTerrainProbe `json:"state"`
	// Previous XY, candidate XY, candidate X/previous depth, previous X/candidate depth.
	Samples [4]CourtSurface `json:"samples"`
}

// ProbeOriginalCourtTerrain restores 89FB through 8B1E. Its edge flags and
// one-pixel depth changes precede the separate floor/landing response.
func ProbeOriginalCourtTerrain(original CourtTerrainProbe, court uint8) (CourtTerrainProbeResult, error) {
	r := CourtTerrainProbeResult{State: original}
	points := [4][2]Fixed{{original.Position.X, original.Position.Depth}, {original.Candidate.X, original.Candidate.Depth}, {original.Candidate.X, original.Position.Depth}, {original.Position.X, original.Candidate.Depth}}
	for i, p := range points {
		cell, e := OriginalCourtSurface(court, p[0], p[1])
		if e != nil {
			return CourtTerrainProbeResult{State: original}, e
		}
		r.Samples[i] = cell
	}
	s := &r.State
	s.Flags &^= 64
	x := uint16(original.Position.X >> 8)
	depth := uint8(original.Position.Depth >> 8)
	height := uint8(original.Position.Height >> 8)
	threshold, ceiling := uint8(0), uint8(0)
	switch court {
	case 7:
		if depth == 79 && (x < 120 || x >= 392) {
			threshold, ceiling = 80, 128
		}
	case 5:
		if depth == 79 && ((x >= 104 && x < 112) || (x >= 192 && x < 200) || (x >= 256 && x < 264)) {
			threshold, ceiling = 120, 224
		} else if depth == 71 && x >= 352 && x < 448 {
			threshold, ceiling = 72, 128
		}
	}
	if threshold == 0 || height >= ceiling {
		return r, nil
	}
	if height < threshold {
		s.Flags |= 64
		if s.Action&127 == 14 {
			s.VelocityDepth = Pixel + Fixed(uint8(s.VelocityDepth))
		}
	} else {
		// Original instructions increment the integer depth byte only; fractions
		// and page overflow are not propagated through a 16-bit addition.
		s.Position.Depth = Fixed(uint8(depth+1))*Pixel + (s.Position.Depth & 255)
		s.Candidate.Depth = Fixed(uint8(uint8(s.Candidate.Depth>>8)+1))*Pixel + (s.Candidate.Depth & 255)
	}
	return r, nil
}

type CourtBoundaryState struct {
	Body    Body  `json:"body"`
	Action  uint8 `json:"action"`
	Entered bool  `json:"entered"`
	Control uint8 `json:"control"`
}

// ClampOriginalCourtTravel restores 8021..80C6 before terrain queries. Ball
// and auxiliary objects use 47/480 X limits; players/hoops use 55/472. The
// original compares X page bytes, including the explicit page-2 right clamp.
func ClampOriginalCourtTravel(original CourtBoundaryState, court, object uint8) (CourtBoundaryState, error) {
	profile, e := OriginalCourtProfile(court)
	if e != nil {
		return original, e
	}
	if object >= 13 {
		return original, fmt.Errorf("invalid court boundary object %d", object)
	}
	s := original
	p, v := &s.Body.Position, &s.Body.Velocity
	if p.X < -32768*Pixel || p.X >= 32768*Pixel || p.Depth < 0 || p.Depth >= 256*Pixel {
		return original, fmt.Errorf("court boundary position outside original byte range")
	}
	d := uint8(p.Depth >> 8)
	if d < 16 {
		p.Depth = 16*Pixel + (p.Depth & 255)
	} else if d >= profile.MaximumDepth {
		p.Depth = Fixed(profile.MaximumDepth-1)*Pixel + (p.Depth & 255)
	}
	left, right := uint8(55), uint8(216)
	if object >= 4 && object <= 6 {
		left, right = 47, 224
	}
	page, low := uint8(p.X>>16), uint8(p.X>>8)
	clamped := false
	if (page == 0 && low < left) || int8(page) < 0 {
		p.X = Fixed(left+1)*Pixel + (p.X & 255)
		clamped = true
	} else if page != 0 && (page == 2 || low >= right) {
		p.X = (256+Fixed(right-1))*Pixel + (p.X & 255)
		clamped = true
	}
	if clamped {
		v.X, v.Depth = 0, 0
		if object < 4 && s.Control&128 == 0 {
			switch s.Action {
			case 2, 18, 34, 50, 66, 82:
				s.Action = 0
				s.Entered = false
			}
		}
	}
	return s, nil
}
