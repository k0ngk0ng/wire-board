package conquest_test

// Public-module scenarios fixed before implementation; see ROUND_FLOW_FAILURE_SCENARIOS.md.
import (
	"bytes"
	"encoding/json"
	"fmt"
	maps "github.com/k0ngk0ng/wire-board/games/conquest/maps"
	conquest "github.com/k0ngk0ng/wire-board/games/conquest/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func roundArtifact(t *testing.T, name string, value any) {
	t.Helper()
	root := filepath.Join("..", "..", "..", ".local", "conquest-round", "rules")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, name+".json"), append(b, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}
func TestConquestRandomStartsE2E(t *testing.T) {
	records := []any{}
	defer func() { roundArtifact(t, "random-starts", map[string]any{"passed": !t.Failed(), "cases": records}) }()
	mod := conquest.Module()
	for _, mapID := range []string{"world", "europe", "usa", "china"} {
		for n := 2; n <= 8; n++ {
			t.Run(fmt.Sprintf("%s/%d", mapID, n), func(t *testing.T) {
				geo, err := maps.Load(mapID)
				if err != nil {
					t.Fatal(err)
				}
				links := map[string]map[string]bool{}
				for _, p := range geo.Provinces {
					links[p.ID] = map[string]bool{}
					for _, id := range append(append([]string{}, p.Neighbors...), p.SeaNeighbors...) {
						links[p.ID][id] = true
					}
				}
				first := map[string]bool{}
				all := map[string]bool{}
				layouts := []any{}
				opts, _ := mod.Configure(n, jsonRaw(map[string]any{"mapId": mapID}))
				for seed := 0; seed < 32; seed++ {
					c := gameapi.Context{Players: n, Version: seed + 1, Now: time.Unix(1792830000, int64(seed)*1000003)}
					m, err := mod.Start(c, opts)
					if err != nil {
						t.Fatal(err)
					}
					v, err := mod.View(m, 0)
					if err != nil {
						t.Fatal(err)
					}
					d := v.(conquest.View)
					ids := []string{}
					seen := map[string]bool{}
					for _, p := range d.Players {
						if seen[p.King] || links[p.King] == nil {
							t.Fatal("duplicate or missing starting region", p.King)
						}
						for other := range seen {
							if links[p.King][other] || links[other][p.King] {
								t.Fatal("adjacent starting regions", p.King, other)
							}
						}
						seen[p.King] = true
						ids = append(ids, p.King)
						all[p.King] = true
					}
					first[ids[0]] = true
					layouts = append(layouts, ids)
					replay, err := mod.Start(c, opts)
					if err != nil || !bytes.Equal(replay.Data, m.Data) {
						t.Fatal("start is not replayable", err)
					}
					restored, err := mod.Restore(c, m)
					if err != nil || !bytes.Equal(restored.Data, m.Data) {
						t.Fatal("restore rerolled or changed start", err)
					}
				}
				records = append(records, map[string]any{"map": mapID, "players": n, "layouts": layouts})
				if len(first) < 8 || len(all) <= n {
					t.Errorf("starts remain fixed or only shuffle one preset: first=%d total=%d", len(first), len(all))
				}
			})
		}
	}
}
func TestConquestOriginalBudgetAndCarryE2E(t *testing.T) {
	m, c := start(t, "europe", 2)
	d := view(t, m, 0)
	if d["availableMovement"] != 3.2 {
		t.Fatalf("original opening budget changed: %v", d["availableMovement"])
	}
	controls := d["controls"].(map[string]any)
	for key, want := range map[string]float64{"moveCost": 1, "recruitMovement": 1, "fortMovement": .5, "towerMovement": .3, "townMovement": 5} {
		if controls[key] != want {
			t.Fatal(key, controls[key], want)
		}
	}
	home := own(d, 0)
	m = act(t, m, c, map[string]any{"type": "build", "province": home, "building": "fort"})
	if view(t, m, 0)["availableMovement"] != 2.7 {
		t.Fatal("original fort cost changed")
	}
	m = act(t, m, c, map[string]any{"type": "disband", "province": home, "amount": 1})
	if view(t, m, 0)["availableMovement"] != 2.6 {
		t.Fatal("original disband cost changed")
	}
	m = act(t, m, c, map[string]any{"type": "tax", "value": 20})
	if view(t, m, 0)["availableMovement"] != 2.6 {
		t.Fatal("tax must remain free of movement points")
	}
	m, c = start(t, "europe", 2)
	home = own(view(t, m, 0), 0)
	for i := 0; i < 3; i++ {
		m = act(t, m, c, map[string]any{"type": "recruit", "province": home, "amount": 1})
	}
	d = view(t, m, 0)
	if d["availableMovement"] != .2 {
		t.Fatal("fractional remainder was discarded", d["availableMovement"])
	}
	_, err := conquest.Module().Apply(c, m, jsonRaw(map[string]any{"type": "recruit", "province": home, "amount": 1, "session": d["session"], "epoch": d["epoch"]}))
	if err == nil {
		t.Fatal("fractional remainder incorrectly permits another full recruitment")
	}
	expected := []float64{3.3, 4.8, 5.6}
	for _, want := range expected {
		m = submitAll(t, m, c)
		m = resolve(t, m, &c)
		if got := view(t, m, 0)["availableMovement"]; got != want {
			t.Fatal("original half-carry formula changed", got, want)
		}
	}
	roundArtifact(t, "original-budget", map[string]any{"passed": true, "final": snapshot(t, m)})
}
func TestConquestSequentialVisibleActionsE2E(t *testing.T) {
	mod := conquest.Module()
	m, c := start(t, "europe", 2)
	initial := snapshot(t, m)
	home := own(view(t, m, 0), 0)
	geo, _ := maps.Load("europe")
	target := ""
	for _, p := range geo.Provinces {
		if p.ID == home {
			targets := append(append([]string{}, p.Neighbors...), p.SeaNeighbors...)
			target = targets[0]
		}
	}
	m = act(t, m, c, map[string]any{"type": "recruit", "province": home, "amount": 1})
	m = act(t, m, c, map[string]any{"type": "move", "from": home, "to": target, "amount": 100})
	c.Actor = 1
	m = act(t, m, c, map[string]any{"type": "recruit", "province": own(view(t, m, 1), 1), "amount": 1})
	m = submitAll(t, m, c)
	d := view(t, m, 0)
	events := d["events"].([]any)
	if len(events) != 1 {
		t.Fatal("multiple operations executed before playback", events)
	}
	e := events[0].(map[string]any)
	if e["type"] != "recruit" || e["until"].(float64)-e["at"].(float64) < 900 {
		t.Fatal("recruitment has no readable timeline", e)
	}
	if !reflect.DeepEqual(initial["provinces"], snapshot(t, m)["provinces"]) {
		t.Fatal("resources changed before the visible action completed")
	}
	c.Now = time.UnixMilli(m.SystemDeadline - 1)
	early, err := mod.OnTimer(c, m)
	if err != nil || !bytes.Equal(early.Data, m.Data) {
		t.Fatal("early timer committed action", err)
	}
	c.Now = time.UnixMilli(m.SystemDeadline)
	m, err = mod.OnTimer(c, m)
	if err != nil {
		t.Fatal(err)
	}
	d = view(t, m, 0)
	events = d["events"].([]any)
	e = events[len(events)-1].(map[string]any)
	if len(events) != 1 || e["applied"] != true || m.SystemDeadline-c.Now.UnixMilli() < 500 {
		t.Fatal("no separate result-reading hold", e, m.SystemDeadline)
	}
	restored, err := mod.Restore(c, m)
	if err != nil || !bytes.Equal(restored.Data, m.Data) {
		t.Fatal("restoring result hold changes progress", err)
	}
	m = resolve(t, m, &c)
	d = view(t, m, 0)
	events = d["events"].([]any)
	if len(events) != 3 {
		t.Fatal("missing or duplicated own/enemy actions", events)
	}
	previous := float64(0)
	kinds := []string{}
	for _, raw := range events {
		e := raw.(map[string]any)
		if e["applied"] != true || e["at"].(float64) < previous || e["until"].(float64)-e["at"].(float64) < 900 {
			t.Fatal("broken sequential timeline", e)
		}
		previous = e["displayUntil"].(float64)
		kinds = append(kinds, e["type"].(string))
	}
	if !reflect.DeepEqual(kinds, []string{"recruit", "recruit", "move"}) {
		t.Fatal(kinds)
	}
	roundArtifact(t, "sequential-actions", map[string]any{"passed": true, "events": events, "final": snapshot(t, m)})
}
func TestConquestLegacyTimelineRestoreE2E(t *testing.T) {
	for _, name := range []string{"planning", "paused", "resolving"} {
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("..", "tests", "testdata", "v1333-"+name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var fixture struct {
				Context gameapi.Context `json:"context"`
				Match   gameapi.Match   `json:"match"`
			}
			if err = json.Unmarshal(b, &fixture); err != nil {
				t.Fatal(err)
			}
			mod := conquest.Module()
			before := snapshot(t, fixture.Match)
			m, err := mod.Restore(fixture.Context, fixture.Match)
			if err != nil {
				t.Fatal(err)
			}
			after := snapshot(t, m)
			for _, key := range []string{"session", "epoch", "round", "phase", "players", "provinces", "plans", "remaining", "resolveAt", "cursor"} {
				if !reflect.DeepEqual(before[key], after[key]) {
					t.Fatal("restore changed live game", key)
				}
			}
			if name != "resolving" {
				if view(t, m, 0)["availableMovement"] != float64(2) {
					t.Fatal("legacy queued plans changed remaining movement points")
				}
				if view(t, m, 0)["controls"].(map[string]any)["fortMovement"] != .5 {
					t.Fatal("legacy planning changed its original fort cost")
				}
			} else {
				if m.SystemDeadline != fixture.Match.SystemDeadline {
					t.Fatal("in-flight timer was reset")
				}
				m = resolve(t, m, &fixture.Context)
				v := view(t, m, 0)
				n := v["availableMovement"].(float64)
				if n != 3.8 || v["controls"].(map[string]any)["fortMovement"] != .5 {
					t.Fatal("old resolving save changed its original point carry")
				}
			}
			again, err := mod.Restore(fixture.Context, m)
			if err != nil || !bytes.Equal(again.Data, m.Data) {
				t.Fatal("restore is not idempotent", err)
			}
			roundArtifact(t, "legacy-"+name, map[string]any{"passed": true, "final": snapshot(t, m)})
		})
	}
}
