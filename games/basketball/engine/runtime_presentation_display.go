package engine

import "fmt"

// PostResultRenderSnapshot contains committed semantic video memory. It has no
// ROM, CPU state, captured frame or pending resource/upload queue.
type PostResultRenderSnapshot struct {
	Surface OriginalRasterSurface   `json:"surface"`
	Plan    OriginalRasterFramePlan `json:"plan"`
	OAM     [256]uint8              `json:"oam"`
}

func seedRuntimePostResultSurface(scene OpeningState, display DisplayState) (OriginalRasterSurface, error) {
	court := scene.Transition.Court
	if court >= 8 {
		return OriginalRasterSurface{}, fmt.Errorf("invalid retained result court")
	}
	s := OriginalRasterSurface{Mirroring: 1}
	stream := func(column, phase uint8) {
		slot, page := int(column&31), int(column>>5)
		otherMask, currentMask := uint8(0xcc), uint8(0x33)
		if column&2 != 0 {
			otherMask, currentMask = 0x33, 0xcc
		}
		full := phase == 0 || column&32 == 0 && column&2 == 0 || column&32 != 0 && column&2 != 0
		for plane := 0; plane < 2; plane++ {
			for row := 0; row < 30; row++ {
				s.Nametables[plane][row*32+slot] = originalRuntimeCourtTiles[court][plane][row][column]
			}
			for row := 0; row < 8; row++ {
				at := row*8 + slot/4
				current := originalRuntimeCourtAttributes[court][plane][page][at]
				if !full {
					current = current&currentMask | originalRuntimeCourtAttributes[court][plane][page^1][at]&otherMask
				}
				s.Nametables[plane][960+at] = current
			}
		}
	}
	for column := uint8(16); column < 48; column++ {
		stream(column, 0)
	}
	for _, c := range display.CourtColumns {
		if c.Column >= 64 {
			return s, fmt.Errorf("invalid retained column")
		}
		stream(c.Column, c.Phase)
	}
	for _, p := range display.CourtPatches {
		if e := validateCourtPatch(p); e != nil {
			return s, e
		}
		for i, tile := range p.Tiles {
			s.Nametables[p.Plane][int(p.Row)*32+int(p.Column)+i] = uint8(tile)
		}
	}
	copy(s.Nametables[0][:192], originalRuntimeResultHUD[:])
	s.Nametables[1] = originalRuntimeResultBackground[court]
	for side, digits := range scene.Transition.Match.DisplayedScores {
		for column, digit := range digits {
			if digit >= 10 {
				return s, fmt.Errorf("invalid result score glyph")
			}
			for row := 0; row < 3; row++ {
				at := (row+1)*32 + 8 + side*10 + column*2
				s.Nametables[0][at] = originalRuntimeResultDigits[digit] + uint8(row*16)
				s.Nametables[0][at+1] = s.Nametables[0][at] + 1
			}
		}
	}
	for i := range s.Palette {
		s.Palette[i] = 15
	}
	return s, nil
}

func initializeRuntimePresentationDisplay(s *RuntimeState) error {
	p := s.Presentation
	if p == nil || p.Video == nil {
		return fmt.Errorf("presentation display has no scene")
	}
	if p.Display != nil {
		return nil
	}
	if p.Video.Clock.VideoTicks != 0 {
		return fmt.Errorf("presentation restore lost committed display")
	}
	surface, e := seedRuntimePostResultSurface(p.Video.Scene, s.Display)
	if e != nil {
		return e
	}
	v := p.Video
	d := OriginalPostResultDisplayState{Surface: surface, SpriteContext: OriginalPostResultSpriteContext{SurfaceCodes: s.Display.SurfaceCodes}, FieldCHR: v.FieldCHR, VisibleFieldCHR: v.FieldCHR, SpriteCHR: v.Context.Ending.CHRPages, Control: v.Control, Mask: v.Mask}
	// At this boundary the court fade is black. Unavailable inactive OAM bytes
	// are canonical zero; hidden Y248 prevents a stale fragment from appearing.
	for i := 0; i < 256; i += 4 {
		d.WorkingOAM[i] = 248
		d.VisibleOAM[i] = 248
	}
	p.Display = &d
	return nil
}

// buildRuntimePostResultPlan deliberately uses canonical scanline arrival, not
// source-observed CPU timing: NMI effects precede visible fetch, first IRQ starts
// at its latch scanline, rearmed IRQs follow reload completion plus latch+1 rows.
// Handler-relative source cycles remain 3 picture dots each. Mainline completes
// at the declared frame boundary before IRQ sampling; offsets are LIVE, while
// the NMI's scene/phase choose this frame's IRQ chain.
func buildRuntimePostResultPlan(nmi, live OriginalPostResultRasterSource, odd bool) (OriginalRasterFramePlan, error) {
	plan := OriginalRasterFramePlan{Source: nmi, OddFrame: odd, TimingMode: OriginalRasterCanonical, TimingSource: "runtime-v1: NMI before fetch; IRQ latch scanlines + source handler cycles; live post-dispatch offsets"}
	first, e := BuildOriginalPostResultRasterNMI(nmi)
	if e != nil {
		return plan, e
	}
	for _, q := range first.Effects {
		plan.Effects = append(plan.Effects, OriginalRasterTimedEffect{Dot: -2048 + int32(q.Cycle)*3, Kind: q.Kind, Value: q.Value})
	}
	origin := int32(*first.Latch) * 341
	stage, control := first.Stage, first.Control
	live.Scene, live.Phase = nmi.Scene, nmi.Phase
	for count := 0; count < 8; count++ {
		handler, e := BuildOriginalPostResultRasterIRQ(stage, control, live)
		if e != nil {
			return plan, e
		}
		for _, q := range handler.Effects {
			dot := origin + int32(q.Cycle)*3
			if dot <= 240*341 {
				plan.Effects = append(plan.Effects, OriginalRasterTimedEffect{Dot: dot, Kind: q.Kind, Value: q.Value})
			}
		}
		if handler.DisableCycle != nil {
			return plan, nil
		}
		if handler.Latch == nil || handler.ReloadCycle == nil {
			return plan, fmt.Errorf("unterminated presentation IRQ")
		}
		origin += int32(*handler.ReloadCycle)*3 + (int32(*handler.Latch)+1)*341
		stage, control = handler.Stage, handler.Control
	}
	return plan, fmt.Errorf("presentation IRQ chain exceeds source bound")
}

func RuntimePostResultRender(s RuntimeState) (*PostResultRenderSnapshot, error) {
	p := s.Presentation
	if p == nil || p.Video == nil || p.Display == nil {
		return nil, nil
	}
	d := p.Display
	live := postResultRaster(*p.Video)
	plan := OriginalRasterFramePlan{Source: live, OddFrame: s.VideoFrames&1 != 0, TimingMode: OriginalRasterCanonical, TimingSource: "runtime-v1: committed blanking before first presentation NMI"}
	plan.Source.BackgroundCHR = d.VisibleFieldCHR
	plan.Source.SpriteCHR = d.SpriteCHR
	plan.Source.Mask = 0
	black := true
	for _, color := range d.Surface.Palette {
		if color != 15 {
			black = false
			break
		}
	}
	if d.HasNMI && d.Mask&24 != 0 && !black {
		var e error
		plan, e = buildRuntimePostResultPlan(d.NMIRaster, live, s.VideoFrames&1 != 0)
		if e != nil {
			return nil, e
		}
		// A source resource commit can publish a CHR mapping after the NMI. The
		// declared canonical frame boundary samples that actual mapping explicitly.
		plan.Source.BackgroundCHR = d.VisibleFieldCHR
		plan.Source.SpriteCHR = d.SpriteCHR
	}
	return &PostResultRenderSnapshot{Surface: d.Surface, Plan: plan, OAM: d.VisibleOAM}, nil
}

func publishRuntimePresentationDisplay(s *RuntimeState) error {
	render, e := RuntimePostResultRender(*s)
	if e != nil {
		return e
	}
	s.Display.Visible.PostResult = render
	s.Display.Visible.Court = s.Config.Match.Court
	s.Display.HasVisible = true
	return nil
}
