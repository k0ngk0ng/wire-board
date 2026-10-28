package contra

import "errors"

// This is the RuntimeRenderer presentation contract, including its pinned
// old-PPU raster normalization. It is not a CPU/PPU or gameplay interpreter.
type botVisualRaster struct {
	frame PresentedFrame
	pages [8]*[4096]uint8
	rgb   [192]uint8
}

func (r *botVisualRaster) pattern(slot, tile, row, col int) uint8 {
	return r.pages[slot][((tile>>3)*8+row)*64+(tile&7)*8+col]
}
func (r *botVisualRaster) background(x, y int, control, mask, scrollX uint8) (uint8, bool) {
	colorMask := uint8(63)
	if mask&1 != 0 {
		colorMask = 48
	}
	color := r.frame.Palette[0] & colorMask
	if mask&8 == 0 || x < 8 && mask&2 == 0 {
		return color, false
	}
	sx, sy := x+int(scrollX), y+int(r.frame.Scroll.Y)
	px, py := sx&255, sy%240
	tx, ty := px>>3, py>>3
	page := int(control & 1)
	if sx >= 256 {
		page ^= 1
	}
	// Vertical mirroring is [first,second,first,second]; the vertical nametable
	// bit never selects extra hidden storage in this cartridge's renderer.
	base := page * 1024
	tile := int(r.frame.Background[base+ty*32+tx])
	patternBase := 0
	if control&16 != 0 {
		patternBase = 4
	}
	v := r.pattern(patternBase+(tile>>6), tile&63, py&7, px&7)
	if v == 0 {
		return color, false
	}
	attribute := r.frame.Background[base+960+(ty>>2)*8+(tx>>2)]
	palette := (attribute >> uint(((ty&2)<<1)|(tx&2))) & 3
	return r.frame.Palette[int(palette)*4+int(v)] & colorMask, true
}
func (r *botVisualRaster) spritePixel(slot, y, col int, control uint8) uint8 {
	at := slot * 4
	tile, attr := r.frame.OAM[at+1], r.frame.OAM[at+2]
	height := 8
	if control&32 != 0 {
		height = 16
	}
	row := y - int(r.frame.OAM[at]) - 1
	if attr&128 != 0 {
		row = height - 1 - row
	}
	global := int(tile)
	if height == 16 {
		global = int(tile&1)*256 + int(tile&254) + (row >> 3)
	} else if control&8 != 0 {
		global += 256
	}
	if attr&64 != 0 {
		col = 7 - col
	}
	return r.pattern(global>>6, global&63, row&7, col)
}
func (r *botVisualRaster) baseScroll(y int) uint8 {
	if r.frame.Display.SplitActive && y >= 64 {
		return r.frame.Display.SplitScroll
	}
	return r.frame.Scroll.X
}
func (r *botVisualRaster) spriteZeroHit() (int, error) {
	d := r.frame.Display
	control := r.frame.Scroll.NameTable
	if d.AppliedMask&24 != 24 {
		return 0, errors.New("active raster lacks visible sprite-zero hit")
	}
	top := int(r.frame.OAM[0]) + 1
	height := 8
	if control&32 != 0 {
		height = 16
	}
	for y := top; y < 240 && y < top+height; y++ {
		for col := 0; col < 8; col++ {
			x := int(r.frame.OAM[3]) + col
			if x >= 255 || x < 8 && d.AppliedMask&6 != 6 {
				continue
			}
			_, opaque := r.background(x, y, control, d.AppliedMask, r.baseScroll(y))
			if opaque && r.spritePixel(0, y, col, control) != 0 {
				return y, nil
			}
		}
	}
	return 0, errors.New("active raster lacks visible sprite-zero hit")
}
func (r *botVisualRaster) render() ([]uint8, error) {
	d := r.frame.Display
	control := r.frame.Scroll.NameTable
	hit := -1
	if d.RasterActive {
		var e error
		hit, e = r.spriteZeroHit()
		if e != nil {
			return nil, e
		}
	}
	image := make([]uint8, 256*240*4)
	height := 8
	if control&32 != 0 {
		height = 16
	}
	for y := 0; y < 240; y++ {
		mask, bgControl, sx := d.AppliedMask, control, r.baseScroll(y)
		if hit >= 0 && y >= hit {
			mask = d.RasterMask
			sx = r.frame.Scroll.X
			if y > hit {
				bgControl = d.RasterControl
			}
		}
		var candidates [8]int
		count := 0
		if mask&16 != 0 {
			for slot := 0; slot < 64 && count < 8; slot++ {
				top := int(r.frame.OAM[slot*4]) + 1
				if top <= y && y < top+height {
					candidates[count] = slot
					count++
				}
			}
		}
		for x := 0; x < 256; x++ {
			color, opaque := r.background(x, y, bgControl, mask, sx)
			if x >= 8 || mask&4 != 0 {
				for j := 0; j < count; j++ {
					slot := candidates[j]
					at := slot * 4
					col := x - int(r.frame.OAM[at+3])
					if col < 0 || col >= 8 {
						continue
					}
					v := r.spritePixel(slot, y, col, control)
					if v == 0 {
						continue
					}
					attr := r.frame.OAM[at+2]
					// First opaque OAM pixel wins before background priority is applied.
					// A hidden behind-background winner does not expose a later sprite.
					if attr&32 == 0 || !opaque {
						color = r.frame.Palette[16+int(attr&3)*4+int(v)] & 63
						if mask&1 != 0 {
							color &= 48
						}
					}
					break
				}
			}
			at := (y*256 + x) * 4
			image[at], image[at+1], image[at+2], image[at+3] = r.rgb[int(color)*3], r.rgb[int(color)*3+1], r.rgb[int(color)*3+2], 255
		}
	}
	return image, nil
}
