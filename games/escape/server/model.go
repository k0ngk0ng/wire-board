// Package escape implements the independent cooperative 密室逃脱 game.
package escape

import "encoding/json"

const tickMS int64 = 100
const speed = 155.0
const minX, maxX, minY, maxY = 145.0, 855.0, 270.0, 565.0

type Options struct {
	Scenario   string `json:"scenario"`
	Difficulty string `json:"difficulty"`
	Seed       string `json:"seed,omitempty"`
}
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Door struct {
	Room      string  `json:"room"`
	Direction string  `json:"direction"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
}
type Device struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"`
	Label       string  `json:"label"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Done        bool    `json:"done"`
	Dials       []int   `json:"dials,omitempty"`
	Description string  `json:"description"`
	HeldUntil   int64   `json:"heldUntil,omitempty"`
}
type Room struct {
	ID      string   `json:"id"`
	X       int      `json:"x"`
	Y       int      `json:"y"`
	Known   bool     `json:"known"`
	Kind    string   `json:"kind,omitempty"`
	Title   string   `json:"title,omitempty"`
	Doors   []Door   `json:"doors,omitempty"`
	Objects []Device `json:"objects,omitempty"`
	Flood   float64  `json:"flood"`
}
type Player struct {
	Seat       int     `json:"seat"`
	Room       string  `json:"room"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	DX         float64 `json:"dx"`
	DY         float64 `json:"dy"`
	Seq        int64   `json:"seq"`
	AppliedSeq int64   `json:"appliedSeq"`
	MotionAt   int64   `json:"motionAt,omitempty"`
	Facing     string  `json:"facing"`
	HP         float64 `json:"hp"`
	Carry      string  `json:"carry,omitempty"`
	Ready      bool    `json:"ready"`
	Focus      string  `json:"focus,omitempty"`
	Goal       *Point  `json:"goal,omitempty"`
	Task       string  `json:"task,omitempty"`
	Progress   float64 `json:"progress,omitempty"`
	InputUntil int64   `json:"inputUntil,omitempty"`
	Queue      string  `json:"queue,omitempty"`
	Mode       string  `json:"mode,omitempty"`
	Work       int64   `json:"work,omitempty"`
}
type Clue struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Text   string `json:"text"`
	Values []int  `json:"values,omitempty"`
	Room   string `json:"room"`
}
type Supplies struct {
	Fuses     int `json:"fuses"`
	Cells     int `json:"cells"`
	Meds      int `json:"meds"`
	Documents int `json:"documents"`
}
type Incident struct {
	Room   string `json:"room"`
	Kind   string `json:"kind"`
	Begins int64  `json:"begins"`
	Ends   int64  `json:"ends"`
}
type Rally struct {
	Room  string `json:"room"`
	Until int64  `json:"until"`
	Actor int    `json:"actor"`
}
type Event struct {
	ID    int    `json:"id"`
	Kind  string `json:"kind"`
	Text  string `json:"text"`
	Room  string `json:"room"`
	Actor int    `json:"actor"`
	At    int64  `json:"at"`
}
type Objective struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Done     bool   `json:"done"`
	Progress string `json:"progress,omitempty"`
}
type Route struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}
type Stats struct {
	Rescues  int    `json:"rescues"`
	Mistakes int    `json:"mistakes"`
	Explored int    `json:"explored"`
	Route    string `json:"route"`
}
type State struct {
	Options      Options          `json:"options"`
	Epoch        string           `json:"epoch"`
	Seed         string           `json:"seed"`
	Random       uint64           `json:"randomState"`
	Phase        string           `json:"phase"`
	Tick         int64            `json:"tick"`
	ServerAt     int64            `json:"serverAt"`
	Duration     int64            `json:"duration"`
	Power        float64          `json:"power"`
	Circuits     []string         `json:"circuits"`
	Rooms        []Room           `json:"rooms"`
	Players      []Player         `json:"players"`
	Solutions    map[string][]int `json:"solutions"`
	Journal      []Clue           `json:"journal"`
	Supplies     Supplies         `json:"supplies"`
	Solved       map[string]bool  `json:"solved"`
	Gate         bool             `json:"gate"`
	Incident     *Incident        `json:"incident,omitempty"`
	Rally        *Rally           `json:"rally,omitempty"`
	NextIncident int64            `json:"nextIncident"`
	Events       []Event          `json:"events"`
	EventID      int              `json:"eventID"`
	Revision     uint64           `json:"revision"`
	Votes        []bool           `json:"votes"`
	Paused       bool             `json:"paused"`
	Stats        Stats            `json:"stats"`
	Victory      bool             `json:"victory"`
	Reason       string           `json:"reason"`
}
type View struct {
	Protocol   int         `json:"protocol"`
	Partial    bool        `json:"partial,omitempty"`
	Options    Options     `json:"options"`
	Epoch      string      `json:"epoch"`
	Phase      string      `json:"phase"`
	Tick       int64       `json:"tick"`
	ServerAt   int64       `json:"serverAt"`
	Remaining  int64       `json:"remaining"`
	Power      float64     `json:"power"`
	Circuits   []string    `json:"circuits"`
	Players    []Player    `json:"players"`
	Rooms      []Room      `json:"rooms,omitempty"`
	Water      []float64   `json:"water"`
	Journal    []Clue      `json:"journal,omitempty"`
	Supplies   Supplies    `json:"supplies"`
	Objectives []Objective `json:"objectives,omitempty"`
	Routes     []Route     `json:"routes,omitempty"`
	Incident   *Incident   `json:"incident"`
	Rally      *Rally      `json:"rally"`
	Events     []Event     `json:"events,omitempty"`
	Revision   uint64      `json:"revision"`
	Paused     bool        `json:"paused"`
	Votes      []bool      `json:"votes"`
	CanAct     bool        `json:"canAct"`
	You        int         `json:"you"`
	Stats      Stats       `json:"stats"`
	Reason     string      `json:"reason,omitempty"`
	Seed       string      `json:"seed,omitempty"`
}
type Action struct {
	Type    string  `json:"type"`
	Epoch   string  `json:"epoch"`
	Seq     int64   `json:"seq"`
	Target  string  `json:"target,omitempty"`
	DX      float64 `json:"dx,omitempty"`
	DY      float64 `json:"dy,omitempty"`
	X       float64 `json:"x,omitempty"`
	Y       float64 `json:"y,omitempty"`
	Index   int     `json:"index,omitempty"`
	Circuit string  `json:"circuit,omitempty"`
	Route   string  `json:"route,omitempty"`
}

func rawAction(a Action) json.RawMessage { b, _ := json.Marshal(a); return b }
