package engine

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
)

func TestFighter4CanonicalChildViews(t *testing.T) {
	objects := make(WorldObjects)
	for i := 0; i < 4; i++ {
		id := uint16(0x8400 + i*256)
		o, _ := denseWorldObjectTest(t, id, i+43)
		objects[id] = o
	}
	before := cloneWorldObjects(objects)
	f, err := BuildChuActorFighter(objects, 0x8400)
	if err != nil || f.MovementSource != objects[0x8400].MovementSource {
		t.Fatal("parent movement source lost", err)
	}
	for i, extra := range f.ChildExtras {
		o := objects[0x8500+uint16(i)*256]
		if extra.Direction != o.Aux48 || extra.Held != uint8(o.Controls.Held) || extra.Pressed != uint8(o.Controls.Pressed) || extra.ContactFlags != o.ContactFlags || extra.DragonBank != o.DragonBank {
			t.Fatalf("child%d view is not its canonical state", i)
		}
		f.ChildExtras[i] = Fighter4ChildExtra{}
	}
	f.MovementSource = 0
	merged, err := MergeWorldChuActor(objects, f)
	if err != nil || !reflect.DeepEqual(objects, before) || merged[0x8400].MovementSource != 0 {
		t.Fatal("merge changed input or retained stale movement", err)
	}
	for i := 0; i < 3; i++ {
		id := uint16(0x8500 + i*256)
		want := before[id]
		want.Aux48, want.ContactFlags, want.DragonBank = 0, 0, 0
		want.Controls.Held &= 0xff00
		want.Controls.Pressed &= 0xff00
		if !reflect.DeepEqual(merged[id], want) {
			t.Fatalf("child%d narrow merge lost unrelated fields or retained zero", i)
		}
	}
}

func TestFighter4CanonicalActionView(t *testing.T) {
	f := ChuActorFighter{ObjectID: 0x8400, MovementSource: 0x15946,
		ChildExtras: [3]Fighter4ChildExtra{{Direction: 0x1234abcd, Held: 8, Pressed: 0x40, ContactFlags: 0x10, DragonBank: 0x600}, {Direction: -234567}, {DragonBank: 0x200}}}
	s := ChuActorState{Fighter: f, World: ChuActorWorld{DragonBankCursor: 0x600}}
	a, err := s.actorAction(ChuActorView{})
	if err != nil || a.MovementSource != f.MovementSource || a.DragonBankCursor != 0x600 || a.ChildExtras != f.ChildExtras {
		t.Fatal("actor to action lost canonical views", err)
	}
	a.MovementSource, a.DragonBankCursor, a.ChildExtras = 0, 0, [3]Fighter4ChildExtra{}
	s.acceptActorAction(a)
	if s.Fighter.MovementSource != 0 || s.World.DragonBankCursor != 0 || s.Fighter.ChildExtras != a.ChildExtras {
		t.Fatal("action to actor retained stale fields")
	}
}

func TestFighter4CanonicalObjectCodec(t *testing.T) {
	o, raw := denseWorldObjectTest(t, 0x8500, 71)
	b, _ := hex.DecodeString(raw)
	if o.DragonBank != binary.BigEndian.Uint16(b[0xfe:]) {
		t.Fatal("FE is absent from typed storage")
	}
	o.DragonBank = 0
	encoded := encodeWorldObjectTest(t, o, raw, 0)
	b[0xfe], b[0xff] = 0, 0
	if encoded != hex.EncodeToString(b) {
		t.Fatal("FE codec kept previous bytes or changed neighbors")
	}
	for _, size := range []uint16{224, 240, 256} {
		dirty, _ := denseWorldObjectTest(t, 0x8500, 71)
		got, err := ApplyObjectClearPrefix(dirty, size)
		want := dirty.DragonBank
		if size == 256 {
			want = 0
		}
		if err != nil || got.DragonBank != want {
			t.Fatalf("clear%d FE=%x want%x: %v", size, got.DragonBank, want, err)
		}
	}
}

func TestFighter4CanonicalActionCodec(t *testing.T) {
	f, _, _, _ := readChuAction(t, "chu-action-oracle")
	var rows []chuActionCall
	if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) == 0 {
		t.Fatal("missing original action fixture", err)
	}
	raw := rows[0].Before
	raw.GlobalA038 = 0x600
	a := decodeChuAction(t, raw, rows[0].ObjectID)
	if a.DragonBankCursor != 0x600 || !reflect.DeepEqual(encodeChuAction(t, a, raw), raw) {
		t.Fatal("original action view does not roundtrip")
	}
	a.MovementSource, a.DragonBankCursor, a.ChildExtras = 0, 0, [3]Fighter4ChildExtra{}
	got := encodeChuAction(t, a, raw)
	b, _ := hex.DecodeString(raw.Tail.ActorBytes)
	binary.BigEndian.PutUint32(b[0x90:], 0)
	children, _ := hex.DecodeString(raw.Children)
	for i := 0; i < 3; i++ {
		c := children[i*256:]
		binary.BigEndian.PutUint32(c[0x48:], 0)
		c[0x71], c[0x72], c[0x89], c[0xfe], c[0xff] = 0, 0, 0, 0, 0
	}
	if got.GlobalA038 != 0 || got.Tail.ActorBytes != hex.EncodeToString(b) || got.Children != hex.EncodeToString(children) {
		t.Fatal("action codec reused previous canonical bytes")
	}
}
