package contra

// Counts cover whole 8x8 tiles, so every rectangle query is a conservative
// upper bound on the actual visible pixels in a candidate's bounding box.
// Prefix values cannot exceed the 61,440 pixels in the public viewport.
type botPixelCounts map[uint32]*[33 * 31]uint16
type botPatternColor struct {
	color  uint32
	count  uint16
	detail bool
}

func newBotPixelCounts(locations map[uint32][]int) botPixelCounts {
	counts := make(botPixelCounts, len(locations))
	for color, positions := range locations {
		grid := new([33 * 31]uint16)
		for _, p := range positions {
			grid[(p/2048+1)*33+(p%256/8+1)]++
		}
		for y := 1; y <= 30; y++ {
			for x := 1; x <= 32; x++ {
				at := y*33 + x
				grid[at] = uint16(int(grid[at]) + int(grid[at-33]) + int(grid[at-1]) - int(grid[at-34]))
			}
		}
		counts[color] = grid
	}
	return counts
}
func (c botPixelCounts) upper(color uint32, x, y, w, h int) int {
	grid := c[color]
	if grid == nil {
		return 0
	}
	left, top, right, bottom := x/8, y/8, (x+w+7)/8, (y+h+7)/8
	return int(grid[bottom*33+right]) - int(grid[top*33+right]) - int(grid[bottom*33+left]) + int(grid[top*33+left])
}
func (c botPixelCounts) possible(pattern *botVisionPattern, x, y int, partial bool) bool {
	if !partial || !pattern.partialPossible {
		for _, color := range pattern.colorCounts {
			if c.upper(color.color, x, y, pattern.width, pattern.height) < int(color.count) {
				return false
			}
		}
		return true
	}
	possible, colors := 0, 0
	for _, color := range pattern.colorCounts {
		if !color.detail {
			continue
		}
		count := c.upper(color.color, x, y, pattern.width, pattern.height)
		if count > 0 {
			colors++
			possible += min(count, int(color.count))
		}
	}
	return colors >= 2 && possible >= pattern.minimumDetails
}
