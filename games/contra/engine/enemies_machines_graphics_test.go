package contra

// Failure inventory precedes implementation: wrong source-level table;
// hardcoded 2-row shape; admission at 64 instead of 80 or projected size;
// palette header masking; wrong quadrant/240-pixel wrap; changing caller 06,
// 23, terrain or collision; rejected calls overwriting lookup/queue; aliasing
// mutable animation data between queues.

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalMachineTileAnimationRequests(t *testing.T) {
	raw, err := os.ReadFile("leveldata/levels.json")
	if err != nil {
		t.Fatal(err)
	}
	resources, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"controlled", "natural"} {
		f, err := os.Open("testdata/original-enemy-machines-graphics-" + scope + ".json.gz")
		if err != nil {
			t.Fatal(err)
		}
		z, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		var data struct {
			Original bool `json:"source_is_original_execution"`
			Records  []struct {
				Index             int
				X, Y, Code, Carry uint8
				Before, After     struct {
					graphicsOracleState
					Offset06 uint8
				}
			}
		}
		err = json.NewDecoder(z).Decode(&data)
		z.Close()
		f.Close()
		if err != nil || !data.Original || len(data.Records) == 0 {
			t.Fatal(scope, err)
		}
		accepted, rejected := 0, 0
		for _, r := range data.Records {
			animations, err := OriginalTileAnimations(r.Before.Level)
			if err != nil {
				t.Fatal(err)
			}
			tiles, err := resources.Graphics(int(r.Before.Level), RegularMap)
			if err != nil {
				t.Fatal(err)
			}
			mapBytes, err := hex.DecodeString(r.Before.Supertiles)
			if err != nil || len(mapBytes) != 128 {
				t.Fatal(err)
			}
			maps := ([128]uint8)(mapBytes)
			beforeMap := maps
			q := GraphicsQueue{Used: r.Before.Used}
			got, err := q.RequestTileAnimation(r.X, r.Y, r.Code, GraphicsContext{Scroll: CollisionScroll{X: r.Before.ScrollX, Y: r.Before.ScrollY, NameTable: r.Before.Control}, Supertiles: &maps, Tileset: tiles}, animations)
			if err != nil {
				t.Fatalf("%s %d: %v", scope, r.Index, err)
			}
			if got.Accepted != (r.Carry == 0) || q.Used != r.After.Used {
				t.Fatalf("%s %d result %+v cursor%d want carry%d cursor%d", scope, r.Index, got, q.Used, r.Carry, r.After.Used)
			}
			if r.Before.Offset06 != r.After.Offset06 || r.Before.Mode != r.After.Mode || r.Before.Supertiles != r.After.Supertiles || r.Before.Collision != r.After.Collision || maps != beforeMap {
				t.Fatal("tile animation changed terrain/global state", scope, r.Index)
			}
			lookup := r.Before.AttributeLookup
			if got.AttributeLookupWritten {
				lookup = got.AttributeLookup
			}
			if lookup != r.After.AttributeLookup || got.AttributeLookupWritten != got.Accepted {
				t.Fatalf("%s %d lookup %+v want%d", scope, r.Index, got, r.After.AttributeLookup)
			}
			before, _ := hex.DecodeString(r.Before.Queue)
			after, _ := hex.DecodeString(r.After.Queue)
			var encoded []uint8
			for _, packet := range q.Packets {
				b, err := packet.Encode()
				if err != nil {
					t.Fatal(err)
				}
				encoded = append(encoded, b...)
			}
			if got.Accepted {
				accepted++
				if got.Address != r.After.Address {
					t.Fatalf("%s %d address %04x want%04x", scope, r.Index, got.Address, r.After.Address)
				}
				if !bytes.Equal(encoded, after[int(r.Before.Used):int(r.After.Used)]) {
					t.Fatalf("%s %d packets %x want%x", scope, r.Index, encoded, after[int(r.Before.Used):int(r.After.Used)])
				}
				copy(before[int(r.Before.Used):], encoded)
			} else {
				rejected++
				if len(encoded) != 0 {
					t.Fatal("rejected graphics writes")
				}
			}
			if !bytes.Equal(before, after) {
				t.Fatal("modified outside queued packets", scope, r.Index)
			}
		}
		if scope == "controlled" && (len(data.Records) != 1920 || accepted != 960 || rejected != 960) {
			t.Fatal("controlled coverage changed", len(data.Records), accepted, rejected)
		}
		t.Logf("%s: %d original requests, %d accepted/%d rejected", scope, len(data.Records), accepted, rejected)
	}
}

func TestMachineTileAnimationValidationAndAdmission(t *testing.T) {
	q := GraphicsQueue{Used: 80}
	if got, err := q.RequestTileAnimation(0, 0, 127, GraphicsContext{}, nil); err != nil || got.Accepted || q.Used != 80 {
		t.Fatal("gate must precede data access", got, err)
	}
	q.Reset()
	old := q
	if _, err := q.RequestTileAnimation(0, 0, 0, GraphicsContext{}, nil); err == nil || !reflect.DeepEqual(q, old) {
		t.Fatal("invalid request mutated queue")
	}
	if _, err := OriginalTileAnimations(8); err == nil {
		t.Fatal("invalid level accepted")
	}
	set, err := OriginalTileAnimations(6)
	if err != nil {
		t.Fatal(err)
	}
	if len(set[0].Rows) != 3 {
		t.Fatal("original claw reads third row across entry boundary")
	}
	q.Used = 79
	got, err := q.RequestTileAnimation(0, 16, 128, GraphicsContext{}, set)
	if err != nil || !got.Accepted || q.Used != 94 {
		t.Fatal("native admission allows crossing 80", got, q.Used, err)
	}
	set[0].Rows[0][0] ^= 255
	if q.Packets[0].Segments[0].Values[0] == set[0].Rows[0][0] {
		t.Fatal("packet aliases animation")
	}
	other, err := OriginalTileAnimations(6)
	if err != nil || other[0].Rows[0][0] == set[0].Rows[0][0] {
		t.Fatal("animation catalog aliases callers")
	}
}
