package runtimebridge

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/sessiontick"
)

// This inventory is intentionally structural: a future reference field must be
// named here and handled explicitly rather than silently shallow-copied.
func mutablePaths(typ reflect.Type, path string, out *[]string) {
	switch typ.Kind() {
	case reflect.Map, reflect.Slice, reflect.Pointer, reflect.Interface, reflect.Func, reflect.Chan:
		*out = append(*out, path)
		if typ.Kind() == reflect.Map || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Pointer {
			mutablePaths(typ.Elem(), path+"[]", out)
		}
	case reflect.Array:
		mutablePaths(typ.Elem(), path+"[]", out)
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			mutablePaths(field.Type, path+"."+field.Name, out)
		}
	}
}
func TestClientBridgeMutableOwnersBaseline(t *testing.T) {
	var got []string
	mutablePaths(reflect.TypeOf(snapshot{}), "saved", &got)
	mutablePaths(reflect.TypeOf(legacyPresentation{}), "legacy", &got)
	// computed itself is the sole variant pointer, its target is checked separately.
	s := reflect.TypeOf(Session{})
	computed, _ := s.FieldByName("computed")
	mutablePaths(computed.Type.Elem(), "computed", &got)
	sort.Strings(got)
	want := []string{"computed.HUD.Slots", "legacy.HUD.Slots", "saved.Options.Players", "saved.World.Objects", "saved.World.Registration.Owners", "saved.World.Stream", "saved.World.Throws.Records"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mutable inventory changed: got=%v want=%v", got, want)
	}
	t.Logf("mutable containers=%v", got)
}
func benchmarkAsset(b *testing.B, name string) []byte {
	b.Helper()
	packed, e := os.ReadFile("../data/" + name + ".json.gz")
	if e != nil {
		b.Fatal(e)
	}
	reader, e := gzip.NewReader(bytes.NewReader(packed))
	if e != nil {
		b.Fatal(e)
	}
	raw, e := io.ReadAll(reader)
	if e != nil {
		b.Fatal(e)
	}
	if e = reader.Close(); e != nil {
		b.Fatal(e)
	}
	return raw
}
func benchmarkNetworkSession(b *testing.B) *Session {
	raw := AssetBytes{Rules: benchmarkAsset(b, "chu-world-rules"), Sprites: benchmarkAsset(b, "world-sprites"), Arena: benchmarkAsset(b, "arena-assets-a"), HUD: benchmarkAsset(b, "hud-assets"), ArenaAux: benchmarkAsset(b, "arena-a-aux-assets"), RenderMetadata: benchmarkAsset(b, "renderstate-metadata")}
	var e error
	raw.Descriptor, e = os.ReadFile("../data/runtime-resource-descriptor.json")
	if e != nil {
		b.Fatal(e)
	}
	a, e := DecodeNetworkAssets(raw, 10)
	if e != nil {
		b.Fatal(e)
	}
	s, e := NewSession(networkOptions(10, []uint8{7, 9}, false, false), a)
	if e != nil {
		b.Fatal(e)
	}
	if e = s.Ready(); e != nil {
		b.Fatal(e)
	}
	for i := 0; i < 320; i++ {
		if _, e = s.StepControlled([4]engine.Buttons{}, 0); e != nil {
			b.Fatal(e)
		}
	}
	return s
}
func BenchmarkClientBridgeCurrent(b *testing.B) {
	b.Run("JSONFork", func(b *testing.B) {
		s := benchmarkNetworkSession(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			raw, e := s.Save()
			if e != nil {
				b.Fatal(e)
			}
			if _, e = RestoreSession(raw, s.assets); e != nil {
				b.Fatal(e)
			}
		}
	})
	b.Run("StepWithPixels", func(b *testing.B) {
		s := benchmarkNetworkSession(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, e := s.StepControlled([4]engine.Buttons{}, 0); e != nil {
				b.Fatal(e)
			}
		}
	})
	b.Run("CanonicalAdvanceOnly", func(b *testing.B) {
		s := benchmarkNetworkSession(b)
		w, p := s.saved.World, *s.computed
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var e error
			w, p, _, _, e = sessiontick.Advance(w, p, s.saved.Options, [4]engine.Buttons{}, 0, s.assets.rules, s.assets.graphics)
			if e != nil {
				b.Fatal(e)
			}
		}
	})
	b.Run("DualDigests", func(b *testing.B) {
		s := benchmarkNetworkSession(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, _, e := s.Digests(); e != nil {
				b.Fatal(e)
			}
		}
	})
	b.Run("PreviewJSONFork", func(b *testing.B) {
		s := benchmarkNetworkSession(b)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, e := s.Preview(); e != nil {
				b.Fatal(e)
			}
		}
	})
}
