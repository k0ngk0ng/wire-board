package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

type ConditionSpec struct {
	Kind             string  `json:"kind"`
	Chance           float64 `json:"chance"`
	DurationMS       int64   `json:"durationMs"`
	Magnitude        float64 `json:"magnitude"`
	Stacks           int     `json:"stacks"`
	IgnoreResistance bool    `json:"ignoreResistance,omitempty"`
	ResistantScale   float64 `json:"resistantScale,omitempty"`
	MinimumRoll      float64 `json:"minimumRoll,omitempty"`
	SourcePokemonID  int     `json:"sourcePokemonID,omitempty"`
}

type Condition struct {
	Kind            string  `json:"kind"`
	DurationMS      int64   `json:"durationMs"`
	RemainingMS     int64   `json:"remainingMs"`
	Magnitude       float64 `json:"magnitude"`
	Stacks          int     `json:"stacks"`
	TickElapsedMS   float64 `json:"tickElapsedMs"`
	SourcePokemonID int     `json:"sourcePokemonID,omitempty"`
	SourceItemID    string  `json:"sourceItemID,omitempty"`
}

type EnemyEffectState struct {
	HP                    float64            `json:"health"`
	MaxHP                 float64            `json:"maxHealth"`
	Armor                 float64            `json:"armor"`
	MaxArmor              float64            `json:"maxArmor"`
	Shield                float64            `json:"shield"`
	BaseSpeed             float64            `json:"baseSpeed"`
	Invisible             bool               `json:"invisible"`
	CanStun               bool               `json:"canStun"`
	CanSlow               bool               `json:"canSlow"`
	CanBurn               bool               `json:"canBurn"`
	CanPoison             bool               `json:"canPoison"`
	Boss                  bool               `json:"boss"`
	PassiveID             string             `json:"passiveId"`
	WeatherID             string             `json:"weatherId"`
	Regeneration          float64            `json:"regeneration"`
	ReceivedHits          int                `json:"receivedHits"`
	LivingEnemies         int                `json:"livingEnemies"`
	LunarActivated        bool               `json:"lunarActivated"`
	PassiveElapsedMS      int64              `json:"passiveElapsedMs"`
	CriticalBlocks        int                `json:"criticalBlocks"`
	StunApplications      uint64             `json:"stunApplications"`
	HealingEvents         uint64             `json:"healingEvents"`
	RegenerationElapsedMS int64              `json:"regenerationElapsedMS"`
	SynthesisElapsedMS    int64              `json:"synthesisElapsedMS"`
	WeatherElapsedMS      int64              `json:"weatherElapsedMS"`
	WorldSandMultiplier   float64            `json:"worldSandMultiplier"`
	TemporaryStopMS       int64              `json:"temporaryStopMs"`
	InvulnerableMS        int64              `json:"invulnerableMs"`
	PassiveFlags          map[string]bool    `json:"passiveFlags,omitempty"`
	PassiveTimers         map[string]int64   `json:"passiveTimers,omitempty"`
	PassiveValues         map[string]float64 `json:"passiveValues,omitempty"`
	PassiveCounters       map[string]int     `json:"passiveCounters,omitempty"`
	Conditions            []Condition        `json:"conditions"`
	LastIncomingDamage    float64            `json:"lastIncomingDamage"`
	LastPreHitHP          float64            `json:"lastPreHitHp"`
	LastResolvedDamage    float64            `json:"lastResolvedDamage"`
	LastHPDamage          float64            `json:"lastHPDamage"`
	LastArmorDamage       float64            `json:"lastArmorDamage"`
	LastShieldDamage      float64            `json:"lastShieldDamage"`
	LastDamageSourceID    int                `json:"lastDamageSourceID,omitempty"`
	LastDamageKind        string             `json:"lastDamageKind,omitempty"`
	LastHitEvaded         bool               `json:"lastHitEvaded"`
	LastHitAbilityID      string             `json:"lastHitAbilityID,omitempty"`
	LastHitItemID         string             `json:"lastHitItemID,omitempty"`
	LastHitTerrain        int                `json:"lastHitTerrain"`
	LastHitCritical       bool               `json:"lastHitCritical"`
	CloudNine             bool               `json:"cloudNine"`
}

func finiteNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func HasCondition(enemy *EnemyEffectState, kind string) bool {
	for _, condition := range enemy.Conditions {
		if condition.Kind == kind && condition.RemainingMS != 0 && condition.Stacks > 0 {
			return true
		}
	}
	return false
}

func ConditionStacks(enemy *EnemyEffectState, kind string) int {
	stacks := 0
	for _, condition := range enemy.Conditions {
		if condition.Kind == kind && condition.RemainingMS != 0 {
			stacks += condition.Stacks
		}
	}
	return stacks
}

func ConditionNominalDuration(enemy *EnemyEffectState, kind string) int64 {
	if enemy == nil {
		return 0
	}
	for _, condition := range enemy.Conditions {
		if condition.Kind == kind && condition.RemainingMS != 0 {
			if condition.DurationMS != 0 {
				return condition.DurationMS
			}
			return condition.RemainingMS
		}
	}
	return 0
}

func conditionAllowed(enemy *EnemyEffectState, kind string) bool {
	switch kind {
	case "burn", "scorch":
		return enemy.CanBurn
	case "poison":
		return enemy.CanPoison
	case "stun":
		return enemy.CanStun
	case "slow":
		return enemy.CanSlow
	case "curse":
		return enemy.PassiveID != "mummy"
	default:
		return true
	}
}

func conditionImmune(enemy *EnemyEffectState, kind string) bool {
	if kind == "nightmare" || kind == "curse" {
		return kind == "curse" && enemy.PassiveID == "mummy"
	}
	return enemy.PassiveID == "waterVeil" && (enemy.WeatherID == "rain" || enemy.WeatherID == "thunderStorm") || enemy.PassiveID == "leafGuard" && enemy.WeatherID == "harshSunlight" && !enemy.CloudNine
}

func ApplyConditions(enemy *EnemyEffectState, profile AttackProfile, roll float64) {
	if enemy == nil || enemy.HP <= 0 || enemy.InvulnerableMS > 0 || !finiteNumber(roll) || roll < 0 || roll >= 1 {
		return
	}
	for _, specification := range profile.Conditions {
		if specification.Chance <= 0 || roll < specification.MinimumRoll || roll >= specification.MinimumRoll+specification.Chance {
			continue
		}
		if conditionImmune(enemy, specification.Kind) {
			continue
		}
		allowed := conditionAllowed(enemy, specification.Kind)
		if !allowed && !specification.IgnoreResistance {
			continue
		}
		scale := 1.0
		if !allowed && specification.ResistantScale > 0 {
			scale = specification.ResistantScale
		}
		duration := specification.DurationMS
		magnitude := specification.Magnitude
		if specification.Kind == "stun" {
			duration = int64(float64(duration) * scale)
			if enemy.PassiveID == "electrogenesis" {
				duration *= 2
			}
		} else {
			magnitude *= scale
		}
		stacks := max(1, specification.Stacks)
		found := -1
		for index := range enemy.Conditions {
			if enemy.Conditions[index].Kind == specification.Kind {
				found = index
				break
			}
		}
		if found < 0 {
			if enemy.PassiveID == "vCrate" && (specification.Kind == "slow" || specification.Kind == "stun") {
				enemy.BaseSpeed -= 24
				continue
			}
			enemy.Conditions = append(enemy.Conditions, Condition{Kind: specification.Kind, DurationMS: duration, RemainingMS: duration, Magnitude: magnitude, Stacks: stacks, SourcePokemonID: specification.SourcePokemonID})
			if specification.Kind == "stun" {
				enemy.StunApplications++
			}
		} else {
			condition := &enemy.Conditions[found]
			if specification.Kind == "poison" || specification.Kind == "nightmare" {
				condition.Stacks += stacks
				condition.SourcePokemonID = specification.SourcePokemonID
				if specification.Kind == "nightmare" {
					condition.Magnitude = magnitude
				}
			} else if specification.Kind == "shadowTagSlow" || specification.Kind == "auraSlow" {
				condition.RemainingMS, condition.Magnitude = duration, magnitude
			}
		}
		if enemy.PassiveID == "justified" && specification.Kind != "curse" && specification.Kind != "nightmare" && specification.Kind != "auraSlow" {
			enemy.Armor += 5000
			enemy.MaxArmor += 5000
			if enemy.PassiveFlags == nil {
				enemy.PassiveFlags = map[string]bool{}
			}
			if enemy.PassiveCounters == nil {
				enemy.PassiveCounters = map[string]int{}
			}
			enemy.PassiveFlags["justifiedTriggered"] = true
			enemy.PassiveCounters["justified"]++
		}
		if found < 0 && enemy.PassiveID == "spectralThief" && (specification.Kind == "burn" || specification.Kind == "stun" || specification.Kind == "slow") {
			enemy.Invisible = true
		}
		if found < 0 && specification.Kind == "burn" && enemy.PassiveID == "magmaStorm" {
			enemy.MaxHP += enemy.Armor
			enemy.HP += enemy.Armor
		}
	}
}

func EffectiveEnemySpeed(enemy *EnemyEffectState) float64 {
	return enemySpeed(enemy, true)
}

func EffectiveEnemySpeedForDamage(enemy *EnemyEffectState) float64 {
	return enemySpeed(enemy, false)
}

func enemySpeed(enemy *EnemyEffectState, movement bool) float64 {
	if enemy == nil || enemy.HP <= 0 || !finiteNumber(enemy.BaseSpeed) {
		return 0
	}
	if movement && (HasCondition(enemy, "stun") || enemy.TemporaryStopMS > 0) {
		return 0
	}
	multiplier := 1.0
	for _, condition := range enemy.Conditions {
		if (condition.Kind == "slow" || condition.Kind == "shadowTagSlow" || condition.Kind == "auraSlow") && condition.RemainingMS != 0 {
			multiplier *= max(0, 1-condition.Magnitude)
		}
	}
	if enemy.WeatherID == "strongWinds" {
		multiplier *= 0.9
	}
	if !enemy.CloudNine && (enemy.PassiveID == "swiftSwim" && (enemy.WeatherID == "rain" || enemy.WeatherID == "thunderStorm") || enemy.PassiveID == "chlorophyll" && enemy.WeatherID == "harshSunlight") {
		multiplier *= 2
	}
	if enemy.PassiveID == "turboblaze" && HasCondition(enemy, "burn") {
		multiplier *= 3
	}
	if enemy.PassiveID == "luchadoro" && enemy.Armor <= 0 {
		multiplier *= 3
	}
	if enemy.PassiveValues["tailWindMultiplier"] > 0 {
		multiplier *= enemy.PassiveValues["tailWindMultiplier"]
	}
	if enemy.PassiveID == "heavyMetal" && multiplier > 1 {
		multiplier *= 1.5
	}
	return max(0, enemy.BaseSpeed*multiplier)
}

func ApplyDamage(enemy *EnemyEffectState, amount float64, kind string, armorPierce float64) float64 {
	loss, _ := applyDamageWithCredit(enemy, amount, kind, armorPierce)
	return loss
}

func applyDamageWithCredit(enemy *EnemyEffectState, amount float64, kind string, armorPierce float64) (float64, float64) {
	if enemy == nil || enemy.HP <= 0 || amount < 0 || !finiteNumber(amount) || !finiteNumber(armorPierce) {
		return 0, 0
	}
	enemy.LastIncomingDamage, enemy.LastResolvedDamage, enemy.LastHPDamage, enemy.LastArmorDamage, enemy.LastShieldDamage = amount, 0, 0, 0, 0
	enemy.LastPreHitHP = enemy.HP
	enemy.LastDamageKind, enemy.LastHitEvaded = kind, false
	if enemy.InvulnerableMS > 0 {
		return 0, 0
	}
	direct := kind == "direct" || kind == "critical"
	if kind == "critical" && enemy.PassiveID == "iceBody" {
		enemy.CriticalBlocks++
		return 0, 0
	}
	if enemy.PassiveID == "criticalLearning" {
		if kind != "critical" {
			return 0, 0
		}
		amount = 1
	}
	if enemy.PassiveID == "nebulogenesis" && direct {
		hit := enemy.ReceivedHits % 11
		enemy.ReceivedHits++
		if hit >= 8 {
			enemy.LastHitEvaded = true
			return 0, 0
		}
	} else if direct {
		enemy.ReceivedHits++
	}
	if enemy.PassiveID == "electrogenesis" && enemy.HP > enemy.MaxHP*0.5 && !HasCondition(enemy, "stun") {
		return 0, 0
	}
	switch enemy.PassiveID {
	case "lightMetal":
		amount = min(amount, 1000)
	case "sturdy":
		amount = max(0, amount-200)
	case "clangingScales":
		if enemy.Armor > 0 {
			amount = math.Ceil(amount * 0.5)
		} else {
			amount = math.Ceil(amount * 2)
		}
	case "sheerForce":
		if direct && amount < 1000 {
			return 0, 0
		}
	case "heavyMetal":
		if direct {
			amount *= 0.3
		}
	case "luchadoro":
		if direct && enemy.Armor > 0 {
			amount *= 0.5
		}
	case "fluffy":
		if direct && HasCondition(enemy, "burn") {
			amount *= 2
		}
	case "guardSplit":
		if enemy.MaxArmor > 0 {
			amount = math.Ceil(amount * (1 - 0.5*(1-max(0, enemy.Armor)/enemy.MaxArmor)))
		}
	case "ironDefense":
		if enemy.PassiveCounters == nil {
			enemy.PassiveCounters = map[string]int{}
		}
		amount = math.Floor(amount * max(0, 1-float64(enemy.PassiveCounters["ironDefenseHits"])*.01))
		enemy.PassiveCounters["ironDefenseHits"] = min(100, enemy.PassiveCounters["ironDefenseHits"]+1)
	case "dauntlessShield":
		if direct {
			charges := 400 - enemy.ReceivedHits*2
			amount = math.Floor(max(0, amount*(1-float64(charges)/100)))
			if charges < 0 {
				enemy.BaseSpeed = 48
			}
		}
	case "intrepidSword":
		if direct && !enemy.PassiveFlags["intrepidActivated"] {
			charges := -100 + enemy.ReceivedHits*2
			amount = math.Floor(max(0, amount*(1-float64(charges)/100)))
			if charges >= 100 {
				if enemy.PassiveFlags == nil {
					enemy.PassiveFlags = map[string]bool{}
				}
				enemy.PassiveFlags["intrepidActivated"] = true
				enemy.BaseSpeed = 72
			}
		}
	}
	if enemy.PassiveID == "pheromoneRush" {
		amount = math.Floor(amount * (1 + float64(ConditionStacks(enemy, "poison"))*0.03))
	}
	if enemy.PassiveID == "prankster" && (kind == "burn" || kind == "poison") {
		amount = math.Floor(amount * 0.5)
	}
	if enemy.PassiveID == "dragonsMaw" {
		amount += math.Floor(amount * .15 * float64(enemy.PassiveCounters["damageCharges"]))
	}
	if enemy.PassiveID == "asOne" {
		amount = math.Ceil(amount * enemy.PassiveValues["activePieces"] * 2)
	}
	if enemy.PassiveID == "disguise" && direct && enemy.Armor > 0 {
		amount = 0
	}
	if enemy.PassiveID == "slackOff" && direct {
		enemy.HP = min(enemy.MaxHP, enemy.HP+50)
	}
	enemy.LastResolvedDamage = amount
	credit := min(enemy.HP+max(0, enemy.Armor), amount)
	previous := enemy.HP
	if kind != "true" {
		pierce := min(1, max(0, armorPierce))
		bypass := amount * pierce
		blocked := amount - bypass
		shieldLoss := min(enemy.Shield, blocked)
		enemy.Shield -= shieldLoss
		enemy.LastShieldDamage = shieldLoss
		blocked -= shieldLoss
		armorLoss := min(max(0, enemy.Armor), blocked)
		enemy.Armor -= armorLoss
		enemy.LastArmorDamage = armorLoss
		blocked -= armorLoss
		enemy.HP = max(0, enemy.HP-bypass-blocked)
	} else {
		enemy.HP = max(0, enemy.HP-amount)
	}
	if enemy.PassiveID == "magmaStorm" && !HasCondition(enemy, "burn") {
		enemy.HP = max(1, enemy.HP)
	}
	loss := previous - enemy.HP
	enemy.LastHPDamage = max(0, loss)
	if enemy.PassiveID == "fullMetalBody" && direct {
		enemy.Armor += loss
	}
	if enemy.PassiveID == "lunarBlessing" && enemy.HP > 0 && enemy.HP < enemy.MaxHP*0.5 {
		enemy.LunarActivated = true
		kept := enemy.Conditions[:0]
		for _, condition := range enemy.Conditions {
			if condition.Kind == "nightmare" {
				kept = append(kept, condition)
			}
		}
		enemy.Conditions = kept
		enemy.Regeneration = 1000
	}
	return max(0, loss), credit
}

type ConditionDamageRecord struct {
	Kind            string  `json:"kind"`
	SourcePokemonID int     `json:"sourcePokemonID"`
	HPDamage        float64 `json:"hpDamage"`
	ArmorDamage     float64 `json:"armorDamage"`
	ShieldDamage    float64 `json:"shieldDamage"`
	AttemptedDamage float64 `json:"attemptedDamage"`
	ResolvedDamage  float64 `json:"resolvedDamage"`
	PreHitHP        float64 `json:"preHitHP"`
	DexCredit       float64 `json:"dexCredit"`
}

func advanceConditionsStep(enemy *EnemyEffectState, milliseconds int64, periodicMS float64) []ConditionDamageRecord {
	if enemy == nil || milliseconds <= 0 || enemy.HP <= 0 {
		return nil
	}
	records := []ConditionDamageRecord{}
	seconds := float64(milliseconds) / 1000
	for index := 0; index < len(enemy.Conditions); index++ {
		condition := &enemy.Conditions[index]
		activeMS := milliseconds
		if condition.RemainingMS >= 0 {
			activeMS = min(activeMS, condition.RemainingMS)
		}
		if activeMS <= 0 {
			continue
		}
		elapsed := periodicMS
		if activeMS < milliseconds {
			elapsed *= float64(activeMS) / float64(milliseconds)
		}
		condition.TickElapsedMS += elapsed
		ticks := int64(condition.TickElapsedMS / 1000)
		condition.TickElapsedMS -= float64(ticks) * 1000
		finitePeriodic := condition.RemainingMS >= 0 && stringsIn(condition.Kind, "burn scorch")
		if finitePeriodic {
			ticks = min(ticks, (condition.RemainingMS+999)/1000)
		}
		for tick := int64(0); tick < ticks && enemy.HP > 0; tick++ {
			hp, armor, shield := enemy.HP, enemy.Armor, enemy.Shield
			credit := 0.0
			enemy.LastDamageSourceID = condition.SourcePokemonID
			if stringsIn(condition.Kind, "burn poison nightmare scorch") {
				reduceHerbogenesis(enemy, "", condition.SourceItemID)
			}
			switch condition.Kind {
			case "burn":
				if enemy.PassiveID == "flashFire" {
					enemy.HP = min(enemy.MaxHP, enemy.HP+enemy.MaxHP*0.01)
				} else {
					_, credit = applyDamageWithCredit(enemy, math.Ceil(enemy.MaxHP*condition.Magnitude), "burn", 0)
				}
			case "poison":
				if enemy.PassiveID != "pheromoneRush" {
					_, credit = applyDamageWithCredit(enemy, math.Ceil(enemy.MaxHP*condition.Magnitude*float64(condition.Stacks)), "poison", 0)
				}
			case "nightmare":
				_, credit = applyDamageWithCredit(enemy, math.Ceil(condition.Magnitude*float64(condition.Stacks)), "nightmare", 0)
			case "scorch":
				_, credit = applyDamageWithCredit(enemy, math.Ceil(enemy.MaxArmor*condition.Magnitude), "scorch", 0)
			}
			if hp > enemy.HP || armor > enemy.Armor || shield > enemy.Shield || credit > 0 {
				records = append(records, ConditionDamageRecord{Kind: condition.Kind, SourcePokemonID: condition.SourcePokemonID, HPDamage: max(0, hp-enemy.HP), ArmorDamage: max(0, armor-enemy.Armor), ShieldDamage: max(0, shield-enemy.Shield), AttemptedDamage: enemy.LastIncomingDamage, ResolvedDamage: enemy.LastResolvedDamage, PreHitHP: hp, DexCredit: credit})
			}
			if len(enemy.Conditions) == 0 {
				break
			}
		}
		if condition.RemainingMS >= 0 {
			if finitePeriodic {
				condition.RemainingMS = max(0, condition.RemainingMS-ticks*1000)
			} else {
				condition.RemainingMS = max(0, condition.RemainingMS-milliseconds)
			}
		}
	}
	live := enemy.Conditions[:0]
	for _, condition := range enemy.Conditions {
		if condition.RemainingMS != 0 {
			live = append(live, condition)
		}
	}
	enemy.Conditions = live
	if enemy.HP > 0 {
		if !HasCondition(enemy, "healBlock") && enemy.PassiveID != "healBlock" {
			if enemy.Regeneration > 0 {
				enemy.RegenerationElapsedMS += milliseconds
				for enemy.RegenerationElapsedMS >= 1000 {
					enemy.RegenerationElapsedMS -= 1000
					enemy.HP = min(enemy.MaxHP, enemy.HP+enemy.Regeneration)
					enemy.HealingEvents++
				}
			}
			if enemy.PassiveID == "synthesis" {
				enemy.SynthesisElapsedMS += milliseconds
				for enemy.SynthesisElapsedMS >= 1000 {
					enemy.SynthesisElapsedMS -= 1000
					percentage := .025
					if isSun(enemy.WeatherID) {
						percentage = .05
					}
					enemy.HP = min(enemy.MaxHP, enemy.HP*(1+percentage))
					enemy.HealingEvents++
				}
			}
			if enemy.PassiveID == "clearBody" {
				enemy.HP = min(enemy.MaxHP, enemy.HP+50*float64(max(0, enemy.LivingEnemies-1))*seconds)
			}
		}
	}
	return records
}

func AdvanceConditions(enemy *EnemyEffectState, milliseconds int64) []ConditionDamageRecord {
	return advanceConditionsPrecise(enemy, milliseconds, float64(milliseconds))
}

func advanceConditionsPrecise(enemy *EnemyEffectState, milliseconds int64, periodicMS float64) []ConditionDamageRecord {
	if enemy == nil || milliseconds <= 0 || enemy.HP <= 0 {
		return nil
	}
	records := []ConditionDamageRecord{}
	remaining := milliseconds
	for remaining > 0 && enemy.HP > 0 {
		step := min(remaining, battleFrameUpperMS)
		if enemy.PassiveID == "naturalCure" {
			step = min(step, 5000-enemy.PassiveElapsedMS)
		}
		elapsed := periodicMS
		if step < remaining {
			elapsed *= float64(step) / float64(remaining)
		}
		records = append(records, advanceConditionsStep(enemy, step, elapsed)...)
		periodicMS -= elapsed
		remaining -= step
		if enemy.PassiveID == "naturalCure" {
			enemy.PassiveElapsedMS += step
		}
		if enemy.PassiveID == "naturalCure" && enemy.PassiveElapsedMS >= 5000 {
			keep := enemy.Conditions[:0]
			for _, condition := range enemy.Conditions {
				if condition.Kind == "nightmare" {
					keep = append(keep, condition)
				}
			}
			enemy.Conditions = keep
			enemy.PassiveElapsedMS = 0
		}
	}
	return records
}

func SetEnemyWeather(enemy *EnemyEffectState, weatherID string, spawning bool) error {
	if enemy == nil {
		return fmt.Errorf("missing enemy weather state")
	}
	switch weatherID {
	case "", "rain", "heavyRain", "thunderStorm", "harshSunlight", "extremelyHarshSunlight", "hail", "sandstorm", "sandAvalanche", "strongWinds", "fog", "starryNight":
	default:
		return fmt.Errorf("unknown weather %q", weatherID)
	}
	enemy.WeatherID = weatherID
	if spawning && weatherID == "fog" {
		enemy.Invisible = true
	}
	return nil
}

func AdvanceWeatherEffects(enemy *EnemyEffectState, milliseconds int64) float64 {
	if enemy == nil || enemy.HP <= 0 || enemy.Armor > 0 || milliseconds <= 0 {
		return 0
	}
	credit := 0.0
	magnitude := 0.0
	if enemy.WeatherID == "sandstorm" {
		magnitude = 0.003
	} else if enemy.WeatherID == "sandAvalanche" {
		magnitude = 0.005
	}
	if magnitude > 0 {
		enemy.WeatherElapsedMS += milliseconds
		multiplier := enemy.WorldSandMultiplier
		if multiplier <= 0 {
			multiplier = 1
		}
		for enemy.WeatherElapsedMS >= 1000 && enemy.HP > 0 {
			enemy.WeatherElapsedMS -= 1000
			amount := math.Ceil(enemy.MaxHP * magnitude * multiplier)
			if enemy.PassiveID == "asOne" {
				amount = math.Floor(amount / 4)
			}
			_, applied := applyDamageWithCredit(enemy, amount, "residual", 0)
			credit += applied
		}
	}
	return credit
}

func LineHitIDs(origin, end Point, halfWidth float64, enemies []Enemy, canSeeInvisible bool) []int {
	if !finiteNumber(origin.X) || !finiteNumber(origin.Y) || !finiteNumber(end.X) || !finiteNumber(end.Y) || !finiteNumber(halfWidth) || halfWidth < 0 {
		return nil
	}
	dx, dy := end.X-origin.X, end.Y-origin.Y
	length := vectorLength(dx, dy)
	if length <= 0 {
		return nil
	}
	type intersection struct {
		id    int
		along float64
	}
	hits := []intersection{}
	for _, enemy := range enemies {
		if enemy.HP <= 0 || enemy.Dead || enemy.Escaped || !enemy.Entered || enemy.Invisible && !canSeeInvisible {
			continue
		}
		x, y := enemy.X-origin.X, enemy.Y-origin.Y
		along := (x*dx + y*dy) / length
		across := math.Abs(x*dy-y*dx) / length
		if along >= 0 && along <= length && across <= halfWidth {
			hits = append(hits, intersection{id: enemy.ID, along: along})
		}
	}
	sort.SliceStable(hits, func(left, right int) bool {
		if hits[left].along == hits[right].along {
			return hits[left].id < hits[right].id
		}
		return hits[left].along < hits[right].along
	})
	ids := make([]int, len(hits))
	for index, hit := range hits {
		ids[index] = hit.id
	}
	return ids
}

type EnemyPassiveEvent struct {
	Kind               string  `json:"kind"`
	Key                string  `json:"key,omitempty"`
	EnemyID            int     `json:"enemyID"`
	Amount             float64 `json:"amount"`
	X                  float64 `json:"x"`
	Y                  float64 `json:"y"`
	Radius             float64 `json:"radius,omitempty"`
	PathIndex          int     `json:"pathIndex"`
	Waypoint           int     `json:"waypoint"`
	DelayMS            int64   `json:"delayMS,omitempty"`
	HealthScale        float64 `json:"healthScale,omitempty"`
	SourcePokemonID    int     `json:"sourcePokemonID,omitempty"`
	SourceAbilityID    string  `json:"sourceAbilityID,omitempty"`
	AtRouteStart       bool    `json:"atRouteStart,omitempty"`
	Scatter            float64 `json:"scatter,omitempty"`
	Offset             float64 `json:"offset,omitempty"`
	PathOriginWaypoint int     `json:"pathOriginWaypoint,omitempty"`
}

func enemyOnCanvas(state *State, enemy *Enemy) bool {
	route, ok := routeForState(state)
	return ok && enemy != nil && !enemy.Dead && !enemy.Escaped && enemy.HP > 0 && enemy.Entered && enemy.X >= 0 && enemy.Y >= 0 && enemy.X <= float64(route.Width) && enemy.Y <= float64(route.Height)
}

func EnemyHitTarget(state *State, target *Enemy) *Enemy {
	if state == nil || target == nil {
		return target
	}
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if enemy.ID != target.ID && enemy.PassiveID == "stanceChange" && enemyOnCanvas(state, enemy) {
			if target.Invisible {
				target.Invisible = false
				initializePassiveState(target)
				target.PassiveCounters["invisibilityBreaks"]++
			}
			return enemy
		}
	}
	return target
}

func EnemyCriticalFlag(state *State, critical bool) bool {
	if state == nil || !critical {
		return critical
	}
	for index := range state.Enemies {
		enemy := &state.Enemies[index]
		if enemy.PassiveID == "criticalAura" && enemyOnCanvas(state, enemy) {
			return false
		}
	}
	return critical
}

func PrepareEnemyHit(state *State, enemy *Enemy, amount float64, kind string) (float64, bool, error) {
	if state == nil || enemy == nil || !finiteNumber(amount) || amount < 0 {
		return 0, false, fmt.Errorf("invalid enemy hit context")
	}
	initializePassiveState(enemy)
	if kind == "direct" || kind == "critical" {
		reduceHerbogenesis(&enemy.EnemyEffectState, enemy.LastHitAbilityID, enemy.LastHitItemID)
		if enemy.PassiveID == "sweetScent" && NextRandom(state) < .05 {
			enemy.Invisible, enemy.LastHitEvaded = true, true
			enemy.PassiveCounters["evades"]++
			return 0, false, nil
		}
	}
	if enemy.PassiveID == "asOne" {
		enemy.PassiveValues["activePieces"] = float64(len(state.World.ChessActive))
	}
	return amount, true, nil
}

func reduceHerbogenesis(enemy *EnemyEffectState, abilityID, itemID string) {
	if enemy.PassiveID != "herbogenesis" || !stringsIn(abilityID, "heal bitterBlade takeHeart soulHeartReactor seedFlare") && !stringsIn(itemID, "leftovers shellBell clefairyDoll") {
		return
	}
	if enemy.PassiveFlags == nil {
		enemy.PassiveFlags = map[string]bool{}
	}
	enemy.Regeneration = max(0, enemy.Regeneration-1000)
	enemy.PassiveFlags["regenerationReduced"] = true
}

func EnemyAfterHit(state *State, enemy *Enemy) ([]EnemyPassiveEvent, error) {
	if state == nil || enemy == nil {
		return nil, fmt.Errorf("invalid enemy hit result")
	}
	initializePassiveState(enemy)
	events := []EnemyPassiveEvent{}
	switch enemy.PassiveID {
	case "eonUnion":
		amount := enemy.LastHPDamage + enemy.LastArmorDamage
		if amount > 0 {
			for index := range state.Enemies {
				other := &state.Enemies[index]
				if other.ID != enemy.ID && other.HP > 0 && !other.Dead && !other.Escaped && other.PassiveID == "eonUnion" {
					event := passiveEvent(other, "sharedDamage", enemy.LastDamageKind, amount)
					event.SourcePokemonID = enemy.LastDamageSourceID
					event.SourceAbilityID = enemy.LastHitAbilityID
					events = append(events, event)
				}
			}
		}
	case "anchorShot", "pressure":
		if enemy.Armor <= 0 && !enemy.PassiveFlags["armorBreakTriggered"] {
			enemy.PassiveFlags["armorBreakTriggered"] = true
			event := passiveEvent(enemy, enemy.PassiveID, enemy.PassiveID, 0)
			event.SourcePokemonID = enemy.LastDamageSourceID
			events = append(events, event)
		}
	case "klutz":
		event := passiveEvent(enemy, "stripItem", enemy.LastHitItemID, 0)
		event.SourcePokemonID = enemy.LastDamageSourceID
		events = append(events, event)
	}
	return events, nil
}

func EnemyAttackAllowed(state *State, pokemon *Pokemon, target *Enemy, area bool) bool {
	if state == nil || target == nil || target.PassiveID != "relicSong" || !area && pokemon != nil && CatalogData().Species[pokemon.Species].ID == 53 {
		return true
	}
	return NextRandom(state) >= .2
}

func stringsIn(value, choices string) bool {
	if value == "" {
		return false
	}
	for _, choice := range strings.Fields(choices) {
		if value == choice {
			return true
		}
	}
	return false
}

func ConfigureEnemyPath(state *State, enemy *Enemy) {
	if state == nil || enemy == nil {
		return
	}
	enemy.PathMirrored = ProgressionMirrored(state)
	enemy.PathVariant = ""
	if enemy.PassiveID == "contrary" {
		enemy.PathMirrored = !enemy.PathMirrored
	}
	if enemy.PassiveID == "ownTempo" {
		enemy.PathVariant = "spindaWaypoints"
		if enemy.Key == "superSpinda" {
			enemy.PathVariant = "superSpindaWaypoints"
		}
	}
}

func EnemyPath(state *State, enemy *Enemy) []Point {
	if state == nil || enemy == nil {
		return nil
	}
	route, ok := routeForState(state)
	if !ok {
		return nil
	}
	paths := route.Paths
	if enemy.PathMirrored && len(route.MirrorPaths) > 0 {
		paths = route.MirrorPaths
	}
	if enemy.PathVariant != "" {
		key := enemy.PathVariant
		if enemy.PathMirrored {
			key += "Mirror"
		}
		var special [][]Point
		if json.Unmarshal(route.Facts[key], &special) == nil && len(special) > 0 {
			paths = special
		}
	}
	if len(paths) == 0 {
		return nil
	}
	path := paths[max(0, enemy.PathIndex)%len(paths)]
	origin := max(0, enemy.PathOriginWaypoint)
	if origin >= len(path) {
		return nil
	}
	return path[origin:]
}

func EnemySpawnOrigin(state *State, enemy *Enemy, sequenceIndex int) Point {
	path := EnemyPath(state, enemy)
	if len(path) == 0 {
		return Point{}
	}
	route, _ := routeForState(state)
	offset := route.Offset
	if enemy.PathMirrored {
		offset = route.MirrorOffset
	}
	return Point{X: path[0].X - float64(sequenceIndex+1)*offset.X + enemy.Sprite.FrameWidth/2, Y: path[0].Y - float64(sequenceIndex+1)*offset.Y + enemy.Sprite.FrameHeight/2}
}

func EnemyTerrainMultiplier(state *State, enemy *Enemy, dx float64) float64 {
	if state == nil || enemy == nil {
		return 1
	}
	route, ok := routeForState(state)
	if !ok {
		return 1
	}
	tileSize := route.TileSize
	if tileSize <= 0 {
		tileSize = 24
	}
	column, row := int(math.Floor(enemy.X/float64(tileSize))), int(math.Floor(enemy.Y/float64(tileSize)))
	if column < 0 || row < 0 || column >= route.GridColumns {
		return 1
	}
	index := row*route.GridColumns + column
	lookup := func(key string, value int) bool {
		grid := route.PlacementGrid
		if raw, exists := route.Facts[key]; exists {
			if json.Unmarshal(raw, &grid) != nil {
				return false
			}
		}
		return index < len(grid) && grid[index] == value
	}
	wind, mud := 1.0, 1.0
	if dx != 0 && lookup("windTiles", 8) {
		wind = .6
		if dx > 0 {
			wind = 2
		}
		if enemy.PassiveID == "gust" && wind == .6 {
			wind = 2
		} else if enemy.PassiveID == "windRider" {
			if wind == .6 {
				wind = .3
			} else {
				wind = 4
			}
		}
	}
	if lookup("slowTiles", 7) {
		mud = .3
		if enemy.PassiveID == "hustle" {
			mud = 2
		}
	}
	return wind * mud
}

func passiveEvent(enemy *Enemy, kind, key string, amount float64) EnemyPassiveEvent {
	return EnemyPassiveEvent{Kind: kind, Key: key, EnemyID: enemy.ID, Amount: amount, X: enemy.X, Y: enemy.Y, PathIndex: enemy.PathIndex, Waypoint: enemy.Waypoint, PathOriginWaypoint: enemy.PathOriginWaypoint, HealthScale: 1}
}

func initializePassiveState(enemy *Enemy) {
	if enemy.PassiveFlags == nil {
		enemy.PassiveFlags = map[string]bool{}
	}
	if enemy.PassiveTimers == nil {
		enemy.PassiveTimers = map[string]int64{}
	}
	if enemy.PassiveValues == nil {
		enemy.PassiveValues = map[string]float64{}
	}
	if enemy.PassiveCounters == nil {
		enemy.PassiveCounters = map[string]int{}
	}
	if _, ok := enemy.PassiveValues["baseSpeed"]; !ok {
		enemy.PassiveValues["baseSpeed"] = enemy.BaseSpeed
	}
}

func pulsePassive(enemy *Enemy, key string, milliseconds, period int64) int64 {
	enemy.PassiveTimers[key] += milliseconds
	count := enemy.PassiveTimers[key] / period
	enemy.PassiveTimers[key] %= period
	return count
}

func RegisterEnemyAttackFailure(enemy *Enemy, reason string) {
	if enemy == nil {
		return
	}
	initializePassiveState(enemy)
	enemy.PassiveCounters[reason]++
	if reason == "static" {
		enemy.PassiveCounters["paralyses"]++
	}
}

func RefreshConditionSources(state *State, enemy *Enemy) error {
	if state == nil || enemy == nil {
		return fmt.Errorf("missing condition source context")
	}
	for index := range enemy.Conditions {
		condition := &enemy.Conditions[index]
		if condition.SourcePokemonID <= 0 {
			continue
		}
		owner := ownedPokemonFor(state, condition.SourcePokemonID)
		if owner == nil {
			continue
		}
		condition.SourceItemID = owner.Item
		if condition.Kind != "nightmare" {
			continue
		}
		level := owner.Level
		profile, err := BuildAttackProfile(AbilityContext{ItemID: owner.Item, Level: level, Power: owner.Stats.Power, Range: owner.Stats.Range, AttackMS: owner.Stats.AttackMS, CriticalPct: owner.Stats.CriticalPct, Hearts: state.Health, MaxHearts: state.MaxHealth, Stars: state.Stars, Ribbons: state.Ribbons, HealingUnit: true, EffectsVolume: pokemonSettings(state, owner).Audio.Effects, Random: 0.5})
		if err != nil {
			return err
		}
		condition.Magnitude = profile.Power * 0.2
	}
	return nil
}

func EnemyPassiveEvents(state *State, enemy *Enemy, stage string, milliseconds int64) ([]EnemyPassiveEvent, error) {
	if state == nil || enemy == nil || milliseconds < 0 || milliseconds > 600000 {
		return nil, fmt.Errorf("invalid enemy passive context")
	}
	switch stage {
	case "spawn", "tick", "hit", "death", "escape", "bell":
	default:
		return nil, fmt.Errorf("unknown enemy passive stage %q", stage)
	}
	initializePassiveState(enemy)
	events := []EnemyPassiveEvent{}
	if stage == "tick" {
		enemy.TemporaryStopMS = max(0, enemy.TemporaryStopMS-milliseconds)
		enemy.InvulnerableMS = max(0, enemy.InvulnerableMS-milliseconds)
		enemy.PassiveValues["tailWindMultiplier"] = 1
		for index := range state.Enemies {
			other := &state.Enemies[index]
			if other.ID != enemy.ID && other.HP > 0 && !other.Dead && !other.Escaped && other.PassiveID == "tailWind" && vectorLength(other.X-enemy.X, other.Y-enemy.Y) <= 75 {
				enemy.PassiveValues["tailWindMultiplier"] = 1.25
				break
			}
		}
	}
	if stage == "death" && enemy.PassiveFlags["deathHandled"] {
		return events, nil
	}
	switch enemy.PassiveID {
	case "regenerator":
		if stage == "death" && enemy.HP <= 0 {
			if !enemy.PassiveFlags["regeneratorStarted"] {
				enemy.PassiveFlags["regeneratorStarted"] = true
				enemy.HP, enemy.Armor, enemy.MaxArmor = 1, 50000, 50000
				enemy.Conditions = nil
				enemy.ReviveRemainingMS, enemy.TemporaryStopMS = 5000, 5000
				enemy.ReviveHP = enemy.MaxHP
				events = append(events, passiveEvent(enemy, "phase", "regenerating", 5000))
			} else {
				if enemy.ReviveRemainingMS > 0 {
					enemy.PassiveFlags["revivalInterrupted"] = true
				}
				enemy.ReviveRemainingMS, enemy.TemporaryStopMS = 0, 0
				enemy.PassiveFlags["deathHandled"] = true
			}
		}
		if stage == "tick" && enemy.ReviveRemainingMS > 0 && enemy.HP > 0 {
			enemy.ReviveRemainingMS = max(0, enemy.ReviveRemainingMS-milliseconds)
			if enemy.ReviveRemainingMS == 0 {
				enemy.HP = enemy.ReviveHP
				enemy.Armor, enemy.MaxArmor, enemy.TemporaryStopMS = 0, 0, 0
				enemy.BaseSpeed = enemy.PassiveValues["baseSpeed"]
				enemy.Revived = true
				events = append(events, passiveEvent(enemy, "revive", enemy.Key, enemy.HP))
			}
		}
	case "shadowShield":
		if stage == "hit" && (enemy.LastDamageKind == "direct" || enemy.LastDamageKind == "critical") && enemy.Invisible {
			enemy.Invisible = false
			enemy.PassiveFlags["shadowLost"] = true
			enemy.PassiveTimers["shadowLost"] = 0
			enemy.PassiveCounters["invisibilityBreaks"]++
		}
		if stage == "tick" && enemy.PassiveFlags["shadowLost"] {
			enemy.PassiveTimers["shadowLost"] += milliseconds
			if enemy.PassiveTimers["shadowLost"] >= 4000 {
				enemy.Invisible = true
				enemy.Armor = enemy.MaxArmor
				enemy.PassiveFlags["shadowLost"] = false
				events = append(events, passiveEvent(enemy, "phase", "shadowRestored", enemy.Armor))
			}
		}
	case "timeTravel":
		if stage == "tick" {
			if enemy.PassiveTimers["speedBoost"] > 0 {
				enemy.PassiveTimers["speedBoost"] = max(0, enemy.PassiveTimers["speedBoost"]-milliseconds)
				if enemy.PassiveTimers["speedBoost"] == 0 {
					enemy.BaseSpeed = enemy.PassiveValues["baseSpeed"]
				}
			}
			count := pulsePassive(enemy, "timeTravel", milliseconds, 8000)
			for index := int64(0); index < count; index++ {
				enemy.HP = min(enemy.MaxHP, enemy.HP+7000)
				keep := enemy.Conditions[:0]
				for _, condition := range enemy.Conditions {
					if condition.Kind != "burn" && condition.Kind != "poison" && condition.Kind != "nightmare" && condition.Kind != "scorch" {
						keep = append(keep, condition)
					}
				}
				enemy.Conditions = keep
				events = append(events, passiveEvent(enemy, "rewind", "timeTravel", 150+50*NextRandom(state)))
			}
			if count > 0 {
				enemy.PassiveTimers["speedBoost"] = max(0, 1000-enemy.PassiveTimers["timeTravel"])
				enemy.BaseSpeed = enemy.PassiveValues["baseSpeed"]
				if enemy.PassiveTimers["speedBoost"] > 0 {
					enemy.BaseSpeed += 60
				}
			}
		}
	case "drought", "drizzle":
		weather, extreme := "harshSunlight", "extremelyHarshSunlight"
		if enemy.PassiveID == "drizzle" {
			weather, extreme = "rain", "heavyRain"
		}
		if stage == "spawn" && !enemy.PassiveFlags["spawned"] && state.Weather != "starryNight" {
			events = append(events, passiveEvent(enemy, "weather", weather, 0))
		}
		if (stage == "tick" || stage == "hit") && enemy.HP > 0 && enemy.HP <= enemy.MaxHP*0.5 && !enemy.PassiveFlags["extremeWeather"] {
			enemy.PassiveFlags["extremeWeather"] = true
			enemy.Armor += 75000
			enemy.MaxArmor += 75000
			enemy.TemporaryStopMS, enemy.InvulnerableMS = 3000, 3000
			if state.Weather != "starryNight" {
				events = append(events, passiveEvent(enemy, "weather", extreme, 0))
			}
		}
	case "schooling":
		if stage == "death" {
			event := passiveEvent(enemy, "spawn", "wishiwashiSolo", 100)
			event.Scatter, event.Offset = 30, -20
			event.PathOriginWaypoint = enemy.PathOriginWaypoint + enemy.Waypoint
			events = append(events, event)
		}
	case "healer", "healerChallenge":
		if stage == "tick" && enemy.HP > 0 {
			healing := 400.0
			if enemy.PassiveID == "healerChallenge" {
				healing = 200
			}
			event := passiveEvent(enemy, "heal", "aura", healing*float64(milliseconds)/1000)
			event.Radius = 100
			events = append(events, event)
		}
		if stage == "death" {
			events = append(events, passiveEvent(enemy, "heart", "healer", 1))
		}
	case "slowStart":
		if stage == "tick" {
			enemy.BaseSpeed += 3 * float64(pulsePassive(enemy, "slowStart", milliseconds, 1000))
		}
	case "soulHeart":
		if stage == "spawn" && !enemy.PassiveFlags["spawned"] {
			events = append(events, passiveEvent(enemy, "heart", "soulHeart", float64(state.MaxHealth-state.Health)))
		}
		if stage == "tick" {
			if count := pulsePassive(enemy, "soulHeart", milliseconds, 5000); count > 0 {
				events = append(events, passiveEvent(enemy, "heart", "soulHeart", -float64(count)))
			}
		}
	case "honeyGather":
		if stage == "death" {
			events = append(events, passiveEvent(enemy, "power", "waveHoney", 5))
		}
	case "honeyGatherBoss":
		if stage == "hit" && (enemy.LastDamageKind == "direct" || enemy.LastDamageKind == "critical") {
			events = append(events, passiveEvent(enemy, "power", "waveHoney", 1))
		}
	case "coconutDrop":
		if stage == "hit" {
			completed := enemy.ReceivedHits / 6
			count := completed - enemy.PassiveCounters["coconuts"]
			if count > 0 {
				event := passiveEvent(enemy, "spawn", "exeggcute", float64(count))
				event.HealthScale = 0.25
				events = append(events, event)
				enemy.PassiveCounters["coconuts"] = completed
			}
		}
	case "kingsShield":
		if stage == "hit" && enemy.MaxHP > 0 {
			if !enemy.PassiveFlags["kingsShieldInitialized"] {
				enemy.PassiveFlags["kingsShieldInitialized"] = true
				enemy.PassiveCounters["shieldsRemaining"] = 4
			}
			remaining := enemy.PassiveCounters["shieldsRemaining"]
			if remaining > 0 && enemy.LastPreHitHP < enemy.MaxHP*float64(remaining)*.2 {
				enemy.PassiveCounters["shieldsRemaining"]--
				event := passiveEvent(enemy, "spawn", "aegislash", 1)
				event.AtRouteStart, event.Waypoint = true, 0
				event.PathIndex = NextInt(state, len(pathsForState(state)))
				events = append(events, event)
			}
		}
	case "ironDefense":
		if stage == "bell" {
			enemy.ReceivedHits = 0
			enemy.PassiveCounters["ironDefenseHits"] = 0
			enemy.BaseSpeed += 18
			enemy.PassiveCounters["bells"]++
		}
	case "shellSmash":
		if stage == "hit" && enemy.LastDamageKind == "critical" {
			enemy.BaseSpeed += 30
			enemy.PassiveTimers["shellSmash"] = 2000
		}
		if stage == "tick" && enemy.PassiveTimers["shellSmash"] > 0 {
			enemy.PassiveTimers["shellSmash"] = max(0, enemy.PassiveTimers["shellSmash"]-milliseconds)
			if enemy.PassiveTimers["shellSmash"] == 0 {
				enemy.BaseSpeed -= 30
			}
		}
	case "steamEngine":
		if stage == "hit" && enemy.LastHitTerrain == 3 && enemy.PassiveTimers["steamEngine"] == 0 {
			enemy.BaseSpeed += 60
			enemy.PassiveTimers["steamEngine"] = 1000
		}
		if stage == "tick" && enemy.PassiveTimers["steamEngine"] > 0 {
			enemy.PassiveTimers["steamEngine"] = max(0, enemy.PassiveTimers["steamEngine"]-milliseconds)
			if enemy.PassiveTimers["steamEngine"] == 0 {
				enemy.BaseSpeed -= 60
			}
		}
	case "remoaid":
		if stage == "hit" && (enemy.LastDamageKind == "direct" || enemy.LastDamageKind == "critical") && NextRandom(state) < .2 {
			event := passiveEvent(enemy, "spawn", "remoraid", 1)
			event.AtRouteStart, event.Waypoint = true, 0
			paths := pathsForState(state)
			event.PathIndex = NextInt(state, len(paths))
			events = append(events, event)
		}
	case "dragonsMaw":
		if stage == "tick" {
			enemy.PassiveCounters["damageCharges"] += int(pulsePassive(enemy, "dragonsMaw", milliseconds, 1000))
		}
	case "ingrain":
		if stage == "tick" && pulsePassive(enemy, "ingrain", milliseconds, 250) > 0 {
			route, _ := routeForState(state)
			count := 0
			for _, placement := range route.Placements {
				if placement.Terrain != 2 || vectorLength(placement.X-enemy.X, placement.Y-enemy.Y) > 150 {
					continue
				}
				occupied := false
				for _, tower := range state.Towers {
					occupied = occupied || tower.TileID == placement.ID
				}
				if !occupied {
					count++
				}
			}
			enemy.Regeneration = float64(count) * 2000
		}
	case "transistor":
		if stage == "tick" && enemy.Waypoint > 1 && enemy.Waypoint != enemy.PassiveCounters["lastStunWaypoint"] {
			enemy.PassiveCounters["lastStunWaypoint"] = enemy.Waypoint
			ApplyConditions(&enemy.EnemyEffectState, AttackProfile{Conditions: []ConditionSpec{{Kind: "stun", Chance: 1, DurationMS: 2000, Stacks: 1, IgnoreResistance: true}}}, 0)
		}
	case "sunnyDay":
		if stage == "spawn" && !enemy.PassiveFlags["spawned"] && state.Weather != "starryNight" {
			events = append(events, passiveEvent(enemy, "weather", "harshSunlight", 0))
		}
	case "asOne":
		enemy.PassiveValues["activePieces"] = float64(len(state.World.ChessActive))
	case "", "eonUnion", "anchorShot", "pressure", "klutz", "criticalAura", "stanceChange", "stall", "relicSong", "herbogenesis", "sweetScent", "tailWind", "windRider", "gust", "hustle", "vCrate", "ownTempo", "contrary", "mirage", "fairyLock", "grimNeigh", "soundproof":
	case "naturalCure", "waterVeil", "leafGuard", "prankster", "justified", "spectralThief", "mummy", "magmaStorm", "swiftSwim", "chlorophyll", "turboblaze", "luchadoro", "criticalLearning", "nebulogenesis", "electrogenesis", "lightMetal", "sturdy", "clangingScales", "sheerForce", "heavyMetal", "fluffy", "guardSplit", "dauntlessShield", "intrepidSword", "pheromoneRush", "disguise", "fullMetalBody", "slackOff", "lunarBlessing", "flashFire", "synthesis", "clearBody", "healBlock", "iceBody", "flameBody", "static", "snowCloak", "psychogenesis", "abyssalBound", "rockHead", "weakArmor":
	default:
		return nil, fmt.Errorf("enemy passive %q needs an independently implemented world rule", enemy.PassiveID)
	}
	if stage == "spawn" {
		enemy.PassiveFlags["spawned"] = true
	}
	if stage == "death" && enemy.HP <= 0 {
		for index := range state.Enemies {
			stall := &state.Enemies[index]
			if stall.ID == enemy.ID || stall.PassiveID != "stall" || stall.Dead || stall.Escaped || stall.HP <= 0 || stall.PassiveFlags["stallUsed"] {
				continue
			}
			initializePassiveState(stall)
			stall.X, stall.Y = enemy.X, enemy.Y
			stall.PathIndex, stall.PathVariant, stall.PathMirrored = enemy.PathIndex, enemy.PathVariant, enemy.PathMirrored
			stall.PathOriginWaypoint = enemy.PathOriginWaypoint
			stall.Waypoint, stall.Distance, stall.Entered = enemy.Waypoint, enemy.Distance, enemy.Entered
			stall.PassiveFlags["stallUsed"] = true
			break
		}
		enemy.PassiveFlags["deathHandled"] = true
	}
	return events, nil
}
