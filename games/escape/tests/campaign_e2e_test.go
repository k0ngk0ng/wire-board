package escape_test

// End-to-end rule contracts specified before the production implementation.
// The test drives only the registered module's ordinary actions and clock.
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func module(t *testing.T) gameapi.Module {
	t.Helper()
	for _, m := range games.Modules() {
		if m.ID == "escape" {
			return m
		}
	}
	t.Fatal("密室逃脱 has not been registered")
	return gameapi.Module{}
}
func readView(t *testing.T, m gameapi.Module, s gameapi.Match, who int) map[string]any {
	t.Helper()
	v, e := m.View(s, who)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	var out map[string]any
	if e = json.Unmarshal(raw, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func array(v map[string]any, k string) []any    { a, _ := v[k].([]any); return a }
func obj(x any) map[string]any                  { v, _ := x.(map[string]any); return v }
func number(v map[string]any, k string) float64 { n, _ := v[k].(float64); return n }
func text(v map[string]any, k string) string    { n, _ := v[k].(string); return n }
func boolv(v map[string]any, k string) bool     { n, _ := v[k].(bool); return n }

var epochTime = time.UnixMilli(1_790_000_000_000)

func start(t *testing.T, m gameapi.Module, n int, scenario, difficulty, seed string) gameapi.Match {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"scenario": scenario, "difficulty": difficulty, "seed": seed})
	s, e := m.Start(gameapi.Context{Players: n, Now: epochTime}, raw)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Validate(n); e != nil {
		t.Fatal(e)
	}
	return s
}
func action(t *testing.T, m gameapi.Module, s gameapi.Match, who int, a map[string]any, now time.Time) (gameapi.Match, error) {
	t.Helper()
	v := readView(t, m, s, who)
	a["epoch"] = v["epoch"]
	a["seq"] = number(obj(array(v, "players")[who]), "seq") + 1
	raw, _ := json.Marshal(a)
	return m.Apply(gameapi.Context{Actor: who, Players: len(array(v, "players")), Now: now}, s, raw)
}
func evidence(t *testing.T, name string, v any) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", ".local", "escape", "rules")
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	raw, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, name+".json"), append(raw, '\n'), 0644); e != nil {
		t.Fatal(e)
	}
}
func TestEscapeRegistrationAndOptions(t *testing.T) {
	m := module(t)
	if m.Name != "密室逃脱" || m.MinPlayers != 2 || m.MaxPlayers != 4 || !m.Realtime || m.BotAction == nil || m.Reclaim == nil || m.Restore == nil || m.StreamView == nil || m.SyncSnapshot == nil || m.Controls == nil {
		t.Fatal("incomplete game integration")
	}
	for _, n := range []int{2, 3, 4} {
		for _, scene := range []string{"station", "archive"} {
			for _, d := range []string{"easy", "normal", "hard"} {
				_ = start(t, m, n, scene, d, "blue-lantern")
			}
		}
	}
	for _, n := range []int{0, 1, 5} {
		if _, e := m.Configure(n, nil); e == nil {
			t.Fatal("invalid capacity accepted", n)
		}
	}
	for _, raw := range []string{`{"scenario":"unknown"}`, `{"difficulty":"nightmare"}`, `{"seed":"<script>"}`, `{"power":999}`, `null`, `{"scenario":"station"} {}`} {
		if _, e := m.Configure(2, json.RawMessage(raw)); e == nil {
			t.Fatal("invalid settings accepted", raw)
		}
	}
}
func TestEscapeSeededExplorationAndVisibility(t *testing.T) {
	m := module(t)
	a := start(t, m, 2, "station", "normal", "lantern-17")
	b := start(t, m, 2, "station", "normal", "lantern-17")
	va, vb := readView(t, m, a, 0), readView(t, m, b, 0)
	if !reflect.DeepEqual(va["rooms"], vb["rooms"]) {
		t.Fatal("same seed changes map")
	}
	for _, who := range []int{-1, 0, 1} {
		v := readView(t, m, a, who)
		hidden := 0
		for _, raw := range array(v, "rooms") {
			r := obj(raw)
			if !boolv(r, "known") {
				hidden++
				if text(r, "kind") != "" || len(array(r, "objects")) != 0 {
					t.Fatal("unexplored room leaked", r)
				}
			}
		}
		if hidden < 4 {
			t.Fatal("facility revealed on entry")
		}
		out, _ := json.Marshal(v)
		for _, bad := range []string{`"solution"`, `"randomState"`, `"secret"`} {
			if strings.Contains(string(out), bad) {
				t.Fatal("private state leaked", bad)
			}
		}
		sync, e := m.SyncSnapshot(a, who)
		if e != nil {
			t.Fatal(e)
		}
		out, _ = json.Marshal(sync)
		if strings.Contains(string(out), `"solution"`) {
			t.Fatal("recovery leaks solutions")
		}
	}
	// All graph nodes remain reachable by normal doors; the room-kind and
	// adjacency fingerprint is inspected after discovery by ordinary bots.
	evidence(t, "entry-visibility", map[string]any{"passed": true, "view": va})
}

func TestEscapeRandomControlEpochDoesNotRevealMapSeed(t *testing.T) {
	// Failure: reusing a public transport token as the hidden generator seed
	// lets a client derive undiscovered rooms and answers from the input epoch.
	m := module(t)
	s, e := m.Start(gameapi.Context{Players: 2, Now: epochTime}, nil)
	if e != nil {
		t.Fatal(e)
	}
	var private map[string]any
	if e = json.Unmarshal(s.Data, &private); e != nil {
		t.Fatal(e)
	}
	public := readView(t, m, s, 0)
	if text(private, "seed") == text(public, "epoch") || text(private, "seed") == "" || text(public, "seed") != "" {
		t.Fatal("random map seed is not private to the unfinished match")
	}
}
func TestEscapeRejectsForgedAndStaleActions(t *testing.T) {
	m := module(t)
	s := start(t, m, 2, "station", "normal", "security")
	for _, a := range []map[string]any{{"type": "input", "dx": 99}, {"type": "teleport", "x": 500}, {"type": "interact", "target": "unknown"}, {"type": "input", "hp": 999}, {"type": "turn", "target": "generator", "index": 7}, {"type": "travel", "target": "99"}} {
		if _, e := action(t, m, s, 0, a, epochTime); e == nil {
			t.Fatal("forged action accepted", a)
		}
	}
	v := readView(t, m, s, 0)
	raw, _ := json.Marshal(map[string]any{"type": "input", "dx": 1, "dy": 0, "epoch": v["epoch"], "seq": 1})
	next, e := m.Apply(gameapi.Context{Actor: 0, Players: 2, Now: epochTime}, s, raw)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Apply(gameapi.Context{Actor: 0, Players: 2, Now: epochTime}, next, raw); e == nil {
		t.Fatal("sequence replay accepted")
	}
	raw, _ = json.Marshal(map[string]any{"type": "input", "epoch": "old", "seq": 2})
	if _, e = m.Apply(gameapi.Context{Actor: 0, Players: 2, Now: epochTime}, next, raw); e == nil {
		t.Fatal("old epoch accepted")
	}
	if !reflect.DeepEqual(readView(t, m, s, 0), v) {
		t.Fatal("input mutated previous state")
	}
}

func TestEscapeShortMovementEdgesBeforeNextTick(t *testing.T) {
	// Failure: a press/release wholly between two server ticks is acknowledged
	// but never moves, or is counted a second time on the following timer frame.
	m := module(t)
	s := start(t, m, 2, "station", "normal", "short-input")
	var e error
	s, e = action(t, m, s, 0, map[string]any{"type": "input", "dx": 1, "dy": 0}, epochTime.Add(10*time.Millisecond))
	if e != nil {
		t.Fatal(e)
	}
	s, e = action(t, m, s, 0, map[string]any{"type": "input", "dx": 0, "dy": 0}, epochTime.Add(50*time.Millisecond))
	if e != nil {
		t.Fatal(e)
	}
	before := readView(t, m, s, 0)
	p := obj(array(before, "players")[0])
	x := number(p, "x")
	if x < 446 || x > 447 || number(p, "seq") != 2 || number(p, "appliedSeq") != 1 {
		t.Fatal("short pulse was lost or receipt/applied acknowledgements collapsed", p)
	}
	s, e = m.OnTimer(gameapi.Context{Players: 2, Now: epochTime.Add(100 * time.Millisecond)}, s)
	if e != nil {
		t.Fatal(e)
	}
	after := readView(t, m, s, 0)
	p = obj(array(after, "players")[0])
	if number(p, "x") != x || number(p, "appliedSeq") != 2 {
		t.Fatal("released motion replayed on next frame", p)
	}
	evidence(t, "short-movement-edges", map[string]any{"passed": true, "beforeTick": before, "afterTick": after})
}
func TestEscapePauseRestoreAndManualReclaim(t *testing.T) {
	m := module(t)
	s := start(t, m, 2, "station", "normal", "pause")
	s, e := action(t, m, s, 0, map[string]any{"type": "pause"}, epochTime)
	if e != nil {
		t.Fatal(e)
	}
	s, e = action(t, m, s, 1, map[string]any{"type": "pause"}, epochTime)
	if e != nil || !s.Paused {
		t.Fatal("unanimous pause failed", e)
	}
	before := readView(t, m, s, 0)
	after, e := m.OnTimer(gameapi.Context{Players: 2, Now: epochTime.Add(time.Minute)}, s)
	if e != nil {
		t.Fatal(e)
	}
	if number(readView(t, m, after, 0), "remaining") != number(before, "remaining") {
		t.Fatal("paused clock advanced")
	}
	s, e = action(t, m, after, 0, map[string]any{"type": "resume"}, epochTime.Add(time.Minute))
	if e != nil {
		t.Fatal(e)
	}
	s, e = action(t, m, s, 1, map[string]any{"type": "resume"}, epochTime.Add(time.Minute))
	if e != nil || s.Paused {
		t.Fatal("resume vote failed", e)
	}
	s, e = action(t, m, s, 0, map[string]any{"type": "input", "dx": 1, "dy": 0}, epochTime.Add(time.Minute))
	if e != nil {
		t.Fatal(e)
	}
	s, e = m.Reclaim(gameapi.Context{Actor: 0, Players: 2, Now: epochTime.Add(time.Minute)}, s)
	if e != nil {
		t.Fatal(e)
	}
	p := obj(array(readView(t, m, s, 0), "players")[0])
	if number(p, "dx") != 0 || number(p, "dy") != 0 {
		t.Fatal("old input retained on manual takeover")
	}
	restored, e := m.Restore(gameapi.Context{Players: 2, Now: epochTime.Add(time.Hour)}, s)
	if e != nil {
		t.Fatal(e)
	}
	v := readView(t, m, restored, 0)
	if number(v, "remaining") != number(readView(t, m, s, 0), "remaining") || number(v, "power") != number(readView(t, m, s, 0), "power") {
		t.Fatal("restart consumed offline resources")
	}
	evidence(t, "pause-restore", map[string]any{"passed": true, "before": before, "restored": v})
}
func TestEscapeAllComputerCampaignsE2E(t *testing.T) {
	m := module(t)
	results := []any{}
	for _, scenario := range []string{"station", "archive"} {
		for _, n := range []int{2, 3, 4} {
			for _, difficulty := range []string{"easy", "normal", "hard"} {
				for _, seed := range []string{"lantern-17", "salt-42"} {
					name := fmt.Sprintf("%s-%d-%s-%s", scenario, n, difficulty, seed)
					t.Run(name, func(t *testing.T) {
						s := start(t, m, n, scenario, difficulty, seed)
						now := epochTime
						automated := make([]bool, n)
						for i := range automated {
							automated[i] = true
						}
						for step := 0; step < 16000 && s.Result == nil; step++ {
							now = now.Add(100 * time.Millisecond)
							if step%2 == 0 {
								for seat := 0; seat < n; seat++ {
									ctx := gameapi.Context{Actor: seat, Players: n, Automated: automated, Now: now}
									raw, e := m.BotAction(ctx, s)
									if e != nil {
										t.Fatal(e)
									}
									if len(raw) > 0 {
										s, e = m.Apply(ctx, s, raw)
										if e != nil {
											t.Fatalf("bot %d action %s: %v", seat, raw, e)
										}
									}
								}
							}
							var e error
							s, e = m.OnTimer(gameapi.Context{Players: n, Automated: automated, Now: now}, s)
							if e != nil {
								t.Fatal(e)
							}
							if step == 600 {
								s, e = m.Restore(gameapi.Context{Players: n, Automated: automated, Now: now}, s)
								if e != nil {
									t.Fatal(e)
								}
							}
						}
						v := readView(t, m, s, 0)
						if s.Result == nil || !s.Result.Cooperative || len(s.Result.Winners) != n {
							raw, _ := json.Marshal(v)
							t.Fatalf("campaign failed %s: %s", name, raw)
						}
						if len(array(v, "journal")) < 2 {
							t.Fatal("computer skipped clue discovery")
						}
						if e := s.Validate(n); e != nil {
							t.Fatal(e)
						}
						result := map[string]any{"case": name, "passed": true, "elapsedMs": now.Sub(epochTime).Milliseconds(), "view": v, "result": s.Result}
						results = append(results, result)
						evidence(t, name, result)
					})
				}
			}
		}
	}
	evidence(t, "campaign-matrix", map[string]any{"passed": !t.Failed(), "command": ". scripts/env.sh && go test ./games/escape/tests -run TestEscape -count=1 -v", "cases": results})
}
