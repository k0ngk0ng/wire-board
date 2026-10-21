package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Failure inventory and controlled source-call boundaries precede this file in
// POSTRESULT_DISPLAY_SOURCE.md. These tests precede all Go helper code.
func TestOriginalPostResultRasterProgramSource(t *testing.T) {
	file, err := os.Open("testdata/postresult-display-program.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	packed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()
	var cases []struct {
		Context OriginalPostResultRasterSource `json:"context"`
		NMI     OriginalRasterHandler          `json:"nmi"`
		Stages  []OriginalRasterHandler        `json:"stages"`
	}
	if err := json.NewDecoder(packed).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 78 {
		t.Fatalf("source case count %d", len(cases))
	}
	calls := 0
	for i, c := range cases {
		before := c.Context
		got, err := BuildOriginalPostResultRasterProgram(c.Context)
		if err != nil {
			t.Fatalf("case%d: %v", i, err)
		}
		if !reflect.DeepEqual(got.NMI, c.NMI) {
			t.Fatalf("case%d NMI got%+v want%+v", i, got.NMI, c.NMI)
		}
		if !reflect.DeepEqual(got.Stages, c.Stages) {
			t.Fatalf("case%d IRQ got%+v want%+v", i, got.Stages, c.Stages)
		}
		if c.Context != before {
			t.Fatal("program modified source context")
		}
		data, err := json.Marshal(struct {
			Source  OriginalPostResultRasterSource
			Program OriginalPostResultRasterProgram
		}{c.Context, got})
		if err != nil {
			t.Fatal(err)
		}
		var restored struct {
			Source  OriginalPostResultRasterSource
			Program OriginalPostResultRasterProgram
		}
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		replay, err := BuildOriginalPostResultRasterProgram(restored.Source)
		if err != nil || !reflect.DeepEqual(replay, restored.Program) {
			t.Fatalf("JSON replay case%d: %v", i, err)
		}
		calls += len(got.Stages)
	}
	if calls != 228 {
		t.Fatalf("source IRQ count%d", calls)
	}
}

func TestOriginalPostResultRasterProgramRejectsUnrestored(t *testing.T) {
	base := OriginalPostResultRasterSource{Scene: 7, MachineControl: 128, BackgroundCHR: [2]uint8{60, 62}, SpriteCHR: [4]uint8{37, 38, 11, 40}, Control: 136, Mask: 30}
	bad := []OriginalPostResultRasterSource{base, base, base, base, base}
	bad[0].Scene = 0
	bad[1].Phase = 12
	bad[2].MachineControl = 192
	bad[3].Scene = 1
	bad[3].MachineControl = 192
	bad[3].Phase = 6
	bad[4].SpriteCHR[0] = 128
	for i, c := range bad {
		if _, err := BuildOriginalPostResultRasterProgram(c); err == nil {
			t.Fatalf("invalid case%d accepted", i)
		}
	}
}

func TestOriginalPostResultRasterLiveIRQSource(t *testing.T) {
	source := OriginalPostResultRasterSource{Scene: 7, Phase: 8, MachineControl: 128, BackgroundCHR: [2]uint8{12, 12}, SpriteCHR: [4]uint8{56, 57, 11, 40}, Control: 136, Mask: 30, Credits: [4]uint8{184, 92, 46, 92}}
	nmi, err := BuildOriginalPostResultRasterNMI(source)
	if err != nil || nmi.Stage != 15 {
		t.Fatalf("NMI: %+v %v", nmi, err)
	}
	first, err := BuildOriginalPostResultRasterIRQ(15, nmi.Control, source)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildOriginalPostResultRasterIRQ(16, first.Control, source)
	if err != nil {
		t.Fatal(err)
	}
	source.Credits = [4]uint8{182, 91, 45, 91}
	third, err := BuildOriginalPostResultRasterIRQ(17, second.Control, source)
	if err != nil {
		t.Fatal(err)
	}
	fourth, err := BuildOriginalPostResultRasterIRQ(18, third.Control, source)
	if err != nil {
		t.Fatal(err)
	}
	for i, h := range []OriginalRasterHandler{first, second, third, fourth} {
		wanted := []uint16{184, 92, 45, 91}[i]
		found := false
		for _, effect := range h.Effects {
			if effect.Kind == OriginalRasterScrollX {
				found = true
				if effect.Value != wanted {
					t.Fatalf("live IRQ%d got%d want%d", i, effect.Value, wanted)
				}
			}
		}
		if !found {
			t.Fatalf("missing live offset%d", i)
		}
	}
}

func TestOriginalPostResultRasterRewardLivePhaseFont(t *testing.T) {
	// F35C has phase6 page14; other admitted phase0..8 entries select98.
	source := OriginalPostResultRasterSource{Phase: 6}
	got, err := BuildOriginalPostResultRasterIRQ(0, 136, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Effects) != 5 || got.Effects[1].Kind != OriginalRasterCHR1 || got.Effects[1].Value != 14 {
		t.Fatalf("result font: %+v", got)
	}
	source.Phase = 7
	got, err = BuildOriginalPostResultRasterIRQ(0, 136, source)
	if err != nil || got.Effects[1].Value != 98 {
		t.Fatalf("reward font: %+v %v", got, err)
	}
	source.Phase = 9
	if _, err = BuildOriginalPostResultRasterIRQ(0, 136, source); err == nil {
		t.Fatal("unrestored font table index accepted")
	}
}
