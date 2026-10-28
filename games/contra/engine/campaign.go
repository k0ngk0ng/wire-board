package contra

import "errors"

func NewCampaign() Campaign { return Campaign{Continues: 3} }

// Validate checks a persisted progression state. A completed World savepoint
// must be quiescent; an in-flight multi-phase tick is never a recoverable commit.
func (c *Campaign) Validate(w *OutdoorWorld, quiescent bool) error {
	if w == nil || w.PlayerMode > 1 || w.LevelRoutine > 11 || c.Choice > 1 || c.Continues > 3 && c.Continues != 255 {
		return errors.New("invalid campaign state")
	}
	switch c.Outcome {
	case CampaignOngoing:
		if w.GameRoutine != 5 || w.Level >= 8 || w.LevelRoutine == 6 && c.Continues > 2 {
			return errors.New("invalid ongoing campaign state")
		}
	case CampaignCleared:
		if w.Level != 8 || w.GameRoutine != 6 {
			return errors.New("invalid cleared campaign state")
		}
	case CampaignFailed:
		if w.Level >= 8 || w.GameRoutine != 0 && w.GameRoutine != 5 {
			return errors.New("invalid failed campaign state")
		}
	default:
		return errors.New("unknown campaign outcome")
	}
	if !c.HasTick && c.LastTick != 0 || c.Pending > CampaignRunInterlude || c.Pending == CampaignNoWork && c.OriginRoutine != 0 {
		return errors.New("invalid campaign tick state")
	}
	if quiescent && c.Pending != CampaignNoWork {
		return errors.New("campaign savepoint has unfinished work")
	}
	if c.Outcome != CampaignOngoing && c.Pending != CampaignNoWork {
		return errors.New("terminal campaign has pending work")
	}
	return nil
}

// RemainingContinues includes the currently offered continue. The cartridge
// decrements its byte when showing that offer, not when Start confirms it.
func (c *Campaign) RemainingContinues(levelRoutine uint8) int {
	if c.Continues&0x80 != 0 {
		return 0
	}
	remaining := int(c.Continues)
	if levelRoutine == 6 {
		remaining++
	}
	return remaining
}

func (c *Campaign) command(plan *CampaignPlan, kind string, from, to uint8, reset *CampaignReset) {
	c.CommandSerial++
	plan.Commands = append(plan.Commands, CampaignCommand{ID: c.CommandSerial, Kind: kind, FromLevel: from, ToLevel: to, Reset: reset})
}

func (c *Campaign) setRoutine(w *OutdoorWorld, next uint8) {
	w.LevelRoutine = next
	c.EndSequence = 0
}

func (c *Campaign) finish() { c.Pending = CampaignNoWork; c.OriginRoutine = 0 }

func (c *Campaign) work(work CampaignWork, plan CampaignPlan) CampaignPlan {
	c.Pending = work
	plan.Work = work
	return plan
}

// BeginTick chooses the original work order. Duplicate completed ticks and a
// terminal challenge are inert, including after JSON save/restore. Work is
// completed by AfterCombat/AfterEnemies/AfterPresentation on the same World copy.
func (c *Campaign) BeginTick(tick uint64, w *OutdoorWorld, input CampaignInput) (CampaignPlan, error) {
	var plan CampaignPlan
	if err := c.Validate(w, true); err != nil {
		return plan, err
	}
	if c.Outcome != CampaignOngoing {
		return plan, nil
	}
	if c.HasTick {
		if tick == c.LastTick {
			plan.Duplicate = true
			return plan, nil
		}
		if tick < c.LastTick {
			return plan, errors.New("campaign tick moved backwards")
		}
	}
	c.LastTick, c.HasTick, c.OriginRoutine = tick, true, w.LevelRoutine
	switch w.LevelRoutine {
	case 0:
		return c.work(CampaignInitializeLevel, plan), nil
	case 1:
		return c.work(CampaignLevelIntro, plan), nil
	case 2:
		return c.work(CampaignPrepareBattle, plan), nil
	case 3:
		return c.work(CampaignLevelEntrance, plan), nil
	case 4:
		plan = c.updatePause(input)
		if c.Paused {
			c.finish()
			return plan, nil
		}
		if w.BossDefeated != 0 {
			c.setRoutine(w, 8)
			c.finish()
			return plan, nil
		}
		return c.work(CampaignCombat, plan), nil
	case 5:
		plan = c.transition(w, input)
		c.finish()
		return plan, nil
	case 6:
		if c.Continues&0x80 != 0 {
			return plan, errors.New("continue menu has no remaining offer")
		}
		if input.Pressed&ButtonStart != 0 {
			c.command(&plan, "stopAudio", w.Level, w.Level, nil)
			if c.Choice != 0 {
				w.GameRoutine = 0
				c.DelayLow, c.DelayHigh = 64, 2
				c.Outcome = CampaignFailed
				c.command(&plan, "failed", w.Level, w.Level, nil)
			} else {
				resources := c.continueResources(w, input)
				plan.Commands = append(plan.Commands, resources.Commands...)
				w.LevelRoutine = 0 // Original $CDBF leaves the sequence byte alone.
			}
		} else if input.Pressed&ButtonSelect != 0 {
			c.Choice ^= 1
		}
		c.finish()
		return plan, nil
	case 7:
		if input.Pressed&ButtonStart != 0 {
			w.GameRoutine = 0
			c.DelayLow, c.DelayHigh = 64, 2
		}
		c.Outcome = CampaignFailed
		c.command(&plan, "failed", w.Level, w.Level, nil)
		c.finish()
		return plan, nil
	case 8:
		return c.work(CampaignCombat, plan), nil
	case 9:
		return c.work(CampaignRunEndSequence, plan), nil
	case 10:
		return c.work(CampaignRunInterlude, plan), nil
	case 11:
		return c.work(CampaignEnemies, plan), nil
	}
	return plan, errors.New("unreachable campaign routine")
}

func (c *Campaign) updatePause(input CampaignInput) CampaignPlan {
	var plan CampaignPlan
	if input.PauseBlocked || input.Pressed&ButtonStart == 0 {
		return plan
	}
	c.Paused = !c.Paused
	if c.Paused {
		plan.Sounds = append(plan.Sounds, 0x74)
	}
	return plan
}

// AfterCombat reads the real post-player state. No caller supplies a fabricated
// victory flag: both game-over bytes are checked before enemies run in routines
// 4 and 8. Routine 9 follows its separate exit-sequence path without this check.
func (c *Campaign) AfterCombat(w *OutdoorWorld, players [2]CampaignPlayer) (CampaignPlan, error) {
	var plan CampaignPlan
	if w == nil || w.GameRoutine != 5 || w.Level >= 8 || c.Pending != CampaignCombat {
		return plan, errors.New("campaign is not awaiting combat")
	}
	if c.OriginRoutine == 4 || c.OriginRoutine == 8 {
		if players[0].GameOver&players[1].GameOver != 0 {
			c.GameOverDelay = 96
			c.setRoutine(w, 11)
			c.finish()
			return plan, nil
		}
		return c.work(CampaignEnemies, plan), nil
	}
	if c.OriginRoutine == 9 {
		return c.work(CampaignExitEnemies, plan), nil
	}
	return plan, errors.New("invalid campaign combat origin")
}

func (c *Campaign) AfterEnemies(w *OutdoorWorld, players [2]CampaignPlayer) (CampaignPlan, error) {
	var plan CampaignPlan
	if w == nil || w.GameRoutine != 5 || w.Level >= 8 || c.Pending != CampaignEnemies && c.Pending != CampaignExitEnemies {
		return plan, errors.New("campaign is not awaiting enemies")
	}
	switch c.OriginRoutine {
	case 4, 9:
	case 8:
		if w.LevelRoutine != 11 {
			ready := c.DelayLow == 0
			if c.DelayLow != 0 && c.DelayLow != 255 {
				ready = c.advanceDelay()
			}
			if ready {
				c.AliveAtExit = 0
				for _, p := range players {
					if p.GameOver == 0 && p.State == 1 {
						c.AliveAtExit++
					}
				}
				if c.AliveAtExit != 0 {
					if w.Level >= 8 {
						return plan, errors.New("invalid clear-tune level")
					}
					if sound := [8]uint8{0, 0, 0, 0, 0x62, 0x62, 0, 0x66}[w.Level]; sound != 0 {
						plan.Sounds = append(plan.Sounds, sound)
					}
					w.LevelRoutine++ // $CE2F increments without clearing the sequence.
				}
			}
		}
	case 11:
		c.GameOverDelay--
		if c.GameOverDelay == 0 {
			w.BossDefeated = 0
			c.setRoutine(w, 5)
			c.command(&plan, "stopAudio", w.Level, w.Level, nil)
		}
	default:
		return plan, errors.New("invalid campaign enemy origin")
	}
	c.finish()
	return plan, nil
}

// AfterPresentation never substitutes a boss flag for a completed exit scene.
// The recovered server-side intro/interlude/end-sequence runner owns its own
// sequence changes; completed=true alone cannot skip those routines. For an
// end-sequence frame, original combat and exit-only enemies still run afterward.
func (c *Campaign) AfterPresentation(w *OutdoorWorld, completed bool) (CampaignPlan, error) {
	var plan CampaignPlan
	if w == nil || w.GameRoutine != 5 || w.Level >= 8 {
		return plan, errors.New("invalid campaign presentation context")
	}
	switch c.Pending {
	case CampaignInitializeLevel:
		if completed {
			c.command(&plan, "stopAudio", w.Level, w.Level, nil)
			w.LevelRoutine++
			c.EndSequence = 0
		}
	case CampaignPrepareBattle:
		if completed {
			w.BossDefeated = 0
			c.AliveAtExit = 0
			plan.Sounds = append(plan.Sounds, [8]uint8{0x46, 0x5a, 0x4a, 0x5a, 0x4e, 0x52, 0x46, 0x56}[w.Level])
			w.LevelRoutine++
		}
	case CampaignLevelEntrance:
		if completed {
			w.LevelRoutine++
			c.EndSequence = 0
		}
	case CampaignLevelIntro, CampaignRunInterlude:
		// Their original banked sequence code, not a generic completion boolean,
		// must change the level routine. Keep the boundary explicit until wired.
	case CampaignRunEndSequence:
		return c.work(CampaignCombat, plan), nil
	default:
		return plan, errors.New("campaign is not awaiting presentation")
	}
	c.finish()
	return plan, nil
}

// The original byte countdown is not uint16 decrement: high is decremented
// when low becomes zero, and completion is reported on a later zero/zero call.
func (c *Campaign) advanceDelay() bool {
	if c.DelayLow|c.DelayHigh == 0 {
		return true
	}
	c.DelayLow--
	if c.DelayLow == 0 && c.DelayHigh != 0 {
		c.DelayHigh--
	}
	return false
}

func (c *Campaign) continueResources(w *OutdoorWorld, input CampaignInput) CampaignPlan {
	var plan CampaignPlan
	players := input.Players
	players[0].GameOver = 0
	players[1].GameOver = 1
	if w.PlayerMode != 0 {
		players[1].GameOver = 0
	}
	lives := uint8(2)
	if input.CheatLives {
		lives = 29
	}
	for actor := 0; actor <= int(w.PlayerMode); actor++ {
		players[actor].Lives = lives
	}
	reset := &CampaignReset{Kind: CampaignResetContinue, Players: players, ClearArcadeScores: true, NextLife: [2]uint16{200, 200}, PlayerMode: w.PlayerMode}
	c.command(&plan, "restartLevel", w.Level, w.Level, reset)
	return plan
}

func (c *Campaign) transition(w *OutdoorWorld, input CampaignInput) CampaignPlan {
	var plan CampaignPlan
	from := w.Level
	win := w.BossDefeated != 0
	players := input.Players
	kind := CampaignResetGameOver
	if win {
		kind = CampaignResetAdvance
	}
	for actor := range players {
		players[actor].State = 0
		if !win {
			players[actor].Weapon = 0
		}
	}
	c.Choice = 0 // Original transient-memory clear includes the menu selection.
	reset := &CampaignReset{Kind: kind, Players: players, ClearStageRuntime: true, PlayerMode: w.PlayerMode}
	c.command(&plan, "resetRuntime", from, from, reset)
	if win {
		w.Level++
		if w.Level >= 8 {
			w.GameRoutine++
			c.Loops++
			c.DelayLow = 128
			c.Outcome = CampaignCleared
			c.command(&plan, "cleared", from, w.Level, nil)
		} else {
			c.command(&plan, "advanceLevel", from, w.Level, nil)
		}
		c.setRoutine(w, 0)
	} else {
		plan.Sounds = append(plan.Sounds, 0x6e)
		c.Continues--
		if c.Continues&128 != 0 {
			c.setRoutine(w, 7)
			c.Outcome = CampaignFailed
			c.command(&plan, "failed", from, from, nil)
		} else {
			c.setRoutine(w, 6)
			c.command(&plan, "offerContinue", from, from, nil)
		}
	}
	return plan
}
