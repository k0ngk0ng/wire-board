package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalScoreDifferencePhase(t *testing.T) {
	data, err := os.ReadFile("testdata/score-difference.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Frame      int
		Scores     [2]uint16
		Difference uint8
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("missing original score-difference calls")
	}
	for _, r := range rows {
		if got := OriginalScoreDifference(r.Scores); got != r.Difference {
			t.Fatalf("frame %d scores %v got %x want %x", r.Frame, r.Scores, got, r.Difference)
		}
	}
	t.Logf("matched %d original score-difference phase calls", len(rows))
}

func TestOriginalScoreDifferenceThresholdsAndBorrow(t *testing.T) {
	for _, tc := range []struct {
		scores [2]uint16
		want   uint8
	}{
		{[2]uint16{0, 0}, 0}, {[2]uint16{1000, 1000}, 0},
		{[2]uint16{1, 0}, 1}, {[2]uint16{3, 0}, 1}, {[2]uint16{4, 0}, 2}, {[2]uint16{15, 0}, 2}, {[2]uint16{16, 0}, 3},
		{[2]uint16{0, 1}, 129}, {[2]uint16{0, 3}, 129}, {[2]uint16{0, 4}, 130}, {[2]uint16{0, 15}, 130}, {[2]uint16{0, 16}, 131},
		{[2]uint16{256, 255}, 1}, {[2]uint16{255, 256}, 129}, {[2]uint16{256, 0}, 3},
		{[2]uint16{32768, 0}, 131}, {[2]uint16{65535, 0}, 129},
	} {
		if got := OriginalScoreDifference(tc.scores); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}
