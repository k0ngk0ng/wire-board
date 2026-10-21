package engine

import "fmt"

// OriginalRasterSurface is committed presentation memory, never a pending
// upload queue. Mirroring0 maps A,B,A,B; mirroring1 maps A,A,B,B.
type OriginalRasterSurface struct {
	Nametables [2][1024]uint8 `json:"nametables"`
	Palette    [32]uint8      `json:"palette"`
	Mirroring  uint8          `json:"mirroring"`
}

type OriginalRasterTimingMode string

const (
	OriginalRasterObserved  OriginalRasterTimingMode = "source-observed"
	OriginalRasterCanonical OriginalRasterTimingMode = "canonical"
	OriginalRasterMask      OriginalRasterEffectKind = "mask"
)

// Dot is relative to this frame's first visible tile-fetch origin, not a CPU
// address. The caller positions source handler effects with an explicit timing
// policy. Canonical plans must never be labeled source-observed pixel timing.
type OriginalRasterTimedEffect struct {
	Dot   int32                    `json:"dot"`
	Kind  OriginalRasterEffectKind `json:"kind"`
	Value uint16                   `json:"value"`
}
type OriginalRasterFramePlan struct {
	Source       OriginalPostResultRasterSource `json:"source"`
	OddFrame     bool                           `json:"oddFrame"`
	TimingMode   OriginalRasterTimingMode       `json:"timingMode"`
	TimingSource string                         `json:"timingSource"`
	Effects      []OriginalRasterTimedEffect    `json:"effects"`
}

type OriginalRasterSegment struct {
	X       uint16 `json:"x"`
	Length  uint8  `json:"length"`
	Page    uint8  `json:"page"`
	Tile    uint8  `json:"tile"`
	Row     uint8  `json:"row"`
	Column  uint8  `json:"column"`
	Palette uint8  `json:"palette"`
	Mask    uint8  `json:"mask"`
}
type OriginalRasterRow struct {
	Y         uint16                  `json:"y"`
	Segments  []OriginalRasterSegment `json:"segments"`
	SpriteCHR [4]uint8                `json:"spriteCHR"`
	Mask      uint8                   `json:"mask"`
}
type OriginalRasterFrameRows struct {
	Palette [32]uint8              `json:"palette"`
	Rows    [240]OriginalRasterRow `json:"rows"`
}

type originalRasterRowSource struct{ page, tile, row, palette uint8 }
type originalRasterProjection struct {
	surface                     OriginalRasterSurface
	v, t                        uint16
	fine, control, mask, mirror uint8
	pages                       [8]uint8
	fetched                     [34]originalRasterRowSource
	fetchedCount                int
	fixed                       bool
	segments                    []OriginalRasterSegment
}

func validateOriginalRasterRows(surface OriginalRasterSurface, p OriginalRasterFramePlan) error {
	if p.TimingMode != OriginalRasterObserved && p.TimingMode != OriginalRasterCanonical {
		return fmt.Errorf("raster timing mode must be explicit")
	}
	if p.TimingSource == "" {
		return fmt.Errorf("raster timing provenance is required")
	}
	if surface.Mirroring > 1 {
		return fmt.Errorf("invalid raster mirroring")
	}
	if p.Source.Control&32 != 0 {
		return fmt.Errorf("post-result raster supports original 8x8 sprites only")
	}
	if p.Source.Mask&224 != 0 {
		return fmt.Errorf("unrestored post-result emphasis colors")
	}
	for _, v := range surface.Palette {
		if v >= 64 {
			return fmt.Errorf("invalid raster palette index")
		}
	}
	for _, page := range append(p.Source.BackgroundCHR[:], p.Source.SpriteCHR[:]...) {
		if page >= 128 {
			return fmt.Errorf("invalid raster CHR page")
		}
	}
	previous := int32(-16384)
	for _, e := range p.Effects {
		if e.Dot < previous || e.Dot > 240*341 {
			return fmt.Errorf("raster effects must be ordered within one frame")
		}
		previous = e.Dot
		switch e.Kind {
		case OriginalRasterAddress:
			if e.Value > 0x3fff {
				return fmt.Errorf("invalid raster address")
			}
		case OriginalRasterScrollX, OriginalRasterScrollY, OriginalRasterControl:
			if e.Value > 255 {
				return fmt.Errorf("invalid raster byte effect")
			}
		case OriginalRasterMask:
			if e.Value > 31 {
				return fmt.Errorf("unrestored raster mask effect")
			}
		case OriginalRasterMirroring:
			if e.Value > 1 {
				return fmt.Errorf("invalid raster mirror effect")
			}
		case OriginalRasterCHR0, OriginalRasterCHR1, OriginalRasterCHR2, OriginalRasterCHR3, OriginalRasterCHR4, OriginalRasterCHR5:
			if e.Value >= 128 {
				return fmt.Errorf("invalid raster CHR effect")
			}
		default:
			return fmt.Errorf("unknown raster effect %q", e.Kind)
		}
	}
	return nil
}

// ProjectOriginalPostResultRasterRows is a pure tile-row projection. It reads
// no ROM, CPU program, captured timeline, screenshot, clock or global state.
// It does not decide IRQ arrival or advance game logic. The supplied semantic
// changes preserve address-reset/fine-scroll ordering and two fetched rows.
func ProjectOriginalPostResultRasterRows(surface OriginalRasterSurface, p OriginalRasterFramePlan) (OriginalRasterFrameRows, error) {
	var result OriginalRasterFrameRows
	if err := validateOriginalRasterRows(surface, p); err != nil {
		return result, err
	}
	r := originalRasterProjection{surface: surface, control: p.Source.Control, mask: p.Source.Mask, mirror: surface.Mirroring, fine: uint8(p.Source.CameraX) & 7}
	r.t = uint16((p.Source.CameraX&248)>>3) | uint16((p.Source.CameraY&248)<<2) | uint16((p.Source.CameraY&7)<<12) | uint16((p.Source.CameraX>>8&1)<<10) | uint16((p.Source.CameraY>>8&1)<<11)
	for i, page := range p.Source.BackgroundCHR {
		r.pages[i*2] = page & 254
		r.pages[i*2+1] = page&254 + 1
	}
	copy(r.pages[4:], p.Source.SpriteCHR[:])
	at := 0
	for at < len(p.Effects) && p.Effects[at].Dot < 0 {
		r.apply(p.Effects[at])
		at++
	}
	r.v = r.t
	odd := int32(0)
	if p.OddFrame {
		odd = 1
	}
	for y := int32(0); y < 240; y++ {
		start := y * 341
		if y > 0 {
			start -= odd
		}
		end := start + 272
		if y == 0 {
			end -= odd
		}
		r.fetchedCount = 0
		r.fixed = false
		r.segments = make([]OriginalRasterSegment, 0, 34)
		for at < len(p.Effects) && p.Effects[at].Dot <= end {
			e := p.Effects[at]
			r.advance(e.Dot - start)
			r.apply(e)
			at++
		}
		r.advance(272)
		if !r.fixed {
			r.incrementY()
			r.fixed = true
		}
		row := OriginalRasterRow{Y: uint16(y), Segments: r.segments, Mask: r.mask}
		copy(row.SpriteCHR[:], r.pages[4:])
		result.Rows[y] = row
		horizontal := end + 6
		for at < len(p.Effects) && p.Effects[at].Dot <= horizontal {
			r.apply(p.Effects[at])
			at++
		}
		r.v = r.v&0xfbe0 | r.t&0x041f
		next := (y+1)*341 - odd
		for at < len(p.Effects) && p.Effects[at].Dot < next {
			r.apply(p.Effects[at])
			at++
		}
	}
	result.Palette = surface.Palette
	return result, nil
}

func (r *originalRasterProjection) apply(e OriginalRasterTimedEffect) {
	value := uint8(e.Value)
	switch e.Kind {
	case OriginalRasterAddress:
		r.t = e.Value
		r.v = e.Value
	case OriginalRasterScrollX:
		r.t = r.t&0xffe0 | uint16(value>>3)
		r.fine = value & 7
	case OriginalRasterScrollY:
		r.t = r.t&0x8c1f | uint16(value&248)<<2 | uint16(value&7)<<12
	case OriginalRasterControl:
		r.control = value
		r.t = r.t&0xf3ff | uint16(value&3)<<10
	case OriginalRasterMask:
		r.mask = value
	case OriginalRasterMirroring:
		r.mirror = value
	case OriginalRasterCHR0:
		r.pages[0], r.pages[1] = value&254, value&254+1
	case OriginalRasterCHR1:
		r.pages[2], r.pages[3] = value&254, value&254+1
	case OriginalRasterCHR2:
		r.pages[4] = value
	case OriginalRasterCHR3:
		r.pages[5] = value
	case OriginalRasterCHR4:
		r.pages[6] = value
	case OriginalRasterCHR5:
		r.pages[7] = value
	}
}
func (r *originalRasterProjection) incrementY() {
	if r.v&0x7000 == 0x7000 {
		r.v ^= 0x7000
		if r.v&0x3e0 == 0x3a0 {
			r.v ^= 0xba0
		} else if r.v&0x3e0 == 0x3e0 {
			r.v ^= 0x3e0
		} else {
			r.v += 32
		}
	} else {
		r.v += 0x1000
	}
}
func (r *originalRasterProjection) advance(lastPixel int32) {
	end := int(lastPixel / 8)
	if end < 0 {
		end = 0
	}
	if end > 34 {
		end = 34
	}
	for r.fetchedCount < end {
		i := r.fetchedCount
		if i >= 2 {
			for x := 0; x < 8; x++ {
				offset := x + int(r.fine)
				prior := r.fetched[i-2+offset/8]
				seg := OriginalRasterSegment{X: uint16((i-2)*8 + x), Length: 1, Page: prior.page, Tile: prior.tile, Row: prior.row, Column: uint8(offset & 7), Palette: prior.palette, Mask: r.mask}
				count := len(r.segments)
				if count > 0 {
					old := &r.segments[count-1]
					if old.X+uint16(old.Length) == seg.X && old.Column+old.Length == seg.Column && old.Page == seg.Page && old.Tile == seg.Tile && old.Row == seg.Row && old.Palette == seg.Palette && old.Mask == seg.Mask {
						old.Length++
						continue
					}
				}
				r.segments = append(r.segments, seg)
			}
		}
		table := int(r.v>>10) & 3
		physical := table & 1
		if r.mirror == 1 {
			physical = table >> 1
		}
		nt := &r.surface.Nametables[physical]
		x := r.v & 31
		tile := nt[r.v&1023]
		attribute := nt[960+(x>>2)+((r.v&0x380)>>4)]
		slot := int(tile >> 6)
		if r.control&16 != 0 {
			slot += 4
		}
		r.fetched[i] = originalRasterRowSource{page: r.pages[slot], tile: tile & 63, row: uint8(r.v>>12) & 7, palette: (attribute >> ((x & 2) + ((r.v & 0x40) >> 4))) & 3}
		if r.v&31 == 31 {
			r.v ^= 0x41f
		} else {
			r.v++
		}
		r.fetchedCount++
	}
	if lastPixel >= 268 && !r.fixed {
		r.incrementY()
		r.fixed = true
	}
}
