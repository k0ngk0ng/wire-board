package engine

// Failures: current simulation pose selects the displayed CHR too late; drawing
// clears the visible frame before DMA; NMI reversal leaks across a new draw;
// shadow cache follows the final position rather than real motion origin;
// glyph accents are mistaken for controller markers; HUD updates both sides
// instead of the selected original slot; restoring loses court-column history.
import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestOriginalDisplaySemanticStages(t *testing.T) {
	f, e := os.Open("testdata/display-stages.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []struct {
		Kind                                                string
		Frame                                               int
		Before, After, Pose, Phase, Team, Character, Tactic uint8
		Upper, Lower                                        []uint8
		State                                               struct {
			Objects           [13]RenderObject
			DrawOrder         []int
			Teams             [2]uint8
			BallSource, Phase uint8
		}
		Surfaces [13]uint8
	}
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	rawFile, e := os.Open("testdata/display-stages.ram.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer rawFile.Close()
	raw, e := gzip.NewReader(rawFile)
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Close()
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Kind]++
		switch r.Kind {
		case "chr":
			got, e := SelectOriginalBallCHR(r.Before, r.Pose, r.Phase)
			if e != nil || got != r.After {
				t.Fatalf("CHR frame %d got %d want %d err %v", r.Frame, got, r.After, e)
			}
		case "name":
			got, e := OriginalCharacterHUDName(r.Team, r.Character)
			if e != nil || !reflect.DeepEqual(got.Upper[:], r.Upper) || !reflect.DeepEqual(got.Lower[:], r.Lower) {
				t.Fatal(r, got, e)
			}
		case "tactic":
			got, e := OriginalTacticHUDLabel(r.Tactic)
			if e != nil || !reflect.DeepEqual(got.Upper[:], r.Upper) || !reflect.DeepEqual(got.Lower[:], r.Lower) {
				t.Fatal(r, got, e)
			}
		case "draw":
			b := make([]byte, 2048)
			if _, e = io.ReadFull(raw, b); e != nil {
				t.Fatal(e)
			}
			opening := openingRAM(b)
			config := DefaultOriginalMatchConfiguration()
			config.Teams = opening.Transition.Teams
			config.Characters = opening.Transition.Characters
			d, e := NewOriginalDisplay(config)
			if e != nil {
				t.Fatal(e)
			}
			d.SurfaceCodes = r.Surfaces
			d.BallCHR = b[0x54]
			got, e := StageDisplay(d, DisplaySourceFromOpening(opening))
			if e != nil {
				t.Fatalf("frame %d %v", r.Frame, e)
			}
			if got.Submitted.Objects != r.State.Objects || !reflect.DeepEqual(got.Submitted.DrawOrder, r.State.DrawOrder) || got.Submitted.Teams != r.State.Teams || got.Submitted.Phase != r.State.Phase || got.Submitted.BallSource != r.State.BallSource {
				t.Fatalf("draw frame %d\ngot %+v\nwant %+v", r.Frame, got.Submitted, r.State)
			}
		}
	}
	if counts["draw"] != 2083 || counts["chr"] != 2083 || counts["name"] != 4 || counts["tactic"] != 1020 {
		t.Fatal(counts)
	}
	t.Logf("matched original display calls %v", counts)
}

func TestDisplayVideoCommitAndRestore(t *testing.T) {
	d, e := NewOriginalDisplay(DefaultOriginalMatchConfiguration())
	if e != nil {
		t.Fatal(e)
	}
	source := DisplaySource{Teams: [2]uint8{0, 1}, Characters: [4]uint8{0, 1, 0, 1}, Camera: RenderCamera{X: 128, Y: 297}, Phase: 1}
	for i := range source.Projection.DrawOrder {
		source.Projection.DrawOrder[i] = 255
	}
	for i := range source.Objects {
		source.Objects[i].ID = uint8(i)
		source.Objects[i].Pose = 127
	}
	source.Objects[6].Pose = 4
	d, e = StageDisplay(d, source)
	if e != nil {
		t.Fatal(e)
	}
	if d.HasVisible || d.BallCHR != 35 {
		t.Fatal(d)
	}
	d, e = QueueDisplayColumn(d, 49, 1)
	if e != nil {
		t.Fatal(e)
	}
	d, e = CommitDisplayVideo(d, 128, 0)
	if e != nil || !d.HasVisible || d.Visible.ReverseSprites || d.Visible.BallCHR != 35 || d.CourtSerial != 1 {
		t.Fatal(d, e)
	}
	d, e = CommitDisplayVideo(d, 128, 0)
	if e != nil || !d.Visible.ReverseSprites {
		t.Fatal(d, e)
	}
	d, e = StageDisplay(d, source)
	if e != nil {
		t.Fatal(e)
	}
	d, e = CommitDisplayVideo(d, 128, 1)
	if e != nil || d.Visible.ReverseSprites {
		t.Fatal(d, e)
	}
	for i := 0; i < 90; i++ {
		d, e = QueueDisplayColumn(d, uint8(i%64), 1)
		if e != nil {
			t.Fatal(e)
		}
		d, e = CommitDisplayVideo(d, 0, 0)
		if e != nil {
			t.Fatal(e)
		}
	}
	if len(d.CourtColumns) > 32 {
		t.Fatal("unbounded restore history", len(d.CourtColumns))
	}
	data, e := json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
	var restored DisplayState
	if e = json.Unmarshal(data, &restored); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(d, restored) {
		t.Fatal("display restore differs")
	}
	next, e := CommitDisplayVideo(restored, 0, 0)
	if e != nil || next.CourtSerial != d.CourtSerial {
		t.Fatal("restore replays old column", next, e)
	}
}

func TestDisplayHUDNamesAndSelectedUpdate(t *testing.T) {
	c := DefaultOriginalMatchConfiguration()
	d, e := NewOriginalDisplay(c)
	if e != nil {
		t.Fatal(e)
	}
	if d.HUD.Panels[1].HeaderGlyphs[6] != 254 {
		t.Fatal("original voiced accent missing", d.HUD)
	}
	before := MatchState{Clock: MatchClock{HUDTurn: 0}, Period: 0}
	after := before
	after.Clock.HUDTurn = 1
	after.DisplayedScores[0] = [3]uint8{1, 2, 3}
	d, e = ObserveDisplayHUD(d, before, after, [4]uint8{1, 2, 0, 0})
	if e != nil {
		t.Fatal(e)
	}
	if d.HUD.Scores[0] != 123 || d.HUD.Scores[1] != 0 {
		t.Fatal(d.HUD)
	}
	before = after
	after.Clock.HUDTurn = 2
	after.Clock.Minutes, after.Clock.Tens, after.Clock.Ones = 2, 4, 5
	after.Period = 1
	d, e = ObserveDisplayHUD(d, before, after, [4]uint8{})
	if e != nil || d.HUD.Clock.Minutes != 2 || d.HUD.Clock.Seconds != 45 || d.HUD.ClockLabel[0] != 0xf2 {
		t.Fatal(d, e)
	}
	if _, e = OriginalCharacterHUDName(8, 0); e == nil {
		t.Fatal("unknown team accepted")
	}
	if _, e = OriginalTacticHUDLabel(13); e == nil {
		t.Fatal("unknown tactic accepted")
	}
}

func TestDisplayShadowCacheAgainstRealMotionWrites(t *testing.T) {
	count := 0
	for _, name := range []string{"motion-surface.json.gz", "motion-surface-rider.json.gz"} {
		f, e := os.Open("testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		z, e := gzip.NewReader(f)
		if e != nil {
			t.Fatal(e)
		}
		var rows []struct {
			Frame    int
			Writes   []MotionSurfaceWrite
			Surfaces []uint8
		}
		if e = json.NewDecoder(z).Decode(&rows); e != nil {
			t.Fatal(e)
		}
		z.Close()
		f.Close()
		d := DisplayState{}
		for _, row := range rows {
			if len(row.Writes) != len(row.Surfaces) {
				t.Fatal("surface alignment")
			}
			for i, w := range row.Writes {
				next, e := ObserveDisplayMotion(d, []MotionSurfaceWrite{w})
				if e != nil || next.SurfaceCodes[w.Object] != row.Surfaces[i] {
					t.Fatalf("%s frame%d object%d origin%+v got%d want%d err%v", name, row.Frame, w.Object, w.Origin, next.SurfaceCodes[w.Object], row.Surfaces[i], e)
				}
				d = next
				count++
			}
		}
	}
	if count != 14845 {
		t.Fatal(count)
	}
	t.Log("matched 14845 original terrain-cache writes")
}

func TestOriginalTeamHUDNamesAndControllerMetadata(t *testing.T) {
	name, e := OriginalTeamHUDName(1)
	if e != nil || name.Lower != ([13]uint8{0, 0, 0, 11, 187, 213, 11, 216, 11, 159, 11, 0, 0}) {
		t.Fatal(name, e)
	}
	c := DefaultOriginalMatchConfiguration()
	c.Teams = [2]uint8{7, 2}
	c.Characters = [4]uint8{3, 2, 1, 0}
	c.ControllerAssignments = [4]int8{3, -1, 1, -1}
	d, e := NewOriginalDisplay(c)
	if e != nil || d.Controllers != c.ControllerAssignments {
		t.Fatal(d, e)
	}
	for side := range c.Teams {
		want, e := OriginalTeamHUDName(c.Teams[side])
		if e != nil || want != d.HUD.TeamNames[side] {
			t.Fatal(d.HUD, e)
		}
	}
}

func TestDisplayCourtResetAndSecondHalfHUD(t *testing.T) {
	d, e := NewOriginalDisplay(DefaultOriginalMatchConfiguration())
	if e != nil {
		t.Fatal(e)
	}
	d, e = QueueDisplayColumn(d, 50, 1)
	if e != nil {
		t.Fatal(e)
	}
	d, e = CommitDisplayVideo(d, 0, 0)
	if e != nil {
		t.Fatal(e)
	}
	d.SurfaceCodes[3] = 64
	reset := ResetDisplayCourt(d)
	if reset.CourtEpoch != d.CourtEpoch+1 || reset.CourtSerial != 0 || len(reset.CourtColumns) != 0 || len(d.CourtColumns) != 1 || reset.SurfaceCodes[3] != 64 {
		t.Fatal(reset, d)
	}
	m := MatchState{Period: 1, DisplayedScores: [2][3]uint8{{1, 2, 3}, {0, 4, 5}}}
	next, e := InitializeOriginalDisplayHUD(d, DefaultOriginalMatchConfiguration(), m)
	if e != nil || next.HUD.ClockLabel[0] != 0xf2 || next.HUD.Scores != ([2]uint16{123, 45}) || next.HUD.Clock != (RenderClock{Minutes: 3}) {
		t.Fatal(next, e)
	}
}
