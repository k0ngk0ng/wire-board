package engine

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func openingRAM(r []byte) OpeningState {
	s := OpeningState{Transition: transitionRAM(r), BallBounces: r[0x511], BallDeflection: DeflectionAxes(r[0x564]), ExcludedCatcher: int8(r[0x561]), DetachedCount: r[0xe4], ScoreSoundDelay: r[0xe3]}
	for i := 0; i < 4; i++ {
		s.Grades[i] = CharacterGrades{Run: r[0x615+i], Jump: r[0x619+i], Shot: r[0x61d+i], Recovery: r[0x621+i], Attack: r[0x625+i]}
		for j := 0; j < 3; j++ {
			s.Equipment[i][j] = int8(r[0x6e4+i*3+j])
		}
	}
	copy(s.Links[:], r[0x642:0x64f])
	copy(s.InputDelays[:], r[0x505:0x511])
	copy(s.MountFlags[:], r[0x6f0:0x6f6])
	copy(s.TacticCache[:], r[0x3c7:0x3cb])
	copy(s.Projection.DrawOrder[:], r[0x41d:0x42a])
	s.Projection.DrawCount = r[0x41c]
	for i := 0; i < 13; i++ {
		s.Projection.Projected[i] = ScreenPosition{X: int16(uint16(r[0x3e7+i]) | uint16(r[0x3f4+i])<<8), Y: int16(uint16(r[0x401+i]) | uint16(r[0x40e+i])<<8)}
	}
	return s
}
func TestOriginalOpeningStages(t *testing.T) {
	data, e := os.ReadFile("testdata/opening.json")
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
	f, e := os.Open("testdata/opening.ram.gz")
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
		before, after := openingRAM(raw[:2048]), openingRAM(raw[2048:])
		var got OpeningStep
		switch row.Kind {
		case "grades":
			got, e = InitializeOpeningGrades(before)
		case "court":
			got, e = InitializeOriginalCourt(before)
		case "start":
			got, e = StartOriginalCourt(before)
		case "throw":
			got, e = StepOriginalRefereeOpening(before)
		case "period":
			got, e = InitializeOpeningPeriod(before)
		case "team":
			got, e = EnterOriginalTeamTransition(before)
		default:
			t.Fatal(row.Kind)
		}
		if e != nil {
			t.Fatal(row.Kind, row.Frame, e)
		}
		got.State.Transition.Match.Clock.VideoRemainder += row.Ticks
		if got.State != after {
			a, b := reflect.ValueOf(got.State), reflect.ValueOf(after)
			for i := 0; i < a.NumField(); i++ {
				if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
					t.Errorf("%s frame %d field %s\ngot %+v\nwant %+v", row.Kind, row.Frame, a.Type().Field(i).Name, a.Field(i).Interface(), b.Field(i).Interface())
				}
			}
			t.FailNow()
		}
		if len(got.SoundIDs) != len(row.Sounds) || (len(row.Sounds) > 0 && !reflect.DeepEqual(got.SoundIDs, row.Sounds)) {
			t.Fatal("opening sound differs", row, got.SoundIDs)
		}
		counts[row.Kind]++
	}
	if !reflect.DeepEqual(counts, map[string]int{"court": 1, "grades": 1, "period": 1, "start": 1, "throw": 33}) {
		t.Fatal(counts)
	}
	t.Logf("matched all %d original initialization/opening calls: %v", len(rows), counts)
}
func TestNewMatchConfigurationBuildsOpeningWithoutFixtures(t *testing.T) {
	cfg := DefaultOriginalMatchConfiguration()
	cfg.Seed = RandomState(0x12345678)
	s, e := NewOriginalMatch(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if s.Transition.Random != cfg.Seed || s.Transition.Scores != ([2]uint16{}) || s.Transition.Match.Phase != 0 {
		t.Fatal(s)
	}
	first, e := InitializeOriginalCourt(s)
	if e != nil {
		t.Fatal(e)
	}
	start, e := StartOriginalCourt(first.State)
	if e != nil {
		t.Fatal(e)
	}
	s = start.State
	var sounds []uint8
	for n := 0; n < 33; n++ {
		s.Projection = resetOriginalWorldDrawList(s.Projection)
		next, e := StepOriginalRefereeOpening(s)
		if e != nil {
			t.Fatal(n, e)
		}
		s = next.State
		sounds = append(sounds, next.SoundIDs...)
	}
	if s.Transition.Match.Phase != MatchPlaying || s.Transition.Match.Entered || s.Transition.Objects[6].Body.Position.Height != 73*Pixel || s.Transition.Objects[6].Body.Velocity.Height != 11*Pixel || !reflect.DeepEqual(sounds, []uint8{55}) {
		t.Fatal(s.Transition.Objects[6], sounds)
	}
	done, e := InitializeOpeningPeriod(s)
	if e != nil || !done.BeginWorldInputs {
		t.Fatal(done, e)
	}
	if done.State.Transition.Random != cfg.Seed || done.State.Transition.Owners != ([9]uint8{255, 255, 6, 7, 8, 9, 10, 11, 12}) || done.State.MountFlags != ([6]uint8{1, 1, 1, 1, 1, 1}) || done.State.Grades[0] != (CharacterGrades{Run: 5, Jump: 3, Shot: 6, Recovery: 4, Attack: 3}) {
		t.Fatal(done, e)
	}
	profiles, e := done.State.CharacterProfiles()
	if e != nil || profiles[0].RunSpeed != 1344 || profiles[0].JumpVelocity != 2176 || profiles[0].ShotSkill != 6 {
		t.Fatal(profiles, e)
	}
	cfg.Court = 7
	if _, e = NewOriginalMatch(cfg); e == nil {
		t.Fatal("unsupported court opening accepted")
	}
}
