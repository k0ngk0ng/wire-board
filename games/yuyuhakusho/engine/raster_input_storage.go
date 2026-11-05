package engine

import (
	"encoding/binary"
	"encoding/json"
	"errors"
)

// PanRows is a copied raster interpretation of the recording/playback bytes.
// The two 224-word views begin 512 bytes apart; each leaves a 64-byte gap.
// Stream is their only owner. Incomplete storage has no complete raster view.
func (state ChuWorldState) PanRows(plane int) (rows [224]int16) {
	if plane < 0 || plane > 1 || len(state.Stream) != 1024 {
		return rows
	}
	for i := range rows {
		rows[i] = int16(binary.BigEndian.Uint16(state.Stream[plane*512+i*2:]))
	}
	return rows
}

func (state *ChuWorldState) storePanRows(a, b [224]int16) error {
	if len(state.Stream) != 1024 {
		return errors.New("arena feedback stream storage is unavailable")
	}
	for i := range a {
		binary.BigEndian.PutUint16(state.Stream[i*2:], uint16(a[i]))
		binary.BigEndian.PutUint16(state.Stream[512+i*2:], uint16(b[i]))
	}
	return nil
}

// chuWorldJSONProjection is constructed only for serialization. It preserves
// the historical JSON field order, including the derived PanA/PanB fields.
// It is never stored in a world, decoded, or passed to rule callbacks.
type chuWorldJSONProjection struct {
	Objects                                           WorldObjects
	Clock                                             WorldClockState
	Controllers                                       [8]ControllerFrame
	RandomSeed                                        uint32
	Palette                                           ArenaPaletteState
	Camera                                            DefaultArenaCamera
	Cells                                             [768]uint16
	Buffers                                           [4]AnimationShapeBuffer
	Throws                                            ThrowContactBuffer
	Registration                                      FighterRegistrationState
	PendingSound                                      uint8
	SoundQueue                                        [4]uint8
	SlowGate                                          uint8
	SlowTicks                                         uint16
	EffectCursor                                      uint16
	DragonBankCursor                                  uint16                `json:",omitempty"`
	DragonHistory                                     Fighter4DragonHistory `json:",omitzero"`
	EffectTileCursor                                  uint8
	OccupiedLanes                                     uint8
	ComboFlags                                        uint8
	Stream                                            []byte
	Stopped                                           uint8
	HeadToHead                                        [16]uint8
	WinsByCharacter, LossesByCharacter                [64]uint8
	Difficulty                                        uint16
	ResetEnabled                                      uint8
	VideoMode, SceneMode, SceneRequest, InterruptMode uint16
	ScrollMode                                        uint8
	Vertical                                          [2]int16
	PanA, PanB                                        [224]int16
	ScrollSources                                     [2]int32
	ResourceList                                      uint32
	ResourceBusy                                      uint8
	Match                                             ChuWorldMatch
	MatchComplete                                     bool
}

func (state ChuWorldState) MarshalJSON() ([]byte, error) {
	return json.Marshal(chuWorldJSONProjection{
		Objects: state.Objects, Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed,
		Palette: state.Palette, Camera: state.Camera, Cells: state.Cells, Buffers: state.Buffers,
		Throws: state.Throws, Registration: state.Registration, PendingSound: state.PendingSound, SoundQueue: state.SoundQueue,
		SlowGate: state.SlowGate, SlowTicks: state.SlowTicks, EffectCursor: state.EffectCursor,
		DragonBankCursor: state.DragonBankCursor, DragonHistory: state.DragonHistory,
		EffectTileCursor: state.EffectTileCursor, OccupiedLanes: state.OccupiedLanes, ComboFlags: state.ComboFlags,
		Stream: state.Stream, Stopped: state.Stopped, HeadToHead: state.HeadToHead,
		WinsByCharacter: state.WinsByCharacter, LossesByCharacter: state.LossesByCharacter,
		Difficulty: state.Difficulty, ResetEnabled: state.ResetEnabled, VideoMode: state.VideoMode,
		SceneMode: state.SceneMode, SceneRequest: state.SceneRequest, InterruptMode: state.InterruptMode,
		ScrollMode: state.ScrollMode, Vertical: state.Vertical, PanA: state.PanRows(0), PanB: state.PanRows(1),
		ScrollSources: state.ScrollSources, ResourceList: state.ResourceList, ResourceBusy: state.ResourceBusy,
		Match: state.Match, MatchComplete: state.MatchComplete,
	})
}

// UnmarshalJSON accepts the old wire schema, but PanA/PanB are projections and
// cannot supply mutable state. As for other unknown fields, they are ignored.
// Existing Stream bytes determine every subsequent raster read and write.
func (state *ChuWorldState) UnmarshalJSON(data []byte) error {
	type plain ChuWorldState
	next := plain(*state)
	if err := json.Unmarshal(data, &next); err != nil {
		return err
	}
	*state = ChuWorldState(next)
	return nil
}
