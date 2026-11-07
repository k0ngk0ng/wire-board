package contra

import "errors"

// Original bank 4/5 animation tables. These are decoded game data, not machine
// instructions. Source: sprites-catalog.json.gz and REA $A551..A778.
var playerWalkSprites = [5][6]uint8{
	{2, 3, 4, 5, 3, 6}, {13, 14, 15, 13, 14, 15}, {16, 17, 18, 16, 17, 18},
	{19, 20, 21, 19, 20, 21}, {81, 82, 83, 81, 82, 83},
}
var playerAimSequence = [10]uint8{0, 2, 0, 3, 0, 0, 3, 0, 2, 0}
var playerWaterSprites = [10][2]uint8{{25, 0}, {25, 0}, {25, 0}, {24, 0}, {24, 0}, {24, 64}, {24, 64}, {25, 64}, {25, 64}, {25, 64}}
var playerWaterRecoilSprites = [10][2]uint8{{27, 0}, {28, 0}, {29, 0}, {24, 0}, {24, 0}, {24, 64}, {24, 64}, {29, 64}, {28, 64}, {27, 64}}

func (p *Player) faceAim() {
	p.SpriteFlip &= 0x3f
	if p.Motion.Aim >= 5 {
		p.SpriteFlip |= 0x40
	}
}

func (p *Player) animatePlayer(c *PlayerStepContext, sounds *[]uint8) error {
	switch {
	case p.Water != 0:
		p.animateWater(c)
	case p.Fall != 0:
		p.Sprite = 5
		p.faceAim()
	case p.Motion.Jump != 0:
		if p.Motion.AnimationFrame >= 4 {
			return errors.New("invalid jumping animation frame")
		}
		flip := uint8(0)
		if p.Motion.Jump&0x80 != 0 {
			flip = 0x40
		}
		p.SpriteFlip &= 0x3f
		if p.Motion.AnimationFrame >= 2 {
			p.SpriteFlip |= 0xc0
		}
		p.SpriteFlip ^= flip
		p.Sprite = [4]uint8{8, 9, 8, 9}[p.Motion.AnimationFrame]
		p.SpecialSpriteTimer++
		if p.SpecialSpriteTimer >= 5 {
			p.SpecialSpriteTimer = 0
			p.Motion.AnimationFrame++
			if p.Motion.AnimationFrame >= 4 {
				p.Motion.AnimationFrame = 0
			}
		}
	case c.World.SceneType != 0:
		if err := p.animateBase(c.World.SceneType&0x80 != 0, sounds); err != nil {
			return err
		}
	default:
		if p.Sequence < 3 {
			p.Sprite = [3]uint8{15, 22, 23}[p.Sequence]
		} else if p.Sequence == 3 {
			if p.Motion.Aim >= 10 {
				return errors.New("invalid walking aim")
			}
			sequence := playerAimSequence[p.Motion.Aim]
			if sequence == 0 && p.Recoil != 0 {
				sequence = 1
			}
			if err := p.animateWalk(sequence); err != nil {
				return err
			}
		} else if p.Sequence == 4 {
			p.SpecialSpriteTimer++
			if p.SpecialSpriteTimer&7 == 0 {
				p.Motion.AnimationFrame++
				if p.Motion.AnimationFrame >= 5 {
					p.Motion.AnimationFrame = 4
				}
			}
			if p.Motion.AnimationFrame >= 5 {
				return errors.New("invalid dying animation frame")
			}
			frame := [5][2]uint8{{10, 0}, {11, 0}, {10, 192}, {11, 192}, {12, 0}}[p.Motion.AnimationFrame]
			p.Sprite, p.SpriteFlip = frame[0], frame[1]
			if p.Death&2 != 0 {
				p.SpriteFlip ^= 0x40
			}
		} else {
			return errors.New("outdoor normal player has unsupported animation sequence")
		}
		if p.Sequence != 4 {
			p.faceAim()
		}
	}
	p.publishSprite(c.Actor, c.World.Frame)
	return nil
}

func (p *Player) animateWalk(sequence uint8) error {
	if p.Motion.AnimationFrame >= 6 || sequence >= 5 {
		return errors.New("invalid walking animation frame")
	}
	p.Sprite = playerWalkSprites[sequence][p.Motion.AnimationFrame]
	p.Motion.AnimationTimer++
	if p.Motion.AnimationTimer&7 == 0 {
		p.Motion.AnimationFrame++
		if p.Motion.AnimationFrame >= 6 {
			p.Motion.AnimationFrame = 0
		}
	}
	p.faceAim()
	return nil
}

func (p *Player) animateBase(boss bool, sounds *[]uint8) error {
	if boss {
		switch p.Sequence {
		case 0, 1, 2:
			p.SpriteFlip &= 0x3f
			p.Sprite = 0x50
		case 3:
			if p.Motion.Aim >= 10 {
				return errors.New("invalid boss walking aim")
			}
			sequence := playerAimSequence[p.Motion.Aim]
			if sequence == 0 && p.Recoil != 0 {
				sequence = 1
			}
			return p.animateWalk(sequence)
		case 4:
			p.animateBaseDeath()
		default:
			return errors.New("invalid boss animation sequence")
		}
		return nil
	}
	switch p.Sequence {
	case 0:
		p.Sprite = 0x50
	case 1:
		p.Sprite = 0x55
	case 2:
		p.Sprite = 0x54
	case 3, 4:
		sequence := uint8(0)
		if p.Recoil != 0 {
			sequence = 4
		}
		return p.animateWalk(sequence)
	case 5:
		p.Motion.AnimationTimer--
		if int8(p.Motion.AnimationTimer) < 0 {
			*sounds = append(*sounds, 0x0b)
			p.Motion.AnimationTimer = 10
			p.Motion.AnimationFrame++
		}
		p.Sprite = 0x57 + (p.Motion.AnimationFrame & 1)
	case 6:
		p.animateBaseDeath()
	case 7:
		p.Sprite = 0x91
	default:
		return errors.New("invalid base animation sequence")
	}
	return nil
}
func (p *Player) animateBaseDeath() {
	p.Sprite = 0x56
	if p.SpecialSpriteTimer < 27 {
		p.SpecialSpriteTimer++
		p.Sprite = 0x55
	}
	p.SpriteFlip = 0
}

// Water animation changes gameplay position and movement gates. Keep it in the
// deterministic rule step, after ordinary motion, rather than in the renderer.
func (p *Player) animateWater(c *PlayerStepContext) {
	if p.Water&4 == 0 {
		if p.Water&0x10 == 0 {
			p.Motion.AnimationFrame = 0
			p.Sprite = 5
			p.setY(p.y() + 16)
			if p.Motion.Aim >= 5 {
				p.Water |= 2
			}
			p.WaterTimer = 16
			p.Water |= 0x90
		}
		p.VelocityX = 0
		if p.WaterTimer != 0 {
			if p.WaterTimer < 12 {
				p.Sprite = 0x73
				// The original compares the loaded $73 sprite code with $08,
				// not the timer; its following $18 assignment is unreachable.
			}
			p.waterTransitionTick()
			return
		}
	}
	if p.Water&8 != 0 {
		p.VelocityX = 6
		if p.WaterTimer != 0 {
			if p.WaterTimer < 5 {
				p.Sprite = 5
			}
			p.waterTransitionTick()
			return
		}
		p.Water, p.Motion.AnimationFrame = 0, 0
		p.setY(p.y() - 16)
		return
	}
	p.Water = (p.Water | 4) & 0x7f
	sprite := playerWaterSprites[p.Motion.Aim]
	if p.Recoil != 0 {
		sprite = playerWaterRecoilSprites[p.Motion.Aim]
	}
	p.Sprite = sprite[0]
	p.SpriteFlip = p.SpriteFlip&15 | sprite[1]
	if c.World.Frame&15 == 0 {
		p.Motion.AnimationFrame++
	}
	if p.Motion.AnimationFrame&1 == 0 {
		p.SpriteFlip |= 8
	} else {
		p.SpriteFlip &= 0xf7
	}
	if c.sampleTerrain(p.X, p.y()).Kind != TerrainWater {
		p.Water |= 0x88
		p.WaterTimer = 12
		p.Sprite = 0x1a
		if p.Motion.Aim >= 5 {
			p.Water |= 2
		} else {
			p.Water &= 0xfd
		}
	}
}
func (p *Player) waterTransitionTick() {
	p.WaterTimer--
	if p.Water&2 != 0 {
		p.SpriteFlip |= 0x40
	}
}

func (p *Player) publishSprite(actor int, frame uint8) {
	p.DisplaySprite = p.Sprite
	if p.hidden() || p.SpawnInvincible != 0 && frame&1 != 0 {
		p.DisplaySprite = 0
	}
	attribute := [2]uint8{0, 5}[actor]
	mask := uint8(0)
	if p.Shock != 0 {
		mask = 2
	} else if p.Invincible != 0 {
		mask = 4
	}
	if mask != 0 {
		phase := frame
		if actor == 1 {
			phase ^= 0xff
		}
		attribute = 4
		if phase&mask != 0 {
			attribute = 5
		}
	}
	if p.SkipLedge&0x80 != 0 {
		attribute |= 0x20
	}
	if p.Recoil >= 12 {
		attribute |= 8
	}
	p.DisplayAttribute = p.SpriteFlip&0xc8 | attribute
}
