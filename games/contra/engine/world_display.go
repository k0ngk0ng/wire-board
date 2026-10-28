package contra

import "errors"

// DisplayState separates the requested mask from the currently visible mask.
// Busy is the original display countdown, not a pause of the game simulation.
// Raster fields describe scene split effects; they never replace shadow scroll
// or control state used by subsequent gameplay.
type DisplayState struct {
	SplitActive, RasterActive              bool
	Busy, Mask, AppliedMask                uint8
	SplitScroll, RasterMask, RasterControl uint8
}

// ApplyImmediate applies normalized tile/color data. Scene background copies
// use it immediately; queued commands use it at the next display boundary.
// There is no CPU memory/ROM execution here. Only nametables and colors exist.
func (p *WorldPresentation) ApplyImmediate(writes []BackgroundWrite) error {
	if p == nil {
		return errors.New("missing presentation")
	}
	next := *p
	for _, write := range writes {
		if write.Step != 1 && write.Step != 32 {
			return errors.New("invalid presentation write stride")
		}
		for i, v := range write.Values {
			address := (uint32(write.Address) + uint32(i)*uint32(write.Step)) & 0x3fff
			if address < 0x2000 {
				return errors.New("presentation data cannot overwrite pattern memory")
			}
			if address < 0x3f00 {
				next.Background[address&0x7ff] = v
			} else {
				index := address & 31
				if index >= 16 && index%4 == 0 {
					index -= 16
				}
				next.Palette.Applied[index] = v & 63
				if index < 16 && index%4 == 0 {
					next.Palette.Applied[index+16] = v & 63
				}
			}
		}
	}
	*p = next
	return nil
}

func (w *World) applyScene(result CampaignSceneResult) error {
	s := &w.Active
	if err := s.Presentation.ApplyImmediate(result.ImmediateBackground); err != nil {
		return err
	}
	if len(result.ImmediateBackground) > 0 {
		s.Presentation.DiscardedGraphicsBytes = 0
	} // C93A resets the native cursor.
	for _, control := range result.Controls {
		switch control.Kind {
		case "graphics-mode":
			s.Flicker = control.Value&1 != 0
		case "queue-mode":
			mode := GraphicsRectangular
			if control.Value == 0 {
				mode = GraphicsText
			}
			if s.Presentation.Queue.Mode != mode {
				return errors.New("scene parser effect does not match its queue")
			}
		case "ppu-control":
			s.Combat.World.Scroll.NameTable = control.Value
			s.Presentation.AppliedScroll.NameTable = control.Value
		case "display-mask":
			s.Presentation.Display.Mask = control.Value
		case "display-disable":
			s.Presentation.Display.AppliedMask = control.Value
		case "display-busy":
			s.Presentation.Display.Busy = control.Value
		case "split-scroll":
			s.Presentation.Display.SplitActive = true
			s.Presentation.Display.SplitScroll = control.Value
		case "raster-mask":
			s.Presentation.Display.RasterActive = true
			s.Presentation.Display.RasterMask = control.Value
		case "raster-control":
			s.Presentation.Display.RasterActive = true
			s.Presentation.Display.RasterControl = control.Value
		default:
			return errors.New("unknown scene display effect")
		}
	}
	return nil
}
