package contra

import "errors"

// GraphicsSegment describes one address plus a fixed-length group of values.
type GraphicsSegment struct {
	Address uint16
	Values  []uint8
}

// GraphicsPacket retains the original row/column grouping because its encoded
// byte cost, rather than the number of writes, controls later request admission.
// Kind 1 advances one PPU address per value; Kind 2 advances 32 addresses.
type GraphicsPacket struct {
	Kind, Length uint8
	Segments     []GraphicsSegment
}

func (p GraphicsPacket) EncodedSize() int { return 3 + len(p.Segments)*(2+int(p.Length)) }

// Encode is for deterministic verification and replay inspection. Runtime state
// consists of semantic packets, not an emulated CPU graphics-buffer address space.
func (p GraphicsPacket) Encode() ([]uint8, error) {
	if p.Kind != 1 && p.Kind != 2 || p.Length == 0 || len(p.Segments) == 0 || len(p.Segments) > 255 || p.EncodedSize() > 255 {
		return nil, errors.New("invalid graphics packet shape")
	}
	raw := make([]uint8, 0, p.EncodedSize())
	raw = append(raw, p.Kind, p.Length, uint8(len(p.Segments)))
	for _, segment := range p.Segments {
		if len(segment.Values) != int(p.Length) || segment.Address > 0x3fff {
			return nil, errors.New("invalid graphics packet segment")
		}
		raw = append(raw, uint8(segment.Address>>8), uint8(segment.Address))
		raw = append(raw, segment.Values...)
	}
	return raw, nil
}

// GraphicsQueue is the current frame's deferred presentation work. Used is the
// original byte cursor; Append updates it, while a request checks it before
// adding its own packets. The display boundary owns Reset.
type GraphicsQueue struct {
	Used        uint8
	Packets     []GraphicsPacket
	Mode        GraphicsQueueMode
	TextPackets []GraphicsTextPacket
	HeadClear   [5]uint8
}

// The original graphics region ends at $07BF; palette colors start at $07C0.
// A typed independent scene never represents requests which overwrite colors
// or CHR state through an out-of-domain CPU byte cursor.
const GraphicsBufferCapacity = 192

func (q *GraphicsQueue) Append(packet GraphicsPacket) error {
	if err := q.validateMode(); err != nil {
		return err
	}
	if q.Mode != GraphicsRectangular {
		return errors.New("rectangle packet requires rectangle graphics mode")
	}
	raw, err := packet.Encode()
	if err != nil {
		return err
	}
	if int(q.Used)+len(raw) > GraphicsBufferCapacity {
		return errors.New("graphics packet exceeds the frame buffer")
	}
	copyPacket := GraphicsPacket{Kind: packet.Kind, Length: packet.Length, Segments: make([]GraphicsSegment, len(packet.Segments))}
	for i, segment := range packet.Segments {
		copyPacket.Segments[i] = GraphicsSegment{Address: segment.Address, Values: append([]uint8(nil), segment.Values...)}
	}
	q.Packets = append(q.Packets, copyPacket)
	q.Used += uint8(len(raw))
	return nil
}

func (q *GraphicsQueue) Writes() []BackgroundWrite {
	writes, err := q.DecodedWrites()
	if err != nil {
		panic(err)
	} // This convenience projection requires a validated queue.
	return writes
}

func (q *GraphicsQueue) plainWrites() []BackgroundWrite {
	var writes []BackgroundWrite
	if q.Mode == GraphicsText {
		for _, packet := range q.TextPackets {
			step := uint16(1)
			if packet.Kind == 2 {
				step = 32
			}
			writes = append(writes, BackgroundWrite{Address: packet.Address, Step: step, Values: append([]uint8(nil), packet.Values...)})
		}
		return writes
	}
	for _, packet := range q.Packets {
		step := uint16(1)
		if packet.Kind == 2 {
			step = 32
		}
		for _, segment := range packet.Segments {
			writes = append(writes, BackgroundWrite{Address: segment.Address, Step: step, Values: append([]uint8(nil), segment.Values...)})
		}
	}
	return writes
}

func (q *GraphicsQueue) Reset() {
	q.Used, q.Packets, q.TextPackets = 0, nil, nil
	q.HeadClear = [5]uint8{}
}

type GraphicsSupertile struct {
	Tiles     [16]uint8
	Attribute uint8
}

// GraphicsTileset is normalized level data. BaseAttributes correspond to the
// level's supertile IDs; Updates are the separate replacement-code table.
type GraphicsTileset struct {
	BaseAttributes []uint8
	Updates        []GraphicsSupertile
	// The original replacement attribute table's address high byte. A shared
	// caller-visible lookup result depends on this source datum, not on any
	// host object address or an emulated general-purpose scratch space.
	SourceAttributePage uint8
}

func JungleGraphicsTileset() (GraphicsTileset, error) {
	layout, err := jungle()
	if err != nil {
		return GraphicsTileset{}, err
	}
	// Japanese C56E points to $83B1/$86FC in bank6/7, exactly 59
	// entries after the base $8001/$86C1 first-stage definitions.
	const replacementBase = 59
	if len(layout.Supertiles) != 108 {
		return GraphicsTileset{}, errors.New("invalid first-stage graphics definitions")
	}
	set := GraphicsTileset{BaseAttributes: make([]uint8, len(layout.Supertiles)), Updates: make([]GraphicsSupertile, len(layout.Supertiles)-replacementBase), SourceAttributePage: 0x86}
	for i, tile := range layout.Supertiles {
		set.BaseAttributes[i] = tile.Attribute
		if i >= replacementBase {
			set.Updates[i-replacementBase] = GraphicsSupertile{Tiles: tile.Tiles, Attribute: tile.Attribute}
		}
	}
	return set, nil
}

type GraphicsContext struct {
	Scroll     CollisionScroll
	Supertiles *[128]uint8
	Tileset    GraphicsTileset
}

type GraphicsRequestResult struct {
	Accepted bool
	// Address is valid only when Accepted; it is the original C656 locator's
	// nametable address, also consumed by later enemy collision helpers.
	Address uint16
	// Some original enemy routines reuse this named attribute-selection result
	// after drawing. Rejection leaves their previous value unchanged. Accepted
	// no-palette requests produce $80; palette requests produce the source page.
	AttributeLookupWritten bool
	AttributeLookup        uint8
}

// RequestSupertile reproduces original fixed-bank C3D1. It neither changes the
// base supertile map nor collision cells. Those are separate rule operations;
// only the presentation packets and admission cursor are committed here.
func (q *GraphicsQueue) RequestSupertile(x, y, code uint8, c GraphicsContext) (GraphicsRequestResult, error) {
	if err := q.validateMode(); err != nil {
		return GraphicsRequestResult{}, err
	}
	if q.Mode != GraphicsRectangular {
		return GraphicsRequestResult{}, errors.New("supertile requires rectangle graphics mode")
	}
	if q.Used >= 0x40 {
		return GraphicsRequestResult{}, nil
	}
	index := int(code & 0x7f)
	if index >= len(c.Tileset.Updates) {
		return GraphicsRequestResult{}, errors.New("unknown replacement supertile")
	}
	tile := c.Tileset.Updates[index]
	position := locateGraphics(x, y, c.Scroll)
	var packets []GraphicsPacket
	if code&0x80 == 0 {
		palette, err := graphicsPalettePacket(tile.Attribute, position, c)
		if err != nil {
			return GraphicsRequestResult{}, err
		}
		packets = append(packets, palette)
	}
	patterns := GraphicsPacket{Kind: 1, Length: 4}
	for row := 0; row < 4; row++ {
		patterns.Segments = append(patterns.Segments, GraphicsSegment{Address: position.address + uint16(row*32), Values: append([]uint8(nil), tile.Tiles[row*4:row*4+4]...)})
	}
	packets = append(packets, patterns)
	next := q.Clone()
	for _, packet := range packets {
		if err := next.Append(packet); err != nil {
			return GraphicsRequestResult{}, err
		}
	}
	*q = next
	lookup := uint8(0x80)
	if code&0x80 == 0 {
		lookup = c.Tileset.SourceAttributePage
	}
	return GraphicsRequestResult{Accepted: true, Address: position.address, AttributeLookupWritten: true, AttributeLookup: lookup}, nil
}

type graphicsPosition struct {
	address, attribute uint16
	mapOffset          int
	mode               uint8
}

func locateGraphics(x, y uint8, scroll CollisionScroll) graphicsPosition {
	sumY := uint16(y) + uint16(scroll.Y)
	py := uint8(sumY)
	if sumY >= 240 {
		py += 16
	}
	py &= 0xf8
	sumX := uint16(x) + uint16(scroll.X)
	px := uint8(sumX) & 0xf8
	page := scroll.NameTable & 1
	if sumX >= 256 {
		page ^= 1
	}
	tileX, tileY := px>>3, py>>3
	attributeOffset := (tileY>>2)*8 | tileX>>2
	return graphicsPosition{
		address:   0x2000 | uint16(page)<<10 | uint16(tileY)<<5 | uint16(tileX),
		attribute: 0x23c0 | uint16(page)<<10 | uint16(attributeOffset),
		mapOffset: int(page)*64 | int(attributeOffset),
		mode:      tileY&2 | (tileX>>1)&1,
	}
}

func graphicsPalettePacket(attribute uint8, p graphicsPosition, c GraphicsContext) (GraphicsPacket, error) {
	old := func(offset int) (uint8, error) {
		if c.Supertiles == nil || offset < 0 || offset >= len(c.Supertiles) {
			return 0, errors.New("graphics palette requires a mapped supertile")
		}
		id := int(c.Supertiles[offset])
		if id >= len(c.Tileset.BaseAttributes) {
			return 0, errors.New("unknown base supertile attribute")
		}
		return c.Tileset.BaseAttributes[id], nil
	}
	if p.mode == 0 {
		return GraphicsPacket{Kind: 1, Length: 1, Segments: []GraphicsSegment{{Address: p.attribute, Values: []uint8{attribute}}}}, nil
	}
	positions := []int{p.mapOffset}
	if p.mode&1 != 0 {
		positions = append(positions, p.mapOffset+1)
	}
	if p.mode&2 != 0 {
		positions = append(positions, p.mapOffset+8)
	}
	if p.mode == 3 {
		positions = append(positions, p.mapOffset+9)
	}
	values := make([]uint8, len(positions))
	for i, position := range positions {
		v, err := old(position)
		if err != nil {
			return GraphicsPacket{}, err
		}
		values[i] = v
	}
	// C553 increments only the attribute address low byte, retaining its page.
	below := p.attribute&0xff00 | uint16(uint8(p.attribute+8))
	switch p.mode {
	case 1:
		values[0] = values[0]&0x33 | (attribute&0x33)<<2
		values[1] = values[1]&0xcc | (attribute&0xcc)>>2
		return GraphicsPacket{Kind: 1, Length: 2, Segments: []GraphicsSegment{{Address: p.attribute, Values: values}}}, nil
	case 2:
		values[0] = values[0]&0x0f | attribute<<4
		values[1] = values[1]&0xf0 | attribute>>4
		return GraphicsPacket{Kind: 1, Length: 1, Segments: []GraphicsSegment{{Address: p.attribute, Values: values[:1]}, {Address: below, Values: values[1:]}}}, nil
	case 3:
		values[0] = values[0]&0x3f | (attribute&3)<<6
		values[1] = values[1]&0xcf | (attribute&0x0c)<<2
		values[2] = values[2]&0xf3 | (attribute&0x30)>>2
		values[3] = values[3]&0xfc | attribute>>6
		return GraphicsPacket{Kind: 1, Length: 2, Segments: []GraphicsSegment{{Address: p.attribute, Values: values[:2]}, {Address: below, Values: values[2:]}}}, nil
	}
	return GraphicsPacket{}, errors.New("invalid graphics palette mode")
}
