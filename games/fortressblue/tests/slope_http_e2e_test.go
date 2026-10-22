package tests

// HTTP failure scenarios: the default room still grants 30 seconds; a selected
// tank on a real incline shoots as if flat; reversing direction flips the wrong
// component; SQLite restoration recalculates an already committed flight.
import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	fb "github.com/k0ngk0ng/wire-board/games/fortressblue/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestSlopeLaunchAndMinuteDefaultThroughHTTP(t *testing.T) {
	for _, facing := range []int{-1, 1} {
		t.Run(fmt.Sprint(facing), func(t *testing.T) {
			module := fb.Module()
			apply := module.Apply
			module.Apply = func(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
				var before fb.State
				_ = json.Unmarshal(m.Data, &before)
				next, err := apply(ctx, m, raw)
				if err != nil {
					return next, err
				}
				var after fb.State
				_ = json.Unmarshal(next.Data, &after)
				if before.Phase == fb.PhaseSelect && after.Phase == fb.PhasePlaying {
					// Labelled opening only: existing stage-16 collision terrain has
					// wheel supports (196,788), (220,796), no synthetic ground.
					after.Tanks[0].X, after.Tanks[0].Y = 208, 792
					after.Wind = 0
					next.Data, _ = json.Marshal(after)
				}
				return next, nil
			}
			h := gametest.NewHTTP(t, 2, module, "fortressblue")
			h.Start(map[string]any{"map": "stage-16"})
			s := selectAll(t, h)
			if s.Options.TurnSeconds != 60 || s.Deadline-time.Now().UnixMilli() < 59000 {
				t.Fatal("new room does not grant a minute", s.Options, s.Deadline)
			}
			h.People[0].Action(map[string]any{"type": "fire", "epoch": s.Epoch, "turn": s.Turn, "angle": 45, "power": 60, "facing": facing, "weapon": "primary"}, 200)
			shot := state(h.People[0]).LastShot
			p, q := shot.Arcs[0].Path[0], shot.Arcs[0].Path[1]
			angle := math.Atan2(-(q.Y-p.Y), (q.X-p.X)*float64(facing)) * 180 / math.Pi
			want := 45 - float64(facing)*math.Atan2(8, 24)*180/math.Pi
			if math.Abs(angle-want) > 1 {
				t.Fatalf("world angle=%f want=%f", angle, want)
			}
			h.Restart()
			restored := state(h.People[0]).LastShot
			if !reflect.DeepEqual(shot.Arcs, restored.Arcs) {
				t.Fatal("SQLite restore rewrote committed paths")
			}
			t.Logf("real HTTP shot facing=%d angle=%.3f expected=%.3f deadline=60s; restore preserved path", facing, angle, want)
			h.Record("sloped shot, full-minute default and SQLite path restore")
		})
	}
}
