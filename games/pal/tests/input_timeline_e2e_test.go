package pal_test

// Test first using actual original GameMain and PAL_StartBattle. Failures:
// late press backfills older native frames; late release erases valid earlier
// movement; a lease is applied past expiry or renewed into its expired gap;
// short edges are applied before arrival; received sequence is falsely called
// applied; a capped host backlog replays new input through old time. Batched
// execution must match a paced reference's native state and remaining budget.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type timelineRun struct {
	t      *testing.T
	m      gameapi.Module
	c      gameapi.Context
	g      gameapi.Match
	anchor time.Time
}

// Additional native lifecycle failures: unbounded held-level history; a full
// queue rejects the release needed to stop; overload permanently disables
// subsequent movement; old optional control fields make matching-build saved
// rooms unreadable or replay their stale keys after Restore.
func TestOriginalInputTimelineOverflowReleaseAndLegacyRestore(t *testing.T) {
	m, _ := nativeModule(t)
	c := gameapi.Context{Players: 1, Actor: 0, Now: time.Unix(1800000000, 0)}
	base := originalInn(t, m, &c)
	run := timelineRun{t, m, c, base, c.Now}
	for i := 0; i < 64; i++ {
		run.input(10, i%2*2, 0)
	}
	v := view(t, m, run.g, 0)
	before := bytes.Clone(run.g.Data)
	if _, e := m.Apply(run.c, run.g, raw(pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 0})); e == nil || !bytes.Equal(before, run.g.Data) {
		t.Fatal("held-input overload was accepted or mutated its source")
	}
	run.input(20, 4, 0)
	for at := 21; at <= 50; at++ {
		run.input(at, 4, 0)
	}
	run.tick(250)
	initial, stopped := view(t, m, base, 0), view(t, m, run.g, 0)
	if stopped.X != initial.X || stopped.Y != initial.Y || stopped.AppliedSeq != stopped.Seq {
		t.Fatal("a full queue prevented a timely release", stopped.X, stopped.Y, stopped.Seq, stopped.AppliedSeq)
	}
	run.input(260, 0, 0)
	run.tick(300)
	run.tick(350)
	moved := view(t, m, run.g, 0)
	if moved.X != initial.X-16 || moved.Y != initial.Y+8 {
		t.Fatal("input did not recover after its bounded history drained", moved.X, moved.Y)
	}
	run.input(360, 4, 0)
	run.tick(400)
	run.tick(450)
	if after := view(t, m, run.g, 0); after.X != moved.X || after.Y != moved.Y {
		t.Fatal("recovered held direction did not release")
	}
	var legacy map[string]any
	if e := json.Unmarshal(base.Data, &legacy); e != nil {
		t.Fatal(e)
	}
	control := legacy["controls"].([]any)[0].(map[string]any)
	control["seq"], control["direction"], control["until"] = 1, 0, c.Now.UnixMilli()+300
	control["edges"] = []any{map[string]any{"seq": 1, "keys": 4, "until": c.Now.UnixMilli() + 300}}
	delete(control, "levels")
	old := base
	old.Data = raw(legacy)
	c.Now = c.Now.Add(time.Hour)
	restored, e := m.Restore(c, old)
	if e != nil {
		t.Fatal("matching-build legacy optional controls could not restore", e)
	}
	c.Now = c.Now.Add(250 * time.Millisecond)
	restored, e = m.OnTimer(c, restored)
	if e != nil {
		t.Fatal(e)
	}
	if after := view(t, m, restored, 0); after.X != initial.X || after.Y != initial.Y || after.Epoch == initial.Epoch {
		t.Fatal("legacy held/edge input survived Restore")
	}
	data, _ := json.MarshalIndent(map[string]any{"passed": true, "test": t.Name(), "scope": "actual original inn, single seat, bounded input overload and reserved release, subsequent movement, matching-native-build legacy JSON control restore without stale replay; not cross-build save migration", "boundedLevels": 65, "legacyFieldsOptional": true}, "", "  ")
	if e = os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/input-timeline-lifecycle.json"), append(data, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
}

func (r *timelineRun) input(at int, direction int, keys uint32) {
	r.t.Helper()
	r.c.Now = r.anchor.Add(time.Duration(at) * time.Millisecond)
	v := view(r.t, r.m, r.g, 0)
	r.g = apply(r.t, r.m, r.c, r.g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
}
func (r *timelineRun) tick(at int) {
	r.t.Helper()
	r.c.Now = r.anchor.Add(time.Duration(at) * time.Millisecond)
	var e error
	r.g, e = r.m.OnTimer(r.c, r.g)
	if e != nil {
		r.t.Fatal(e)
	}
}
func nativeTiming(t *testing.T, runtime *engine.Runtime, g gameapi.Match) (engine.Snapshot, uint32) {
	t.Helper()
	var saved struct {
		Native      []byte `json:"native"`
		RemainingMS uint32 `json:"remainingMS"`
	}
	if e := json.Unmarshal(g.Data, &saved); e != nil {
		t.Fatal(e)
	}
	s, e := runtime.Decode(saved.Native)
	if e != nil {
		t.Fatal(e)
	}
	return s, saved.RemainingMS
}

func TestOriginalFieldInputArrivalTimeline(t *testing.T) {
	m, runtime := nativeModule(t)
	results := []map[string]any{}
	for _, players := range []int{1} {
		c := gameapi.Context{Players: players, Actor: 0, Now: time.Unix(1800000000, 0)}
		base := originalInn(t, m, &c)
		for _, name := range []string{"late_press", "late_release", "lease_expiry", "renew_after_gap", "short_edge_arrival"} {
			t.Run(fmt.Sprintf("%d/%s", players, name), func(t *testing.T) {
				paced := timelineRun{t, m, c, base, c.Now}
				batch := paced
				if name == "short_edge_arrival" {
					_, remaining := nativeTiming(t, runtime, base)
					at := int(remaining) + 105
					for tick := 50; tick < at; tick += 50 {
						paced.tick(tick)
					}
					paced.input(at, 4, 4)
					batch.input(at, 4, 4)
					batch.tick(at + 5)
					before := view(t, m, base, 0)
					after := view(t, m, batch.g, 0)
					if after.X != before.X || after.Y != before.Y || after.AppliedSeq != 0 || after.Seq != 1 {
						t.Fatal("a short edge was applied before the first eligible original frame", before.X, before.Y, after.X, after.Y, after.Seq, after.AppliedSeq)
					}
					results = append(results, map[string]any{"players": players, "case": name, "receivedSeq": after.Seq, "appliedSeq": after.AppliedSeq})
					return
				}
				if name == "late_press" {
					for at := 50; at <= 200; at += 50 {
						paced.tick(at)
					}
					paced.input(210, 0, 0)
					batch.input(210, 0, 0)
					paced.tick(250)
					batch.tick(250)
				} else {
					paced.input(10, 0, 0)
					batch.input(10, 0, 0)
					if name == "late_release" {
						for at := 50; at <= 200; at += 50 {
							paced.tick(at)
						}
						paced.input(210, 4, 0)
						batch.input(210, 4, 0)
						paced.tick(250)
						batch.tick(250)
					} else {
						for at := 50; at <= 200; at += 50 {
							paced.tick(at)
							batch.tick(at)
						}
						for at := 250; at <= 450; at += 50 {
							if name == "renew_after_gap" && at == 400 {
								paced.input(390, 0, 0)
								batch.input(390, 0, 0)
							}
							paced.tick(at)
						}
						batch.tick(450)
					}
				}
				want, wantRemaining := nativeTiming(t, runtime, paced.g)
				got, gotRemaining := nativeTiming(t, runtime, batch.g)
				if got.View.X != want.View.X || got.View.Y != want.View.Y || got.View.Clock != want.View.Clock || got.View.Seed != want.View.Seed || gotRemaining != wantRemaining {
					t.Fatalf("batch differs from paced original rules: want pos=(%d,%d) clock=%d seed=%d remaining=%d; got pos=(%d,%d) clock=%d seed=%d remaining=%d", want.View.X, want.View.Y, want.View.Clock, want.View.Seed, wantRemaining, got.View.X, got.View.Y, got.View.Clock, got.View.Seed, gotRemaining)
				}
				results = append(results, map[string]any{"players": players, "case": name, "position": []int{got.View.X, got.View.Y}, "nativeClock": got.View.Clock, "remainingMS": gotRemaining})
			})
		}
		t.Run(fmt.Sprintf("%d/capped_backlog", players), func(t *testing.T) {
			run := timelineRun{t, m, c, base, c.Now}
			run.input(1000, 0, 0)
			run.tick(1000)
			before, after := view(t, m, base, 0), view(t, m, run.g, 0)
			steps := (before.X - after.X) / 16
			if steps < 0 || steps > 1 || after.Y-before.Y != steps*8 {
				t.Fatal("new held input replayed through the discarded host backlog", before.X, before.Y, after.X, after.Y)
			}
			results = append(results, map[string]any{"players": players, "case": "capped_backlog", "stepsAtArrival": steps})
		})
	}
	if !t.Failed() {
		data, _ := json.MarshalIndent(map[string]any{"passed": true, "test": t.Name(), "scope": "actual original campaign and native time; paced and batched arrival/lease timelines for the single-player platform mode, not full campaign", "cases": results}, "", "  ")
		if e := os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/input-arrival-timeline.json"), append(data, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
}
