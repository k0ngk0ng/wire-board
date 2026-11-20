package engine

import "encoding/json"

const SchemaVersion = 1

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type SpriteSpec struct {
	Base             string  `json:"base"`
	Image            string  `json:"image"`
	Frames           int     `json:"frames"`
	Rows             int     `json:"rows"`
	Hold             int     `json:"hold"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	FrameWidth       float64 `json:"frameWidth"`
	FractionalFrames bool    `json:"fractionalFrames"`
	FrameHeight      float64 `json:"frameHeight"`
}

type TextSpec struct {
	Key           string                     `json:"key"`
	ID            json.RawMessage            `json:"id"`
	Name          string                     `json:"name"`
	NameEn        string                     `json:"nameEn"`
	Description   string                     `json:"description"`
	DescriptionEn string                     `json:"descriptionEn"`
	Names         []string                   `json:"names"`
	Descriptions  []string                   `json:"descriptions"`
	Sprite        string                     `json:"sprite,omitempty"`
	Facts         map[string]json.RawMessage `json:"facts"`
}

type Species struct {
	Key              string                     `json:"key"`
	ID               int                        `json:"id"`
	Name             string                     `json:"name"`
	Names            []string                   `json:"names"`
	Color            string                     `json:"color"`
	PowerBase        float64                    `json:"powerBase"`
	PowerPerLevel    float64                    `json:"powerPerLevel"`
	AttackMSBase     float64                    `json:"attackMsBase"`
	AttackMSPerLevel float64                    `json:"attackMsPerLevel"`
	RangeBase        float64                    `json:"rangeBase"`
	RangePerLevel    float64                    `json:"rangePerLevel"`
	CriticalBase     float64                    `json:"criticalBase"`
	CriticalPerLevel float64                    `json:"criticalPerLevel"`
	AbilityID        string                     `json:"abilityId"`
	Terrains         []int                      `json:"terrains"`
	EvolutionKey     string                     `json:"evolutionKey"`
	EvolutionLevel   int                        `json:"evolutionLevel"`
	ProjectileID     string                     `json:"projectileId"`
	ProjectileSpeed  float64                    `json:"projectileSpeed"`
	AttackType       string                     `json:"attackType"`
	RangeType        string                     `json:"rangeType"`
	InnerRange       float64                    `json:"innerRange"`
	CostScale        string                     `json:"costScale"`
	Sprite           SpriteSpec                 `json:"sprite"`
	Facts            map[string]json.RawMessage `json:"facts"`
}

type EnemySpec struct {
	Key          string                     `json:"key"`
	ID           json.RawMessage            `json:"id"`
	Name         string                     `json:"name"`
	Health       float64                    `json:"health"`
	Armor        float64                    `json:"armor"`
	Speed        float64                    `json:"speed"`
	Power        int                        `json:"power"`
	Regeneration float64                    `json:"regeneration"`
	CanStun      bool                       `json:"canStun"`
	CanSlow      bool                       `json:"canSlow"`
	CanBurn      bool                       `json:"canBurn"`
	CanPoison    bool                       `json:"canPoison"`
	Invisible    bool                       `json:"invisible"`
	PassiveID    string                     `json:"passiveId"`
	IsBoss       bool                       `json:"isBoss"`
	IsElite      bool                       `json:"isElite"`
	Gold         int64                      `json:"gold"`
	Sprite       SpriteSpec                 `json:"sprite"`
	Facts        map[string]json.RawMessage `json:"facts"`
}

type Placement struct {
	ID      int     `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Terrain int     `json:"terrain"`
}

type WaveSpec struct {
	Preview  []string  `json:"preview"`
	Sequence []*string `json:"sequence"`
}

type ChessPlacement struct {
	Key     string  `json:"key"`
	Species string  `json:"species"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

type RouteSpec struct {
	ID               int                        `json:"id"`
	Name             string                     `json:"name"`
	Background       string                     `json:"background"`
	Effect           string                     `json:"effect"`
	Width            int                        `json:"width"`
	Height           int                        `json:"height"`
	UnlockStars      int                        `json:"unlockStars"`
	Paths            [][]Point                  `json:"paths"`
	MirrorPaths      [][]Point                  `json:"mirrorPaths"`
	Offset           Point                      `json:"offset"`
	MirrorOffset     Point                      `json:"mirrorOffset"`
	PlacementGrid    []int                      `json:"placementGrid"`
	GridColumns      int                        `json:"gridColumns"`
	GridRows         int                        `json:"gridRows"`
	TileSize         int                        `json:"tileSize"`
	Placements       []Placement                `json:"placements"`
	ChessPlacements  []ChessPlacement           `json:"chessPlacements,omitempty"`
	Waves            map[string]WaveSpec        `json:"waves"`
	WeatherID        string                     `json:"weatherId"`
	ChallengeRewards []string                   `json:"challengeRewards"`
	Facts            map[string]json.RawMessage `json:"facts"`
	SecretKey        string                     `json:"secretKey"`
	MusicID          int                        `json:"musicId"`
	MusicKey         string                     `json:"musicKey"`
	IsSecret         bool                       `json:"isSecret"`
}

type SongSpec struct {
	ID     int      `json:"id"`
	Key    string   `json:"key"`
	Name   string   `json:"name"`
	NameEn string   `json:"nameEn"`
	Names  []string `json:"names"`
	Audio  string   `json:"audio"`
}

type ItemSpec struct {
	TextSpec
	Sprite      string                     `json:"sprite"`
	Tags        []string                   `json:"tags"`
	MegaStone   bool                       `json:"megaStone"`
	MegaPos     int                        `json:"megaPos"`
	Price       int64                      `json:"price"`
	Tier        string                     `json:"tier"`
	Equipable   bool                       `json:"equipable"`
	Retirable   bool                       `json:"retirable"`
	Condition   map[string]json.RawMessage `json:"condition"`
	Restriction map[string]json.RawMessage `json:"restriction"`
}

type ProjectileSpec struct {
	Key    string                     `json:"key"`
	ID     string                     `json:"id"`
	Sprite SpriteSpec                 `json:"sprite"`
	Facts  map[string]json.RawMessage `json:"facts"`
}

type CatalogSource struct {
	URL          string `json:"url"`
	Version      string `json:"version"`
	SourceSHA256 string `json:"sourceSha256"`
}

type Catalog struct {
	Version             string                    `json:"version"`
	Source              CatalogSource             `json:"source"`
	Species             map[string]Species        `json:"species"`
	Enemies             map[string]EnemySpec      `json:"enemies"`
	Routes              map[string]RouteSpec      `json:"routes"`
	Songs               map[string]SongSpec       `json:"songs"`
	Playlist            []string                  `json:"playlist"`
	Items               map[string]ItemSpec       `json:"items"`
	Abilities           map[string]TextSpec       `json:"abilities"`
	Passives            map[string]TextSpec       `json:"passives"`
	Weather             map[string]TextSpec       `json:"weather"`
	Projectiles         map[string]ProjectileSpec `json:"projectiles"`
	Achievements        []TextSpec                `json:"achievements"`
	EggPool             []string                  `json:"eggPool"`
	ForcedInitialPool   []string                  `json:"forcedInitialPool"`
	ForcedSecondaryPool []string                  `json:"forcedSecondaryPool"`
	ItemPools           map[string][]string       `json:"itemPools"`
	RemovedItemIDs      []string                  `json:"removedItemIds"`
	UpgradeCosts        map[string][]int64        `json:"upgradeCosts"`
	Collection          []string                  `json:"collection"`
}

type Options struct {
	Starter string  `json:"starter"`
	Route   int     `json:"route"`
	Speed   float64 `json:"speed,omitempty"`
}

type Stats struct {
	Power       float64 `json:"power"`
	AttackMS    float64 `json:"attackMS"`
	Range       float64 `json:"range"`
	CriticalPct float64 `json:"criticalPct"`
}

type Pokemon struct {
	ID                   int              `json:"id"`
	Owner                int              `json:"owner"`
	Dex                  DexTowerStats    `json:"dex,omitzero"`
	Species              string           `json:"species"`
	UpgradeCost          int64            `json:"upgradeCost"`
	UpgradeCosts         map[string]int64 `json:"upgradeCosts"`
	Level                int              `json:"level"`
	TargetModes          []string         `json:"targetModes"`
	Target               string           `json:"target"`
	Item                 string           `json:"item,omitempty"`
	TileID               int              `json:"tileID"`
	Favorite             bool             `json:"favorite"`
	Alias                string           `json:"alias,omitempty"`
	Shiny                bool             `json:"shiny"`
	Overdamage           float64          `json:"overdamage"`
	Damage               float64          `json:"damage"`
	Kills                int              `json:"kills"`
	Stats                Stats            `json:"stats"`
	CombatStats          *Stats           `json:"combatStats,omitempty"`
	CombatRangeHalfWidth float64          `json:"combatRangeHalfWidth,omitempty"`
	CombatInnerRange     float64          `json:"combatInnerRange,omitempty"`
	CombatAttackMode     string           `json:"combatAttackMode,omitempty"`
	CombatMoveMode       string           `json:"combatMoveMode,omitempty"`
	CombatRangeType      string           `json:"combatRangeType,omitempty"`
	EquipableItems       []string         `json:"equipableItems,omitempty"`
	Sprite               SpriteSpec       `json:"sprite"`
}

type OwnedItem struct {
	ID         string `json:"id"`
	Owner      int    `json:"owner"`
	EquippedBy int    `json:"equippedBy"`
	Favorite   bool   `json:"favorite"`
}

type Tower struct {
	ID                int                `json:"id"`
	DexCountdownMS    float64            `json:"dexCountdownMS,omitempty"`
	PokemonID         int                `json:"pokemonID"`
	TileID            int                `json:"tileID"`
	X                 float64            `json:"x"`
	Y                 float64            `json:"y"`
	TargetID          int                `json:"targetID"`
	CooldownMS        float64            `json:"cooldownMS"`
	AbilityCooldownMS int64              `json:"abilityCooldownMS"`
	WaveActiveMS      int64              `json:"waveActiveMS"`
	StatsRefreshMS    float64            `json:"statsRefreshMS"`
	IncenseTimerMS    float64            `json:"incenseTimerMS"`
	IncenseSeconds    int64              `json:"incenseSeconds"`
	PublishedIncense  int64              `json:"publishedIncense"`
	CriticalStreak    int                `json:"criticalStreak"`
	SinceCriticalMS   int64              `json:"sinceCriticalMS"`
	DeployedMS        int64              `json:"deployedMS"`
	IdleMS            int64              `json:"idleMS"`
	AttackCount       int                `json:"attackCount"`
	SameTargetHits    int                `json:"sameTargetHits"`
	Charges           int                `json:"charges"`
	StoredShots       int                `json:"storedShots"`
	Stacks            map[string]float64 `json:"stacks"`
}

type Enemy struct {
	EnemyEffectState
	ID                 int        `json:"id"`
	Key                string     `json:"key"`
	Species            string     `json:"species"`
	X                  float64    `json:"x"`
	Y                  float64    `json:"y"`
	PathVariant        string     `json:"pathVariant,omitempty"`
	PathOriginWaypoint int        `json:"pathOriginWaypoint"`
	PathMirrored       bool       `json:"pathMirrored"`
	PathIndex          int        `json:"pathIndex"`
	Waypoint           int        `json:"waypoint"`
	Distance           float64    `json:"distance"`
	Power              int        `json:"power"`
	Gold               int64      `json:"gold"`
	Entered            bool       `json:"entered"`
	Dead               bool       `json:"dead"`
	DeathRemainingMS   int64      `json:"deathRemainingMS"`
	Escaped            bool       `json:"escaped"`
	ReviveRemainingMS  int64      `json:"reviveRemainingMS"`
	ReviveHP           float64    `json:"reviveHP"`
	Revived            bool       `json:"revived"`
	SpawnAt            int64      `json:"spawnAt"`
	AgeMS              int64      `json:"ageMS"`
	LastHitBy          int        `json:"lastHitBy"`
	Sprite             SpriteSpec `json:"sprite"`
}

type Projectile struct {
	ID                   int            `json:"id"`
	TowerID              int            `json:"towerID"`
	PokemonID            int            `json:"pokemonID"`
	TargetID             int            `json:"targetID"`
	X                    float64        `json:"x"`
	Y                    float64        `json:"y"`
	TargetX              float64        `json:"targetX"`
	TargetY              float64        `json:"targetY"`
	Speed                float64        `json:"speed"`
	Kind                 string         `json:"kind"`
	Power                float64        `json:"power"`
	Critical             bool           `json:"critical"`
	RolledCritical       bool           `json:"rolledCritical"`
	MotionPhase          string         `json:"motionPhase,omitempty"`
	OrbitRadius          float64        `json:"orbitRadius"`
	RemainingDistance    float64        `json:"remainingDistance"`
	CriticalMultiplier   float64        `json:"criticalMultiplier"`
	Context              AbilityContext `json:"context"`
	DamageScale          float64        `json:"damageScale"`
	ExtraPowerMultiplier float64        `json:"extraPowerMultiplier,omitempty"`
	ModeBonusPct         float64        `json:"modeBonusPct,omitempty"`
	ModeCritical         *bool          `json:"modeCritical,omitempty"`
	ModeFinalMultiplier  float64        `json:"modeFinalMultiplier,omitempty"`
	PulseMS              int64          `json:"pulseMS"`
	RayDX                float64        `json:"rayDX"`
	RayDY                float64        `json:"rayDY"`
	RayLength            float64        `json:"rayLength"`
	BeamWidth            float64        `json:"beamWidth,omitempty"`
	Travelled            float64        `json:"travelled"`
	HitAt                map[int]int64  `json:"hitAt"`
	Angle                float64        `json:"angle"`
	SourceProfile        AttackProfile  `json:"sourceProfile"`
	Profile              AttackProfile  `json:"profile"`
	DelayMS              int64          `json:"delayMS"`
	LifeMS               int64          `json:"lifeMS"`
	Bounces              int            `json:"bounces"`
	RicochetHits         int            `json:"ricochetHits,omitempty"`
	Pierces              int            `json:"pierces"`
	HitIDs               []int          `json:"hitIDs"`
	Sprite               SpriteSpec     `json:"sprite"`
}

type Event struct {
	Critical           bool    `json:"critical,omitempty"`
	HPDamage           float64 `json:"hpDamage,omitempty"`
	ArmorDamage        float64 `json:"armorDamage,omitempty"`
	ShieldDamage       float64 `json:"shieldDamage,omitempty"`
	AttemptedDamage    float64 `json:"attemptedDamage,omitempty"`
	ResolvedDamage     float64 `json:"resolvedDamage,omitempty"`
	DisplayHPDamage    float64 `json:"displayHPDamage,omitempty"`
	DisplayArmorDamage float64 `json:"displayArmorDamage,omitempty"`
	SourceKind         string  `json:"sourceKind,omitempty"`
	Evaded             bool    `json:"evaded,omitempty"`
	ID                 int     `json:"id"`
	At                 int64   `json:"at"`
	Type               string  `json:"type"`
	Source             int     `json:"source,omitempty"`
	Target             int     `json:"target,omitempty"`
	X                  float64 `json:"x,omitempty"`
	Y                  float64 `json:"y,omitempty"`
	Amount             float64 `json:"amount,omitempty"`
	Key                string  `json:"key,omitempty"`
}

type Action struct {
	Type      string          `json:"type"`
	Epoch     int             `json:"epoch"`
	PokemonID int             `json:"pokemonID,omitempty"`
	TileID    int             `json:"tileID,omitempty"`
	TowerID   int             `json:"towerID,omitempty"`
	Count     int             `json:"count,omitempty"`
	Mode      string          `json:"mode,omitempty"`
	ItemID    string          `json:"itemID,omitempty"`
	Stock     int             `json:"stock,omitempty"`
	Quote     int             `json:"quote,omitempty"`
	Route     int             `json:"route,omitempty"`
	Enabled   bool            `json:"enabled,omitempty"`
	Value     float64         `json:"value,omitempty"`
	Slot      int             `json:"slot,omitempty"`
	Pick      int             `json:"pick,omitempty"`
	Name      string          `json:"name,omitempty"`
	TargetID  int             `json:"targetID,omitempty"`
	X         float64         `json:"x,omitempty"`
	Y         float64         `json:"y,omitempty"`
	Settings  json.RawMessage `json:"settings,omitempty"`
}

type State struct {
	Trainers         []TrainerState   `json:"trainers,omitempty"`
	Schema           int              `json:"schema"`
	RulesVersion     string           `json:"rulesVersion"`
	Options          Options          `json:"options"`
	Players          int              `json:"players"`
	Epoch            int              `json:"epoch"`
	Sequence         uint64           `json:"sequence"`
	Now              int64            `json:"now"`
	Clock            int64            `json:"clock"`
	Tick             int64            `json:"tick"`
	BattleFrame      uint64           `json:"battleFrame"`
	SimulationUnits  int64            `json:"simulationUnits"`
	StepUnits        int64            `json:"stepUnits"`
	RemainderMS      int64            `json:"remainderMS"`
	SpeedRemainder   float64          `json:"speedRemainder"`
	Random           uint32           `json:"random"`
	NextID           int              `json:"nextID"`
	NextEventID      int              `json:"nextEventID"`
	Phase            string           `json:"phase"`
	Gold             int64            `json:"gold"`
	Health           int              `json:"health"`
	MaxHealth        int              `json:"maxHealth"`
	Stars            int              `json:"stars"`
	Ribbons          int              `json:"ribbons"`
	TeamSlots        int              `json:"teamSlots"`
	Route            int              `json:"route"`
	Wave             int              `json:"wave"`
	WaveActive       bool             `json:"waveActive"`
	Paused           bool             `json:"paused"`
	AutoWave         bool             `json:"autoWave"`
	Speed            float64          `json:"speed"`
	Weather          string           `json:"weather"`
	WeatherUntil     int64            `json:"weatherUntil"`
	Records          map[int]int      `json:"records"`
	RouteWaves       map[int]int      `json:"routeWaves"`
	Team             []Pokemon        `json:"team"`
	Box              []Pokemon        `json:"box"`
	Inventory        []OwnedItem      `json:"inventory"`
	Towers           []Tower          `json:"towers"`
	Enemies          []Enemy          `json:"enemies"`
	Projectiles      []Projectile     `json:"projectiles"`
	Events           []Event          `json:"events"`
	WaveKills        int              `json:"waveKills"`
	WaveGold         int64            `json:"waveGold"`
	WaveHeartsLost   int              `json:"waveHeartsLost"`
	WaveHeartsHealed int              `json:"waveHeartsHealed"`
	TotalKills       int              `json:"totalKills"`
	TotalDamage      float64          `json:"totalDamage"`
	TotalOverdamage  float64          `json:"totalOverdamage"`
	Progression      ProgressionState `json:"progression"`
	World            WorldState       `json:"world"`
	AttackModes      AttackModeState  `json:"attackModes"`
}

type View struct {
	Balances        []int64         `json:"balances"`
	Trainers        []TrainerView   `json:"trainers,omitempty"`
	BattleTeam      []Pokemon       `json:"battleTeam,omitempty"`
	PhysicsClock    int64           `json:"physicsClock"`
	Tick            int64           `json:"tick"`
	RemainderMS     int64           `json:"remainderMS"`
	RulesVersion    string          `json:"rulesVersion"`
	Epoch           int             `json:"epoch"`
	Sequence        uint64          `json:"sequence"`
	Clock           int64           `json:"clock"`
	Random          uint32          `json:"random"`
	Phase           string          `json:"phase"`
	Gold            int64           `json:"gold"`
	Health          int             `json:"health"`
	MaxHealth       int             `json:"maxHealth"`
	Stars           int             `json:"stars"`
	Ribbons         int             `json:"ribbons"`
	TeamSlots       int             `json:"teamSlots"`
	Route           int             `json:"route"`
	Wave            int             `json:"wave"`
	WaveActive      bool            `json:"waveActive"`
	WaveGold        int64           `json:"waveGold"`
	TotalDamage     float64         `json:"totalDamage"`
	TotalOverdamage float64         `json:"totalOverdamage"`
	WaveElapsedMS   int64           `json:"waveElapsedMS"`
	Paused          bool            `json:"paused"`
	AutoWave        bool            `json:"autoWave"`
	Speed           float64         `json:"speed"`
	Weather         string          `json:"weather"`
	Width           int             `json:"width"`
	Height          int             `json:"height"`
	Background      string          `json:"background"`
	Tiles           []Placement     `json:"tiles"`
	Paths           [][]Point       `json:"paths"`
	Team            []Pokemon       `json:"team"`
	Box             []Pokemon       `json:"box"`
	Items           []OwnedItem     `json:"items"`
	Towers          []Tower         `json:"towers"`
	Enemies         []Enemy         `json:"enemies"`
	Projectiles     []Projectile    `json:"projectiles"`
	Events          []Event         `json:"events"`
	Records         map[int]int     `json:"records"`
	Progression     ProgressionView `json:"progression"`
	World           WorldState      `json:"world"`
	AttackModes     AttackModeState `json:"attackModes"`
}
