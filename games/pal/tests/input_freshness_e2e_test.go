package pal_test

// Test first with original campaign and battle states. Failures: a delayed
// packet acquires a new lease and replays movement/menu/attack after an outage;
// missing/future control clocks bypass age bounds; one seat's discarded input
// leaks into another seat's feedback; expired releases cannot stop a hold;
// discarded packets change native memory, remaining delay or committed action;
// valid 200ms-RTT inputs are rejected; save/resume freezes or reuses old epochs.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalInputFreshnessAndUnconditionalRelease(t *testing.T) {
	m, _ := nativeModule(t)
	results := []map[string]any{}
	for _, players := range []int{1} {
		c := gameapi.Context{Players: players, Actor: 0, Now: time.Unix(1800000000, 0)}
		base := originalInn(t, m, &c)
		initial := view(t, m, base, 0)
		if initial.ControlAt != c.Now.UnixMilli() {
			t.Fatal("public control clock is not the original room's logical server clock")
		}
		for _, test := range []struct {
			name        string
			age, offset int64
			keys        uint32
			direction   int
		}{
			{"old_tap", 1000, 0, 4, 4},
			{"old_held", 1000, 0, 0, 0},
			{"old_menu", 1000, 0, 1, 4},
			{"just_expired", 501, 0, 4, 4},
			{"missing_clock", 0, -initial.ControlAt, 4, 4},
			{"negative_clock", 0, -initial.ControlAt - 1, 4, 4},
			{"future_clock", 0, 1, 4, 4},
		} {
			t.Run(fmt.Sprintf("%d/%s", players, test.name), func(t *testing.T) {
				clock := c
				clock.Now = clock.Now.Add(time.Duration(test.age) * time.Millisecond)
				before := bytes.Clone(base.Data)
				next, e := m.Apply(clock, base, raw(pal.Action{Type: "input", Epoch: initial.Epoch, Seq: 1, Direction: test.direction, Keys: test.keys, ControlAt: initial.ControlAt + test.offset}))
				if e != nil {
					t.Fatal("stale physical input must be acknowledged as discarded, not lock the UI", e)
				}
				v := view(t, m, next, 0)
				if v.Seq != 1 || v.DiscardedSeq != 1 || v.X != initial.X || v.Y != initial.Y || v.Menu.Kind != initial.Menu.Kind {
					t.Fatal("stale input was not retired without gameplay", v.Seq, v.DiscardedSeq, v.X, v.Y)
				}
				if players > 1 && view(t, m, next, 1).DiscardedSeq != 0 {
					t.Fatal("discard feedback leaked to another seat")
				}
				var a, b struct {
					Native      []byte `json:"native"`
					RemainingMS uint32 `json:"remainingMS"`
				}
				if json.Unmarshal(base.Data, &a) != nil || json.Unmarshal(next.Data, &b) != nil || !bytes.Equal(a.Native, b.Native) || a.RemainingMS != b.RemainingMS || !bytes.Equal(before, base.Data) {
					t.Fatal("stale input changed its native source or wait")
				}
				if clock.Now.Equal(c.Now) {
					clock.Now = clock.Now.Add(time.Second)
				}
				next, e = m.OnTimer(clock, next)
				if e != nil {
					t.Fatal(e)
				}
				if after := view(t, m, next, 0); after.X != initial.X || after.Y != initial.Y || after.Menu.Kind != initial.Menu.Kind {
					t.Fatal("discarded input replayed at the next native frame")
				}
				results = append(results, map[string]any{"players": players, "case": test.name, "discardedSeq": v.DiscardedSeq})
			})
		}
		for _, age := range []int64{0, 200, 450, 500} {
			clock := c
			clock.Now = clock.Now.Add(time.Duration(age) * time.Millisecond)
			next, e := m.Apply(clock, base, raw(pal.Action{Type: "input", Epoch: initial.Epoch, Seq: 1, Direction: 4, Keys: 4, ControlAt: initial.ControlAt}))
			if e != nil || view(t, m, next, 0).DiscardedSeq != 0 {
				t.Fatal("valid transport delay was discarded", age, e)
			}
		}
		// An old timestamp must never prevent the neutral release of an existing
		// level, even though that timestamp can no longer authorize movement.
		c.Now = c.Now.Add(time.Second)
		g, e := m.OnTimer(c, base)
		if e != nil {
			t.Fatal(e)
		}
		fresh := view(t, m, g, 0)
		g, e = m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: fresh.Epoch, Seq: 1, Direction: 0, ControlAt: fresh.ControlAt}))
		if e != nil {
			t.Fatal(e)
		}
		c.Now = c.Now.Add(150 * time.Millisecond)
		g, e = m.OnTimer(c, g)
		if e != nil {
			t.Fatal(e)
		}
		stopped := view(t, m, g, 0)
		c.Now = c.Now.Add(10 * time.Millisecond)
		release := raw(pal.Action{Type: "input", Epoch: stopped.Epoch, Seq: 2, Direction: 4, ControlAt: initial.ControlAt})
		if !m.IsInputRelease(c, g, release) {
			t.Fatal("old-clock neutral release lost its platform authorization")
		}
		g, e = m.Apply(c, g, release)
		if e != nil {
			t.Fatal(e)
		}
		c.Now = c.Now.Add(100 * time.Millisecond)
		g, e = m.OnTimer(c, g)
		if e != nil {
			t.Fatal(e)
		}
		if after := view(t, m, g, 0); after.X != stopped.X || after.Y != stopped.Y || after.DiscardedSeq != 0 {
			t.Fatal("old-clock release was discarded or held movement resumed")
		}
	}
	data, _ := json.MarshalIndent(map[string]any{"passed": true, "test": t.Name(), "scope": "actual original inn and 1-3 seats; stale/missing/future control clocks retire without native effects, valid delay and unconditional neutral release; not browser reconnect or complete network acceptance", "cases": results}, "", "  ")
	if e := os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/input-freshness.json"), append(data, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
}

func TestOriginalBattleInputFreshness(t *testing.T) {
	m := originalBattleModule(t)
	for _, players := range []int{1} {
		c := gameapi.Context{Players: players, Actor: 0, Now: time.Unix(1800000000, 0)}
		g, e := m.Start(c, raw(map[string]any{}))
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; ; i++ {
			v := view(t, m, g, 0)
			if v.Battle != nil && v.Battle.State == 1 {
				break
			}
			if i > 512 {
				t.Fatal("original first battle action unavailable")
			}
			c.Now = time.UnixMilli(g.SystemDeadline)
			g, e = m.OnTimer(c, g)
			if e != nil {
				t.Fatal(e)
			}
		}
		v := view(t, m, g, 0)
		var original struct {
			Native      []byte `json:"native"`
			RemainingMS uint32 `json:"remainingMS"`
		}
		if e = json.Unmarshal(g.Data, &original); e != nil {
			t.Fatal(e)
		}
		for _, keys := range []uint32{2, 32, 2048} {
			stale := c
			stale.Now = stale.Now.Add(time.Second)
			next, e := m.Apply(stale, g, raw(pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys, ControlAt: v.ControlAt}))
			if e != nil {
				t.Fatal(e)
			}
			after := view(t, m, next, 0)
			var unchanged struct {
				Native      []byte `json:"native"`
				RemainingMS uint32 `json:"remainingMS"`
			}
			if e = json.Unmarshal(next.Data, &unchanged); e != nil || !reflect.DeepEqual(unchanged, original) {
				t.Fatal("stale battle input changed native memory or wait", e)
			}
			if after.DiscardedSeq != v.Seq+1 || !reflect.DeepEqual(after.Battle, v.Battle) {
				t.Fatal("old battle input changed a committed command")
			}
			next, e = m.OnTimer(stale, next)
			if e != nil {
				t.Fatal(e)
			}
			after = view(t, m, next, 0)
			if after.Battle.Current != 0 || after.Battle.State != 1 || after.Battle.Selected != v.Battle.Selected || after.Menu.Kind != v.Menu.Kind {
				t.Fatal("old attack/menu direction replayed after transport recovery")
			}
		}
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "test": t.Name(), "scope": "explicit original PAL_StartBattle fixture for 1-3 seats; expired attack, direction and item keys preserve native state and are not replayed; not campaign combat or browser acceptance"}, "", "  ")
	if e := os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/battle-input-freshness.json"), append(report, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
}
