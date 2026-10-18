package engine

import "fmt"

type RenderObject struct {
	ID               uint8          `json:"id"`
	Kind             uint8          `json:"kind"`
	Character        uint8          `json:"character"`
	Pose             uint8          `json:"pose"`
	Projected        ScreenPosition `json:"projected"`
	Position         Position       `json:"position"`
	Airborne         bool           `json:"airborne"`
	ShadowSuppressed bool           `json:"shadowSuppressed"`
	Action           uint8          `json:"action"`
}
type RenderCamera struct {
	X uint16 `json:"x"`
	Y uint16 `json:"y"`
}
type RenderClock struct {
	Minutes uint8 `json:"minutes"`
	Seconds uint8 `json:"seconds"`
}
type RenderHUDPanel struct {
	Lines        [3][9]uint8 `json:"lines"`
	HeaderGlyphs [9]uint8    `json:"headerGlyphs"`
	EdgeGlyphs   [3]uint8    `json:"edgeGlyphs"`
	TurnMarker   *uint8      `json:"turnMarker"` // nil: original upper marks are glyph accents.
}
type RenderHUD struct {
	TeamNames  [2]TeamHUDName    `json:"teamNames"`
	Clock      RenderClock       `json:"clock"`
	ClockLabel [4]uint8          `json:"clockLabel"`
	Scores     [2]uint16         `json:"scores"`
	Panels     [2]RenderHUDPanel `json:"panels"`
}
type RenderResult struct {
	Court       uint8       `json:"court"`
	Digits      [2][3]uint8 `json:"digits"`
	WinningTeam uint8       `json:"winningTeam"`
}
type BasketballRenderSnapshot struct {
	PaletteID      uint8            `json:"paletteID,omitempty"`
	PaletteLevel   *uint8           `json:"paletteLevel,omitempty"` // nil: legacy full colors; 0: original black palette.
	Court          uint8            `json:"court"`
	Result         *RenderResult    `json:"result,omitempty"`
	Objects        [13]RenderObject `json:"objects"`
	DrawOrder      []int            `json:"drawOrder"`
	Teams          [2]uint8         `json:"teams"`
	Camera         RenderCamera     `json:"camera"`
	ReverseSprites bool             `json:"reverseSprites"`
	BallCHR        uint8            `json:"ballCHR"`
	BallSource     uint8            `json:"ballSource"`
	Phase          uint8            `json:"phase"`
	HUD            RenderHUD        `json:"hud"`
	Controllers    [4]int8          `json:"controllers"`
}
type CourtColumnCommit struct {
	Column uint8 `json:"column"`
	Phase  uint8 `json:"phase"`
}

// DisplayState retains submitted and visible images independently. CourtColumns
// is a bounded chronological list: only the last write to each physical column
// is needed to rebuild both tiles and its four-column attribute group.
type DisplayState struct {
	// Palette commands share the video boundary with original PPU updates.
	PaletteID                 uint8               `json:",omitempty"`
	PendingPaletteID          uint8               `json:",omitempty"`
	PaletteLevel              *uint8              `json:",omitempty"`
	PendingPaletteLevel       *uint8              `json:",omitempty"`
	PaletteFade               *DisplayPaletteFade `json:",omitempty"`
	Submitted, Visible        BasketballRenderSnapshot
	HasSubmission, HasVisible bool
	BallCHR                   uint8
	SurfaceCodes              [13]uint8
	HUD                       RenderHUD
	Controllers               [4]int8
	PendingColumn             *CourtColumnCommit
	CourtColumns              []CourtColumnCommit
	CourtPatches              []CourtTilePatch `json:",omitempty"`
	PendingPatches            []CourtTilePatch `json:",omitempty"`
	CourtEpoch, CourtSerial   uint64
}
type DisplaySource struct {
	Court                                    uint8
	Result                                   *RenderResult
	Objects                                  [13]RenderObject
	Projection                               ProjectionState
	Camera                                   RenderCamera
	Teams                                    [2]uint8
	Characters                               [4]uint8
	Phase, Period                            uint8
	Controls                                 [4]uint8
	DisplayFlags, DetachedCount, LastShooter uint8
}
type TeamHUDName struct {
	Upper [13]uint8 `json:"upper"`
	Lower [13]uint8 `json:"lower"`
}

func OriginalTeamHUDName(team uint8) (TeamHUDName, error) {
	if team >= 8 {
		return TeamHUDName{}, fmt.Errorf("original team name %d not recovered", team)
	}
	return originalTeamHUDNames[team], nil
}

type CharacterHUDName struct{ Upper, Lower [4]uint8 }
type TacticHUDLabel struct{ Upper, Lower [6]uint8 }

func OriginalCharacterHUDName(team, character uint8) (CharacterHUDName, error) {
	if team >= 8 || character >= 4 {
		return CharacterHUDName{}, fmt.Errorf("original HUD team/character out of range")
	}
	return originalCharacterHUDNames[team][character], nil
}
func OriginalTacticHUDLabel(tactic uint8) (TacticHUDLabel, error) {
	if tactic >= 13 {
		return TacticHUDLabel{}, fmt.Errorf("original HUD tactic %d not recovered", tactic)
	}
	return originalTacticHUDLabels[tactic], nil
}
func NewOriginalDisplay(c OriginalMatchConfiguration) (DisplayState, error) {
	s := DisplayState{BallCHR: 32, CourtEpoch: 1, Controllers: c.ControllerAssignments, HUD: RenderHUD{Clock: RenderClock{Minutes: 3}, ClockLabel: [4]uint8{0xf1, 0xf3}}}
	s.PaletteLevel = displayPaletteLevel(0)
	for i, v := range c.ControllerAssignments {
		if v < -1 || v > 3 {
			return DisplayState{}, fmt.Errorf("invalid displayed controller for seat %d", i)
		}
	}
	for side := 0; side < 2; side++ {
		teamName, e := OriginalTeamHUDName(c.Teams[side])
		if e != nil {
			return DisplayState{}, e
		}
		s.HUD.TeamNames[side] = teamName
		p := &s.HUD.Panels[side]
		p.EdgeGlyphs = [3]uint8{0xc8, 0xc8, 0xc8}
		for i := range p.HeaderGlyphs {
			p.HeaderGlyphs[i] = 0xe5
		}
		for player := 0; player < 2; player++ {
			name, e := OriginalCharacterHUDName(c.Teams[side], c.Characters[side*2+player])
			if e != nil {
				return DisplayState{}, e
			}
			copy(p.HeaderGlyphs[player*5:], name.Upper[:])
			copy(p.Lines[0][player*5:], name.Lower[:])
		}
	}
	return s, nil
}

// SelectOriginalBallCHR is F049/F061. A raw scene byte of 0x86 retains the
// current selection. The function runs after submitted drawing, before logic.
func SelectOriginalBallCHR(previous, packedPose, rawPhase uint8) (uint8, error) {
	if rawPhase == 0x86 {
		return previous, nil
	}
	pose := packedPose & 127
	if pose == 127 {
		pose = 0
	}
	if pose >= 40 {
		return previous, fmt.Errorf("ball display pose %d not recovered", pose)
	}
	return originalDisplayBallCHR[pose], nil
}
func StageDisplay(original DisplayState, src DisplaySource) (DisplayState, error) {
	s := original
	bank, e := SelectOriginalBallCHR(s.BallCHR, src.Objects[6].Pose, src.Phase)
	if e != nil {
		return original, e
	}
	s.BallCHR = bank
	out := BasketballRenderSnapshot{PaletteID: s.PaletteID, PaletteLevel: s.PaletteLevel, Court: src.Court, Result: src.Result, Objects: src.Objects, Teams: src.Teams, Camera: src.Camera, BallCHR: bank, BallSource: src.LastShooter, Phase: src.Phase & 127, HUD: s.HUD, Controllers: s.Controllers, DrawOrder: []int{}}
	for i := range out.Objects {
		out.Objects[i].ID = uint8(i)
		out.Objects[i].Projected = src.Projection.Projected[i]
		out.Objects[i].ShadowSuppressed = s.SurfaceCodes[i] != 0
		if i < 4 {
			out.Objects[i].Character = src.Characters[i]
		}
	}
	for _, id := range src.Projection.DrawOrder[:12] {
		if id < 13 {
			out.DrawOrder = append(out.DrawOrder, int(id))
		}
	}
	s.Submitted = out
	s.HasSubmission = true
	return s, nil
}
func QueueDisplayColumn(original DisplayState, column, phase uint8) (DisplayState, error) {
	if column >= 64 {
		return original, fmt.Errorf("display court column %d out of range", column)
	}
	s := original
	s.PendingColumn = &CourtColumnCommit{Column: column, Phase: phase & 127}
	return s, nil
}
func CommitDisplayVideo(original DisplayState, flags, detached uint8) (DisplayState, error) {
	s := original
	if err := commitDisplayPaletteFade(&s); err != nil {
		return original, err
	}
	if err := commitDisplayCourtPatches(&s, s.PendingColumn); err != nil {
		return original, err
	}
	if s.HasSubmission {
		s.Visible = s.Submitted
		s.Visible.HUD = s.HUD
		s.Visible.BallCHR = s.BallCHR
		s.HasVisible = true
	}
	if s.PendingPaletteID != 0 {
		if s.PendingPaletteID > 15 {
			return original, fmt.Errorf("invalid pending display palette")
		}
		s.PaletteID = s.PendingPaletteID
		s.PendingPaletteID = 0
	}
	if s.HasVisible {
		s.Visible.PaletteID = s.PaletteID
		s.Visible.PaletteLevel = s.PaletteLevel
	}
	if s.PendingColumn != nil {
		next := *s.PendingColumn
		history := make([]CourtColumnCommit, 0, 32)
		for _, old := range s.CourtColumns {
			if old.Column&31 != next.Column&31 {
				history = append(history, old)
			}
		}
		s.CourtColumns = append(history, next)
		s.PendingColumn = nil
		s.CourtSerial++
	}
	if flags&128 != 0 && detached == 0 {
		s.Submitted.ReverseSprites = !s.Submitted.ReverseSprites
	}
	return s, nil
}
func ObserveDisplayHUD(original DisplayState, before, after MatchState, tactics [4]uint8) (DisplayState, error) {
	s := original
	// The caller invokes this only when the original HUD routine actually ran;
	// unchanged slot values remain valid because the scheduler can wrap to itself.
	if after.Clock.HUDTurn&1 == 0 {
		s.HUD.Clock = RenderClock{Minutes: after.Clock.Minutes, Seconds: after.Clock.Tens*10 + after.Clock.Ones}
		s.HUD.ClockLabel = [4]uint8{0xf1, 0xf3}
		if after.Period&1 != 0 {
			s.HUD.ClockLabel[0] = 0xf2
		}
		if after.OvertimeFlags&128 != 0 {
			s.HUD.ClockLabel = [4]uint8{0xe9, 0xf0}
		}
	} else {
		side := after.Clock.HUDTurn >> 1
		if side >= 2 {
			return original, fmt.Errorf("invalid display HUD slot")
		}
		p := &s.HUD.Panels[side]
		// B17A places the team's main message at 0 and its response at 7.
		// B074 uploads only nine glyphs: six message cells, a blank, then
		// two response cells. These are messages, not per-player labels.
		for message, offset := range [2]int{0, 7} {
			label, e := OriginalTacticHUDLabel(tactics[int(side)*2+message])
			if e != nil {
				return original, e
			}
			copy(p.Lines[1][offset:], label.Upper[:])
			copy(p.Lines[2][offset:], label.Lower[:])
		}
		p.Lines[1][6], p.Lines[2][6] = 0, 0
		score := after.DisplayedScores[side]
		s.HUD.Scores[side] = uint16(score[0])*100 + uint16(score[1])*10 + uint16(score[2])
	}
	_ = before
	return s, nil
}

// ResetDisplayCourt mirrors an explicit background reload, not a network
// reconnect. It does not clear terrain caches that the original loader retains.
func ResetDisplayCourt(original DisplayState) DisplayState {
	s := original
	s.PendingColumn = nil
	s.CourtColumns = nil
	s.CourtPatches = nil
	s.PendingPatches = nil
	s.CourtSerial = 0
	s.CourtEpoch++
	return s
}

// InitializeOriginalDisplayHUD restores the B39A name/period template writes.
// An odd period copies the retained decimal scoreboard into that new template.
func InitializeOriginalDisplayHUD(original DisplayState, c OriginalMatchConfiguration, m MatchState) (DisplayState, error) {
	fresh, e := NewOriginalDisplay(c)
	if e != nil {
		return original, e
	}
	s := original
	s.HUD = fresh.HUD
	s.Controllers = c.ControllerAssignments
	if m.Period&1 != 0 {
		s.HUD.ClockLabel[0] = 0xf2
		for side, digits := range m.DisplayedScores {
			s.HUD.Scores[side] = uint16(digits[0])*100 + uint16(digits[1])*10 + uint16(digits[2])
		}
	}
	return s, nil
}

// QueueDisplayPalette models FCEC's palette upload request. Palette zero is
// reserved for an absent override in old snapshots, never a real command.
func QueueDisplayPalette(original DisplayState, palette uint8) (DisplayState, error) {
	return QueueDisplayPaletteLevel(original, palette, 4)
}
