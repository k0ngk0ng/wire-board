package conquest

import "encoding/json"

const (
	schema         = 1
	maxTroops      = 999999
	maxCivil       = 100000
	maxOrders      = 128
	moveDuration   = 1000
	actionDuration = 900
	resultDuration = 500
)

var colors = []string{"#b15d54", "#46728d", "#b28a35", "#547b53", "#886793", "#a96286", "#487f7b", "#877144"}

type Options struct {
	Fog         bool   `json:"fog"`
	MapID       string `json:"mapId"`
	Difficulty  string `json:"difficulty"`
	TurnSeconds int    `json:"turnSeconds"`
}
type Province struct {
	ID         string `json:"id"`
	Owner      int    `json:"owner"`
	Troops     int    `json:"troops"`
	Mobile     int    `json:"mobile"`
	Town       bool   `json:"town"`
	Fort       bool   `json:"fort"`
	Tower      bool   `json:"tower"`
	Population int    `json:"population"`
	Economy    int    `json:"economy"`
	Morale     int    `json:"morale"`
}
type Player struct {
	Seat            int     `json:"seat"`
	Color           string  `json:"color"`
	Gold            int64   `json:"gold"`
	Income          int64   `json:"income"`
	Upkeep          int64   `json:"upkeep"`
	EconomySpending int64   `json:"economySpending"`
	Movement        float64 `json:"movement"`
	King            string  `json:"king"`
	Territories     int     `json:"territories"`
	Troops          int     `json:"troops"`
	Submitted       bool    `json:"submitted"`
	Eliminated      bool    `json:"eliminated"`
	Deadline        int64   `json:"deadline"`
	Tax             int     `json:"tax"`
	Morale          int     `json:"morale"`
	EconomyRate     int     `json:"economyRate"`
	MilitaryRate    int     `json:"militaryRate"`
	MP10            int     `json:"mp10"`
	LostRound       int     `json:"lostRound"`
}
type Order struct {
	ID       int    `json:"id"`
	Type     string `json:"type"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Province string `json:"province,omitempty"`
	Amount   int    `json:"amount,omitempty"`
	King     bool   `json:"king,omitempty"`
	Building string `json:"building,omitempty"`
	Value    int    `json:"value,omitempty"`
}
type Action struct {
	Type     string `json:"type"`
	Session  string `json:"session"`
	Epoch    int    `json:"epoch"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Province string `json:"province,omitempty"`
	Amount   int    `json:"amount,omitempty"`
	King     bool   `json:"king,omitempty"`
	Building string `json:"building,omitempty"`
	Value    *int   `json:"value,omitempty"`
	ID       int    `json:"id,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
}
type Event struct {
	ID           int       `json:"id"`
	Type         string    `json:"type"`
	From         string    `json:"from,omitempty"`
	To           string    `json:"to,omitempty"`
	Province     string    `json:"province,omitempty"`
	Seat         int       `json:"seat"`
	Amount       int       `json:"amount"`
	Captured     bool      `json:"captured"`
	At           int64     `json:"at"`
	Until        int64     `json:"until"`
	DisplayUntil int64     `json:"displayUntil,omitempty"`
	Applied      bool      `json:"applied,omitempty"`
	Skipped      bool      `json:"skipped,omitempty"`
	Before       *Province `json:"before,omitempty"`
	After        *Province `json:"after,omitempty"`
	Text         string    `json:"text"`
}
type queued struct {
	Seat  int   `json:"seat"`
	Order Order `json:"order"`
}
type State struct {
	ResolutionHold bool       `json:"resolutionHold,omitempty"`
	Options        Options    `json:"options"`
	Session        string     `json:"session"`
	Epoch          int        `json:"epoch"`
	Round          int        `json:"round"`
	Phase          string     `json:"phase"`
	MapID          string     `json:"mapId"`
	MapVersion     int        `json:"mapVersion"`
	MapSHA256      string     `json:"mapSHA256"`
	Seed           uint64     `json:"seed"`
	Provinces      []Province `json:"provinces"`
	Players        []Player   `json:"players"`
	Plans          [][]Order  `json:"plans"`
	NextOrder      []int      `json:"nextOrder"`
	PauseVotes     []bool     `json:"pauseVotes"`
	ResumeVotes    []bool     `json:"resumeVotes"`
	Remaining      []int64    `json:"remaining"`
	Events         []Event    `json:"events"`
	NextEvent      int        `json:"nextEvent"`
	Pending        []queued   `json:"pending"`
	Cursor         int        `json:"cursor"`
	ResolveAt      int64      `json:"resolveAt"`
	Winner         int        `json:"winner"`
	Log            []string   `json:"log"`
}
type Controls struct {
	CanPlan         bool    `json:"canPlan"`
	CanSubmit       bool    `json:"canSubmit"`
	CanUnsubmit     bool    `json:"canUnsubmit"`
	CanPause        bool    `json:"canPause"`
	CanResume       bool    `json:"canResume"`
	CanResign       bool    `json:"canResign"`
	CanBuildTower   bool    `json:"canBuildTower"`
	RecruitCost     int     `json:"recruitCost"`
	FortCost        int     `json:"fortCost"`
	TowerCost       int     `json:"towerCost"`
	TownCost        int     `json:"townCost"`
	SeaCost         int     `json:"seaCost"`
	MoveCost        float64 `json:"moveCost"`
	RecruitMovement float64 `json:"recruitMovement"`
	FortMovement    float64 `json:"fortMovement"`
	TowerMovement   float64 `json:"towerMovement"`
	TownMovement    float64 `json:"townMovement"`
	MaxOrders       int     `json:"maxOrders"`
}
type View struct {
	Options           Options    `json:"options"`
	Session           string     `json:"session"`
	Epoch             int        `json:"epoch"`
	Round             int        `json:"round"`
	Phase             string     `json:"phase"`
	MapID             string     `json:"mapId"`
	MapVersion        int        `json:"mapVersion"`
	MapSHA256         string     `json:"mapSHA256"`
	Provinces         []Province `json:"provinces"`
	Players           []Player   `json:"players"`
	Orders            []Order    `json:"orders"`
	Projected         []Province `json:"projected"`
	AvailableGold     int64      `json:"availableGold"`
	AvailableMovement float64    `json:"availableMovement"`
	PauseVotes        []bool     `json:"pauseVotes"`
	ResumeVotes       []bool     `json:"resumeVotes"`
	Events            []Event    `json:"events"`
	Controls          Controls   `json:"controls"`
	Winner            int        `json:"winner"`
	Log               []string   `json:"log"`
}

func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
