package engine

import "fmt"

// CourtAnimationState contains persistent state of the selected court only.
// SwitchFlags retain contact-trigger bits; ReelFrames use bit 7 for a stopped
// reel and low bits for its original eight-step symbol animation.
type CourtAnimationState struct {
	WaterFrame      uint8    `json:"waterFrame,omitempty"`
	AlternatingSide uint8    `json:"alternatingSide,omitempty"`
	SwitchFlags     [2]uint8 `json:"switchFlags,omitempty"`
	SwitchTicks     [2]uint8 `json:"switchTicks,omitempty"`
	GateFlags       uint8    `json:"gateFlags,omitempty"`
	GateTicks       uint8    `json:"gateTicks,omitempty"`
	PaletteFrame    uint8    `json:"paletteFrame,omitempty"`
	PaletteReverse  bool     `json:"paletteReverse,omitempty"`
	ReelFrames      [6]uint8 `json:"reelFrames,omitempty"`
	SlotProgress    [2]uint8 `json:"slotProgress,omitempty"`
	OutcomeFlags    uint8    `json:"outcomeFlags,omitempty"`
}

// CourtTilePatch uses horizontal nametable mirroring: plane 1 includes $2800
// and $2C00. Tiles replace only background tile indices, never attributes.
type CourtTilePatch struct {
	Plane  uint8 `json:"plane"`
	Row    uint8 `json:"row"`
	Column uint8 `json:"column"`
	Tiles  []int `json:"tiles"`
}

type CourtAnimationContext struct {
	Court      uint8       `json:"court"`
	Active     bool        `json:"active"`
	VideoFrame uint8       `json:"videoFrame"`
	CameraX    uint8       `json:"cameraX"`
	Random     RandomState `json:"random"`
	Scores     [2]uint16   `json:"scores"`
	HoopOwners [6]uint8    `json:"hoopOwners"`
	HoopLocks  [6]uint8    `json:"hoopLocks"`
}

type CourtAnimationStep struct {
	State     CourtAnimationState `json:"state"`
	Patches   []CourtTilePatch    `json:"patches"`
	Palette   bool                `json:"palette"`
	PaletteID uint8               `json:"paletteID"`
	Sounds    []uint8             `json:"sounds"`
	Random    RandomState         `json:"random"`
	Scores    [2]uint16           `json:"scores"`
}

func (s *CourtAnimationStep) tiles(address uint16, stride uint16, values []uint8) {
	for _, v := range values {
		offset := address & 1023
		s.Patches = append(s.Patches, CourtTilePatch{Plane: uint8((address >> 11) & 1), Row: uint8(offset / 32), Column: uint8(offset % 32), Tiles: []int{int(v)}})
		address += stride
	}
}

// StepCourtAnimation restores bank 4 $9B95. Call at the original video phase;
// inactive screens and even frame counters are no-ops. Slot machines consume
// the shared world RNG and scores, so this is not a renderer-side simulation.
// Outputs are ordered video patches and explicit palette/audio/score effects.
func StepCourtAnimation(original CourtAnimationState, ctx CourtAnimationContext) (CourtAnimationStep, error) {
	initial := CourtAnimationStep{State: original, Random: ctx.Random, Scores: ctx.Scores, Patches: []CourtTilePatch{}, Sounds: []uint8{}}
	out := initial
	if ctx.Court >= 8 {
		return initial, fmt.Errorf("unknown court animation %d", ctx.Court)
	}
	if !ctx.Active || ctx.VideoFrame&1 == 0 {
		return out, nil
	}
	s := &out.State
	switch ctx.Court {
	case 0:
		s.WaterFrame = (s.WaterFrame + 1) & 3
		if ctx.CameraX >= 136 {
			return out, nil
		}
		offset := 0
		if ctx.CameraX >= 32 {
			offset = int(ctx.CameraX/8) - 3
		}
		for row, address := range []uint16{0x29e4, 0x2a04, 0x2a24} {
			out.tiles(address+uint16(offset), 1, courtWaterTiles[s.WaterFrame][row][offset:])
		}
	case 1, 7:
	case 2, 5:
		s.AlternatingSide = (s.AlternatingSide + 1) & 1
		side := s.AlternatingSide
		if s.SwitchFlags[side]&128 != 0 {
			s.SwitchFlags[side] &= 127
			s.SwitchTicks[side] = 2
		}
		if s.SwitchTicks[side] == 0 {
			return out, nil
		}
		s.SwitchTicks[side]--
		frame := 0
		if s.SwitchTicks[side] != 0 {
			frame = 1
		}
		if ctx.Court == 2 {
			count := 5
			if side == 0 {
				if ctx.CameraX < 8 {
					count = 0
				} else if ctx.CameraX < 48 {
					count = int(ctx.CameraX / 8)
				}
			} else {
				if ctx.CameraX < 128 {
					count = 0
				} else if ctx.CameraX < 168 {
					count = int((ctx.CameraX&127)/8) + 1
				}
			}
			for row := 0; row < 3; row++ {
				out.tiles(0x2a00+uint16(courtSpringAddress[side])+uint16(row*32), 1, courtSpringTiles[side][frame][row][:count])
			}
		} else {
			offset, count := 0, 8
			if side == 0 {
				if ctx.CameraX >= 168 {
					return out, nil
				}
				if ctx.CameraX >= 112 {
					offset = int(ctx.CameraX/8) - 13
					count = 8 - offset
				}
			} else {
				count = 7
				if ctx.CameraX < 16 {
					count = 0
				} else if ctx.CameraX < 72 {
					count = int((ctx.CameraX&120)/8) - 1
				}
			}
			for row := 0; row < 3; row++ {
				length := count
				if side == 1 && ctx.CameraX >= 72 && row > 0 {
					length = 8
				}
				values := append([]uint8(nil), courtBridgeTiles[frame][min(row, 1)][offset:offset+length]...)
				if row == 2 {
					for i := range values {
						values[i] += 16
					}
				}
				// Original third row is explicitly in $2Axx; the first two are $29xx.
				high := uint16(0x2900)
				if row == 2 {
					high = 0x2a00
				}
				low := uint8(int(courtBridgeAddress[side]) + offset + row*32)
				out.tiles(high+uint16(low), 1, values)
			}
		}
	case 3:
		if s.GateFlags&128 != 0 {
			s.GateFlags = (s.GateFlags & 127) | 1
			s.GateTicks = 8
		}
		if s.GateFlags&1 == 0 {
			frame := (ctx.VideoFrame >> 4) & 1
			for row, address := range []uint16{0x2a6e, 0x2a6f} {
				count := 2
				if int(ctx.CameraX) >= 112+row*8 {
					count = 0
				}
				out.tiles(address, 32, courtGateIdleTiles[frame][row][:count])
			}
		} else {
			s.GateTicks--
			if s.GateTicks == 0 {
				s.GateFlags &= 254
			}
			frame := s.GateFlags & 1
			for row, address := range []uint16{0x29d0, 0x29b1, 0x29b2} {
				count := 8
				if row == 0 {
					count = 5
				}
				if int(ctx.CameraX) >= 128+row*8 {
					count = 0
				}
				out.tiles(address, 32, courtGateTiles[frame][row][:count])
			}
		}
	case 4:
		if s.PaletteReverse {
			s.PaletteFrame--
		} else {
			s.PaletteFrame++
		}
		if s.PaletteFrame&128 != 0 {
			s.PaletteFrame = 2
		} else if s.PaletteFrame == 3 {
			s.PaletteFrame = 0
		}
		if s.PaletteFrame >= 3 {
			return initial, fmt.Errorf("court palette frame outside original cycle")
		}
		out.Palette, out.PaletteID = true, [3]uint8{5, 9, 10}[s.PaletteFrame]
	case 6:
		for i, v := range s.ReelFrames {
			if v&128 == 0 {
				s.ReelFrames[i] = (v + 1) & 7
			}
		}
		for side := 0; side < 2; side++ {
			if s.SlotProgress[side] == 0 {
				for reel := 0; reel < 3; reel++ {
					s.ReelFrames[side*3+reel] &= 127
				}
				continue
			}
			var value uint8
			out.Random, value = NextRandom(out.Random)
			s.SlotProgress[side] += (value & 7) | 3
			phase := s.SlotProgress[side] & 240
			stop := -1
			switch phase {
			case 0:
				stop = 0
			case 32:
				stop = 1
			case 64:
				stop = 2
			case 112:
				s.SlotProgress[side] = 0
				out.Sounds = append(out.Sounds, 13)
			}
			if stop >= 0 {
				s.ReelFrames[side*3+stop] = (s.ReelFrames[side*3+stop] | 128) & 254
				s.SlotProgress[side] += 16
				out.Sounds = append(out.Sounds, 14)
				if stop == 2 {
					var symbols [3]uint8
					for reel := 0; reel < 3; reel++ {
						index := (s.ReelFrames[side*3+reel] & 127) / 2
						if index >= 5 {
							return initial, fmt.Errorf("slot symbol outside original table")
						}
						symbols[reel] = courtSlotSymbols[reel][index]
					}
					if symbols[0] == symbols[1] && symbols[0] == symbols[2] {
						s.OutcomeFlags |= 128
						tier := 0
						owner := uint8(16)
						for i, lock := range ctx.HoopLocks {
							if lock != 0 && ctx.HoopOwners[i] < owner {
								owner = ctx.HoopOwners[i]
							}
						}
						if owner != 16 {
							if owner < 7 || owner > 12 {
								return initial, fmt.Errorf("slot award hoop owner outside original mounted domain")
							}
							tier = int(owner-7) / 2
						}
						target := side ^ 1
						symbol := symbols[0]
						amount := uint16(1)
						switch symbol {
						case 0:
							amount = [3]uint16{12, 12, 16}[tier]
						case 1:
							amount = [3]uint16{9, 9, 12}[tier]
						case 2:
							amount = [3]uint16{6, 6, 8}[tier]
						}
						if symbol == 1 {
							if out.Scores[target] < amount {
								out.Scores[target] = 0
							} else {
								out.Scores[target] -= amount
							}
							out.Sounds = append(out.Sounds, 9)
						} else {
							out.Scores[target] += amount
							out.Sounds = append(out.Sounds, 46)
						}
					}
				}
			}
		}
		var top, bottom [8]uint8
		top[2], top[5], bottom[2], bottom[5] = 74, 74, 74, 74
		side := 0
		if ctx.CameraX >= 128 {
			side = 1
		}
		for reel := 0; reel < 3; reel++ {
			frame := s.ReelFrames[side*3+reel] & 127
			if frame > 7 {
				return initial, fmt.Errorf("slot frame outside original cycle")
			}
			symbol := courtSlotSymbols[reel][frame/2]
			above, below := 8+2*symbol, 24+2*symbol
			if frame&1 != 0 {
				above = 24 + 2*courtSlotSymbols[reel][(frame+1)/2]
				below = 8 + 2*symbol
			}
			at := reel * 3
			top[at], top[at+1], bottom[at], bottom[at+1] = above, above+1, below, below+1
		}
		index := ctx.CameraX / 8
		count := courtSlotCounts[index]
		if count != 0 {
			offset := uint8(0)
			if index < 12 {
				offset = courtSlotOffsets[count]
			}
			if int(offset)+int(count) > 8 {
				return initial, fmt.Errorf("slot clipping outside tile row")
			}
			out.tiles(0x2a00+uint16(courtSlotAddresses[index]), 1, top[offset:offset+count])
			out.tiles(0x2a00+uint16(uint8(courtSlotAddresses[index]+32)), 1, bottom[offset:offset+count])
		}
		palette := (ctx.VideoFrame >> 1) & 3
		if s.SlotProgress == ([2]uint8{}) {
			s.OutcomeFlags = 0
		} else if s.OutcomeFlags&128 != 0 {
			palette += 4
		}
		out.Palette, out.PaletteID = true, [8]uint8{7, 13, 14, 15, 7, 13, 11, 12}[palette]
	}
	return out, nil
}
