package racing

// Failure cases recorded before the sequence exhaustion repair: a single
// accepted JS-safe maximum blocks every atomic bot frame; bad saved sequences
// block whole-service startup; repair silently resets normal cars, clocks,
// lap progress or epoch; old-epoch requests remain accepted after repair.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"reflect"
	"testing"
	"time"
)

func TestSequenceJumpAndIntegerLimits(t *testing.T) {
	mod := Module()
	ctx := gameapi.Context{Players: 2, Actor: 0, Now: time.Unix(100000, 0)}
	m, e := mod.Start(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := decode(m)
	for _, seq := range []int64{1025, 2147483648, 9007199254740991} {
		raw, _ := json.Marshal(Action{Type: "drive", Epoch: s.Epoch, Seq: seq, Throttle: 1})
		if mod.AcceptStale(ctx, m, raw) {
			t.Fatal("accept stale admitted excessive sequence", seq)
		}
		if next, e := mod.Apply(ctx, m, raw); e == nil || !reflect.DeepEqual(next, m) {
			t.Fatal("excessive sequence accepted or changed snapshot", seq, e)
		}
	}
	raw, _ := json.Marshal(Action{Type: "drive", Epoch: s.Epoch, Seq: 1024, Throttle: 1})
	if _, e = mod.Apply(ctx, m, raw); e != nil {
		t.Fatal("bounded network gap rejected", e)
	}
}
func TestExhaustedSequenceCannotFreezeTimerOrStartup(t *testing.T) {
	for _, seq := range []int64{-1, 2147483647, 9007199254740991} {
		t.Run(time.Unix(seq, 0).Format("20060102T150405"), func(t *testing.T) {
			mod := Module()
			ctx := gameapi.Context{Players: 2, Actor: 0, Now: time.Unix(100000, 0)}
			m, e := mod.Start(ctx, nil)
			if e != nil {
				t.Fatal(e)
			}
			s, _ := decode(m)
			s.Cars[0].Seq = seq
			s.Cars[0].Throttle = 1
			s.Cars[0].Steer = .5
			s.Cars[0].Pending = []Input{{At: s.ServerAt, Throttle: 1, Steer: .5}}
			s.Cars[0].Applied = Input{At: s.ServerAt, Throttle: 1, Steer: .5}
			m, e = s.match()
			if e != nil {
				t.Fatal(e)
			}
			action, e := mod.BotAction(ctx, m)
			if e != nil || action != nil {
				t.Fatal("bad sequence produced action that would roll back timer frame", string(action), e)
			}
			restored, e := mod.Restore(ctx, m)
			if e != nil {
				t.Fatal("one bad room would block startup", e)
			}
			r, _ := decode(restored)
			if r.Epoch == s.Epoch || r.Cars[0].Seq != 0 || r.Cars[0].Throttle != 0 || len(r.Cars[0].Pending) != 0 || !r.Cars[0].Brake {
				t.Fatal("repair did not rotate token and neutralize only bad input")
			}
			if r.Cars[0].X != s.Cars[0].X || r.Cars[0].Progress != s.Cars[0].Progress || r.StartAt != s.StartAt || r.FinishDeadline != s.FinishDeadline || r.MaxRaceAt != s.MaxRaceAt || r.Cars[0].IdleAt != s.Cars[0].IdleAt || r.Cars[0].LastInputAt != s.Cars[0].LastInputAt || !reflect.DeepEqual(r.Cars[1], s.Cars[1]) {
				t.Fatal("repair reset position, progress, unrelated seat or deadlines")
			}
			if restored.Deadline != m.Deadline || restored.SystemDeadline != m.SystemDeadline {
				t.Fatal("repair changed host timers")
			}
			again, e := mod.Restore(ctx, restored)
			if e != nil || !reflect.DeepEqual(again, restored) {
				t.Fatal("normal restore changed epoch or data")
			}
			old, _ := json.Marshal(Action{Type: "drive", Epoch: s.Epoch, Seq: 1, Throttle: 1})
			if mod.AcceptStale(ctx, restored, old) {
				t.Fatal("old-epoch request accepted after recovery")
			}
			ctx.Actor = -1
			ctx.Now = ctx.Now.Add(100 * time.Millisecond)
			ticked, e := mod.OnTimer(ctx, m)
			if e != nil {
				t.Fatal(e)
			}
			after, _ := decode(ticked)
			if after.ServerAt <= s.ServerAt || after.Cars[0].Seq != 0 || after.Epoch == s.Epoch {
				t.Fatal("live old snapshot still froze frame")
			}
			ctx.Actor = 0
			action, e = mod.BotAction(ctx, ticked)
			if e != nil || action == nil {
				t.Fatal("repaired bot cannot resume", e)
			}
			if _, e = mod.Apply(ctx, ticked, action); e != nil {
				t.Fatal("repaired bot action rejected", e)
			}
		})
	}
}
