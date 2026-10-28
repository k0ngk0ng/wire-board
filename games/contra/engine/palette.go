package contra

import "errors"

// PaletteOutput separates pending colors from the palette actually submitted
// at the display boundary. Header palette IDs remain owned by the world.
type PaletteOutput struct {
	Desired, Applied                                          [32]uint8
	RequestedColors, FadeTimer, Cycle, PauseCycle, FlashTimer uint8
}

type PaletteResult struct {
	GenerationBiasChanged bool
	GenerationBiasValue   uint8
}

type PaletteCycleContext struct {
	GameRoutine, Counter, Level, SceneType, Alternate uint8
	Header                                            *PaletteState
	Resources                                         *LevelResources
	Queue                                             *GraphicsQueue
}

// The indexed source at CF64 uses only indices 1..8. CF64 itself is the BB
// displacement byte of the preceding branch, not a fade amount for zero.
var paletteFadeAmounts = [...]uint8{0, 0, 16, 16, 32, 32, 48, 48}
var paletteCycleCounts = [...]uint8{4, 4, 3, 4, 4, 4, 4, 4, 4}     // D03D
var paletteBossSecond = [...]uint8{19, 20, 21, 20, 27, 28, 29, 28} // D046
var paletteFlashColors = [...]uint8{15, 32, 22, 17}                // D04E
var paletteOrdinarySecond = [...]uint8{4, 92, 4, 93}               // D052
var paletteEndingSecond = [...]uint8{102, 106, 107, 106}           // D056

// Load follows CEE3. It replaces the pending count, preserves unrequested
// colors, and consumes one positive fade step only after a complete load.
func (p *PaletteOutput) Load(count, sceneType uint8, header PaletteState, resources *LevelResources) (PaletteResult, error) {
	if count != 16 && count != 32 || resources == nil {
		return PaletteResult{}, errors.New("invalid palette load request")
	}
	indices := header.PaletteIndices
	if count == 16 {
		// Palette() expands eight groups. Do not require unused sprite groups
		// to be installed when the original requested only the background.
		for i := 4; i < 8; i++ {
			indices[i] = indices[0]
		}
	}
	colors, err := resources.Palette(indices)
	if err != nil {
		return PaletteResult{}, err
	}
	next := *p
	next.RequestedColors = count
	for i := 0; i < int(count); i++ {
		color := colors[i]
		if i%4 != 0 && next.FadeTimer != 0 && i < 16 && (sceneType&0x80 != 0 || i >= 4) {
			if next.FadeTimer&0x80 != 0 || next.FadeTimer >= 9 {
				color = 0x0f
			} else {
				amount := paletteFadeAmounts[next.FadeTimer-1]
				if color < amount {
					color = 0x0f
				} else {
					color -= amount
				}
			}
		}
		next.Desired[i] = color
	}
	if next.FadeTimer != 0 && next.FadeTimer&0x80 == 0 {
		next.FadeTimer--
	}
	*p = next
	// The last source palette lookup writes CF09's pointer low byte even if
	// it equals the previously held compatibility value. D0E3 is source data.
	last := header.PaletteIndices[count/4-1]
	return PaletteResult{GenerationBiasChanged: true, GenerationBiasValue: uint8(0xe3 + uint16(last)*3)}, nil
}

// Commit follows Japanese CC02. Busy graphics defer the palette without
// clearing its request. Palette mirrors are applied in original write order;
// the later sprite-universal entries can change the visible backdrop in flash.
func (p *PaletteOutput) Commit(graphicsUsed uint8) (bool, error) {
	if p.RequestedColors == 0 {
		return false, nil
	}
	if p.RequestedColors != 16 && p.RequestedColors != 32 {
		return false, errors.New("invalid pending palette length")
	}
	if graphicsUsed >= 0x47 {
		return false, nil
	}
	next := *p
	for i := 0; i < int(next.RequestedColors); i++ {
		address := i
		if address >= 16 && address%4 == 0 {
			address -= 16
		}
		color := next.Desired[i] & 0x3f
		next.Applied[address] = color
		if address < 16 && address%4 == 0 {
			next.Applied[address+16] = color
		}
	}
	next.RequestedColors = 0
	*p = next
	return true, nil
}

// Step follows CF6D, including the cycle-before-flash order and the Base 1 Boss
// attribute packet. The palette, world Header and shared queue commit together.
func (p *PaletteOutput) Step(c PaletteCycleContext) (PaletteResult, error) {
	if c.Level >= uint8(len(paletteCycleCounts)) || c.Header == nil || c.Resources == nil || c.Queue == nil {
		return PaletteResult{}, errors.New("incomplete palette cycle context")
	}
	// Original palette storage starts at graphics offset C0. Inputs beyond
	// that boundary alias separate owners and are not a valid semantic queue.
	if c.Queue.Used > 0xc0 {
		return PaletteResult{}, errors.New("graphics queue overlaps palette storage")
	}
	if p.RequestedColors >= c.GameRoutine {
		return PaletteResult{}, nil
	}
	next, header := *p, *c.Header
	queue := c.Queue.Clone()
	var result PaletteResult
	if c.Counter&7 == 5 && next.PauseCycle == 0 {
		next.Cycle++
		if next.Cycle >= paletteCycleCounts[c.Level] {
			next.Cycle = 0
		}
		header.PaletteIndices[3] = header.CycleIndices[next.Cycle]
		if c.SceneType&0x80 != 0 {
			header.PaletteIndices[2] = paletteBossSecond[(c.Level&2)*2+next.Cycle]
			if c.Level == 1 {
				if int(queue.Used)+9 > 0xc0 {
					return PaletteResult{}, errors.New("palette attribute packet overlaps color storage")
				}
				phase := (c.Counter >> 3) & 15
				values := []uint8{phase | 0x50, (phase+3)&15 | 0x50, (phase+12)&15 | 0x50, (phase+15)&15 | 0x50}
				packet := GraphicsPacket{Kind: 1, Length: 4, Segments: []GraphicsSegment{{Address: 0x23c2, Values: values}}}
				// CF6D has no general 64-byte admission gate. Only the bounded
				// frame buffer can reject a malformed/unrepresentable request.
				if err := queue.Append(packet); err != nil {
					return PaletteResult{}, err
				}
			}
		} else if c.Level == 8 {
			header.PaletteIndices[2] = paletteEndingSecond[next.Cycle]
		} else if c.Level == 0 || c.Level != 7 && c.Alternate&0x80 == 0 {
			header.PaletteIndices[2] = paletteOrdinarySecond[next.Cycle]
		}
		var err error
		result, err = next.Load(16, c.SceneType, header, c.Resources)
		if err != nil {
			return PaletteResult{}, err
		}
	}
	if next.FlashTimer != 0 {
		next.FlashTimer--
		if next.FlashTimer&1 == 0 {
			color := paletteFlashColors[(next.FlashTimer>>1)&3]
			for i := 16; i < 32; i += 4 {
				next.Desired[i] = color
			}
			next.RequestedColors = 32
		}
	}
	*p, *c.Header, *c.Queue = next, header, queue
	return result, nil
}
