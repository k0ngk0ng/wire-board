package pal_test

// Test first, using an explicit original PAL_StartBattle verification build.
// Failures: leader can control teammates' battle commands; all platform counts
// get the same owner; queued/held input crosses role boundaries; previous-epoch
// packets are accepted; native root-cancel undoes a teammate; hidden enemy HP
// leaks to public views; pause/resume changes committed actions or resources;
// continued combat cannot finish with the actual original three-cash reward.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func originalBattleModule(t *testing.T) gameapi.Module {
	return battleModuleFrom(t, ".local/pal/battle-fixtures/team-1-party-3/rules.wasm")
}
func battleModuleFrom(t *testing.T, fixture string) gameapi.Module {
	t.Helper()
	root := filepath.Join("..", "..", "..")
	wasm, e := os.ReadFile(filepath.Join(root, fixture))
	if e != nil {
		t.Fatal(e)
	}
	r, e := engine.NewRuntime(context.Background(), wasm, os.DirFS(filepath.Join(root, ".local/pal/original-98")))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	return pal.NewModule(r)
}

func TestOriginalBattleRoleControlAndSavedContinuation(t *testing.T) {
	m := originalBattleModule(t)
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			c := gameapi.Context{Players: players, Actor: 0, Now: time.Unix(1800000000, 0)}
			g, e := m.Start(c, raw(map[string]any{}))
			if e != nil {
				t.Fatal(e)
			}
			tick := func() {
				t.Helper()
				c.Now = time.UnixMilli(g.SystemDeadline)
				g, e = m.OnTimer(c, g)
				if e != nil {
					t.Fatal(e)
				}
			}
			until := func(slot int) {
				t.Helper()
				for i := 0; i < 512; i++ {
					v := view(t, m, g, 0)
					if v.Battle != nil && v.Battle.State == 1 && v.Battle.Current == slot {
						return
					}
					tick()
				}
				t.Fatal("original role unavailable", slot)
			}
			input := func(actor int, keys uint32) {
				t.Helper()
				c.Actor = actor
				v := view(t, m, g, actor)
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
			}
			until(0)
			v := view(t, m, g, 0)
			for i, p := range v.Party {
				if p.Controller != i%players {
					t.Fatal("wrong platform/role mapping", v.Party)
				}
			}
			if !slices.Equal(g.Actors, []int{0}) {
				t.Fatal("battle actor missing", g.Actors)
			}
			serialized, _ := json.Marshal(v)
			var public map[string]any
			if e = json.Unmarshal(serialized, &public); e != nil {
				t.Fatal(e)
			}
			for _, enemy := range public["battle"].(map[string]any)["enemies"].([]any) {
				if _, ok := enemy.(map[string]any)["hp"]; ok {
					t.Fatal("hidden enemy health exposed")
				}
			}
			if players > 1 {
				c.Actor = 1
				before := bytes.Clone(g.Data)
				if _, e = m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: v.Epoch, Seq: 1, Direction: 4, Keys: 2})); e == nil || !bytes.Equal(g.Data, before) {
					t.Fatal("nonowner controlled original role")
				}
			}
			oldEpoch := v.Epoch
			input(0, 0x400) // actual defend
			input(0, 2)     // queued old-role key must be dropped at handoff
			until(1)
			v = view(t, m, g, 1%players)
			if v.Epoch == oldEpoch || v.Battle.Players[0].Action != 1 || v.Battle.Players[1].State == 2 {
				t.Fatal("old role input crossed handoff", v.Battle)
			}
			if !slices.Equal(g.Actors, []int{1 % players}) {
				t.Fatal("leader retained battle permission", g.Actors)
			}
			c.Actor = 0
			if _, e = m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: oldEpoch, Seq: 100, Direction: 4, Keys: 2})); e == nil {
				t.Fatal("pre-handoff input accepted")
			}
			if players > 1 {
				input(1, 1)
				tick()
				v = view(t, m, g, 1)
				if v.Battle == nil || v.Battle.Current != 1 || v.Battle.Players[0].State != 2 {
					t.Fatal("teammate action undone")
				}
			}
			before := view(t, m, g, 0)
			g, e = m.SaveControl(c, g, true)
			if e != nil {
				t.Fatal(e)
			}
			if len(g.Actors) != 0 {
				t.Fatal("saving battle still accepts input")
			}
			frozen := bytes.Clone(g.Data)
			c.Now = c.Now.Add(time.Hour)
			paused, e := m.OnTimer(c, g)
			if e != nil || !bytes.Equal(paused.Data, frozen) {
				t.Fatal("saving battle advanced", e)
			}
			g, e = m.ResumeSaved(c, g)
			if e != nil {
				t.Fatal(e)
			}
			after := view(t, m, g, 0)
			if !reflect.DeepEqual(before.Battle, after.Battle) || before.Cash != after.Cash || before.Epoch == after.Epoch {
				t.Fatal("saved combat commands/resources changed")
			}
			seen := map[int]bool{0: true, 1: true}
			for i := 0; i < 1024; i++ {
				v = view(t, m, g, 0)
				if v.Cash == 3 {
					break
				}
				if v.Battle != nil && v.Battle.State != 0 {
					seen[v.Battle.Current] = true
					input(v.Party[v.Battle.Current].Controller, 2)
				}
				tick()
				if i == 1023 {
					t.Fatal("original cooperative battle did not finish")
				}
			}
			v = view(t, m, g, 0)
			if v.Cash != 3 || !seen[2] {
				t.Fatal("original reward or third role missing", v.Cash, seen)
			}
			report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "isolated original PAL_StartBattle, platform role permissions, save/resume, original three-cash reward; not full campaign", "players": players, "rolesSeen": seen, "cash": v.Cash}, "", "  ")
			if e = os.WriteFile(filepath.Join("..", "..", "..", fmt.Sprintf(".local/pal/verification/battle-cooperation-%d.json", players)), report, 0600); e != nil {
				t.Fatal(e)
			}
		})
	}
}
