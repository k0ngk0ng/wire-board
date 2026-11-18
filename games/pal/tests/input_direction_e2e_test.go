package pal_test

// Test first against the actual original campaign and battle core. Failures:
// a press released before the next frame disappears; a short press repeats
// throughout a 250ms timer batch; queued directions reorder or replace one
// another; expired, saved or reclaimed input moves the hero later; a short
// original battle direction never selects its icon; another owner can move.
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func originalInn(t *testing.T, m gameapi.Module, c *gameapi.Context) gameapi.Match {
	t.Helper()
	g, e := m.Start(*c, raw(map[string]any{}))
	if e != nil {
		t.Fatal(e)
	}
	for page := 0; page < 100; page++ {
		g = timers(t, m, c, g)
		v := view(t, m, g, 0)
		if v.Phase == "field" && v.Scene == 2 {
			c.Actor = 0
			return g
		}
		if v.Dialogue == nil {
			t.Fatal("original opening did not reach a reading stop")
		}
		for actor := 0; actor < c.Players; actor++ {
			c.Actor = actor
			g = apply(t, m, *c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
		}
	}
	t.Fatal("original opening did not finish")
	return g
}

func TestOriginalFieldShortDirectionsReleaseAndSave(t *testing.T) {
	m, _ := nativeModule(t)
	results := []map[string]any{}
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			c := gameapi.Context{Players: players, Actor: 0, Now: time.Unix(1800000000, 0)}
			g := originalInn(t, m, &c)
			input := func(direction int, keys uint32) {
				t.Helper()
				v := view(t, m, g, 0)
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
			}
			advance := func(milliseconds int64) pal.View {
				t.Helper()
				c.Now = c.Now.Add(time.Duration(milliseconds) * time.Millisecond)
				var e error
				g, e = m.OnTimer(c, g)
				if e != nil {
					t.Fatal(e)
				}
				return view(t, m, g, 0)
			}
			start := view(t, m, g, 0)
			input(0, 4)
			input(4, 0) // Both arrive before any original frame is executed.
			moved := advance(250)
			if moved.X != start.X-16 || moved.Y != start.Y+8 {
				t.Fatalf("short down press must move exactly one original step: (%d,%d) -> (%d,%d)", start.X, start.Y, moved.X, moved.Y)
			}
			stopped := advance(250)
			if stopped.X != moved.X || stopped.Y != moved.Y {
				t.Fatal("released short direction kept walking")
			}
			// A held south level must not overwrite the older queued north edge.
			input(2, 16)
			input(0, 4)
			first := stopped
			for i := 0; i < 4 && first.X == stopped.X && first.Y == stopped.Y; i++ {
				first = advance(50)
			}
			if first.X != stopped.X+16 || first.Y != stopped.Y-8 {
				t.Fatal("queued north edge lost its original order", first.X, first.Y)
			}
			input(4, 0)
			second := first
			for i := 0; i < 4 && second.X == first.X && second.Y == first.Y; i++ {
				second = advance(50)
			}
			if second.X != stopped.X || second.Y != stopped.Y {
				t.Fatal("second queued direction was collapsed or repeated", second.X, second.Y)
			}
			input(0, 4)
			input(4, 0)
			expired := advance(301)
			if expired.X != second.X || expired.Y != second.Y {
				t.Fatal("expired direction was replayed")
			}
			for _, cleanup := range []string{"save", "reclaim"} {
				input(0, 4)
				input(4, 0)
				var e error
				if cleanup == "save" {
					g, e = m.SaveControl(c, g, true)
					if e == nil {
						c.Now = c.Now.Add(time.Hour)
						g, e = m.ResumeSaved(c, g)
					}
				} else {
					g, e = m.Reclaim(c, g)
				}
				if e != nil {
					t.Fatal(e)
				}
				after := advance(250)
				if after.X != expired.X || after.Y != expired.Y {
					t.Fatal(cleanup, "replayed a pending direction")
				}
			}
			results = append(results, map[string]any{"players": players, "initial": []int{start.X, start.Y}, "oneStep": []int{moved.X, moved.Y}, "orderedDirections": true, "expiredDropped": true, "saveAndReclaimReleased": true})
		})
	}
	if !t.Failed() {
		writeDirectionEvidence(t, "field-short-directions.json", results)
	}
}

func TestOriginalBattleShortDirectionReleasedBeforeFrame(t *testing.T) {
	m := originalBattleModule(t)
	results := []map[string]any{}
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			c := gameapi.Context{Players: players, Actor: 0, Now: time.Unix(1800000000, 0)}
			g, e := m.Start(c, raw(map[string]any{}))
			if e != nil {
				t.Fatal(e)
			}
			for i := 0; i < 256; i++ {
				v := view(t, m, g, 0)
				if v.Battle != nil && v.Battle.State == 1 && v.Battle.Current == 0 {
					break
				}
				c.Now = time.UnixMilli(g.SystemDeadline)
				g, e = m.OnTimer(c, g)
				if e != nil {
					t.Fatal(e)
				}
			}
			v := view(t, m, g, 0)
			if v.Battle == nil || v.Battle.State != 1 {
				t.Fatal("first original battle command unavailable")
			}
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: 1, Direction: 3, Keys: 32})
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: 2, Direction: 4})
			c.Now = c.Now.Add(250 * time.Millisecond)
			g, e = m.OnTimer(c, g)
			if e != nil {
				t.Fatal(e)
			}
			after := view(t, m, g, 0)
			if after.Battle == nil || after.Battle.Selected != 2 || after.Battle.State != 1 || after.Battle.Players[0].State != 1 {
				t.Fatal("released short direction failed to select the original cooperative icon", after.Battle)
			}
			results = append(results, map[string]any{"players": players, "selected": after.Battle.Selected, "noCommandSubmitted": true})
		})
	}
	if !t.Failed() {
		writeDirectionEvidence(t, "battle-short-directions.json", results)
	}
}

func writeDirectionEvidence(t *testing.T, name string, cases []map[string]any) {
	t.Helper()
	data, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": true, "scope": "actual original core and multiplayer input adapter; input edges arrive and release before the next native frame, not complete campaign", "cases": cases}, "", "  ")
	if e := os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification", name), append(data, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
}
