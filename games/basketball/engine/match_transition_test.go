package engine

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func transitionRAM(r []byte) MatchTransitionState {
	b := boardContactRAM(r)
	word := func(a int) uint16 { return binary.LittleEndian.Uint16(r[a : a+2]) }
	s := MatchTransitionState{Scene: r[0x588], Mode: r[0x58d], Court: r[0x58b], SpecialHolder: int8(r[0x56b]), PassTarget: int8(r[0x569]), TrackedObject: r[0x3a2], DisplayFlags: r[0x6d6], EndCounter: r[0x6d5], SelectedController: int8(r[0x6ce]), LastShooter: r[0x6d1], ResultTimer: r[0x3c3], FirstTeamLost: r[0x3c5] >= 128, ResultExitFlag: int8(r[0x3d2]), CameraScroll: [4]uint16{word(0x397), word(0x399), word(0x39b), word(0x39d)}, Random: b.Random, Scores: b.Scores}
	s.Match = MatchState{Clock: MatchClock{Minutes: r[0x6c2], Tens: r[0x6c3], Ones: r[0x6c4], VideoRemainder: r[0x6c5], DisplayedSecond: r[0x3cd], HUDTurn: r[0x3ce], ScoreLowCache: [2]uint8{r[0x3cb], r[0x3cc]}}, Phase: MatchPhase(r[0x589] & 127), Entered: r[0x589] >= 128, Period: r[0x58c], OvertimeFlags: r[0x6f7], DisplayedScores: [2][3]uint8{{r[0x160], r[0x161], r[0x162]}, {r[0x163], r[0x164], r[0x165]}}}
	for i := 0; i < 13; i++ {
		var p Player
		if i < 4 {
			p = b.Players[i]
		} else if i < 6 {
			a := b.Auxiliary[i-4]
			p = Player{Body: a.Body, Action: Action(a.Action), Entered: a.Entered, Request: a.Request, Pose: a.Pose, Animation: a.Animation, Airborne: a.Airborne, FacingLeft: a.FacingLeft}
		} else if i == 6 {
			a := b.Ball
			p = Player{Body: a.Body, Action: Action(a.Action), Entered: a.Entered, Request: ActionRequest{Action: Action(a.Request.Action), Pending: a.Request.Pending}, Pose: a.Pose, Animation: a.Animation, Airborne: a.Airborne, FacingLeft: a.FacingLeft}
		} else {
			a := b.Hoops[i-7]
			p = Player{Body: Body{Position: a.Position, Velocity: a.Velocity, Gravity: a.Gravity}, Action: Action(a.Action), Entered: a.Entered, Request: ActionRequest{Action: Action(a.Request.Action), Pending: a.Request.Pending}, Pose: a.Pose, Animation: a.Animation, Airborne: a.Airborne, FacingLeft: a.FacingLeft}
		}
		s.Objects[i] = PeriodObject{Body: p.Body, Action: uint8(p.Action), Entered: p.Entered, Request: p.Request, Pose: p.Pose, Animation: p.Animation, Airborne: p.Airborne, FacingLeft: p.FacingLeft}
		s.Kinds[i] = r[0x3d6+i]
	}
	copy(s.Owners[:], r[0x53d:0x546])
	copy(s.PlayerControls[:], r[0x539:0x53d])
	copy(s.Characters[:], r[0x3e3:0x3e7])
	copy(s.Teams[:], r[0x586:0x588])
	copy(s.Endurance[:], r[0x611:0x615])
	copy(s.AIModes[:], r[0x672:0x676])
	copy(s.AIPlans[:], r[0x676:0x67a])
	copy(s.TeamAI[:], r[0x663:0x665])
	copy(s.TeamFacing[:], r[0x584:0x586])
	copy(s.EndActorFlags[:], r[0x6ca:0x6ce])
	copy(s.Passages[:], r[0x6d7:0x6e0])
	copy(s.TeamWinBits[:], r[0x6d2:0x6d4])
	for i := 0; i < 4; i++ {
		s.ControllerAssignments[i] = int8(r[0x4e8+i])
		s.ShotCycle[i] = int8(r[0x572+i])
	}
	for i := 0; i < 2; i++ {
		s.AuxiliaryLinks[i] = int8(r[0x56f+i])
		s.ResultControllers[i] = int8(r[0x6cf+i])
	}
	for i := 0; i < 6; i++ {
		s.HoopDurability[i] = int8(r[0x57e+i])
	}
	return s
}
func TestOriginalMatchTransitions(t *testing.T) {
	data, e := os.ReadFile("testdata/match-transition.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Kind   string
		Frame  int
		Ticks  uint8
		Sounds []uint8
	}
	if e = json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open("testdata/match-transition.ram.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	counts := map[string]int{}
	for _, row := range rows {
		var raw [4096]byte
		if _, e = io.ReadFull(z, raw[:]); e != nil {
			t.Fatal(e)
		}
		s, want := transitionRAM(raw[:2048]), transitionRAM(raw[2048:])
		var got MatchTransitionStep
		switch row.Kind {
		case "setup":
			got, e = InitializeOriginalPeriod(s)
		case "wait":
			got, e = WaitOriginalPeriodEnd(s)
		case "reset":
			got, e = ResetOriginalPeriod(s)
		case "result":
			got, e = StepOriginalMatchResult(s, raw[4])
		default:
			t.Fatal(row.Kind)
		}
		if e != nil {
			t.Fatal(row.Kind, row.Frame, e)
		}
		got.State.Match.Clock.VideoRemainder += row.Ticks
		if got.State != want {
			a, b := reflect.ValueOf(got.State), reflect.ValueOf(want)
			for i := 0; i < a.NumField(); i++ {
				if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
					t.Errorf("%s frame %d field %s\ngot %+v\nwant %+v", row.Kind, row.Frame, a.Type().Field(i).Name, a.Field(i).Interface(), b.Field(i).Interface())
				}
			}
			t.FailNow()
		}
		if len(got.SoundIDs) != len(row.Sounds) || (len(row.Sounds) > 0 && !reflect.DeepEqual(got.SoundIDs, row.Sounds)) {
			t.Fatal("transition sound mismatch", row, got.SoundIDs)
		}
		counts[row.Kind]++
	}
	if !reflect.DeepEqual(counts, map[string]int{"setup": 2, "wait": 4, "reset": 2, "result": 145}) {
		t.Fatal("transition coverage changed", counts)
	}
	t.Logf("matched all %d semantic phase transitions and result updates: %v", len(rows), counts)
}
func TestPeriodResetPreservesScoresAndUntouchedSubpixels(t *testing.T) {
	s := MatchTransitionState{Match: MatchState{Phase: 4}, Scores: [2]uint16{12, 34}, Characters: [4]uint8{0, 1, 2, 3}}
	for i := range s.Objects {
		s.Objects[i] = PeriodObject{Body: Body{Position: Position{X: 400*Pixel + 13, Depth: 70*Pixel + 17, Height: 80*Pixel + 19}, Velocity: Velocity{X: 20, Depth: 21, Height: 22}, Gravity: 23}, Action: 4, Entered: true, Animation: AnimationState{Cursor: 4, Elapsed: 5, Loops: 6}, Airborne: true}
	}
	got, e := ResetOriginalPeriod(s)
	if e != nil || got.State.Scores != s.Scores || got.State.Match.Period != 1 || got.State.Scene != 2 || got.State.Match.Phase != 4 {
		t.Fatal(got, e)
	}
	o := got.State.Objects[0]
	if o.Body.Position != (Position{X: 144*Pixel + 13, Depth: 70*Pixel + 17, Height: 19}) || o.Body.Velocity != (Velocity{}) || o.Body.Gravity != 0 || !o.Airborne || o.Animation.Loops != 6 {
		t.Fatal(o)
	}
	s.Match.Period = 1
	got, e = ResetOriginalPeriod(s)
	if e != nil || got.State.Match.Phase != 6 || got.State.Scene != s.Scene || got.State.Match.Period != 2 {
		t.Fatal(got, e)
	}
}
func TestPeriodSetupModesAndResultBranches(t *testing.T) {
	s := MatchTransitionState{Match: MatchState{Phase: MatchPlaying, Period: 1}, Court: 1, ControllerAssignments: [4]int8{0, -1, -1, -1}, Scores: [2]uint16{4, 3}}
	got, e := InitializeOriginalPeriod(s)
	if e != nil || got.State.AIModes != ([4]uint8{}) || got.State.AIPlans != got.State.AIModes || got.State.Match.Clock.Minutes != 3 || got.State.Match.Clock.Ones != 1 || got.State.Scores != s.Scores {
		t.Fatal(got, e)
	}
	s.Match.Phase = 6
	s.Match.Period = 2
	s.Teams = [2]uint8{0, 2}
	got, e = StepOriginalMatchResult(s, 0)
	if e != nil || got.State.FirstTeamLost || got.State.TeamWinBits[0] != 2 || got.State.Scores != s.Scores {
		t.Fatal(got, e)
	}
	s = got.State
	s.ResultTimer = 143
	got, e = StepOriginalMatchResult(s, 0)
	if e != nil || got.State.Match.Phase != 7 || got.State.Characters[0] != 0 || got.State.Characters[2] != 2 || got.State.Random != s.Random {
		t.Fatal(got, e)
	}
	s.Mode = 128
	s.ResultTimer = 143
	s.ResultExitFlag = -1
	got, e = StepOriginalMatchResult(s, 0)
	if e != nil || got.State.Scene != 2 || got.State.Match.Phase != 2 || got.State.Mode != 129 || got.State.ControllerAssignments != ([4]int8{-1, -1, -1, -1}) {
		t.Fatal(got, e)
	}
}
