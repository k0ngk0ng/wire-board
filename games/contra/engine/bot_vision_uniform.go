package contra

// Capped horizontal RGB runs let a whole uniform 16x16 cell be rejected using
// sixteen row checks. Uniform cells have no unique normalized terrain pattern.
func botSameColorRuns(pixels []uint8) []uint8 {
	runs := make([]uint8, 256*240)
	for y := 0; y < 240; y++ {
		var previous uint32
		var count uint8
		for x := 255; x >= 0; x-- {
			at := y*256 + x
			color := botPixelRGB(pixels, at*4)
			if count != 0 && color == previous {
				count = min(count+1, 16)
			} else {
				count = 1
			}
			runs[at] = count
			previous = color
		}
	}
	return runs
}
func botUniformCell(pixels, runs []uint8, x, y int) bool {
	if x < 0 || y < 0 || x+16 > 256 || y+16 > 240 {
		return false
	}
	color := botPixelRGB(pixels, (y*256+x)*4)
	for row := 0; row < 16; row++ {
		at := (y+row)*256 + x
		if runs[at] < 16 || botPixelRGB(pixels, at*4) != color {
			return false
		}
	}
	return true
}
