package engine

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

// NewOriginalPostResultVideoAfterResult bridges an actual completed BA40 actor
// tail and committed black fade. beforeAward MUST be the live pre-award
// campaign inventory, normally Config.Campaign, never SettleCampaignRuntime's
// already-awarded Progress. Period/wins come from the confirmed scene.
//
// The legacy runtime has no original menu text/queue history. It supplies a
// canonical inactive history here: no pending text, FF queue headers, zero
// unused bytes. Source acceptance proves equal effective writes/continuations
// for this boundary, not byte identity of unavailable inactive RAM. No source
// fixture, fresh match, score, seed or actor reinitialization is used.
func NewOriginalPostResultVideoAfterResult(scene OpeningState, display DisplayState, beforeAward campaign.Progress, logicByte uint8) (PostResultVideoState, error) {
	t := scene.Transition
	validDestination := t.Scene == 1 && t.Match.Phase == 7 && t.Match.Period < 28 || t.Scene == 7 && t.Match.Phase == 0 && t.Match.Period == 28
	if !validDestination || t.Match.Entered || t.FirstTeamLost || t.Mode&128 != 0 || t.Court >= 8 || t.Teams[0] >= 8 || t.Teams[1] >= 8 {
		return PostResultVideoState{}, fmt.Errorf("postresult handoff requires an unentered campaign winning destination")
	}
	if display.PaletteFade != nil || display.PendingPaletteLevel != nil || display.PaletteLevel == nil || *display.PaletteLevel != 0 {
		return PostResultVideoState{}, fmt.Errorf("postresult handoff requires committed result fade-out")
	}
	if display.BallCHR != 13 {
		return PostResultVideoState{}, fmt.Errorf("postresult handoff requires source result CHR13")
	}
	if beforeAward.Mode != t.Mode {
		return PostResultVideoState{}, fmt.Errorf("postresult campaign mode differs")
	}
	if e := beforeAward.Validate(); e != nil {
		return PostResultVideoState{}, e
	}
	progress := beforeAward
	progress.Period, progress.WinBits = t.Match.Period, t.TeamWinBits
	if e := progress.Validate(); e != nil {
		return PostResultVideoState{}, e
	}
	palette := display.PaletteID
	if palette == 0 {
		palette = t.Court + 1
	}
	if !validDisplayCourtPalette(t.Court, palette) {
		return PostResultVideoState{}, fmt.Errorf("postresult court palette differs")
	}
	field := originalPresentationResultFieldCHR[t.Court]
	s := PostResultVideoState{Scene: scene, Progress: progress, FieldCHR: [2]uint8{field, field + 2}, PaletteBuildLevel: 0xf0, Control: 0x88 | uint8((t.CameraScroll[0]>>8)&1) | uint8((t.CameraScroll[1]>>7)&2), Mask: 0x18, LogicByte: logicByte}
	s.Context.Ending = EndingPhaseContext{Actor: ResultActorContext{PaletteIDs: [2]uint8{t.Teams[0] + 2, t.Teams[1] + 2}, DisplayMask: palette}, MachineControl: 0xc0, Background: 99, FadeMode: 128, FadeDelay: 1, CHRPages: [4]uint8{0, t.Teams[0] + 1, t.Teams[1] + 1, 13}, Upload: EndingUpload{Address: 0xff00}}
	s.Context.SecondUpload = EndingUpload{Address: 0xff00}
	s.TailUploads[0] = EndingUpload{Address: 0xff00}
	black, e := BuildOriginalPresentationPalette(EndingUpload{}, palette, s.Context.Ending.Actor.PaletteIDs, 128, 0)
	if e != nil {
		return PostResultVideoState{}, e
	}
	black.Address = 0xff00
	s.TailUploads[1] = black
	s.Clock.Counter = 1
	return WaitAfterOriginalPostResultDispatch(s), nil
}
