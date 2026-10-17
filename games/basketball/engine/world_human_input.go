package engine

import "fmt"

// HumanInputEffects writes the canonical proximity flags held by the world's
// preparation/contact stage. It is not a second persistent copy of $0634.
type HumanInputEffects struct {
	ChangePickupFlags bool
	PickupFlags       uint8
}

func tickOriginalHumanWindow(v uint8) uint8 {
	v--
	if int8(v) < 0 {
		return 0
	}
	return v
}

func originalHumanDirections(result *InputResult, p Player, hasBall bool, frame ControllerFrame) {
	m := &result.Memory
	finish := func() {
		m.TapWindow = tickOriginalHumanWindow(m.TapWindow)
		if m.TapWindow == 0 && m.LastTap != 0 {
			m.LastTap = 0
			return
		}
		vertical := frame.Pressed & (ButtonUp | ButtonDown)
		if vertical == 0 {
			return
		}
		if result.Heading != HeadingUp && result.Heading != HeadingDown {
			m.TapWindow, m.LastTap = 0, 0
			return
		}
		if m.LastTap != 0 && vertical == m.LastTap {
			action := Action(0x2d)
			if p.Action == 0x42 {
				action = 0x2f
			} else if hasBall {
				action = 0x2e
			}
			requestInput(result, action)
			m.TapWindow, m.LastTap = 0, 0
			return
		}
		m.LastTap, m.TapWindow = vertical, 10
	}
	held := func() {
		if frame.Held&directions != 0 {
			requestInput(result, ActionWalk)
		} else {
			requestInput(result, ActionIdle)
			finish()
		}
	}
	if p.Action < 0x60 && p.Action&15 == 2 {
		if frame.Held&directions != 0 && result.Heading != HeadingNone && result.Heading != HeadingUp && result.Heading != HeadingDown && (result.Heading >= HeadingDownLeft) != p.Pose.Mirrored {
			requestInput(result, ActionBrake)
			m.LastTap, m.TapWindow = 0, 0
			return
		}
		finish()
		return
	}
	if p.Action >= 0x60 {
		if m.AttackTapWindow == 0 {
			m.LastAttackTap = 0
		}
		pressed := frame.Pressed & directions
		switch pressed {
		case ButtonDown:
			m.TapWindow = 0
			held()
			return
		case ButtonUp:
			if m.LastAttackTap == ButtonUp {
				if !hasBall {
					m.AttackTapWindow, m.LastAttackTap = 0, 0
					return
				}
			} else {
				m.LastAttackTap = ButtonUp
				m.AttackTapWindow = 8
				m.TapWindow = 0
				m.AttackTapWindow = tickOriginalHumanWindow(m.AttackTapWindow)
			}
		case 0:
			m.AttackTapWindow = tickOriginalHumanWindow(m.AttackTapWindow)
		default:
			m.AttackTapWindow = 0
		}
	}
	if m.TapWindow == 0 {
		m.LastTap = 0
	}
	horizontal := frame.Pressed & (ButtonLeft | ButtonRight)
	if horizontal != 0 {
		if horizontal == m.LastTap {
			requestInput(result, ActionRun)
			m.LastTap, m.TapWindow = 0, 0
			return
		}
		m.LastTap, m.TapWindow = horizontal, 8
	}
	m.TapWindow = tickOriginalHumanWindow(m.TapWindow)
	held()
}

func humanInputObject(s WorldActionState, object uint8) (Position, uint8, uint8) {
	if object < 6 {
		a := s.Auxiliary[object-4]
		return a.Body.Position, a.Action, uint8(a.Owner)
	}
	if object == 6 {
		return s.Ball.Body.Position, uint8(s.Ball.Action), uint8(s.Holder)
	}
	h := s.Hoops[object-7]
	return h.Position, uint8(h.Action), h.Owner
}

// originalHumanPickupSearch is $849A, including original ordered traversal
// and the half-distance low-byte comparison, rather than a nearest search.
func originalHumanPickupSearch(s *WorldInputState, actor uint8, result *InputResult, effects *HumanInputEffects) bool {
	p := s.World.Actors[actor].Player
	if p.Action < 4 {
		for object := uint8(4); object < 13; object++ {
			position, action, owner := humanInputObject(s.World, object)
			if action != 0 || uint16(p.Body.Position.Height>>8) != uint16(position.Height>>8) || owner < 6 {
				continue
			}
			depth := uint8((p.Body.Position.Depth >> 8) - (position.Depth >> 8))
			if depth >= 17 && depth < 240 {
				continue
			}
			distance := int32(int16((p.Body.Position.X >> 8) - (position.X >> 8)))
			if distance < 0 {
				distance = -distance
			}
			if uint8(uint16(distance)>>1) >= 8 {
				continue
			}
			if object < 7 {
				effects.ChangePickupFlags = true
				effects.PickupFlags = [3]uint8{16, 32, 1}[object-4]
				return false
			}
			hoop := &s.World.Hoops[object-7]
			hoop.Action, hoop.Entered, hoop.Owner = HoopCarried, false, actor
			requestInput(result, 0x57)
			return true
		}
	}
	effects.ChangePickupFlags = true
	effects.PickupFlags = 0
	return false
}

func originalHumanCanAirFollow(s WorldInputState, actor uint8, ctx WorldInputContext) (bool, error) {
	p := s.World.Actors[actor].Player
	if !p.Airborne {
		return false, nil
	}
	link := s.World.PlayerLinks[actor]
	if link&128 == 0 {
		return true, nil
	}
	parent := link & 15
	if parent >= 13 {
		return false, fmt.Errorf("invalid input parent %d", parent)
	}
	if ctx.ObjectKinds[parent] != 12 {
		return false, nil
	}
	return worldInputAirborne(s.World, parent)
}

func originalHumanButtonContext(s *WorldInputState, actor uint8, ctx WorldInputContext, result *InputResult, effects *HumanInputEffects) (bool, bool, error) {
	w := &s.World
	a := &w.Actors[actor]
	p := a.Player
	ownsAux := w.Auxiliary[0].Owner == int8(actor) || w.Auxiliary[1].Owner == int8(actor)
	search := !ownsAux
	if !ownsAux && p.Airborne {
		search = w.CarriedPlayer == int8(actor)
		if search {
			air, err := worldInputAirborne(*w, w.PlayerLinks[actor]&15)
			if err != nil {
				return false, false, err
			}
			search = !air
		}
	}
	if search {
		if originalHumanPickupSearch(s, actor, result, effects) {
			return false, true, nil
		}
		if effects.PickupFlags&1 != 0 {
			requestInput(result, 0x57)
			return false, false, nil
		}
	}
	if a.LinkedActor < 0 {
		if ownsAux {
			// $83B9 scans owners4/5 in order. $8455 takes the displayed
			// pose's mirror bit, whereas the horizontal B path copies facing.
			auxiliary := 0
			if w.Auxiliary[0].Owner != int8(actor) {
				auxiliary = 1
			}
			object := &w.Auxiliary[auxiliary]
			if result.Memory.PendingButtons&ButtonA != 0 {
				object.Heading = HeadingRight
				if p.Pose.Mirrored {
					object.Heading = HeadingLeft
				}
				requestInput(result, 0x27)
				return false, false, nil
			}
			// $F9A2 has already written this frame's controller direction.
			if result.Heading == HeadingRight || result.Heading == HeadingLeft || p.Action == 0x3d {
				requestInput(result, 0x29)
				object.FacingLeft = p.FacingLeft
				return false, false, nil
			}
			if p.Action&15 == 2 {
				requestInput(result, 0x68)
				return false, false, nil
			}
			if !p.Airborne {
				requestInput(result, 0x4f)
				return false, false, nil
			}
			if p.Action == 0x42 {
				requestInput(result, 0x5f)
				return false, false, nil
			}
			return true, false, nil
		}
		flags := effects.PickupFlags & 0x30
		if flags != 0 {
			auxiliary := 0
			if flags == 0x20 {
				auxiliary = 1
			}
			if w.Auxiliary[auxiliary].Owner >= 0 {
				return false, false, nil
			}
			if w.CarriedPlayer != int8(actor) {
				requestInput(result, 0x4e)
				return false, false, nil
			}
		}
		link := w.PlayerLinks[actor]
		if link&128 != 0 {
			parent := link & 15
			if parent >= 13 {
				return false, false, fmt.Errorf("invalid input parent %d", parent)
			}
			if ctx.ObjectKinds[parent] != 12 {
				return true, false, nil
			}
		}
	}
	if p.Action == 0x42 {
		requestInput(result, 0x5f)
		return false, false, nil
	}
	return true, false, nil
}

func groupOriginalHumanRequest(s *WorldInputState, actor uint8, ctx WorldInputContext, result *InputResult) error {
	if !result.Request.Pending || result.Request.Action >= 16 {
		return nil
	}
	w := &s.World
	p := w.Actors[actor].Player
	request := result.Request.Action
	group := Action(0)
	if w.SpecialHolder == int8(actor) {
		group = 0x50
		if request >= 7 {
			group = 0x60
		}
		result.Request.Action |= group
		return nil
	}
	if request < 7 && w.PlayerLinks[actor]&128 != 0 {
		parent := w.PlayerLinks[actor] & 15
		if parent >= 13 {
			return fmt.Errorf("invalid input parent %d", parent)
		}
		parentLink, err := worldInputAttachment(*w, ctx, parent)
		if err != nil {
			return err
		}
		parentAir, err := worldInputAirborne(*w, parent)
		if err != nil {
			return err
		}
		forceOrdinary := ctx.ObjectKinds[parent] != 12 || parentLink&128 != 0 || (parentAir && p.Action == 0x3d)
		if forceOrdinary {
			if request != 4 {
				requestInput(result, 0x3d)
				return nil
			}
		} else if !parentAir && (p.Action == 0x40 || p.Action == 0x41) && request == 4 {
			// The original permits an ordinary jump out of these mounted actions.
		} else {
			w.CarriedPlayer = int8(actor)
			result.Request.Action |= 0x40
			return nil
		}
	}
	if w.Holder == int8(actor) {
		group = 0x10
	} else {
		for _, h := range w.Hoops {
			if h.Owner == actor {
				group = 0x30
				break
			}
		}
		if group == 0 && (w.Auxiliary[0].Owner == int8(actor) || w.Auxiliary[1].Owner == int8(actor)) {
			group = 0x20
		}
	}
	result.Request.Action |= group
	return nil
}

func advanceWorldHumanInputWithEffects(s *WorldInputState, actor uint8, ctx WorldInputContext) (HumanInputEffects, error) {
	effects := HumanInputEffects{}
	if actor >= 4 {
		return effects, fmt.Errorf("invalid human actor %d", actor)
	}
	effects.PickupFlags = ctx.PickupFlags[actor]
	if err := advanceOriginalHumanCommands(s, actor, ctx); err != nil {
		return effects, err
	}
	w := &s.World
	a := &w.Actors[actor]
	p := &a.Player
	frame := ctx.Frames[actor]
	s.Held[actor], s.Pressed[actor] = frame.Held, frame.Pressed
	result := InputResult{Memory: s.Memory[actor], Request: p.Request, Heading: HeadingForButtons(frame.Held)}
	originalHumanDirections(&result, *p, w.Holder == int8(actor), frame)
	if !(result.Request.Pending && result.Request.Action == ActionBrake) {
		m := &result.Memory
		m.PendingButtons |= frame.Pressed & actionButtons
		processed := false
		if m.PendingButtons != 0 {
			if m.ButtonDelay == 0 {
				m.ButtonDelay = 2
			}
			m.ButtonDelay--
			if m.ButtonDelay == 0 {
				processed = true
				if m.PendingButtons == actionButtons {
					requestInput(&result, ActionJump)
					if p.Action&15 == 2 {
						requestInput(&result, ActionRunningJump)
					} else {
						air, err := originalHumanCanAirFollow(*s, actor, ctx)
						if err != nil {
							return effects, err
						}
						if air {
							permit := a.JumpPhase >= 0
							if !permit && ctx.World.EnhancementEnabled && ctx.World.Teams[actor/2] == 0 && ctx.JumpEnhancements[actor] == 0x1a && p.Body.Velocity.Height >= 0 && uint8(p.Body.Position.Height>>16) == 0 && uint8(p.Body.Position.Height>>8) < 208 {
								permit = true
							}
							if !permit {
								requestInput(&result, ActionIdle)
							} else if p.Action < 0x3d || p.Action > 0x3f {
								a.JumpPhase = -128
							}
						}
					}
				} else {
					ownsHoop := false
					for _, h := range w.Hoops {
						if h.Owner == actor {
							ownsHoop = true
							break
						}
					}
					carrying := w.Holder == int8(actor) || ownsHoop
					attack, handled := false, false
					if !carrying {
						var err error
						attack, handled, err = originalHumanButtonContext(s, actor, ctx, &result, &effects)
						if err != nil {
							return effects, err
						}
					}
					if !handled {
						if carrying || a.LinkedActor >= 0 {
							action := Action(7)
							if m.PendingButtons == ButtonB {
								action = 9
							}
							requestInput(&result, action)
						} else if attack {
							action := Action(0x60)
							if m.PendingButtons == ButtonB {
								action = 0x64
							}
							if p.Action == ActionRun {
								action += 2
							}
							requestInput(&result, action)
						}
						air, err := originalHumanCanAirFollow(*s, actor, ctx)
						if err != nil {
							return effects, err
						}
						if air {
							result.Request.Action++
						}
					}
				}
				m.PendingButtons = 0
			}
		}
		if !processed && p.Action < 0x40 && (p.Action&15 == 9 || p.Action&15 == 10) && frame.Held&ButtonB == 0 {
			requestInput(&result, 0x0b)
			air, err := originalHumanCanAirFollow(*s, actor, ctx)
			if err != nil {
				return effects, err
			}
			if air {
				result.Request.Action++
			}
		}
	}
	if err := groupOriginalHumanRequest(s, actor, ctx, &result); err != nil {
		return effects, err
	}
	s.Memory[actor] = result.Memory
	p.Request, p.Heading = result.Request, result.Heading
	return effects, nil
}
