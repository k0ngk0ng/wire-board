package runtimebridge

import (
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/games/basketball/syncwire"
)

// DisplayView contains committed visible data only. Submitted/pending pictures,
// simulation fields and AI memory never cross the hot JS/Wasm call boundary.
type DisplayView struct {
	HasVisible   bool
	Visible      engine.BasketballRenderSnapshot
	CourtEpoch   uint64
	CourtSerial  uint64
	CourtColumns []engine.CourtColumnCommit
	CourtPatches []engine.CourtTilePatch `json:",omitempty"`
}
type CompactVideoFrame struct {
	Frame   uint64      `json:"frame"`
	Display DisplayView `json:"display"`
}
type CompactView struct {
	Session    uint64              `json:"session"`
	Frame      uint64              `json:"frame"`
	Finished   bool                `json:"finished"`
	HasOutcome bool                `json:"hasOutcome"`
	Display    DisplayView         `json:"display"`
	Digest     string              `json:"digest,omitempty"`
	SoundIDs   []int               `json:"soundIDs,omitempty"`
	Frames     []CompactVideoFrame `json:"frames,omitempty"`
}

func visibleDisplay(d engine.DisplayState) DisplayView {
	return DisplayView{HasVisible: d.HasVisible, Visible: d.Visible, CourtEpoch: d.CourtEpoch, CourtSerial: d.CourtSerial, CourtColumns: d.CourtColumns, CourtPatches: d.CourtPatches}
}
func (b *Bridge) createRequest(state engine.RuntimeState, request Request) Response {
	r := b.create(state)
	if !r.OK {
		return r
	}
	return b.reply(request, r.Data.(View))
}
func (b *Bridge) reply(request Request, view View) Response {
	if !request.Compact {
		return success(view)
	}
	value := CompactView{Session: view.Session, Frame: view.State.VideoFrames, Finished: engine.RuntimeFinished(view.State), HasOutcome: view.State.Outcome != nil, Display: visibleDisplay(view.State.Display), SoundIDs: view.SoundIDs, Frames: view.renderFrames}
	if request.Verify {
		var err error
		value.Digest, err = syncwire.RuntimeDigest(view.State)
		if err != nil {
			return failed("checksum_error", err)
		}
	}
	return success(value)
}
