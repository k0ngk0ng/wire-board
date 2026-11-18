package engine

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type AbilityContext struct {
	AbilityID            string
	ItemID               string
	WeatherID            string
	Level                int
	Terrain              int
	SandShotRoute        bool `json:",omitempty"`
	Stars                int
	Hearts               int
	MaxHearts            int
	Targets              int
	NearbyAllies         int
	Power                float64
	Range                float64
	AttackMS             float64
	CriticalPct          float64
	CriticalDamage       float64
	TargetHP             float64
	TargetMaxHP          float64
	TargetArmor          float64
	TargetMaxArmor       float64
	TargetSpeed          float64
	TargetPassiveID      string
	TargetInvisible      bool
	TargetBurned         bool
	TargetBurnDurationMS int64
	TargetPoisoned       bool
	TargetStunned        bool
	TargetSlowed         bool
	TargetCursed         bool
	TargetNightmares     int
	Distance             float64
	FlightMS             int64
	SameTargetHits       int
	AttackCount          int
	WaveKills            int
	HealingUses          float64
	HealedHearts         int
	HeartsLost           int
	FossilCount          int
	PastParadox          int
	FutureParadox        int
	MissingAllies        int
	Ribbons              int
	TotalGoldDigits      int
	DeployedMS           int64
	IdleMS               int64
	StoredShots          int
	HoneyStacks          int
	UltraBeastKills      int
	Charges              int
	EffectsVolume        int
	Teleports            int
	PowerStacks          int
	BoostRemainingMS     int64
	WaveElapsedMS        int64
	ShellSmashActive     bool
	CloudNine            bool
	Carried              bool
	AttackType           string
	WaterAllies          int
	GrassAllies          int
	MountainAllies       int
	BerryActive          bool
	CriticalStreak       int
	SinceCriticalMS      int64
	SpecialReduction     int
	PokemonID            int
	CursedEnemies        int
	DamageThisWave       float64
	StreamAngle          float64
	Day                  bool
	HasNegativeItem      bool
	HealingUnit          bool
	Ricochet             int
	SplashRadius         float64
	Shape                string
	SpeciesID            int
	Random               float64
}

type AuraSpec struct {
	PowerMultiplier          float64 `json:"powerMultiplier"`
	PowerFlat                float64 `json:"powerFlat"`
	AttackMultiplier         float64 `json:"attackMultiplier"`
	RangeMultiplier          float64 `json:"rangeMultiplier"`
	RangeFlat                float64 `json:"rangeFlat"`
	CriticalFlat             float64 `json:"criticalFlat"`
	CriticalDamageMultiplier float64 `json:"criticalDamageMultiplier"`
	CanSeeInvisible          bool    `json:"canSeeInvisible"`
	Shape                    string  `json:"shape,omitempty"`
}

type AttackProfile struct {
	Power                   float64         `json:"power"`
	Range                   float64         `json:"range"`
	AttackMS                float64         `json:"attackMs"`
	CriticalPct             float64         `json:"criticalPct"`
	CriticalMultiplier      float64         `json:"criticalMultiplier"`
	DamageMultiplier        float64         `json:"damageMultiplier"`
	TargetCount             int             `json:"targetCount"`
	SplashRadius            float64         `json:"splashRadius"`
	SplashMultiplier        float64         `json:"splashMultiplier"`
	Ricochet                int             `json:"ricochet"`
	RicochetMultiplier      float64         `json:"ricochetMultiplier"`
	Pierce                  int             `json:"pierce"`
	Burst                   int             `json:"burst"`
	BurstIntervalMS         int64           `json:"burstIntervalMs"`
	Orbit                   bool            `json:"orbit"`
	OrbitSpeed              float64         `json:"orbitSpeed"`
	BeamDurationMS          int64           `json:"beamDurationMs"`
	BeamTickMS              int64           `json:"beamTickMs"`
	AuraDurationMS          int64           `json:"auraDurationMs"`
	AuraTickMS              int64           `json:"auraTickMs"`
	Conditions              []ConditionSpec `json:"conditions"`
	CanSeeInvisible         bool            `json:"canSeeInvisible"`
	ArmorPierce             float64         `json:"armorPierce"`
	ArmorRemovePct          float64         `json:"armorRemovePct"`
	RegenerationLoss        float64         `json:"regenerationLoss"`
	HeartChance             float64         `json:"heartChance"`
	HeartLimit              int             `json:"heartLimit"`
	HeartAmount             int             `json:"heartAmount"`
	GoldTargetFraction      float64         `json:"goldTargetFraction"`
	GoldDamageFraction      float64         `json:"goldDamageFraction"`
	CurseLink               bool            `json:"curseLink"`
	RepeatOnCritical        bool            `json:"repeatOnCritical"`
	RandomTarget            bool            `json:"randomTarget"`
	WeatherOverride         string          `json:"weatherOverride,omitempty"`
	Disabled                bool            `json:"disabled"`
	Returning               bool            `json:"returning"`
	MaxHPDamage             float64         `json:"maxHpDamage"`
	MaxHPDamageBosses       bool            `json:"maxHpDamageBosses"`
	RoundBaseDamage         bool            `json:"roundBaseDamage"`
	SoftSand                bool            `json:"softSand"`
	StrangeIdol             bool            `json:"strangeIdol"`
	ForcedCritical          bool            `json:"forcedCritical"`
	ConsumeBurn             bool            `json:"consumeBurn"`
	ConsumeNightmare        bool            `json:"consumeNightmare"`
	HealBlock               bool            `json:"healBlock"`
	CanMovePaused           bool            `json:"canMovePaused"`
	ItemDisabled            bool            `json:"itemDisabled"`
	ItemReusable            bool            `json:"itemReusable"`
	ItemEffectScale         float64         `json:"itemEffectScale"`
	BerryEffectScale        float64         `json:"berryEffectScale"`
	Aura                    AuraSpec        `json:"aura"`
	AttackMode              string          `json:"attackMode,omitempty"`
	WorldHook               string          `json:"worldHook,omitempty"`
	ConsumeCharges          int             `json:"consumeCharges"`
	RequiredCharges         int             `json:"requiredCharges"`
	HeartCost               int             `json:"heartCost"`
	HeartProtection         int             `json:"heartProtection"`
	BerryDurationScale      float64         `json:"berryDurationScale"`
	BerryAtWaveStart        bool            `json:"berryAtWaveStart"`
	BerryAfterAttack        bool            `json:"berryAfterAttack"`
	ShinyChance             float64         `json:"shinyChance"`
	PassengerTerrains       []int           `json:"passengerTerrains,omitempty"`
	PassengerAttackMS       float64         `json:"passengerAttackMs"`
	PassengerTerrain        int             `json:"passengerTerrain"`
	SuppressWeather         bool            `json:"suppressWeather"`
	IgnoreWeatherCost       bool            `json:"ignoreWeatherCost"`
	AuraRadius              float64         `json:"auraRadius"`
	AuraCondition           ConditionSpec   `json:"auraCondition"`
	TriggerDelayMS          int64           `json:"triggerDelayMs"`
	OncePerTarget           bool            `json:"oncePerTarget"`
	MoveMode                string          `json:"moveMode,omitempty"`
	OrbitalCount            int             `json:"orbitalCount"`
	BeamWidthScale          float64         `json:"beamWidthScale"`
	SplashOnCritical        bool            `json:"splashOnCritical"`
	PassengerPower          float64         `json:"passengerPower"`
	HeartInverted           bool            `json:"heartInverted"`
	HoneyMultiplier         float64         `json:"honeyMultiplier"`
	ItemStage               string          `json:"itemStage,omitempty"`
	FixedPower              float64         `json:"fixedPower"`
	HeartDamageGoal         float64         `json:"heartDamageGoal"`
	RangeShape              string          `json:"rangeShape,omitempty"`
	SpreadNightmare         bool            `json:"spreadNightmare"`
	ProjectileOverride      string          `json:"projectileOverride,omitempty"`
	MovementSpeed           float64         `json:"movementSpeed"`
	ArrivalThreshold        float64         `json:"arrivalThreshold"`
	TeleportIntervalMS      float64         `json:"teleportIntervalMs"`
	AttackFailure           string          `json:"attackFailure,omitempty"`
	FailureConsumesCooldown bool            `json:"failureConsumesCooldown"`
	ExtraDamagePct          float64         `json:"extraDamagePct"`
	ExtraDamageCeil         bool            `json:"extraDamageCeil"`
	SourcePowerBonusPct     float64         `json:"sourcePowerBonusPct"`
	InnerRange              float64         `json:"innerRange"`
	NightmareSpreadRadius   float64         `json:"nightmareSpreadRadius"`
	ExtraWorldHooks         []string        `json:"extraWorldHooks,omitempty"`
	TickDamageScale         float64         `json:"tickDamageScale"`
	ConeHalfAngle           float64         `json:"coneHalfAngle"`
	Technician              bool            `json:"technician"`
	SourcePowerScale        float64         `json:"sourcePowerScale"`
	SourcePowerFlat         float64         `json:"sourcePowerFlat"`
}

type RuleCoverage struct {
	ID             string `json:"id"`
	Implemented    bool   `json:"implemented"`
	WorldHook      string `json:"worldHook,omitempty"`
	RemainingFacts string `json:"remainingFacts,omitempty"`
	Stage          string `json:"stage"`
}

type ItemProfileFailure struct {
	Species string `json:"species"`
	Level   int    `json:"level"`
	Error   string `json:"error"`
}

type ItemProfileCoverage struct {
	ID                  string               `json:"id"`
	Equipable           bool                 `json:"equipable"`
	Removed             bool                 `json:"removed"`
	ShopPools           []string             `json:"shopPools"`
	EligibleRecipients  int                  `json:"eligibleRecipients"`
	ConstructedProfiles int                  `json:"constructedProfiles"`
	Failures            []ItemProfileFailure `json:"failures"`
}

func isSun(id string) bool  { return id == "harshSunlight" || id == "extremelyHarshSunlight" }
func isRain(id string) bool { return id == "rain" || id == "heavyRain" || id == "thunderStorm" }
func isSand(id string) bool { return id == "sandstorm" || id == "sandAvalanche" }

func status(kind string, chance float64, duration int64, magnitude float64, stacks int) ConditionSpec {
	return ConditionSpec{Kind: kind, Chance: chance, DurationMS: duration, Magnitude: magnitude, Stacks: stacks}
}

func burnCondition() ConditionSpec             { return status("burn", 1, 10000, 0.005, 1) }
func slowCondition() ConditionSpec             { return status("slow", 1, 2000, 0.5, 1) }
func poisonCondition(stacks int) ConditionSpec { return status("poison", 1, -1, 0.001, stacks) }
func curseCondition() ConditionSpec            { return status("curse", 1, -1, 0, 1) }

func splash(profile *AttackProfile, context AbilityContext, multiplier float64) {
	profile.SplashRadius = context.SplashRadius
	if profile.SplashRadius <= 0 {
		profile.SplashRadius = 65
	}
	profile.SplashMultiplier = multiplier
}

func ricochet(profile *AttackProfile, context AbilityContext) {
	profile.Ricochet = max(1, context.Ricochet)
}

func applyAbility(profile *AttackProfile, context AbilityContext) error {
	missing := max(0, context.MaxHearts-context.Hearts)
	switch context.AbilityID {
	case "":
	case "ambusher":
		if context.Terrain == 2 {
			profile.Power *= 2
		}
	case "swimmer":
		if context.Terrain == 3 {
			profile.AttackMS *= 0.5
		}
	case "vigilant", "vigilantFrisk":
		if context.Terrain == 4 {
			profile.Range *= 2
		}
		profile.CanSeeInvisible = context.AbilityID == "vigilantFrisk"
	case "frisk", "scrappy":
		profile.CanSeeInvisible = true
	case "burn", "burnDoubleShot", "burnNerf":
		condition := burnCondition()
		if context.AbilityID == "burnDoubleShot" {
			profile.TargetCount = 2
		}
		if context.AbilityID == "burnNerf" {
			condition.Chance = 0.5
			profile.WeatherOverride = "harshSunlight"
		}
		profile.Conditions = append(profile.Conditions, condition)
	case "poison", "toxic", "poisonDoubleShot", "direClaw":
		stacks := 1
		if context.AbilityID == "toxic" {
			stacks = 2
		}
		if context.AbilityID == "poisonDoubleShot" {
			profile.TargetCount = 2
		}
		profile.Conditions = append(profile.Conditions, poisonCondition(stacks))
	case "slow":
		profile.Conditions = append(profile.Conditions, slowCondition())
	case "stunMono", "stunMonoNerf", "stunArea", "moldBreaker":
		chance, duration := 0.3, int64(2000)
		if context.AbilityID == "stunArea" || context.AbilityID == "moldBreaker" {
			duration = 1500
		}
		if context.AbilityID == "stunMonoNerf" {
			chance, duration = 0.05, 1500
		}
		condition := status("stun", chance, duration, 1, 1)
		if context.AbilityID == "moldBreaker" {
			condition.IgnoreResistance, condition.ResistantScale = true, 0.5
		}
		profile.Conditions = append(profile.Conditions, condition)
	case "armorBreak", "armorBreakDoubleShot", "rageFist", "piercingDrill":
		if (context.TargetMaxArmor > 0 || context.TargetArmor > 0) && context.TargetPassiveID != "rockHead" {
			profile.DamageMultiplier *= 2
		}
		if context.AbilityID == "armorBreakDoubleShot" {
			profile.TargetCount = 2
		}
		if context.AbilityID == "rageFist" {
			profile.Power += 50 * float64(context.HeartsLost)
		}
		if context.AbilityID == "piercingDrill" {
			profile.CriticalMultiplier = 2
		}
	case "ninja", "tripleAxel", "static", "slowRicochet", "burnRicochet", "magicBounce", "armaldo":
		ricochet(profile, context)
		if context.AbilityID == "tripleAxel" {
			profile.RicochetMultiplier = 1.5
		}
		if context.AbilityID == "static" {
			profile.Conditions = append(profile.Conditions, status("stun", 0.075, 1500, 1, 1))
		}
		if context.AbilityID == "slowRicochet" {
			profile.Conditions = append(profile.Conditions, slowCondition())
		}
		if context.AbilityID == "burnRicochet" {
			profile.Conditions = append(profile.Conditions, burnCondition())
		}
		if context.AbilityID == "magicBounce" {
			profile.Conditions = append(profile.Conditions, curseCondition())
			profile.CurseLink = true
		}
		if context.AbilityID == "armaldo" {
			profile.CriticalPct = min(profile.CriticalPct, 95)
			profile.AttackMode = "criticalRicochet"
		}
	case "heal", "takeHeart", "bitterBlade", "seedFlare", "soulHeartReactor":
		profile.HeartChance, profile.HeartLimit = 0.05, 3
		switch context.AbilityID {
		case "heal":
		case "takeHeart":
			profile.HeartLimit = 2
		case "bitterBlade":
			profile.HeartChance, profile.HeartLimit, profile.TargetCount = 0.01, 1, 2
		case "seedFlare":
			profile.HeartChance, profile.TargetCount = 0.02, 1+min(2, context.HealedHearts)
		case "soulHeartReactor":
			profile.HeartChance, profile.HeartLimit, profile.Pierce = 0.01, 2, 10000
			profile.AttackMode = "beam"
			profile.Power *= 1 + 0.1*float64(context.HealedHearts)
			profile.AttackMS *= math.Pow(0.997, float64(context.WaveKills))
		}
		if context.AbilityID != "takeHeart" {
			profile.HeartChance *= math.Pow(.5, context.HealingUses)
		}
		if context.HealingUses >= 3 {
			profile.HeartChance = 0
		}
	case "greed":
		profile.GoldTargetFraction = 0.1
	case "focus":
		profile.DamageMultiplier *= 1 + 0.075*float64(context.SameTargetHits)
	case "curse", "curseDoubleShot", "curseSplash", "willOWisp":
		profile.CurseLink = true
		profile.Conditions = append(profile.Conditions, curseCondition())
		if context.AbilityID == "curseDoubleShot" {
			profile.TargetCount = 2
		}
		if context.AbilityID == "curseSplash" {
			splash(profile, context, 0.5)
		}
		if context.AbilityID == "willOWisp" {
			profile.Conditions = []ConditionSpec{burnCondition()}
			profile.CurseLink = context.TargetCursed
		}
	case "doubleShot", "tripleShot", "quadraShot", "octaShot", "doubleShotSand", "quadraShotSand":
		profile.TargetCount = map[string]int{"doubleShot": 2, "tripleShot": 3, "quadraShot": 4, "octaShot": 8, "doubleShotSand": 2, "quadraShotSand": 4}[context.AbilityID]
		if context.SandShotRoute && (context.AbilityID == "doubleShotSand" || context.AbilityID == "quadraShotSand") {
			profile.Range *= 2
		}
	case "splash", "slowSplash", "armorBreakSplash", "synchronySplash", "burnSplash", "fireBlast", "auraSphere", "pollenPuff", "autotomize", "naturesJudgement", "magmaStorm":
		splash(profile, context, 0.5)
		switch context.AbilityID {
		case "slowSplash":
			profile.Conditions = append(profile.Conditions, slowCondition())
		case "armorBreakSplash":
			if (context.TargetMaxArmor > 0 || context.TargetArmor > 0) && context.TargetPassiveID != "rockHead" {
				profile.DamageMultiplier *= 2
			}
		case "burnSplash", "magmaStorm":
			profile.Conditions = append(profile.Conditions, burnCondition())
			if context.AbilityID == "magmaStorm" {
				profile.WorldHook = "burnDeathExplosion"
			}
		case "fireBlast":
			condition := burnCondition()
			condition.IgnoreResistance = true
			profile.Conditions = append(profile.Conditions, condition)
		case "auraSphere":
			profile.SplashMultiplier = 1
		case "pollenPuff":
		case "autotomize":
		case "synchronySplash":
			profile.WorldHook = "spreadPrimaryConditions"
		case "naturesJudgement":
			profile.WorldHook = "brutalFauna"
		}
	case "superCritical":
		profile.CriticalMultiplier = 2
	case "powerAura", "criticalDamageAura", "criticalAura", "triage", "illuminate", "illuminateBuff", "helpingHand", "swordsDance", "heartOfSteel", "sacredSword", "victoryStar", "genesisAura", "hugePower":
		switch context.AbilityID {
		case "powerAura":
			profile.Aura.PowerMultiplier = 1.2
		case "criticalDamageAura":
			profile.Aura.CriticalDamageMultiplier = 1.5
		case "criticalAura":
			profile.Aura.CriticalFlat = 20
		case "triage":
			profile.Aura.AttackMultiplier = 0.85
			profile.WorldHook = "endWaveHeartChance"
		case "illuminate":
			profile.Aura.RangeFlat, profile.Aura.CanSeeInvisible = 15, true
		case "illuminateBuff":
			profile.Aura.RangeFlat, profile.Aura.CanSeeInvisible = 25, true
		case "helpingHand":
			profile.Aura.PowerFlat, profile.WorldHook = 50, "onlyUnheldAllies"
		case "swordsDance":
			profile.Aura.PowerMultiplier, profile.WorldHook = 1.2, "justiceAllies"
		case "heartOfSteel":
			profile.Aura.PowerFlat, profile.Aura.CriticalFlat, profile.WorldHook = 25, 5, "justiceAllies"
			if context.HeartsLost > 0 {
				profile.Aura.PowerFlat *= 3
				profile.Aura.CriticalFlat *= 3
			}
		case "sacredSword":
			profile.Aura.RangeMultiplier, profile.WorldHook = 1.2, "justiceAllies"
		case "victoryStar":
			profile.WorldHook = "crossShapeWidth"
		case "genesisAura":
			profile.Aura.PowerMultiplier, profile.Aura.CriticalFlat, profile.Aura.AttackMultiplier = 1.15, 5, 0.9
		case "hugePower":
			profile.Power *= 2
			profile.Aura.PowerMultiplier, profile.Aura.Shape = 1.15, "star"
			profile.AttackMode = ""
		}
	case "spinda":
		profile.RandomTarget = context.ItemID != "ringTarget"
	case "castform":
		if context.Terrain == 2 {
			profile.Power *= 2
		}
		if context.Terrain == 3 {
			profile.AttackMS *= 0.5
		}
		if context.Terrain == 4 {
			profile.Range *= 2
		}
	case "star":
		profile.FixedPower = float64(context.Stars) * 0.33
	case "nightmare", "shadowBall", "shadowTag":
		profile.Conditions = append(profile.Conditions, status("nightmare", 1, -1, context.Power*0.2, 1))
		if context.AbilityID == "shadowBall" {
			profile.Orbit = true
		}
		if context.AbilityID == "shadowTag" {
			profile.AttackMode, profile.BeamDurationMS = "delayedArea", 2000
			profile.Conditions[0].Stacks = 2
			profile.TriggerDelayMS, profile.AuraRadius, profile.WorldHook = 2000, 60, "followingNightmareZone"
			profile.SplashRadius, profile.SplashMultiplier = 60, 1
			profile.AuraCondition = status("shadowTagSlow", 1, 100, .5, 1)
		}
	case "bastiodon":
		profile.AttackMS -= 500 * float64(context.FossilCount)
	case "rampardos":
		profile.Power *= 1 + 0.05*float64(missing)
	case "cradily":
		profile.TargetCount = max(1, context.FossilCount)
		profile.Conditions = append(profile.Conditions, slowCondition())
	case "fieryDance":
		if context.TargetBurned {
			profile.DamageMultiplier *= 1.35
			profile.ConsumeBurn = true
			profile.MaxHPDamage = .003 * float64(context.TargetBurnDurationMS) / 1000
			profile.MaxHPDamageBosses, profile.RoundBaseDamage = true, true
		}
	case "contrary":
		profile.ItemEffectScale = -1
	case "defiant":
		profile.WorldHook = "ignoreItemPowerPenalty"
	case "firstImpression":
		if context.SameTargetHits == 0 {
			profile.DamageMultiplier *= 3
		}
	case "moxie":
		profile.Power *= 1 + 0.05*float64(context.WaveKills)
	case "sniper":
		profile.DamageMultiplier *= math.Pow(2, context.Distance/150)
	case "toughClaws", "toughClawsDay", "toughClawsNight":
		power, critical := context.Terrain == 4, context.Terrain == 2
		if context.AbilityID == "toughClawsDay" {
			critical = context.Terrain == 4
		}
		if context.AbilityID == "toughClawsNight" {
			power = context.Terrain == 2
		}
		if power {
			profile.Power *= 1.5
		}
		if critical {
			profile.CriticalPct = 100
		}
	case "chatter":
		if context.EffectsVolume < 0 || context.EffectsVolume > 20 {
			return fmt.Errorf("invalid effects volume")
		}
		profile.DamageMultiplier *= 1 - float64(20-context.EffectsVolume)*0.05
	case "speedBoost":
		profile.AttackMS -= 300 * float64(min(10, context.AttackCount))
	case "simple":
		profile.ItemEffectScale = 1.75
	case "drizzle":
		profile.WeatherOverride = "rain"
	case "drought", "flamethrower":
		profile.AttackMode, profile.BeamTickMS, profile.TickDamageScale, profile.ConeHalfAngle = "cone", 100, 0.1, context.StreamAngle
		condition := burnCondition()
		condition.Chance = 0.1
		profile.Conditions = append(profile.Conditions, condition)
		if context.AbilityID == "drought" {
			profile.WeatherOverride = "harshSunlight"
		}
	case "defeatist":
		if context.Hearts <= 7 {
			profile.Power *= 0.5
		}
	case "dreamEater":
		if context.TargetNightmares > 0 {
			profile.DamageMultiplier *= 2
		}
	case "torrent":
		if context.Terrain == 3 && context.Hearts < 7 {
			profile.Power *= 1.75
		}
	case "overgrow":
		if context.Terrain == 2 && context.Hearts < 7 {
			profile.Power *= 1.75
		}
	case "magician":
		profile.ItemReusable = true
	case "makeItRain":
		perDigit := .075
		if context.ItemID == "amuletCoin" {
			perDigit = .15
		}
		profile.Power *= 1 + perDigit*float64(context.TotalGoldDigits)
	case "invalidState":
		choices := []ConditionSpec{burnCondition(), slowCondition(), status("stun", 1, 2000, 1, 1), poisonCondition(1)}
		condition := choices[min(3, int(context.Random*4))]
		condition.Chance = 0.05
		profile.Conditions = append(profile.Conditions, condition)
	case "solarBeam":
		profile.Pierce, profile.AttackMode = 10000, "beam"
		if isSun(context.WeatherID) {
			profile.AttackMS *= 0.9
		}
	case "hydroCannon", "moongeistBeam":
		profile.Pierce, profile.AttackMode, profile.BeamDurationMS, profile.BeamTickMS = 10000, "trackingBeam", 4000, 390
		if context.AbilityID == "moongeistBeam" {
			profile.CanSeeInvisible = true
			if context.TargetInvisible {
				profile.DamageMultiplier *= 3
			}
		}
	case "flameWheel", "waterBubble", "shiftGear", "triAttack", "stoneEdge", "stoneEdgeMega", "hiddenPower", "spiralDrain":
		profile.Orbit = true
		switch context.AbilityID {
		case "flameWheel":
			condition := burnCondition()
			condition.Chance = 0.5
			profile.Conditions = append(profile.Conditions, condition)
		case "waterBubble":
			profile.Conditions = append(profile.Conditions, slowCondition())
		case "shiftGear":
			profile.OrbitSpeed, profile.WorldHook = 0, "orbitAccelerationOnHit"
		case "triAttack":
			profile.WorldHook = "randomControlCondition"
		case "stoneEdge":
			if isSand(context.WeatherID) {
				profile.Power *= 1.2
			}
		case "stoneEdgeMega":
			if isSand(context.WeatherID) {
				profile.Power *= 1.25
				profile.OrbitSpeed = 1.5
			}
		case "hiddenPower":
			profile.CanSeeInvisible = true
		case "spiralDrain":
			profile.MaxHPDamage, profile.WorldHook = 0.03, "sereneWaves"
		}
	case "sandStream", "sandStreamTyranitar", "sandStreamMegaTyranitar":
		profile.WeatherOverride = "sandstorm"
		if context.AbilityID != "sandStream" {
			profile.WorldHook = "rangeSandDamage50"
		}
	case "snowWarning":
		profile.WeatherOverride = "hail"
	case "heatCrash":
		profile.DamageMultiplier *= max(0.1, context.TargetSpeed/60)
	case "flashFire", "solarPower":
		if context.TargetBurned {
			profile.DamageMultiplier *= 2
		}
		if context.AbilityID == "flashFire" && isSun(context.WeatherID) {
			profile.AttackMS *= 0.8
		}
		if context.AbilityID == "solarPower" {
			profile.WorldHook = "allBurnUnitsSun25"
		}
	case "venomDrench":
		if context.TargetPoisoned {
			profile.DamageMultiplier *= 2
		}
	case "spikes", "spikyShield", "precipiceBlades":
		profile.AttackMode, profile.AuraDurationMS, profile.AuraTickMS = "area", 5000, 500
		if context.AbilityID == "spikyShield" {
			profile.AuraDurationMS, profile.WorldHook = 2500, "chance25Area"
		}
		if context.AbilityID == "precipiceBlades" {
			profile.AuraDurationMS, profile.AuraTickMS = 1500, 450
		}
	case "technician":
		profile.Technician = true
	case "chlorophyll":
		if isSun(context.WeatherID) {
			profile.AttackMS *= 0.5
		}
	case "hyperCutter", "destructiveMaw":
		if context.AbilityID == "destructiveMaw" {
			profile.WorldHook = "halfDamageExecuteAura"
		}
	case "refrigerate":
		profile.Conditions = append(profile.Conditions, status("stun", min(1, 0.2+0.02*float64(context.FossilCount)), 2000, 1, 1))
	case "corrosion", "corrosionMega":
		profile.ArmorRemovePct = 0.05
		if context.AbilityID == "corrosionMega" {
			profile.ArmorRemovePct, profile.WorldHook = 0.075, "virulentReef"
		}
	case "bubbleBeam", "hyperDrill", "icicleSpear", "twineedle", "twineedleMega":
		profile.BurstIntervalMS = 100
		switch context.AbilityID {
		case "bubbleBeam":
			profile.Burst = 8
		case "hyperDrill":
			profile.Burst = 3
			splash(profile, context, 0.5)
			profile.Conditions = append(profile.Conditions, slowCondition())
		case "icicleSpear":
			profile.Burst = 2 + min(3, int(context.Random*4))
			profile.Conditions = append(profile.Conditions, status("stun", 0.3, 2000, 1, 1))
			profile.ForcedCritical = context.TargetStunned
		case "twineedle":
			profile.Burst = 2
			condition := poisonCondition(1)
			condition.Chance = 0.5
			profile.Conditions = append(profile.Conditions, condition)
		case "twineedleMega":
			profile.Burst = 3
			condition := poisonCondition(1)
			condition.Chance = 0.33
			profile.Conditions = append(profile.Conditions, condition)
		}
	case "zapCannon":
		profile.Pierce, profile.AttackMode = 10000, "beam"
		profile.Conditions = append(profile.Conditions, status("stun", 0.3, 2000, 1, 1))
	case "waterCompaction":
		if isRain(context.WeatherID) || isSand(context.WeatherID) {
			profile.Power *= 1.3
			profile.CriticalPct *= 1.3
			profile.CriticalMultiplier *= 1.3
		}
	case "armorCannon":
		splash(profile, context, 0.5)
		profile.AttackMS -= 100 * float64(missing)
	case "noGuard":
		if context.Terrain == 4 {
			profile.Range *= 10
		}
		profile.DamageMultiplier *= 1 + 0.05*context.Distance/100
	case "sandRush":
		if isSand(context.WeatherID) {
			profile.AttackMS *= 0.5
		}
	case "featherDance":
		profile.Burst = max(1, min(20, context.StoredShots))
		profile.BurstIntervalMS = 100
	case "tailGlow":
		if context.HealedHearts > 0 {
			profile.Power *= 1.75
		}
	case "klutz":
		profile.ItemDisabled = true
	case "hyperBeam", "iceBeam", "machBolt":
		period := 5
		if context.AbilityID == "machBolt" {
			period = 40
			profile.WorldHook = "erraticStatic"
		}
		period = max(1, period-context.SpecialReduction)
		if (context.AttackCount+1)%period == 0 {
			profile.Pierce, profile.AttackMode = 10000, "beam"
			if context.AbilityID == "hyperBeam" {
				profile.DamageMultiplier *= 3
			} else {
				profile.Conditions = append(profile.Conditions, status("stun", 1, 2000, 1, 1))
			}
		}
	case "drySkin":
		if context.Terrain == 2 && isSun(context.WeatherID) {
			profile.Power *= 2
			if context.WeatherID == "extremelyHarshSunlight" {
				profile.Power *= 1.5
			}
		}
	case "marvelScale":
		multiplier := 1.0
		if context.Shape == "challenge" {
			multiplier = 3
		}
		profile.Power += float64(context.Ribbons) * multiplier
	case "diurnal":
		if context.Day {
			profile.Power *= 1.5
		}
	case "nocturnal":
		if !context.Day {
			profile.AttackMS *= 0.75
		}
	case "meteorMash":
		profile.Power += math.Floor(profile.Power * float64(min(6, context.AttackCount/5)) * .2)
	case "ancientPower", "ancientPowerPlus":
		if context.TargetArmor > 0 {
			multiplier := 0.15
			if context.AbilityID == "ancientPowerPlus" {
				multiplier = 0.2
			}
			profile.DamageMultiplier *= 1 + multiplier*float64(context.FossilCount)
		}
	case "bulkUp":
		profile.Power += 15 * float64(context.IdleMS) / 1000
	case "strongJaw":
		if context.TargetArmor <= 0 && context.TargetMaxArmor > 0 {
			profile.DamageMultiplier *= 2
		} else if context.TargetArmor > 0 {
			profile.CriticalPct = 100
		}
	case "meteorBeam":
		profile.Pierce, profile.AttackMode = 10000, "beam"
		if context.Shape == "orbit" {
			profile.Power *= 2
		}
	case "rivalryPower":
		if context.NearbyAllies > 0 {
			profile.Power *= 1.5
		}
	case "rivalrySpeed":
		if context.NearbyAllies > 0 {
			profile.AttackMS *= 0.7
		}
	case "boomburst", "liquidVoice", "originPulse", "charmedCharm", "drumBeating", "laundryGoRound", "emptyLight":
		profile.AttackMode = "wave"
		switch context.AbilityID {
		case "boomburst":
			profile.Burst = 3
			profile.Conditions = append(profile.Conditions, status("stun", 0.1, 1500, 1, 1))
		case "liquidVoice":
			profile.Burst = 2
			condition := slowCondition()
			condition.Chance = 0.25
			profile.Conditions = append(profile.Conditions, condition)
		case "originPulse":
			profile.Burst = 3
			condition := slowCondition()
			condition.Chance = 0.5
			profile.Conditions = append(profile.Conditions, condition)
			if context.TargetSlowed {
				profile.Conditions = append(profile.Conditions, status("stun", 0.5, 1000, 1, 1))
			}
		case "charmedCharm":
			profile.Burst = 2
			profile.Power *= 1 + 0.05*float64(context.HealedHearts)
			profile.WorldHook = "ragingBlossom"
		case "drumBeating":
			profile.Conditions = append(profile.Conditions, slowCondition())
		case "laundryGoRound":
			profile.Power += 25 * float64(context.AttackCount)
		case "emptyLight":
			profile.Burst = 2
			condition := poisonCondition(1)
			condition.Chance = 0.2
			profile.Conditions = append(profile.Conditions, condition, status("stun", min(1, 0.02*float64(context.UltraBeastKills)), 1000, 1, 1))
		}
	case "secretSword":
		profile.AttackMode, profile.RegenerationLoss = "sweep", 10
	case "voltSurge":
		profile.Ricochet = 3 + context.PastParadox
		profile.RicochetMultiplier = 1
	case "dragonDarts", "lavaPlume", "volcanicKaboom", "sludgeBomb", "diamondStorm", "rockThrow":
		profile.AttackMode = "bombardment"
		switch context.AbilityID {
		case "dragonDarts":
			profile.Burst = 2
		case "lavaPlume":
			splash(profile, context, 0.5)
			profile.Conditions = append(profile.Conditions, burnCondition())
		case "volcanicKaboom":
			profile.Burst = 3
			splash(profile, context, 0.5)
			profile.Conditions = append(profile.Conditions, burnCondition(), status("stun", 0.3, 1500, 1, 1))
		case "sludgeBomb":
			splash(profile, context, 0.5)
			profile.Conditions = append(profile.Conditions, poisonCondition(1))
		case "diamondStorm":
			splash(profile, context, 0.5)
			profile.SplashRadius *= 1 + min(2, 0.4*float64(context.HealedHearts))
		case "rockThrow":
			profile.Burst = max(1, context.FossilCount)
			splash(profile, context, 0.5)
		}
	case "supremeOverlord":
		profile.Power *= 1 + 0.1*float64(context.MissingAllies)
		profile.CriticalPct += 5 * float64(context.MissingAllies)
	case "bonemerang":
		profile.Pierce, profile.Returning = 10000, true
	case "signalBeam", "cryogenicCannon":
		profile.AttackMode = "trackingBeam"
		profile.BeamTickMS = 250
		if context.AbilityID == "cryogenicCannon" {
			profile.BeamTickMS = 500
			profile.Power += 50 * float64(context.PastParadox*context.SameTargetHits)
		}
	case "fireLash", "icyWind", "psychoCut", "psychoCutMega", "ceaselessEdge":
		profile.AttackMode = "sweep"
		if context.AbilityID == "fireLash" {
			profile.Conditions = append(profile.Conditions, burnCondition())
		}
		if context.AbilityID == "icyWind" {
			profile.Conditions = append(profile.Conditions, slowCondition())
		}
		if context.AbilityID == "ceaselessEdge" {
			profile.RepeatOnCritical, profile.CriticalPct = true, min(profile.CriticalPct, 90)
		}
	case "waterSpout":
		if context.TargetMaxHP > 0 {
			profile.DamageMultiplier *= max(0, context.TargetHP/context.TargetMaxHP)
		}
	case "focusEnergy":
		profile.CriticalPct += 5 * float64(context.AttackCount)
	case "sandForce":
		if isSand(context.WeatherID) {
			profile.Power *= 1.5
		}
	case "sandVeil":
		if isSand(context.WeatherID) {
			profile.Power *= 1.15
			profile.Range *= 1.15
			profile.CriticalPct *= 1.15
			profile.AttackMS /= 1.15
		}
	case "timeWarp":
		profile.CanMovePaused = true
	case "slowStart":
		if context.DeployedMS+int64(context.HeartsLost)*20000 >= 60000 {
			profile.Power *= 2
			profile.Range *= 2
			profile.AttackMS *= 0.5
		}
	case "steelBeam":
		profile.Pierce, profile.AttackMode, profile.WorldHook = 10000, "beam", "beamOverFiveLoseHeart"
	case "steamEruption":
		profile.Pierce, profile.AttackMode = 10000, "beam"
		profile.BeamDurationMS, profile.BeamTickMS = 220, 140
		profile.AuraDurationMS, profile.AuraTickMS, profile.AuraRadius = 3000, 500, 22
		profile.Conditions = append(profile.Conditions, burnCondition())
	case "grimNeigh":
		profile.CanSeeInvisible = true
		profile.Power += 3 * float64(context.WaveKills)
		profile.CriticalPct += float64(context.WaveKills)
	case "psybeam":
		profile.Pierce, profile.Burst, profile.AttackMode = 10000, 4, "orthogonalBeam"
	case "dazzlingGleam":
		profile.AttackMode = "cone"
		profile.Power += 50 * float64(context.FutureParadox)
		profile.Conditions = append(profile.Conditions, status("stun", 0.25, 1000, 1, 1))
	case "auroraVeil":
		if context.WeatherID == "hail" {
			profile.Range += 40
			profile.CriticalPct += 40
		}
	case "spicySpray":
		profile.TargetCount = 2
		profile.Conditions = append(profile.Conditions, burnCondition())
		if context.TargetBurned {
			profile.Conditions = append(profile.Conditions, status("scorch", 1, 10000, 0.01, 1))
		}
	case "knockOff":
		if context.TargetPassiveID != "" {
			profile.DamageMultiplier *= 2
		}
	case "berserk", "berserkMega":
		profile.AttackMode, profile.BeamTickMS, profile.TickDamageScale, profile.ConeHalfAngle = "cone", 100, 0.1, context.StreamAngle
		if context.AbilityID == "berserk" {
			if context.Hearts <= 7 {
				profile.Power *= 1.5
			}
		} else {
			profile.Power *= 1 + 0.2*float64(context.HeartsLost)
			profile.Range *= 1 + 0.2*float64(context.HeartsLost)
			profile.WeatherOverride = "rain"
		}
	case "supremeCommanders":
		profile.TargetCount, profile.WorldHook = 3, "commanderStats"
	case "supersonic":
	case "shadowClaw":
		profile.AttackMode = "sweep"
	case "teleport":
		profile.Power *= 1 + float64(context.Teleports)
		profile.ConsumeCharges, profile.MoveMode, profile.WorldHook = context.Teleports, "teleport", "teleportDestinations"
		profile.TeleportIntervalMS = 10000.0 / 3
	case "unburden":
		profile.MoveMode, profile.MovementSpeed, profile.ArrivalThreshold, profile.WorldHook = "wander", 50, 6, "flyingTowerMovement"
	case "trickOrTreat":
		if context.Random < 0.4 {
			profile.ProjectileOverride = "candy"
			if context.Random >= 0.2 {
				profile.ProjectileOverride = "candy1"
			}
			profile.CurseLink = true
			profile.Conditions = append(profile.Conditions, curseCondition())
		} else if context.Random < 0.8 {
			profile.ProjectileOverride = "spinarak"
			ricochet(profile, context)
			profile.Ricochet += 2
			profile.Conditions = append(profile.Conditions, poisonCondition(1))
		} else {
			profile.ProjectileOverride = "apple"
			splash(profile, context, 0.5)
		}
	case "transform":
		profile.WorldHook = "copyFirstSpeciesAtOwnLevel"
	case "scheme":
		profile.ShinyChance, profile.WorldHook = 1.0/9000, "finalEvolutionShiny"
	case "grassPlatform", "icePlatform", "rest":
		profile.PassengerTerrains, profile.PassengerAttackMS = []int{1, 2}, 0.85
		profile.PassengerTerrain = 2
		if context.AbilityID == "icePlatform" {
			profile.PassengerTerrains, profile.PassengerTerrain = []int{1, 3}, 3
		}
		if context.AbilityID == "rest" {
			profile.PassengerTerrains, profile.PassengerTerrain, profile.PassengerAttackMS = []int{1, 2, 3, 4}, 0, 1
		}
		profile.WorldHook = "passengerPlacement"
	case "mount", "commander":
		profile.WorldHook = "commanderStats"
	case "plus", "minus":
		profile.WorldHook = "oppositeChargeLink"
	case "shellSmash":
		profile.HeartCost, profile.WorldHook = 1, "waveStartHeartCost"
		if context.ShellSmashActive {
			profile.Power *= 1.5
			profile.AttackMS *= 0.75
		}
	case "cloudNine":
		profile.SuppressWeather, profile.IgnoreWeatherCost, profile.WorldHook = true, true, "preventEnemyWeatherBonuses"
	case "pixilate":
		if context.WeatherID != "" {
			profile.WeatherOverride = "starryNight"
		}
	case "heavyMetal":
		profile.AuraCondition = status("stun", 1, 2000, 1, 1)
		profile.WorldHook = "deployGlobalStunAndMovementLock"
	case "squishyToy":
		profile.WorldHook = "allyIdleTarget"
	case "honeyGather":
		profile.WorldHook = "addHoneyOnAttack"
	case "intrepidSword", "dauntlessShield":
		profile.AttackMode, profile.RequiredCharges, profile.WorldHook = "sweep", 50, "chargeBoostMeter"
		if context.BoostRemainingMS > 0 {
			if context.AbilityID == "intrepidSword" {
				profile.Range *= 1.25
				profile.CriticalPct = 100
			} else {
				profile.AttackMS *= 0.5
				if context.TargetArmor > 0 {
					profile.DamageMultiplier *= 2
				}
			}
		}
	case "aggressiveNature":
		profile.WorldHook = "attackWithFutureParadoxAlly"
	case "anchorShot":
		profile.AttackMode, profile.TargetCount, profile.WorldHook = "hook", 6, "tetherTargetsInRange"
	case "stickyWeb":
		profile.AttackMode, profile.WorldHook = "webProjectile", "persistentWebZone"
		profile.Conditions = append(profile.Conditions, slowCondition())
	case "electroBall":
		profile.DamageMultiplier *= 1 + 0.5*max(0, float64(context.FlightMS)/1000-1)
		splash(profile, context, 0.5)
		profile.Conditions = append(profile.Conditions, status("stun", 1, 2000, 1, 1))
	case "unnerve":
		profile.BerryEffectScale, profile.WorldHook = 1.1, "globalBerryEffect"
	case "harvest":
		profile.BerryDurationScale, profile.WorldHook = 2, "globalBerryDuration"
	case "ripen":
		profile.BerryEffectScale = 2
	case "gluttony":
		profile.BerryAtWaveStart, profile.WorldHook = true, "globalBerryWaveStart"
	case "uTurn":
		profile.AttackMode, profile.OncePerTarget, profile.MoveMode = "chainCharge", true, "enemyChain"
	case "bugBite":
		profile.BerryAfterAttack = true
	case "discharge":
		profile.AuraCondition, profile.TriggerDelayMS, profile.OncePerTarget, profile.WorldHook = status("stun", 1, 2000, 1, 1), 3000, true, "continuousRangeExposure"
	case "flameBody":
		profile.AuraCondition, profile.HealBlock, profile.WorldHook = burnCondition(), true, "continuousRangeConditions"
	case "blizzard":
		profile.AuraCondition, profile.WorldHook = status("slow", 1, 40, 0.2, 1), "continuousRangeConditions"
	case "sturdy":
		profile.HeartProtection, profile.WorldHook = 1, "firstWaveHeartLoss"
	case "iceBody":
		profile.WorldHook = "firstHeartLossDoubleHail"
	case "bulletPunch", "bulletPunchMega":
		profile.RequiredCharges, profile.WorldHook = 10, "idleRechargeCharges"
		if context.AbilityID == "bulletPunchMega" {
			profile.RequiredCharges, profile.WorldHook = 5, "idleRechargeChargesAndKillRefill"
		}
		profile.Disabled = context.Charges < profile.RequiredCharges
		if !profile.Disabled {
			profile.Burst, profile.ConsumeCharges, profile.BurstIntervalMS = context.Charges, context.Charges, 100
		}
	case "sacredFire":
		profile.Orbit, profile.MoveMode, profile.BeamTickMS = true, "pursueEnemy", 500
		profile.Conditions = append(profile.Conditions, burnCondition())
		if context.TargetNightmares > 0 {
			profile.DamageMultiplier *= float64(context.TargetNightmares)
			profile.ConsumeNightmare = true
		}
	case "lightningRod":
		profile.Power *= 1 + 0.1*float64(context.Charges)
		profile.WorldHook = "chargeOnNearbyStun"
		if context.Charges >= 10 {
			splash(profile, context, 0.5)
			profile.ConsumeCharges = context.Charges
		}
	case "wildboltStorm":
		weather := context.WeatherID
		if weather == "rain" || weather == "heavyRain" {
			profile.WeatherOverride, weather = "thunderStorm", "thunderStorm"
		}
		profile.Disabled = weather != "thunderStorm"
		profile.AttackMode, profile.Burst = "lightningArea", 3+min(4, int(context.Random*5))
	case "sandsearStorm":
		weather := context.WeatherID
		if weather == "sandstorm" {
			profile.WeatherOverride, weather = "sandAvalanche", "sandAvalanche"
		}
		profile.Disabled = weather != "sandAvalanche"
		profile.AttackMode, profile.Pierce = "dune", 10000
	case "bleakwindStorm":
		profile.WeatherOverride, profile.AttackMode, profile.Pierce, profile.MoveMode = "strongWinds", "tornado", 10000, "oscillating"
		profile.WorldHook = "windDisplacement"
	case "fairySingularity":
		profile.AttackMode, profile.TriggerDelayMS, profile.WorldHook = "delayedArea", 1000, "pullToProjectile"
		splash(profile, context, 1)
	case "whiteRay":
		profile.TargetCount = 2
		profile.Power *= 1 + 0.01*float64(context.UltraBeastKills)
	case "explosion":
		profile.Disabled, profile.HeartCost, profile.WorldHook = context.Targets < 5, 2, "countdownExplosion"
		profile.AttackMode = "area"
		profile.Conditions = append(profile.Conditions, status("stun", 1, 3000, 1, 1))
	case "damp":
		profile.WorldHook = "preventExplosionHeartCost"
	case "aerialAce", "stalwart":
		profile.MoveMode = "circleOrigin"
		if context.AbilityID == "stalwart" {
			profile.MoveMode = "circleTarget"
		}
		profile.WorldHook = "flyingTowerMovement"
	case "amnesia":
		profile.Power *= 1 + float64(context.PowerStacks)
		profile.Disabled = context.ItemID == "laggingTail" || context.Random < 0.5
		profile.WorldHook = "storeAmnesiaPower"
	case "spiritShackle":
		profile.Burst, profile.WorldHook = 3, "impactSkyArrows"
		splash(profile, context, 0.5)
	case "flowerTrick":
		profile.RepeatOnCritical, profile.CriticalPct = true, min(profile.CriticalPct, 95)
	default:
		return fmt.Errorf("ability %q needs an independently implemented world hook", context.AbilityID)
	}
	return nil
}

func applyWeather(profile *AttackProfile, context AbilityContext) error {
	if profile.WeatherOverride != "" {
		context.WeatherID = profile.WeatherOverride
	}
	protected := context.CloudNine || profile.IgnoreWeatherCost
	switch context.WeatherID {
	case "":
	case "rain":
		if context.Terrain == 3 {
			profile.DamageMultiplier *= 1.2
		}
	case "heavyRain":
		if context.Terrain == 3 && !protected {
			profile.DamageMultiplier *= 0.5
		}
	case "thunderStorm":
		if context.Terrain == 3 {
			profile.DamageMultiplier *= 1.25
		}
	case "harshSunlight":
		if context.Terrain == 2 && !protected {
			profile.AttackMS /= 0.85
		}
	case "extremelyHarshSunlight":
		if context.Terrain == 2 {
			profile.AttackMS /= 3
		}
	case "hail":
		profile.CriticalMultiplier *= 1.25
	case "sandstorm", "sandAvalanche", "strongWinds", "fog":
	case "starryNight":
		profile.Power += float64(context.Stars) / 50
	default:
		return fmt.Errorf("unknown weather %q", context.WeatherID)
	}
	return nil
}

func applyItem(profile *AttackProfile, context AbilityContext) error {
	if context.ItemID == "" {
		return nil
	}
	if profile.ItemDisabled {
		return fmt.Errorf("ability %q cannot hold items", context.AbilityID)
	}
	scale := profile.ItemEffectScale
	if scale == 0 {
		scale = 1
	}
	original := *profile
	modify := func(base, factor float64) float64 {
		return base * factor
	}
	switch context.ItemID {
	case "protein":
		profile.Power += 15
	case "carbos":
		profile.AttackMS = modify(profile.AttackMS, 0.9)
	case "iron":
		profile.CriticalPct += 10
	case "zinc":
		profile.Range = modify(profile.Range, 1.1)
	case "lifeOrb":
		if !context.HealingUnit {
			return fmt.Errorf("lifeOrb requires a healing ability")
		}
		profile.Power = modify(profile.Power, 1.5)
		profile.AttackMS = modify(profile.AttackMS, 0.5)
		profile.HeartChance = 0
	case "lightClay":
		for index := range profile.Conditions {
			if profile.Conditions[index].Kind == "slow" || profile.Conditions[index].Kind == "stun" {
				profile.Conditions[index].DurationMS = int64(float64(profile.Conditions[index].DurationMS) * 1.1)
			}
		}
	case "fertiliser":
		if context.Terrain == 1 {
			adjusted := context
			adjusted.Terrain, adjusted.ItemID = 2, ""
			base, err := BuildAttackProfile(adjusted)
			if err != nil {
				return err
			}
			*profile = base
		}
	case "squirtBottle":
		if context.Terrain == 1 {
			adjusted := context
			adjusted.Terrain, adjusted.ItemID = 3, ""
			base, err := BuildAttackProfile(adjusted)
			if err != nil {
				return err
			}
			*profile = base
		}
	case "wideLens":
	case "softSand":
		profile.SoftSand = true
	case "choiceScarf":
		removed := max(0, profile.TargetCount-1)
		profile.TargetCount = 1
		profile.AttackMS *= math.Pow(0.5, float64(removed))
	case "oldRod":
		profile.Range += 75
	case "heatRock":
		for index := range profile.Conditions {
			if profile.Conditions[index].Kind == "burn" {
				profile.Conditions[index].Chance = 1
			}
		}
	case "shieldBreakerBullet":
		profile.AttackMS += 2000
		if context.TargetArmor > 0 {
			profile.DamageMultiplier *= 2
		}
	case "stickyBarb":
		if context.TargetSlowed || context.TargetStunned {
			profile.DamageMultiplier *= 1.3
		}
	case "starCandy":
		profile.Range += float64(context.Stars) * 0.035
	case "electirizer":
		if context.TargetStunned {
			profile.DamageMultiplier *= 1.3
		}
	case "cellBattery":
		profile.Power *= 1.5
		profile.Conditions = append(profile.Conditions, status("stun", 1, 100, 1, 1))
	case "leek":
		profile.CriticalPct *= 2
		profile.CriticalMultiplier *= 2
	case "thickClub", "lightBall":
		profile.Power *= 1.5
	case "nanabBerry":
		profile.Range *= 1.3
		profile.Power *= 1.5
		profile.AttackMS *= 1.2
	case "bigRoot":
		profile.HeartAmount *= 2
	case "sprayduck":
		splash(profile, context, 0.5)
	case "razorClaw":
		addWorldHook(profile, "criticalSlow200")
	case "spellTag":
		count := 0
		for _, active := range []bool{context.TargetBurned, context.TargetStunned, context.TargetSlowed, context.TargetPoisoned} {
			if active {
				count++
			}
		}
		profile.DamageMultiplier *= 1 + min(0.5, float64(count)*0.15)
	case "clawFossil":
		profile.Power *= 1 + 0.1*float64(context.FossilCount)
	case "sniperScope":
		if context.Distance > 150 {
			profile.DamageMultiplier *= 1.25
		} else {
			profile.DamageMultiplier *= 0.75
		}
	case "loadedDice":
		profile.Power *= 1 + 0.6*float64(profile.Ricochet)
		profile.Ricochet = 0
	case "quickPowder":
		profile.Power *= 0.5
		profile.AttackMS *= 0.75
	case "metalPowder":
		profile.Power *= 1.5
		profile.AttackMS *= 1.25
	case "hardStone":
		profile.Power *= 1.25
	case "domeFossil":
		profile.CriticalPct += 5 * float64(context.FossilCount)
	case "rockyHelmet":
		if context.AbilityID == "rampardos" {
			missing := max(0, context.MaxHearts-context.Hearts)
			profile.Power = context.Power * (1 + 0.1*float64(missing))
		}
	case "berryJuice":
		for index := range profile.Conditions {
			if profile.Conditions[index].Kind == "slow" {
				profile.Conditions[index].Magnitude *= 1.25
			}
		}
	case "blueBandana":
		profile.Power *= 1 + profile.CriticalPct*0.01
		profile.CriticalPct = 0
	case "adrenalineOrb":
		profile.AttackMS *= max(0, 1-0.025*float64(max(0, context.MaxHearts-context.Hearts)))
	case "zoomLens":
		profile.TargetCount++
		profile.SourcePowerBonusPct = -0.5
	case "metronome":
		profile.RicochetMultiplier = 0.875
	case "helixFossil":
		profile.Range += 10 * float64(context.FossilCount)
	case "spindaCocktail":
		profile.Range *= 1.25
		profile.RandomTarget = true
	case "dragonFang":
		profile.SplashRadius *= 3
	case "poisonBarb":
		profile.AttackMS *= 0.8
		condition := poisonCondition(1)
		condition.Chance = 0.5
		profile.Conditions = append(profile.Conditions, condition)
	case "silphScope":
		if profile.CanSeeInvisible {
			profile.Range += 15
			profile.AttackMS *= 0.75
			profile.Power += 175
		}
		profile.CanSeeInvisible = true
	case "quickClaw":
		profile.SourcePowerBonusPct = -0.5
		profile.AttackMS *= 0.5
	case "laggingTail":
		profile.SourcePowerBonusPct = 0.5
		profile.AttackMS *= 1.5
	case "revelationAroma":
		profile.Range += 25
	case "falmeOrb":
		for index := range profile.Conditions {
			if profile.Conditions[index].Kind == "burn" {
				profile.Conditions[index].Magnitude *= 1.5
			}
		}
	case "badgeOfHonor":
		profile.Power *= 1 + min(0.3, float64(context.Stars/30)*0.01)
	case "toxicOrb":
		profile.Conditions = append(profile.Conditions, poisonCondition(1))
	case "stretchySpring":
		profile.Ricochet += 2
	case "blackGlasses":
		if context.AbilityID == "moxie" {
			profile.Power = context.Power * (1 + 0.1*float64(context.WaveKills))
		}
	case "ancientSword":
		profile.Power *= 1.2
		profile.AttackMS *= 0.8
	case "ancientShield":
		profile.Power *= 1.2
		profile.Range *= 1.2
	case "wrestlingMask":
		profile.Range -= 75
		profile.AttackMS *= 0.65
	case "magmarizer":
		if context.TargetBurned {
			profile.DamageMultiplier *= 1.3
		}
	case "xAttack":
		profile.Power += 75
	case "xSpeed":
		profile.AttackMS *= 0.8
	case "weaknessPolicy":
		profile.Power *= 2.5
		profile.SplashRadius = 0
	case "muscleBand":
		profile.AttackMS *= 1.25
		profile.SplashMultiplier = 1
	case "litCoal":
		condition := burnCondition()
		condition.Chance = 0.1
		profile.Conditions = append(profile.Conditions, condition)
	case "magnet":
		if context.TargetArmor > 0 {
			condition := slowCondition()
			condition.DurationMS = 1000
			profile.Conditions = append(profile.Conditions, condition)
		}
	case "leftovers":
		if profile.Ricochet <= 0 {
			profile.HeartChance, profile.HeartLimit = 0.01, 1
		}
	case "hikingKit":
		if context.Terrain == 1 {
			adjusted := context
			adjusted.Terrain, adjusted.ItemID = 4, ""
			base, err := BuildAttackProfile(adjusted)
			if err != nil {
				return err
			}
			*profile = base
		}
	case "bindingBand":
		profile.AttackMS += 1500
		for index := range profile.Conditions {
			if profile.Conditions[index].Kind == "slow" {
				profile.Conditions[index].IgnoreResistance = true
			}
		}
	case "direHit":
		profile.CriticalPct += 25
	case "clover":
		profile.CriticalMultiplier *= 1.5
	case "sunflowerPetal":
		profile.Aura.PowerMultiplier += 0.15
		profile.Range -= 40
	case "cherryBlossom":
		profile.TargetCount++
	case "focusBand":
		profile.DamageMultiplier *= 1 + 0.05*float64(context.SameTargetHits)
	case "magmaStone":
		for index := range profile.Conditions {
			if profile.Conditions[index].Kind == "burn" {
				profile.Conditions[index].DurationMS = 20000
			}
		}
	case "jadeOrb":
		profile.OrbitalCount += 2
	case "lustrousOrb":
		profile.OrbitSpeed *= 2
	case "safetyGoggles":
		profile.IgnoreWeatherCost = true
	case "crunchies":
		profile.Power *= 0.6
		profile.SplashOnCritical = true
		splash(profile, context, 0.5)
	case "ovalCharm":
		profile.Power *= 1 + 0.125*float64(context.MissingAllies)
	case "dampRock", "smoothRock", "icyRock", "heatRockWeather":
		profile.AttackMS *= 2
		profile.WeatherOverride = map[string]string{"dampRock": "rain", "smoothRock": "sandstorm", "icyRock": "hail", "heatRockWeather": "harshSunlight"}[context.ItemID]
	case "sailFossil":
		for _, condition := range profile.Conditions {
			if condition.Kind == "stun" && condition.Chance < 1 {
				fallback := slowCondition()
				fallback.DurationMS, fallback.MinimumRoll, fallback.Chance = 500, condition.Chance, 1-condition.Chance
				profile.Conditions = append(profile.Conditions, fallback)
				break
			}
		}
	case "maliciousArmor":
		splash(profile, context, 0.5)
		profile.AttackMS -= 20 * float64(max(0, context.MaxHearts-context.Hearts))
	case "auspiciousArmor":
		profile.TargetCount = 2
		if context.HealingUses < 3 {
			profile.HeartChance, profile.HeartLimit = 0.002, 1
		}
	case "rockyHelmetSpikes":
		profile.AuraTickMS = 250
	case "terrainExtender":
		profile.BeamWidthScale *= 2
	case "scovillainSiracha":
		profile.Power *= 0.5
		profile.Conditions = append(profile.Conditions, burnCondition())
	case "fullIncense":
		profile.Power += math.Floor(2.5 * float64(context.WaveElapsedMS/1000))
	case "eviolite":
		if context.Level > 50 {
			return fmt.Errorf("eviolite requires level 50 or lower")
		}
		profile.SourcePowerBonusPct = 0.2
		profile.Range *= 1.2
		profile.AttackMS *= 0.8
	case "koffingJelly":
		profile.AttackMS += 1100
		profile.Conditions = append(profile.Conditions, poisonCondition(1))
	case "luminousMoss":
		profile.Range += 25
	case "sootheBell":
		profile.Burst += 2
	case "propCase":
		profile.HeartInverted = true
		addWorldHook(profile, "restoreInvertedHeartsAtWaveEnd")
	case "boiledEgg":
		if context.AbilityID == "featherDance" {
			profile.Burst = max(1, min(30, context.StoredShots))
		}
	case "nectar":
		profile.HoneyMultiplier = 3
	case "salacBerry", "liechiBerry", "lansatBerry", "starfBerry":
		addWorldHook(profile, "berryOnHeartLoss")
		if context.BerryActive {
			switch context.ItemID {
			case "salacBerry":
				profile.AttackMS *= 0.5
			case "liechiBerry":
				profile.Power += 200
			case "lansatBerry":
				profile.CriticalPct += 60
			case "starfBerry":
				profile.Range += 80
			}
		}
	case "oddKeystone":
		profile.Power += 40
		addWorldHook(profile, "orbitalRangeDecay")
	case "expertBelt":
		profile.Power += 50
		profile.CriticalPct += 10
	case "brickMail":
		profile.Range += 8 * float64(context.MountainAllies)
	case "tropicalMail":
		profile.Power += 20 * float64(context.GrassAllies)
	case "waveMail":
		profile.AttackMS *= max(0, 1-0.03*float64(context.WaterAllies))
	case "prismScale":
		profile.Range += 0.5 * float64(context.Ribbons)
	case "metalCoat":
		for index := range profile.Conditions {
			if profile.Conditions[index].Kind == "stun" {
				profile.Conditions[index].DurationMS *= 2
			}
		}
	case "metalCoatReal":
		profile.Power -= 250
		profile.AttackMS -= 250
		profile.Technician = true
	case "powerHerb":
	case "shinyCharm":
		if context.AbilityID == "scheme" {
			profile.ShinyChance = 1.0 / 3000
		}
	case "roomService":
		profile.Aura.PowerFlat += 20 * float64(context.HealedHearts)
		addWorldHook(profile, "globalRoomServicePower")
	case "ridePager":
		profile.PassengerPower = 50
	case "wacanBerry":
		profile.Aura.PowerMultiplier = 1.2
	case "pokeSkates":
		profile.MoveMode = "nudge"
		addWorldHook(profile, "nearbyTowerMovement")
	case "berryPouch":
		addWorldHook(profile, "receivePassengerBerry")
	case "luckyPunch":
		if context.SinceCriticalMS < 6000 {
			profile.Range += 10 * float64(min(5, context.CriticalStreak))
		}
	case "tropicalPetal":
		profile.Aura.AttackMultiplier -= 0.1
		profile.Range -= 20
	case "heartScale":
		profile.AttackMS *= 1 - 0.1*float64(min(5, context.HealedHearts))
	case "clefairyDoll", "shellBell":
		profile.HeartDamageGoal, profile.HeartLimit = 25000, 1
		if context.ItemID == "shellBell" {
			profile.HeartDamageGoal = 50000
		}
		addWorldHook(profile, "waveDamageHealing")
	case "amuletCoin":
		profile.GoldDamageFraction = 0.0005 * float64(context.Stars)
	case "sharpBeak":
		if context.Terrain == 4 && profile.Range > 0 {
			profile.DamageMultiplier *= 1 + min(0.5, max(0, 0.5*(1-context.Distance/profile.Range)))
		}
	case "strangeIdol":
		profile.StrangeIdol = true
	case "nightmareCloth":
		profile.SpreadNightmare, profile.NightmareSpreadRadius = true, 65
		addWorldHook(profile, "spreadNightmare")
	case "condensedBlizzard":
		profile.Range *= 0.5
		profile.RangeShape = "circle"
	case "spindasSpecialDelivery":
		choices := []ConditionSpec{burnCondition(), slowCondition(), status("stun", 1, 2000, 1, 1), poisonCondition(1)}
		condition := choices[min(3, int(context.Random*4))]
		condition.Chance = 0.06
		profile.Conditions = append(profile.Conditions, condition)
	case "ejectButton":
		profile.TeleportIntervalMS *= 0.5
		addWorldHook(profile, "halfTeleportCooldown")
	case "blimpKeys":
		profile.MoveMode, profile.MovementSpeed = "followPointer", 80
	case "twistedSpoon":
		profile.MoveMode = "teleportInPlace"
		profile.Power = context.Power
		if context.Teleports > 0 {
			profile.ExtraDamagePct = 0.25 * float64(context.Teleports+1)
			profile.ExtraDamageCeil = true
		}
	case "inverter":
		profile.Range, profile.InnerRange, profile.RangeShape = 300, 150, "donut"
		profile.AttackMS *= 0.25
		profile.CriticalPct += 15
		profile.ExtraDamagePct, profile.ExtraDamageCeil = 0.5, true
	default:
		if strings.Contains(" "+progressionItemIDs+" ", " "+context.ItemID+" ") {
			profile.ItemStage = "progression"
		} else {
			return fmt.Errorf("item %q needs an independently implemented rule", context.ItemID)
		}
	}
	negative := profile.Power < original.Power || profile.Range < original.Range || profile.AttackMS > original.AttackMS || profile.CriticalPct < original.CriticalPct || profile.CriticalMultiplier < original.CriticalMultiplier || profile.SourcePowerBonusPct < original.SourcePowerBonusPct
	adjust := func(base, value float64, smallerIsBetter bool) float64 {
		delta := value - base
		if scale < 0 && (delta < 0 && !smallerIsBetter || delta > 0 && smallerIsBetter) {
			delta = -delta
		}
		return base + delta*math.Abs(scale)
	}
	profile.Power = adjust(original.Power, profile.Power, false)
	profile.Range = adjust(original.Range, profile.Range, false)
	profile.AttackMS = adjust(original.AttackMS, profile.AttackMS, true)
	profile.CriticalPct = adjust(original.CriticalPct, profile.CriticalPct, false)
	profile.CriticalMultiplier = adjust(original.CriticalMultiplier, profile.CriticalMultiplier, false)
	profile.SourcePowerBonusPct = adjust(original.SourcePowerBonusPct, profile.SourcePowerBonusPct, false)
	if context.AbilityID == "defiant" && (negative || context.HasNegativeItem) {
		profile.Power = max(original.Power, profile.Power) + 500
		profile.SourcePowerBonusPct = max(original.SourcePowerBonusPct, profile.SourcePowerBonusPct)
	}
	return nil
}

func BuildAttackProfile(context AbilityContext) (AttackProfile, error) {
	if context.MaxHearts <= 0 {
		context.MaxHearts = 14
	}
	if context.AttackMS <= 0 {
		context.AttackMS = 1000
	}
	if context.ItemID == "powerHerb" {
		context.SpecialReduction = 2
	}
	if context.Random < 0 || context.Random >= 1 || !finiteNumber(context.Random) {
		return AttackProfile{}, fmt.Errorf("invalid supplied ability random result")
	}
	profile := AttackProfile{Power: context.Power, Range: context.Range, AttackMS: context.AttackMS, CriticalPct: context.CriticalPct, CriticalMultiplier: 1.5, DamageMultiplier: 1, TargetCount: 1, SplashMultiplier: 0.5, RicochetMultiplier: 0.7, Burst: 1, OrbitSpeed: 1, HeartAmount: 1, ItemEffectScale: 1, BerryEffectScale: 1, BerryDurationScale: 1, BeamWidthScale: 1, HoneyMultiplier: 1, TickDamageScale: 1, AttackMode: context.AttackType, Aura: AuraSpec{PowerMultiplier: 1, AttackMultiplier: 1, RangeMultiplier: 1, CriticalDamageMultiplier: 1}}
	if err := applyAbility(&profile, context); err != nil {
		return AttackProfile{}, err
	}
	if err := applyItem(&profile, context); err != nil {
		return AttackProfile{}, err
	}
	switch context.AbilityID {
	case "pollenPuff":
		profile.SplashRadius += float64(context.HoneyStacks) * profile.HoneyMultiplier
	case "auraSphere":
		profile.SplashRadius += 6.5 * float64(max(0, context.MaxHearts-context.Hearts))
	case "autotomize":
		profile.SplashRadius += .65 * float64(context.UltraBeastKills)
	}
	for index := range profile.Conditions {
		profile.Conditions[index].SourcePokemonID = context.PokemonID
		if profile.Conditions[index].Kind == "nightmare" {
			profile.Conditions[index].Magnitude = profile.Power * 0.2
		}
	}
	if err := applyWeather(&profile, context); err != nil {
		return AttackProfile{}, err
	}
	switch context.TargetPassiveID {
	case "flameBody":
		if context.Distance < 120 {
			profile.DamageMultiplier *= 0.5
		}
	case "static":
		if context.Distance <= 115 && context.Random < 0.25 {
			profile.Disabled, profile.AttackFailure, profile.FailureConsumesCooldown = true, "static", true
		}
	case "snowCloak":
		if context.Distance <= 160 {
			profile.AttackMS *= 1.5
		}
	case "psychogenesis":
		if context.ItemID == "" || context.AbilityID == "scrappy" {
			profile.DamageMultiplier *= 1.15
		} else {
			profile.DamageMultiplier *= 0.65
		}
	case "abyssalBound":
		if context.Terrain != 3 && context.ItemID != "squirtBottle" {
			profile.Disabled = true
		}
	case "weakArmor":
		if strings.Contains(" armorBreak armorBreakDoubleShot armorBreakSplash rageFist piercingDrill ancientPower ancientPowerPlus ", " "+context.AbilityID+" ") {
			profile.DamageMultiplier *= 1.5
		}
	}
	profile.AttackMS = max(50, profile.AttackMS)
	profile.Range = max(0, profile.Range)
	profile.CriticalPct = min(100, max(0, profile.CriticalPct))
	profile.TargetCount = max(1, profile.TargetCount)
	profile.DamageMultiplier = max(0, profile.DamageMultiplier)
	profile.Power = max(0, profile.Power)
	profile.Power += profile.FixedPower
	return profile, nil
}

func RetargetAttackProfile(frozen AttackProfile, launch, current AbilityContext) (AttackProfile, error) {
	base, err := BuildAttackProfile(launch)
	if err != nil {
		return AttackProfile{}, err
	}
	live := launch
	live.TargetHP, live.TargetMaxHP = current.TargetHP, current.TargetMaxHP
	live.TargetArmor, live.TargetMaxArmor = current.TargetArmor, current.TargetMaxArmor
	live.TargetSpeed, live.TargetPassiveID = current.TargetSpeed, current.TargetPassiveID
	live.TargetInvisible, live.TargetBurned = current.TargetInvisible, current.TargetBurned
	live.TargetBurnDurationMS = current.TargetBurnDurationMS
	live.TargetPoisoned, live.TargetStunned = current.TargetPoisoned, current.TargetStunned
	live.TargetSlowed, live.TargetCursed = current.TargetSlowed, current.TargetCursed
	live.TargetNightmares, live.Distance, live.FlightMS = current.TargetNightmares, current.Distance, current.FlightMS
	live.CursedEnemies = current.CursedEnemies
	if launch.AbilityID == "teleport" {
		live.Teleports = current.Teleports
	}
	if launch.AbilityID == "amnesia" {
		live.PowerStacks = current.PowerStacks
	}
	if launch.AbilityID == "makeItRain" {
		live.TotalGoldDigits = current.TotalGoldDigits
	}
	profile, err := BuildAttackProfile(live)
	if err != nil {
		return AttackProfile{}, err
	}
	if launch.AbilityID == "teleport" || launch.AbilityID == "amnesia" || launch.AbilityID == "makeItRain" {
		if frozen.SourcePowerScale != 0 {
			profile.Power = profile.Power*frozen.SourcePowerScale + frozen.SourcePowerFlat
		} else {
			profile.Power += frozen.Power - base.Power
		}
	} else {
		profile.Power = frozen.Power
	}
	profile.Range, profile.AttackMS = frozen.Range, frozen.AttackMS
	profile.CriticalPct = frozen.CriticalPct + profile.CriticalPct - base.CriticalPct
	profile.CriticalMultiplier = frozen.CriticalMultiplier
	profile.CanSeeInvisible = frozen.CanSeeInvisible
	if base.DamageMultiplier != 0 {
		profile.DamageMultiplier *= frozen.DamageMultiplier / base.DamageMultiplier
	}
	for index := range profile.Conditions {
		if profile.Conditions[index].Kind == "nightmare" {
			profile.Conditions[index].Magnitude = frozen.Power * .2
		}
	}
	return profile, nil
}

func FinalizeAttackDamage(profile AttackProfile, amount float64) float64 {
	if !finiteNumber(amount) || amount < 0 {
		return 0
	}
	if profile.Technician && amount <= 1000 {
		amount = math.Ceil(amount * 1.5)
	}
	return amount
}

func SoftSandDamage(amount float64, targets int) float64 {
	if !finiteNumber(amount) || amount < 0 {
		return 0
	}
	if targets <= 0 {
		return amount
	}
	bonus := max(1, 400*math.Pow(.5, float64(targets-1)))
	return amount + math.Floor(amount*bonus/100)
}

func StrangeIdolDamage(amount float64, cursedEnemies int) float64 {
	if !finiteNumber(amount) || amount < 0 {
		return 0
	}
	bonus := max(0, 50-cursedEnemies)
	return amount + math.Ceil(amount*float64(bonus)/100)
}

func addWorldHook(profile *AttackProfile, hook string) {
	if hook == "" {
		return
	}
	if profile.WorldHook == "" {
		profile.WorldHook = hook
		return
	}
	for _, existing := range AllWorldHooks(*profile) {
		if existing == hook {
			return
		}
	}
	profile.ExtraWorldHooks = append(profile.ExtraWorldHooks, hook)
}

func AllWorldHooks(profile AttackProfile) []string {
	hooks := []string{}
	if profile.WorldHook != "" {
		hooks = append(hooks, profile.WorldHook)
	}
	return append(hooks, profile.ExtraWorldHooks...)
}

func ImplementedAbilities() []string {
	ids := strings.Fields(implementedAbilityIDs)
	return append([]string(nil), ids...)
}

func AuditAbilityRules(ids []string, supportedWorldHooks []string) []RuleCoverage {
	ordered := append([]string(nil), ids...)
	sort.Strings(ordered)
	supported := map[string]bool{}
	for _, hook := range supportedWorldHooks {
		supported[hook] = true
	}
	output := make([]RuleCoverage, 0, len(ordered))
	for _, id := range ordered {
		context := AbilityContext{AbilityID: id, Level: 50, Terrain: 1, Hearts: 14, MaxHearts: 14, Power: 100, Range: 120, AttackMS: 1000, Targets: 10, EffectsVolume: 8, Random: 0.2}
		profile, err := BuildAttackProfile(context)
		rule := RuleCoverage{ID: id, Stage: "profile"}
		if err != nil {
			rule.Stage, rule.RemainingFacts = "unsupported", err.Error()
			output = append(output, rule)
			continue
		}
		rule.Implemented = profile.WorldHook == "" || supported[profile.WorldHook]
		rule.WorldHook = profile.WorldHook
		if !rule.Implemented {
			rule.Stage, rule.RemainingFacts = "world-hook", "world event integration and end-to-end behavior still required"
		}
		if details := unresolvedAbilityFacts[id]; details != "" {
			rule.Implemented, rule.RemainingFacts, rule.Stage = false, details, "fact-gap"
		}
		output = append(output, rule)
	}
	return output
}

func AuditItemProfiles() ([]ItemProfileCoverage, error) {
	state, err := New(Options{Starter: "charmander"}, 1, 27183, 1794916800000)
	if err != nil {
		return nil, err
	}
	catalog := CatalogData()
	speciesKeys := make([]string, 0, len(catalog.Species))
	for key := range catalog.Species {
		speciesKeys = append(speciesKeys, key)
	}
	sort.Strings(speciesKeys)
	recipients := []Pokemon{}
	for _, key := range speciesKeys {
		for _, level := range []int{1, 50, 75, 100} {
			pokemon, createErr := newPokemon(state, key, level)
			if createErr != nil {
				return nil, fmt.Errorf("construct item recipient %s level %d: %w", key, level, createErr)
			}
			recipients = append(recipients, pokemon)
		}
	}
	keys := make([]string, 0, len(catalog.Items))
	for key := range catalog.Items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	removed := map[string]bool{}
	for _, key := range catalog.RemovedItemIDs {
		removed[key] = true
	}
	route, _ := routeForState(state)
	output := make([]ItemProfileCoverage, 0, len(keys))
	for _, key := range keys {
		item := catalog.Items[key]
		entry := ItemProfileCoverage{ID: key, Equipable: item.Equipable, Removed: removed[key], ShopPools: []string{}, Failures: []ItemProfileFailure{}}
		for pool, values := range catalog.ItemPools {
			for _, value := range values {
				if value == key {
					entry.ShopPools = append(entry.ShopPools, pool)
					break
				}
			}
		}
		sort.Strings(entry.ShopPools)
		if item.Equipable {
			for _, recipient := range recipients {
				pokemon := recipient
				state.Team, state.Towers = []Pokemon{pokemon}, []Tower{}
				state.Box = []Pokemon{}
				state.Progression.Forms = map[int]string{}
				state.Inventory = []OwnedItem{{ID: key}}
				if !progressionCanEquip(state, &pokemon, item) {
					continue
				}
				entry.EligibleRecipients++
				if equipErr := progressionEquip(state, pokemon.ID, key); equipErr != nil {
					entry.Failures = append(entry.Failures, ItemProfileFailure{Species: pokemon.Species, Level: pokemon.Level, Error: equipErr.Error()})
					continue
				}
				pokemon = state.Team[0]
				placement := Placement{ID: -1, Terrain: 1}
				terrains := catalog.Species[pokemon.Species].Terrains
				for _, candidate := range route.Placements {
					for _, terrain := range terrains {
						if candidate.Terrain == terrain {
							placement = candidate
							break
						}
					}
					if placement.ID >= 0 {
						break
					}
				}
				tower := Tower{ID: state.NextID, PokemonID: pokemon.ID, TileID: placement.ID, X: placement.X, Y: placement.Y}
				state.Towers = []Tower{tower}
				context := contextFor(state, &tower, &state.Team[0], nil, .8)
				profile, buildErr := BuildAttackProfile(context)
				if buildErr == nil && (!finiteNumber(profile.Power) || !finiteNumber(profile.Range) || !finiteNumber(profile.AttackMS)) {
					buildErr = fmt.Errorf("non-finite attack profile")
				}
				if buildErr != nil {
					entry.Failures = append(entry.Failures, ItemProfileFailure{Species: pokemon.Species, Level: pokemon.Level, Error: buildErr.Error()})
				} else {
					entry.ConstructedProfiles++
				}
			}
		}
		output = append(output, entry)
	}
	return output, nil
}

var unresolvedAbilityFacts = map[string]string{
	"placeholder": "source placeholder has no playable species or behavior fact",
}

const implementedAbilityIDs = "ambusher swimmer vigilant vigilantFrisk frisk scrappy burn burnDoubleShot burnNerf poison toxic poisonDoubleShot direClaw slow stunMono stunMonoNerf stunArea moldBreaker armorBreak armorBreakDoubleShot rageFist piercingDrill ninja tripleAxel static slowRicochet burnRicochet magicBounce armaldo heal takeHeart bitterBlade seedFlare soulHeartReactor greed focus curse curseDoubleShot curseSplash willOWisp doubleShot tripleShot quadraShot octaShot doubleShotSand quadraShotSand splash slowSplash armorBreakSplash synchronySplash burnSplash fireBlast auraSphere pollenPuff autotomize naturesJudgement magmaStorm superCritical powerAura criticalDamageAura criticalAura triage illuminate illuminateBuff helpingHand swordsDance heartOfSteel sacredSword victoryStar genesisAura hugePower spinda castform star nightmare shadowBall shadowTag bastiodon rampardos cradily fieryDance contrary defiant firstImpression moxie sniper toughClaws toughClawsDay toughClawsNight chatter speedBoost simple drizzle drought flamethrower defeatist dreamEater torrent overgrow magician makeItRain invalidState solarBeam hydroCannon moongeistBeam flameWheel waterBubble shiftGear triAttack stoneEdge stoneEdgeMega hiddenPower spiralDrain sandStream sandStreamTyranitar sandStreamMegaTyranitar snowWarning heatCrash flashFire solarPower venomDrench spikes spikyShield precipiceBlades technician chlorophyll hyperCutter destructiveMaw refrigerate corrosion corrosionMega bubbleBeam hyperDrill icicleSpear twineedle twineedleMega zapCannon waterCompaction armorCannon noGuard sandRush featherDance tailGlow klutz hyperBeam iceBeam machBolt drySkin marvelScale diurnal nocturnal meteorMash ancientPower ancientPowerPlus bulkUp strongJaw meteorBeam rivalryPower rivalrySpeed boomburst liquidVoice originPulse charmedCharm drumBeating laundryGoRound emptyLight secretSword voltSurge dragonDarts lavaPlume volcanicKaboom sludgeBomb diamondStorm rockThrow supremeOverlord bonemerang signalBeam cryogenicCannon fireLash icyWind psychoCut psychoCutMega ceaselessEdge waterSpout focusEnergy sandForce sandVeil timeWarp slowStart steelBeam steamEruption grimNeigh psybeam dazzlingGleam auroraVeil spicySpray knockOff berserk berserkMega supremeCommanders supersonic shadowClaw flowerTrick teleport transform scheme grassPlatform icePlatform rest mount commander plus minus shellSmash cloudNine pixilate heavyMetal squishyToy honeyGather intrepidSword dauntlessShield aggressiveNature anchorShot stickyWeb electroBall unnerve harvest ripen gluttony uTurn bugBite discharge flameBody blizzard sturdy iceBody bulletPunch bulletPunchMega sacredFire lightningRod wildboltStorm sandsearStorm bleakwindStorm fairySingularity whiteRay explosion damp aerialAce stalwart amnesia spiritShackle unburden trickOrTreat"

const progressionItemIDs = "ringTarget airBalloon dampMulch heavyDutyBoots starPiece shinyCharm bicycle subwoofer assaultVest gimmighoul alakazite absolite sceptilite charizarditeX tinyMushroom rareBone slowpokeTail pearl shoalShell diamond nugget crown smallBouquet ruby flygonite barbaraclite ampharosite pidgeotite sablenite blimpKeys lucarionite charizarditeY sokudosPortfolio mitsuesCocktail revysBook altarianite sharpedonite aerodactylite thundersDrive noxDex rosesItem scovillainite cameruptite dragoninite metagrossite glalitite froslassite dragalgite starminite garchompite houndoominite gyaradosite lopunnite magearnite heracronite gardevoirite beedrillite scizorite galladite aggronite tyranitarite excadrite chimechite mawilite skarmorite scraftinite steelixite tatsugirinite drampanite slowbronite gengarite"
