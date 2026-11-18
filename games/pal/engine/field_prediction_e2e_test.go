package engine_test

// Test first. Public field observations must expose enough original motor
// state without native memory/scripts, preserve complete saves, detach all
// nested arrays, reject malformed bounds/actors/cells, and agree with the
// original campaign's authoritative movement after a scheduled input.
import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestOriginalFieldPredictionProjection(t *testing.T) {
	r := nativeRuntime(t)
	inn := originalInn(t, r)
	f := inn.View.Field
	if f == nil || f.Radius != 8 || len(f.Cells) != 334 || f.Party != 1 || len(f.Actors) < f.Party+f.Followers || f.Anchor != [2]int{inn.View.X, inn.View.Y} {
		t.Fatal("original inn lacks its bounded field motor state", f)
	}
	encoded, e := r.Encode(inn)
	if e != nil {
		t.Fatal(e)
	}
	tx, e := r.Continue(encoded)
	if e != nil {
		t.Fatal(e)
	}
	v := tx.View()
	v.Field.Actors[0][3]++
	v.Field.Cells[0] ^= 1
	v.Field.Trail[0][0]++
	if !reflect.DeepEqual(tx.View().Field, f) {
		t.Fatal("field projection aliases private native movement state")
	}
	for _, change := range []func(*engine.FieldState){
		func(f *engine.FieldState) { f.Radius++ },
		func(f *engine.FieldState) { f.Cells = f.Cells[:1] },
		func(f *engine.FieldState) { f.Cells[0] = 4 },
		func(f *engine.FieldState) { f.Step = 4 },
		func(f *engine.FieldState) { f.Party = 4 },
		func(f *engine.FieldState) { f.Trail[0][2] = 4 },
		func(f *engine.FieldState) { f.Actors[0][0] = 1 },
		func(f *engine.FieldState) { f.Camera[0]++ },
	} {
		bad, e := r.Decode(encoded)
		if e != nil {
			t.Fatal(e)
		}
		change(bad.View.Field)
		if _, e = r.Encode(bad); e == nil {
			t.Fatal("malformed public motor state accepted")
		}
	}
	empty, e := r.Decode(encoded)
	if e != nil {
		t.Fatal(e)
	}
	empty.View.Party = nil
	empty.View.Field.Party = 0
	empty.View.Field.Followers = 0
	empty.View.Field.Actors = nil
	if _, e = r.Encode(empty); e == nil {
		t.Fatal("empty native actor metadata was accepted")
	}
	advanced, e := tx.AdvanceScheduled(context.Background(), uint32(inn.Event.Value), 250, func(uint32) engine.Input { return engine.Input{Direction: 0} })
	if e != nil || advanced.Steps == 0 {
		t.Fatal("original motor did not run", e)
	}
	after := tx.View()
	if after.Field == nil || after.Field.Frame <= f.Frame || after.Field.Camera == f.Camera || after.Field.Actors[0][3] != after.X || after.Field.Actors[0][4] != after.Y {
		t.Fatal("authoritative movement differs from its public motor observation")
	}
	saved, e := tx.Encode()
	if e != nil {
		t.Fatal(e)
	}
	restored, e := r.Decode(saved)
	if e != nil || !reflect.DeepEqual(restored.View.Field, after.Field) {
		t.Fatal("native save lost public motor continuation", e)
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "test": t.Name(), "scope": "actual original campaign inn, bounded public motor observation, nested output isolation, malformed metadata rejection and authoritative scheduled movement/save continuation; not latency or full-campaign acceptance"}, "", "  ")
	if e = os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/field-prediction-go.json"), append(report, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
}
