package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Surface failure inventory precedes implementation in the source report.
func TestOriginalPostResultSurfaceSourceUploads(t *testing.T) {
	var cases []struct {
		Before, After OriginalRasterSurface
		Address       uint16
		Data          []uint8
		Control       uint8
	}
	readPropAttackFixture(t, "testdata/postresult-surface.json.gz", &cases)
	if len(cases) != 720 {
		t.Fatalf("source upload cases%d", len(cases))
	}
	for i, c := range cases {
		before := c.Before
		got, e := CommitOriginalPostResultSurface(c.Before, c.Address, c.Data, c.Control&4 != 0)
		if e != nil || got != c.After {
			t.Fatalf("upload%d %04x control%d: %v", i, c.Address, c.Control, e)
		}
		if c.Before != before {
			t.Fatal("mutated prior surface")
		}
	}
}

func TestOriginalPostResultDisplayCommitOwnership(t *testing.T) {
	var state OriginalPostResultDisplayState
	for i := range state.WorkingOAM {
		state.WorkingOAM[i] = uint8(i)
	}
	state.Surface.Mirroring = 1
	initial := state
	queued := PostResultVideoEffect{Kind: "upload-queued", Upload: EndingUpload{Address: 0x2000, Length: 1, Data: [32]uint8{7}}}
	got, e := ApplyOriginalPostResultDisplayEffects(state, []PostResultVideoEffect{queued, {Kind: "sound", Value: 75}})
	if e != nil || got != state {
		t.Fatal("renderer consumed pending queue or owned sound", e)
	}
	committed := queued
	committed.Kind = "upload-commit"
	got, e = ApplyOriginalPostResultDisplayEffects(got, []PostResultVideoEffect{committed, {Kind: "normal-nmi", Raster: OriginalPostResultRasterSource{Scene: 1, Phase: 7, MachineControl: 192, BackgroundCHR: [2]uint8{68, 70}, SpriteCHR: [4]uint8{0, 59, 59, 32}, Control: 138, Mask: 24}}, {Kind: "oam-reverse"}})
	if e != nil {
		t.Fatal(e)
	}
	if got.Surface.Nametables[0][0] != 7 || got.VisibleOAM != initial.WorkingOAM || got.WorkingOAM != ReverseOriginalPostResultOAM(initial.WorkingOAM) {
		t.Fatal("commit/DMA/reversal order")
	}
	if state != initial {
		t.Fatal("effect application mutated prior display")
	}
	saved, e := json.Marshal(got)
	if e != nil {
		t.Fatal(e)
	}
	var restored OriginalPostResultDisplayState
	if e = json.Unmarshal(saved, &restored); e != nil || !reflect.DeepEqual(got, restored) {
		t.Fatal("display restore", e)
	}
	invalid := []PostResultVideoEffect{committed, {Kind: "unknown"}}
	failed, e := ApplyOriginalPostResultDisplayEffects(state, invalid)
	if e == nil || failed != state {
		t.Fatal("invalid batch partly committed")
	}
}

func TestOriginalPostResultSurfaceWrapAndInvalid(t *testing.T) {
	s := OriginalRasterSurface{}
	got, e := CommitOriginalPostResultSurface(s, 0x8000, []uint8{7}, false)
	if e == nil || got != s {
		t.Fatal("inactive upload committed")
	}
	s.Mirroring = 2
	got, e = CommitOriginalPostResultSurface(s, 0x2000, []uint8{1}, false)
	if e == nil || got != s {
		t.Fatal("invalid mirror committed")
	}
}
