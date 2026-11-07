package contra

// Before server/client resource identity: one stable bundle binds all stages,
// while each client may fetch only its current payload. Reject a changed
// manifest, substituted stage or modified bytes; callers cannot mutate assets.
import "testing"

func TestBuiltinLevelBundleIdentityAndStageBinding(t *testing.T) {
	b, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	m := b.Manifest()
	if e = m.Validate(); e != nil {
		t.Fatal(e)
	}
	if len(m.ID) != 64 {
		t.Fatal("missing stable bundle id")
	}
	for level := uint8(0); level < 8; level++ {
		raw, e := b.Payload(level)
		if e != nil {
			t.Fatal(e)
		}
		r, e := m.DecodeStage(level, raw)
		if e != nil {
			t.Fatal(level, e)
		}
		if _, e = r.Stage(int(level)); e != nil {
			t.Fatal(e)
		}
		if _, e = m.DecodeStage((level+1)%8, raw); e == nil {
			t.Fatal("other stage substituted")
		}
		raw[0] ^= 1
		if _, e = m.DecodeStage(level, raw); e == nil {
			t.Fatal("changed bytes accepted")
		}
		fresh, _ := b.Payload(level)
		if fresh[0] == raw[0] {
			t.Fatal("caller mutated embedded asset")
		}
	}
	if _, e = b.Payload(8); e == nil {
		t.Fatal("invalid stage accepted")
	}
	m.Stages[0].Digest = m.Stages[1].Digest
	if e = m.Validate(); e == nil {
		t.Fatal("manifest mismatch accepted")
	}
}
