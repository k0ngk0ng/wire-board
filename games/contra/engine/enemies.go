package contra

import "errors"

// Enemy is a gameplay entity, including original hostile bullets and pickups.
// The last four bytes are a per-kind state payload: each recovered kind owns
// their meaning (for example bullet variant or runner facing/jump counters).
// This is not a CPU address space and carries no executable ROM state.
type Enemy struct {
	Sprite, Y, X, SpriteAttributes, Routine uint8
	FractionY, FractionX                    uint8
	VelocityY, VelocityX                    int16
	Kind, AnimationDelay, VariableA         uint8
	AttackDelay, Frame, HP                  uint8
	ScoreCollision, StateWidth, Attributes  uint8
	Data                                    [4]uint8
}

// EnemySystem retains inactive slot contents because original allocation and
// replacement rules inspect selected fields independently of slot activity.
type EnemySystem struct {
	Base                                           BaseEnemyState
	BossPartsDestroyed, AlienAimIndex, JointScroll uint8
	// Compatibility effect consumed by Tick in actual entity order. Producers
	// are explicit recovered operations, never a generic CPU/RAM register file.
	GenerationBiasChanged bool
	GenerationBiasValue   uint8
	Generator             SoldierGenerator
	Slots                 [16]Enemy
	ScreenReadOffset      uint8
	// Original sound requests produced by entity phases. The world drains these
	// into its authoritative event stream after the complete enemy phase.
	PendingSounds []uint8
}

type enemyProperties struct {
	StateWidth, ScoreCollision, HP, VariableA uint8
}

type EnemySpawnContext struct {
	ScreenCleared                    *uint8
	Level, SceneType, Screen, Scroll uint8
	Vertical                         bool
}

func (s *EnemySystem) recordGenerationBias(value uint8) {
	s.GenerationBiasChanged, s.GenerationBiasValue = true, value
}

// Initialize follows fixed-bank $EDAD. Clearing creates a clean semantic
// entity, preserving the requested kind and assigning its original properties.
func (s *EnemySystem) Initialize(slot int, kind, level uint8) error {
	if slot < 0 || slot >= len(s.Slots) || level >= 8 || int(kind) >= len(allEnemyProperties[level]) {
		return errors.New("enemy initialization has not been recovered for this kind/level")
	}
	properties := allEnemyProperties[level][kind]
	s.Slots[slot] = Enemy{Kind: kind, Routine: 1, Sprite: 1, StateWidth: properties.StateWidth,
		ScoreCollision: properties.ScoreCollision, HP: properties.HP, VariableA: properties.VariableA}
	return nil
}

func (s *EnemySystem) freeSlot() int {
	for i := len(s.Slots) - 1; i >= 0; i-- {
		if s.Slots[i].Routine == 0 {
			return i
		}
	}
	return -1
}

func (s *EnemySystem) replaceableBulletSlot() int {
	for i := len(s.Slots) - 1; i >= 0; i-- {
		if s.Slots[i].Kind == 1 {
			return i
		}
	}
	return 0
}

// SpawnScreen follows bank4/5 $A97B and $AA11. One outdoor trigger group is
// consumed per call, including repetitions that could not acquire a slot.
// Odd trigger bytes mark mandatory entities: those may replace an enemy bullet
// or, if none exists, slot zero. Camera progression remains the world's job.
func (s *EnemySystem) SpawnScreen(c EnemySpawnContext) error {
	if c.Level >= 8 || int(c.Screen) >= len(allEnemyScripts[c.Level]) || (c.SceneType != 0) != (c.Level == 1 || c.Level == 3) {
		return errors.New("enemy screen script has not been recovered for this scene")
	}
	script := allEnemyScripts[c.Level][c.Screen]
	if c.SceneType != 0 {
		return s.spawnIndoorScreen(c, script)
	}
	offset := int(s.ScreenReadOffset)
	if offset >= len(script) {
		return errors.New("enemy screen cursor is outside its script")
	}
	trigger := script[offset]
	if trigger == 0xff || trigger&0xfe > c.Scroll {
		return nil
	}
	if offset+2 >= len(script) {
		return errors.New("truncated enemy screen trigger")
	}
	distance := c.Scroll - (trigger & 0xfe)
	typeAndCount := script[offset+1]
	kind, count := typeAndCount&0x3f, int(typeAndCount>>6)+1
	if offset+2+count > len(script) {
		return errors.New("truncated repeated enemy group")
	}
	for _, position := range script[offset+2 : offset+2+count] {
		slot := s.freeSlot()
		if slot < 0 {
			if trigger&1 == 0 {
				continue
			}
			slot = s.replaceableBulletSlot()
		}
		if err := s.Initialize(slot, kind, c.Level); err != nil {
			return err
		}
		enemy := &s.Slots[slot]
		enemy.Attributes = position & 15
		if c.Vertical {
			enemy.X, enemy.Y = position&0xf0, distance
		} else {
			enemy.Y, enemy.X = position&0xf0, 0xf0-distance
		}
	}
	s.ScreenReadOffset = uint8(offset + 2 + count)
	return nil
}
