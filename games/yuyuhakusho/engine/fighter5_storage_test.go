package engine

import (
	"reflect"
	"testing"
)

func TestFighter5CanonicalChildViews(t *testing.T) {
	objects := make(WorldObjects)
	for i := 0; i < 4; i++ {
		id := uint16(0x8400 + i*256)
		o, _ := denseWorldObjectTest(t, id, 90+i)
		if i == 0 {
			o.Tag = 5
		}
		objects[id] = o
	}
	before := cloneWorldObjects(objects)
	f, err := BuildChuActorFighter(objects, 0x8400)
	if err != nil {
		t.Fatal(err)
	}
	for i, extra := range f.Fighter5ChildExtras {
		o := objects[0x8500+uint16(i)*256]
		if extra.Gravity != o.Body.Gravity || extra.Owner != o.DragonBank || f.ChildExtras[i].DragonBank != 0 {
			t.Fatal("source5 child word/long views lost or duplicated", i)
		}
		f.Fighter5ChildExtras[i] = Fighter5ChildExtra{}
	}
	merged, err := MergeWorldChuActor(objects, f)
	if err != nil || !reflect.DeepEqual(objects, before) {
		t.Fatal("source5 merge changed caller", err)
	}
	for i := 0; i < 3; i++ {
		id := uint16(0x8500 + i*256)
		want := before[id]
		want.Body.Gravity, want.DragonBank = 0, 0
		if !reflect.DeepEqual(merged[id], want) {
			t.Fatal("source5 narrow merge lost unrelated fields or retained stale zero", i)
		}
	}
}

func TestFighter5CanonicalActionViews(t *testing.T) {
	f := ChuActorFighter{ObjectID: 0x8400, Fighter5ChildExtras: [3]Fighter5ChildExtra{{Gravity: 0x12345678, Owner: 0x8400}, {Gravity: -1234567, Owner: 0x8800}, {Owner: 0x9000}}}
	s := ChuActorState{Fighter: f}
	a, err := s.actorAction(ChuActorView{})
	if err != nil || a.Fighter5ChildExtras != f.Fighter5ChildExtras {
		t.Fatal("source5 actor to action lost child fields", err)
	}
	a.Fighter5ChildExtras = [3]Fighter5ChildExtra{}
	s.acceptActorAction(a)
	if s.Fighter.Fighter5ChildExtras != a.Fighter5ChildExtras {
		t.Fatal("source5 action to actor retained stale fields")
	}
}
