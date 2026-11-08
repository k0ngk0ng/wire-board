// Package stoneage implements the self-contained StoneAge arena. All rules,
// snapshots, bots and seat visibility belong to this module.
package stoneage

import "github.com/k0ngk0ng/wire-board/internal/gameapi"

const schema = 1
const rulesVersion = "stoneage-arena-v2"
const pointBudget = 480

type Options struct {
	TurnSeconds        int   `json:"turnSeconds"`
	PreparationSeconds int   `json:"preparationSeconds"`
	MaxRounds          int   `json:"maxRounds"`
	Teams              []int `json:"teams"`
}
type Rider struct {
	Avatar  int `json:"avatar"`
	Graphic int `json:"graphic"`
}
type PetDefinition struct {
	Loyalty  int     `json:"loyalty"`
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Graphic  int     `json:"graphic"`
	Weights  [4]int  `json:"weights"`
	Elements [4]int  `json:"elements"`
	Riders   []Rider `json:"riders,omitempty"`
}
type Skill struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Family      string `json:"family"`
	Description string `json:"description"`
	Power       int    `json:"power"`
	Hits        int    `json:"hits"`
	Status      string `json:"status,omitempty"`
	Chance      int    `json:"chance,omitempty"`
}
type Magic struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	MP     int    `json:"mp"`
	Power  int    `json:"power"`
	Group  bool   `json:"group,omitempty"`
	Status string `json:"status,omitempty"`
}
type Equipment struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Slot              string `json:"slot"`
	WeaponType        int    `json:"weaponType"`
	Icon              string `json:"icon"`
	Variant           int    `json:"variant"`
	SourceDescription string `json:"sourceDescription,omitempty"`
	Attack            int    `json:"attack"`
	Defense           int    `json:"defense"`
	Speed             int    `json:"speed"`
	HP                int    `json:"hp"`
	MP                int    `json:"mp"`
	Resist            string `json:"resist,omitempty"`
	Magic             *Magic `json:"magic,omitempty"`
}
type Item struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Power       int    `json:"power"`
	Group       bool   `json:"group,omitempty"`
}
type Pool struct {
	Pets        []PetDefinition `json:"pets"`
	Rides       []PetDefinition `json:"rides"`
	Skills      []Skill         `json:"skills"`
	Hats        []Equipment     `json:"hats"`
	Weapons     []Equipment     `json:"weapons"`
	Clothes     []Equipment     `json:"clothes"`
	Accessories []Equipment     `json:"accessories"`
	HPItems     []Item          `json:"hpItems"`
	MPItems     []Item          `json:"mpItems"`
	CureItems   []Item          `json:"cureItems"`
	Tactics     []Item          `json:"tactics"`
}
type Loadout struct {
	Pet         int   `json:"pet"`
	Ride        int   `json:"ride"`
	Avatar      int   `json:"avatar"`
	Points      []int `json:"points"`
	Skills      []int `json:"skills"`
	Hat         int   `json:"hat"`
	Weapon      int   `json:"weapon"`
	Clothes     int   `json:"clothes"`
	Accessories []int `json:"accessories"`
	HPItem      int   `json:"hpItem"`
	MPItem      int   `json:"mpItem"`
	CureItem    int   `json:"cureItem"`
	Tactics     []int `json:"tactics"`
}
type Status struct {
	Poison       int `json:"poison"`
	Stone        int `json:"stone"`
	Sleep        int `json:"sleep"`
	Confusion    int `json:"confusion"`
	Drunk        int `json:"drunk"`
	Shield       int `json:"shield"`
	Haste        int `json:"haste"`
	Rage         int `json:"rage"`
	Immune       int `json:"immune"`
	PoisonSource int `json:"poisonSource"`
}
type Unit struct {
	Loyalty     int    `json:"loyalty"`
	Fled        bool   `json:"fled,omitempty"`
	TurnAttack  int    `json:"turnAttack,omitempty"`
	TurnDefense int    `json:"turnDefense,omitempty"`
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Graphic     int    `json:"graphic"`
	HP          int    `json:"hp"`
	MaxHP       int    `json:"maxHp"`
	MP          int    `json:"mp"`
	MaxMP       int    `json:"maxMp"`
	Attack      int    `json:"attack"`
	Defense     int    `json:"defense"`
	Speed       int    `json:"speed"`
	Elements    [4]int `json:"elements"`
	Status      Status `json:"status"`
	Guard       bool   `json:"guard"`
}
type InventoryItem struct {
	Item  Item `json:"item"`
	Count int  `json:"count"`
}
type Player struct {
	BattleReady bool            `json:"battleReady"`
	Seat        int             `json:"seat"`
	Team        int             `json:"team"`
	Deadline    int64           `json:"deadline"`
	Locked      bool            `json:"locked"`
	Submitted   bool            `json:"submitted"`
	Surrendered bool            `json:"surrendered"`
	Loadout     *Loadout        `json:"loadout,omitempty"`
	Body        Unit            `json:"body"`
	Pet         Unit            `json:"pet"`
	Ride        Unit            `json:"ride"`
	Riding      bool            `json:"riding"`
	Damage      int             `json:"damage"`
	Healing     int             `json:"healing"`
	Kills       int             `json:"kills"`
	Inventory   []InventoryItem `json:"inventory,omitempty"`
	Magic       []Magic         `json:"magic,omitempty"`
}
type Command struct {
	Kind   string `json:"kind"`
	ID     int    `json:"id"`
	Target int    `json:"target"`
}
type Plan struct {
	Player Command `json:"player"`
	Pet    Command `json:"pet"`
}
type Action struct {
	Type    string   `json:"type"`
	Session string   `json:"session"`
	Window  int      `json:"window"`
	Loadout *Loadout `json:"loadout,omitempty"`
	Plan    *Plan    `json:"plan,omitempty"`
}
type Event struct {
	Sequence         int            `json:"sequence,omitempty"`
	Projectiles      []Projectile   `json:"projectiles,omitempty"`
	Motions          []AttackMotion `json:"motions,omitempty"`
	ImpactAt         *int           `json:"impactAt,omitempty"`
	ReactionDuration int            `json:"reactionDuration,omitempty"`
	OriginalTarget   *int           `json:"originalTarget,omitempty"`
	RedirectReason   string         `json:"redirectReason,omitempty"`
	Actors           []int          `json:"actors,omitempty"`
	Effect           string         `json:"effect,omitempty"`
	ID               int            `json:"id"`
	Kind             string         `json:"kind"`
	Actor            int            `json:"actor"`
	Target           int            `json:"target"`
	Name             string         `json:"name"`
	Amount           int            `json:"amount"`
	RideAmount       int            `json:"rideAmount"`
	Critical         bool           `json:"critical,omitempty"`
	At               int            `json:"at"`
	Duration         int            `json:"duration"`
	Players          []Player       `json:"players"`
}
type Playback struct {
	StartedAt int64    `json:"startedAt"`
	Duration  int      `json:"duration"`
	Before    []Player `json:"before"`
}
type State struct {
	Rules         string          `json:"rules"`
	Session       string          `json:"session"`
	Window        int             `json:"window"`
	Phase         string          `json:"phase"`
	Round         int             `json:"round"`
	Options       Options         `json:"options"`
	Pool          Pool            `json:"pool"`
	Players       []Player        `json:"players"`
	Plans         []*Plan         `json:"plans"`
	Random        uint64          `json:"random"`
	Events        []Event         `json:"events"`
	EventID       int             `json:"eventId"`
	Playback      *Playback       `json:"playback,omitempty"`
	ResolveAt     int64           `json:"resolveAt"`
	Result        *gameapi.Result `json:"result,omitempty"`
	PendingResult *gameapi.Result `json:"pendingResult,omitempty"`
	Reason        string          `json:"reason"`
	Log           []string        `json:"log"`
}
type Self struct {
	Loadout   *Loadout        `json:"loadout"`
	Inventory []InventoryItem `json:"inventory"`
	Magic     []Magic         `json:"magic"`
	Plan      *Plan           `json:"plan"`
}
type View struct {
	PointBudget int       `json:"pointBudget"`
	Rules       string    `json:"rules"`
	Session     string    `json:"session"`
	Window      int       `json:"window"`
	Phase       string    `json:"phase"`
	Round       int       `json:"round"`
	Options     Options   `json:"options"`
	Pool        Pool      `json:"pool"`
	Players     []Player  `json:"players"`
	Self        *Self     `json:"self"`
	Viewer      int       `json:"viewer"`
	Events      []Event   `json:"events"`
	Playback    *Playback `json:"playback,omitempty"`
	Reason      string    `json:"reason"`
}
