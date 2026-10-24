package conquest_test

// These contract scenarios precede the implementation. See
// ../tests/RULE_FAILURE_SCENARIOS.md. All decisions and transitions enter through
// the public module; targeted fixtures change only the starting snapshot.
import (
	"bytes"
	"encoding/json"
	"fmt"
	conquest "github.com/k0ngk0ng/wire-board/games/conquest/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func jsonRaw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func object(t *testing.T, v any) map[string]any {
	t.Helper()
	var d map[string]any
	if e := json.Unmarshal(jsonRaw(v), &d); e != nil {
		t.Fatal(e)
	}
	return d
}
func snapshot(t *testing.T, m gameapi.Match) map[string]any {
	t.Helper()
	var d map[string]any
	if e := json.Unmarshal(m.Data, &d); e != nil {
		t.Fatal(e)
	}
	return d
}
func view(t *testing.T, m gameapi.Match, seat int) map[string]any {
	t.Helper()
	v, e := conquest.Module().View(m, seat)
	if e != nil {
		t.Fatal(e)
	}
	return object(t, v)
}
func start(t *testing.T, mapID string, n int) (gameapi.Match, gameapi.Context) {
	t.Helper()
	c := gameapi.Context{Actor: 0, Players: n, Version: 1, Now: time.Unix(1700000000, 0), Automated: make([]bool, n)}
	o, e := conquest.Module().Configure(n, jsonRaw(map[string]any{"mapId": mapID, "difficulty": "normal", "turnSeconds": 120}))
	if e != nil {
		t.Fatal(e)
	}
	m, e := conquest.Module().Start(c, o)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Validate(n); e != nil {
		t.Fatal(e)
	}
	return m, c
}
func act(t *testing.T, m gameapi.Match, c gameapi.Context, a map[string]any) gameapi.Match {
	t.Helper()
	v := view(t, m, c.Actor)
	a["session"] = v["session"]
	a["epoch"] = v["epoch"]
	n, e := conquest.Module().Apply(c, m, jsonRaw(a))
	if e != nil {
		t.Fatalf("%v: %v", a, e)
	}
	if e = n.Validate(c.Players); e != nil {
		t.Fatal(e)
	}
	return n
}
func resolve(t *testing.T, m gameapi.Match, c *gameapi.Context) gameapi.Match {
	t.Helper()
	for i := 0; i < 2000 && m.SystemDeadline > 0; i++ {
		c.Now = time.UnixMilli(m.SystemDeadline)
		var e error
		m, e = conquest.Module().OnTimer(*c, m)
		if e != nil {
			t.Fatal(e)
		}
	}
	if m.SystemDeadline > 0 {
		t.Fatal("resolution did not finish")
	}
	if e := m.Validate(c.Players); e != nil {
		t.Fatal(e)
	}
	return m
}
func provinces(v map[string]any) []any { return v["provinces"].([]any) }
func own(v map[string]any, seat int) string {
	for _, raw := range provinces(v) {
		p := raw.(map[string]any)
		if int(p["owner"].(float64)) == seat {
			return p["id"].(string)
		}
	}
	return ""
}
func submitAll(t *testing.T, m gameapi.Match, c gameapi.Context) gameapi.Match {
	t.Helper()
	for i := 0; i < c.Players; i++ {
		c.Actor = i
		if view(t, m, i)["controls"].(map[string]any)["canSubmit"] == true {
			m = act(t, m, c, map[string]any{"type": "submit"})
		}
	}
	return m
}

func TestConquestConfigurationAndViews(t *testing.T) {
	mod := conquest.Module()
	if e := mod.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, n := range []int{0, 1, 9, 33} {
		if _, e := mod.Configure(n, nil); e == nil {
			t.Fatalf("accepted capacity %d", n)
		}
	}
	for _, raw := range []string{`null`, `{"mapId":"fake"}`, `{"turnSeconds":1}`, `{"difficulty":"cheat"}`, `{"seed":1}`, `{} {}`} {
		if _, e := mod.Configure(2, json.RawMessage(raw)); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, mapID := range []string{"world", "europe", "usa", "china"} {
		for n := 2; n <= 8; n++ {
			t.Run(fmt.Sprintf("%s-%d", mapID, n), func(t *testing.T) {
				m, _ := start(t, mapID, n)
				v := view(t, m, -1)
				if v["mapId"] != mapID || len(v["players"].([]any)) != n || len(v["orders"].([]any)) != 0 {
					t.Fatal("incorrect projection")
				}
				if v["controls"].(map[string]any)["canPlan"] != false {
					t.Fatal("spectator can plan")
				}
				if len(provinces(v)) != map[string]int{"world": 206, "europe": 75, "usa": 51, "china": 34}[mapID] {
					t.Fatal("map not geographic campaign")
				}
				if v["mapSHA256"] == "" {
					t.Fatal("map identity absent")
				}
				if _, present := v["seed"]; present {
					t.Fatal("private random seed exposed")
				}
			})
		}
	}
}
func TestConquestPlanningSecretsAndAtomicValidation(t *testing.T) {
	m, c := start(t, "europe", 2)
	id := own(view(t, m, 0), 0)
	enemyBefore := view(t, m, 1)
	m = act(t, m, c, map[string]any{"type": "recruit", "province": id, "amount": 100})
	v := view(t, m, 0)
	if len(v["orders"].([]any)) != 1 || len(view(t, m, 1)["orders"].([]any)) != 0 || len(view(t, m, -1)["orders"].([]any)) != 0 {
		t.Fatal("orders projection")
	}
	if !reflect.DeepEqual(provinces(enemyBefore), provinces(view(t, m, 1))) {
		t.Fatal("planning mutated public board")
	}
	public := jsonRaw(view(t, m, 1))
	if bytes.Contains(public, []byte(`"plans"`)) {
		t.Fatal("private plans exposed")
	}
	for _, a := range []map[string]any{{"type": "recruit", "province": id, "amount": -1}, {"type": "recruit", "province": id, "amount": 999999999}, {"type": "recruit", "province": own(v, 1), "amount": 1}, {"type": "move", "from": id, "to": "missing", "amount": 1}, {"type": "build", "province": id, "building": "nuclear"}, {"type": "tax", "value": 101}, {"type": "bogus"}} {
		a["session"] = v["session"]
		a["epoch"] = v["epoch"]
		before := append([]byte(nil), m.Data...)
		if _, e := conquest.Module().Apply(c, m, jsonRaw(a)); e == nil {
			t.Fatalf("accepted bad action %v", a)
		}
		if !bytes.Equal(before, m.Data) {
			t.Fatal("rejection mutated input")
		}
	}
	o := v["orders"].([]any)[0].(map[string]any)
	m = act(t, m, c, map[string]any{"type": "undo", "id": o["id"]})
	if len(view(t, m, 0)["orders"].([]any)) != 0 {
		t.Fatal("undo failed")
	}
	m = act(t, m, c, map[string]any{"type": "submit"})
	if view(t, m, 0)["controls"].(map[string]any)["canPlan"] != false {
		t.Fatal("submitted can plan")
	}
	m = act(t, m, c, map[string]any{"type": "unsubmit"})
	if view(t, m, 0)["controls"].(map[string]any)["canPlan"] != true {
		t.Fatal("cannot reopen")
	}
}
func TestConquestRoundResolutionRestoreAndReplay(t *testing.T) {
	m, c := start(t, "europe", 2)
	initial := m
	id := own(view(t, m, 0), 0)
	m = act(t, m, c, map[string]any{"type": "recruit", "province": id, "amount": 100})
	planned := m
	// A stored JSON roundtrip is the same public state and the same next action.
	var restored gameapi.Match
	if e := json.Unmarshal(jsonRaw(m), &restored); e != nil {
		t.Fatal(e)
	}
	mod := conquest.Module()
	if mod.Restore != nil {
		var e error
		restored, e = mod.Restore(c, restored)
		if e != nil {
			t.Fatal(e)
		}
	}
	if !reflect.DeepEqual(view(t, m, 0), view(t, restored, 0)) {
		t.Fatal("restore changed plan")
	}
	m = submitAll(t, m, c)
	restored = submitAll(t, restored, c)
	if !bytes.Equal(m.Data, restored.Data) {
		t.Fatal("submission depends on hidden runtime state")
	}
	m = resolve(t, m, &c)
	restored = resolve(t, restored, &c)
	if !bytes.Equal(m.Data, restored.Data) {
		t.Fatal("replay differs")
	}
	if view(t, m, 0)["round"].(float64) != 2 {
		t.Fatal("round did not advance")
	}
	dir := filepath.Join("..", "..", "..", ".local", "conquest", "rules")
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "round-replay.json"), jsonRaw(map[string]any{"passed": true, "initial": initial, "planned": planned, "final": m}), 0644); e != nil {
		t.Fatal(e)
	}
}
func TestConquestPauseAndDeadlineBudget(t *testing.T) {
	m, c := start(t, "europe", 2)
	deadline := m.Deadline
	c.Now = c.Now.Add(17 * time.Second)
	m = act(t, m, c, map[string]any{"type": "pause", "enabled": true})
	if view(t, m, 0)["phase"] == "paused" {
		t.Fatal("one vote paused two human players")
	}
	c.Actor = 1
	m = act(t, m, c, map[string]any{"type": "pause", "enabled": true})
	if view(t, m, 0)["phase"] != "paused" || m.Deadline != 0 || m.SystemDeadline != 0 {
		t.Fatal("pause did not freeze")
	}
	if b, e := conquest.Module().BotAction(c, m); e != nil || len(b) != 0 {
		t.Fatal("paused bot action", string(b), e)
	}
	c.Now = c.Now.Add(48 * time.Hour)
	c.Actor = 0
	m = act(t, m, c, map[string]any{"type": "resume", "enabled": true})
	c.Actor = 1
	m = act(t, m, c, map[string]any{"type": "resume", "enabled": true})
	if view(t, m, 0)["phase"] != "planning" {
		t.Fatal("resume did not restore")
	}
	want := c.Now.UnixMilli() + deadline - time.Unix(1700000000, 0).Add(17*time.Second).UnixMilli()
	if m.Deadline != want {
		t.Fatalf("resume changed budget got %d want %d", m.Deadline, want)
	}
}
func TestConquestRejectsTamperedSave(t *testing.T) {
	m, c := start(t, "world", 2)
	mod := conquest.Module()
	for _, field := range []string{"mapSHA256", "mapVersion"} {
		bad := m
		s := snapshot(t, m)
		if field == "mapVersion" {
			s[field] = 999
		} else {
			s[field] = "wrong"
		}
		bad.Data = jsonRaw(s)
		if _, e := mod.Restore(c, bad); e == nil {
			t.Fatalf("accepted bad %s", field)
		}
	}
	bad := m
	bad.Schema = 999
	if _, e := mod.View(bad, 0); e == nil {
		t.Fatal("unknown schema accepted")
	}
}

func TestConquestFogProjectionBeforeImplementation(t *testing.T) {
	mod := conquest.Module()
	c := gameapi.Context{Actor: 0, Players: 2, Version: 1, Now: time.Unix(1700000000, 0)}
	opts, e := mod.Configure(2, jsonRaw(map[string]any{"mapId": "world", "difficulty": "normal", "turnSeconds": 120, "fog": true}))
	if e != nil {
		t.Fatal(e)
	}
	m, e := mod.Start(c, opts)
	if e != nil {
		t.Fatal(e)
	}
	v := view(t, m, 0)
	hidden := 0
	for _, r := range provinces(v) {
		p := r.(map[string]any)
		if p["owner"].(float64) != 0 && p["troops"].(float64) < 0 {
			hidden++
		}
	}
	if hidden == 0 {
		t.Fatal("fog reveals every garrison")
	}
	enemy := v["players"].([]any)[1].(map[string]any)
	if enemy["gold"].(float64) != -1 || enemy["troops"].(float64) != -1 || enemy["king"] == "" {
		t.Fatal("fog summary leaks", enemy)
	}
	if v["controls"].(map[string]any)["canBuildTower"] != true {
		t.Fatal("fog missing tower")
	}
	// The old renderer exposes a ruler inside a town even when the army is
	// unknown. A ruler outside a town remains hidden without reconnaissance.
	state := snapshot(t, m)
	king := enemy["king"].(string)
	for _, raw := range state["provinces"].([]any) {
		p := raw.(map[string]any)
		if p["id"] == king {
			p["town"] = false
		}
	}
	m.Data = jsonRaw(state)
	if view(t, m, 0)["players"].([]any)[1].(map[string]any)["king"] != "" {
		t.Fatal("unseen ruler outside a town revealed")
	}
}
