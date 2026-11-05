package engine

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestFighter5WorldChildCallbacks(t *testing.T) {
	seen, saved := map[string]int{}, map[string]int{}
	walkChuActor(t, "fighter5-projectile-boundaries", func(d *json.Decoder, animations AnimationLibrary) {
		var row fighter5ProjectileCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		// The adjacent slave continuations have separate isolated evidence;
		// the actual world dispatches only its stored primary Behavior.
		if strings.HasSuffix(row.Kind, "_stop") {
			return
		}
		m := decodeFighter5Projectile(t, row.Before)
		s := m.State
		var raw []byte
		if saved[row.Kind] < 8 {
			var err error
			raw, err = json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
		}
		events, err := stepChuWorldObject(&s, s.Objects[row.ObjectID], ChuWorldView{}, ChuWorldLibrary{Animations: animations}, new(uint16))
		if err != nil {
			t.Fatal("source5 child world dispatch", row.Name, err)
		}
		draw, clear := 0, uint16(0)
		for _, e := range events {
			if e.Kind == "draw" {
				draw++
			}
			if e.Kind == "clear" {
				clear = e.ClearedBytes
			}
		}
		if (draw == 1) != row.Draw || draw > 1 || clear != row.ClearedBytes {
			t.Fatal("source5 world child outcome differs", row.Name, draw, clear)
		}
		if row.Draw && (row.DrawRequest == nil || s.Objects[row.ObjectID].DrawRequest() != *row.DrawRequest) {
			t.Fatal("source5 callback-time draw value differs", row.Name)
		}
		m.State = s
		compareFighter3AirRaw(t, m.encode(t, map[uint16]uint16{row.ObjectID: clear}), row.After)
		if raw != nil {
			var restored ChuWorldState
			if err := json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			other, err := stepChuWorldObject(&restored, restored.Objects[row.ObjectID], ChuWorldView{}, ChuWorldLibrary{Animations: animations}, new(uint16))
			if err != nil || !reflect.DeepEqual(restored, s) || !reflect.DeepEqual(other, events) {
				t.Fatal("source5 world child save/restore differs", row.Name, err)
			}
			saved[row.Kind]++
		}
		seen[row.Kind]++
	})
	if seen["linear"] != 2763 || seen["steered"] != 2833 || seen["follower"] != 639 || saved["linear"] != 8 || saved["steered"] != 8 || saved["follower"] != 8 {
		t.Fatal("source5 world child coverage lost", seen, saved)
	}
	t.Logf("source5 child world original=%v restores=%v", seen, saved)
}
