package engine

import "fmt"

// BoardContactState is the shared state of bank 4's descending object contact
// phase. Action history changes once, at the ball's turn, not once per frame.
type BoardContactState struct {
	Players                                [4]Player
	Auxiliary                              [2]AuxiliaryState
	Ball                                   Ball
	Hoops                                  [6]Hoop
	Holder                                 int8
	Random                                 RandomState
	Scores                                 [2]uint16
	ScoreSoundDelay, DetachedCount         uint8
	PreviousBallAction, ObservedBallAction BallAction
	MountFlags                             [6]uint8
	SelectedBoard, DisplayFlags            uint8
	CourtToggle                            uint8
	CourtScoreFlags                        [2]uint8
}
type BoardContactContext struct {
	Rim                        RimContext
	CourtID, BackboardStrength uint8
	AuxiliaryKinds             [2]uint8
}
type BoardContactStep struct {
	State    BoardContactState
	SoundIDs []uint8
	Rim      RimResolution
}

func boardLow(v Fixed) uint8                 { return uint8(v >> 8) }
func boardReplaceLow(v Fixed, b uint8) Fixed { return v&^0xff00 | Fixed(b)<<8 }
func boardBody(s *BoardContactState, object uint8) *Body {
	if object < 4 {
		return &s.Players[object].Body
	}
	if object < 6 {
		return &s.Auxiliary[object-4].Body
	}
	if object == 6 {
		return &s.Ball.Body
	}
	return nil
}
func boardPosition(s *BoardContactState, object uint8) *Position {
	if object >= 7 {
		return &s.Hoops[object-7].Position
	}
	return &boardBody(s, object).Position
}

// boardBallRebound follows $936A (the fixed strength-3 half step) followed
// by $92BF. Each original velocity byte is negated independently before the
// half step; this differs from negating the 16-bit signed velocity.
func boardBallRebound(v Fixed, strength uint8, right bool) (Fixed, error) {
	if strength > 2 {
		return v, fmt.Errorf("backboard strength %d is not reconstructed", strength)
	}
	hi, lo := uint8(v>>8), uint8(v)
	negative := int8(hi) < 0
	if negative {
		hi, lo = -hi, -lo
	}
	half := (uint16(hi)<<8 | uint16(lo)) >> 1
	hi, lo = uint8(half>>8), uint8(half)
	if negative {
		hi, lo = -hi, -lo
	}
	// $92BF discards the fractional byte and scales the absolute integer.
	_ = lo
	negative = int8(hi) < 0
	if negative {
		hi = -hi
	}
	if hi == 0 {
		hi = 1
		negative = right
	}
	if strength == 2 {
		hi += hi >> 1
	}
	if negative {
		hi = -hi
	}
	return Fixed(int8(hi)) * Pixel, nil
}

func StepBoardContactObject(original BoardContactState, object uint8, ctx BoardContactContext) (BoardContactStep, error) {
	initial := BoardContactStep{State: original}
	result := initial
	s := &result.State
	if object >= 13 {
		return initial, fmt.Errorf("invalid board contact object %d", object)
	}
	if object == 6 {
		s.DetachedCount = 0
		for _, h := range s.Hoops {
			if h.Action == HoopInactive || h.Action >= HoopCarried {
				s.DetachedCount++
			}
		}
		if s.ScoreSoundDelay < 128 {
			s.ScoreSoundDelay--
			if s.ScoreSoundDelay == 0 {
				result.SoundIDs = append(result.SoundIDs, 0x32)
			}
		}
		if s.Ball.Action != s.ObservedBallAction {
			s.PreviousBallAction, s.ObservedBallAction = s.ObservedBallAction, s.Ball.Action
		}
		if s.Ball.Action == BallStuck {
			var value uint8
			s.Random, value = NextRandom(s.Random)
			if value&63 == 0 {
				if ctx.Rim.SelectedHoop < 0 || ctx.Rim.SelectedHoop >= 6 {
					return initial, fmt.Errorf("hanging ball has no selected hoop")
				}
				h := s.Hoops[ctx.Rim.SelectedHoop]
				b := &s.Ball
				b.BouncesRemaining = 1
				b.Body.Position.X = boardReplaceLow(b.Body.Position.X, boardLow(h.Position.X))
				b.Body.Position.Depth = boardReplaceLow(b.Body.Position.Depth, boardLow(h.Position.Depth)+2)
				b.Body.Velocity = Velocity{Height: 257}
				b.Body.Gravity = 0
			}
		}
	}
	if s.Holder != 6 {
		for i := range s.Hoops {
			s.Hoops[i].ScoreLock = 0
		}
	}
	p := boardPosition(s, object)
	depth, height := boardLow(p.Depth), boardLow(p.Height)
	maxDepth := uint8(56)
	lowA, highA, lowB, highB, lowC, highC := uint8(72), uint8(136), uint8(168), uint8(232), uint8(8), uint8(72)
	if object >= 7 {
		maxDepth = 47
		lowA, highA, lowB, highB, lowC, highC = 76, 88, 172, 184, 12, 24
	}
	candidate := false
	if depth >= 40 && depth < maxDepth {
		if uint8(p.Height>>16) == 1 {
			s.SelectedBoard = 4
			candidate = height >= lowC && height < highC
		} else if height >= lowA && height < highA {
			s.SelectedBoard = 0
			candidate = true
		} else if height >= lowB && height < highB {
			s.SelectedBoard = 2
			candidate = true
		}
	}
	hit := false
	if candidate {
		section := (depth & 16) >> 4
		x := boardLow(p.X)
		if uint8(p.X>>16) == 0 {
			edge := [2]uint8{60, 64}[section]
			if x < edge && x >= edge-12 {
				p.X = boardReplaceLow(p.X, edge+2)
				hit = true
			}
		} else {
			edge := [2]uint8{212, 208}[section]
			if x >= edge {
				s.SelectedBoard++
				if x < edge+12 {
					p.X = boardReplaceLow(p.X, edge-1)
					hit = true
				}
			}
		}
	}
	if hit {
		switch {
		case object < 6:
			boardBody(s, object).Velocity.X = 0
			return result, nil
		case object == 6:
			vx, err := boardBallRebound(s.Ball.Body.Velocity.X, ctx.BackboardStrength, uint8(p.X>>16) != 0)
			if err != nil {
				return initial, err
			}
			s.Ball.Body.Velocity.X = vx
			s.Ball.Deflection = DeflectX
			s.Ball.Request = BallRequest{Action: BallDeflected, Pending: true}
			if s.Holder == 6 {
				result.SoundIDs = append(result.SoundIDs, 3)
			}
		default:
			h := &s.Hoops[object-7]
			if (h.Action == HoopCarried || h.Action == HoopThrown || h.Action == HoopLobbed) && s.MountFlags[s.SelectedBoard] != 3 {
				h.Action, h.Entered, h.Airborne = HoopReady, false, false
				side := uint8(h.Position.X >> 16)
				if side > 1 {
					return initial, fmt.Errorf("hoop outside original board halves")
				}
				h.Position.X = boardReplaceLow(h.Position.X, [2]uint8{65, 207}[side])
				h.Position.Depth = boardReplaceLow(h.Position.Depth, 43)
				level := (s.SelectedBoard & 254) + (s.MountFlags[s.SelectedBoard] & 1)
				h.Position.Height = (h.Position.Height & 255) | Fixed([6]uint16{80, 112, 176, 208, 272, 304}[level])*Pixel
				if s.MountFlags[s.SelectedBoard] == 0 {
					s.MountFlags[s.SelectedBoard] = 1
				} else {
					s.MountFlags[s.SelectedBoard] = 3
				}
				h.Durability = [6]int8{8, 8, 6, 6, 5, 5}[s.SelectedBoard]
			}
		}
	}
	if object < 4 || object >= 7 {
		return result, nil
	}
	if object == 6 {
		rimCtx := ctx.Rim
		rimCtx.PreviousBallAction = s.PreviousBallAction
		rim, err := ResolveRimContact(RimState{Ball: s.Ball, Hoops: s.Hoops, Random: s.Random, Scores: s.Scores, ScoreSoundDelay: s.ScoreSoundDelay}, rimCtx)
		if err != nil {
			return initial, err
		}
		result.Rim = rim
		s.Ball, s.Hoops, s.Random, s.Scores, s.ScoreSoundDelay = rim.State.Ball, rim.State.Hoops, rim.State.Random, rim.State.Scores, rim.State.ScoreSoundDelay
		switch rim.Outcome {
		case RimUnderside:
			if s.Holder == 6 {
				result.SoundIDs = append(result.SoundIDs, 15)
			}
		case RimMiss, RimStuck:
			result.SoundIDs = append(result.SoundIDs, 15)
		case RimGoal:
			result.SoundIDs = append(result.SoundIDs, 33)
			if ctx.CourtID == 6 {
				s.CourtScoreFlags[(rim.ScoringSide+1)&1] = 1
			} else if ctx.CourtID == 4 {
				s.CourtToggle = (s.CourtToggle ^ 1) & 1
			}
		}
		if s.Hoops[0].Action|s.Hoops[1].Action|s.Hoops[2].Action|s.Hoops[3].Action == HoopScored {
			s.DisplayFlags &= 127
		}
		return result, nil
	}
	// Auxiliary objects use the same geometry, ignoring ball-only score locks
	// and receive-owner checks. They never score or use distance probability.
	aux := &s.Auxiliary[object-4]
	for i, h := range s.Hoops {
		geometry := h
		geometry.ScoreLock = 0
		geometry.CanReceive = true
		if !rimContains(aux.Body.Position, geometry) {
			continue
		}
		if aux.Body.Velocity.Height >= 0 {
			hi, lo := uint8(aux.Body.Velocity.Height>>8), uint8(aux.Body.Velocity.Height)
			aux.Body.Velocity.Height = Fixed(int16(uint16(-hi)<<8 | uint16(lo)))
			if aux.Body.Velocity.Height != 0 {
				result.SoundIDs = append(result.SoundIDs, 15)
			}
			return result, nil
		}
		shooter := ctx.Rim.ShooterAction
		accepted := shooter >= 0x58 && shooter <= 0x5c
		if !accepted {
			var value uint8
			s.Random, value = NextRandom(s.Random)
			accepted = value&7 < ctx.Rim.ShotSkill
		}
		if accepted && ctx.AuxiliaryKinds[object-4] == 11 && aux.Action == 2 {
			aux.Owner = int8(i + 7)
			aux.Body.Velocity = Velocity{}
			aux.Body.Gravity = 0
			aux.Action, aux.Entered = 0, false
			aux.Body.Position.X = boardReplaceLow(aux.Body.Position.X, boardLow(h.Position.X))
			aux.Body.Position.Depth = boardReplaceLow(aux.Body.Position.Depth, boardLow(h.Position.Depth)+4)
			aux.Body.Position.Height = boardReplaceLow(aux.Body.Position.Height, boardLow(h.Position.Height)+1)
		}
		return result, nil
	}
	return result, nil
}

// StepBoardContacts preserves the original descending 12..0 order and commits
// only a complete phase; an unsupported branch cannot leave partial scores.
func StepBoardContacts(original BoardContactState, ctx BoardContactContext) (BoardContactStep, error) {
	result := BoardContactStep{State: original}
	for object := 12; object >= 0; object-- {
		step, err := StepBoardContactObject(result.State, uint8(object), ctx)
		if err != nil {
			return BoardContactStep{State: original}, fmt.Errorf("board object %d: %w", object, err)
		}
		result.State = step.State
		result.SoundIDs = append(result.SoundIDs, step.SoundIDs...)
		if object == 6 {
			result.Rim = step.Rim
		}
	}
	return result, nil
}
