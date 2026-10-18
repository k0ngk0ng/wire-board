package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type resultActorSourceRow struct {
	Label         string
	Actor         uint8
	Before, After []byte
	Sounds        []uint8
	Opcode        uint8
	Operands      []uint8
}

func resultActorSource(t *testing.T) (actors, handlers []resultActorSourceRow) {
	t.Helper()
	f, e := os.Open("testdata/result-actor-extension.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows struct{ Actors, Handlers []resultActorSourceRow }
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	return rows.Actors, rows.Handlers
}

// The original private API already exposes the missing reward continuation;
// this source comparison is recorded RED before extending its implementation.
func TestOriginalResultActorExtendedSource(t *testing.T) {
	rows, _ := resultActorSource(t)
	for _, r := range rows {
		s, want := openingRAM(r.Before), openingRAM(r.After)
		sounds, e := stepOriginalResultActor(&s, r.Actor)
		if e != nil {
			t.Fatalf("%s: %v", r.Label, e)
		}
		if !reflect.DeepEqual(s, want) {
			t.Fatalf("%s original full C033 state differs", r.Label)
		}
		if len(sounds) != len(r.Sounds) || len(sounds) > 0 && !reflect.DeepEqual(sounds, r.Sounds) {
			t.Fatalf("%s sound order: %v want %v", r.Label, sounds, r.Sounds)
		}
	}
	t.Logf("matched %d complete original C033 calls including motion and projection", len(rows))
}

func resultActorContextRAM(r []byte) ResultActorContext {
	return ResultActorContext{TextRequestFlags: r[0x6ab], TextID: r[0x6ad], PaletteIDs: [2]uint8{r[0x305], r[0x306]}, DisplayMask: r[0x304]}
}
func TestOriginalResultActorContextAndRestore(t *testing.T) {
	rows, _ := resultActorSource(t)
	for i, r := range rows {
		s, c := openingRAM(r.Before), resultActorContextRAM(r.Before)
		got, e := StepOriginalResultActor(s, c, r.Actor)
		if e != nil {
			t.Fatalf("%s: %v", r.Label, e)
		}
		if got.State != openingRAM(r.After) || got.Context != resultActorContextRAM(r.After) {
			t.Fatalf("%s context or state differs: %+v want %+v", r.Label, got.Context, resultActorContextRAM(r.After))
		}
		if i%31 == 0 {
			raw, _ := json.Marshal(struct {
				State   OpeningState
				Context ResultActorContext
			}{s, c})
			var restored struct {
				State   OpeningState
				Context ResultActorContext
			}
			if e = json.Unmarshal(raw, &restored); e != nil {
				t.Fatal(e)
			}
			replay, e := StepOriginalResultActor(restored.State, restored.Context, r.Actor)
			if e != nil || !reflect.DeepEqual(got, replay) {
				t.Fatal("restored actor diverged", r.Label, e)
			}
		}
	}
}
func TestOriginalResultActorCommandHandlers(t *testing.T) {
	_, rows := resultActorSource(t)
	for _, r := range rows {
		s, c := openingRAM(r.Before), resultActorContextRAM(r.Before)
		cmd := resultActorCommand{Kind: resultActorCommandKind(r.Opcode)}
		if cmd.Kind >= 0xfe {
			cmd.Kind = resultOtherAction
		}
		copy(cmd.Args[:], r.Operands)
		reload, sounds, e := applyOriginalResultActorCommand(&s, &c, r.Actor, cmd)
		if e != nil {
			t.Fatal(r.Label, e)
		}
		if s != openingRAM(r.After) || c != resultActorContextRAM(r.After) {
			t.Fatalf("%s original handler differs: context %+v want %+v; state %+v want %+v", r.Label, c, resultActorContextRAM(r.After), s, openingRAM(r.After))
		}
		if reload != (r.Opcode == 0xf8) || len(sounds) != len(r.Sounds) || len(sounds) > 0 && !reflect.DeepEqual(sounds, r.Sounds) {
			t.Fatal(r.Label, reload, sounds, r.Sounds)
		}
	}
	t.Logf("matched %d original command handlers, including explicit unused palette/FE/FF alias operand probes", len(rows))
}
func TestOriginalResultActorFailureAtomicity(t *testing.T) {
	rows, _ := resultActorSource(t)
	original := openingRAM(rows[0].Before)
	context := resultActorContextRAM(rows[0].Before)
	for _, kind := range []string{"actor", "action", "cursor", "projection"} {
		s, actor := original, rows[0].Actor
		switch kind {
		case "actor":
			actor = 4
		case "action":
			s.Transition.Objects[actor].Action = 127
		case "cursor":
			s.Transition.Objects[actor].Animation.Cursor = 255
		case "projection":
			s.Transition.CameraScroll[3] = 0x200
		}
		got, e := StepOriginalResultActor(s, context, actor)
		if e == nil || got.State != s || got.Context != context || len(got.SoundIDs) != 0 {
			t.Fatal(kind, "failed call changed input or accepted invalid state", e)
		}
	}
}
