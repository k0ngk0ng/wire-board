package contra

import "errors"

// LogicalSprite is one semantic render entry before meta-sprite composition.
type LogicalSprite struct{ Code, X, Y, Attributes uint8 }

// CombatFrame holds the player/projectile phase of a world tick. The scene
// runner separately owns controls/frame count, scrolling terrain, enemies and
// campaign transitions; their order follows the original level scheduler.
type CombatFrame struct {
	World          OutdoorWorld
	Players        [2]Player
	Weapons        WeaponSystem
	Terrain        CollisionWindow
	SolidCollision uint8
	ScreenCleared  uint8
	Display        [10]LogicalSprite
}

// AdvancePlayersAndProjectiles reproduces original $D296..D2D8. It commits only
// a complete phase; failed rule/phase validation preserves the caller's state.
func (f *CombatFrame) AdvancePlayersAndProjectiles(inputs [2]PlayerInput) ([]uint8, error) {
	result, err := f.AdvancePlayersAndProjectilesWithEffects(inputs)
	return result.Sounds, err
}

type CombatPhaseResult struct {
	Sounds                []uint8
	GenerationBiasChanged bool
	GenerationBiasValue   uint8
}

func (f *CombatFrame) AdvancePlayersAndProjectilesWithEffects(inputs [2]PlayerInput, work ...*TerrainWork) (CombatPhaseResult, error) {
	if len(work) > 1 {
		return CombatPhaseResult{}, errors.New("multiple terrain work owners")
	}
	var terrain *TerrainWork
	if len(work) == 1 {
		terrain = work[0]
	}
	return f.advanceWithWork(inputs, terrain, nil)
}
func (f *CombatFrame) advanceWithWork(inputs [2]PlayerInput, terrain *TerrainWork, inherited *BaseThrowWork) (CombatPhaseResult, error) {
	next := *f
	var local TerrainWork
	var owner *TerrainWork
	if terrain != nil {
		local = *terrain
		owner = &local
	}
	var throw BaseThrowWork
	var throwOwner *BaseThrowWork
	if inherited != nil {
		throw = *inherited
		throwOwner = &throw
	}
	var result CombatPhaseResult
	sounds, err := next.advanceCombat(inputs, &result, owner, throwOwner)
	if err != nil {
		return CombatPhaseResult{}, err
	}
	result.Sounds = sounds
	*f = next
	if owner != nil {
		*terrain = local
	}
	if throwOwner != nil {
		*inherited = throw
	}
	return result, nil
}

func (f *CombatFrame) advanceCombat(inputs [2]PlayerInput, effects *CombatPhaseResult, work *TerrainWork, throw *BaseThrowWork) ([]uint8, error) {
	w := &f.World
	w.FrameScroll, w.AttackEnabled, w.WeaponStrength = 0, 0, 0
	f.Players[0].Scrolling, f.Players[1].Scrolling = 0, 0
	mask := uint8(0)
	for actor := range f.Players {
		if f.Players[actor].GameOver == 0 {
			mask |= 1 << actor
		}
	}
	var sounds []uint8
	fire := func(request FireRequest) FireResult {
		request.Work = work
		request.ThrowWork = throw
		result := f.Weapons.Fire(request)
		if result.GenerationBiasChanged {
			effects.GenerationBiasChanged = true
			effects.GenerationBiasValue = result.GenerationBiasValue
		}
		return result
	}
	if mask != 0 {
		w.ActivePlayers = mask - 1
		first := int(w.ActivePlayers & 1)
		last := first
		if w.ActivePlayers == 2 {
			last = 1
		}
		for actor := first; actor <= last; actor++ {
			p := &f.Players[actor]
			context := PlayerStepContext{ThrowWork: throw, Actor: actor, Peer: &f.Players[1-actor], World: w, Terrain: &f.Terrain, Input: inputs[actor], PeerInput: inputs[1-actor], ScreenCleared: f.ScreenCleared, Fire: fire, Work: work}
			events, err := p.Step(&context)
			if err != nil {
				return nil, err
			}
			sounds = append(sounds, events...)
			if p.SpawnInvincible != 0 {
				p.SpawnInvincible--
			} else if p.State == 1 {
				w.AttackEnabled = 1
			}
			if p.Invincible != 0 && w.Frame&7 == 0 {
				p.Invincible--
			}
			if p.Recoil != 0 {
				p.Recoil--
			}
			p.BoostX = 0
			weapon := p.Weapon & 7
			if weapon >= 5 {
				return nil, errors.New("invalid player weapon strength")
			}
			w.WeaponStrength = max(w.WeaponStrength, [...]uint8{0, 2, 1, 3, 2}[weapon])
		}
		if w.ActivePlayers == 2 {
			f.correctPeerCamera()
		}
	}
	BorrowLife(&f.Players, inputs, w)
	if w.IndoorScroll >= 2 {
		w.IndoorScroll = 0
	}
	if w.AutoScroll[0]|w.AutoScroll[1] != 0 {
		w.FrameScroll = 1
	}
	f.Weapons.Step(BulletStep{ThrowWork: throw, StageType: w.SceneType, ScrollType: w.ScrollType, Scroll: w.FrameScroll,
		SolidCollision: f.SolidCollision&0x80 != 0, ScreenCleared: f.ScreenCleared != 0,
		Terrain: func(x, y uint8) TerrainKind {
			sample := f.Terrain.At(x, y, w.Scroll)
			if work != nil {
				work.RecordQuery(sample)
			}
			// Original weapon probe 87F7 calls DFB8: a platform backed by
			// solid terrain blocks the shot as solid, before projectile motion.
			if sample.Kind == TerrainPlatform {
				below := sample.SourceIndex&0xc0 | (sample.SourceIndex+4)&0x3f
				if (f.Terrain[below]>>(6-2*sample.Column))&3 == 3 {
					return TerrainSolid
				}
			}
			return sample.Kind
		}})
	for actor, p := range f.Players {
		f.Display[actor] = LogicalSprite{Code: p.DisplaySprite, X: p.X, Y: p.y(), Attributes: p.DisplayAttribute}
	}
	for slot := range 8 {
		p := f.Weapons.Projectiles[slot*2+int(w.Frame&1)]
		f.Display[slot+2] = LogicalSprite{Code: p.Sprite, X: p.X, Y: p.Y, Attributes: p.Attributes}
	}
	return sounds, nil
}

func (f *CombatFrame) correctPeerCamera() {
	w := &f.World
	if w.SceneType != 0 || f.Players[0].Scrolling == f.Players[1].Scrolling {
		return
	}
	follower := 0
	if f.Players[0].Scrolling > f.Players[1].Scrolling {
		follower = 1
	}
	p := &f.Players[follower]
	if w.ScrollType == 0 {
		// Original horizontal correction subtracts one even if frame scroll is
		// larger (for example on a moving platform). Preserve that behavior.
		p.X--
	} else if w.AutoScroll[0] == 0 {
		position := (p.Motion.Position + int32(w.FrameScroll)<<8) & 0xffffff
		if position&0x800000 != 0 {
			position -= 0x1000000
		}
		p.Motion.Position = position
	}
}
