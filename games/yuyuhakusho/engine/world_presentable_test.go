package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func worldFirstBytesTest(t *testing.T, id uint16, raw []byte) WorldObject {
	t.Helper()
	b := make([]byte, 256)
	copy(b, raw)
	return decodeWorldObjectTest(t, id, hex.EncodeToString(b))
}

// These added fields/adapter tests precede the first64 production expansion.
func TestWorldObjectsFirst64PresentableViews(t *testing.T) {
	for seed := 0; seed < 4; seed++ {
		o, raw := denseWorldObjectTest(t, 0x9c80, seed)
		b := attackContactBytes(t, raw, 256)
		want := overlayObject(b)
		if got := o.AsMatchOverlay(); got != want {
			t.Fatal("first64 overlay view differs from original-byte adapter")
		}
		if o.MergeMatchOverlay(want) != o {
			t.Fatal("overlay identity merge changed unowned fields")
		}
		want.Counter ^= 0x1357
		want.Timer ^= 0x2468
		want.Data ^= 0x9753
		want.ScreenX++
		want.ScreenY--
		merged := o.MergeMatchOverlay(want)
		expected := append([]byte(nil), b...)
		encodeOverlayObject(want, expected)
		if encodeWorldObjectTest(t, merged, raw, 0) != hex.EncodeToString(expected) || merged.Aux0A != o.Aux0A || merged.Aux0E != o.Aux0E || uint16(merged.Body.VelocityX) != uint16(o.Body.VelocityX) || merged.Body.VerticalVelocity != o.Body.VerticalVelocity {
			t.Fatal("overlay merge lost screen/velocity fractions or other first64 fields")
		}
		for _, kind := range []string{"sun", "moon", "spark"} {
			k, off := arenaSkyKind(t, kind)
			id := uint16(0x9c80 + off)
			object := o
			object.ObjectID = id
			skyBytes := make([]byte, 384)
			copy(skyBytes[off:off+128], b[:128])
			snapshot := defaultArenaSnapshot{Sky: hex.EncodeToString(skyBytes), Camera: hex.EncodeToString(make([]byte, 14)), Samples: hex.EncodeToString(make([]byte, 128)),
				PaletteRegisters: hex.EncodeToString(make([]byte, 8)), Palettes: hex.EncodeToString(make([]byte, 384))}
			legacy, _ := decodeArenaSky(t, snapshot, kind)
			if object.AsArenaSky(k) != legacy {
				t.Fatal("sky view differs from independent original-byte decoder", kind)
			}
			back, err := object.MergeArenaSky(legacy)
			if err != nil || back != object {
				t.Fatal("sky identity merge modified unowned metadata", err)
			}
			legacy.ScreenXFraction ^= 0x4681
			legacy.Phase ^= 0x12345678
			legacy.Sequence ^= 0x87654321
			back, err = object.MergeArenaSky(legacy)
			if err != nil || back.Aux0A != legacy.ScreenXFraction || uint32(back.Aux48) != legacy.Phase || back.AnimationSet != legacy.Sequence || back.Aux0E != object.Aux0E || back.Body.Clock.Duration != object.Body.Clock.Duration || back.Body.Clock.Completed != object.Body.Clock.Completed || back.SkipGraphics != object.SkipGraphics {
				t.Fatal("sky alias merge duplicated phase/sequence or erased unowned fields", err)
			}
			bad := legacy
			bad.ObjectID++
			back, err = object.MergeArenaSky(bad)
			if err == nil || back != object {
				t.Fatal("sky cross-slot identity accepted")
			}
		}
		shadowSnapshot := matchOverlaySnapshot{ShadowBytes: hex.EncodeToString(b[:128]), Registration: hex.EncodeToString(make([]byte, 128))}
		shadow, _ := decodeMatchShadow(t, shadowSnapshot)
		shadow.Registration = FighterRegistrationState{}
		if !reflect.DeepEqual(o.AsMatchShadow(), shadow) || o.MergeMatchShadow(shadow) != o {
			t.Fatal("shadow view/merge differs from original-byte adapter")
		}
		shadow.Cursor ^= 0x1579
		back := o.MergeMatchShadow(shadow)
		if uint16(back.AnimationSet>>16) != shadow.Cursor || uint16(back.AnimationSet) != uint16(o.AnimationSet) {
			t.Fatal("shadow cursor was not a high-word view of animation/sequence storage")
		}
	}
}

func readWorldObjectViewSamples(t *testing.T, name string, target any) int {
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
		Pass, Baseline bool
		Records        int
		Match          bool   `json:"frames_match_original_core"`
		SHA            string `json:"fixture_sha256"`
		Gzip           string `json:"gzip_sha256"`
		Deriver        string `json:"derive_script_sha256"`
		Source         struct {
			Fixture     string
			Gzip        string `json:"gzip_sha256"`
			Manifest    string
			ManifestSHA string `json:"manifest_sha256"`
		}
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	source, err := os.ReadFile("testdata/" + filepath.Base(m.Source.Fixture))
	if err != nil {
		t.Fatal(err)
	}
	sm, err := os.ReadFile("testdata/" + filepath.Base(m.Source.Manifest))
	if err != nil {
		t.Fatal(err)
	}
	deriver, err := os.ReadFile("../../../scripts/yuyuhakusho/derive_world_object_views.py")
	if err != nil {
		t.Fatal(err)
	}
	sh, mh, dh := sha256.Sum256(source), sha256.Sum256(sm), sha256.Sum256(deriver)
	if !m.Pass || !m.Match || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip || hex.EncodeToString(sh[:]) != m.Source.Gzip || hex.EncodeToString(mh[:]) != m.Source.ManifestSHA || hex.EncodeToString(dh[:]) != m.Deriver {
		t.Fatal("unbound derived original-CPU representative rows")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
	return m.Records
}

func TestWorldObjectsOverlayRulesAgainstOriginal(t *testing.T) {
	var fixture struct{ Calls []matchOverlayCall }
	count := readWorldObjectViewSamples(t, "world-objects-overlay-samples", &fixture)
	if count != 484 || len(fixture.Calls) != count {
		t.Fatal("missing representative overlay rows")
	}
	kinds := map[string]int{}
	for i, row := range fixture.Calls {
		if row.Kind == "shadow" {
			raw := overlayBytes(t, row.Before.ShadowBytes, 128)
			object := worldFirstBytesTest(t, row.ObjectID, raw)
			before, owners := decodeMatchShadow(t, row.Before)
			state := object.AsMatchShadow()
			state.Registration = before.Registration
			if !reflect.DeepEqual(state, before) {
				t.Fatal("shadow canonical input differs")
			}
			got, out, err := StepMatchShadow(state, MatchShadowEnvironment{AttributeMask: row.Before.Attributes, CameraX: row.Before.CameraX, CameraY: row.Before.CameraY, Shake: row.Before.Shake}, owners)
			if err != nil {
				t.Fatal(err)
			}
			object, err = ApplyObjectClearPrefix(object, out.ClearedBytes)
			if err != nil {
				t.Fatal(err)
			}
			object = object.MergeMatchShadow(got)
			encoded, _ := hex.DecodeString(encodeWorldObjectTest(t, object, hex.EncodeToString(append(append([]byte(nil), raw...), make([]byte, 128)...)), out.ClearedBytes))
			if hex.EncodeToString(encoded[:128]) != row.After.ShadowBytes || got.Registration.Used != row.After.Used || out.Draw != row.Draw || out.ClearedBytes != row.Cleared {
				t.Fatalf("original shadow merge %d differs", i)
			}
		} else {
			all := overlayBytes(t, row.Before.Objects, 768)
			off := int(row.ObjectID) - 0x8100
			object := worldFirstBytesTest(t, row.ObjectID, all[off:off+64])
			state := MatchOverlayState{ObjectID: row.ObjectID, Object: object.AsMatchOverlay(), SoundQueue: row.Before.Queue, ModeFlag: row.Before.Mode, SlowFlag: row.Before.Slow}
			if row.Kind == "intro" {
				for child := range state.Children {
					state.Children[child] = worldFirstBytesTest(t, row.ObjectID+uint16(child+1)*64, all[off+(child+1)*64:off+(child+2)*64]).AsMatchOverlay()
				}
			}
			if state != decodeMatchOverlay(t, row.Before, row.ObjectID, row.Kind) {
				t.Fatal("overlay canonical input differs")
			}
			got, out, err := StepMatchOverlay(state, MatchOverlayEnvironment{PauseFlags: row.Before.Pause, WinnerCode: row.Before.Winner, Characters: row.Before.Characters, TeamTitles: row.Before.TeamTitles, RoundIndex: row.Before.Round, Blink: row.Before.Blink})
			if err != nil {
				t.Fatal(err)
			}
			object, err = ApplyObjectClearPrefix(object, out.ClearedBytes)
			if err != nil {
				t.Fatal(err)
			}
			object = object.MergeMatchOverlay(got.Object)
			base := append(append([]byte(nil), all[off:off+64]...), make([]byte, 192)...)
			encoded, _ := hex.DecodeString(encodeWorldObjectTest(t, object, hex.EncodeToString(base), out.ClearedBytes))
			copy(all[off:off+64], encoded[:64])
			if row.Kind == "intro" {
				for child := range got.Children {
					start := off + (child+1)*64
					v := worldFirstBytesTest(t, row.ObjectID+uint16(child+1)*64, all[start:start+64]).MergeMatchOverlay(got.Children[child])
					b := append(append([]byte(nil), all[start:start+64]...), make([]byte, 192)...)
					e, _ := hex.DecodeString(encodeWorldObjectTest(t, v, hex.EncodeToString(b), 0))
					copy(all[start:start+64], e[:64])
				}
			}
			if hex.EncodeToString(all) != row.After.Objects || got.SoundQueue != row.After.Queue || got.ModeFlag != row.After.Mode || got.SlowFlag != row.After.Slow || out.Draw != row.Draw || out.ClearedBytes != row.Cleared {
				t.Fatalf("original overlay canonical merge %d %s/%s differs", i, row.Kind, row.Name)
			}
		}
		kinds[row.Kind]++
	}
	if len(kinds) != 8 {
		t.Fatal("missing representative overlay kinds")
	}
	t.Logf("original overlay canonical merges=%d kinds=%v", count, kinds)
}

func TestWorldObjectsSkyRulesAgainstOriginal(t *testing.T) {
	var fixture struct {
		Library ArenaRuntimeLibrary
		Calls   []defaultArenaCall
	}
	count := readWorldObjectViewSamples(t, "world-objects-sky-samples", &fixture)
	if count != 59 || len(fixture.Calls) != count {
		t.Fatal("missing representative sky rows")
	}
	kinds := map[string]int{}
	for i, row := range fixture.Calls {
		kind, off := arenaSkyKind(t, row.Kind)
		sky := arenaBytes(t, row.Before.Sky, 384)
		object := worldFirstBytesTest(t, uint16(0x9c80+off), sky[off:off+128])
		before, world := decodeArenaSky(t, row.Before, row.Kind)
		state := object.AsArenaSky(kind)
		if state != before {
			t.Fatal("sky canonical input differs")
		}
		got, w, out, err := StepArenaSky(state, world, fixture.Library)
		if err != nil {
			t.Fatal(i, row.Name, err)
		}
		object, err = ApplyObjectClearPrefix(object, out.ClearedBytes)
		if err != nil {
			t.Fatal(err)
		}
		object, err = object.MergeArenaSky(got)
		if err != nil {
			t.Fatal(err)
		}
		b := append(append([]byte(nil), sky[off:off+128]...), make([]byte, 128)...)
		encoded, _ := hex.DecodeString(encodeWorldObjectTest(t, object, hex.EncodeToString(b), out.ClearedBytes))
		copy(sky[off:off+128], encoded[:128])
		if kind == ArenaMoon {
			next := worldFirstBytesTest(t, 0x9d80, sky[256:384])
			next.Header = w.Next
			e, _ := hex.DecodeString(encodeWorldObjectTest(t, next, hex.EncodeToString(append(append([]byte(nil), sky[256:384]...), make([]byte, 128)...)), 0))
			copy(sky[256:384], e[:128])
		}
		want, wantWorld := decodeArenaSky(t, row.After, row.Kind)
		if got != want || w != wantWorld || hex.EncodeToString(sky) != row.After.Sky || out.Draw != row.Draw || out.ClearedBytes != row.ClearedBytes || out.Spawned != row.Spawned {
			t.Fatalf("original sky canonical merge %d %s/%s differs", i, row.Kind, row.Name)
		}
		kinds[row.Kind]++
	}
	if len(kinds) != 3 {
		t.Fatal("missing sky kinds")
	}
	t.Logf("original sky canonical merges=%d kinds=%v", count, kinds)
}
