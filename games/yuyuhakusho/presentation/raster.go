package presentation

import (
	"errors"
	"image"
	"image/color"
)

type Raster struct {
	Width, Height int
	Colors        []uint16 // Normal-mode RGB565, separate from opacity.
	Opaque        []bool
	HighPriority  []bool
	Owners        []uint16
}

func color565(compact uint16, limited bool) uint16 {
	if limited {
		compact &= 0x49
	}
	r, g, b := (compact&7)*2, ((compact>>3)&7)*2, ((compact>>6)&7)*2
	return r<<12 | (r>>3)<<11 | g<<7 | (g>>2)<<5 | b<<1 | b>>3
}

// RasterizeSprites returns the ordered sprite plane. Its high/low priority
// mask is available for independent background composition. It does not model
// per-scanline sprite overflow or shadow/highlight modes.
func RasterizeSprites(state FrameState, catalog Catalog, palette [64]uint16, width, height int, limited bool) (Raster, error) {
	if width <= 0 || height <= 0 || width > 8192 || height > 8192 {
		return Raster{}, errors.New("unsupported sprite raster dimensions")
	}
	for _, c := range palette {
		if c > 511 {
			return Raster{}, errors.New("invalid compact world palette")
		}
	}
	n := width * height
	raster := Raster{Width: width, Height: height, Colors: make([]uint16, n), Opaque: make([]bool, n), HighPriority: make([]bool, n), Owners: make([]uint16, n)}
	for _, part := range OrderedParts(state) {
		// The completed layer list ends at a zero link or an index outside
		// the80 normal-mode entries, including an item written after count80.
		if part.Index == 0 || part.Index >= 80 {
			break
		}
		if part.WidthTiles < 1 || part.WidthTiles > 4 || part.HeightTiles < 1 || part.HeightTiles > 4 || part.Palette >= 4 || part.TileSlot >= 2048 {
			return Raster{}, errors.New("invalid independent sprite part")
		}
		sx, sy := int(part.X&511)-128, int(part.Y&511)-128
		w, h := int(part.WidthTiles)*8, int(part.HeightTiles)*8
		for py := 0; py < h; py++ {
			y := sy + py
			if y < 0 || y >= height {
				continue
			}
			for px := 0; px < w; px++ {
				x := sx + px
				if x < 0 || x >= width || raster.Opaque[y*width+x] {
					continue
				}
				u, v := px, py
				if part.FlipX {
					u = w - 1 - u
				}
				if part.FlipY {
					v = h - 1 - v
				}
				slot := (part.TileSlot + uint16(u/8)*uint16(part.HeightTiles) + uint16(v/8)) & 2047
				tile, exists := state.Slots.Lookup(slot)
				if !exists || int(tile) >= len(catalog.Tiles) {
					return Raster{}, errors.New("sprite references unavailable prior tile context")
				}
				value := catalog.Tiles[tile][(v%8)*4+(u%8)/2]
				value >>= 4 * uint(1-u&1)
				value &= 15
				if value == 0 {
					continue
				}
				position := y*width + x
				raster.Opaque[position], raster.HighPriority[position], raster.Owners[position] = true, part.HighPriority, part.Owner
				raster.Colors[position] = color565(palette[int(part.Palette)*16+int(value)], limited)
			}
		}
	}
	return raster, nil
}

func (r Raster) RGBA() *image.RGBA {
	image := image.NewRGBA(image.Rect(0, 0, r.Width, r.Height))
	for i, opaque := range r.Opaque {
		if !opaque {
			continue
		}
		c := r.Colors[i]
		red, green, blue := uint8(c>>11)&31, uint8(c>>5)&63, uint8(c)&31
		image.SetRGBA(i%r.Width, i/r.Width, color.RGBA{red<<3 | red>>2, green<<2 | green>>4, blue<<3 | blue>>2, 255})
	}
	return image
}
