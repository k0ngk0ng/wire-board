package engine

import "fmt"

func beginPostResultBackground(r *PostResultVideoStep, preset uint8, resume PostResultPhaseContinuation) error {
	if _, e := OriginalPresentationBackground(preset); e != nil {
		return e
	}
	s := &r.State
	s.Context.Ending.Background = preset
	s.Clock.Resume = resume
	s.Resource = PostResultVideoResource{Kind: "background", Stage: "wait-entry", ID: preset, Counter: s.Clock.Counter}
	postResultSound(r, 253)
	return nil
}
func beginPostResultFade(r *PostResultVideoStep, mode, delay uint8, resume PostResultPhaseContinuation) error {
	s := &r.State
	s.Context.Ending.FadeMode, s.Context.Ending.FadeDelay = mode, delay
	s.Clock.Resume = resume
	s.Resource = PostResultVideoResource{Kind: "fade", Stage: "palette", Flags: mode, Delay: delay}
	s.PaletteBuildLevel = 16
	if mode&128 != 0 {
		s.PaletteBuildLevel = 48
	}
	if delay == 0 {
		s.PaletteBuildLevel = 64
	}
	return queuePostResultPalette(r)
}
func queuePostResultPalette(r *PostResultVideoStep) error {
	s := &r.State
	c := &s.Context.Ending
	u, e := BuildOriginalPresentationPalette(s.TailUploads[1], c.Actor.DisplayMask, c.Actor.PaletteIDs, s.Resource.Flags, s.PaletteBuildLevel)
	if e != nil {
		return e
	}
	s.TailUploads[1] = u
	s.Context.UploadControl[3] = 0
	s.Resource.Stage = "palette"
	r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "upload-queued", Slot: 3, Upload: u})
	return nil
}
func beginPostResultMessage(r *PostResultVideoStep, id uint8, resume PostResultPhaseContinuation) error {
	s := &r.State
	t := &s.Scene.Transition
	t.Match.OvertimeFlags = id
	q, e := PrepareOriginalPostResultMessage(t.Scene, t.Match.Phase, id, s.TailUploads)
	if e != nil {
		return e
	}
	s.TailUploads = q.Uploads
	s.Clock.Resume = resume
	s.Resource = PostResultVideoResource{Kind: "message", Stage: "counter", ID: id, Counter: s.Clock.Counter}
	for i, u := range q.Uploads {
		r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "upload-queued", Slot: uint8(i + 2), Upload: u})
	}
	return nil
}
func postResultBlanking(s *PostResultVideoState, stage PostResultResourceStage) {
	s.Resource.Stage = stage
	s.Resource.SavedControl = s.Context.Ending.MachineControl
	s.Context.Ending.MachineControl = 0
	s.Control |= 128
}
func consumePostResultBackgroundWork(r *PostResultVideoStep) error {
	s := &r.State
	// Costs are generated from source decode loops, not from movie durations.
	// The nine supported assets are far from the omitted sub-frame overhead edge.
	const budget uint32 = 29780
	if s.Resource.RemainingCycles > budget {
		s.Resource.RemainingCycles -= budget
		return nil
	}
	s.Resource.RemainingCycles = 0
	b, e := OriginalPresentationBackground(s.Resource.ID)
	if e != nil {
		return e
	}
	s.FieldCHR = b.FieldCHR
	s.Context.Ending.Actor.DisplayMask = b.PaletteID
	s.Mask |= 0x18
	r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "background-commit", Background: b}, PostResultVideoEffect{Kind: "render-mask", Value: s.Mask})
	postResultBlanking(s, "blank-exit")
	return nil
}
func advancePostResultResource(r *PostResultVideoStep) error {
	s := &r.State
	q := &s.Resource
	switch q.Kind {
	case "message":
		if s.Clock.Counter != q.Counter {
			s.Resource = PostResultVideoResource{}
		}
	case "fade":
		switch q.Stage {
		case "palette":
			if s.TailUploads[1].Address&0x8000 != 0 {
				s.Clock.Counter = 0
				q.Stage = "delay"
				if q.Delay == 0 {
					s.Resource = PostResultVideoResource{}
				}
			}
		case "delay":
			if s.Clock.Counter >= q.Delay {
				if q.Flags&128 != 0 {
					s.PaletteBuildLevel -= 16
				} else {
					s.PaletteBuildLevel += 16
				}
				q.Level++
				if q.Level == 4 {
					s.Resource = PostResultVideoResource{}
				} else {
					return queuePostResultPalette(r)
				}
			}
		default:
			return fmt.Errorf("invalid palette continuation")
		}
	case "background":
		switch q.Stage {
		case "wait-entry":
			if s.Clock.Counter != q.Counter {
				postResultBlanking(s, "blank-enter")
			}
		case "blank-enter":
			b, e := OriginalPresentationBackground(q.ID)
			if e != nil {
				return e
			}
			s.FieldCHR = [2]uint8{b.CompressedCHR, b.CompressedCHR + 2}
			s.Mask &= 0xe7
			s.Control &= 0xfb
			q.Stage = "work"
			q.RemainingCycles = b.WorkCycles
			r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "render-mask", Value: s.Mask}, PostResultVideoEffect{Kind: "background-chr", FieldCHR: s.FieldCHR})
			return consumePostResultBackgroundWork(r)
		case "work":
			return consumePostResultBackgroundWork(r)
		case "blank-exit":
			s.Control |= 128
			q.Stage = "wait-exit"
			q.Counter = s.Clock.Counter
			postResultSound(r, 255)
		case "wait-exit":
			if s.Clock.Counter != q.Counter {
				s.Resource = PostResultVideoResource{}
			}
		default:
			return fmt.Errorf("invalid background continuation")
		}
	default:
		return fmt.Errorf("unknown postresult resource %q", q.Kind)
	}
	return nil
}
