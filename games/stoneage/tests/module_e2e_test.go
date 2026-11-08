package stoneage_test

// Written before implementation. Failure inventory: FAILURE_SCENARIOS.md.
// These scenarios call the public production module, including full matches.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	stoneage "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func obj(t *testing.T, v any) map[string]any {
	t.Helper()
	var out map[string]any
	if e := json.Unmarshal(raw(v), &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func data(t *testing.T, m gameapi.Match) map[string]any {
	t.Helper()
	var out map[string]any
	if e := json.Unmarshal(m.Data, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func view(t *testing.T, m gameapi.Match, seat int) map[string]any {
	t.Helper()
	v, e := stoneage.Module().View(m, seat)
	if e != nil {
		t.Fatal(e)
	}
	return obj(t, v)
}
func start(t *testing.T, n int) (gameapi.Match, gameapi.Context) {
	t.Helper()
	c := gameapi.Context{Actor: 0, Players: n, Version: 1, Now: time.Unix(1794000000, 0), Automated: make([]bool, n)}
	mod := stoneage.Module()
	o, e := mod.Configure(n, nil)
	if e != nil {
		t.Fatal(e)
	}
	m, e := mod.Start(c, o)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.Validate(n); e != nil {
		t.Fatal(e)
	}
	return m, c
}
func action(t *testing.T, m gameapi.Match, c gameapi.Context, a map[string]any) gameapi.Match {
	t.Helper()
	v := view(t, m, c.Actor)
	a["session"], a["window"] = v["session"], v["window"]
	next, e := stoneage.Module().Apply(c, m, raw(a))
	if e != nil {
		t.Fatalf("action %v: %v", a, e)
	}
	if e = next.Validate(c.Players); e != nil {
		t.Fatal(e)
	}
	return next
}
func bot(t *testing.T, m gameapi.Match, c gameapi.Context) json.RawMessage {
	t.Helper()
	b, e := stoneage.Module().BotAction(c, m)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func botsStep(t *testing.T, m gameapi.Match, c *gameapi.Context) gameapi.Match {
	t.Helper()
	mod := stoneage.Module()
	if m.SystemDeadline > 0 {
		c.Actor = -1
		c.Now = time.UnixMilli(m.SystemDeadline)
		n, e := mod.OnTimer(*c, m)
		if e != nil {
			t.Fatal(e)
		}
		return n
	}
	for _, p := range m.Actors {
		c.Actor = p
		b := bot(t, m, *c)
		if len(b) == 0 {
			continue
		}
		n, e := mod.Apply(*c, m, b)
		if e != nil {
			t.Fatalf("bot %d %s: %v", p, b, e)
		}
		return n
	}
	t.Fatal("match has no progress action")
	return m
}
func report(t *testing.T, name string, value any) {
	t.Helper()
	p := filepath.Join("..", "..", "..", ".local", "stoneage", "verification")
	if e := os.MkdirAll(p, 0755); e != nil {
		t.Fatal(e)
	}
	b, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "evidence": value}, "", "  ")
	if e := os.WriteFile(filepath.Join(p, name+".json"), b, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestStoneageConfigurationAndCompletePools(t *testing.T) {
	mod := stoneage.Module()
	if e := mod.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, n := range []int{0, 1, 3, 5, 7, 9, 11} {
		if _, e := mod.Configure(n, nil); e == nil {
			t.Fatalf("accepted %d", n)
		}
	}
	for _, bad := range []any{map[string]any{"turnSeconds": 0}, map[string]any{"maxRounds": 999}, map[string]any{"reservePets": 1}} {
		if _, e := mod.Configure(2, raw(bad)); e == nil {
			t.Fatalf("accepted options %v", bad)
		}
	}
	hashes := map[string]bool{}
	for k := 0; k < 40; k++ {
		c := gameapi.Context{Players: 10, Now: time.Unix(1794000000+int64(k), 0), Version: k + 1}
		o, _ := mod.Configure(10, nil)
		m, e := mod.Start(c, o)
		if e != nil {
			t.Fatal(e)
		}
		first := view(t, m, 0)
		pool := first["pool"].(map[string]any)
		hashes[string(raw(pool))] = true
		for name, min := range map[string]int{"pets": 20, "rides": 6, "skills": 14, "hats": 4, "weapons": 6, "clothes": 4, "accessories": 6, "hpItems": 2, "mpItems": 2, "cureItems": 2, "tactics": 4} {
			if len(pool[name].([]any)) < min {
				t.Fatalf("%s pool missing: %v", name, pool[name])
			}
		}
		for p := -1; p < 10; p++ {
			if !reflect.DeepEqual(view(t, m, p)["pool"], pool) {
				t.Fatalf("different pool for seat %d", p)
			}
		}
		if strings.Contains(string(raw(first)), `"random"`) || strings.Contains(string(raw(first)), `"plans"`) {
			t.Fatal("hidden state leaked")
		}
	}
	if len(hashes) < 35 {
		t.Fatal("pools are not varied", len(hashes))
	}
	report(t, "random-pool-invariants", map[string]any{"matches": 40, "distinctPools": len(hashes), "modes": []int{2, 4, 6, 8, 10}})
}
func TestStoneageSelectionPrivacyAndIllegalLoadouts(t *testing.T) {
	m, c := start(t, 4)
	mod := stoneage.Module()
	b := bot(t, m, c)
	var a map[string]any
	json.Unmarshal(b, &a)
	if a["type"] != "loadout" {
		t.Fatal("bot must configure first", a)
	}
	before := append([]byte(nil), m.Data...)
	for _, field := range []string{"pet", "ride", "hat", "weapon", "clothes", "hpItem", "mpItem", "cureItem"} {
		bad := obj(t, a)
		l := bad["loadout"].(map[string]any)
		l[field] = float64(999999)
		if _, e := mod.Apply(c, m, raw(bad)); e == nil {
			t.Fatalf("accepted unknown %s", field)
		}
		if !bytes.Equal(before, m.Data) {
			t.Fatal("rejection mutated input")
		}
	}
	bad := obj(t, a)
	bad["loadout"].(map[string]any)["skills"] = []int{1, 2, 3, 10, 20, 30, 40, 50}
	if _, e := mod.Apply(c, m, raw(bad)); e == nil {
		t.Fatal("accepted eight skills")
	}
	bad = obj(t, a)
	bad["loadout"].(map[string]any)["reservePets"] = []int{1}
	if _, e := mod.Apply(c, m, raw(bad)); e == nil {
		t.Fatal("accepted reserves")
	}
	next, e := mod.Apply(c, m, b)
	if e != nil {
		t.Fatal(e)
	}
	m = next
	for _, p := range []int{-1, 1, 2, 3} {
		v := view(t, m, p)
		if v["self"] != nil && p < 0 {
			t.Fatal("spectator private projection")
		}
		players := v["players"].([]any)
		if players[0].(map[string]any)["loadout"] != nil {
			t.Fatal("unlocked opponent loadout leaked")
		}
	}
	if view(t, m, 0)["self"].(map[string]any)["loadout"] == nil {
		t.Fatal("own loadout missing")
	}
	m = action(t, m, c, map[string]any{"type": "lock"})
	if _, e = mod.Apply(c, m, b); e == nil {
		t.Fatal("changed locked loadout")
	}
	for _, kind := range []string{"switch_pet", "ride", "revive"} {
		if _, e = mod.Apply(c, m, raw(map[string]any{"type": kind, "session": view(t, m, 0)["session"], "window": view(t, m, 0)["window"]})); e == nil {
			t.Fatal("accepted forbidden action", kind)
		}
	}
	report(t, "selection-privacy", view(t, m, -1))
}
func TestStoneageAllModesBotCompleteAndRestoreE2E(t *testing.T) {
	for _, n := range []int{2, 4, 6, 8, 10} {
		t.Run(fmt.Sprintf("%dv%d", n/2, n/2), func(t *testing.T) {
			m, c := start(t, n)
			mod := stoneage.Module()
			trace := []any{}
			rounds := map[int]bool{}
			damage := 0
			for steps := 0; steps < 1200 && m.Result == nil; steps++ {
				old := append([]byte(nil), m.Data...)
				m = botsStep(t, m, &c)
				if e := m.Validate(n); e != nil {
					t.Fatal(e)
				}
				v := view(t, m, 0)
				r := int(v["round"].(float64))
				rounds[r] = true
				if r > 26 {
					t.Fatal("turn limit exceeded")
				}
				if steps%19 == 0 {
					restored, e := mod.Restore(c, gameapi.Match{Schema: m.Schema, Data: append([]byte(nil), m.Data...), Actors: m.Actors, TimeoutActors: m.TimeoutActors, Deadline: m.Deadline, SystemDeadline: m.SystemDeadline, Result: m.Result})
					if e != nil {
						t.Fatal(e)
					}
					if !bytes.Equal(restored.Data, m.Data) {
						t.Fatal("restore changed persisted game")
					}
				}
				if string(old) != string(m.Data) {
					trace = append(trace, map[string]any{"phase": v["phase"], "round": r, "events": v["events"]})
				}
			}
			if m.Result == nil {
				t.Fatal("bots stalled")
			}
			if len(rounds) < 2 {
				t.Fatal("no real combat rounds")
			}
			final := view(t, m, -1)
			for _, p := range final["players"].([]any) {
				damage += int(p.(map[string]any)["damage"].(float64))
			}
			if damage <= 0 {
				t.Fatal("no actual damage")
			}
			if m.Result.Draw && len(m.Result.Winners) != 0 {
				t.Fatal("draw has winners")
			}
			if !m.Result.Draw && len(m.Result.Winners) != n/2 {
				t.Fatal("incomplete team result", m.Result)
			}
			report(t, fmt.Sprintf("complete-%dv%d", n/2, n/2), map[string]any{"trace": trace, "final": final, "result": m.Result})
		})
	}
}
func TestStoneageConcurrentPlansAndReclaim(t *testing.T) {
	m, c := start(t, 10)
	mod := stoneage.Module()
	for steps := 0; (view(t, m, 0)["phase"] == "selection" || view(t, m, 0)["phase"] == "arming") && steps < 40; steps++ {
		m = botsStep(t, m, &c)
	}
	if view(t, m, 0)["phase"] != "planning" {
		t.Fatal("selection did not start combat")
	}
	c.Actor = 0
	b := bot(t, m, c)
	before := append([]byte(nil), m.Data...)
	next, e := mod.Apply(c, m, b)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(before, m.Data) {
		t.Fatal("apply mutated input")
	}
	m = next
	for _, seat := range []int{-1, 1} {
		v := view(t, m, seat)
		if strings.Contains(string(raw(v)), `"plan":{`) {
			t.Fatal("submitted plan leaked")
		}
	}
	if _, e = mod.Apply(c, m, b); e == nil {
		t.Fatal("duplicate plan accepted")
	}
	oldDeadlines := data(t, m)["players"]
	restored, e := mod.Reclaim(c, m)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(data(t, restored)["players"], oldDeadlines) {
		t.Fatal("reclaim reopened submitted command or moved deadlines")
	}
	for i := 1; i < 10; i++ {
		c.Actor = i
		b = bot(t, m, c)
		var e error
		m, e = mod.Apply(c, m, b)
		if e != nil {
			t.Fatal(e)
		}
	}
	if view(t, m, 0)["phase"] != "resolving" || m.SystemDeadline == 0 {
		t.Fatal("joint decisions not resolved")
	}
	c.Actor = -1
	c.Now = time.UnixMilli(m.SystemDeadline)
	m, e = mod.OnTimer(c, m)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = mod.Apply(gameapi.Context{Actor: 9, Players: 10, Now: c.Now}, m, b); e == nil {
		t.Fatal("old-round plan accepted")
	}
	report(t, "simultaneous-plans-and-reclaim", view(t, m, -1))
}
