package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

type fighter0ProjectileSnapshot struct {
	chuProjectileSnapshot
	RNGBytes string `json:"rng_bytes"`
}

type fighter0ProjectileCall struct {
	Kind, Name    string
	ObjectID      uint16 `json:"object_id"`
	Before, After fighter0ProjectileSnapshot
	Draw, Deleted bool
	ClearedBytes  uint16 `json:"cleared_bytes"`
	External      struct {
		ContactFlags *uint8 `json:"contact_flags"`
		PauseFlags   *uint8 `json:"pause_flags"`
		SlowFlag     *uint8 `json:"slow_flag"`
	} `json:"external_input"`
}

type fighter0ProjectileFixture struct {
	Library struct {
		Animations []struct {
			Key  AnimationFrameKey
			Data AnimationRecordData
		}
		Sequences []struct {
			Identity uint32
			Frames   []ChuAttachmentFrame
		}
	}
	Calls json.RawMessage
}

func readFighter0Projectile(t *testing.T, name string) (fighter0ProjectileFixture, Fighter0ProjectileLibrary, int, int) {
	t.Helper()
	packed, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass            bool
		Records, Calls  int
		Frames          int    `json:"library_frames"`
		Sequences       int    `json:"short_sequences"`
		ExternalQueue   int    `json:"external_queue_calls"`
		ExternalChanges int    `json:"external_change_calls"`
		FramesMatch     bool   `json:"frames_match_original_core"`
		FixtureSHA      string `json:"fixture_sha256"`
		GzipSHA         string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(manifest, &m); err != nil {
		t.Fatal(err)
	}
	digest, zipped := sha256.Sum256(raw), sha256.Sum256(packed)
	if !m.Pass || !m.FramesMatch || m.Records == 0 || m.Frames != 25 || m.Sequences != 8 || m.ExternalQueue != 0 || m.ExternalChanges != 0 ||
		hex.EncodeToString(digest[:]) != m.FixtureSHA || hex.EncodeToString(zipped[:]) != m.GzipSHA {
		t.Fatal("unverified source0 child/projectile fixture")
	}
	var f fighter0ProjectileFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	library := Fighter0ProjectileLibrary{Animations: make(AnimationLibrary), Sequences: make(map[uint32][]ChuAttachmentFrame)}
	for _, row := range f.Library.Animations {
		if _, exists := library.Animations[row.Key]; exists {
			t.Fatal("duplicate source0 projectile frame")
		}
		library.Animations[row.Key] = row.Data
	}
	for _, row := range f.Library.Sequences {
		if _, exists := library.Sequences[row.Identity]; exists {
			t.Fatal("duplicate source0 release sequence")
		}
		library.Sequences[row.Identity] = row.Frames
	}
	if len(library.Animations) != 25 || len(library.Sequences) != 8 {
		t.Fatal("incomplete source0 projectile semantic library")
	}
	return f, library, m.Records, m.Calls
}

func decodeFighter0Projectile(t *testing.T, kind string, raw fighter0ProjectileSnapshot, id uint16) Fighter0ProjectileState {
	t.Helper()
	k := Fighter0ScatterProjectile
	if kind == "release" {
		k = Fighter0ReleaseLight
	} else if kind != "scatter" {
		t.Fatal("unknown original source0 projectile kind", kind)
	}
	return Fighter0ProjectileState{Kind: k, Projectile: decodeChuProjectile(t, raw.chuProjectileSnapshot, id)}
}

func encodeFighter0Projectile(t *testing.T, state Fighter0ProjectileState, outcome ChuProjectileOutcome, old fighter0ProjectileSnapshot) fighter0ProjectileSnapshot {
	t.Helper()
	old.chuProjectileSnapshot = encodeChuProjectile(t, state.Projectile, outcome, old.chuProjectileSnapshot)
	return old
}

func fighter0ProjectileDiff(t *testing.T, got, want fighter0ProjectileSnapshot) string {
	t.Helper()
	for name, pair := range map[string][2]string{"actor": {got.ActorBytes, want.ActorBytes}, "FX": {got.Pool.Bytes, want.Pool.Bytes}, "buffers": {got.Buffers, want.Buffers}} {
		a, _ := hex.DecodeString(pair[0])
		b, _ := hex.DecodeString(pair[1])
		for i := range a {
			if a[i] != b[i] {
				return name + " at " + hex.EncodeToString([]byte{uint8(i >> 8), uint8(i)}) + " got " + hex.EncodeToString(a[i:i+1]) + " want " + hex.EncodeToString(b[i:i+1])
			}
		}
	}
	return "metadata or typed owner differs"
}

// Raw CPU fixtures, failure scenarios and these tests precede the rule file.
func TestFighter0ProjectileOriginalCPU(t *testing.T) {
	for _, name := range []string{"fighter0-projectile-oracle", "fighter0-projectile-boundaries"} {
		t.Run(name, func(t *testing.T) {
			f, library, records, _ := readFighter0Projectile(t, name)
			var rows []fighter0ProjectileCall
			if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != records {
				t.Fatal("missing complete source0 projectile calls", err)
			}
			kinds, deleted := map[string]int{}, 0
			for i, row := range rows {
				input := decodeFighter0Projectile(t, row.Kind, row.Before, row.ObjectID)
				copy := input
				got, outcome, err := StepFighter0Projectile(input, row.Before.Environment, library)
				want := ChuProjectileOutcome{Draw: row.Draw, Deleted: row.Deleted, ClearedBytes: row.ClearedBytes}
				encoded := encodeFighter0Projectile(t, got, outcome, row.Before)
				if err != nil || outcome != want || !reflect.DeepEqual(encoded, row.After) || !reflect.DeepEqual(got, decodeFighter0Projectile(t, row.Kind, row.After, row.ObjectID)) {
					t.Fatalf("source0 projectile %d %s/%s err=%v outcome=%+v/%+v %s", i, row.Kind, row.Name, err, outcome, want, fighter0ProjectileDiff(t, encoded, row.After))
				}
				if !reflect.DeepEqual(input, copy) || row.Before.ParentBytes != row.After.ParentBytes || row.Before.RNGBytes != row.After.RNGBytes {
					t.Fatal("changed input, parent or shared RNG")
				}
				kinds[row.Kind]++
				if row.Deleted {
					deleted++
				}
			}
			if name == "fighter0-projectile-oracle" && (records != 47 || kinds["scatter"] != 34 || kinds["release"] != 13 || deleted != 4) {
				t.Fatal("incomplete source0 natural two-kind coverage", records, kinds, deleted)
			}
			if name == "fighter0-projectile-boundaries" && records != 5837 {
				t.Fatal("incomplete source0 isolated matrix")
			}
			t.Logf("complete original calls=%d kinds=%v deletions=%d", records, kinds, deleted)
		})
	}
}

func TestFighter0ProjectileContinuousDeletion(t *testing.T) {
	f, library, records, count := readFighter0Projectile(t, "fighter0-projectile-sequences")
	var rows []struct {
		Kind    string
		Initial fighter0ProjectileSnapshot
		Calls   []fighter0ProjectileCall
	}
	if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != records || records != 44 {
		t.Fatal("missing single-initial-state source0 lifecycles", err)
	}
	steps := 0
	for _, stream := range rows {
		if len(stream.Calls) == 0 || !stream.Calls[len(stream.Calls)-1].Deleted {
			t.Fatal("source0 lifecycle does not reach original prefix clear")
		}
		state := decodeFighter0Projectile(t, stream.Kind, stream.Initial, stream.Calls[0].ObjectID)
		encoded, env := stream.Initial, stream.Initial.Environment
		for i, call := range stream.Calls {
			if call.External.ContactFlags != nil {
				state.Projectile.ContactFlags = *call.External.ContactFlags
			}
			if call.External.PauseFlags != nil {
				env.PauseFlags = *call.External.PauseFlags
			}
			if call.External.SlowFlag != nil {
				env.SlowFlag = *call.External.SlowFlag
			}
			encoded.Environment = env
			encoded = encodeFighter0Projectile(t, state, ChuProjectileOutcome{}, encoded)
			if !reflect.DeepEqual(encoded, call.Before) {
				t.Fatalf("undeclared source0 lifecycle input at %s %d: %s", stream.Kind, i, fighter0ProjectileDiff(t, encoded, call.Before))
			}
			var outcome ChuProjectileOutcome
			var err error
			state, outcome, err = StepFighter0Projectile(state, env, library)
			encoded = encodeFighter0Projectile(t, state, outcome, encoded)
			if err != nil || outcome != (ChuProjectileOutcome{Draw: call.Draw, Deleted: call.Deleted, ClearedBytes: call.ClearedBytes}) ||
				!reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(state, decodeFighter0Projectile(t, stream.Kind, call.After, call.ObjectID)) {
				t.Fatalf("source0 continuous %s step%d err=%v: %s", stream.Kind, i, err, fighter0ProjectileDiff(t, encoded, call.After))
			}
			steps++
		}
	}
	if steps != count || steps != 472 {
		t.Fatal("incomplete continuous source0 lifecycle steps", steps)
	}
}

func TestFighter0ProjectileErrorsAreAtomic(t *testing.T) {
	f, library, _, _ := readFighter0Projectile(t, "fighter0-projectile-boundaries")
	var rows []fighter0ProjectileCall
	if err := json.Unmarshal(f.Calls, &rows); err != nil {
		t.Fatal(err)
	}
	base := decodeFighter0Projectile(t, "scatter", rows[0].Before, rows[0].ObjectID)
	env := rows[0].Before.Environment
	check := func(name string, input Fighter0ProjectileState, lib Fighter0ProjectileLibrary) {
		t.Helper()
		got, outcome, err := StepFighter0Projectile(input, env, lib)
		if err == nil || !reflect.DeepEqual(got, input) || outcome != (ChuProjectileOutcome{}) {
			t.Fatalf("%s did not reject atomically: %v %+v", name, err, outcome)
		}
	}
	base.Projectile.Stage, base.Projectile.ContactFlags = 0, 0
	check("missing scatter data after initialization and motion", base, Fighter0ProjectileLibrary{})
	base.Projectile.Stage, base.Projectile.ContactFlags, base.Projectile.Pool.Cursor = 2, 8, 1
	check("misaligned FX pool before impact clear", base, library)
	base.Projectile.Pool.Cursor, base.Projectile.Size = 0, 1
	check("late deletion error after impact allocation", base, library)
	base.Kind = 255
	check("unknown callback kind", base, library)
	base = decodeFighter0Projectile(t, "release", rows[0].Before, rows[0].ObjectID)
	base.Projectile.Stage = 0
	check("missing release data after initialization", base, Fighter0ProjectileLibrary{})
	malformed := Fighter0ProjectileLibrary{Sequences: map[uint32][]ChuAttachmentFrame{0x135ee: {{Parameter: 2, Duration: 0}, {Parameter: 0, Duration: 0}}}}
	check("early release terminator", base, malformed)
	malformed.Sequences[0x135ee] = []ChuAttachmentFrame{{Parameter: 511, Duration: 1}, {Parameter: 0, Duration: 0}}
	check("invalid release graphic parameter", base, malformed)
	base.Projectile.Stage, base.Projectile.Power = 2, 0
	base.Projectile.Animation.AnimationSet, base.Projectile.Animation.Clock.FrameIndex, base.Projectile.Animation.Clock.Ticks = 0x135ee, 3, 1
	check("release index beyond finite sequence", base, library)
	base.Projectile.Animation.Clock.FrameIndex, base.Projectile.Size = 1, 1
	check("late release clear error", base, library)
	for _, kind := range []Fighter0ProjectileKind{Fighter0ScatterProjectile, Fighter0ReleaseLight} {
		input := decodeFighter0Projectile(t, "scatter", rows[0].Before, rows[0].ObjectID)
		input.Kind, input.Projectile.Stage = kind, 0
		input.Projectile.Animation.Clock.Completed = 2
		env.PauseFlags = 128
		got, outcome, err := StepFighter0Projectile(input, env, Fighter0ProjectileLibrary{})
		if err != nil || outcome != (ChuProjectileOutcome{Draw: true}) || got.Projectile.Stage != 2 || got.Projectile.Animation.Clock.Completed != 2 ||
			got.Projectile.ScreenX != input.Projectile.ScreenX || got.Projectile.ScreenOffsetX != input.Projectile.ScreenOffsetX {
			t.Fatal("paused init required data or advanced projection", kind, err, outcome)
		}
		if kind == Fighter0ReleaseLight && got.Projectile.Health != input.Projectile.Health {
			t.Fatal("release light applied scatter health write")
		}
	}
}
