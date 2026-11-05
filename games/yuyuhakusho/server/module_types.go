package yuyuhakusho

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/syncwire"
)

const (
	frameMS          int64 = 50
	lostInputMS      int64 = 750
	maxAdvanceMS     int64 = 250
	maxSequence      int64 = 1<<53 - 1
	maxSequenceJump  int64 = 1024
	maxInputQueue          = 64
	maxActionBytes         = 1024
	clockNumerator   int64 = 715909
	clockDenominator int64 = 11947200
)

type InputChange struct {
	Frame    uint64         `json:"frame,string"`
	Sequence int64          `json:"sequence"`
	Buttons  engine.Buttons `json:"buttons"`
}
type InputState struct {
	Sequence        int64          `json:"sequence"`
	AppliedSequence int64          `json:"appliedSequence"`
	Buttons         engine.Buttons `json:"buttons"`
	LastPacketAt    int64          `json:"lastPacketAt"`
	Automated       bool           `json:"automated"`
	Changes         []InputChange  `json:"changes,omitempty"`
}
type LoadingState struct {
	Ready []bool `json:"ready"`
}
type State struct {
	ResourceDescriptorSHA256 string                `json:"resourceDescriptorSHA256"`
	Options                  Options               `json:"options"`
	Epoch                    string                `json:"epoch"`
	Frame                    uint64                `json:"frame,string"`
	ServerAt                 int64                 `json:"serverAt"`
	Remainder                int64                 `json:"remainder"`
	World                    engine.ChuWorldState  `json:"world"`
	Presentation             renderstate.State     `json:"presentation"`
	Inputs                   []InputState          `json:"inputs"`
	Loading                  *LoadingState         `json:"loading,omitempty"`
	History                  []syncwire.InputFrame `json:"history"`
	HadComputer              bool                  `json:"hadComputer"`
}
type PlayerView struct {
	Seat        int   `json:"seat"`
	Slot        uint8 `json:"slot"`
	CharacterID uint8 `json:"characterID"`
	Computer    bool  `json:"computer"`
	Active      bool  `json:"active"`
	Health      int16 `json:"health"`
	Energy      int16 `json:"energy"`
	X           int16 `json:"x"`
	Y           int16 `json:"y"`
}
type WireInput struct {
	Sequence        int64          `json:"sequence"`
	AppliedSequence int64          `json:"appliedSequence"`
	Buttons         engine.Buttons `json:"buttons"`
	AppliedButtons  engine.Buttons `json:"appliedButtons"`
	Automated       bool           `json:"automated"`
}
type View struct {
	Protocol                 uint8                `json:"protocol"`
	Epoch                    string               `json:"epoch"`
	Frame                    uint64               `json:"frame,string"`
	Phase                    string               `json:"phase"`
	Options                  Options              `json:"options"`
	SeatSlots                []uint8              `json:"seatSlots"`
	LocalSeat                int                  `json:"localSeat"`
	Players                  []PlayerView         `json:"players"`
	Inputs                   []WireInput          `json:"inputs"`
	Loading                  *LoadingState        `json:"loading,omitempty"`
	Digest                   string               `json:"digest"`
	PresentationDigest       string               `json:"presentationDigest"`
	Replay                   syncwire.InputWindow `json:"replay"`
	ServerAt                 int64                `json:"serverAt"`
	ResourceDescriptorSHA256 string               `json:"resourceDescriptorSHA256"`
}

// Checkpoint is typed only inside Go. Its network representation is an opaque
// exact JSON string so JavaScript cannot round the uint64 sprite bitmaps.
type Checkpoint struct {
	Version                  int                      `json:"version"`
	Epoch                    string                   `json:"epoch"`
	Frame                    uint64                   `json:"frame,string"`
	Options                  engine.ChuSessionOptions `json:"options"`
	World                    engine.ChuWorldState     `json:"world"`
	Presentation             renderstate.State        `json:"presentation"`
	Identity                 renderstate.Identity     `json:"identity"`
	ResourceDescriptorSHA256 string                   `json:"resourceDescriptorSHA256"`
}
type SyncSnapshot struct {
	View       View   `json:"view"`
	Checkpoint string `json:"checkpoint"`
}
type action struct {
	Type     string
	Epoch    string
	Sequence int64
	Buttons  engine.Buttons
}

func marshalOptions(o Options) json.RawMessage { raw, _ := json.Marshal(o); return raw }
