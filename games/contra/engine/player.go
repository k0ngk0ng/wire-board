package contra

import "errors"

// Player is the independent gameplay state of one character. Coordinates and
// small counters retain the original widths, but no emulated address space or
// instruction interpreter is stored here.
type Player struct {
	Motion                                                                  PlayerMotion
	State, X, FractionX                                                     uint8
	VelocityX                                                               int8
	FractionVelocityX                                                       uint8
	IndoorVelocityY                                                         int16
	Scrolling, Fall                                                         uint8
	Weapon, MachineGunTimer, SpawnInvincible, Invincible                    uint8
	Water, Death, OnEnemy, FallLockY, Sequence, PreviousAim                 uint8
	Shock, ForceJump, WaterTimer, Recoil, IndoorAdvance, SpecialSpriteTimer uint8
	BoostX                                                                  int8
	Sprite, SpriteFlip, SkipLedge, Lives, GameOver                          uint8
	DisplaySprite, DisplayAttribute                                         uint8
	IndoorAnchorX, IndoorAnchorY                                            uint8
}

type PlayerInput struct{ Held, Pressed Buttons }

// OutdoorWorld contains shared scene and camera state for every player mode;
// the name is retained for the initial outdoor/frame integration.
// ActivePlayers follows the original three-way set: 0 = only first actor,
// 1 = only second actor, 2 = both. It is frozen while the pair is advanced.
type OutdoorWorld struct {
	GameRoutine, Frame, PlayerMode, LevelRoutine           uint8
	Level, BossDefeated, SceneType, ScrollType, StopScroll uint8
	Screen, ScreenScroll, FrameScroll                      uint8
	AutoScroll                                             [2]uint8
	BossScrollComplete, ActivePlayers                      uint8
	WeaponStrength, AttackEnabled, IndoorScroll            uint8
	Scroll                                                 CollisionScroll
}

type PlayerStepContext struct {
	ThrowWork     *BaseThrowWork
	Work          *TerrainWork
	Actor         int
	Peer          *Player
	World         *OutdoorWorld
	Terrain       *CollisionWindow
	Input         PlayerInput
	PeerInput     PlayerInput
	ScreenCleared uint8
	Fire          func(FireRequest) FireResult
}

func (c *PlayerStepContext) sampleTerrain(x, y uint8) TerrainSample {
	sample := c.Terrain.At(x, y, c.World.Scroll)
	if c.Work != nil {
		c.Work.RecordQuery(sample)
	}
	return sample
}

// StepOutdoor retains the explicit horizontal-outdoor entry used by its original
// reconstruction gate. Step selects the recovered scene-specific paths.
func (p *Player) StepOutdoor(c *PlayerStepContext) ([]uint8, error) {
	if c == nil || c.World == nil || c.World.SceneType != 0 || c.World.ScrollType != 0 {
		return nil, errors.New("StepOutdoor requires horizontal outdoor context")
	}
	return p.stepNormal(c)
}

// stepNormal reproduces the complete $D45E call, including the different base,
// boss and waterfall controls. The frame runner owns shared-camera correction
// and actor-frame timers after this call.
func (p *Player) stepNormal(c *PlayerStepContext) ([]uint8, error) {
	if c == nil || c.World == nil || c.Terrain == nil || c.Fire == nil || c.Actor < 0 || c.Actor > 1 {
		return nil, errors.New("incomplete player step context")
	}
	w := c.World
	if p.State != 1 || !validPlayerScene(w) {
		return nil, errors.New("invalid normal-player scene")
	}
	if (w.ActivePlayers == 2 || w.SceneType != 0) && c.Peer == nil {
		return nil, errors.New("cooperative player step requires peer state")
	}
	var sounds []uint8
	p.setAim(c)
	p.checkLedge(c)
	fire := c.Fire(FireRequest{Actor: c.Actor, X: p.X, Y: p.y(), Hidden: p.hidden(), Aim: p.Motion.Aim,
		Weapon: p.Weapon, Jump: p.Motion.Jump, Water: p.Water, Electrocution: p.Shock, StageType: w.SceneType,
		MachineGunTimer: p.MachineGunTimer, Recoil: p.Recoil, Held: c.Input.Held, Pressed: c.Input.Pressed})
	p.MachineGunTimer, p.Recoil = fire.MachineGunTimer, fire.Recoil
	sounds = append(sounds, fire.Sounds...)
	if p.IndoorAdvance == 0 {
		p.VelocityX, p.FractionVelocityX = 0, 0
	}
	p.Sequence = 3
	switch {
	case p.ForceJump != 0:
		p.ForceJump, p.IndoorAdvance = 0, 0
		p.endIndoorMotion()
		p.Motion.BeginJump(w.SceneType&1 != 0)
	case p.Shock != 0:
		p.Sequence = 1
		p.Shock--
		if c.ScreenCleared != 0 {
			p.Shock = 0
		}
	case p.Fall != 0:
		p.stepFall(c, &sounds)
	case p.Motion.Jump != 0:
		p.stepJump(c, &sounds)
	case p.IndoorAdvance != 0:
		p.stepIndoorMotion(w)
	case p.Water == 0 && c.Input.Pressed&ButtonA != 0:
		if c.Input.Held&(ButtonDown|ButtonLeft|ButtonRight) == ButtonDown {
			p.Sequence = 2
			if p.canDropDown(c) {
				p.startFall(0x81)
			}
		} else {
			p.Motion.BeginJump(w.SceneType&1 != 0)
		}
	default:
		p.groundDirection(c, &sounds)
	}
	p.applyX(c)
	p.autoScroll(w)
	if err := p.animatePlayer(c, &sounds); err != nil {
		return nil, err
	}
	p.PreviousAim = p.Motion.Aim
	if !p.hidden() && p.y() >= 0xe8 {
		sounds = append(sounds, p.Hit()...)
	}
	return sounds, nil
}

func (p *Player) y() uint8     { return uint8(p.Motion.Position >> 8) }
func (p *Player) hidden() bool { return uint8(p.Motion.Position>>16) != 0 }
func (p *Player) setY(y uint8) {
	v := (uint32(p.Motion.Position) & 0xff00ff) | uint32(y)<<8
	p.Motion.Position = int32(v)
	if v&0x800000 != 0 {
		p.Motion.Position -= 0x1000000
	}
}

// $D870 aim rows: facing right/left and boss standing/jumping.
// Direction conflicts are preserved through controller sampling.
var playerAim = [4][16]uint8{
	{2, 2, 7, 2, 4, 3, 6, 2, 0, 1, 8, 2, 7, 2, 7, 2},
	{7, 2, 7, 2, 5, 3, 6, 2, 9, 1, 8, 2, 7, 2, 7, 2},
	{0, 2, 7, 0, 0, 2, 7, 2, 0, 1, 8, 2, 7, 2, 7, 2},
	{0, 2, 7, 0, 10, 3, 6, 2, 0, 1, 8, 2, 7, 2, 7, 2},
}

func (p *Player) setAim(c *PlayerStepContext) {
	row := 0
	if c.World.SceneType&0x80 != 0 {
		row = 2
		firstJump := p.Motion.Jump
		if c.Actor == 1 {
			firstJump = c.Peer.Motion.Jump
		}
		if firstJump != 0 {
			row = 3
		}
	} else if p.PreviousAim >= 5 {
		row = 1
	}
	p.Motion.Aim = playerAim[row][uint8(c.Input.Held)&15]
}

func (p *Player) foot(c *PlayerStepContext) TerrainKind {
	if c.World.SceneType != 0 {
		floor := uint8(160)
		if c.World.SceneType&0x80 != 0 {
			floor = 200
		}
		if p.y() > floor {
			return TerrainPlatform
		}
		return TerrainEmpty
	}
	y := uint16(p.y()) + 16
	if y >= 256 {
		return TerrainEmpty
	}
	return c.sampleTerrain(p.X, uint8(y)).Kind
}
func (p *Player) checkLedge(c *PlayerStepContext) {
	if p.OnEnemy != 0 {
		p.Fall = 0
		return
	}
	if p.Water|p.Motion.Jump|p.Fall|p.IndoorAdvance != 0 {
		return
	}
	if p.SkipLedge&1 != 0 {
		p.Fall = 0
		return
	}
	switch p.foot(c) {
	case TerrainEmpty:
		code := uint8(0x21)
		if p.Motion.Aim >= 5 {
			code = 0x41
		}
		p.startFall(code)
	case TerrainWater:
		p.Water = 1
	default:
		p.Fall = 0
	}
}
func (p *Player) startFall(code uint8) {
	p.Fall = code
	p.FallLockY = uint8(min(255, uint16(p.y())+20))
}
func (p *Player) groundDirection(c *PlayerStepContext, sounds *[]uint8) {
	input := c.Input
	switch {
	case input.Held&ButtonRight != 0:
		p.setXVelocity(1, input)
	case input.Held&ButtonLeft != 0:
		p.setXVelocity(-1, input)
	case input.Held&ButtonDown != 0:
		p.Sequence = 2
	case input.Held&ButtonUp != 0:
		p.Sequence = 1
		if c.World.SceneType&1 != 0 {
			p.indoorUp(c, sounds)
		}
	default:
		p.Sequence = 0
	}
}
func (p *Player) setXVelocity(v int8, input PlayerInput) {
	if p.Water != 0 && input.Held&ButtonDown != 0 {
		return
	}
	p.VelocityX = v
}
func airDirection(input PlayerInput) uint8 {
	// Unlike ground input, the second test overwrites right when both are held.
	if input.Held&ButtonLeft != 0 {
		return 0x40
	}
	if input.Held&ButtonRight != 0 {
		return 0x20
	}
	return 0
}
func (p *Player) applyAirCode(code uint8, input PlayerInput) {
	if code&0x40 != 0 {
		p.setXVelocity(-1, input)
	} else if code&0x20 != 0 {
		p.setXVelocity(1, input)
	}
}
func (p *Player) updateJumpDirection(input PlayerInput) {
	if direction := airDirection(input); direction != 0 {
		p.Motion.Jump = p.Motion.Jump&0x9f | direction
	}
	p.applyAirCode(p.Motion.Jump, input)
}
func (p *Player) stepJump(c *PlayerStepContext, sounds *[]uint8) {
	p.IndoorAdvance = 0
	if p.Motion.Velocity >= 0 {
		fast := uint8(p.Motion.Velocity >> 8)
		checkFloor := c.World.SceneType != 0 || fast >= 1 && (fast >= 4 || (p.y()+c.World.Scroll.Y)&15 < 8)
		if checkFloor && p.foot(c) != TerrainEmpty {
			p.snapLanding(c.World)
			p.updateJumpDirection(c.Input)
			p.land(sounds)
			return
		}
	} else if c.World.BossDefeated&0x80 == 0 && c.sampleTerrain(p.X, p.y()-10).Kind == TerrainSolid {
		p.Motion.Velocity = 0
	}
	p.Motion.ApplyGravity()
	if c.World.ScrollType == 0 || !p.camera(c) {
		p.Motion.AdvanceY()
	}
	p.updateJumpDirection(c.Input)
}
func (p *Player) stepFall(c *PlayerStepContext, sounds *[]uint8) {
	if p.y() >= p.FallLockY && p.foot(c) != TerrainEmpty {
		p.snapLanding(c.World)
		p.land(sounds)
		return
	}
	p.Motion.ApplyGravity()
	p.Motion.AdvanceY()
	if p.y() >= p.FallLockY {
		if direction := airDirection(c.Input); direction != 0 {
			p.Fall = p.Fall&0x9f | direction
		}
	}
	p.applyAirCode(p.Fall, c.Input)
}
func (p *Player) snapLanding(w *OutdoorWorld) {
	if w.SceneType != 0 {
		if w.SceneType&0x80 != 0 {
			p.setY(201)
		} else {
			p.setY(168)
		}
		return
	}
	base := w.Scroll.Y&15 | 0xf0
	p.setY(((p.y() + base) & 0xf0) - base + 4)
}
func (p *Player) land(sounds *[]uint8) {
	if p.Motion.Jump|p.Fall != 0 {
		p.Motion.AnimationFrame, p.Motion.AnimationTimer, p.FallLockY = 0, 0, 0
		*sounds = append(*sounds, 0x0b)
	}
	p.faceAim()
	p.clearMotion()
}
func (p *Player) clearMotion() {
	p.Motion.Jump, p.Fall, p.Sequence = 0, 0, 0
	p.Motion.Velocity, p.IndoorVelocityY = 0, 0
}
func nextCollisionRow(index uint8) uint8 { return index&0xc0 | (index+4)&0x3f }
func collisionAtIndex(w *CollisionWindow, index, column uint8) TerrainKind {
	return [...]TerrainKind{TerrainEmpty, TerrainPlatform, TerrainWater, TerrainSolid}[(w[index]>>(6-2*column))&3]
}
func collisionWithSolidBelow(w *CollisionWindow, index, column uint8) TerrainKind {
	kind := collisionAtIndex(w, index, column)
	if kind == TerrainPlatform && collisionAtIndex(w, nextCollisionRow(index), column) == TerrainSolid {
		return TerrainSolid
	}
	return kind
}
func (p *Player) canDropDown(c *PlayerStepContext) bool {
	if c.World.StopScroll != 0xff && c.World.ScrollType != 0 {
		return true
	}
	return p.hasSupportBelow(c, p.y()+16)
}
func (p *Player) hasSupportBelow(c *PlayerStepContext, y uint8) bool {
	sample := c.sampleTerrain(p.X, y)
	kind := sample.Kind
	if kind == TerrainPlatform && collisionAtIndex(c.Terrain, nextCollisionRow(sample.SourceIndex), sample.Column) == TerrainSolid {
		kind = TerrainSolid
	}
	if kind == TerrainSolid {
		return false
	}
	index := sample.SourceIndex
	for row := y >> 4; row < 13; {
		row++
		index = nextCollisionRow(index)
		if c.Work != nil {
			c.Work.RecordIndexRead(index)
		}
		if collisionAtIndex(c.Terrain, index, sample.Column) != TerrainEmpty {
			return true
		}
	}
	return false
}
func (p *Player) sideSolid(c *PlayerStepContext, dx uint8) bool {
	c.ThrowWork.record(p.X + dx)
	padding := uint8(11)
	if p.Sprite == 0x17 {
		padding = 0
	}
	top := uint8(10)
	if !p.hidden() && p.y() >= padding && p.y()-padding >= 16 {
		top = p.y() - padding
	}
	height := uint8(10) + padding
	if p.foot(c) == TerrainEmpty && p.Motion.Jump&1 != 0 {
		height = 27
	}
	distance := uint8(16) - ((top + c.World.Scroll.Y) & 15)
	sample := c.sampleTerrain(p.X+dx, top)
	kind := sample.Kind
	if kind == TerrainPlatform && collisionAtIndex(c.Terrain, nextCollisionRow(sample.SourceIndex), sample.Column) == TerrainSolid {
		kind = TerrainSolid
	}
	index := sample.SourceIndex
	for {
		if kind == TerrainSolid {
			return true
		}
		if distance >= height {
			return false
		}
		distance += 16
		index = nextCollisionRow(index)
		if c.Work != nil {
			c.Work.RecordIndexRead(index)
		}
		kind = collisionWithSolidBelow(c.Terrain, index, sample.Column)
	}
}
func (p *Player) applyX(c *PlayerStepContext) {
	w := c.World
	p.VelocityX += p.BoostX
	if p.Water&0x80 != 0 || uint8(p.VelocityX)|p.FractionVelocityX == 0 {
		return
	}
	if p.VelocityX >= 0 {
		if w.BossDefeated&0x80 == 0 {
			if p.sideSolid(c, 8) || p.X >= [3]uint8{0xe6, 0xe0, 0xd0}[sceneKind(w)] {
				return
			}
			limits := [8]uint8{0x90, 0xff, 0xff, 0xff, 0xa0, 0xd0, 0xb0, 0xb0}
			if w.BossScrollComplete != 0 && p.X >= limits[w.Level] {
				return
			}
		}
		p.camera(c)
	} else {
		if p.sideSolid(c, 0xf8) || (w.BossDefeated&0x80 == 0 && p.X < [3]uint8{0x1a, 0x20, 0x30}[sceneKind(w)]) {
			return
		}
	}
	sum := uint16(p.FractionX) + uint16(p.FractionVelocityX)
	p.FractionX = uint8(sum)
	p.X += uint8(p.VelocityX) + uint8(sum>>8)
}
func (p *Player) camera(c *PlayerStepContext) bool {
	w := c.World
	if w.SceneType != 0 || w.AutoScroll[0]|w.AutoScroll[1] != 0 || p.State != 1 {
		return false
	}
	if w.ScrollType != 0 {
		if p.y() >= 80 || p.Motion.Velocity >= 0 || !w.allowPlayerScroll() {
			return false
		}
		sum := uint16(uint8(p.Motion.Position)) + uint16(uint8(p.Motion.Velocity))
		p.Motion.Position = (p.Motion.Position &^ 255) | int32(uint8(sum))
		nextY := p.y() + uint8(p.Motion.Velocity>>8) + uint8(sum>>8)
		w.FrameScroll = p.y() - nextY
		p.Scrolling = 1
		return true
	}
	if w.StopScroll&0x80 != 0 {
		return false
	}
	limit := uint8(128)
	if w.ActivePlayers == 2 {
		limit = 168
	}
	if p.X < limit {
		return false
	}
	if w.ActivePlayers == 2 && c.Peer.X < 33 {
		p.VelocityX, p.FractionVelocityX = 0, 0
		return false
	}
	if !w.allowPlayerScroll() {
		return false
	}
	sum := uint16(p.FractionX) + uint16(p.FractionVelocityX)
	p.FractionX = uint8(sum)
	w.FrameScroll = uint8(p.VelocityX) + uint8(sum>>8)
	p.Scrolling = 1
	p.VelocityX, p.FractionVelocityX = 0, 0
	return true
}

func (w *OutdoorWorld) allowPlayerScroll() bool {
	if w.BossDefeated != 0 {
		return true
	}
	if w.StopScroll&0x80 != 0 {
		return false
	}
	threshold, ticks := uint8(160), uint8(96)
	if w.ScrollType != 0 {
		threshold, ticks = 192, 64
	}
	if w.StopScroll == w.Screen && w.ScreenScroll >= threshold {
		w.AutoScroll[0] = ticks
		w.StopScroll = 0xff
		return false
	}
	return true
}
func (p *Player) autoScroll(w *OutdoorWorld) {
	if w.AutoScroll[0]|w.AutoScroll[1] == 0 {
		return
	}
	if w.ScrollType != 0 {
		p.setY(p.y() + 1)
	} else if p.X >= 0x1a {
		p.X--
	}
}
