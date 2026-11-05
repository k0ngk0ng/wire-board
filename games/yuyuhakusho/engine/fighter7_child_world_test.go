package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFighter7WorldChildCallbacks(t *testing.T) {
	seen, saved := map[string]int{}, map[string]int{}
	walkChuActor(t, "fighter7-children-boundaries", func(d *json.Decoder, frames AnimationLibrary) {
		var row fighter7ChildrenCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		m := decodeFighter7Children(t, row.Before)
		s := m.Base.State
		var raw []byte
		if saved[row.Kind] < 8 {
			var err error
			raw, err = json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
		}
		events, err := stepChuWorldObject(&s, s.Objects[row.ObjectID], ChuWorldView{}, ChuWorldLibrary{Animations: frames}, new(uint16))
		if err != nil {
			t.Fatal("source7 child world dispatch", row.Name, err)
		}
		draw, clear := 0, uint16(0)
		for _, event := range events {
			if event.Kind == "draw" {
				draw++
			}
			if event.Kind == "clear" {
				clear = event.ClearedBytes
			}
		}
		if (draw == 1) != row.Draw || draw > 1 || clear != row.ClearedBytes {
			t.Fatal("source7 world child outcome differs", row.Name, draw, clear)
		}
		if row.Draw && (row.DrawRequest == nil || s.Objects[row.ObjectID].DrawRequest() != *row.DrawRequest) {
			t.Fatal("source7 callback-time draw differs", row.Name)
		}
		m.Base.State = s
		compareFighter7Children(t, m.encode(t, map[uint16]uint16{row.ObjectID: clear}), row.After)
		if raw != nil {
			var other ChuWorldState
			if err := json.Unmarshal(raw, &other); err != nil {
				t.Fatal(err)
			}
			replay, err := stepChuWorldObject(&other, other.Objects[row.ObjectID], ChuWorldView{}, ChuWorldLibrary{Animations: frames}, new(uint16))
			if err != nil || !reflect.DeepEqual(s, other) || !reflect.DeepEqual(events, replay) {
				t.Fatal("source7 child world resume differs", row.Name, err)
			}
			saved[row.Kind]++
		}
		seen[row.Kind]++
	})
	if seen["first"] != 4321 || seen["second"] != 6497 || seen["pan"] != 505 || saved["first"] != 8 || saved["second"] != 8 || saved["pan"] != 8 {
		t.Fatal("source7 world child coverage lost", seen, saved)
	}
	t.Logf("source7 original child world=%v restores=%v", seen, saved)
}
