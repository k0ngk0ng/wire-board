package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Full original callback records exercise the actual world dispatcher before
// source4 callbacks are wired into it. The fixture adapter never executes rules.
func TestFighter4DragonWorldCallbacks(t *testing.T) {
	seen, resumed := map[string]int{}, map[string]int{}
	walkFighter4Dragon(t, "boundaries", func(d *json.Decoder, dragon Fighter4DragonLibrary) {
		var row fighter4DragonCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		m := decodeFighter4Dragon(t, row.Before)
		s := m.Base.State
		s.DragonHistory, s.DragonBankCursor = m.History, m.BankCursor
		library := ChuWorldLibrary{Animations: dragon.Animations, Fighter4Dragon: dragon}
		var saved []byte
		if resumed[row.Kind] < 8 {
			var err error
			saved, err = json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
		}
		cursor := uint16(0)
		events, err := stepChuWorldObject(&s, s.Objects[row.ObjectID], ChuWorldView{}, library, &cursor)
		if err != nil {
			t.Fatal("source4 dragon world dispatcher", row.Name, err)
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
			t.Fatal("world dragon callback emissions differ", row.Name, draw, clear)
		}
		if row.Draw && s.Objects[row.ObjectID].DrawRequest() != row.DrawRequest {
			t.Fatal("world dragon callback-time draw differs", row.Name)
		}
		m.Base.State, m.History, m.BankCursor = s, s.DragonHistory, s.DragonBankCursor
		compareFighter4Dragon(t, m.encode(t, map[uint16]uint16{row.ObjectID: clear}), row.After)
		if saved != nil {
			var restored ChuWorldState
			if err := json.Unmarshal(saved, &restored); err != nil {
				t.Fatal(err)
			}
			other, err := stepChuWorldObject(&restored, restored.Objects[row.ObjectID], ChuWorldView{}, library, new(uint16))
			if err != nil || !reflect.DeepEqual(s, restored) || !reflect.DeepEqual(events, other) {
				t.Fatal("world dragon canonical restore diverged", row.Name, err)
			}
			resumed[row.Kind]++
		}
		seen[row.Kind]++
	})
	if seen["head"] != 923 || seen["trail"] != 1098 || seen["boost"] != 586 || resumed["head"] != 8 || resumed["trail"] != 8 || resumed["boost"] != 8 {
		t.Fatal("world dragon coverage lost", seen, resumed)
	}
	t.Logf("world dragon original callbacks=%v restored=%v", seen, resumed)
}
