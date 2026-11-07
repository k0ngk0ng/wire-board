package contra

// Detail membership is immutable template metadata. Matching still evaluates
// every required visible pixel; only repeated color classification and the
// two-distinct-colors set bookkeeping are removed from the candidate hot path.
func botMatchVisualPattern(pixels []uint32, pattern *botVisionPattern, x, y int, partial bool) (int, bool) {
	matched, details := 0, 0
	minX, minY, maxX, maxY := 256, 240, -1, -1
	var firstColor uint32
	hasColor, twoColors := false, false
	partial = partial && pattern.partialPossible
	remainingDetails := pattern.detailCount
	base := y*256 + x
	for _, point := range pattern.points {
		if point.detail {
			remainingDetails--
		}
		if pixels[base+int(point.offset)] != point.color {
			if !partial || details+remainingDetails < pattern.minimumDetails {
				return 0, false
			}
			continue
		}
		matched++
		if point.detail {
			details++
			if !twoColors {
				if !hasColor {
					firstColor = point.color
					hasColor = true
				} else if firstColor != point.color {
					twoColors = true
				}
			}
			minX = min(minX, point.x)
			maxX = max(maxX, point.x)
			minY = min(minY, point.y)
			maxY = max(maxY, point.y)
		}
	}
	if matched == len(pattern.points) {
		return details*20 + matched, true
	}
	if partial && details >= 12 && details*100 >= pattern.detailCount*40 && twoColors && maxX-minX >= 7 && maxY-minY >= 11 {
		return details*20 + matched, true
	}
	return 0, false
}
