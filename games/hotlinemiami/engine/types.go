package engine

const (
	Unit                 = 256
	FPS                  = 60
	Radius               = 9 * Unit
	MoveSpeed            = 3 * Unit
	MaxEvents            = 128
	MaxDrops             = 48
	MaxBullets           = 128
	MaxCorpses           = 48
	MaxPending           = 64
	MaxInputSeq          = int64(1_000_000_000_000)
	ViewportWidthPixels  = 576
	ViewportHeightPixels = 360
)

type Options struct {
	Map          string   `json:"map"`
	RoundSeconds int      `json:"roundSeconds"`
	Characters   []string `json:"characters"`
	Weapons      []string `json:"weapons"`
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Rect struct {
	X    int    `json:"x"`
	Y    int    `json:"y"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	Kind string `json:"kind,omitempty"`
}

type DoorDef struct {
	ID int `json:"id"`
	X  int `json:"x"`
	Y  int `json:"y"`
	W  int `json:"w"`
	H  int `json:"h"`
}

type WeaponSpawn struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Weapon string `json:"weapon"`
}

type Map struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Width        int           `json:"width"`
	Height       int           `json:"height"`
	Walls        []Rect        `json:"walls"`
	Spawns       []Point       `json:"spawns"`
	WeaponSpawns []WeaponSpawn `json:"weaponSpawns"`
	Doors        []DoorDef     `json:"doors"`
}

type Input struct {
	Seq       int64 `json:"seq"`
	ViewFrame int64 `json:"viewFrame,omitempty"`
	Life      int   `json:"life"`
	MoveX     int   `json:"moveX"`
	MoveY     int   `json:"moveY"`
	Aim       int   `json:"aim"`
	Fire      bool  `json:"fire"`
	Throw     bool  `json:"throw"`
	Pickup    bool  `json:"pickup"`
	Finish    bool  `json:"finish"`
	Interact  bool  `json:"interact"`
}

type Player struct {
	Seat            int     `json:"seat"`
	Character       string  `json:"character"`
	X               int     `json:"x"`
	Y               int     `json:"y"`
	Aim             int     `json:"aim"`
	Alive           bool    `json:"alive"`
	Life            int     `json:"life"`
	Weapon          string  `json:"weapon"`
	Ammo            int     `json:"ammo"`
	Score           int     `json:"score"`
	Kills           int     `json:"kills"`
	Deaths          int     `json:"deaths"`
	RespawnFrame    int64   `json:"respawnFrame"`
	ProtectedUntil  int64   `json:"protectedUntil"`
	DownUntil       int64   `json:"downUntil"`
	NextAttackFrame int64   `json:"nextAttackFrame"`
	BusyUntil       int64   `json:"busyUntil"`
	ReceivedSeq     int64   `json:"receivedSeq"`
	AppliedSeq      int64   `json:"appliedSeq"`
	LastInputFrame  int64   `json:"lastInputFrame"`
	Input           Input   `json:"input"`
	Pending         []Input `json:"pending,omitempty"`
	SpawnWeapon     string  `json:"spawnWeapon"`
	BotGoalX        int     `json:"botGoalX,omitempty"`
	BotGoalY        int     `json:"botGoalY,omitempty"`
	BotDecision     int64   `json:"botDecision,omitempty"`
}

type Bullet struct {
	ID        int    `json:"id"`
	Owner     int    `json:"owner"`
	Life      int    `json:"life"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	PrevX     int    `json:"prevX"`
	PrevY     int    `json:"prevY"`
	DX        int    `json:"dx"`
	DY        int    `json:"dy"`
	Remaining int    `json:"remaining"`
	Weapon    string `json:"weapon"`
	Thrown    bool   `json:"thrown"`
	Ammo      int    `json:"ammo"`
}

type Drop struct {
	ID      int    `json:"id"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Weapon  string `json:"weapon"`
	Ammo    int    `json:"ammo"`
	SpawnID int    `json:"spawnId,omitempty"`
	Frame   int64  `json:"frame"`
}

type Door struct {
	ID   int  `json:"id"`
	Open bool `json:"open"`
}

type Corpse struct {
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Aim       int    `json:"aim"`
	Character string `json:"character"`
	Frame     int64  `json:"frame"`
}

type Event struct {
	ID     int64  `json:"id"`
	Frame  int64  `json:"frame"`
	Kind   string `json:"kind"`
	Seat   int    `json:"seat"`
	Target int    `json:"target"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Weapon string `json:"weapon,omitempty"`
	Life   int    `json:"life,omitempty"`
}

type State struct {
	Options      Options   `json:"options"`
	Frame        int64     `json:"frame"`
	EndFrame     int64     `json:"endFrame"`
	MapID        string    `json:"mapId"`
	Players      []Player  `json:"players"`
	Bullets      []Bullet  `json:"bullets"`
	Drops        []Drop    `json:"drops"`
	Doors        []Door    `json:"doors"`
	BrokenGlass  []int     `json:"brokenGlass"`
	Corpses      []Corpse  `json:"corpses"`
	Events       []Event   `json:"events"`
	EventID      int64     `json:"eventId"`
	NextID       int       `json:"nextId"`
	Finished     bool      `json:"finished"`
	Seed         uint64    `json:"seed"`
	WeaponTimers []int64   `json:"weaponTimers,omitempty"`
	World        Map       `json:"-"`
	History      []History `json:"history,omitempty"`
}

type HistoryPlayer struct {
	X              int   `json:"x"`
	Y              int   `json:"y"`
	Alive          bool  `json:"alive"`
	Life           int   `json:"life"`
	ProtectedUntil int64 `json:"protectedUntil"`
}

type History struct {
	Frame       int64           `json:"frame"`
	Players     []HistoryPlayer `json:"players"`
	Doors       []Door          `json:"doors"`
	BrokenGlass []int           `json:"brokenGlass"`
}

type Weapon struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Ammo     int    `json:"ammo"`
	Cooldown int64  `json:"cooldown"`
	Range    int    `json:"range"`
	Speed    int    `json:"speed"`
	Pellets  int    `json:"pellets"`
	Spread   int    `json:"spread"`
	Melee    bool   `json:"melee"`
}
