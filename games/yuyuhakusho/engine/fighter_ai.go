package engine

// The visible-world and controller contracts are shared by all original AI
// branches. Aliases keep the existing Chu callers source compatible.
type FighterAIActor = ChuAIActor
type FighterAIFighter = ChuAIFighter
type FighterAIState = ChuAIState

// StepFighterAI recovers the complete1cc8a return for all eleven source IDs.
// Its outputs still enter the same fighter input/action rules as human input.
// It does not execute an attack, advance other actors or decide a match result.
func StepFighterAI(state FighterAIState) (FighterAIState, error) {
	return stepFighterAI(state, chooseFighterAI)
}

type fighterAIChoice uint8

const (
	aiNone fighterAIChoice = iota
	aiNeutralB
	aiForwardB
	aiNeutralA
	aiForwardA
	aiCrouchB
	aiCrouchA
	aiFacingCrouchA
	aiUpA
	aiRetreatJump
	aiSpecialA1
	aiSpecialB1
	_ // There is no shared C/request1 leaf; reserve its uniform stride.
	aiSpecialA2
	aiSpecialB2
	aiSpecialC2
	aiSpecialA3
	aiSpecialB3
	aiSpecialC3
	aiSpecialA4
	aiSpecialB4
	aiSpecialC4
	aiSpecialA5
	aiSpecialB5
	aiSpecialC5
	aiLocalCrouch1
	aiLocalNeutral1
)

func chooseFighterAI(state *FighterAIState, self, target FighterAIFighter) {
	cid := state.Actor.CharacterID
	if cid == 9 {
		chooseChuAI(state, self, target)
		return
	}
	// Source10's projectile-power response precedes both chance gates.
	if cid == 10 && chuAIDistance(target.ChildX, self.X) <= 80 && target.ChildAttackValue >= 48 && target.ChildY != 0 && target.ChildGround == self.Ground {
		applyFighterAIChoice(state, self, target, aiSpecialA1)
		return
	}
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
	dx, dy := chuAIDistance(target.X, self.X), chuAIDistance(target.Y, self.Y)
	air, above := self.Y < self.Ground, target.Y < self.Y
	var choice fighterAIChoice
	switch cid {
	case 0:
		choice = chooseAI0(state, dx, dy, air, above)
	case 1:
		choice = chooseAI1(state, dx, dy, air, above)
	case 2:
		choice = chooseAI2(state, dx, dy, air, above)
	case 3:
		choice = chooseAI3(state, dx, dy, air, above)
	case 4:
		choice = chooseAI4(state, dx, dy, air, above)
	case 5:
		choice = chooseAI5(state, dx, dy, air, above)
	case 6:
		choice = chooseAI6(state, dx, dy, air, above)
	case 7:
		choice = chooseAI7(state, dx, dy, air, above)
	case 8:
		choice = chooseAI8(state, dx, dy, air, above)
	case 10:
		choice = chooseAI10(state, dx, dy, air, above)
	}
	applyFighterAIChoice(state, self, target, choice)
}

func applyFighterAIChoice(state *FighterAIState, self, target FighterAIFighter, choice fighterAIChoice) {
	switch choice {
	case aiNone:
		return
	case aiRetreatJump:
		resetChuAIBehavior(&state.Actor, 7, 255)
	case aiLocalCrouch1, aiLocalNeutral1:
		value := uint8(0)
		if choice == aiLocalCrouch1 {
			value = 2
		}
		setChuAIBase(&state.Actor, value, true)
		state.Actor.Request = 1 // These source2 local leaves return without facing.
	case aiNeutralB:
		chuAIAttack(state, self, target, 0x10)
	case aiForwardB:
		value := uint8(0x18)
		if state.Actor.Facing != 0 {
			value = 0x14
		}
		chuAIAttack(state, self, target, value)
	case aiNeutralA:
		chuAIAttack(state, self, target, 0x40)
	case aiForwardA:
		chuAICloseAttack(state, self, target)
	case aiCrouchB:
		chuAIAttack(state, self, target, 0x12)
	case aiCrouchA:
		chuAIAttack(state, self, target, 0x42)
	case aiFacingCrouchA:
		value := uint8(0x4a)
		if state.Actor.Facing&1 != 0 {
			value = 0x46
		}
		chuAIAttack(state, self, target, value)
	case aiUpA:
		chuAIAttack(state, self, target, 0x41)
	default:
		// Special leaves1..5 each have A/B/C button variants and share the
		// original low-byte writes plus final held-only facing adjustment.
		index := uint8(choice - aiSpecialA1)
		request := index/3 + 1
		button := [...]uint8{0x40, 0x10, 0x20}[index%3]
		chuAISpecial(state, self, target, button, request)
	}
}

// aiFarPlant is the identical distant ground branch used by sources0/3.
func aiFarPlant(s *FighterAIState, dx int16) fighterAIChoice {
	if chuAIRandom(s)&3 == 0 && dx < 128 {
		return aiRetreatJump
	}
	if dx < 192 {
		if chuAIRandom(s)&3 == 0 {
			return aiSpecialA4
		}
		return aiNone
	}
	if chuAIRandom(s)&15 == 0 {
		return aiSpecialA4
	}
	if s.Actor.Pose != 0xa4 {
		return aiSpecialC4
	}
	return aiNone
}

func chooseAI0(s *FighterAIState, dx, dy int16, air, above bool) fighterAIChoice {
	if air {
		if dy > 64 {
			return aiNone
		}
		if dx < 32 {
			return aiForwardA
		}
		if dx < 56 {
			if above {
				return aiUpA
			}
			return aiCrouchA
		}
		if dx < 96 {
			return aiSpecialB3
		}
		return aiNone
	}
	if above {
		if dy <= 72 && dx < 56 {
			return aiNeutralA
		}
		if dy <= 72 && dx < 96 {
			return aiSpecialB3
		}
		return aiNone
	}
	switch {
	case dx < 28:
		return aiForwardA
	case dx < 40:
		return [...]fighterAIChoice{aiNeutralB, aiCrouchB, aiSpecialB1, aiSpecialA2}[chuAIRandom(s)&3]
	case dx < 56:
		if chuAIRandom(s)&1 != 0 {
			return aiForwardA
		}
		return aiCrouchA
	case dx < 64:
		return aiSpecialB5
	case dx < 72:
		return aiSpecialA5
	case dx < 96:
		return aiNone
	default:
		return aiFarPlant(s, dx)
	}
}

func chooseAI1(s *FighterAIState, dx, dy int16, air, above bool) fighterAIChoice {
	if air {
		if dy > 64 {
			return aiNone
		}
		if dx < 48 {
			return aiForwardA
		}
		if dx < 64 {
			if above {
				return aiUpA
			}
			return aiCrouchA
		}
		return aiNone
	}
	if above {
		if dy <= 72 && dx < 40 {
			return aiSpecialB1
		}
		if dy <= 72 && dx < 56 {
			return aiNeutralA
		}
		if dy <= 80 && dx < 96 {
			return aiSpecialB3
		}
		return aiNone
	}
	switch {
	case dx < 32:
		return aiForwardA
	case dx < 48:
		return aiCrouchB
	case dx < 56:
		return aiNeutralB
	case dx < 60:
		return aiForwardA
	case dx < 64:
		return aiCrouchA
	case dx < 88:
		return aiSpecialB3
	case dx < 96:
		return aiNone
	}
	if chuAIRandom(s)&3 == 0 && dx < 128 {
		return aiRetreatJump
	}
	if chuAIRandom(s)&1 == 0 {
		_ = chuAIRandom(s)
		// Original1d55e/1d562 are BLT after BTST, not BRA. Their N/V come
		// from restored D1, whose high word the target loop cleared with
		// MOVEQ. Complete-entry CPU observations confirm both stay false.
	}
	if dx < 192 {
		return aiSpecialB4
	}
	return aiNone
}

func chooseAI2(s *FighterAIState, dx, dy int16, air, above bool) fighterAIChoice {
	if air {
		if dy > 64 {
			return aiNone
		}
		if above {
			if dx < 48 {
				return aiNeutralB
			}
			if dx < 64 {
				return aiUpA
			}
		} else {
			if dx < 48 {
				return aiSpecialA1
			}
			if dx < 64 {
				return aiCrouchA
			}
		}
		return aiNone
	}
	if above {
		if dy <= 80 && dx < 16 {
			return aiNeutralA
		}
		if dy <= 72 && dx < 80 {
			return aiLocalNeutral1
		}
		if dy <= 32 && dx < 96 {
			return aiLocalCrouch1
		}
		return aiNone
	}
	switch {
	case dx < 34:
		return aiForwardA
	case dx < 48:
		return aiCrouchB
	case dx < 72:
		return aiLocalCrouch1
	case dx < 88:
		return aiSpecialB5
	case dx < 96:
		return aiSpecialA3
	}
	if chuAIRandom(s)&3 == 0 && dx < 128 {
		return aiRetreatJump
	}
	if dx < 128 {
		return aiSpecialB4
	}
	if dx < 160 {
		return aiSpecialA5
	}
	if dx < 192 {
		return aiSpecialA4
	}
	return aiNone
}

func chooseAI3(s *FighterAIState, dx, dy int16, air, above bool) fighterAIChoice {
	if air {
		if dy > 64 {
			return aiNone
		}
		if above {
			if dx < 48 {
				return aiNeutralB
			}
			if dx < 64 {
				return aiUpA
			}
			if dx < 128 {
				return aiSpecialA4
			}
		} else if dx < 56 {
			return aiCrouchA
		}
		return aiNone
	}
	if above {
		if dy <= 64 && dx < 64 {
			return aiSpecialA3
		}
		if dy <= 64 && dx < 96 {
			return aiCrouchA
		}
		return aiNone
	}
	switch {
	case dx < 32:
		if chuAIRandom(s)&1 != 0 {
			return aiSpecialA3
		}
		return aiForwardA
	case dx < 48:
		return aiCrouchB
	case dx < 56:
		return aiSpecialA2
	case dx < 72:
		if chuAIRandom(s)&1 != 0 {
			return aiForwardA
		}
		return aiCrouchA
	case dx < 88:
		return aiForwardA
	default:
		return aiFarPlant(s, dx)
	}
}

func chooseAI4(s *FighterAIState, dx, dy int16, air, above bool) fighterAIChoice {
	if air {
		if dy > 64 {
			return aiNone
		}
		if above {
			if dx < 40 {
				return aiNeutralB
			}
			if dx < 56 {
				return aiSpecialA4
			}
			if dx < 64 {
				return aiUpA
			}
		} else if dx < 48 {
			return aiCrouchA
		}
		return aiNone
	}
	if above {
		if dy <= 56 && dx < 64 {
			return aiSpecialA2
		}
		if dy <= 32 && dx < 80 {
			return aiCrouchA
		}
		return aiNone
	}
	switch {
	case dx < 30:
		return aiForwardA
	case dx < 48:
		return aiCrouchB
	case dx < 56:
		return [...]fighterAIChoice{aiNeutralA, aiCrouchA, aiCrouchA, aiSpecialA2}[chuAIRandom(s)&3]
	case dx < 72:
		if chuAIRandom(s)&1 == 0 {
			return aiCrouchA
		}
		return aiSpecialA2
	case dx < 88:
		return [...]fighterAIChoice{aiForwardA, aiSpecialA1, aiSpecialB3, aiSpecialB4}[chuAIRandom(s)&3]
	}
	if chuAIRandom(s)&1 == 0 && dx < 128 {
		return aiRetreatJump
	}
	if dx < 128 {
		switch chuAIRandom(s) & 7 {
		case 1:
			return aiSpecialA1
		case 2:
			return aiSpecialA3
		case 3:
			return aiSpecialA4
		}
	}
	return aiNone
}

func chooseAI5(s *FighterAIState, dx, dy int16, air, above bool) fighterAIChoice {
	if air {
		if above {
			if dy <= 64 {
				if dx < 56 {
					return aiNeutralB
				}
				if dx < 72 {
					return aiUpA
				}
			}
			return aiNone
		}
		if dy <= 64 && dx < 64 {
			return aiCrouchA
		}
		if dy <= 96 && dx < 80 {
			return aiSpecialA4
		}
		return aiNone
	}
	if above {
		if dy <= 88 && dx < 48 {
			return aiNeutralA
		}
		if dy <= 96 && dx < 80 {
			return aiSpecialC4
		}
		return aiNone
	}
	switch {
	case dx < 32:
		if chuAIRandom(s)&1 == 0 {
			return aiForwardA
		}
		return aiNeutralA
	case dx < 72:
		return [...]fighterAIChoice{aiNeutralA, aiCrouchB, aiCrouchB, aiSpecialB3}[chuAIRandom(s)&3]
	case dx < 88:
		if chuAIRandom(s)&1 == 0 {
			return aiCrouchA
		}
		return aiSpecialB3
	}
	if chuAIRandom(s)&3 == 0 && dx < 128 {
		return aiRetreatJump
	}
	if dx < 192 {
		if chuAIRandom(s)&3 == 0 {
			return aiSpecialB4
		}
		return aiSpecialA4
	}
	return aiNone
}

func chooseAI6(s *FighterAIState, dx, dy int16, air, above bool) fighterAIChoice {
	if air {
		if above {
			if dy <= 64 && dx < 48 {
				return aiSpecialA2
			}
			if dy <= 32 && dx < 80 {
				return aiSpecialB1
			}
		} else {
			if dy <= 48 && dx < 24 {
				return aiSpecialA2
			}
			if dy <= 64 && dx < 48 {
				return aiCrouchA
			}
			if dy <= 160 && dx < 32 {
				return aiSpecialA4
			}
			if dy <= 32 && dx < 80 {
				return aiSpecialB1
			}
		}
		if chuAIRandom(s)&15 == 0 {
			s.Actor.Request = 8
		}
		return aiNone
	}
	if above {
		if dy <= 80 && dx < 48 {
			return aiSpecialB2
		}
		if dy <= 32 && dx < 80 {
			return aiCrouchA
		}
		return aiNone
	}
	switch {
	case dx < 32:
		if chuAIRandom(s)&1 == 0 {
			return aiForwardA
		}
		return aiSpecialA2
	case dx < 48:
		return [...]fighterAIChoice{aiCrouchB, aiCrouchB, aiNeutralA, aiSpecialB2}[chuAIRandom(s)&3]
	case dx < 56:
		return [...]fighterAIChoice{aiForwardA, aiForwardB, aiCrouchA, aiSpecialB1}[chuAIRandom(s)&3]
	case dx < 96:
		if chuAIRandom(s)&3 == 0 {
			return aiSpecialA1
		}
		return aiSpecialB1
	case dx < 128:
		return aiNone
	}
	if chuAIRandom(s)&1 == 0 && dx < 160 {
		return aiRetreatJump
	}
	return aiNone
}

func chooseAI7Special(s *FighterAIState) fighterAIChoice {
	return [...]fighterAIChoice{aiSpecialA3, aiSpecialB3, aiSpecialB3, aiSpecialC3}[chuAIRandom(s)&3]
}

func chooseAI7(s *FighterAIState, dx, dy int16, air, above bool) fighterAIChoice {
	if air {
		if above {
			if dy <= 64 {
				if dx < 48 {
					return chooseAI7Special(s)
				}
				if dx < 64 {
					return aiUpA
				}
			}
			return aiNone
		}
		if dy <= 64 && dx < 56 {
			return aiCrouchA
		}
		if dy <= 96 && dx < 128 {
			switch chuAIRandom(s) & 7 {
			case 0:
				return aiSpecialA2
			case 1:
				return aiSpecialB2
			}
		}
		return aiNone
	}
	if above {
		if dy <= 64 && dx < 48 {
			return chooseAI7Special(s)
		}
		if dy <= 64 && dx < 64 {
			return aiForwardA
		}
		return aiNone
	}
	switch {
	case dx < 32:
		return aiForwardA
	case dx < 48:
		return aiCrouchB
	case dx < 64:
		return [...]fighterAIChoice{aiCrouchA, aiNeutralA, aiForwardA, aiSpecialB1}[chuAIRandom(s)&3]
	case dx < 96:
		switch chuAIRandom(s) & 3 {
		case 0:
			return aiFacingCrouchA
		case 1:
			return chooseAI7Special(s)
		case 2:
			return aiSpecialA1
		}
		return aiNone
	}
	if chuAIRandom(s)&1 == 0 && dx < 128 {
		return aiRetreatJump
	}
	if dx < 192 {
		return aiSpecialA1
	}
	if dx < 256 {
		if chuAIRandom(s)&3 == 0 {
			return chooseAI7Special(s)
		}
		return aiNone
	}
	return aiSpecialA2
}

func chooseAI8(s *FighterAIState, dx, dy int16, air, above bool) fighterAIChoice {
	if air {
		if above {
			if dy > 64 {
				return aiNone
			}
			if dx < 48 {
				return aiNeutralB
			}
			if dx < 64 {
				return aiUpA
			}
			return aiSpecialA3
		}
		if dy <= 72 && dx < 56 {
			return aiCrouchA
		}
		return aiNone
	}
	if above {
		if dy <= 64 && dx < 48 {
			return aiSpecialA1
		}
		if dy <= 64 && dx < 64 {
			return aiNeutralA
		}
		return aiNone
	}
	switch {
	case dx < 32:
		return aiForwardA
	case dx < 48:
		return [...]fighterAIChoice{aiCrouchB, aiNeutralA, aiSpecialA1, aiSpecialC2}[chuAIRandom(s)&3]
	case dx < 72:
		// The fourth original comparison repeats #2 after #2 already left.
		return [...]fighterAIChoice{aiFacingCrouchA, aiForwardA, aiSpecialB3, aiNone}[chuAIRandom(s)&3]
	case dx < 88:
		return [...]fighterAIChoice{aiSpecialB2, aiSpecialB2, aiSpecialA3, aiSpecialB4}[chuAIRandom(s)&3]
	}
	if chuAIRandom(s)&1 == 0 && dx < 128 {
		return aiRetreatJump
	}
	if dx < 128 {
		return [...]fighterAIChoice{aiSpecialA2, aiSpecialA2, aiSpecialA3, aiSpecialA4}[chuAIRandom(s)&3]
	}
	if chuAIRandom(s)&7 == 0 {
		return aiSpecialA3
	}
	return aiNone
}

func chooseAI10(s *FighterAIState, dx, dy int16, air, above bool) fighterAIChoice {
	if air {
		if above {
			if dy <= 64 {
				if dx < 48 {
					return aiNeutralB
				}
				if dx < 64 {
					return aiUpA
				}
			}
			return aiNone
		}
		if dy <= 80 && dx < 48 {
			return aiCrouchA
		}
		if dy <= 96 && dx < 128 {
			switch chuAIRandom(s) & 7 {
			case 0:
				return aiSpecialA3
			case 1:
				return aiSpecialB3
			}
		}
		return aiNone
	}
	if above {
		if dy <= 80 && dx < 32 {
			return aiSpecialA1
		}
		if dy <= 128 && dx < 56 {
			return aiNeutralA
		}
		if dy <= 128 && dx < 96 {
			return aiSpecialB3
		}
		return aiNone
	}
	switch {
	case dx < 40:
		return aiForwardA
	case dx < 56:
		return aiCrouchB
	case dx < 112:
		return [...]fighterAIChoice{aiForwardB, aiForwardA, aiSpecialB2, aiSpecialB3}[chuAIRandom(s)&3]
	}
	if chuAIRandom(s)&3 == 0 && dx < 128 {
		return aiRetreatJump
	}
	if dx < 128 {
		return aiSpecialA3
	}
	return aiNone
}
