package contra

import "errors"

// ArcadeScores are the original game's binary score counters, in hundreds of
// displayed points. They are unrelated to the platform's cooperative ledger.
type ArcadeScores struct {
	Hundreds  [2]uint16
	HighScore uint16
	NextLife  [2]uint16
}

func NewArcadeScores() ArcadeScores {
	return ArcadeScores{HighScore: 200, NextLife: [2]uint16{200, 200}}
}

type EnemyCollisionContext struct {
	Players                 *[2]Player
	Inputs                  [2]PlayerInput
	Weapons                 *WeaponSystem
	Scores                  *ArcadeScores
	Level, SceneType, Flash uint8
	Demo                    bool
}

type ArcadeScoreEvent struct {
	Actor         int
	HundredsAdded uint16
	Total         uint16
}
type EnemyPickupEvent struct {
	Actor int
	Item  uint8
}
type EnemyProjectileContact struct{ Enemy, SourceProjectile, ConsumedProjectile, Owner int }
type EnemyCollisionResult struct {
	HitboxHeightWritten bool
	HitboxHeight        uint8
	// AudioReset asks the frame owner to discard earlier sounds, including
	// player-phase requests, when an original Boss transition resets the APU.
	AudioReset  bool
	Sounds      []uint8
	PlayerHits  []int
	Destroyed   []int
	Pickups     []EnemyPickupEvent
	Scores      []ArcadeScoreEvent
	Projectiles []EnemyProjectileContact
	Flash       uint8
}

type enemyHitbox struct{ Y, X, Height, Width uint8 }

func (s *EnemySystem) validateCollision(slot int, c *EnemyCollisionContext, projectiles bool) error {
	if c == nil || c.Players == nil || c.Scores == nil || projectiles && c.Weapons == nil || slot < 0 || slot >= len(s.Slots) || c.Level >= 8 {
		return errors.New("invalid enemy collision context")
	}
	e := &s.Slots[slot]
	if int(e.Kind) >= len(collisionDeathRoutine[c.Level]) || e.ScoreCollision>>4 > 10 {
		return errors.New("enemy collision metadata has not been recovered")
	}
	return nil
}

// hitbox follows fixed $E342. The tables describe an enemy expanded for the
// current player pose or bullet; all additions retain original byte wrapping.
func (e *Enemy) hitbox(profile uint8) enemyHitbox {
	code := e.ScoreCollision & 15
	if code < 15 {
		v := collisionBoxes[profile][code]
		return enemyHitbox{Y: e.Y + v[0], X: e.X + v[1], Height: v[2], Width: v[3]}
	}
	length := e.Data[0]
	if e.Attributes&0x40 != 0 {
		length = -length
	}
	length += 8
	v := collisionVariableBase[profile]
	adjust := collisionVariableAdjust[(e.Attributes>>6)&3]
	for i, value := range adjust {
		if value == 0xfe {
			value = -length
		} else if value == 0xff {
			value = length
		}
		v[i] += value
	}
	return enemyHitbox{Y: e.Y + v[0], X: e.X + v[1], Height: v[2], Width: v[3]}
}

func byteDistance(a, b uint8) uint8 {
	if a >= b {
		return a - b
	}
	return b - a
}

// CollidePlayers is the admitted $E10A contact phase. The frame runner first
// applies the original Sprite/StateWidth gates after AI, then calls this method.
// It checks the second actor first, preserving original early-return behavior.
func (s *EnemySystem) CollidePlayers(slot int, c *EnemyCollisionContext) (EnemyCollisionResult, error) {
	if err := s.validateCollision(slot, c, false); err != nil {
		return EnemyCollisionResult{}, err
	}
	result := EnemyCollisionResult{Flash: c.Flash}
	e := &s.Slots[slot]
	for actor := 1; actor >= 0; actor-- {
		p := &c.Players[actor]
		if p.State != 1 {
			continue
		}
		if c.SceneType&1 != 0 {
			if e.Kind == 1 && p.Sequence == 2 {
				continue
			}
		} else if e.StateWidth&0x40 != 0 {
			if e.StateWidth&0x20 != 0 {
				if e.Y >= p.y() && e.Y-p.y() >= 8 {
					e.StateWidth &^= 0x10
				} else {
					e.StateWidth |= 0x10
				}
			} else if p.Motion.Jump != 0 && p.Motion.Velocity < 256 {
				continue
			}
		}
		profile := uint8(0)
		if p.Water != 0 {
			if c.Inputs[actor].Held&ButtonDown != 0 {
				continue
			}
		} else if p.Motion.Jump != 0 {
			profile = 1
		} else if p.Sprite == 0x17 {
			profile = 2
		} else {
			profile = 3
		}
		box := e.hitbox(profile)
		result.HitboxHeightWritten, result.HitboxHeight = true, box.Height
		if p.y()-box.Y >= box.Height || p.X-box.X >= box.Width || byteDistance(e.X, p.X) >= 0x80 {
			continue
		}
		if e.StateWidth&0x40 != 0 && e.StateWidth&0x10 == 0 {
			if p.y() < p.FallLockY {
				continue
			}
			e.Frame = 1
			fraction := uint16(uint8(e.VelocityX)) + uint16(e.FractionX)
			p.BoostX = int8(uint8(e.VelocityX>>8) + uint8(fraction>>8) + uint8(p.BoostX))
			offset := uint8(0xe4)
			if e.StateWidth&0x20 != 0 {
				offset = 0xe8
			}
			p.setY(e.Y + offset)
			p.OnEnemy = 1
			p.land(&result.Sounds)
			continue
		}
		if e.Kind == 0 {
			result.addScore(c, actor, 10, false)
			result.Sounds = append(result.Sounds, 0x38)
			item := e.Attributes & 7
			result.Pickups = append(result.Pickups, EnemyPickupEvent{Actor: actor, Item: item})
			switch {
			case item == 0:
				p.Weapon |= 0x10
			case item < 5:
				mask := uint8(0xf0)
				if p.Weapon&15 != item {
					mask = 0xe0
				}
				p.Weapon = p.Weapon&mask | item
			case item == 5:
				p.Invincible = 0x80
				if c.Level == 6 {
					p.Invincible = 0x90
				}
			default:
				s.falcon(c, &result)
				result.Flash = 0x20
			}
			e.remove()
			return result, nil
		}
		if p.SpawnInvincible != 0 {
			continue
		}
		if p.Invincible != 0 {
			if e.HP != 0 && e.HP < 0xf0 {
				e.HP = 0
				s.rewardKill(slot, actor, c, &result)
			}
			return result, nil
		}
		result.Sounds = append(result.Sounds, p.Hit()...)
		result.PlayerHits = append(result.PlayerHits, actor)
		if e.Kind == 1 {
			e.remove()
		}
		return result, nil
	}
	return result, nil
}

// CollideProjectiles reproduces $E276's descending scan and at-most-one hit.
// Its SBC edges intentionally differ from a conventional rectangle predicate.
func (s *EnemySystem) CollideProjectiles(slot int, c *EnemyCollisionContext) (EnemyCollisionResult, error) {
	if err := s.validateCollision(slot, c, true); err != nil {
		return EnemyCollisionResult{}, err
	}
	result := EnemyCollisionResult{Flash: c.Flash}
	e := &s.Slots[slot]
	box := e.hitbox(4)
	result.HitboxHeightWritten, result.HitboxHeight = true, box.Height
	for index := len(c.Weapons.Projectiles) - 1; index >= 0; index-- {
		p := &c.Weapons.Projectiles[index]
		if p.Sprite == 0 || p.Routine != 1 {
			continue
		}
		borrow := uint8(1)
		if c.SceneType&1 != 0 {
			if e.StateWidth&0x20 != 0 {
				if p.Kind&0x80 == 0 {
					continue
				}
				borrow = 0
			} else if p.Timer >= 2 {
				continue
			}
		}
		if p.Y-box.Y-borrow >= box.Height || p.X-box.X-1 >= box.Width || byteDistance(e.X, p.X) >= 0x80 {
			continue
		}
		if p.Owner > 1 {
			return result, errors.New("invalid projectile owner")
		}
		if e.HP != 0 && e.HP < 0xf0 {
			e.HP--
			if e.HP == 0 {
				s.rewardKill(slot, int(p.Owner), c, &result)
			} else if e.StateWidth&4 != 0 {
				if int(e.VariableA) >= len(collisionHitSounds) {
					return result, errors.New("unknown enemy hit sound")
				}
				result.Sounds = append(result.Sounds, collisionHitSounds[e.VariableA])
			}
		}
		consumed := index
		if p.Kind == 5 {
			start := 0
			if index >= 10 {
				start = 10
			}
			for i := start; i < index; i++ {
				if c.Weapons.Projectiles[i].Routine == 1 {
					consumed = i
					break
				}
			}
		}
		c.Weapons.Projectiles[consumed].Hit()
		result.Projectiles = append(result.Projectiles, EnemyProjectileContact{Enemy: slot, SourceProjectile: index, ConsumedProjectile: consumed, Owner: int(p.Owner)})
		return result, nil
	}
	return result, nil
}

func (s *EnemySystem) rewardKill(slot, actor int, c *EnemyCollisionContext, result *EnemyCollisionResult) {
	e := &s.Slots[slot]
	code := e.ScoreCollision >> 4
	if code == 10 {
		result.addScore(c, actor, 5000, true)
	} else if value := collisionScores[code]; value != 0 {
		result.addScore(c, actor, value, false)
	}
	e.ScoreCollision &= 15
	s.markDestroyed(slot, c.Level)
	result.Destroyed = append(result.Destroyed, slot)
}
func (s *EnemySystem) markDestroyed(slot int, level uint8) {
	e := &s.Slots[slot]
	next := collisionDeathRoutine[level][e.Kind]
	if next >= e.Routine {
		e.Routine = next
	}
}
func (s *EnemySystem) falcon(c *EnemyCollisionContext, result *EnemyCollisionResult) {
	for i := len(s.Slots) - 1; i >= 0; i-- {
		e := &s.Slots[i]
		if e.Routine == 0 || e.Sprite == 0 || e.Kind == 2 || e.Kind == 3 || e.HP == 0 || e.HP == 0xf0 {
			continue
		}
		if int(e.Kind) >= len(collisionDeathRoutine[c.Level]) {
			continue
		}
		s.markDestroyed(i, c.Level)
		e.Attributes |= 0x80
		result.Destroyed = append(result.Destroyed, i)
	}
}

func (r *EnemyCollisionResult) addScore(c *EnemyCollisionContext, actor int, value uint16, special bool) {
	if c.Demo && !special {
		return
	}
	s := c.Scores
	old := s.Hundreds[actor]
	s.Hundreds[actor] = uint16(min(uint32(old)+uint32(value), 65535))
	if special || s.Hundreds[actor] >= s.NextLife[actor] && s.NextLife[actor] < 0x7500 {
		increment := uint32(300)
		if special {
			increment = 5000
		}
		s.NextLife[actor] = uint16(min(uint32(s.NextLife[actor])+increment, 65535))
		p := &c.Players[actor]
		p.Lives++
		if p.Lives >= 99 {
			p.Lives = 99
		}
		if !special {
			r.Sounds = append(r.Sounds, 0x39)
		}
	}
	if s.Hundreds[actor] >= s.HighScore {
		s.HighScore = s.Hundreds[actor]
	}
	r.Scores = append(r.Scores, ArcadeScoreEvent{Actor: actor, HundredsAdded: s.Hundreds[actor] - old, Total: s.Hundreds[actor]})
}
