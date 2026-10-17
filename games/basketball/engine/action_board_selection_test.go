package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestWorldActionsOriginalBoardSelection(t *testing.T) {
	data, err := os.ReadFile("testdata/action-board-selection.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Frame         int
		Before, After uint8
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	worlds := worldActionFixtures(t)
	if len(rows) != len(worlds) {
		t.Fatal("missing action selector observations")
	}
	writes := 0
	for i, row := range rows {
		if row.Frame != worlds[i].Frame {
			t.Fatal("unaligned original phase")
		}
		step, err := StepWorldActions(worlds[i].Before, worlds[i].context(t))
		if err != nil {
			t.Fatal(err)
		}
		selected := row.Before
		if step.ChangeSelectedBoard {
			selected = step.SelectedBoard
			writes++
		}
		if selected != row.After {
			t.Fatalf("frame %d selected board %d want %d", row.Frame, selected, row.After)
		}
	}
	if writes == 0 {
		t.Fatal("no falling-hoop selector writes exercised")
	}
	t.Logf("matched %d original phases, %d explicit board-selector writes", len(rows), writes)
}

func TestHoopBoardSelectionWritesOnlyAtFallingEntry(t *testing.T) {
	for _, tc := range []struct {
		x, height Fixed
		want      uint8
	}{
		{65 * Pixel, 80 * Pixel, 0}, {463 * Pixel, 80 * Pixel, 1},
		{65 * Pixel, 176 * Pixel, 2}, {463 * Pixel, 272 * Pixel, 5},
	} {
		h := Hoop{Action: HoopFalling, Position: Position{X: tc.x, Depth: 43 * Pixel, Height: tc.height}}
		step, err := StepHoopAction(h, HoopContext{}, NewYorkGroundCourt())
		if err != nil || !step.ChangeSelectedBoard || step.SelectedBoard != tc.want {
			t.Fatal("falling entry selector", step, err)
		}
		next, err := StepHoopAction(step.Hoop, HoopContext{}, NewYorkGroundCourt())
		if err != nil || next.ChangeSelectedBoard {
			t.Fatal("repeated selector write", next, err)
		}
	}
}
