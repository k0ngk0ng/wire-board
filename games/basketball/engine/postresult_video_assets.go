package engine

import "fmt"

type PresentationBackground struct {
	Target                   uint16
	Data                     []uint8
	FieldCHR                 [2]uint8
	PaletteID, CompressedCHR uint8
	WorkCycles               uint32
}

func OriginalPresentationBackground(preset uint8) (PresentationBackground, error) {
	r, ok := originalPresentationBackgrounds[preset]
	if !ok {
		return PresentationBackground{}, fmt.Errorf("unknown original presentation background %d", preset)
	}
	r.Data = append([]uint8(nil), r.Data...)
	return r, nil
}

// BuildOriginalPresentationPalette restores9526 up to its pending0374 header.
// It preserves the shared buffer tail when0301 bit0 excludes sprite colors.
func BuildOriginalPresentationPalette(original EndingUpload, background uint8, sprites [2]uint8, flags, level uint8) (EndingUpload, error) {
	bg, ok := originalPresentationBGPalettes[background]
	if !ok || level > 64 || level&15 != 0 {
		return original, fmt.Errorf("unsupported presentation palette %d level%d", background, level)
	}
	r := original
	r.Address = 0x3f00
	r.Length = 16
	clamp := func(v uint8) uint8 {
		if level == 0 {
			return 15
		}
		if v >= level {
			return (v & 15) + level - 16
		}
		return v
	}
	for i, v := range bg {
		if flags&64 == 0 || i < 8 {
			v = clamp(v)
		}
		r.Data[i] = v
	}
	if flags&1 == 0 {
		r.Length = 32
		for actor, id := range sprites {
			palette, ok := originalPresentationSpritePalettes[id]
			if !ok {
				return original, fmt.Errorf("unsupported sprite palette%d", id)
			}
			for i, v := range palette {
				r.Data[16+actor*8+i] = clamp(v)
			}
		}
	}
	for i := 4; i < 32; i += 4 {
		r.Data[i] = r.Data[0]
	}
	return r, nil
}
