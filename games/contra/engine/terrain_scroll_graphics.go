package contra

import "errors"

func (s *TerrainScrollState) tileAt(c TerrainScrollContext, index int) (GraphicsSupertile, error) {
	if index < 0 || index >= len(s.Supertiles) {
		return GraphicsSupertile{}, errors.New("invalid streaming map index")
	}
	_, tiles, err := c.Resources.mapSet(int(c.World.Level), s.Variant)
	if err != nil {
		return GraphicsSupertile{}, err
	}
	id := int(s.Supertiles[index])
	if id >= len(tiles.Supertiles) {
		return GraphicsSupertile{}, errors.New("missing streaming supertile")
	}
	return tiles.Supertiles[id], nil
}

func (s *TerrainScrollState) tileColumn(c TerrainScrollContext) error {
	if s.Axis >= 32 || s.TileAddress < 0x2000 || s.TileAddress >= 0x3000 || s.TileAddress&0x3e0 != 0 {
		return errors.New("invalid tile-column cursor")
	}
	values := make([]uint8, 28)
	column := int(s.Axis)
	// Collision packing follows the write address, which is independent of
	// the source-map column. Keep both named cursors rather than assuming equal.
	base := int((s.TileAddress>>8)&4)<<4 | int(uint8(s.TileAddress)>>3)
	shift := uint(6 - 2*((uint8(s.TileAddress)>>1)&3))
	for row := 0; row < 28; row++ {
		tile, err := s.tileAt(c, int(s.MapOffset)+(row/4)*8+column/4)
		if err != nil {
			return err
		}
		value := tile.Tiles[(row&3)*4+column%4]
		values[row] = value
		if uint8(s.TileAddress)&1 == 0 && row&1 == 0 {
			index := base + (row/2)*4
			if index >= 128 {
				return errors.New("tile column exceeds collision window")
			}
			c.Terrain[index] = c.Terrain[index]&^(3<<shift) | collisionBits(value, c.Header.CollisionLimits)<<shift
		}
	}
	if err := c.Queue.Append(GraphicsPacket{Kind: 2, Length: 28, Segments: []GraphicsSegment{{Address: s.TileAddress, Values: values}}}); err != nil {
		return err
	}
	if c.Work != nil {
		c.Work.RecordStreamColumn(s.TileAddress)
	}
	return nil
}

func (s *TerrainScrollState) attributeColumn(c TerrainScrollContext) error {
	if s.Axis >= 32 || s.AttributeAddress < 0x2000 || s.AttributeAddress >= 0x3000 || s.AttributeAddress&0x300 != 0x300 {
		return errors.New("invalid attribute-column cursor")
	}
	column := int(s.Axis >> 2)
	address := s.AttributeAddress&0xff00 | uint16(0xc0|column)
	packet := GraphicsPacket{Kind: 1, Length: 1}
	for row := 0; row < 7; row++ {
		tile, err := s.tileAt(c, int(s.MapOffset)+row*8+column)
		if err != nil {
			return err
		}
		packet.Segments = append(packet.Segments, GraphicsSegment{Address: address + uint16(row)*8, Values: []uint8{tile.Attribute}})
	}
	return c.Queue.Append(packet)
}

func (s *TerrainScrollState) tileRow(c TerrainScrollContext) error {
	if s.Axis >= 30 || s.TileAddress < 0x2000 || s.TileAddress >= 0x2400 || s.TileAddress&31 != 0 {
		return errors.New("invalid vertical tile-row cursor")
	}
	values := make([]uint8, 32)
	first := int((s.Axis&0x1c)<<1 | s.MapOffset)
	for col := 0; col < 32; col++ {
		tile, err := s.tileAt(c, first+col/4)
		if err != nil {
			return err
		}
		value := tile.Tiles[int(s.Axis&3)*4+col%4]
		values[col] = value
		if s.Axis&1 == 0 && col&1 == 0 {
			// Vertical streaming reuses one physical nametable; MapOffset only
			// chooses the source supertile window, not a second collision page.
			index := int(s.Axis>>1)*4 + col/8
			shift := uint(6 - 2*((col/2)&3))
			c.Terrain[index] = c.Terrain[index]&^(3<<shift) | collisionBits(value, c.Header.CollisionLimits)<<shift
		}
	}
	if err := c.Queue.Append(GraphicsPacket{Kind: 1, Length: 32, Segments: []GraphicsSegment{{Address: s.TileAddress, Values: values}}}); err != nil {
		return err
	}
	if c.Work != nil {
		c.Work.RecordStreamRow(s.Axis)
	}
	return nil
}

func (s *TerrainScrollState) attributeRow(c TerrainScrollContext) error {
	if s.Axis >= 30 {
		return errors.New("invalid vertical attribute row")
	}
	first := int((s.Axis<<1)&0x38 | s.MapOffset)
	address := uint16(0x2300) | uint16(uint8(first&0xbf)+0xc0)
	values := make([]uint8, 8)
	for col := 0; col < 8; col++ {
		tile, err := s.tileAt(c, first+col)
		if err != nil {
			return err
		}
		values[col] = tile.Attribute
		if s.Axis&3 == 3 {
			other, err := s.tileAt(c, (first+col)^64)
			if err != nil {
				return err
			}
			values[col] = values[col]&0xf0 | other.Attribute&0x0f
		}
	}
	return c.Queue.Append(GraphicsPacket{Kind: 1, Length: 8, Segments: []GraphicsSegment{{Address: address, Values: values}}})
}
