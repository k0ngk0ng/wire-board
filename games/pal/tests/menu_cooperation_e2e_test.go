package pal_test

// Original solo menu/input regression: an invalid seat drives menus;
// two quick direction edges collapse; releases erase pending edges; stale menu
// input crosses a submenu or saved epoch; cursor/C stack is lost by save/reclaim;
// oversized edge queues are accepted; the platform system boundary advances
// automatically or accepts another seat's close operation.
import (
	"bytes"
	"slices"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalSoloMenusControlAndSave(t *testing.T) {
	m, _ := nativeModule(t)
	c := gameapi.Context{Actor: 0, Players: 1, Now: time.Unix(1800000000, 0)}
	g, e := m.Start(c, raw(map[string]any{}))
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 100; i++ {
		g = timers(t, m, &c, g)
		v := view(t, m, g, 0)
		if v.Phase == "field" && v.Scene == 2 {
			break
		}
		for actor := 0; actor < c.Players; actor++ {
			c.Actor = actor
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
		}
	}
	c.Actor = 0
	until := func(kind, selected int) {
		t.Helper()
		for i := 0; i < 32; i++ {
			v := view(t, m, g, 0)
			if v.Menu.Kind == kind && (selected < 0 || v.Menu.Selected == selected) {
				return
			}
			c.Now = time.UnixMilli(g.SystemDeadline)
			g, e = m.OnTimer(c, g)
			if e != nil {
				t.Fatal(e)
			}
		}
		t.Fatal("original shared menu did not reach target", kind, selected, view(t, m, g, 0))
	}
	input := func(keys uint32) {
		t.Helper()
		v := view(t, m, g, 0)
		g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
	}
	input(1)
	until(1, 1)
	v := view(t, m, g, 0)
	if v.Phase != "menu" {
		t.Fatal("original menu was mistaken for field movement", v.Phase)
	}
	if !slices.Equal(g.Actors, []int{0}) || !slices.Equal(g.Attention, []int{0}) {
		t.Fatal("menu publishes another seat as its controller", g.Actors, g.Attention)
	}
	prior := append([]byte(nil), g.Data...)
	c.Actor = 1
	if _, e = m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: v.Epoch, Seq: 1, Direction: 4, Keys: 4})); e == nil || !bytes.Equal(g.Data, prior) {
		t.Fatal("nonleader controlled shared menu")
	}
	c.Actor = 0
	input(4)
	input(4)
	input(0) // A neutral release must retain both short presses.
	until(1, 2)
	until(1, 3)
	v = view(t, m, g, 0)
	g, e = m.SaveControl(c, g, true)
	if e != nil {
		t.Fatal(e)
	}
	c.Now = c.Now.Add(time.Hour)
	g, e = m.ResumeSaved(c, g)
	if e != nil {
		t.Fatal(e)
	}
	resumed := view(t, m, g, 0)
	if resumed.Menu.Kind != 1 || resumed.Menu.Selected != 3 || resumed.Epoch == v.Epoch {
		t.Fatal("saved menu lost cursor or control epoch")
	}
	if _, e = m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: v.Epoch, Seq: 4, Direction: 4, Keys: 2})); e == nil {
		t.Fatal("pre-save menu input survived resume")
	}
	input(2)
	until(4, 1)
	v = view(t, m, g, 0)
	if len(v.Menu.Items) != 2 || v.Menu.Items[0].Word != 22 || v.Menu.Items[1].Word != 23 {
		t.Fatal("original submenu was not published", v.Menu)
	}
	c.Actor = 1
	prior = bytes.Clone(g.Data)
	if _, e = m.Reclaim(c, g); e == nil || !bytes.Equal(prior, g.Data) {
		t.Fatal("invalid seat reclaimed the personal menu")
	}
	c.Actor = 0
	g, e = m.Reclaim(c, g)
	if e != nil {
		t.Fatal(e)
	}
	if view(t, m, g, 0).Menu.Selected != 1 {
		t.Fatal("personal reclaim reset the original cursor")
	}
	for i := 0; i < 32; i++ {
		input(4)
	}
	v = view(t, m, g, 0)
	prior = append([]byte(nil), g.Data...)
	if _, e = m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 4})); e == nil || !bytes.Equal(prior, g.Data) {
		t.Fatal("input edge queue exceeded bound")
	}
	// Freeze/release clears transport backlog while retaining the original menu.
	g, e = m.SaveControl(c, g, true)
	if e != nil {
		t.Fatal(e)
	}
	g, e = m.SaveControl(c, g, false)
	if e != nil {
		t.Fatal(e)
	}
	input(1)
	until(0, -1)
	input(1)
	until(1, 3)
	input(4)
	until(1, 4)
	input(2)
	until(11, -1)
	v = view(t, m, g, 0)
	if v.Phase != "system" {
		t.Fatal("platform system handoff was not explicit")
	}
	c.Actor = 1
	if _, e = m.Apply(c, g, raw(pal.Action{Type: "system_close", Epoch: v.Epoch})); e == nil {
		t.Fatal("another seat closed leader system controls")
	}
	c.Actor = 0
	g = apply(t, m, c, g, pal.Action{Type: "system_close", Epoch: v.Epoch})
	until(0, -1)
}
