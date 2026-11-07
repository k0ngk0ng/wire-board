package contra

// The before-state is a recorded legal-input replay, not a fabricated enemy.
// Its explicit reserve grants are retained in the fixture. Failures/producer
// ordering are listed in docs/research/nes-contra/base-throw-work.md before fixes.
import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

type baseThrowSessionFixture struct {
	Frame   uint64
	Pads    [2]Buttons
	Before  Session
	Refills []struct {
		Actor         int
		Before, After uint8
		Frame         uint64
	}
}

func loadBaseThrowSession(t *testing.T) baseThrowSessionFixture {
	t.Helper()
	f, e := os.Open("testdata/session-base2-throw-entry.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var v baseThrowSessionFixture
	if e = json.NewDecoder(z).Decode(&v); e != nil {
		t.Fatal(e)
	}
	if v.Before.Random.Frames != 26904 || v.Frame != 26905 || v.Pads != [2]Buttons{ButtonB, 0} {
		t.Fatal("wrong recorded boundary")
	}
	if e = v.Before.Validate(); e != nil {
		t.Fatal(e)
	}
	return v
}
func TestSessionBase2ThrowReceivesActualFrameEntry(t *testing.T) {
	v := loadBaseThrowSession(t)
	r := levelResources(t, "stage-4.json")
	s := v.Before
	if _, e := s.StepPads(v.Frame, v.Pads, r); e != nil {
		t.Fatal("legal next frame rejected:", e)
	}
	if s.World.Active.Enemies.Slots[11].Kind != 17 || s.World.Active.Enemies.Slots[11].Attributes != 0xfc {
		t.Fatal("source projectile family byte not copied to real roller")
	}
	if s.Random.Frames != v.Frame {
		t.Fatal("next rule frame not committed")
	}
	if e := s.Validate(); e != nil {
		t.Fatal(e)
	}
}

func TestSessionBase2ThrowRecordedPrefixE2E(t *testing.T) {
	v := loadBaseThrowSession(t)
	f, e := os.Open("testdata/session-base2-throw-prefix-pads.bin.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	pads, e := io.ReadAll(z)
	if e != nil {
		t.Fatal(e)
	}
	if len(pads) != 26904*2 {
		t.Fatal("incomplete legal input prefix")
	}
	s, e := NewSession(42, 1, v.Before.Audio.Epoch)
	if e != nil {
		t.Fatal(e)
	}
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	refill := 0
	applyRefills := func(frame uint64) {
		for refill < len(v.Refills) && v.Refills[refill].Frame == frame {
			r := v.Refills[refill]
			p := &s.World.Active.Combat.Players[r.Actor]
			if p.Lives != r.Before {
				t.Fatal("declared reserve grant no longer matches source", frame, p.Lives, r.Before)
			}
			p.Lives = r.After
			refill++
		}
	}
	applyRefills(0)
	for n := uint64(1); n <= 26904; n++ {
		res, e := bundle.Stage(s.World.Active.Combat.World.Level)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.StepPads(n, [2]Buttons{Buttons(pads[(n-1)*2]), Buttons(pads[(n-1)*2+1])}, res); e != nil {
			t.Fatal(n, e)
		}
		applyRefills(n)
	}
	if refill != len(v.Refills) {
		t.Fatal("reserve grant history incomplete")
	}
	a, b := s.World.Active, v.Before.World.Active
	for name, pair := range map[string][2]any{"combat": {a.Combat, b.Combat}, "campaign": {s.World.Campaign, v.Before.World.Campaign}, "random": {s.Random, v.Before.Random}, "enemies": {a.Enemies, b.Enemies}, "terrain": {a.Terrain, b.Terrain}, "scores": {a.Scores, b.Scores}, "controls": {s.Controls, v.Before.Controls}, "presentation": {a.Presentation, b.Presentation}, "audio": {s.Audio, v.Before.Audio}} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Fatal("recorded input prefix changed existing state", name)
		}
	}
	res, _ := bundle.Stage(3)
	if _, e = s.StepPads(v.Frame, v.Pads, res); e != nil {
		t.Fatal(e)
	}
	if s.World.Active.Enemies.Slots[11].Attributes != 0xfc {
		t.Fatal("continuous producer/consumer differs")
	}
	t.Logf("%d legal frames plus %d explicitly declared reserve grants; failing next frame now commits", s.Random.Frames, len(v.Refills))
}
