package engine

import "fmt"

// EndingUpload retains the original pending $030B..032D buffer. It is a
// request for the rendering layer, not proof that an NMI upload has completed.
type EndingUpload struct {
	Address uint16    `json:"address"`
	Length  uint8     `json:"length"`
	Data    [32]uint8 `json:"data"`
}

// EndingPhaseContext holds only sources absent from OpeningState. The original
// $06D7..06DF aliases remain authoritative in Transition.Passages; use the named
// accessors below rather than storing another timer or credits copy.
type EndingPhaseContext struct {
	Actor          ResultActorContext `json:"actor"`
	TextActive     uint8              `json:"textActive"`
	MachineControl uint8              `json:"machineControl"`
	Background     uint8              `json:"background"`
	FadeMode       uint8              `json:"fadeMode"`
	FadeDelay      uint8              `json:"fadeDelay"`
	CHRPages       [4]uint8           `json:"chrPages"`
	Upload         EndingUpload       `json:"upload"`
}
type EndingRequestKind string

const (
	EndingBackground EndingRequestKind = "background"
	EndingCHR        EndingRequestKind = "chr"
	EndingFade       EndingRequestKind = "fade"
	EndingSound      EndingRequestKind = "sound"
	EndingMirroring  EndingRequestKind = "mirroring"
)

// EndingPhaseRequest preserves ordering and the register values at the original
// external call. It deliberately makes no claim about pixels or wait duration.
type EndingPhaseRequest struct {
	Kind     EndingRequestKind `json:"kind"`
	Value    uint8             `json:"value,omitempty"`
	Delay    uint8             `json:"delay,omitempty"`
	CHRPages [4]uint8          `json:"chrPages,omitempty"`
}
type EndingPhaseStep struct {
	State    OpeningState
	Context  EndingPhaseContext
	Requests []EndingPhaseRequest
	// Reset denotes the original jump to F79D, before executing machine reset.
	// Every successful non-reset result still requires the BF35 four-actor tail.
	Reset bool
}

func EndingTimer(s OpeningState) uint16 {
	return uint16(s.Transition.Passages[0]) | uint16(s.Transition.Passages[1])<<8
}
func EndingCredits(s OpeningState) [5]uint8 { return [5]uint8(s.Transition.Passages[3:8]) }
func endingSetTimer(s *OpeningState, v uint16) {
	s.Transition.Passages[0], s.Transition.Passages[1] = uint8(v), uint8(v>>8)
}

// StepOriginalEndingPhase executes bank3 B995 through BF35, excluding actors,
// or through the jump to F79D, excluding reset. The caller owns the Scene7
// video scheduler and preceding FD0A text update. No outer/internal RNG draw is
// introduced here, and palette/background/audio calls remain typed requests.
func StepOriginalEndingPhase(original OpeningState, context EndingPhaseContext, pressed Buttons) (EndingPhaseStep, error) {
	initial := EndingPhaseStep{State: original, Context: context}
	if original.Transition.Scene != 7 || original.Transition.Match.Phase > 10 {
		return initial, fmt.Errorf("ending phase requires scene7 phase0..10")
	}
	r := initial
	s := &r.State
	t := &s.Transition
	c := &r.Context
	next := func(phase MatchPhase) { t.Match.Phase, t.Match.Entered = phase, false }
	background := func(id uint8) {
		c.Background = id
		r.Requests = append(r.Requests, EndingPhaseRequest{Kind: EndingBackground, Value: id})
	}
	fade := func(mode, delay uint8) {
		c.FadeMode, c.FadeDelay = mode, delay
		r.Requests = append(r.Requests, EndingPhaseRequest{Kind: EndingFade, Value: mode, Delay: delay})
	}
	sound := func(id uint8) { r.Requests = append(r.Requests, EndingPhaseRequest{Kind: EndingSound, Value: id}) }
	chr := func() { r.Requests = append(r.Requests, EndingPhaseRequest{Kind: EndingCHR, CHRPages: c.CHRPages}) }
	increment := func() uint16 { v := EndingTimer(*s) + 1; endingSetTimer(s, v); return v }
	switch t.Match.Phase {
	case 0:
		if !t.Match.Entered {
			t.Match.Entered = true
			c.MachineControl = 128
			r.Requests = append(r.Requests, EndingPhaseRequest{Kind: EndingMirroring, Value: 0})
			endingSetTimer(s, 0)
			t.CameraScroll[2], t.CameraScroll[3] = 0, 0
			if t.Match.Period != 28 {
				next(11)
				break
			}
			background(95)
			background(96)
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
		} else {
			fade(0, 1)
			next(1)
			sound(75)
		}
	case 1:
		if !t.Match.Entered {
			t.Match.Entered = true
			c.Actor.TextID, c.Actor.TextRequestFlags = 1, 128
		} else {
			if increment() == 0x370 {
				next(2)
			} else {
				t.Passages[2] = (t.Passages[0] >> 3) & 1
			}
		}
	case 2:
		if !t.Match.Entered {
			t.Match.Entered = true
		} else {
			fade(128, 1)
			t.EndActorFlags = [4]uint8{}
			for i := 0; i < 4; i++ {
				t.Objects[i].Pose = Pose{Frame: 127, Mirrored: i < 2}
				t.Kinds[i] = 19
			}
			c.Actor.TextRequestFlags, c.TextActive = 0, 0
			for i, action := range [4]uint8{0x55, 0x56, 0x57, 0} {
				periodSetAction(&t.Objects[i], action)
			}
			t.Kinds[2] = 18
			next(3)
		}
	case 3:
		if !t.Match.Entered {
			t.Match.Entered = true
			t.Passages[2] = 0
			endingSetTimer(s, 0)
			background(97)
			c.Actor.PaletteIDs = [2]uint8{50, 51}
			fade(0, 1)
		} else if increment() == 0xe08 {
			next(4)
		}
	case 4:
		if !t.Match.Entered {
			t.Match.Entered = true
		} else {
			fade(128, 1)
			for i := 0; i < 4; i++ {
				periodSetAction(&t.Objects[i], 0)
				t.EndActorFlags[i] = 0
				t.Objects[i].Body.Velocity.X = 0
				t.Kinds[i] = 17
				t.Objects[i].Pose = Pose{Frame: 127, Mirrored: i == 0}
			}
			c.Actor.TextRequestFlags, c.TextActive = 0, 0
			for i, action := range [3]uint8{0x5c, 0x5d, 0x5e} {
				periodSetAction(&t.Objects[i], action)
			}
			background(33)
			background(34)
			c.Upload = EndingUpload{Address: 0x2420, Length: 32}
			next(5)
		}
	case 5:
		if !t.Match.Entered {
			t.Match.Entered = true
			c.Actor.PaletteIDs = [2]uint8{13, 14}
			endingSetTimer(s, 0)
			fade(0, 1)
		} else {
			timer := increment()
			o := &t.Objects[2]
			if o.Action != 0x5e && uint8(timer) == 0 {
				actions := [16]uint8{12, 12, 12, 12, 13, 14, 15, 16, 16, 16, 16, 16, 16, 16, 16, 16}
				if timer>>8 >= uint16(len(actions)) {
					return initial, fmt.Errorf("ending phase5 table index %d is outside recovered source data", timer>>8)
				}
				periodSetAction(o, actions[timer>>8])
				o.Animation.Cursor, o.Animation.Elapsed = 0, 0
			}
			if o.Action != 0x5e {
				shifts := [12]uint8{1, 2, 3, 4, 5, 6, 6, 6, 6, 6, 6, 6}
				if o.Action < 12 || int(o.Action-12) >= len(shifts) {
					return initial, fmt.Errorf("ending phase5 actor2 action %02x is outside recovered offset table", o.Action)
				}
				t.Passages[2] -= shifts[o.Action-12]
			}
			if timer == 0xf80 {
				next(6)
			}
		}
	case 6:
		fade(128, 1)
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
	case 7:
		if !t.Match.Entered {
			t.Match.Entered = true
		} else {
			for i := 0; i < 4; i++ {
				t.Objects[i].Body.Velocity.X, t.Objects[i].Body.Velocity.Depth = 0, 0
			}
			t.Match.Period = 0
			sound(0)
			c.MachineControl = 0xc0
			r.Reset = true
		}
	case 8:
		if !t.Match.Entered {
			t.Match.Entered = true
			c.TextActive = 0
			for i := 3; i < 8; i++ {
				t.Passages[i] = 0
			}
			endingSetTimer(s, 0)
			background(25)
			background(98)
			fade(0, 1)
			sound(76)
			c.Actor.TextID, c.Actor.TextRequestFlags = 36, 128
		} else {
			t.Passages[3] -= 2
			t.Passages[4]--
			pair := uint16(t.Passages[7]) | uint16(t.Passages[5])<<8
			pair -= 128
			t.Passages[7], t.Passages[5] = uint8(pair), uint8(pair>>8)
			t.Passages[6]--
			if increment() == 0x10c0 {
				fade(128, 2)
				periodSetAction(&t.Objects[0], 0)
				next(9)
			}
		}
	case 9:
		if !t.Match.Entered {
			t.Match.Entered = true
			c.Actor.TextRequestFlags, c.TextActive = 0, 0
			background(23)
			fade(0, 2)
		} else if pressed&(ButtonA|ButtonStart) != 0 {
			fade(128, 1)
			next(10)
		}
	case 10:
		if !t.Match.Entered {
			t.Match.Entered = true
		} else {
			next(7)
		}
	}
	return r, nil
}
