package contra

import "errors"

// Step dispatches the original character lifecycle. Common actor-frame timers
// and cooperative camera correction belong to the frame runner, after Step.
func (p *Player) Step(c *PlayerStepContext) ([]uint8, error) {
	if c == nil || c.World == nil || c.Actor < 0 || c.Actor > 1 {
		return nil, errors.New("incomplete player lifecycle context")
	}
	if !validPlayerScene(c.World) {
		return nil, errors.New("invalid player lifecycle scene")
	}
	switch p.State {
	case 0:
		if c.Terrain == nil {
			return nil, errors.New("spawn requires terrain")
		}
		p.spawnForScene(c)
		return nil, nil
	case 1:
		return p.stepNormal(c)
	case 2:
		if c.Terrain == nil {
			return nil, errors.New("death motion requires terrain")
		}
		return p.stepDying(c)
	case 3:
		if c.World.PlayerMode != 0 && c.Peer == nil {
			return nil, errors.New("cooperative lifecycle requires peer")
		}
		players := [2]*Player{}
		inputs := [2]PlayerInput{}
		players[c.Actor], players[1-c.Actor] = p, c.Peer
		inputs[c.Actor], inputs[1-c.Actor] = c.Input, c.PeerInput
		borrowLifePair(players, inputs, c.World)
		return nil, nil
	default:
		return nil, errors.New("invalid player lifecycle state")
	}
}

// Hit is the legal normal-to-dying transition ($D474). Contact eligibility,
// including invincibility and crouching in water, is checked by the caller;
// falling off the world uses this same transition even during invincibility.
func (p *Player) Hit() []uint8 {
	if p.State != 1 {
		return nil
	}
	p.clearMotion()
	p.Water, p.Shock, p.SpecialSpriteTimer = 0, 0, 0
	p.Motion.AnimationFrame, p.Motion.AnimationTimer = 0, 0
	p.Death = 1
	p.Motion.Velocity = -640
	p.State = 2
	return []uint8{0x72}
}

// Reset is deliberately not a zero-value Player. Original $DA0B preserves the
// state, weapon, remaining lives, game-over marker, special timer and X boost.
func (p *Player) resetAttributes() {
	*p = Player{State: p.State, Weapon: p.Weapon, Lives: p.Lives, GameOver: p.GameOver,
		SpecialSpriteTimer: p.SpecialSpriteTimer, BoostX: p.BoostX}
}
func (p *Player) resetWeaponAndAttributes() { p.Weapon = 0; p.resetAttributes() }

func (p *Player) spawnForScene(c *PlayerStepContext) {
	p.resetAttributes()
	column := [8]uint8{0, 1, 2, 1, 0, 0, 0, 0}[c.World.Level]
	if c.World.SceneType&0x80 != 0 {
		column = 3
	}
	p.setY([4]uint8{32, 96, 80, 96}[column])
	p.X = [2][4]uint8{{48, 112, 48, 112}, {32, 144, 32, 144}}[c.Actor][column]
	p.Motion.Jump = 1
	safe := func() bool { return p.foot(c) != TerrainSolid && p.hasSupportBelow(c, p.y()+32) }
	if c.World.SceneType == 0 && !safe() {
		p.X = 16
		for !safe() {
			p.X += 16
			if p.X >= 224 {
				p.X = 48
				break
			}
		}
	}
	p.Motion.AnimationFrame = 2
	p.Motion.Velocity = 0
	p.SpawnInvincible = 128
	p.State++
}

func (p *Player) stepDying(c *PlayerStepContext) ([]uint8, error) {
	p.autoScroll(c.World)
	p.Sequence = [3]uint8{4, 4, 6}[sceneKind(c.World)]
	if p.Motion.AnimationTimer != 0 {
		p.Motion.AnimationTimer--
		if p.Motion.AnimationTimer == 0 {
			p.resetWeaponAndAttributes()
			p.State = 0
			if p.Lives == 0 {
				p.GameOver = 1
			} else {
				p.Lives--
			}
			return nil, nil
		}
	} else {
		p.VelocityX = [3]int8{-1, 0, 0}[sceneKind(c.World)]
		if p.Motion.Aim >= 5 {
			p.Death |= 2
			p.VelocityX = -p.VelocityX
		}
		if p.Motion.Velocity >= 512 {
			if uint8(p.Motion.Position>>16) == 1 {
				p.Motion.AnimationTimer = 64
			} else if ground := p.foot(c); ground != TerrainEmpty && ground != TerrainWater {
				p.snapLanding(c.World)
				p.Motion.AnimationTimer = 64
			}
		}
		p.Motion.ApplyGravity()
		p.Motion.AdvanceY()
		p.applyX(c)
	}
	var sounds []uint8
	err := p.animatePlayer(c, &sounds)
	return sounds, err
}

// BorrowLife performs the shared original $D30B routine. It always inspects
// second actor first, and stops at that game-over actor even if A was not newly
// pressed. Borrowing creates one active life, without increasing reserve lives.
func BorrowLife(players *[2]Player, inputs [2]PlayerInput, w *OutdoorWorld) bool {
	if players == nil {
		return false
	}
	return borrowLifePair([2]*Player{&players[0], &players[1]}, inputs, w)
}
func borrowLifePair(players [2]*Player, inputs [2]PlayerInput, w *OutdoorWorld) bool {
	if w == nil || w.LevelRoutine != 4 || w.PlayerMode == 0 || players[0] == nil || players[1] == nil {
		return false
	}
	target := 1
	if players[target].GameOver == 0 {
		target = 0
		if players[target].GameOver == 0 {
			return false
		}
	}
	if inputs[target].Pressed&ButtonA == 0 {
		return false
	}
	donor := players[1-target]
	if donor.Lives == 0 || donor.Lives == 1 && donor.State != 1 || donor.IndoorAdvance != 0 {
		return false
	}
	donor.Lives--
	player := players[target]
	player.resetWeaponAndAttributes()
	player.State, player.GameOver = 0, 0
	return true
}
