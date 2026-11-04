package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

type effectMotionSnapshot struct {
	ActorBytes string            `json:"actor_bytes"`
	PoolBytes  string            `json:"pool_bytes"`
	Owners     map[uint16]string `json:"owner_bytes"`
	World      EffectMotionWorld
}

func effectMotionBytes(t *testing.T, encoded string, size int) []byte {
	t.Helper()
	b, err := hex.DecodeString(encoded)
	if err != nil || len(b) != size {
		t.Fatalf("invalid original FX bytes: %v size=%d/%d", err, len(b), size)
	}
	return b
}

func decodeEffectMotion(t *testing.T, raw effectMotionSnapshot, id uint16) (EffectMotionState, map[uint16]EffectMotionOwner) {
	t.Helper()
	b := effectMotionBytes(t, raw.ActorBytes, 128)
	w := func(i int) uint16 { return binary.BigEndian.Uint16(b[i:]) }
	l := func(i int) uint32 { return binary.BigEndian.Uint32(b[i:]) }
	s := EffectMotionState{ObjectID: id, Slot: EffectSlot{Behavior: l(0), Size: w(4), Stage: w(6), VelocityX: int32(l(0x10)), Attributes: w(0x24), Layer: b[0x26], Depth: w(0x3e), X: int32(l(0x40)), Y: int32(l(0x44)), Type: b[0x58]},
		VerticalVelocity: int32(l(0x14)), GraphicSet: l(0x1c), TileBase: l(0x20), Sequence: l(0x50), GraphicOffsetX: int16(w(0x28)), GraphicOffsetY: int16(w(0x2a)), GraphicParameter: w(0x3c),
		SkipTiles: b[0x27], FrameIndex: b[0x59], Delay: b[0x5a], Ticks: b[0x5b], Flags: b[0x5c], FollowOwner: w(0x48),
		ScreenX: int16(w(8)), ScreenY: int16(w(12)), ScreenOffsetX: int16(w(0x54)), ScreenOffsetY: int16(w(0x56))}
	for i := range s.GraphicCache {
		s.GraphicCache[i] = l(0x2c + i*4)
	}
	owners := make(map[uint16]EffectMotionOwner, len(raw.Owners))
	for id, encoded := range raw.Owners {
		a := effectMotionBytes(t, encoded, 128)
		owners[id] = EffectMotionOwner{X: int32(binary.BigEndian.Uint32(a[0x40:])), Y: int32(binary.BigEndian.Uint32(a[0x44:]))}
	}
	pool := effectMotionBytes(t, raw.PoolBytes, 2048)
	index := int(id) - 0x9400
	if index < 0 || index+128 > len(pool) || index&127 != 0 || !bytes.Equal(pool[index:index+128], b) {
		t.Fatal("original FX slot/pool identity mismatch")
	}
	return s, owners
}

func encodeEffectMotion(t *testing.T, s EffectMotionState, world EffectMotionWorld, owners map[uint16]EffectMotionOwner, result EffectMotionOutcome, old effectMotionSnapshot) effectMotionSnapshot {
	t.Helper()
	raw := old
	b := effectMotionBytes(t, old.ActorBytes, 128)
	if result.Deleted {
		if result.ClearedBytes == 0 || result.ClearedBytes > 128 || result.ClearedBytes&15 != 0 {
			t.Fatal("invalid FX clear prefix")
		}
		clear(b[:result.ClearedBytes])
	}
	w := func(i int, v uint16) { binary.BigEndian.PutUint16(b[i:], v) }
	l := func(i int, v uint32) { binary.BigEndian.PutUint32(b[i:], v) }
	l(0, s.Slot.Behavior)
	w(4, s.Slot.Size)
	w(6, s.Slot.Stage)
	w(8, uint16(s.ScreenX))
	w(12, uint16(s.ScreenY))
	l(0x10, uint32(s.Slot.VelocityX))
	l(0x14, uint32(s.VerticalVelocity))
	l(0x1c, s.GraphicSet)
	l(0x20, s.TileBase)
	w(0x24, s.Slot.Attributes)
	b[0x26], b[0x27] = s.Slot.Layer, s.SkipTiles
	w(0x28, uint16(s.GraphicOffsetX))
	w(0x2a, uint16(s.GraphicOffsetY))
	for i, value := range s.GraphicCache {
		l(0x2c+i*4, value)
	}
	w(0x3c, s.GraphicParameter)
	w(0x3e, s.Slot.Depth)
	l(0x40, uint32(s.Slot.X))
	l(0x44, uint32(s.Slot.Y))
	w(0x48, s.FollowOwner)
	l(0x50, s.Sequence)
	w(0x54, uint16(s.ScreenOffsetX))
	w(0x56, uint16(s.ScreenOffsetY))
	b[0x58], b[0x59], b[0x5a], b[0x5b], b[0x5c] = s.Slot.Type, s.FrameIndex, s.Delay, s.Ticks, s.Flags
	raw.ActorBytes = hex.EncodeToString(b)
	pool := effectMotionBytes(t, old.PoolBytes, 2048)
	index := int(s.ObjectID) - 0x9400
	copy(pool[index:index+128], b)
	raw.PoolBytes = hex.EncodeToString(pool)
	raw.World = world
	raw.Owners = make(map[uint16]string, len(old.Owners))
	for id, oldBytes := range old.Owners {
		owner, ok := owners[id]
		if !ok {
			t.Fatal("owner input disappeared")
		}
		a := effectMotionBytes(t, oldBytes, 128)
		binary.BigEndian.PutUint32(a[0x40:], uint32(owner.X))
		binary.BigEndian.PutUint32(a[0x44:], uint32(owner.Y))
		raw.Owners[id] = hex.EncodeToString(a)
	}
	return raw
}

type effectMotionCall struct {
	Name          string
	ObjectID      uint16 `json:"object_id"`
	Before, After effectMotionSnapshot
	Draw, Deleted bool
	ClearedBytes  uint16 `json:"cleared_bytes"`
	External      struct {
		Owner *uint16
		X, Y  int32
	} `json:"external_input"`
}

type effectMotionFixture struct {
	Library EffectMotionLibrary
	Calls   json.RawMessage
}

func readEffectMotion(t *testing.T, name string) (effectMotionFixture, int, int) {
	t.Helper()
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	mb, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass      bool
		Records   int
		Calls     int
		Types     int
		Sequences int
		External  int    `json:"external_queue_calls"`
		Baseline  bool   `json:"frames_match_original_core"`
		SHA       string `json:"fixture_sha256"`
		Gzip      string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !m.Pass || !m.Baseline || m.External != 0 || m.Records == 0 || m.Types != 37 || m.Sequences != 21 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified generic FX original fixture")
	}
	var f effectMotionFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Library.Types) != 37 || len(f.Library.Sequences) != 21 {
		t.Fatal("missing FX type/sequence data")
	}
	return f, m.Records, m.Calls
}

// Complete original rules fixtures and these tests precede effect_motion.go.
func TestEffectMotionOriginalRules(t *testing.T) {
	for _, name := range []string{"effect-motion-oracle", "effect-motion-boundaries"} {
		t.Run(name, func(t *testing.T) {
			f, count, _ := readEffectMotion(t, name)
			var rows []effectMotionCall
			if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != count {
				t.Fatal("missing original FX calls", err)
			}
			beforeLibrary, _ := json.Marshal(f.Library)
			deleted, direct := 0, 0
			for i, r := range rows {
				input, owners := decodeEffectMotion(t, r.Before, r.ObjectID)
				originalOwners, _ := json.Marshal(owners)
				got, world, result, err := StepEffectMotion(input, r.Before.World, owners, f.Library)
				want, _ := decodeEffectMotion(t, r.After, r.ObjectID)
				if err != nil || result != (EffectMotionOutcome{Draw: r.Draw, Deleted: r.Deleted, ClearedBytes: r.ClearedBytes}) || got != want || world != r.After.World || !reflect.DeepEqual(encodeEffectMotion(t, got, world, owners, result, r.Before), r.After) {
					t.Fatalf("FX %d %s differs: %v outcome=%+v", i, r.Name, err, result)
				}
				afterOwners, _ := json.Marshal(owners)
				if !bytes.Equal(originalOwners, afterOwners) {
					t.Fatal("FX changed caller owner data")
				}
				if r.Deleted {
					deleted++
				} else if !r.Draw {
					direct++
				}
			}
			afterLibrary, _ := json.Marshal(f.Library)
			if !bytes.Equal(beforeLibrary, afterLibrary) || deleted == 0 || direct == 0 {
				t.Fatal("FX library mutated or missing deletion/direct return coverage")
			}
			t.Logf("original FX calls=%d deletion=%d no-draw returns=%d", count, deleted, direct)
		})
	}
}

func TestEffectMotionSingleInitialLifetimes(t *testing.T) {
	f, count, calls := readEffectMotion(t, "effect-motion-sequences")
	var rows []struct {
		Initial effectMotionSnapshot
		Calls   []effectMotionCall
	}
	if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != count || count != 111 {
		t.Fatal("missing complete FX lifetimes", err)
	}
	steps := 0
	for _, r := range rows {
		if len(r.Calls) == 0 || !r.Calls[len(r.Calls)-1].Deleted {
			t.Fatal("FX lifetime did not reach actual deletion")
		}
		s, owners := decodeEffectMotion(t, r.Initial, r.Calls[0].ObjectID)
		world := r.Initial.World
		encoded := r.Initial
		for i, call := range r.Calls {
			if call.External.Owner != nil {
				owners[*call.External.Owner] = EffectMotionOwner{X: call.External.X, Y: call.External.Y}
			}
			var result EffectMotionOutcome
			var err error
			s, world, result, err = StepEffectMotion(s, world, owners, f.Library)
			encoded = encodeEffectMotion(t, s, world, owners, result, encoded)
			want, _ := decodeEffectMotion(t, call.After, call.ObjectID)
			if err != nil || result != (EffectMotionOutcome{Draw: call.Draw, Deleted: call.Deleted, ClearedBytes: call.ClearedBytes}) || s != want || world != call.After.World || !reflect.DeepEqual(encoded, call.After) {
				t.Fatalf("FX lifetime step%d differs: %v", i, err)
			}
			steps++
		}
	}
	if steps != calls || steps != 2700 {
		t.Fatal("missing complete FX steps")
	}
}

func TestEffectMotionErrorsAreAtomic(t *testing.T) {
	f, _, _ := readEffectMotion(t, "effect-motion-boundaries")
	var rows []effectMotionCall
	if err := json.Unmarshal(f.Calls, &rows); err != nil {
		t.Fatal(err)
	}
	base, owners := decodeEffectMotion(t, rows[0].Before, rows[0].ObjectID)
	world := rows[0].Before.World
	got, afterWorld, result, err := StepEffectMotion(base, world, owners, EffectMotionLibrary{})
	if err == nil || got != base || afterWorld != world || result != (EffectMotionOutcome{}) {
		t.Fatal("missing initializer leaked state/global writes")
	}
	base.Slot.Stage, base.Delay, base.Ticks, base.FrameIndex = 2, 0, 1, 255
	got, afterWorld, result, err = StepEffectMotion(base, world, owners, EffectMotionLibrary{})
	if err == nil || got != base || afterWorld != world || result != (EffectMotionOutcome{}) {
		t.Fatal("missing sequence leaked clock writes")
	}
	base.Ticks, base.FollowOwner = 2, 0x8400
	got, afterWorld, result, err = StepEffectMotion(base, world, nil, f.Library)
	if err == nil || got != base || afterWorld != world || result != (EffectMotionOutcome{}) {
		t.Fatal("missing owner leaked earlier clock writes")
	}
	base.Ticks, base.FrameIndex, base.Slot.Size = 1, uint8(len(f.Library.Sequences[base.Sequence])-2), 1
	got, afterWorld, result, err = StepEffectMotion(base, world, owners, f.Library)
	if err == nil || got != base || afterWorld != world || result != (EffectMotionOutcome{}) {
		t.Fatal("corrupt deletion size was aliased or partially cleared")
	}
	world.PauseFlags = 0x80
	base.Delay = 255
	got, afterWorld, result, err = StepEffectMotion(base, world, nil, EffectMotionLibrary{})
	if err != nil || got != base || afterWorld != world || !result.Draw || result.Deleted {
		t.Fatal("pause accessed unused data or decremented delay")
	}
}
