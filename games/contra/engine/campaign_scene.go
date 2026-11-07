package contra

import "errors"

// CampaignSceneState owns only dedicated exit-script counters ($0190..0193).
// Routine and sequence remain in World/Campaign; graphics and colors remain in
// ActiveWorld.Presentation. Ordinary player physics follows this script step.
type CampaignSceneState struct {
	Interlude             CampaignInterludeState
	ExitDelay, ExitBudget uint8
	IntroScroll, Reveal   uint8
	ExitFadeSteps         uint8
	ExitActors            [2]uint8
}

// StepEndSequence is the original $888B exit phase. It replaces both
// controller samples with script actions before the caller runs real Combat.
// This method does not advance players, enemies, palette cycles or the frame.
func (s *CampaignSceneState) StepEndSequence(c *Campaign, w *ActiveWorld, inputs *[2]PlayerInput) error {
	if s == nil || c == nil || w == nil || inputs == nil || w.Combat.World.GameRoutine != 5 || w.Combat.World.LevelRoutine != 9 || w.Combat.World.Level >= 8 || c.EndSequence > 3 {
		return errors.New("unsupported campaign exit context")
	}
	*inputs = [2]PlayerInput{}
	world := &w.Combat.World
	switch c.EndSequence {
	case 0:
		airborne := uint8(0)
		for i := 1; i >= 0; i-- {
			s.ExitActors[i] = 0
			p := &w.Combat.Players[i]
			if p.GameOver == 0 {
				airborne |= p.Motion.Jump
				s.ExitActors[i] = 1
			}
		}
		if airborne == 0 {
			world.BossDefeated = 0x81
			s.ExitBudget = 0xf0
			s.ExitDelay = 0x20
			c.EndSequence++
		}
	case 1:
		if s.ExitDelay != 0 {
			s.ExitDelay--
			return nil
		}
		for i := 1; i >= 0; i-- {
			if s.ExitActors[i] == 0 {
				continue
			}
			p := &w.Combat.Players[i]
			actorPhase := s.ExitActors[i] - 1
			switch world.Level {
			case 0:
				p.SkipLedge = 0
				if p.X >= 0x98 {
					p.SkipLedge = 0x80
				}
				switch actorPhase {
				case 0:
					inputs[i].Held = ButtonRight
					if p.X >= 0x90 {
						s.ExitActors[i]++
					}
				case 1:
					if p.Motion.Jump != 0 {
						s.ExitActors[i]++
					} else {
						inputs[i] = PlayerInput{Held: ButtonRight | ButtonA, Pressed: ButtonRight | ButtonA}
					}
				default:
					inputs[i].Held = ButtonRight
				}
			case 1, 3:
				switch actorPhase {
				case 0:
					inputs[i].Held = [2]Buttons{ButtonLeft, ButtonRight}[i]
					target := [2]uint8{12, 244}[i]
					distance := p.X - target
					if p.X < target {
						distance = 0 - distance
					}
					if distance < 2 {
						s.ExitActors[i]++
					}
				case 1:
					p.State = 3
					p.DisplayAttribute = [2]uint8{0, 69}[i]
					w.Combat.Display[i].Attributes = p.DisplayAttribute
					p.DisplaySprite = 0x91
					w.Combat.Display[i].Code = 0x91
					s.ExitActors[i]++
				case 2:
					peer := s.ExitActors[i^1]
					if peer == 0 || peer >= 3 {
						s.ExitActors[i]++
					}
				default:
					p.setY(p.y() - 1)
					w.Combat.Display[i].Y = p.y()
					p.DisplaySprite = 0x91
					w.Combat.Display[i].Code = 0x91
				}
			case 2:
				if actorPhase == 0 {
					inputs[i].Held = ButtonRight
					if p.X >= 128 {
						inputs[i].Held = ButtonLeft
					}
					distance := p.X - 128
					if p.X < 128 {
						distance = 0 - distance
					}
					if distance < 8 {
						s.ExitActors[i]++
					}
				} else {
					inputs[i].Pressed = ButtonA
					if p.Motion.Jump != 0 && p.Motion.Velocity >= 0 && p.y() >= 176 {
						s.hideExitActor(w, i)
					}
				}
			case 4, 5, 6:
				inputs[i].Held = ButtonRight
				p.SkipLedge = 1
				if p.X >= [3]uint8{184, 208, 208}[world.Level-4] {
					p.SkipLedge |= 0x80
				}
			case 7:
				if actorPhase == 0 {
					s.ExitBudget = 0x90
					s.ExitActors[i]++
				}
			}

			if p.y() < 8 || p.X >= 0xf8 || p.X < 4 {
				s.hideExitActor(w, i)
			}
		}
		expired := false
		if world.Frame&1 != 0 {
			s.ExitBudget--
			expired = s.ExitBudget == 0
		}
		if expired || s.ExitActors[0]|s.ExitActors[1] == 0 {
			s.ExitDelay = [8]uint8{160, 160, 224, 160, 160, 160, 160, 160}[world.Level]
			c.EndSequence++
		}
	case 2:
		world.BossDefeated = 2
		s.ExitDelay--
		if s.ExitDelay == 0 {
			s.ExitFadeSteps = 6
			c.EndSequence++
		}
	case 3:
		if world.Level == 7 {
			if world.Frame&15 != 0 {
				return nil
			}
			for i, color := range w.Presentation.Palette.Desired {
				v := color - 16
				if int8(v) < 0 {
					v = 15
				}
				w.Presentation.Palette.Desired[i] = v
			}
			w.Presentation.Palette.RequestedColors = 32
			s.ExitFadeSteps--
			if s.ExitFadeSteps != 0 {
				return nil
			}
		}
		world.LevelRoutine++
		c.EndSequence = 0
	}
	return nil
}

func (s *CampaignSceneState) hideExitActor(w *ActiveWorld, i int) {
	p := &w.Combat.Players[i]
	p.Motion.Position = int32(uint32(p.Motion.Position)&0xffff) - 0x10000
	s.ExitActors[i] = 0
	p.DisplaySprite = 0
	w.Combat.Display[i].Code = 0
}
