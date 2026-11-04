package engine

import "errors"

type ChuAIActor struct {
	CharacterID, Facing, Pose                      uint8
	Energy                                         int16
	Controls                                       FighterControls
	Request, Behavior                              uint8
	TargetCursor, Countdown, Elapsed, FollowTarget uint16
}

// ChuAIFighter exposes only the visible fields consumed by the original AI.
// Coordinates are signed high words, not floating-point or complete positions.
// Child fields belong to that fighter's reserved +0x100 projectile slot; the
// original threat check uses its nonzero Y rather than its allocation size.
type ChuAIFighter struct {
	Size                        uint16
	Team                        uint8
	Health                      int16
	X, Y, Ground                int16
	FrameFlags, AttackFlags     uint8
	ChildX, ChildY, ChildGround int16
	ChildAttackFlags            uint8
	ChildAttackValue            uint8
}

type ChuAIState struct {
	ObjectID              uint16
	Actor                 ChuAIActor
	Fighters              [4]ChuAIFighter // Original object order 8400/8800/8c00/9000.
	Difficulty            uint16          // Original word-table offsets 0, 2, 4, 6.
	RandomSeed            uint32
	ArenaLeft, ArenaRight int16
}

var (
	chuAIPeriodScales       = [4]uint16{16, 64, 128, 256}
	chuAIDecisionThresholds = [4]uint16{0x4000, 0x6000, 0xa000, 0xffff}
	chuAIReactionThresholds = [4]uint16{0x4000, 0x6000, 0xa000, 0xffff}
	chuAILaneMasks          = [4]uint16{255, 127, 63, 31}
)

// StepChuAI recovers the complete 0x1cc8a call for source character 9, including
// all seventeen shared behaviors and the Chu decision tail. The caller owns
// input-mode clearing, subsequent fighter updates and the shared world RNG;
// only this returned seed may be passed to the next world consumer.
func StepChuAI(state ChuAIState) (ChuAIState, error) {
	if state.Actor.CharacterID != 9 {
		return state, errors.New("unsupported Chu AI source, difficulty or target cursor")
	}
	return stepFighterAI(state, chooseChuAI)
}

// stepFighterAI keeps one copy of the original shared driver and behaviors.
// Character wrappers supply only the reached decision tail.
func stepFighterAI(state ChuAIState, decide func(*ChuAIState, ChuAIFighter, ChuAIFighter)) (ChuAIState, error) {
	original := state
	selfIndex, ok := chuAIIndex(state.ObjectID)
	if !ok || state.Actor.CharacterID > 10 || state.Difficulty > 6 || state.Difficulty&1 != 0 || state.Actor.TargetCursor&1 != 0 {
		return state, errors.New("unsupported Chu AI source, difficulty or target cursor")
	}
	a := &state.Actor
	self := state.Fighters[selfIndex]
	oldCountdown := int16(a.Countdown)
	a.Elapsed++
	a.Countdown--
	// SUB.W followed by BGE retains overflow-aware signed comparison. Testing
	// the wrapped result alone would mishandle the minimum signed input.
	if oldCountdown <= 0 {
		product := uint32(chuAIRandom(&state)) * uint32(chuAIPeriodScales[state.Difficulty/2])
		a.Behavior = uint8(product>>16) + 2
		if a.Behavior > 16 {
			a.Behavior = 16
		}
		a.Countdown = (chuAIRandom(&state) & 127) + 1
		a.Elapsed = 0
	}
	for attempt := 0; attempt < 4; attempt++ {
		a.TargetCursor = (a.TargetCursor + 2) & 7
		index := int(a.TargetCursor / 2)
		target := state.Fighters[index]
		if index == selfIndex || target.Size == 0 || target.Health < 0 || target.Team == self.Team {
			continue
		}
		stop, err := reactChuAI(&state, self, target)
		if err != nil {
			return original, err
		}
		if stop {
			return state, nil
		}
		threshold := chuAIThreshold(chuAIDecisionThresholds[state.Difficulty/2], self.Health)
		if chuAIRandom(&state) < threshold {
			decide(&state, self, target)
		}
		return state, nil
	}
	return state, nil
}

func chuAIIndex(id uint16) (int, bool) {
	if id < 0x8400 || id > 0x9000 || (id-0x8400)&0x3ff != 0 {
		return 0, false
	}
	return int((id - 0x8400) / 0x400), true
}

func chuAIObjectID(index int) uint16 { return 0x8400 + uint16(index)*0x400 }

func chuAIRandom(state *ChuAIState) uint16 {
	next, value := NextRandom(state.RandomSeed)
	state.RandomSeed = next
	return uint16(value)
}

func chuAIThreshold(base uint16, health int16) uint16 {
	return base - (uint16(health) << 5)
}

func chuAIDistance(a, b int16) int16 {
	delta := a - b
	if delta < 0 {
		delta = -delta // NEG.W deliberately keeps -32768 unchanged.
	}
	return delta
}

func setChuAIBase(actor *ChuAIActor, value uint8, changed bool) {
	actor.Controls.Held = actor.Controls.Held&0xff00 | Buttons(value)
	actor.Controls.Pressed = actor.Controls.Pressed&0xff00 | Buttons(value)
	if changed {
		actor.Controls.Changed = actor.Controls.Changed&0xff00 | Buttons(value)
	}
}

func resetChuAIBehavior(actor *ChuAIActor, behavior uint8, countdown uint16) {
	actor.Behavior, actor.Countdown, actor.Elapsed = behavior, countdown, 0
}

func reactChuAI(state *ChuAIState, self, target ChuAIFighter) (bool, error) {
	a := &state.Actor
	if a.Energy <= 32 {
		resetChuAIBehavior(a, 12, 255)
		return runChuAIBehavior(state, self, target)
	}
	threshold := chuAIThreshold(chuAIReactionThresholds[state.Difficulty/2], self.Health)
	if chuAIRandom(state) < threshold {
		if chuAIDistance(target.ChildX, self.X) <= 64 && target.ChildY != 0 && target.ChildGround == self.Ground {
			if chuAIDistance(target.X, self.X) >= 128 && chuAIRandom(state)&1 != 0 {
				resetChuAIBehavior(a, 15, 16)
				return runChuAIBehavior(state, self, target)
			}
			if target.ChildAttackFlags&4 != 0 {
				resetChuAIBehavior(a, 0, 4)
				return runChuAIBehavior(state, self, target)
			}
			if target.ChildAttackFlags&8 != 0 {
				resetChuAIBehavior(a, 1, 4)
				return runChuAIBehavior(state, self, target)
			}
		}
		if target.FrameFlags&16 != 0 {
			if target.AttackFlags&4 != 0 {
				if a.CharacterID == 5 && chuAIRandom(state)&15 == 0 {
					setChuAIBase(a, 0, true)
					a.Request = 1
					return true, nil
				}
				resetChuAIBehavior(a, 0, 4)
			} else if target.AttackFlags&8 != 0 {
				if a.CharacterID == 5 && chuAIRandom(state)&15 == 0 {
					setChuAIBase(a, 2, true)
					a.Request = 1
					return true, nil
				}
				resetChuAIBehavior(a, 1, 4)
			}
		}
	}
	return runChuAIBehavior(state, self, target)
}

func runChuAIBehavior(state *ChuAIState, self, target ChuAIFighter) (bool, error) {
	a := &state.Actor
	switch a.Behavior {
	case 0, 1:
		value := uint8(0x20)
		if a.Behavior == 1 {
			value = 0x22
		}
		setChuAIBase(a, value, true)
		return true, nil
	case 2, 3:
		value := uint8(0)
		if a.Behavior == 3 {
			value = 2
		}
		setChuAIBase(a, value, true)
		if a.Countdown == 1 {
			resetChuAIBehavior(a, 13, 1)
		}
	case 4, 5:
		value := uint8(4)
		if a.Behavior == 5 {
			value = 8
		}
		setChuAIBase(a, value, true)
		if a.Behavior == 4 && self.X <= state.ArenaLeft || a.Behavior == 5 && self.X >= state.ArenaRight {
			resetChuAIBehavior(a, 13, 1)
		}
	case 6, 7, 8:
		if int16(a.Elapsed) <= 8 {
			value := uint8(1)
			if a.Behavior != 6 {
				value = 9
				if (a.Facing&1 != 0) == (a.Behavior == 7) {
					value = 5
				}
			}
			a.Controls.Held = a.Controls.Held&0xff00 | Buttons(value)
			a.Countdown = 255
		} else if self.Ground == self.Y {
			resetChuAIBehavior(a, 13, 1)
		}
	case 9, 10:
		if a.Elapsed == 0 {
			a.Request = a.Behavior - 2
			a.Countdown = 8
		}
	case 11:
		if int16(a.Elapsed) <= 8 {
			setChuAIBase(a, 0x21, true)
			a.Countdown = 255
		} else if self.Ground == self.Y {
			a.Countdown = 0
		}
	case 12:
		setChuAIBase(a, 0x60, true)
		if a.Energy > 222 || chuAIDistance(target.X, self.X) < 96 {
			a.Countdown = 0
		}
		return true, nil
	case 13:
		if int16(a.Elapsed) <= 2 {
			value := uint8(4)
			if self.X < target.X {
				value = 8
			}
			setChuAIBase(a, value, true)
		}
	case 14, 15:
		if a.Elapsed == 0 {
			value := uint8(8)
			if self.X < target.X {
				value = 4
			}
			setChuAIBase(a, value, true)
			a.Request, a.Countdown = a.Behavior-7, 8
		}
	case 16:
		if a.Elapsed == 0 {
			a.Countdown += 0x100
			choice := chuAIRandom(state)
			for attempt := 0; attempt < 4; attempt++ {
				choice = (choice + 1) & 3
				id := chuAIObjectID(int(choice))
				a.FollowTarget = id
				candidate := state.Fighters[choice]
				if id == state.ObjectID || candidate.Size == 0 || candidate.Health < 0 {
					continue
				}
				// MOVE.B team,D0 changes the next trial's selection value when
				// the candidate is friendly; preserve this original register use.
				choice = choice&0xff00 | uint16(candidate.Team)
				if candidate.Team != self.Team {
					break
				}
			}
		}
		index, ok := chuAIIndex(a.FollowTarget)
		if !ok {
			return false, errors.New("Chu AI follow target is unavailable")
		}
		follow := state.Fighters[index]
		if a.Elapsed&chuAILaneMasks[state.Difficulty/2] == 0 && follow.Ground != self.Ground {
			setChuAIBase(a, 0x21, true)
			break
		}
		value := uint8(0)
		if chuAIDistance(self.X, follow.X) >= 16 {
			value = 4
			if self.X < follow.X {
				value = 8
			}
		}
		setChuAIBase(a, value, true)
	default:
		return false, errors.New("Chu AI behavior is outside decoded table")
	}
	return false, nil
}

func faceChuAITarget(state *ChuAIState, self, target ChuAIFighter) {
	if state.Actor.Facing == 0 {
		if target.X <= self.X {
			state.Actor.Controls.Held |= ButtonLeft
		}
	} else if target.X >= self.X {
		state.Actor.Controls.Held |= ButtonRight
	}
}

func chuAIAttack(state *ChuAIState, self, target ChuAIFighter, value uint8) {
	setChuAIBase(&state.Actor, value, false)
	faceChuAITarget(state, self, target)
}

func chuAISpecial(state *ChuAIState, self, target ChuAIFighter, value, request uint8) {
	setChuAIBase(&state.Actor, value, true)
	state.Actor.Request = request
	faceChuAITarget(state, self, target)
}

func chooseChuAI(state *ChuAIState, self, target ChuAIFighter) {
	if chuAIRandom(state)&127 == 0 {
		value := uint8(chuAIRandom(state))
		state.Actor.Controls.Changed = state.Actor.Controls.Changed&0xff00 | Buttons(value&0x70)
		state.Actor.Request = value & 7
		faceChuAITarget(state, self, target)
		return
	}
	if chuAIRandom(state)&7 != 0 || target.Ground != self.Ground {
		return
	}
	dx := chuAIDistance(target.X, self.X)
	if self.Y < self.Ground {
		dy := chuAIDistance(target.Y, self.Y)
		if target.Y < self.Y {
			if dy <= 64 {
				if dx < 48 {
					chuAIAttack(state, self, target, 0x10)
				} else if dx < 80 {
					chuAIAttack(state, self, target, 0x41)
				}
			}
			return
		}
		if dy <= 64 && dx < 56 {
			chuAIAttack(state, self, target, 0x42)
			return
		}
		if dy <= 96 && dx < 128 {
			switch chuAIRandom(state) & 7 {
			case 0:
				chuAISpecial(state, self, target, 0x40, 3)
			case 1:
				chuAISpecial(state, self, target, 0x10, 3)
			}
		}
		return
	}
	if target.Y < self.Y {
		dy := chuAIDistance(target.Y, self.Y)
		if dy <= 72 && dx < 48 {
			chuAIAttack(state, self, target, 0x40)
		} else if dy <= 80 && dx < 64 {
			chuAICloseAttack(state, self, target)
		} else if dy <= 32 && dx < 80 {
			chuAISpecial(state, self, target, 0x40, 4)
		}
		return
	}
	switch {
	case dx < 32:
		chuAICloseAttack(state, self, target)
	case dx < 48:
		chuAIAttack(state, self, target, 0x12)
	case dx < 64:
		switch chuAIRandom(state) & 3 {
		case 0:
			chuAIAttack(state, self, target, 0x42)
		case 1:
			chuAISpecial(state, self, target, 0x40, 2)
		case 2:
			chuAISpecial(state, self, target, 0x10, 3)
		case 3:
			chuAISpecial(state, self, target, 0x40, 4)
		}
	case dx < 96:
		switch chuAIRandom(state) & 3 {
		case 1:
			chuAISpecial(state, self, target, 0x40, 3)
		case 2:
			chuAISpecial(state, self, target, 0x40, 4)
		case 3:
			chuAISpecial(state, self, target, 0x40, 5)
		}
	default:
		if chuAIRandom(state)&3 == 0 && dx < 128 {
			resetChuAIBehavior(&state.Actor, 7, 255)
			return
		}
		if dx < 128 {
			request := uint8(3)
			if chuAIRandom(state)&1 != 0 {
				request = 5
			}
			chuAISpecial(state, self, target, 0x40, request)
		} else if dx < 192 {
			if chuAIRandom(state)&3 == 0 {
				chuAISpecial(state, self, target, 0x40, 5)
			}
		} else if chuAIRandom(state)&15 == 0 {
			chuAISpecial(state, self, target, 0x40, 5)
		} else if state.Actor.Pose != 0xa6 {
			chuAISpecial(state, self, target, 0x20, 5)
		}
	}
}

func chuAICloseAttack(state *ChuAIState, self, target ChuAIFighter) {
	value := uint8(0x48)
	if state.Actor.Facing != 0 {
		value = 0x44
	}
	chuAIAttack(state, self, target, value)
}
