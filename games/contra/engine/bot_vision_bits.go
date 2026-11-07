package contra

import (
	"math/bits"
	"sort"
)

// Compiled row masks and per-frame visible RGB planes. These helpers use only
// rendered colors and immutable template geometry, not rule or actor state.
type botRGBPlanes map[uint32]*[240 * 4]uint64
type botBitRow struct {
	mask     uint64
	offset   int
	y, chunk uint8
	count    int
}
type botBitColor struct {
	color  uint32
	detail bool
	count  int
	rows   []botBitRow
}
type botBitPattern struct{ colors []botBitColor }

func newBotRGBPlanes(pixels []uint32) botRGBPlanes {
	out := botRGBPlanes{}
	for p, color := range pixels {
		plane := out[color]
		if plane == nil {
			plane = new([240 * 4]uint64)
			out[color] = plane
		}
		plane[p>>6] |= uint64(1) << uint(p&63)
	}
	return out
}
func botRGBPlanesFromLocations(locations map[uint32][]int) botRGBPlanes {
	out := make(botRGBPlanes, len(locations))
	for color, positions := range locations {
		plane := new([240 * 4]uint64)
		for _, p := range positions {
			plane[p>>6] |= uint64(1) << uint(p&63)
		}
		out[color] = plane
	}
	return out
}
func compileBotBitPattern(pattern *botVisionPattern) *botBitPattern {
	out := &botBitPattern{}
	indexes := map[uint32]int{}
	masks := []map[int]uint64{}
	for _, p := range pattern.points {
		i, ok := indexes[p.color]
		if !ok {
			i = len(out.colors)
			indexes[p.color] = i
			out.colors = append(out.colors, botBitColor{color: p.color, detail: p.detail})
			masks = append(masks, map[int]uint64{})
		}
		masks[i][p.y*4+p.x/64] |= uint64(1) << uint(p.x&63)
	}
	for i := range out.colors {
		keys := make([]int, 0, len(masks[i]))
		for key := range masks[i] {
			keys = append(keys, key)
		}
		sort.Ints(keys)
		for _, key := range keys {
			mask := masks[i][key]
			count := bits.OnesCount64(mask)
			out.colors[i].rows = append(out.colors[i].rows, botBitRow{mask: mask, offset: key, y: uint8(key / 4), chunk: uint8(key % 4), count: count})
			out.colors[i].count += count
		}
	}
	sort.SliceStable(out.colors, func(i, j int) bool { return out.colors[i].detail && !out.colors[j].detail })
	return out
}
func botBitSpan(lo, hi uint64) int {
	first, last := 0, 0
	if lo != 0 {
		first = bits.TrailingZeros64(lo)
	} else {
		first = 64 + bits.TrailingZeros64(hi)
	}
	if hi != 0 {
		last = 127 - bits.LeadingZeros64(hi)
	} else {
		last = 63 - bits.LeadingZeros64(lo)
	}
	return last - first
}
func botMatchBitPattern(pixels []uint32, planes botRGBPlanes, compiled *botBitPattern, pattern *botVisionPattern, x, y int, partial bool) (int, bool) {
	// Exact-only negatives already have a cheap first-mismatch point path.
	if !partial || !pattern.partialPossible {
		return botMatchVisualPattern(pixels, pattern, x, y, partial)
	}
	base, shift := y*4+x/64, uint(x&63)
	matched, details, remaining, colors := 0, 0, pattern.detailCount, 0
	var xs, ys [2]uint64
	for _, group := range compiled.colors {
		plane := planes[group.color]
		if plane == nil {
			if group.detail {
				remaining -= group.count
				if details+remaining < pattern.minimumDetails {
					return 0, false
				}
			}
			continue
		}
		colorMatched := false
		for _, row := range group.rows {
			at := base + row.offset
			word := plane[at] >> shift
			if shift != 0 && at&3 != 3 {
				word |= plane[at+1] << (64 - shift)
			}
			hit := word & row.mask
			n := bits.OnesCount64(hit)
			matched += n
			if group.detail {
				details += n
				remaining -= row.count
				if details+remaining < pattern.minimumDetails {
					return 0, false
				}
				if hit != 0 {
					colorMatched = true
					xs[row.chunk] |= hit
					ys[row.y>>6] |= uint64(1) << uint(row.y&63)
				}
			}
		}
		if group.detail && colorMatched {
			colors++
		}
	}
	if matched == len(pattern.points) {
		return details*20 + matched, true
	}
	if details >= 12 && details*100 >= pattern.detailCount*40 && colors >= 2 && botBitSpan(xs[0], xs[1]) >= 7 && botBitSpan(ys[0], ys[1]) >= 11 {
		return details*20 + matched, true
	}
	return 0, false
}
