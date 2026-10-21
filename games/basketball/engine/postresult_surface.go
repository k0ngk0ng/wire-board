package engine

import "fmt"

// OriginalPostResultDisplayState stores committed memory and OAM submission
// stages, not generated240-row packets. FieldCHR is the live logical4F/50;
// VisibleFieldCHR is the last actual mapping committed to the display.
type OriginalPostResultDisplayState struct {
	Surface         OriginalRasterSurface           `json:"surface"`
	WorkingOAM      [256]uint8                      `json:"workingOAM"`
	VisibleOAM      [256]uint8                      `json:"visibleOAM"`
	SpriteContext   OriginalPostResultSpriteContext `json:"spriteContext"`
	FieldCHR        [2]uint8                        `json:"fieldCHR"`
	VisibleFieldCHR [2]uint8                        `json:"visibleFieldCHR"`
	SpriteCHR       [4]uint8                        `json:"spriteCHR"`
	Control         uint8                           `json:"control"`
	Mask            uint8                           `json:"mask"`
	NMIRaster       OriginalPostResultRasterSource  `json:"nmiRaster"`
	HasSubmission   bool                            `json:"hasSubmission"`
	HasVisible      bool                            `json:"hasVisible"`
	HasNMI          bool                            `json:"hasNMI"`
}

// CommitOriginalPostResultSurface applies an already-committed PPU write. It
// never examines or clears pending queue headers. Address increments wrap at
// 14bits;3000..3EFF mirror nametables, palette entries10/14/18/1C alias00/04/08/0C.
// Writes to the immutable source CHR-ROM window have no effect on this surface.
func CommitOriginalPostResultSurface(original OriginalRasterSurface, address uint16, data []uint8, vertical bool) (OriginalRasterSurface, error) {
	if original.Mirroring > 1 {
		return original, fmt.Errorf("invalid committed surface mirroring")
	}
	if address&0x8000 != 0 {
		return original, fmt.Errorf("inactive source queue is not a committed write")
	}
	if len(data) > 16384 {
		return original, fmt.Errorf("committed write exceeds PPU address space")
	}
	result := original
	step := uint16(1)
	if vertical {
		step = 32
	}
	for _, value := range data {
		at := address & 0x3fff
		switch {
		case at >= 0x3f00:
			index := at & 31
			if index >= 16 && index&3 == 0 {
				index -= 16
			}
			result.Palette[index] = value & 63
			if index < 16 && index&3 == 0 {
				result.Palette[index+16] = value & 63
			}
		case at >= 0x2000:
			offset := (at - 0x2000) & 0xfff
			table := offset >> 10
			physical := table & 1
			if result.Mirroring == 1 {
				physical = table >> 1
			}
			result.Nametables[physical][offset&1023] = value
		}
		address = (address + step) & 0x3fff
	}
	return result, nil
}

// ApplyOriginalPostResultDisplayEffects owns only committed surface/OAM state.
// Queueing and sound requests remain with their owners. Any unsupported effect
// rejects the complete batch, preserving the prior state and all caller data.
func ApplyOriginalPostResultDisplayEffects(original OriginalPostResultDisplayState, effects []PostResultVideoEffect) (OriginalPostResultDisplayState, error) {
	result := original
	for _, effect := range effects {
		switch effect.Kind {
		case "upload-commit":
			if effect.Upload.Length > 32 {
				return original, fmt.Errorf("committed presentation upload exceeds buffer")
			}
			next, err := CommitOriginalPostResultSurface(result.Surface, effect.Upload.Address, effect.Upload.Data[:effect.Upload.Length], effect.Control&4 != 0)
			if err != nil {
				return original, err
			}
			result.Surface = next
		case "background-commit":
			next, err := CommitOriginalPostResultSurface(result.Surface, effect.Background.Target, effect.Background.Data, false)
			if err != nil {
				return original, err
			}
			result.Surface = next
			result.FieldCHR = effect.Background.FieldCHR
			// DC96 updates logical4F/50; next normalNMI publishes the final mapping.
		case "background-chr":
			result.FieldCHR = effect.FieldCHR
			result.VisibleFieldCHR = effect.FieldCHR
			result.Control &^= 4
		case "sprite-chr":
			result.SpriteCHR = effect.CHR
		case "render-mask":
			if effect.Value&224 != 0 {
				return original, fmt.Errorf("unrestored presentation emphasis mask")
			}
			result.Mask = effect.Value
		case "mirroring":
			if effect.Value > 1 {
				return original, fmt.Errorf("invalid committed mirroring")
			}
			result.Surface.Mirroring = effect.Value
		case "actors-submit":
			submission, err := EmitOriginalPostResultOAM(result.WorkingOAM, effect.Actors, result.SpriteContext)
			if err != nil {
				return original, err
			}
			result.WorkingOAM = submission.OAM
			result.HasSubmission = true
		case "normal-nmi":
			source := effect.Raster
			mode := source.MachineControl & 192
			if mode == 0 {
				return original, fmt.Errorf("normal display NMI requires active source mode")
			}
			result.VisibleOAM = result.WorkingOAM
			result.HasVisible = true
			result.NMIRaster = source
			result.HasNMI = true
			result.Control = source.Control
			result.Mask = source.Mask
			result.FieldCHR = source.BackgroundCHR
			result.VisibleFieldCHR = source.BackgroundCHR
			if mode == 192 {
				result.SpriteCHR = source.SpriteCHR
			}
			if mode == 128 && source.Scene == 7 {
				phase := source.Phase & 127
				if phase == 11 || phase < 5 {
					result.Surface.Mirroring = 0
				} else {
					result.Surface.Mirroring = 1
				}
			}
		case "oam-reverse":
			result.WorkingOAM = ReverseOriginalPostResultOAM(result.WorkingOAM)
		case "early-irq":
			step, err := BuildOriginalPostResultRasterIRQ(effect.Value, effect.Raster.Control, effect.Raster)
			if err != nil {
				return original, err
			}
			result.Control = step.Control
		case "blanking-handoff":
			result.Control &^= 128
		case "upload-queued", "sound", "reset": // No pending-queue/audio/reset lifecycle ownership here.
		default:
			return original, fmt.Errorf("unknown committed display effect %q", effect.Kind)
		}
		for _, page := range append(result.VisibleFieldCHR[:], result.SpriteCHR[:]...) {
			if page >= 128 {
				return original, fmt.Errorf("committed CHR page outside source art")
			}
		}
	}
	return result, nil
}
