package pal_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func nativeModule(t *testing.T) (gameapi.Module, *engine.Runtime) {
	t.Helper()
	root := filepath.Join("..", "..", "..")
	program := filepath.Join(root, ".local", "pal", "native", "rules.wasm")
	if profile := developmentHealthProfile(); profile != "" {
		program = filepath.Join(root, ".local/pal/test-health", profile, "campaign/rules.wasm")
	}
	if variant := campaignVariant(); variant != "" {
		profile := developmentHealthProfile()
		if profile == "" {
			profile = "original"
		}
		program = filepath.Join(root, ".local/pal/runtime-candidates", variant, profile, "rules.wasm")
	}
	if err := os.MkdirAll(filepath.Dir(campaignEvidencePath("profile.json", 1)), 0700); err != nil {
		t.Fatal(err)
	}
	wasm, e := os.ReadFile(program)
	if e != nil {
		t.Fatal("build native rule core first", e)
	}
	var r *engine.Runtime
	switch mode := os.Getenv("PAL_TEST_AUTHORITY"); mode {
	case "":
		r, e = engine.NewRuntime(context.Background(), wasm, os.DirFS(filepath.Join(root, ".local", "pal", "original-98")))
	case "1":
		if campaignVariant() != "audio" {
			t.Fatal("authoritative fixture requires the current audio candidate")
		}
		archive, err := os.ReadFile(filepath.Join(root, ".local/pal/authority-prepared/authority.zip"))
		if err != nil {
			t.Fatal(err)
		}
		r, e = engine.NewAuthorityRuntime(context.Background(), wasm, archive)
	default:
		t.Fatal("invalid PAL_TEST_AUTHORITY fixture mode", mode)
	}
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	m := pal.NewModule(r)
	if e = m.Validate(); e != nil {
		t.Fatal(e)
	}
	return m, r
}
func view(t *testing.T, m gameapi.Module, g gameapi.Match, actor int) pal.View {
	t.Helper()
	v, e := m.View(g, actor)
	if e != nil {
		t.Fatal(e)
	}
	return v.(pal.View)
}
func campaignBotContext(c gameapi.Context) gameapi.Context {
	c.Automated = make([]bool, c.Players)
	for i := range c.Automated {
		c.Automated[i] = true
	}
	return c
}
func raw(v any) json.RawMessage { data, _ := json.Marshal(v); return data }
func apply(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, a pal.Action) gameapi.Match {
	t.Helper()
	if a.Type == "input" && a.ControlAt == 0 {
		a.ControlAt = view(t, m, g, c.Actor).ControlAt
	}
	next, e := m.Apply(c, g, raw(a))
	if e != nil {
		t.Fatal(a, e)
	}
	if e = m.ValidateMatch(next, c.Players); e != nil {
		t.Fatal(e)
	}
	return next
}
func timers(t *testing.T, m gameapi.Module, c *gameapi.Context, g gameapi.Match) gameapi.Match {
	t.Helper()
	for i := 0; i < 2048 && view(t, m, g, 0).Phase != "dialogue" && !(view(t, m, g, 0).Phase == "field" && view(t, m, g, 0).Scene == 2); i++ {
		if g.SystemDeadline == 0 {
			t.Fatal("native continuation lost timer")
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		next, e := m.OnTimer(*c, g)
		if e != nil {
			t.Fatal(e)
		}
		g = next
	}
	return g
}

func TestOriginalCampaignCooperativeControlAndSave(t *testing.T) {
	m, runtime := nativeModule(t)
	for _, n := range []int{1} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			c := gameapi.Context{Players: n, Actor: 0, Now: time.Unix(1800000000, 0), Version: 1, RequestedVersion: 1, Automated: make([]bool, n)}
			g, e := m.Start(c, raw(map[string]any{}))
			if e != nil {
				t.Fatal(e)
			}
			g = timers(t, m, &c, g)
			v := view(t, m, g, 0)
			if v.Phase != "dialogue" || v.Dialogue.Text != "李大娘∶\n李逍遥！你皮痒啊？\n敢说老娘是什么鬼婆！" || len(v.Overlay) == 0 || len(v.Party) != 1 || v.Party[0].Role != 0 || v.Party[0].Controller != 0 {
				t.Fatal("original party or opening changed", v)
			}
			public, _ := json.Marshal(v)
			for _, key := range []string{"memory", "native", "seed", "clock", "pc", "buildID", "resourceID"} {
				if strings.Contains(string(public), `"`+key+`"`) {
					t.Fatal("private native state exposed", key)
				}
			}
			initial := append([]byte(nil), g.Data...)
			bad := pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID + 1}
			if _, e = m.Apply(c, g, raw(bad)); e == nil {
				t.Fatal("future prompt accepted")
			}
			if !reflect.DeepEqual(initial, []byte(g.Data)) {
				t.Fatal("rejected action mutated snapshot")
			}
			seen := 0
			for seen < 100 {
				g = timers(t, m, &c, g)
				v = view(t, m, g, 0)
				if v.Phase == "field" && v.Scene == 2 {
					break
				}
				if v.Dialogue == nil {
					t.Fatal("campaign did not reach next original message", v)
				}
				prompt := v.Dialogue.ID
				for actor := 0; actor < n; actor++ {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: prompt})
					if actor < n-1 && view(t, m, g, actor).Dialogue.ID != prompt {
						t.Fatal("one reader skipped teammate")
					}
				}
				if _, e = m.Apply(c, g, raw(pal.Action{Type: "read", Epoch: v.Epoch, Prompt: prompt})); e == nil {
					t.Fatal("duplicate prompt advanced story")
				}
				if seen == 0 {
					g, e = m.SaveControl(c, g, true)
					if e != nil {
						t.Fatal(e)
					}
					c.Now = c.Now.Add(time.Hour)
					g, e = m.ResumeSaved(c, g)
					if e != nil {
						t.Fatal(e)
					}
				}
				seen++
			}
			v = view(t, m, g, 0)
			var stored struct {
				Native []byte `json:"native"`
			}
			if e = json.Unmarshal(g.Data, &stored); e != nil {
				t.Fatal(e)
			}
			native, e := runtime.Decode(stored.Native)
			if e != nil {
				t.Fatal(e)
			}
			if seen != 23 || native.View.Messages != 72 || v.Phase != "field" || v.Scene != 2 || v.Map != 12 || len(v.Party) != 1 {
				t.Fatal("original inn introduction changed", seen, v)
			}
			if n > 1 {
				c.Actor = 1
				if _, e = m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: v.Epoch, Seq: 1, Direction: 0})); e == nil {
					t.Fatal("nonleader moved whole party")
				}
				c.Actor = 0
				g = apply(t, m, c, g, pal.Action{Type: "leader", Epoch: v.Epoch, Target: 1})
				v = view(t, m, g, 1)
				if v.Leader != 1 || len(v.Party) != 1 {
					t.Fatal("leadership transfer invented a party member")
				}
			}
			c.Actor = v.Leader
			beforeX, beforeY := v.X, v.Y
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: 1, Direction: 0})
			for i := 0; i < 5; i++ {
				c.Now = time.UnixMilli(g.SystemDeadline)
				g, e = m.OnTimer(c, g)
				if e != nil {
					t.Fatal(e)
				}
			}
			v = view(t, m, g, c.Actor)
			if v.X == beforeX && v.Y == beforeY {
				t.Fatal("original hero did not move")
			}
			c.Now = time.UnixMilli(g.SystemDeadline - 25)
			frozen, e := m.SaveControl(c, g, true)
			if e != nil {
				t.Fatal(e)
			}
			if !frozen.Paused || frozen.SystemDeadline != 0 {
				t.Fatal("save did not freeze original game")
			}
			if _, e = m.Apply(c, frozen, raw(pal.Action{Type: "input", Epoch: v.Epoch, Seq: 2, Direction: 0})); e == nil {
				t.Fatal("input moved paused original game")
			}
			saved := append([]byte(nil), frozen.Data...)
			c.Now = c.Now.Add(10 * time.Minute)
			again, e := m.OnTimer(c, frozen)
			if e != nil || !reflect.DeepEqual(frozen, again) {
				t.Fatal("saved game advanced while paused", e)
			}
			resumed, e := m.ResumeSaved(c, frozen)
			if e != nil {
				t.Fatal(e)
			}
			if resumed.SystemDeadline != c.Now.Add(25*time.Millisecond).UnixMilli() {
				t.Fatal("saved resume replenished the partially elapsed tick")
			}
			after := view(t, m, resumed, c.Actor)
			if resumed.Paused || after.Epoch == v.Epoch || after.X != v.X || after.Y != v.Y || !bytes.Equal(frozen.Data, saved) {
				t.Fatal("restore changed progress or retained input epoch")
			}
			if _, e = m.Apply(c, resumed, raw(pal.Action{Type: "input", Epoch: v.Epoch, Seq: 2, Direction: 0})); e == nil {
				t.Fatal("pre-save control entered resumed campaign")
			}
			c.Now = time.UnixMilli(resumed.SystemDeadline)
			next, e := m.OnTimer(c, resumed)
			if e != nil {
				t.Fatal(e)
			}
			after = view(t, m, next, c.Actor)
			if after.X != v.X || after.Y != v.Y {
				t.Fatal("restored held key moved hero")
			}
			report, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": true, "seats": n, "originalMessages": native.View.Messages, "originalPages": seen, "scene": after.Scene, "map": after.Map, "savedBytes": len(saved), "scope": "actual original GameMain through inn, all-seat original dialogue pages, original role count, authorized leader movement, save freeze and released resume; later battle/story not yet covered"}, "", "  ")
			if e = os.WriteFile(filepath.Join("..", "..", "..", ".local", "pal", "verification", fmt.Sprintf("cooperation-%d.json", n)), report, 0600); e != nil {
				t.Fatal(e)
			}
		})
	}
}
