package contra

import "errors"

// BackgroundWrite is a semantic deferred tile/attribute update. Collision
// changes occur during simulation; presentation applies these writes at the
// next display boundary, matching the original queued graphics timing.
type BackgroundWrite struct {
	Address uint16
	Step    uint16
	Values  []uint8
}

type ScrollResult struct {
	Writes               []BackgroundWrite
	ResetEnemyReadOffset bool
}

// JungleScroll retains the two partially streamed supertile maps and the
// current offscreen write cursors. These are level data, not a CPU address space.
type JungleScroll struct {
	AlternateGraphicsScreen       uint8
	Column, TransitionRemaining   uint8
	TileAddress, AttributeAddress uint16
	MapOffset, TankScroll         uint8
	Supertiles                    [128]uint8
}

// Advance reproduces the horizontal first-stage branch at $DB81. Alternate
// Boss graphics are a distinct pending feature and fail before any commit.
func (s *JungleScroll) Advance(world *OutdoorWorld, terrain *CollisionWindow) (ScrollResult, error) {
	if world == nil || terrain == nil || world.Level != 0 || world.SceneType != 0 || world.ScrollType != 0 {
		return ScrollResult{}, errors.New("scroll phase is not a recovered jungle phase")
	}
	next, w, c := *s, *world, *terrain
	result, err := next.advance(&w, &c)
	if err != nil {
		return ScrollResult{}, err
	}
	*s, *world, *terrain = next, w, c
	return result, nil
}

func (s *JungleScroll) advance(w *OutdoorWorld, collision *CollisionWindow) (ScrollResult, error) {
	if s.Column >= 32 || s.MapOffset != 0 && s.MapOffset != 64 ||
		(s.TileAddress>>8 != 0x20 && s.TileAddress>>8 != 0x24) || uint8(s.TileAddress) >= 32 ||
		(s.AttributeAddress>>8 != 0x23 && s.AttributeAddress>>8 != 0x27) {
		return ScrollResult{}, errors.New("invalid jungle streaming cursor")
	}
	if w.AutoScroll[1] != 0 {
		w.AutoScroll[1]--
		if w.AutoScroll[1] != 0 {
			w.FrameScroll = 1
		} else if w.AutoScroll[0] != 0 {
			w.AutoScroll[0]--
			if w.AutoScroll[0] == 0 {
				w.BossScrollComplete++
			}
			w.FrameScroll = 1
		}
	} else if w.AutoScroll[0] != 0 {
		w.AutoScroll[0]--
		if w.AutoScroll[0] == 0 {
			w.BossScrollComplete++
		}
		w.FrameScroll = 1
	}
	var result ScrollResult
	remaining := w.FrameScroll + s.TankScroll
	for remaining != 0 {
		w.ScreenScroll++
		if w.ScreenScroll == 0 {
			w.Screen++
			if w.Screen == s.AlternateGraphicsScreen {
				return ScrollResult{}, errors.New("alternate jungle graphics transition has not been recovered")
			}
			result.ResetEnemyReadOffset = true
			w.Scroll.NameTable ^= 1
		}
		if w.ScreenScroll&7 == 0 {
			write, err := s.tileColumn(collision)
			if err != nil {
				return ScrollResult{}, err
			}
			result.Writes = append(result.Writes, write)
			s.TileAddress++
			s.Column++
			if s.Column >= 32 {
				s.MapOffset ^= 64
				screen, err := JungleScreen(int(w.Screen + 2))
				if err != nil {
					return ScrollResult{}, err
				}
				copy(s.Supertiles[int(s.MapOffset):int(s.MapOffset)+56], screen.SupertileIDs)
				s.TileAddress = (s.TileAddress & 0xff00) ^ 0x0400
				s.Column = 0
				s.AttributeAddress ^= 0x0400
			}
		} else if w.ScreenScroll&15 == 3 {
			write, err := s.attributeColumn()
			if err != nil {
				return ScrollResult{}, err
			}
			result.Writes = append(result.Writes, write)
		}
		w.Scroll.X++
		remaining--
	}
	return result, nil
}

func (s *JungleScroll) tileColumn(collision *CollisionWindow) (BackgroundWrite, error) {
	layout, err := jungle()
	if err != nil {
		return BackgroundWrite{}, err
	}
	write := BackgroundWrite{Address: s.TileAddress, Step: 32, Values: make([]uint8, 28)}
	page := int(s.TileAddress>>10) & 1
	column := int(s.Column)
	for row := 0; row < 28; row++ {
		id := s.Supertiles[int(s.MapOffset)+(row/4)*8+column/4]
		if int(id) >= len(layout.Supertiles) {
			return BackgroundWrite{}, errors.New("unknown streaming supertile")
		}
		tile := layout.Supertiles[id].Tiles[(row%4)*4+column%4]
		write.Values[row] = tile
		if uint8(s.TileAddress)&1 == 0 && row&1 == 0 {
			index := page*64 + (row/2)*4 + column/8
			shift := uint(6 - 2*((column/2)%4))
			collision[index] = collision[index]&^(3<<shift) | collisionBits(tile, layout.CollisionLimits)<<shift
		}
	}
	return write, nil
}

func (s *JungleScroll) attributeColumn() (BackgroundWrite, error) {
	layout, err := jungle()
	if err != nil {
		return BackgroundWrite{}, err
	}
	column := int(s.Column >> 2)
	write := BackgroundWrite{Address: (s.AttributeAddress & 0xff00) | uint16(0xc0|column), Step: 8, Values: make([]uint8, 7)}
	for row := 0; row < 7; row++ {
		id := s.Supertiles[int(s.MapOffset)+row*8+column]
		if int(id) >= len(layout.Supertiles) {
			return BackgroundWrite{}, errors.New("unknown attribute supertile")
		}
		write.Values[row] = layout.Supertiles[id].Attribute
	}
	return write, nil
}
