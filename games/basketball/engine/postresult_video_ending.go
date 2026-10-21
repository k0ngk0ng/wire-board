package engine

import "fmt"

// Nonblocking ending branches share the already source-verified phase helper.
// Blocking branches below commit only the writes preceding their next source
// call and retain a typed phase-relative continuation across video boundaries.
func finishImmediatePostResultEnding(r *PostResultVideoStep) error {
	s := &r.State
	p, e := StepOriginalEndingPhase(s.Scene, s.Context.Ending, s.Clock.Input[0].Pressed)
	if e != nil {
		return e
	}
	for _, q := range p.Requests {
		if q.Kind != EndingSound {
			return fmt.Errorf("blocking ending branch passed to immediate adapter")
		}
		postResultSound(r, q.Value)
	}
	s.Scene, s.Context.Ending = p.State, p.Context
	if p.Reset {
		s.Clock.Stage = "reset"
		r.Reset = true
		r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "reset"})
	} else {
		s.Clock.Stage = "actors"
	}
	return nil
}
func advancePostResultEnding(r *PostResultVideoStep) error {
	s := &r.State
	t := &s.Scene.Transition
	c := &s.Context.Ending
	next := func(phase MatchPhase) { t.Match.Phase, t.Match.Entered = phase, false }
	actors := func() { s.Clock.Stage = "actors" }
	chr := func() { r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "sprite-chr", CHR: c.CHRPages}) }
	switch s.Clock.OriginPhase {
	case 0:
		if s.Clock.OriginEntered {
			switch s.Clock.Resume {
			case 0:
				return beginPostResultFade(r, 0, 1, 1)
			case 1:
				next(1)
				postResultSound(r, 75)
				actors()
			default:
				return fmt.Errorf("ending0 transition continuation")
			}
			return nil
		}
		switch s.Clock.Resume {
		case 0:
			t.Match.Entered = true
			c.MachineControl = 128
			endingSetTimer(&s.Scene, 0)
			t.CameraScroll[2], t.CameraScroll[3] = 0, 0
			r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "mirroring", Value: 0})
			if t.Match.Period != 28 {
				next(11)
				actors()
				return nil
			}
			return beginPostResultBackground(r, 95, 1)
		case 1:
			return beginPostResultBackground(r, 96, 2)
		case 2:
			c.CHRPages = [4]uint8{0x25, 0x26, 0x0b, 0x28}
			chr()
			for i := 0; i < 4; i++ {
				t.Kinds[i] = 18
				t.Objects[i].Pose = Pose{Frame: 127, Mirrored: true}
				periodSetAction(&t.Objects[i], uint8(0x51+i))
			}
			t.Kinds[0] = 19
			c.Actor.PaletteIDs[0] = 49
			t.EndActorFlags = [4]uint8{}
			t.SelectedController = 0
			t.ResultControllers = [2]int8{}
			t.LastShooter = 0
			t.Objects[0].Body.Velocity.X, t.Objects[0].Body.Velocity.Depth = 0, 0
			t.Passages[2] = 0
			actors()
		default:
			return fmt.Errorf("ending0 entry continuation")
		}
	case 1, 7, 10:
		return finishImmediatePostResultEnding(r)
	case 2:
		if !s.Clock.OriginEntered {
			return finishImmediatePostResultEnding(r)
		}
		switch s.Clock.Resume {
		case 0:
			return beginPostResultFade(r, 128, 1, 1)
		case 1:
			t.EndActorFlags = [4]uint8{}
			for i := 0; i < 4; i++ {
				t.Objects[i].Pose = Pose{Frame: 127, Mirrored: i < 2}
				t.Kinds[i] = 19
			}
			c.Actor.TextRequestFlags, c.TextActive = 0, 0
			for i, a := range [4]uint8{0x55, 0x56, 0x57, 0} {
				periodSetAction(&t.Objects[i], a)
			}
			t.Kinds[2] = 18
			next(3)
			actors()
		default:
			return fmt.Errorf("ending2 continuation")
		}
	case 3:
		if s.Clock.OriginEntered {
			return finishImmediatePostResultEnding(r)
		}
		switch s.Clock.Resume {
		case 0:
			t.Match.Entered = true
			t.Passages[2] = 0
			endingSetTimer(&s.Scene, 0)
			return beginPostResultBackground(r, 97, 1)
		case 1:
			c.Actor.PaletteIDs = [2]uint8{50, 51}
			return beginPostResultFade(r, 0, 1, 2)
		case 2:
			actors()
		default:
			return fmt.Errorf("ending3 continuation")
		}
	case 4:
		if !s.Clock.OriginEntered {
			return finishImmediatePostResultEnding(r)
		}
		switch s.Clock.Resume {
		case 0:
			return beginPostResultFade(r, 128, 1, 1)
		case 1:
			for i := 0; i < 4; i++ {
				periodSetAction(&t.Objects[i], 0)
				t.EndActorFlags[i] = 0
				t.Objects[i].Body.Velocity.X = 0
				t.Kinds[i] = 17
				t.Objects[i].Pose = Pose{Frame: 127, Mirrored: i == 0}
			}
			c.Actor.TextRequestFlags, c.TextActive = 0, 0
			for i, a := range [3]uint8{0x5c, 0x5d, 0x5e} {
				periodSetAction(&t.Objects[i], a)
			}
			return beginPostResultBackground(r, 33, 2)
		case 2:
			return beginPostResultBackground(r, 34, 3)
		case 3:
			c.Upload = EndingUpload{Address: 0x2420, Length: 32}
			r.Effects = append(r.Effects, PostResultVideoEffect{Kind: "upload-queued", Slot: 0, Upload: c.Upload})
			next(5)
			actors()
		default:
			return fmt.Errorf("ending4 continuation")
		}
	case 5:
		if s.Clock.OriginEntered {
			return finishImmediatePostResultEnding(r)
		}
		switch s.Clock.Resume {
		case 0:
			t.Match.Entered = true
			c.Actor.PaletteIDs = [2]uint8{13, 14}
			endingSetTimer(&s.Scene, 0)
			return beginPostResultFade(r, 0, 1, 1)
		case 1:
			actors()
		default:
			return fmt.Errorf("ending5 continuation")
		}
	case 6:
		switch s.Clock.Resume {
		case 0:
			return beginPostResultFade(r, 128, 1, 1)
		case 1:
			for i := 0; i < 4; i++ {
				t.EndActorFlags[i] = 0
				periodSetAction(&t.Objects[i], 0)
			}
			c.CHRPages[0], c.CHRPages[1] = 0x38, 0x39
			chr()
			t.Kinds[0] = 18
			t.Objects[0].Pose = Pose{Frame: 127, Mirrored: true}
			periodSetAction(&t.Objects[0], 0x5f)
			c.Actor.PaletteIDs = [2]uint8{29, 30}
			next(8)
			actors()
		default:
			return fmt.Errorf("ending6 continuation")
		}
	case 8:
		if s.Clock.OriginEntered {
			switch s.Clock.Resume {
			case 0:
				t.Passages[3] -= 2
				t.Passages[4]--
				pair := uint16(t.Passages[7]) | uint16(t.Passages[5])<<8
				pair -= 128
				t.Passages[7], t.Passages[5] = uint8(pair), uint8(pair>>8)
				t.Passages[6]--
				timer := EndingTimer(s.Scene) + 1
				endingSetTimer(&s.Scene, timer)
				if timer == 0x10c0 {
					return beginPostResultFade(r, 128, 2, 1)
				}
				actors()
			case 1:
				periodSetAction(&t.Objects[0], 0)
				next(9)
				actors()
			default:
				return fmt.Errorf("ending8 timer continuation")
			}
			return nil
		}
		switch s.Clock.Resume {
		case 0:
			t.Match.Entered = true
			c.TextActive = 0
			for i := 3; i < 8; i++ {
				t.Passages[i] = 0
			}
			endingSetTimer(&s.Scene, 0)
			return beginPostResultBackground(r, 25, 1)
		case 1:
			return beginPostResultBackground(r, 98, 2)
		case 2:
			return beginPostResultFade(r, 0, 1, 3)
		case 3:
			postResultSound(r, 76)
			c.Actor.TextID, c.Actor.TextRequestFlags = 36, 128
			actors()
		default:
			return fmt.Errorf("ending8 entry continuation")
		}
	case 9:
		if s.Clock.OriginEntered {
			if s.Clock.Resume == 0 {
				if s.Clock.Input[0].Pressed&(ButtonA|ButtonStart) == 0 {
					return finishImmediatePostResultEnding(r)
				}
				return beginPostResultFade(r, 128, 1, 1)
			}
			if s.Clock.Resume != 1 {
				return fmt.Errorf("ending9 confirm continuation")
			}
			next(10)
			actors()
			return nil
		}
		switch s.Clock.Resume {
		case 0:
			t.Match.Entered = true
			c.Actor.TextRequestFlags, c.TextActive = 0, 0
			return beginPostResultBackground(r, 23, 1)
		case 1:
			return beginPostResultFade(r, 0, 2, 2)
		case 2:
			actors()
		default:
			return fmt.Errorf("ending9 entry continuation")
		}
	default:
		return fmt.Errorf("unknown ending video phase%d", s.Clock.OriginPhase)
	}
	return nil
}
