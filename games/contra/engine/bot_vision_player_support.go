package contra

import "math/bits"

// This path is intentionally restricted to the fixed moving-stone artwork.
// It is not generic enemy partial matching and has no current-stage selector.
func botPlayerCoveredSupport(pattern *botVisionPattern) bool {
	return pattern.label.Class == "enemy" && pattern.label.Kind == 16 && pattern.label.Sprite == 72 && pattern.playerMask == 0
}

// Match every unoccluded opaque point, including dark outline pixels. A known
// player's opaque pixels are the only permitted omissions. Most of both the
// full shape and its non-dark details must remain, with two actual colors and
// broad visible extent; a small convenient patch never proves a support.
func botMatchPlayerCoveredSupport(colors []uint32, mask *[256 * 240 / 64]uint64, p *botVisionPattern, left, top int) bool {
	if mask == nil || !botPlayerCoveredSupport(p) || len(colors) != 256*240 || left < 0 || top < 0 || left+p.width > 256 || top+p.height > 240 {
		return false
	}
	covered, visible, details := 0, 0, 0
	minX, minY, maxX, maxY := p.width, p.height, -1, -1
	var first uint32
	twoColors := false
	for _, point := range p.points {
		at := (top+point.y)*256 + left + point.x
		if mask[at/64]&(uint64(1)<<uint(at%64)) != 0 {
			covered++
			if covered*4 > len(p.points) {
				return false
			}
			continue
		}
		if colors[at] != point.color {
			return false
		}
		visible++
		if point.detail {
			if details == 0 {
				first = point.color
			} else if point.color != first {
				twoColors = true
			}
			details++
			minX = min(minX, point.x)
			maxX = max(maxX, point.x)
			minY = min(minY, point.y)
			maxY = max(maxY, point.y)
		}
	}
	return covered > 0 && visible*4 >= len(p.points)*3 && details >= 12 && details*4 >= p.detailCount*3 && twoColors && (maxX-minX+1)*4 >= p.width*3 && (maxY-minY+1)*2 >= p.height
}

type botCoveredSupport struct {
	pattern   *botVisionPattern
	left, top int
	enemy     BotEnemy
}

func sameBotCoveredSupport(a, b BotEnemy) bool {
	return a.X == b.X && a.Y == b.Y && a.Kind == b.Kind && a.Sprite == b.Sprite && a.Appearance == b.Appearance && a.Family == b.Family
}
func overlapBotCoveredSupport(a, b botCoveredSupport) bool {
	width := min(a.left+a.pattern.width, b.left+b.pattern.width) - max(a.left, b.left)
	height := min(a.top+a.pattern.height, b.top+b.pattern.height) - max(a.top, b.top)
	return width > 0 && height > 0 && width*height*2 >= min(a.pattern.width*a.pattern.height, b.pattern.width*b.pattern.height)
}

// Called after public menu/intro markers have been applied. The mask is made
// only by the existing exact, accepted player path; partial candidates never
// supply it. Search is limited to positions that can intersect that mask.
func (v *BotVision) recoverPlayerCoveredSupports(colors []uint32, mask *[256 * 240 / 64]uint64, out *BotObservation) {
	if mask == nil || out == nil || out.Phase != "battle" || (!out.Players[0].Visible && !out.Players[1].Visible) {
		return
	}
	minX, minY, maxX, maxY := 256, 240, -1, -1
	for word, remaining := range mask {
		for remaining != 0 {
			bit := bits.TrailingZeros64(remaining)
			at := word*64 + bit
			x, y := at%256, at/256
			minX = min(minX, x)
			minY = min(minY, y)
			maxX = max(maxX, x)
			maxY = max(maxY, y)
			remaining &= remaining - 1
		}
	}
	if maxX < 0 {
		return
	}
	var candidates []botCoveredSupport
	for i := range v.sprites {
		pattern := &v.sprites[i]
		if !botPlayerCoveredSupport(pattern) {
			continue
		}
		for top := max(0, minY-pattern.height+1); top <= min(240-pattern.height, maxY); top++ {
			for left := max(0, minX-pattern.width+1); left <= min(256-pattern.width, maxX); left++ {
				if !botMatchPlayerCoveredSupport(colors, mask, pattern, left, top) {
					continue
				}
				e := BotEnemy{X: left + pattern.anchorX, Y: top + pattern.anchorY, Kind: pattern.label.Kind, Sprite: pattern.label.Sprite, Appearance: pattern.label.Appearance, Family: pattern.label.Family}
				candidate := botCoveredSupport{pattern: pattern, left: left, top: top, enemy: e}
				existing := false
				for _, old := range out.Enemies {
					if old.Kind != e.Kind || old.Sprite != e.Sprite {
						continue
					}
					reference := botCoveredSupport{pattern: pattern, left: old.X - pattern.anchorX, top: old.Y - pattern.anchorY, enemy: old}
					if overlapBotCoveredSupport(candidate, reference) {
						existing = true
						break
					}
				}
				if !existing {
					candidates = append(candidates, candidate)
				}
			}
		}
	}
	for i, candidate := range candidates {
		ambiguous, duplicate := false, false
		for j, other := range candidates {
			if i == j || !overlapBotCoveredSupport(candidate, other) {
				continue
			}
			if !sameBotCoveredSupport(candidate.enemy, other.enemy) {
				ambiguous = true
				break
			}
			if j < i {
				duplicate = true
			}
		}
		if !ambiguous && !duplicate {
			out.Enemies = append(out.Enemies, candidate.enemy)
		}
	}
}
