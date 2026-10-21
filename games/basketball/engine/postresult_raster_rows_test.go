package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Declared failures: wrong physical mirroring or CHR mapping; atomic scroll
// substitution; losing two fetched rows; vertical row29/31 wrapping; wrong
// odd-frame origin; missing/overlapping output columns; implicit timing claims;
// mutated committed resources; JSON replay divergence. Written before helper.
func TestOriginalPostResultRasterRowsSourcePixels(t *testing.T) {
	f, err := os.Open("testdata/postresult-raster-rows.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var cases []struct {
		Kind    string
		Frame   int
		Surface OriginalRasterSurface
		Plan    OriginalRasterFramePlan
		Want    OriginalRasterFrameRows
	}
	if err = json.NewDecoder(z).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 16 {
		t.Fatalf("source frames%d", len(cases))
	}
	for _, c := range cases {
		before := c.Surface
		got, err := ProjectOriginalPostResultRasterRows(c.Surface, c.Plan)
		if err != nil {
			t.Fatalf("%s%d: %v", c.Kind, c.Frame, err)
		}
		if c.Surface != before {
			t.Fatal("projection changed committed surface")
		}
		if !reflect.DeepEqual(got, c.Want) {
			for y := 0; y < 240; y++ {
				if !reflect.DeepEqual(got.Rows[y], c.Want.Rows[y]) {
					t.Fatalf("%s%d row%d got%+v want%+v", c.Kind, c.Frame, y, got.Rows[y], c.Want.Rows[y])
				}
			}
			t.Fatalf("%s%d palette mismatch", c.Kind, c.Frame)
		}
		raw, err := json.Marshal(struct {
			Surface OriginalRasterSurface
			Plan    OriginalRasterFramePlan
		}{c.Surface, c.Plan})
		if err != nil {
			t.Fatal(err)
		}
		var restored struct {
			Surface OriginalRasterSurface
			Plan    OriginalRasterFramePlan
		}
		if err = json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		replay, err := ProjectOriginalPostResultRasterRows(restored.Surface, restored.Plan)
		if err != nil || !reflect.DeepEqual(got, replay) {
			t.Fatalf("%s%d JSON replay %v", c.Kind, c.Frame, err)
		}
	}
}

func TestOriginalPostResultRasterRowsRejectAmbiguousPlans(t *testing.T) {
	surface := OriginalRasterSurface{}
	source := OriginalPostResultRasterSource{Scene: 7, Phase: 9, MachineControl: 128, Control: 136, Mask: 30, BackgroundCHR: [2]uint8{20, 22}, SpriteCHR: [4]uint8{56, 57, 11, 40}}
	plan := OriginalRasterFramePlan{Source: source, TimingMode: OriginalRasterCanonical, TimingSource: "explicit canonical test"}
	if _, err := ProjectOriginalPostResultRasterRows(surface, plan); err != nil {
		t.Fatal(err)
	}
	bad := []OriginalRasterFramePlan{plan, plan, plan, plan, plan}
	bad[0].TimingMode = ""
	bad[1].TimingSource = ""
	bad[2].Effects = []OriginalRasterTimedEffect{{Dot: 1, Kind: "unknown"}}
	bad[3].Effects = []OriginalRasterTimedEffect{{Dot: 10, Kind: OriginalRasterScrollX}, {Dot: 9, Kind: OriginalRasterScrollX}}
	bad[4].Source.SpriteCHR[0] = 128
	for i, p := range bad {
		if _, err := ProjectOriginalPostResultRasterRows(surface, p); err == nil {
			t.Fatalf("invalid plan%d accepted", i)
		}
	}
	surface.Mirroring = 2
	if _, err := ProjectOriginalPostResultRasterRows(surface, plan); err == nil {
		t.Fatal("invalid mirroring accepted")
	}
}
