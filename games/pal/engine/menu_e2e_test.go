package engine_test

// Declared before field-menu integration. Failure cases: the actual main loop
// traps on its menu key; default status coordinates remain zero; FBP/equipment
// images are lost; status/caster/inventory selection is simulated outside C;
// a saved menu loses C locals, cursor, or pending script; physical input release
// closes a menu; system settings access local save slots or host files; menu
// input moves the party, spends money, or changes unrelated character stats.
import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func originalInn(t *testing.T, r *engine.Runtime) engine.Snapshot {
	t.Helper()
	s, e := r.StartCampaign(context.Background(), 12345)
	if e != nil {
		t.Fatal(e)
	}
	wait := uint32(s.Event.Value)
	for i := 0; i < 4096; i++ {
		if s.Event.Kind == engine.Frame && s.View.Scene == 2 && s.View.Mode == engine.FieldMode && !s.View.Entering && s.View.Messages == 72 {
			return s
		}
		if s.Event.Kind == engine.Dialogue {
			s, e = r.Step(context.Background(), s, 1)
			wait = uint32(s.Event.Value)
		} else {
			var next engine.TimedResult
			next, e = r.AdvanceTimed(context.Background(), s, engine.Input{Direction: 4}, wait, 250)
			s, wait = next.Snapshot, next.RemainingMS
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	t.Fatal("original inn did not become playable")
	return s
}
func menuStep(t *testing.T, r *engine.Runtime, s engine.Snapshot, keys uint32) engine.Snapshot {
	t.Helper()
	next, e := r.StepWithInput(context.Background(), s, engine.Input{Direction: 4, Keys: keys})
	if e != nil {
		t.Fatal(e)
	}
	return next
}
func menuUntil(t *testing.T, r *engine.Runtime, s engine.Snapshot, kind int) engine.Snapshot {
	t.Helper()
	for i := 0; i < 64; i++ {
		if s.View.Menu.Kind == kind {
			return s
		}
		s = menuStep(t, r, s, 0)
	}
	t.Fatal("original menu did not reach target", kind, s.View.Menu)
	return s
}
func TestOriginalFieldMenusSaveAndSystemBoundary(t *testing.T) {
	r := nativeRuntime(t)
	inn := originalInn(t, r)
	open := func() engine.Snapshot { return menuUntil(t, r, menuStep(t, r, inn, 1), 1) }
	main := open()
	if main.View.Mode != engine.MenuMode || main.View.Menu.Selected != 1 || len(main.View.Menu.Items) != 4 {
		t.Fatal("original main menu missing", main.View.Menu)
	}
	want := []int{3, 4, 5, 6}
	for i, item := range main.View.Menu.Items {
		if item.Word != want[i] || !item.Enabled || item.Label == "" {
			t.Fatal("original main labels changed", main.View.Menu)
		}
	}
	status := menuUntil(t, r, menuStep(t, r, main, 2), 2)
	background, name := false, false
	for _, op := range status.View.Overlay {
		if op.Kind == 1 && op.Source == 4 && op.ID == 0 {
			background = true
		}
		if op.Kind == 0 && op.ID == 0x674e && op.X == 110 && op.Y == 8 && op.Color == 0x2c {
			name = true
		}
	}
	if !background || !name {
		t.Fatal("original status background/layout missing", background, name)
	}
	encoded, e := r.Encode(status)
	if e != nil {
		t.Fatal(e)
	}
	fresh := nativeRuntime(t)
	restored, e := fresh.Decode(encoded)
	if e != nil {
		t.Fatal(e)
	}
	restored, e = fresh.ReleaseInputs(context.Background(), restored)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(restored.View.Menu, status.View.Menu) {
		t.Fatal("released saved menu lost its cursor")
	}
	a := menuUntil(t, r, menuStep(t, r, status, 1), 0)
	b := menuUntil(t, fresh, menuStep(t, fresh, restored, 1), 0)
	if a.View.X != inn.View.X || a.View.Y != inn.View.Y || a.View.HP != inn.View.HP || a.View.MP != inn.View.MP || a.View.Cash != inn.View.Cash || !reflect.DeepEqual(a.View, b.View) {
		t.Fatal("status/save/close changed party progress")
	}
	main = open()
	main = menuStep(t, r, main, 4)
	main = menuStep(t, r, main, 4)
	if main.View.Menu.Selected != 3 {
		t.Fatal("original inventory cursor did not move twice", main.View.Menu)
	}
	inventory := menuUntil(t, r, menuStep(t, r, main, 2), 4)
	if len(inventory.View.Menu.Items) != 2 || inventory.View.Menu.Items[0].Word != 22 || inventory.View.Menu.Items[1].Word != 23 {
		t.Fatal("original inventory entries changed", inventory.View.Menu)
	}
	menuUntil(t, r, menuStep(t, r, inventory, 1), 0)
	main = open()
	main = menuStep(t, r, main, 4)
	magic := menuUntil(t, r, menuStep(t, r, main, 2), 10)
	if magic.View.Menu.Role != 0 {
		t.Fatal("single original party member did not open own magic list", magic.View.Menu)
	}
	menuUntil(t, r, menuStep(t, r, magic, 1), 0)
	main = open()
	for i := 0; i < 3; i++ {
		main = menuStep(t, r, main, 4)
	}
	system := menuStep(t, r, main, 2)
	if system.Event.Kind != engine.SystemMenu || system.View.Menu.Kind != 11 {
		t.Fatal("system menu did not hand off to platform controls", system.Event, system.View.Menu)
	}
	still, e := r.Step(context.Background(), system, 0)
	if e != nil || still.Event.Kind != engine.SystemMenu {
		t.Fatal("system boundary advanced without close", e)
	}
	closed, e := r.Step(context.Background(), system, 1)
	if e != nil {
		t.Fatal(e)
	}
	closed = menuUntil(t, r, closed, 0)
	if closed.View.Cash != inn.View.Cash || closed.View.X != inn.View.X || closed.View.Y != inn.View.Y {
		t.Fatal("platform settings changed original progress")
	}
	report, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": true, "scope": "actual original GameMain -> main/status/caster/inventory menus; original status layout and FBP; menu save/release/fresh instance; platform system boundary without host file I/O", "statusSaveBytes": len(encoded), "main": main.View.Menu, "status": status.View.Menu, "inventory": inventory.View.Menu, "magic": magic.View.Menu}, "", "  ")
	if e = os.WriteFile(filepath.Join("..", "..", "..", ".local", "pal", "verification", "native-field-menus.json"), report, 0600); e != nil {
		t.Fatal(e)
	}
}
