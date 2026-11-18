package main

import (
	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
	"math"
)

type compactEnemyEffects struct {
	HP                    float64            `json:"health,omitempty"`
	MaxHP                 float64            `json:"maxHealth,omitempty"`
	Armor                 float64            `json:"armor,omitempty"`
	MaxArmor              float64            `json:"maxArmor,omitempty"`
	Shield                float64            `json:"shield,omitempty"`
	BaseSpeed             float64            `json:"baseSpeed,omitempty"`
	Invisible             bool               `json:"invisible,omitempty"`
	CanStun               bool               `json:"canStun,omitempty"`
	CanSlow               bool               `json:"canSlow,omitempty"`
	CanBurn               bool               `json:"canBurn,omitempty"`
	CanPoison             bool               `json:"canPoison,omitempty"`
	Boss                  bool               `json:"boss,omitempty"`
	PassiveID             string             `json:"passiveId,omitempty"`
	WeatherID             string             `json:"weatherId,omitempty"`
	Regeneration          float64            `json:"regeneration,omitempty"`
	ReceivedHits          int                `json:"receivedHits,omitempty"`
	LivingEnemies         int                `json:"livingEnemies,omitempty"`
	LunarActivated        bool               `json:"lunarActivated,omitempty"`
	PassiveElapsedMS      int64              `json:"passiveElapsedMs,omitempty"`
	CriticalBlocks        int                `json:"criticalBlocks,omitempty"`
	StunApplications      uint64             `json:"stunApplications,omitempty"`
	HealingEvents         uint64             `json:"healingEvents,omitempty"`
	RegenerationElapsedMS int64              `json:"regenerationElapsedMS,omitempty"`
	SynthesisElapsedMS    int64              `json:"synthesisElapsedMS,omitempty"`
	WeatherElapsedMS      int64              `json:"weatherElapsedMS,omitempty"`
	WorldSandMultiplier   float64            `json:"worldSandMultiplier,omitempty"`
	TemporaryStopMS       int64              `json:"temporaryStopMs,omitempty"`
	InvulnerableMS        int64              `json:"invulnerableMs,omitempty"`
	PassiveFlags          map[string]bool    `json:"passiveFlags,omitempty"`
	PassiveTimers         map[string]int64   `json:"passiveTimers,omitempty"`
	PassiveValues         map[string]float64 `json:"passiveValues,omitempty"`
	PassiveCounters       map[string]int     `json:"passiveCounters,omitempty"`
	Conditions            []engine.Condition `json:"conditions,omitempty"`
	LastIncomingDamage    float64            `json:"lastIncomingDamage,omitempty"`
	LastPreHitHP          float64            `json:"lastPreHitHp,omitempty"`
	LastResolvedDamage    float64            `json:"lastResolvedDamage,omitempty"`
	LastHPDamage          float64            `json:"lastHPDamage,omitempty"`
	LastArmorDamage       float64            `json:"lastArmorDamage,omitempty"`
	LastShieldDamage      float64            `json:"lastShieldDamage,omitempty"`
	LastDamageSourceID    int                `json:"lastDamageSourceID,omitempty"`
	LastDamageKind        string             `json:"lastDamageKind,omitempty"`
	LastHitEvaded         bool               `json:"lastHitEvaded,omitempty"`
	LastHitAbilityID      string             `json:"lastHitAbilityID,omitempty"`
	LastHitItemID         string             `json:"lastHitItemID,omitempty"`
	LastHitTerrain        int                `json:"lastHitTerrain,omitempty"`
	LastHitCritical       bool               `json:"lastHitCritical,omitempty"`
	CloudNine             bool               `json:"cloudNine,omitempty"`
}

type compactEnemyIdentity struct {
	engine.EnemyEffectState
	ID                 int               `json:"id,omitempty"`
	Key                string            `json:"key,omitempty"`
	Species            string            `json:"species,omitempty"`
	X                  float64           `json:"x,omitempty"`
	Y                  float64           `json:"y,omitempty"`
	PathVariant        string            `json:"pathVariant,omitempty"`
	PathOriginWaypoint int               `json:"pathOriginWaypoint,omitempty"`
	PathMirrored       bool              `json:"pathMirrored,omitempty"`
	PathIndex          int               `json:"pathIndex,omitempty"`
	Waypoint           int               `json:"waypoint,omitempty"`
	Distance           float64           `json:"distance,omitempty"`
	Power              int               `json:"power,omitempty"`
	Gold               int64             `json:"gold,omitempty"`
	Entered            bool              `json:"entered,omitempty"`
	Dead               bool              `json:"dead,omitempty"`
	DeathRemainingMS   int64             `json:"deathRemainingMS,omitempty"`
	Escaped            bool              `json:"escaped,omitempty"`
	ReviveRemainingMS  int64             `json:"reviveRemainingMS,omitempty"`
	ReviveHP           float64           `json:"reviveHP,omitempty"`
	Revived            bool              `json:"revived,omitempty"`
	SpawnAt            int64             `json:"spawnAt,omitempty"`
	AgeMS              int64             `json:"ageMS,omitempty"`
	LastHitBy          int               `json:"lastHitBy,omitempty"`
	Sprite             engine.SpriteSpec `json:"sprite,omitempty"`
}

func enemyNegativeZeros(enemy *engine.Enemy) []string {
	fields := [...]struct {
		key   string
		value float64
	}{
		{"health", enemy.HP},
		{"maxHealth", enemy.MaxHP},
		{"armor", enemy.Armor},
		{"maxArmor", enemy.MaxArmor},
		{"shield", enemy.Shield},
		{"baseSpeed", enemy.BaseSpeed},
		{"regeneration", enemy.Regeneration},
		{"worldSandMultiplier", enemy.WorldSandMultiplier},
		{"lastIncomingDamage", enemy.LastIncomingDamage},
		{"lastPreHitHp", enemy.LastPreHitHP},
		{"lastResolvedDamage", enemy.LastResolvedDamage},
		{"lastHPDamage", enemy.LastHPDamage},
		{"lastArmorDamage", enemy.LastArmorDamage},
		{"lastShieldDamage", enemy.LastShieldDamage},
		{"x", enemy.X},
		{"y", enemy.Y},
		{"distance", enemy.Distance},
		{"reviveHP", enemy.ReviveHP},
	}
	var result []string
	for _, field := range fields {
		if field.value == 0 && math.Signbit(field.value) {
			result = append(result, field.key)
		}
	}
	return result
}
