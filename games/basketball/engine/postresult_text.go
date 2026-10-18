package engine

import "fmt"

// SceneTextProgress owns06AE..06B7 only. ActorContext owns06AB/06AD and the
// caller's ending context owns06AC; pending upload buffers stay explicit.
type SceneTextProgress struct {
	Address     uint16 `json:"address"`
	Cursor      uint16 `json:"cursor"`
	Repeat      uint8  `json:"repeat"`
	Clock       uint8  `json:"clock"`
	Pause       uint8  `json:"pause"`
	Delay       uint8  `json:"delay"`
	LineAddress uint16 `json:"lineAddress"`
}
type SceneTextStep struct {
	Context            ResultActorContext
	Active             uint8
	Progress           SceneTextProgress
	Uploads            [2]EndingUpload
	QueueMask          uint8
	ClearUploadControl bool
}

// StepOriginalSceneText restores the complete FD0A/A496 logical call. It runs
// before the Scene7 phase/actor update. Uploads are the retained 030B and032E
// buffers; QueueMask records stores even when their values repeat. It neither
// advances video time nor consumes an upload on behalf of NMI.
func StepOriginalSceneText(scene uint8, context ResultActorContext, active uint8, progress SceneTextProgress, uploads [2]EndingUpload) (SceneTextStep, error) {
	initial := SceneTextStep{Context: context, Active: active, Progress: progress, Uploads: uploads}
	if scene >= 8 {
		return initial, fmt.Errorf("unsupported text scene %d", scene)
	}
	r := initial
	r.ClearUploadControl = true // A496 clears0307..030A on every call.
	p := &r.Progress
	if r.Active&128 != 0 {
		switch {
		case r.Active&0x60 != 0:
			p.Pause--
			if p.Pause == 0 {
				r.Active &= 0x9f
			}
			return r, nil
		case r.Active&0x10 != 0:
			p.Address += 0x40
			if uint8(p.Address) == 0x83 {
				p.Address, p.LineAddress = 0x2303, 0x2303
				r.Active &= 0xef
			} else {
				clearOriginalSceneTextRows(&r)
			}
			return r, nil
		default:
			p.Clock--
			if p.Clock != 0 {
				return r, nil
			}
			if r.Active&0x0c != 0 {
				queueOriginalSceneTextGlyph(&r)
				return r, nil
			}
		}
	} else {
		if r.Context.TextRequestFlags&128 == 0 {
			return r, nil
		}
		r.Context.TextRequestFlags = 0
		p.Cursor = 0xffff
		p.Delay = [8]uint8{2, 2, 1, 2, 2, 2, 2, 2}[scene]
		r.Active |= 128
	}
	p.Clock = p.Delay
	id := r.Context.TextID & 127 // Both ASL carry branches use the same pointer table.
	if int(id) >= len(originalSceneTextStreams) {
		return initial, fmt.Errorf("unknown original text stream %d", id)
	}
	p.Cursor++
	stream := originalSceneTextStreams[id]
	if int(p.Cursor) >= len(stream) {
		return initial, fmt.Errorf("invalid original text cursor %d for stream %d", p.Cursor, id)
	}
	glyph := stream[p.Cursor]
	if glyph >= 0xf0 {
		if err := applyOriginalSceneTextControl(&r, glyph-0xf0); err != nil {
			return initial, err
		}
		return r, nil
	}
	mark := uint8(0)
	if glyph == 0 {
		p.Clock = 1
	}
	if glyph >= 0x30 {
		mark = originalTextGlyphMarks[glyph]
		if mark == 0x0c {
			glyph--
		} else if mark == 0x0d {
			glyph -= 2
		}
	}
	r.Uploads[0].Data[0], r.Uploads[1].Data[0] = mark, glyph
	queueOriginalSceneTextGlyph(&r)
	return r, nil
}

func queueOriginalSceneTextGlyph(s *SceneTextStep) {
	s.Uploads[0].Address, s.Uploads[1].Address = s.Progress.Address, s.Progress.Address+0x20
	s.Uploads[0].Length, s.Uploads[1].Length = 1, 1
	s.Progress.Address++
	s.QueueMask |= 3
}
func clearOriginalSceneTextRows(s *SceneTextStep) {
	for i := range s.Uploads {
		s.Uploads[i].Address = s.Progress.Address + uint16(i)*0x20
		s.Uploads[i].Length = 0x1b
		// A654 stores28 bytes, although the pending transfer length is27.
		for j := 0; j <= 0x1b; j++ {
			s.Uploads[i].Data[j] = 0
		}
	}
	s.QueueMask |= 3
}
func applyOriginalSceneTextControl(s *SceneTextStep, command uint8) error {
	switch command {
	case 0, 1:
		s.Active = 0x84 + command*4
		s.Progress.Repeat = 255
	case 2:
		s.Active |= 0x20
	case 3, 9:
		s.Progress.Pause = 0x40
		if command == 9 {
			s.Progress.Pause = 0x80
		}
		s.Active |= 0x40
	case 4:
		s.Active |= 0x10
		s.Progress.Address = 0x2303
		clearOriginalSceneTextRows(s)
	case 5:
		// FD0A sets the source scratch24 to0 before every command fetch.
		s.Active = 0
	case 6:
	case 7:
		s.Progress.LineAddress += 0x40
		s.Progress.Address = s.Progress.LineAddress
	case 8:
		s.Active &= 0xf3
	default:
		return fmt.Errorf("unknown original text command %d", command)
	}
	return nil
}
