package contra

// TerrainKind retains the original collision meanings. Platforms support feet,
// water changes the player action rules, and solids also block side/head probes.
type TerrainKind uint8

const (
	TerrainEmpty    TerrainKind = 0
	TerrainPlatform TerrainKind = 1
	TerrainWater    TerrainKind = 2
	TerrainSolid    TerrainKind = 0x80
)

// CollisionWindow contains two original streamed 16x16-cell screens, with four
// two-bit cells per byte. Streaming replaces portions of a screen as the camera
// moves; a partially loaded screen is meaningful simulation state.
type CollisionWindow [128]uint8

// CollisionScroll binds viewport coordinates to the currently loaded screens.
// Only NameTable bit 0 selects a collision screen; other video-control bits do
// not affect the map lookup.
type CollisionScroll struct {
	X, Y      uint8
	NameTable uint8
}

// TerrainSample includes the exact source for trace/debug verification. The
// gameplay-facing result is Kind; these source fields do not appear in the UI.
type TerrainSample struct {
	Kind        TerrainKind
	SourceIndex uint8
	SourceByte  uint8
	Column      uint8
}

// At reproduces original fixed-bank $DFEC..E060. Coordinates are viewport
// pixels, not a player's foot position; player probes supply their own offsets.
// The original skips collision for Y>=224 while retaining the computed index.
func (w *CollisionWindow) At(x, y uint8, scroll CollisionScroll) TerrainSample {
	sumY := uint16(y) + uint16(scroll.Y)
	rowY := uint8(sumY)
	if sumY >= 240 {
		// ADC #$0F with carry set is an eight-bit +16. This preserves the
		// original arithmetic even outside the normal scroll range.
		rowY += 16
	}
	sumX := uint16(x) + uint16(scroll.X)
	page := scroll.NameTable & 1
	if sumX >= 256 {
		page ^= 1
	}
	column := uint8(sumX) >> 4
	index := (rowY>>4)*4 | column>>2 | page<<6
	sample := TerrainSample{SourceIndex: index, SourceByte: w[index], Column: column & 3}
	if y < 224 {
		packed := (sample.SourceByte >> (6 - 2*sample.Column)) & 3
		sample.Kind = [...]TerrainKind{TerrainEmpty, TerrainPlatform, TerrainWater, TerrainSolid}[packed]
	}
	return sample
}
