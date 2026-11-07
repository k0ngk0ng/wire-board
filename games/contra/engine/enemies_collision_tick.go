package contra

import (
	"errors"
	"fmt"
)

// Tick is the original E54F enemy phase with stage-specific entity dispatch.
// It owns this phase's temporary sound queue: stale requests are discarded at
// entry; each AI's new requests are consumed before that slot's contact sounds.
// The caller owns a transactional world copy, graphics queue and terrain data.
func (s *EnemySystem) Tick(c *EnemyStepContext, collisions *EnemyCollisionContext) (EnemyCollisionResult, error) {
	if c == nil || collisions == nil || collisions.Players == nil || collisions.Weapons == nil || collisions.Scores == nil || c.Terrain == nil {
		return EnemyCollisionResult{}, errors.New("incomplete enemy phase context")
	}
	if c.Level >= 8 || (c.SceneType != 0) != (c.Level == 1 || c.Level == 3) || collisions.Level != c.Level || collisions.SceneType != c.SceneType {
		return EnemyCollisionResult{}, errors.New("enemy phase dispatcher has not been recovered for this level")
	}
	result := EnemyCollisionResult{Flash: collisions.Flash}
	var inherited BaseThrowWork
	if c.ThrowWork != nil {
		inherited = *c.ThrowWork
	}
	if c.InheritedThrowAttribute != nil {
		inherited.record(*c.InheritedThrowAttribute)
	}
	consumeHeight := func(part EnemyCollisionResult) {
		if part.HitboxHeightWritten {
			inherited.record(part.HitboxHeight)
		}
	}
	s.PendingSounds = nil
	control := func(event BossControl) error {
		if event.Kind == BossAudioReset {
			result.Sounds = nil
			result.AudioReset = true
			s.PendingSounds = nil
		}
		if c.OnBossControl != nil {
			return c.OnBossControl(event)
		}
		if event.Kind == BossAudioReset {
			return nil
		}
		if c.OnBossDoorSignal == nil {
			return errors.New("missing authoritative Boss progression callback")
		}
		switch event.Kind {
		case BossBeginDefeat:
			return c.OnBossDoorSignal(BossDoorDefeated)
		case BossExitReady:
			if event.Delay == 0x30 {
				return c.OnBossDoorSignal(BossDoorTunnelFinished)
			}
			if event.Delay == 0x60 {
				return c.OnBossDoorSignal(BossBaseFinished)
			}
		}
		return errors.New("unsupported legacy Boss progression event")
	}
	// The value is owned by the upstream frame work-state. Only writes that
	// actually occur in this phase may replace it; old notifications are stale.
	s.GenerationBiasChanged = false
	consumeBias := func() {
		if s.GenerationBiasChanged {
			c.SpawnVariantSeed = s.GenerationBiasValue
			s.GenerationBiasChanged = false
		}
	}
	for i := range collisions.Players {
		collisions.Players[i].OnEnemy = 0
	}
	refreshTargets := func() {
		for actor, p := range collisions.Players {
			c.Players[actor] = EnemyTarget{State: p.State, X: p.X, Y: p.y(), GameOver: p.GameOver}
			c.PlayerWeapons[actor] = p.Weapon
		}
	}
	for slot := len(s.Slots) - 1; slot >= 0; slot-- {
		if s.Slots[slot].Routine == 0 {
			continue
		}
		refreshTargets()
		step := *c
		step.InheritedThrowAttribute = nil
		step.ThrowWork = &inherited
		if inherited.Known {
			step.InheritedThrowAttribute = &inherited.Value
		}
		step.OnBossControl = control
		step.SoundRequestCarry = func(code uint8, prefix EnemyAudioPrefix) (bool, error) {
			if c.SoundRequestCarry == nil {
				return false, errors.New("missing authoritative sound carry projection")
			}
			projected := EnemyAudioPrefix{Reset: result.AudioReset || prefix.Reset}
			if !prefix.Reset {
				projected.Sounds = append(projected.Sounds, result.Sounds...)
			}
			projected.Sounds = append(projected.Sounds, prefix.Sounds...)
			return c.SoundRequestCarry(code, projected)
		}
		if c.SharedRandom != nil {
			step.Random = c.SharedRandom.Value
		}
		step.ShooterTargetAt = func(index uint8) (EnemyTarget, error) {
			return JungleShooterTarget(index, step.Players, collisions.Weapons)
		}
		kind := s.Slots[slot].Kind
		bossSignal, err := s.stepEntity(slot, step)
		consumeBias()
		if err == nil && bossSignal != BossDoorNoSignal {
			if bossSignal == BossDoorDefeated {
				// F9D4 occurs inside the Boss body. Its PendingSounds already
				// contain only post-reset requests; earlier slots must be erased.
				pending := s.PendingSounds
				err = control(BossControl{Kind: BossAudioReset})
				s.PendingSounds = pending
			}
			if err == nil {
				switch bossSignal {
				case BossDoorDefeated:
					err = control(BossControl{Kind: BossBeginDefeat})
				case BossDoorTunnelFinished:
					err = control(BossControl{Kind: BossExitReady, Delay: 0x30})
				case BossBaseFinished:
					err = control(BossControl{Kind: BossExitReady, Delay: 0x60})
				}
			}
		}
		result.Sounds = append(result.Sounds, s.PendingSounds...)
		s.PendingSounds = nil
		if err != nil {
			return result, fmt.Errorf("enemy slot %d type %d: %w", slot, kind, err)
		}
		// Re-read the live slot after AI: it may have changed kind, removed
		// itself or spawned another entity into a later-to-be-visited slot.
		if s.Slots[slot].Sprite == 0 {
			continue
		}
		e := &s.Slots[slot]
		if (step.SceneType&1 == 0 || e.Y >= 0x9c) && e.StateWidth&1 == 0 {
			part, err := s.CollidePlayers(slot, collisions)
			if err != nil {
				return result, fmt.Errorf("enemy slot %d player contact: %w", slot, err)
			}
			mergeEnemyPhaseResult(&result, part)
			consumeHeight(part)
			collisions.Flash = part.Flash
			consumeBias()
		}
		// Original E54F does not repeat the Sprite check after player contact.
		// It only checks the current state-width bit before bullet collision.
		if s.Slots[slot].StateWidth&0x80 == 0 {
			part, err := s.CollideProjectiles(slot, collisions)
			if err != nil {
				return result, fmt.Errorf("enemy slot %d projectile contact: %w", slot, err)
			}
			mergeEnemyPhaseResult(&result, part)
			consumeHeight(part)
			collisions.Flash = part.Flash
			consumeBias()
		}
	}
	// The following screen/soldier generation phases may reuse this context.
	// Publish targets after the last contact as well, including an empty phase.
	refreshTargets()
	if c.ThrowWork != nil {
		*c.ThrowWork = inherited
	}
	return result, nil
}

func mergeEnemyPhaseResult(result *EnemyCollisionResult, part EnemyCollisionResult) {
	if part.HitboxHeightWritten {
		result.HitboxHeightWritten, result.HitboxHeight = true, part.HitboxHeight
	}
	if part.AudioReset {
		result.Sounds = nil
		result.AudioReset = true
	}
	result.Sounds = append(result.Sounds, part.Sounds...)
	result.PlayerHits = append(result.PlayerHits, part.PlayerHits...)
	result.Destroyed = append(result.Destroyed, part.Destroyed...)
	result.Pickups = append(result.Pickups, part.Pickups...)
	result.Scores = append(result.Scores, part.Scores...)
	result.Projectiles = append(result.Projectiles, part.Projectiles...)
	result.Flash = part.Flash
}
