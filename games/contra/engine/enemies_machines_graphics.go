package contra

import "errors"

// GraphicsTileAnimation is the normalized C593 definition. Some original claw
// frames have three rows and consume bytes belonging to the next table entry.
// Rows preserve that source order without retaining a CPU memory image.
type GraphicsTileAnimation struct {
	Attribute uint8
	Rows      [][2]uint8
}

type GraphicsTileAnimations []GraphicsTileAnimation

// OriginalTileAnimations exposes only the levels with recovered animation
// definitions. Every caller receives owned slices for deterministic snapshots.
func OriginalTileAnimations(level uint8) (GraphicsTileAnimations, error) {
	source, ok := originalTileAnimationDefinitions[level]
	if !ok {
		return nil, errors.New("tile animations are not recovered for this level")
	}
	result := make(GraphicsTileAnimations, len(source))
	for i, v := range source {
		result[i] = GraphicsTileAnimation{Attribute: v.Attribute, Rows: append([][2]uint8(nil), v.Rows...)}
	}
	return result, nil
}

// RequestTileAnimation follows Japanese C593, reached through F9F7. It is a
// two-column tile update, with a distinct admission threshold and palette
// operation from the four-column RequestSupertile path. It never changes the
// source map, collision cells, frame mode or collision-query source offset.
func (q *GraphicsQueue) RequestTileAnimation(x, y, code uint8, c GraphicsContext, animations GraphicsTileAnimations) (GraphicsRequestResult, error) {
	if q.Used >= 0x50 {
		return GraphicsRequestResult{}, nil
	}
	index := int(code & 0x7f)
	if index >= len(animations) {
		return GraphicsRequestResult{}, errors.New("unknown tile animation")
	}
	animation := animations[index]
	if len(animation.Rows) == 0 || len(animation.Rows) > 7 {
		return GraphicsRequestResult{}, errors.New("invalid tile animation rows")
	}
	p := locateGraphics(x, y, c.Scroll)
	var packets []GraphicsPacket
	if code&0x80 == 0 {
		if c.Supertiles == nil || p.mapOffset < 0 || p.mapOffset >= len(c.Supertiles) {
			return GraphicsRequestResult{}, errors.New("tile animation palette requires mapped supertile")
		}
		id := int(c.Supertiles[p.mapOffset])
		if id >= len(c.Tileset.BaseAttributes) {
			return GraphicsRequestResult{}, errors.New("unknown tile animation base attribute")
		}
		// C5D8 shifts the entire header, not only its two low palette bits.
		value := c.Tileset.BaseAttributes[id]&[...]uint8{0xfc, 0xf3, 0xcf, 0x3f}[p.mode] | animation.Attribute<<(p.mode*2)
		// C5BC replaces C656's attribute-address low byte with the row
		// count before C6D7 emits it. Preserve that original side effect.
		paletteAddress := p.attribute&0xff00 | uint16(len(animation.Rows))
		packets = append(packets, GraphicsPacket{Kind: 1, Length: 1, Segments: []GraphicsSegment{{Address: paletteAddress, Values: []uint8{value}}}})
	}
	pattern := GraphicsPacket{Kind: 1, Length: 2}
	for row, values := range animation.Rows {
		pattern.Segments = append(pattern.Segments, GraphicsSegment{Address: p.address + uint16(row*32), Values: []uint8{values[0], values[1]}})
	}
	packets = append(packets, pattern)
	next := q.Clone()
	for _, packet := range packets {
		if err := next.Append(packet); err != nil {
			return GraphicsRequestResult{}, err
		}
	}
	*q = next
	return GraphicsRequestResult{Accepted: true, Address: p.address, AttributeLookupWritten: true, AttributeLookup: code & 0x80}, nil
}

// Source: Japanese original C63A, bank6/7. See the source-bound oracle.
var originalTileAnimationDefinitions = map[uint8]GraphicsTileAnimations{
	1: { // CPU $8731, SHA256 2404abb45252b5f67c82a5ba7dfd3c590191738b17352f4006e99e1285c69822
		{Attribute: 0x00, Rows: [][2]uint8{{0xe2, 0xe3}, {0xe4, 0xe5}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xe6, 0xe7}, {0xe8, 0xe9}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xea, 0xeb}, {0xec, 0xed}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xde, 0xdf}, {0xe0, 0xe1}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xee, 0xef}, {0xf0, 0xf1}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xf2, 0xf3}, {0xf4, 0xf5}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xf6, 0xf7}, {0xf8, 0xf9}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xca, 0xcb}, {0xcc, 0xcd}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xfa, 0xfb}, {0x53, 0x54}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xce, 0xcf}, {0xd0, 0xd1}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xd2, 0xd3}, {0xd4, 0xd5}}},
	},
	3: { // CPU $8731, SHA256 2404abb45252b5f67c82a5ba7dfd3c590191738b17352f4006e99e1285c69822
		{Attribute: 0x00, Rows: [][2]uint8{{0xe2, 0xe3}, {0xe4, 0xe5}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xe6, 0xe7}, {0xe8, 0xe9}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xea, 0xeb}, {0xec, 0xed}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xde, 0xdf}, {0xe0, 0xe1}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xee, 0xef}, {0xf0, 0xf1}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xf2, 0xf3}, {0xf4, 0xf5}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xf6, 0xf7}, {0xf8, 0xf9}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xca, 0xcb}, {0xcc, 0xcd}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xfa, 0xfb}, {0x53, 0x54}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xce, 0xcf}, {0xd0, 0xd1}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xd2, 0xd3}, {0xd4, 0xd5}}},
	},
	5: { // CPU $9E58, SHA256 ee84b5dedd34cbf690c2221be3404779eb2b377a87b9b6cae025910c305793ce
		{Attribute: 0x01, Rows: [][2]uint8{{0x00, 0x00}, {0x00, 0x00}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0x1a, 0x0c}, {0x1b, 0x1c}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0x0e, 0x0c}, {0x1e, 0x1c}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0x0c, 0x00}, {0x1c, 0x00}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0x0d, 0x0f}, {0x1d, 0x1f}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0x0d, 0x0e}, {0x1d, 0x1e}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0x00, 0x0d}, {0x00, 0x1d}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0x18, 0x19}, {0x14, 0x15}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0x16, 0x17}, {0x14, 0x15}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0x14, 0x15}, {0x00, 0x00}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0x01, 0xa8}, {0x76, 0x77}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0xc9, 0xf8}, {0xc9, 0xa9}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0xc9, 0xa9}, {0xc9, 0x00}}},
		{Attribute: 0x01, Rows: [][2]uint8{{0xc9, 0x00}, {0xc9, 0x00}}},
	},
	6: { // CPU $A5EE, SHA256 daf757260bea24f07532222b4692776896db76020f51a099a47e7ce9ecfe34b0
		{Attribute: 0x83, Rows: [][2]uint8{{0xde, 0xdf}, {0xee, 0xef}, {0x00, 0x00}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0x00, 0x00}, {0x00, 0x00}}},
		{Attribute: 0x83, Rows: [][2]uint8{{0xde, 0xdf}, {0xee, 0xef}, {0x47, 0x48}}},
		{Attribute: 0x47, Rows: [][2]uint8{{0x48, 0x00}, {0x00, 0x00}}},
		{Attribute: 0x83, Rows: [][2]uint8{{0xde, 0xdf}, {0xee, 0xef}, {0x57, 0x58}}},
		{Attribute: 0x57, Rows: [][2]uint8{{0x58, 0x00}, {0x00, 0x00}}},
		{Attribute: 0x83, Rows: [][2]uint8{{0xce, 0xcf}, {0xde, 0xdf}, {0xee, 0xef}}},
		{Attribute: 0xee, Rows: [][2]uint8{{0xef, 0x00}, {0x00, 0x00}, {0x00, 0x30}, {0x31, 0x30}, {0x31, 0x00}, {0xc5, 0xc6}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0x30, 0x31}, {0x30, 0x31}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xc5, 0xc6}, {0xc7, 0xc8}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0xea, 0xeb}, {0xec, 0xed}}},
		{Attribute: 0x00, Rows: [][2]uint8{{0x4a, 0x4f}, {0x4a, 0x52}}},
	},
}
