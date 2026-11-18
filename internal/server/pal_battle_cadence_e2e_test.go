package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALBattleCadenceRetainsOriginalFramesAndSavedDelay(t *testing.T) {
	if os.Getenv("PAL_CADENCE_E2E") != "1" {
		t.Skip("explicit original battle cadence regression")
	}
	m := palBoardModuleAt(t, ".local/pal/screen-input/battle-current/rules.wasm")
	c := gameapi.Context{Players: 1, Actor: 0, Version: 1, Now: time.Now()}
	match, err := m.Start(c, nil)
	if err != nil {
		t.Fatal(err)
	}
	view := func(match gameapi.Match) pal.View {
		t.Helper()
		value, err := m.View(match, 0)
		if err != nil {
			t.Fatal(err)
		}
		return value.(pal.View)
	}
	tick := func(at int64) {
		t.Helper()
		c.Now = time.UnixMilli(at)
		c.Actor = -1
		match, err = m.OnTimer(c, match)
		if err != nil {
			t.Fatal(err)
		}
	}
	arrow := func(view pal.View) int {
		if view.Phase != "battle" || view.Battle == nil || view.Battle.Current != 0 || view.Battle.State != 1 || view.Battle.Menu != 0 || view.Raster == nil {
			return 0
		}
		for i := len(view.Raster.Nodes) - 1; i >= 0; i-- {
			n := view.Raster.Nodes[i]
			if n[0] == 1 && n[5] == 2 && n[6] == 0 && (n[7] == 68 || n[7] == 69) && n[8] == 172 && n[9] == 106 {
				return n[7]
			}
		}
		return 0
	}
	for step := 0; step < 600 && arrow(view(match)) == 0; step++ {
		if match.SystemDeadline <= c.Now.UnixMilli() {
			t.Fatal("native battle startup clock did not advance")
		}
		tick(match.SystemDeadline)
	}
	initial := view(match)
	lastArrow := arrow(initial)
	if lastArrow == 0 || len(initial.Party) != 3 {
		t.Fatal("original battle did not reach its visible arrow")
	}
	start := match.SystemDeadline
	frames := make([]map[string]int64, 0, 25)
	late := []int64{0, 6, 1, 4}
	for i := range 25 {
		due := match.SystemDeadline
		tick(due + late[i%len(late)])
		next := view(match)
		frame := arrow(next)
		if frame == 0 || frame == lastArrow || match.SystemDeadline-due != 40 || !reflect.DeepEqual(next.Party, initial.Party) || next.Cash != initial.Cash {
			t.Fatalf("original arrow cadence changed: at=%d frame=%d previous=%d next=%d", due, frame, lastArrow, match.SystemDeadline)
		}
		lastArrow = frame
		frames = append(frames, map[string]int64{"dueMS": due, "atMS": c.Now.UnixMilli(), "nextMS": match.SystemDeadline, "arrow": int64(frame)})
	}
	if match.SystemDeadline-start != 1000 {
		t.Fatal("battle frames accumulated scheduling drift")
	}
	before := view(match)
	c.Actor = 0
	c.Now = time.UnixMilli(match.SystemDeadline - 17)
	frozen, err := m.SaveControl(c, match, true)
	if err != nil || !frozen.Paused || frozen.SystemDeadline != 0 {
		t.Fatal("battle save did not freeze deadline", err)
	}
	nextModule := palBoardModuleAt(t, ".local/pal/screen-input/battle-current/rules.wasm")
	c.Now = c.Now.Add(24 * time.Hour)
	resumed, err := nextModule.ResumeSaved(c, frozen)
	if err != nil {
		t.Fatal(err)
	}
	after := view(resumed)
	if resumed.SystemDeadline-c.Now.UnixMilli() != 17 || arrow(after) != arrow(before) || after.Epoch == before.Epoch || !reflect.DeepEqual(after.Party, before.Party) {
		t.Fatal("battle resume did not preserve arrow and remaining wait")
	}
	raw, err := json.MarshalIndent(map[string]any{"passed": true, "originalFrames": frames, "nativeElapsedMS": 1000, "offlineHours": 24, "resumedRemainingMS": 17, "oldEpoch": before.Epoch, "newEpoch": after.Epoch, "scope": "original three-member battle, normal difficulty, native arrow frames, deterministic server scheduler jitter and fresh runtime save continuation"}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	evidence := "continuous-battle-rule.json"
	if os.Getenv("PAL_CADENCE_SCOPE") == "mixed" {
		evidence = "mixed-room-cadence.json"
	}
	if err = os.WriteFile(filepath.Join("..", "..", ".local/pal/verification", evidence), append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
