package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

// Failure scenarios precede the implementation in CHU_WORLD_START_CELLS_VERIFICATION.md.
// These fixtures contain original FE68 before/after cells, not Go-generated expectations.
type startCellsOriginal struct {
	Source        uint32
	Offset        uint16
	Words         []uint16
	Before, After string
}

func readStartCellsOriginal(t *testing.T) (natural, isolated []startCellsOriginal) {
	t.Helper()
	zipped, err := os.ReadFile("testdata/match-start-cells.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile("testdata/match-start-cells-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Pass     bool
		Baseline bool   `json:"frames_match_original_core"`
		Natural  int    `json:"natural_calls"`
		Isolated int    `json:"isolated_calls"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
		Library  string `json:"library_sha256"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	libraryBytes, err := os.ReadFile("testdata/match-start-library.json")
	if err != nil {
		t.Fatal(err)
	}
	sha := func(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
	if !manifest.Pass || !manifest.Baseline || manifest.Natural != 6 || manifest.Isolated != 121 || manifest.SHA != sha(raw) || manifest.Gzip != sha(zipped) || manifest.Library != sha(libraryBytes) {
		t.Fatal("unverified FE68 original evidence")
	}
	var fixture struct{ Natural, Isolated []startCellsOriginal }
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Natural) != manifest.Natural || len(fixture.Isolated) != manifest.Isolated {
		t.Fatal("incomplete FE68 records")
	}
	return fixture.Natural, fixture.Isolated
}

func startCellsArray(t *testing.T, encoded string) (out [768]uint16) {
	t.Helper()
	b, err := hex.DecodeString(encoded)
	if err != nil || len(b) != 1536 {
		t.Fatal("invalid full original shared cells", err, len(b))
	}
	for i := range out {
		out[i] = binary.BigEndian.Uint16(b[2*i:])
	}
	return out
}

func TestChuWorldStartCellsOriginal(t *testing.T) {
	natural, isolated := readStartCellsOriginal(t)
	for index, row := range append(natural, isolated...) {
		before, want := startCellsArray(t, row.Before), startCellsArray(t, row.After)
		got, err := matchStartCells(before, []MatchStartEvent{{Kind: "words", Source: row.Source, Offset: row.Offset, Words: row.Words}})
		if err != nil || got != want {
			t.Fatalf("original FE68 row %d offset %x differs: %v", index, row.Offset, err)
		}
	}
}

func TestChuWorldStartCellsBoundaries(t *testing.T) {
	var initial [768]uint16
	for i := range initial {
		initial[i] = uint16(0x5100 + i)
	}
	for _, offset := range []uint16{1, 1535, 1536, 65534} {
		got, err := matchStartCells(initial, []MatchStartEvent{
			{Kind: "words", Offset: 0, Words: []uint16{17, 18}},
			{Kind: "words", Offset: offset, Words: []uint16{27, 28}},
		})
		if err == nil || got != initial {
			t.Fatalf("late malformed words offset %x was not atomic", offset)
		}
	}
	got, err := matchStartCells(initial, []MatchStartEvent{
		{Kind: "words", Offset: 0, Words: []uint16{17, 18}},
		{Kind: "name", Offset: 1},
		{Kind: "portrait", Offset: 65535},
		{Kind: "words", Offset: 2, Words: []uint16{27}},
		{Kind: "words", Offset: 1534, Words: []uint16{37}},
	})
	want := initial
	want[0], want[1], want[767] = 17, 27, 37
	if err != nil || got != want {
		t.Fatal("ordered/shared-cell edge writes differ", err)
	}
}

func TestChuWorldStartCellsManagerIntegration(t *testing.T) {
	library := sessionLibrary(t)
	for _, computers := range []bool{false, true} {
		for count := 2; count <= 4; count++ {
			options := ChuSessionOptions{Seed: 0x12345678, Difficulty: 6}
			for slot := 0; slot < count; slot++ {
				options.Players = append(options.Players, ChuSessionPlayer{Slot: uint8(slot), CharacterID: 9, Color: uint8(slot), Computer: computers, HumanChannel: uint8(slot)})
			}
			state, _, err := NewChuSession(options, library)
			if err != nil {
				t.Fatal(err)
			}
			if state.Cells != [768]uint16{} {
				t.Fatal("waiting manager wrote HUD templates before resources were ready")
			}
			state, err = CompleteChuSessionResource(state, DefaultChuSessionResource())
			if err != nil {
				t.Fatal(err)
			}
			state, events, _, err := StepChuSessionFrame(state, [8]Buttons{}, library)
			if err != nil {
				t.Fatal(err)
			}
			words := 0
			for _, event := range events {
				if event.Kind != "start" || event.Start.Kind != "words" {
					continue
				}
				words++
				offset := int(event.Start.Offset) / 2
				// Later actor HUD writes replace the interior; borders and controller labels remain.
				if state.Cells[offset] != event.Start.Words[0] || state.Cells[offset+len(event.Start.Words)-1] != event.Start.Words[len(event.Start.Words)-1] {
					t.Fatalf("fresh %d-player manager did not materialize template borders at E600+%x", count, event.Start.Offset)
				}
			}
			if words != count*3 {
				t.Fatalf("fresh manager words count %d != %d", words, count*3)
			}
		}
	}

	state, _, err := NewChuSession(sessionOptions(true), library)
	if err != nil {
		t.Fatal(err)
	}
	state, err = CompleteChuSessionResource(state, DefaultChuSessionResource())
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range []uint16{1, 1534, 65534} {
		bad := library
		bad.MatchStart.HUDOffsets[1][1] = offset // First player succeeds; second must roll back the full pass.
		got, events, _, err := StepChuSessionFrame(state, [8]Buttons{}, bad)
		if err == nil || len(events) != 0 || !reflect.DeepEqual(got, state) {
			t.Fatalf("invalid manager HUD offset %x leaked a partial world", offset)
		}
	}
	paused := state
	paused.Clock.PauseFlags = 0x80
	before := paused.Cells
	manager := paused.Objects[0x8040]
	events, err := stepChuWorldStart(&paused, manager, library.MatchStart)
	if err != nil || len(events) != 0 || paused.Cells != before {
		t.Fatal("paused manager changed cells", err)
	}
}
