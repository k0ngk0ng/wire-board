package contra

import "sort"

type botVisionAnchorChoice struct {
	point           botVisionPoint
	distance, index int
}

// Frequency is shared by all pixels of one color. Corner candidates depend on
// immutable geometry; only whether each color is present changes per picture.
func prepareBotPatternAnchors(pattern *botVisionPattern) {
	seen := map[uint32]bool{}
	counts := map[uint32]uint16{}
	minX, minY, maxX, maxY := 256, 240, -1, -1
	for _, point := range pattern.points {
		counts[point.color]++
		if point.detail {
			minX, maxX = min(minX, point.x), max(maxX, point.x)
			minY, maxY = min(minY, point.y), max(maxY, point.y)
		}
		if !seen[point.color] {
			seen[point.color] = true
			pattern.colorAnchors = append(pattern.colorAnchors, point)
		}
	}
	for _, point := range pattern.colorAnchors {
		pattern.colorCounts = append(pattern.colorCounts, botPatternColor{point.color, counts[point.color], point.detail})
	}
	corners := [4][2]int{{0, 0}, {pattern.width - 1, 0}, {0, pattern.height - 1}, {pattern.width - 1, pattern.height - 1}}
	for cornerIndex, corner := range corners {
		nearest := map[uint32]botVisionAnchorChoice{}
		for index, point := range pattern.points {
			if !point.detail {
				continue
			}
			distance := absBot(point.x-corner[0]) + absBot(point.y-corner[1])
			prior, found := nearest[point.color]
			if !found || distance < prior.distance {
				nearest[point.color] = botVisionAnchorChoice{point, distance, index}
			}
		}
		for _, choice := range nearest {
			pattern.cornerAnchors[cornerIndex] = append(pattern.cornerAnchors[cornerIndex], choice)
		}
		choices := pattern.cornerAnchors[cornerIndex]
		sort.Slice(choices, func(i, j int) bool {
			if choices[i].distance != choices[j].distance {
				return choices[i].distance < choices[j].distance
			}
			return choices[i].index < choices[j].index
		})
	}
	pattern.minimumDetails = max(12, (pattern.detailCount*40+99)/100)
	pattern.partialPossible = pattern.detailCount >= 12 && len(pattern.cornerAnchors[0]) >= 2 && maxX-minX >= 7 && maxY-minY >= 11
}

func botVisualAnchors(pattern *botVisionPattern, locations map[uint32][]int, partial bool) ([5]botVisionPoint, int) {
	var anchors [5]botVisionPoint
	anchor := pattern.colorAnchors[0]
	for _, point := range pattern.colorAnchors[1:] {
		if len(locations[point.color]) < len(locations[anchor.color]) {
			anchor = point
		}
	}
	anchors[0] = anchor
	n := 1
	if !partial {
		return anchors, n
	}
	for _, choices := range pattern.cornerAnchors {
		for _, choice := range choices {
			if len(locations[choice.point.color]) == 0 {
				continue
			}
			duplicate := false
			for _, existing := range anchors[:n] {
				if existing == choice.point {
					duplicate = true
					break
				}
			}
			if !duplicate {
				anchors[n] = choice.point
				n++
			}
			break
		}
	}
	return anchors, n
}
