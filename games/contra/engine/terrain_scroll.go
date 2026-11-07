package contra

import "errors"

type TerrainScrollEffectKind string

const (
	ScrollCHRPreset    TerrainScrollEffectKind = "chr-preset"
	ScrollPaletteLoad  TerrainScrollEffectKind = "palette-load"
	ScrollPaletteTimer TerrainScrollEffectKind = "palette-timer"
	ScrollAudioReset   TerrainScrollEffectKind = "audio-reset"
	ScrollSound        TerrainScrollEffectKind = "sound"
)

// Effects describe original visual/audio work in call order. The world owner
// applies them to its sole Palette/CHR state; no recorded output is supplied.
type TerrainScrollEffect struct {
	Kind  TerrainScrollEffectKind
	Value uint8
}

type TerrainScrollResult struct {
	ResetEnemyReadOffset bool
	ForceJump            [2]uint8
	Effects              []TerrainScrollEffect
}

// TerrainScrollState owns streaming cursors and maps only. Screen, camera,
// automatic scroll and base progression remain owned by OutdoorWorld. Variant
// can switch to Boss resources one presentation phase before SceneType changes.
type TerrainScrollState struct {
	AlternateGraphicsScreen       uint8
	Axis, TransitionRemaining     uint8
	TileAddress, AttributeAddress uint16
	MapOffset, TankScroll         uint8
	Variant                       MapVariant
	Supertiles                    [128]uint8
}

type TerrainScrollContext struct {
	Work          *TerrainWork
	World         *OutdoorWorld
	Terrain       *CollisionWindow
	Header        *PaletteState
	ScreenCleared *uint8
	Queue         *GraphicsQueue
	Resources     *LevelResources
}

// AdvanceQueued is one atomic DB81 phase. Its writes stay in the shared queue
// until the next presentation boundary; simulation collision changes commit
// with the phase. The enclosing world transaction owns application of Effects.
func (s *TerrainScrollState) AdvanceQueued(c TerrainScrollContext) (TerrainScrollResult, error) {
	if s == nil || c.World == nil || c.Terrain == nil || c.Header == nil || c.ScreenCleared == nil || c.Queue == nil || c.Resources == nil {
		return TerrainScrollResult{}, errors.New("incomplete terrain scroll context")
	}
	// The physical graphics region ends where palette state begins ($07C0).
	// The shared byte cursor can represent 255, but that is not safe capacity.
	if c.Queue.Used > 192 {
		return TerrainScrollResult{}, errors.New("graphics cursor overlaps palette storage")
	}
	state, world, terrain, header, cleared := *s, *c.World, *c.Terrain, *c.Header, *c.ScreenCleared
	queue := c.Queue.Clone()
	next := TerrainScrollContext{World: &world, Terrain: &terrain, Header: &header, ScreenCleared: &cleared, Queue: &queue, Resources: c.Resources}
	var work TerrainWork
	if c.Work != nil {
		work = *c.Work
		next.Work = &work
	}
	result, err := state.advanceTerrain(next)
	if err != nil {
		return TerrainScrollResult{}, err
	}
	if queue.Used > 192 {
		return TerrainScrollResult{}, errors.New("terrain graphics exceed physical frame storage")
	}
	*s, *c.World, *c.Terrain, *c.Header, *c.ScreenCleared, *c.Queue = state, world, terrain, header, cleared, queue
	if c.Work != nil {
		*c.Work = work
	}
	return result, nil
}

func (s *TerrainScrollState) advanceTerrain(c TerrainScrollContext) (TerrainScrollResult, error) {
	w := c.World
	info, err := c.Resources.Stage(int(w.Level))
	if err != nil {
		return TerrainScrollResult{}, err
	}
	if s.MapOffset != 0 && s.MapOffset != 64 || w.ScrollType != info.ScrollType ||
		(w.SceneType != info.SceneType && !(w.SceneType == 0x80 && info.Boss.Available)) {
		return TerrainScrollResult{}, errors.New("invalid terrain streaming scene")
	}
	if _, _, err := c.Resources.mapSet(int(w.Level), s.Variant); err != nil {
		return TerrainScrollResult{}, err
	}
	var out TerrainScrollResult
	if w.ScrollType != 0 {
		err = s.vertical(c, info, &out)
	} else if w.SceneType != 0 {
		err = s.base(c, info, &out)
	} else {
		err = s.horizontal(c, info, &out)
	}
	return out, err
}

func (s *TerrainScrollState) horizontal(c TerrainScrollContext, info StageDefinition, out *TerrainScrollResult) error {
	w := c.World
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
	for remaining := w.FrameScroll + s.TankScroll; remaining != 0; remaining-- {
		w.ScreenScroll++
		if w.ScreenScroll == 0 {
			w.Screen++
			if w.Screen == s.AlternateGraphicsScreen {
				if err := s.alternateCHR(c, info, out); err != nil {
					return err
				}
				if err := s.alternateHeader(c, info, out); err != nil {
					return err
				}
			}
			out.ResetEnemyReadOffset = true
			w.Scroll.NameTable ^= 1
		}
		if w.ScreenScroll&7 == 0 {
			if err := s.tileColumn(c); err != nil {
				return err
			}
			s.TileAddress = (s.TileAddress & 0xff00) | uint16(uint8(s.TileAddress)+1)
			s.Axis++
			if s.Axis >= 32 {
				s.MapOffset ^= 64
				if err := c.Resources.LoadMap(int(w.Level), s.Variant, int(w.Screen+2), int(s.MapOffset), &s.Supertiles); err != nil {
					return err
				}
				s.TileAddress = (s.TileAddress & 0xff00) ^ 0x400
				s.Axis = 0
				s.AttributeAddress ^= 0x400
			}
		} else if w.ScreenScroll&15 == 3 {
			if err := s.attributeColumn(c); err != nil {
				return err
			}
		}
		w.Scroll.X++
	}
	return nil
}

func (s *TerrainScrollState) vertical(c TerrainScrollContext, info StageDefinition, out *TerrainScrollResult) error {
	w := c.World
	if w.AutoScroll[0] != 0 {
		out.Effects = append(out.Effects, TerrainScrollEffect{ScrollPaletteTimer, 16})
		w.FrameScroll = 1
		w.AutoScroll[0]--
		if w.AutoScroll[0] == 0 {
			w.BossScrollComplete++
		}
	}
	for remaining := w.FrameScroll; remaining != 0; remaining-- {
		w.ScreenScroll++
		if w.ScreenScroll >= 240 {
			w.ScreenScroll = 0
			out.ResetEnemyReadOffset = true
			w.Screen++
			if w.Screen == s.AlternateGraphicsScreen {
				if err := s.alternateCHR(c, info, out); err != nil {
					return err
				}
				out.Effects = append(out.Effects, TerrainScrollEffect{ScrollPaletteTimer, 128})
				if err := s.alternateHeader(c, info, out); err != nil {
					return err
				}
			}
		}
		if w.ScreenScroll&7 == 0 {
			if err := s.tileRow(c); err != nil {
				return err
			}
			s.TileAddress -= 32
			s.Axis--
			if int8(s.Axis) < 0 {
				s.MapOffset ^= 64
				if err := c.Resources.LoadMap(int(w.Level), s.Variant, int(w.Screen+1), int(s.MapOffset), &s.Supertiles); err != nil {
					return err
				}
				s.Axis = 29
				s.TileAddress = 0x23a0
			}
		} else if w.ScreenScroll&15 == 7 {
			if err := s.attributeRow(c); err != nil {
				return err
			}
		}
		w.Scroll.Y--
		if w.Scroll.Y == 255 {
			w.Scroll.Y = 239
		}
	}
	return nil
}

func (s *TerrainScrollState) base(c TerrainScrollContext, info StageDefinition, out *TerrainScrollResult) error {
	w := c.World
	// Japanese DC95 returns here. It does not run the US fence-graphics path.
	if *c.ScreenCleared&0x80 == 0 {
		return nil
	}
	if s.TransitionRemaining&0x80 != 0 {
		s.TransitionRemaining = 0
		w.IndoorScroll++
		w.ScreenScroll++
		if w.ScreenScroll == 4 {
			out.ForceJump = [2]uint8{1, 1}
			*c.ScreenCleared = 0
			w.ScreenScroll = 0
			out.ResetEnemyReadOffset = true
			w.Screen++
			if w.Screen == w.StopScroll {
				if !info.Boss.Available {
					return errors.New("missing base Boss resources")
				}
				w.SceneType = info.Boss.SceneType
				if err := s.alternateHeader(c, info, out); err != nil {
					return err
				}
				out.Effects = append(out.Effects, TerrainScrollEffect{ScrollAudioReset, 0})
				if err := emitScrollCHR(c, info.Boss.CHRPreset, out); err != nil {
					return err
				}
				out.Effects = append(out.Effects, TerrainScrollEffect{ScrollSound, 0x5e})
				w.Scroll.NameTable = 0xb1
				w.Scroll.Y = 0xe0
			}
			out.Effects = append(out.Effects, TerrainScrollEffect{ScrollPaletteTimer, 12})
			if err := emitScrollPalette(c, 32, out); err != nil {
				return err
			}
		}
		w.Scroll.NameTable ^= 1
		return nil
	}
	if s.TransitionRemaining == 0 {
		if w.IndoorScroll == 0 {
			return nil
		}
		s.Axis = 0
		s.TileAddress &= 0xff00
		s.AttributeAddress &= 0xff00
		s.TransitionRemaining = 32
		s.TileAddress ^= 0x400
		s.AttributeAddress ^= 0x400
		index := int(w.Screen*4 + w.ScreenScroll + 1)
		if w.Screen == s.AlternateGraphicsScreen && w.ScreenScroll == 3 {
			if !info.Boss.Available {
				return errors.New("missing base Boss map")
			}
			s.Variant = BaseBossMap
			index = info.Boss.MapIndex
		}
		if err := c.Resources.LoadMap(int(w.Level), s.Variant, index, int(s.MapOffset), &s.Supertiles); err != nil {
			return err
		}
	}
	if err := s.tileColumn(c); err != nil {
		return err
	}
	if err := s.attributeColumn(c); err != nil {
		return err
	}
	s.Axis++
	s.TileAddress = (s.TileAddress & 0xff00) | uint16(uint8(s.TileAddress)+1)
	s.TransitionRemaining--
	if s.TransitionRemaining == 0 {
		s.TransitionRemaining = 128
	}
	return nil
}

func (s *TerrainScrollState) alternateCHR(c TerrainScrollContext, info StageDefinition, out *TerrainScrollResult) error {
	if !info.Alternate.HasCHR {
		return nil
	}
	return emitScrollCHR(c, info.Alternate.CHRPreset, out)
}
func (s *TerrainScrollState) alternateHeader(c TerrainScrollContext, info StageDefinition, out *TerrainScrollResult) error {
	s.AlternateGraphicsScreen = 255
	*c.Header = info.Alternate.Palette
	return emitScrollPalette(c, 32, out)
}
func emitScrollCHR(c TerrainScrollContext, preset int, out *TerrainScrollResult) error {
	if _, err := c.Resources.CHRPreset(preset); err != nil {
		return err
	}
	out.Effects = append(out.Effects, TerrainScrollEffect{ScrollCHRPreset, uint8(preset)})
	return nil
}
func emitScrollPalette(c TerrainScrollContext, count uint8, out *TerrainScrollResult) error {
	if _, err := c.Resources.Palette(c.Header.PaletteIndices); err != nil {
		return err
	}
	out.Effects = append(out.Effects, TerrainScrollEffect{ScrollPaletteLoad, count})
	return nil
}
