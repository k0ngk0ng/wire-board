package engine

// Source-bound checks precede shared runtime opening: wrong kind catalog,
// scene/court conflation, lost original reset fields, and a fabricated Court7
// animation or RNG/palette/audio update.
import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestCourtSevenObservedOriginalOpening(t *testing.T) {
	f, e := os.Open("testdata/court-seven-opening.ram.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var raw [4096]byte
	if _, e = io.ReadFull(z, raw[:]); e != nil {
		t.Fatal(e)
	}
	before, want := transitionRAM(raw[:2048]), transitionRAM(raw[2048:])
	got, e := InitializeOriginalPeriod(before)
	if e != nil {
		t.Fatal(e)
	}
	if got.State != want {
		t.Fatal(firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(want), "opening"))
	}
	if got.State.Scene != 1 || got.State.Court != 7 || got.State.Kinds[4] != 13 || got.State.Kinds[5] != 14 || !got.BeginWorldInputs {
		t.Fatal("Court7 original opening identities differ", got)
	}
	profile, e := OriginalCourtProfile(7)
	if e != nil || profile.TerrainRows != 11 || profile.MaximumDepth != 81 {
		t.Fatal("wrong Court7 profile", profile, e)
	}
	t.Log("matched passive original Court7 D257..D303 setup, actual auxiliary kinds13/14 and Scene1; full reference RAM/screenshots unchanged")
}
func TestCourtSevenOriginalAnimationIsNoOp(t *testing.T) {
	count := 0
	for _, name := range []string{"court-animation.json.gz", "court-animation-probes.json.gz"} {
		f, e := os.Open("testdata/" + name)
		if e != nil {
			t.Fatal(e)
		}
		z, e := gzip.NewReader(f)
		if e != nil {
			t.Fatal(e)
		}
		var rows []struct {
			Court   int
			Before  CourtAnimationState
			Context CourtAnimationContext
			After   CourtAnimationStep
		}
		if e = json.NewDecoder(z).Decode(&rows); e != nil {
			t.Fatal(e)
		}
		z.Close()
		f.Close()
		for _, r := range rows {
			if r.Court != 7 {
				continue
			}
			count++
			got, e := StepCourtAnimation(r.Before, r.Context)
			if e != nil || !reflect.DeepEqual(got, r.After) {
				t.Fatal(name, count, e, got, r.After)
			}
			if got.State != r.Before || got.Random != r.Context.Random || got.Scores != r.Context.Scores || got.Palette || len(got.Sounds) != 0 || len(got.Patches) != 0 {
				t.Fatal("Court7 invented a court animation", got)
			}
		}
	}
	if count != 370 {
		t.Fatal("missing Court7 no-op source calls", count)
	}
	t.Log("matched 114 original Court7 animation calls and 256 source probes; A32C emits no state/RNG/score/palette/tile/sound changes")
}
