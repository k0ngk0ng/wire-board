package engine

const (
	Unit          = 256
	FPS           = 100
	NormalUnit    = 1048576
	ContentsEmpty = -1
	ContentsSolid = -2
	ContentsWater = -3
	MaxEvents     = 128
	MaxPending    = 64
	MaxInputSeq   = int64(1000000000000)
	MaxHistory    = 32
)

type Options struct {
	Map          string   `json:"map"`
	RoundSeconds int      `json:"roundSeconds"`
	MaxRounds    int      `json:"maxRounds"`
	Teams        []string `json:"teams"`
	Characters   []string `json:"characters"`
	FriendlyFire bool     `json:"friendlyFire"`
}

type Vec3 struct {
	X int `json:"x"`
	Y int `json:"y"`
	Z int `json:"z"`
}
type Spawn struct {
	X   int `json:"x"`
	Y   int `json:"y"`
	Z   int `json:"z"`
	Yaw int `json:"yaw"`
}
type Plane struct {
	X    int `json:"x"`
	Y    int `json:"y"`
	Z    int `json:"z"`
	Dist int `json:"dist"`
}
type Node struct {
	Plane int `json:"plane"`
	Front int `json:"front"`
	Back  int `json:"back"`
}
type Hull struct {
	Root   int     `json:"root"`
	Nodes  []Node  `json:"nodes"`
	Planes []Plane `json:"-"`
}
type Bounds struct {
	Min  Vec3   `json:"min"`
	Max  Vec3   `json:"max"`
	Team string `json:"team,omitempty"`
}
type Brush struct {
	ID         int    `json:"id"`
	Kind       string `json:"kind"`
	Bounds     Bounds `json:"bounds"`
	Hulls      []Hull `json:"hulls"`
	Origin     Vec3   `json:"origin"`
	Open       bool   `json:"open"`
	Health     int    `json:"health"`
	Move       Vec3   `json:"move"`
	Speed      int    `json:"speed"`
	Wait       int64  `json:"wait"`
	Flags      int    `json:"flags"`
	Target     string `json:"target,omitempty"`
	TargetName string `json:"targetName,omitempty"`
	ModelIndex int    `json:"modelIndex"`
	Angle      int    `json:"angle"`
}
type Surface struct {
	Plane    int    `json:"plane"`
	Material string `json:"material"`
	Texture  string `json:"texture"`
	Vertices []Vec3 `json:"vertices"`
	Brush    int    `json:"brush"`
}
type Map struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Mode            string        `json:"mode"`
	Planes          []Plane       `json:"planes"`
	Nodes           []Node        `json:"nodes"`
	Root            int           `json:"root"`
	Hulls           []Hull        `json:"hulls"`
	SpawnsT         []Spawn       `json:"spawnsT"`
	SpawnsCT        []Spawn       `json:"spawnsCT"`
	Bombsites       []Bounds      `json:"bombsites"`
	RescueZones     []Bounds      `json:"rescueZones"`
	RescueLegacy    bool          `json:"rescueLegacy,omitempty"`
	BuyZones        []Bounds      `json:"buyZones"`
	Hostages        []Spawn       `json:"hostages"`
	Brushes         []Brush       `json:"brushes,omitempty"`
	Ladders         []Bounds      `json:"ladders,omitempty"`
	Nav             []Vec3        `json:"nav,omitempty"`
	NavEdges        [][]int       `json:"navEdges,omitempty"`
	NavFlags        []int         `json:"navFlags,omitempty"`
	NavConnections  [][]int       `json:"navConnections,omitempty"`
	Surfaces        []Surface     `json:"surfaces,omitempty"`
	NavCosts        [][]int       `json:"-"`
	NavHostageCosts [][]int       `json:"-"`
	NavGroundCosts  [][]int       `json:"-"`
	SurfaceIndex    map[int][]int `json:"-"`
}

type Input struct {
	Seq         int64  `json:"seq"`
	Life        int    `json:"life"`
	ViewFrame   int64  `json:"viewFrame"`
	MoveX       int    `json:"moveX"`
	MoveY       int    `json:"moveY"`
	Yaw         int    `json:"yaw"`
	Pitch       int    `json:"pitch"`
	Fire        bool   `json:"fire"`
	Secondary   bool   `json:"secondary"`
	Jump        bool   `json:"jump"`
	Crouch      bool   `json:"crouch"`
	Walk        bool   `json:"walk"`
	Reload      bool   `json:"reload"`
	Use         bool   `json:"use"`
	Drop        bool   `json:"drop"`
	Select      string `json:"select,omitempty"`
	QueuedFrame int64  `json:"queuedFrame,omitempty"`
}

type Weapon struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Slot             int    `json:"slot"`
	Team             string `json:"team,omitempty"`
	Price            int    `json:"price"`
	Ammo             int    `json:"ammo"`
	Reserve          int    `json:"reserve"`
	Damage           int    `json:"damage"`
	ArmorRatio       int    `json:"armorRatio"`
	Cycle            int64  `json:"cycle"`
	CycleUS          int64  `json:"cycleUs"`
	Reload           int64  `json:"reload"`
	Range            int    `json:"range"`
	RangeModifier    int    `json:"rangeModifier"`
	Penetration      int    `json:"penetration"`
	PenetrationPower int    `json:"penetrationPower"`
	Speed            int    `json:"speed"`
	Pellets          int    `json:"pellets"`
	Spread           int    `json:"spread"`
	Automatic        bool   `json:"automatic"`
	AmmoType         string `json:"ammoType"`
	AmmoPrice        int    `json:"ammoPrice"`
	AmmoPack         int    `json:"ammoPack"`
}

type WeaponState struct {
	ID                  string `json:"id"`
	Ammo                int    `json:"ammo"`
	Reserve             int    `json:"reserve"`
	Silenced            bool   `json:"silenced"`
	Burst               bool   `json:"burst"`
	Zoom                int    `json:"zoom"`
	ReloadUntil         int64  `json:"reloadUntil"`
	Shots               int    `json:"shots"`
	LastShot            int64  `json:"lastShot"`
	BurstLeft           int    `json:"burstLeft"`
	BurstSpread         int    `json:"burstSpread"`
	BurstAt             int64  `json:"burstAt"`
	Accuracy            int    `json:"accuracy"`
	AccuracyInitialized bool   `json:"accuracyInitialized"`
	NextShotUS          int64  `json:"nextShotUs"`
	KickDirection       int    `json:"kickDirection"`
}

type Player struct {
	Seat           int           `json:"seat"`
	Team           string        `json:"team"`
	Character      string        `json:"character"`
	X              int           `json:"x"`
	Y              int           `json:"y"`
	Z              int           `json:"z"`
	VX             int           `json:"vx"`
	VY             int           `json:"vy"`
	VZ             int           `json:"vz"`
	Yaw            int           `json:"yaw"`
	Pitch          int           `json:"pitch"`
	Alive          bool          `json:"alive"`
	Life           int           `json:"life"`
	Health         int           `json:"health"`
	Armor          int           `json:"armor"`
	Helmet         bool          `json:"helmet"`
	Defuser        bool          `json:"defuser"`
	Money          int           `json:"money"`
	Weapon         string        `json:"weapon"`
	Ammo           int           `json:"ammo"`
	Reserve        int           `json:"reserve"`
	Inventory      []WeaponState `json:"inventory"`
	Kills          int           `json:"kills"`
	Deaths         int           `json:"deaths"`
	Crouch         bool          `json:"crouch"`
	Grounded       bool          `json:"grounded"`
	WaterLevel     int           `json:"waterLevel"`
	LandingSpeed   int           `json:"landingSpeed"`
	ReceivedSeq    int64         `json:"receivedSeq"`
	AppliedSeq     int64         `json:"appliedSeq"`
	Input          Input         `json:"input"`
	Previous       Input         `json:"previous"`
	Pending        []Input       `json:"pending,omitempty"`
	LastInputFrame int64         `json:"lastInputFrame"`
	NextAttack     int64         `json:"nextAttack"`
	RecoilYaw      int           `json:"recoilYaw"`
	RecoilPitch    int           `json:"recoilPitch"`
	FlashUntil     int64         `json:"flashUntil"`
	FlashStrength  int           `json:"flashStrength"`
	ActionSince    int64         `json:"actionSince"`
	Action         string        `json:"action,omitempty"`
	BotGoal        Vec3          `json:"botGoal"`
	BotPath        []int         `json:"botPath,omitempty"`
	BotTarget      int           `json:"botTarget"`
	BotDecision    int64         `json:"botDecision"`
	BotRouteMode   int           `json:"botRouteMode,omitempty"`
	BotStuck       int           `json:"botStuck"`
	RemX           int           `json:"remX"`
	RemY           int           `json:"remY"`
	RemZ           int           `json:"remZ"`
}

type Event struct {
	ID     int64  `json:"id"`
	Frame  int64  `json:"frame"`
	Kind   string `json:"type"`
	Seat   int    `json:"seat"`
	Target int    `json:"target"`
	Weapon string `json:"weapon,omitempty"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Z      int    `json:"z"`
	Value  int    `json:"value,omitempty"`
	Life   int    `json:"life,omitempty"`
}
type Bomb struct {
	Carrier    int   `json:"carrier"`
	Planted    bool  `json:"planted"`
	Dropped    bool  `json:"dropped"`
	X          int   `json:"x"`
	Y          int   `json:"y"`
	Z          int   `json:"z"`
	ExplodesAt int64 `json:"explodesAt"`
	Defuser    int   `json:"defuser"`
	DefuseAt   int64 `json:"defuseAt"`
}
type Hostage struct {
	ID        int  `json:"id"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Z         int  `json:"z"`
	Health    int  `json:"health"`
	Following int  `json:"following"`
	Rescued   bool `json:"rescued"`
	Touched   bool `json:"touched,omitempty"`
}
type HostageRoute struct {
	ID        int   `json:"id"`
	Following int   `json:"following"`
	Goal      Vec3  `json:"goal"`
	Path      []int `json:"path"`
	From      int   `json:"from"`
	PlannedAt int64 `json:"plannedAt"`
	VZ        int   `json:"vz"`
}
type Grenade struct {
	ID         int    `json:"id"`
	Owner      int    `json:"owner"`
	Kind       string `json:"kind"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Z          int    `json:"z"`
	VX         int    `json:"vx"`
	VY         int    `json:"vy"`
	VZ         int    `json:"vz"`
	ExplodesAt int64  `json:"explodesAt"`
	ExpiresAt  int64  `json:"expiresAt"`
	Active     bool   `json:"active"`
}
type Drop struct {
	ID     int         `json:"id"`
	X      int         `json:"x"`
	Y      int         `json:"y"`
	Z      int         `json:"z"`
	Weapon WeaponState `json:"weapon"`
	Frame  int64       `json:"frame"`
}
type BrushState struct {
	ID        int   `json:"id"`
	Open      bool  `json:"open"`
	Health    int   `json:"health"`
	Progress  int   `json:"progress"`
	Opening   bool  `json:"opening"`
	OpenedAt  int64 `json:"openedAt"`
	Direction int   `json:"direction,omitempty"`
}
type HistoryPlayer struct {
	Seat   int    `json:"seat"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Z      int    `json:"z"`
	Alive  bool   `json:"alive"`
	Life   int    `json:"life"`
	Crouch bool   `json:"crouch"`
	Yaw    int    `json:"yaw"`
	Weapon string `json:"weapon"`
}
type History struct {
	Frame   int64           `json:"frame"`
	Players []HistoryPlayer `json:"players"`
	Brushes []BrushState    `json:"brushes"`
}
type State struct {
	Options        Options        `json:"options"`
	Frame          int64          `json:"frame"`
	MapID          string         `json:"mapId"`
	World          Map            `json:"-"`
	Players        []Player       `json:"players"`
	Events         []Event        `json:"events"`
	EventID        int64          `json:"eventId"`
	Round          int            `json:"round"`
	Phase          string         `json:"phase"`
	PhaseEndsAt    int64          `json:"phaseEndsAt"`
	BuyEndsAt      int64          `json:"buyEndsAt"`
	TeamScores     [2]int         `json:"teamScores"`
	Bomb           Bomb           `json:"bomb"`
	Hostages       []Hostage      `json:"hostages"`
	HostageRoutes  []HostageRoute `json:"hostageRoutes,omitempty"`
	Grenades       []Grenade      `json:"grenades"`
	Drops          []Drop         `json:"drops"`
	Brushes        []BrushState   `json:"brushes"`
	History        []History      `json:"history"`
	Finished       bool           `json:"finished"`
	Seed           uint64         `json:"seed"`
	NextID         int            `json:"nextId"`
	LastWinner     string         `json:"lastWinner"`
	LossStreak     [2]int         `json:"lossStreak"`
	LoserBonusBase int            `json:"loserBonusBase"`
}
