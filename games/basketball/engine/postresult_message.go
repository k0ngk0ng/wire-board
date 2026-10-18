package engine

import "fmt"

type PostResultMessageStep struct {
	Uploads     [2]EndingUpload
	QueueMask   uint8
	WaitNextNMI bool
}
type textMessageSpec struct {
	Offset, Length uint8
	Glyphs         []uint8
}

// PrepareOriginalPostResultMessage restores C036/BE6E through BEBC. It fills
// the0351/0374 buffers and returns the original wait-for-next-NMI boundary;
// it does not invent elapsed frames or perform the later NMI transfer.
func PrepareOriginalPostResultMessage(scene uint8, phase MatchPhase, message uint8, uploads [2]EndingUpload) (PostResultMessageStep, error) {
	initial := PostResultMessageStep{Uploads: uploads}
	if int(message) >= len(originalTextMessages) {
		return initial, fmt.Errorf("unknown original message %d", message)
	}
	r := initial
	spec := originalTextMessages[message]
	ended := false
	for i := uint8(0); i < spec.Length; i++ {
		glyph := spec.Glyphs[i]
		if glyph == 255 {
			ended = true
		}
		mark := uint8(0)
		if ended {
			glyph = 0
		} else {
			category := originalTextGlyphMarks[glyph]
			if category != 0 {
				if category == 0x0c {
					mark = 0x0c
					glyph--
				} else {
					mark = 0x0d
					glyph -= 2
				}
			}
		}
		r.Uploads[0].Data[i], r.Uploads[1].Data[i] = mark, glyph
	}
	page := uint16(0x23)
	if scene == 1 && uint8(phase)&127 == 7 {
		page = 0x20
	}
	r.Uploads[0].Address = page<<8 | uint16(spec.Offset)
	r.Uploads[1].Address = page<<8 | uint16(spec.Offset+0x20)
	r.Uploads[0].Length, r.Uploads[1].Length = spec.Length, spec.Length
	r.QueueMask = 3
	r.WaitNextNMI = true
	return r, nil
}
