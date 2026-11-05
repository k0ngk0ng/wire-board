package pcmplayer

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"testing"
)

//go:embed testdata/original-player.json.gz
var originalPlayer []byte

type oracleSample struct {
	ID      string `json:"id"`
	Encoded string `json:"encoded_hex"`
	Rate    uint8  `json:"rate"`
}
type oracleEvent struct {
	Cycle  uint64 `json:"cycle"`
	Kind   string `json:"kind"`
	Value  uint8  `json:"value"`
	Slot   uint8  `json:"slot"`
	Offset uint32 `json:"offset"`
}
type oracleFinal struct {
	Consumed     [2]uint32 `json:"consumed"`
	Fetched      [2]uint32 `json:"fetched"`
	Accumulator  [2]uint8  `json:"accumulator"`
	Dummy        uint32    `json:"dummy"`
	PrefetchSlot uint8     `json:"prefetch_slot"`
	Time         uint64    `json:"time"`
}
type oracleCase struct {
	Name, Mode string
	Samples    []*oracleSample
	Events     []oracleEvent
	Final      oracleFinal
}

func cases(t *testing.T) []oracleCase {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(originalPlayer))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	raw, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Version int
		Cases   []oracleCase
	}
	if err = json.Unmarshal(raw, &body); err != nil || body.Version != 1 {
		t.Fatal(err)
	}
	return body.Cases
}
func config(t *testing.T, c oracleCase) Config {
	t.Helper()
	v := Config{Mode: Dual}
	if c.Mode == "single" {
		v.Mode = Single
	}
	for i, s := range c.Samples {
		if s == nil {
			continue
		}
		b, err := hex.DecodeString(s.Encoded)
		if err != nil {
			t.Fatal(err)
		}
		v.Samples[i] = &Sample{ID: s.ID, Encoded: b, Rate: s.Rate}
	}
	return v
}
func normalized(events []Event) []oracleEvent {
	out := make([]oracleEvent, len(events))
	for i, e := range events {
		out[i] = oracleEvent{Cycle: e.Cycle, Kind: string(e.Kind), Value: e.Value, Slot: e.Slot, Offset: e.Offset}
	}
	return out
}
func TestContinuousOriginalPlayerEventsAndOwnBoundaryRestores(t *testing.T) {
	total := 0
	for _, c := range cases(t) {
		t.Run(c.Name, func(t *testing.T) {
			cfg := config(t, c)
			p, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			var output []oracleEvent
			steps := 0
			for !p.Stopped() {
				before := p.Snapshot()
				if steps%97 == 0 {
					p, err = Restore(cfg, before)
					if err != nil {
						t.Fatal(err)
					}
				}
				es, err := p.Next()
				if err != nil {
					t.Fatal(err)
				}
				if len(es) > 6 {
					t.Fatal("unbounded semantic step", len(es))
				}
				for _, e := range es {
					if e.StartCycle > e.Cycle {
						t.Fatal("event precedes its semantic operation")
					}
					if e.Kind == Bank || e.Kind == Dummy {
						lead := uint64(16 * 15)
						if e.Kind == Dummy {
							lead = 19 * 15
						}
						if e.Cycle-e.StartCycle != lead {
							t.Fatal("bank operation boundary changed", e)
						}
					}
				}
				output = append(output, normalized(es)...)
				steps++
				if steps > 40000 {
					t.Fatal("native finite player failed to stop")
				}
			}
			if !reflect.DeepEqual(output, c.Events) {
				for i := 0; i < len(output) && i < len(c.Events); i++ {
					if output[i] != c.Events[i] {
						t.Fatalf("first original event difference at %d: actual=%+v native=%+v", i, output[i], c.Events[i])
					}
				}
				t.Fatalf("event count actual=%d native=%d", len(output), len(c.Events))
			}
			state := p.Snapshot()
			if state.Time != c.Final.Time {
				t.Fatal("native stop completion time", state.Time, c.Final.Time)
			}
			if c.Mode == "single" {
				s := state.Single
				if s == nil || state.Dual != nil || s.Consumed != c.Final.Consumed[0] || s.Fetched != c.Final.Fetched[0] || s.Accumulator != c.Final.Accumulator[0] || s.DummyCursor != c.Final.Dummy {
					t.Fatalf("single final state differs: %+v expected %+v", s, c.Final)
				}
			} else {
				if state.Dual == nil || state.Single != nil || state.Dual.PrefetchSlot != c.Final.PrefetchSlot {
					t.Fatal("dual variant/phase differs")
				}
				for i, s := range state.Dual.Voices {
					if s.Consumed != c.Final.Consumed[i] || s.Fetched != c.Final.Fetched[i] || s.Accumulator != c.Final.Accumulator[i] || s.Active || s.Enabled {
						t.Fatalf("dual final state slot%d: %+v expected %+v", i, s, c.Final)
					}
				}
			}
			before := p.Snapshot()
			if _, err = p.Next(); err == nil || !reflect.DeepEqual(before, p.Snapshot()) {
				t.Fatal("finished player mutation")
			}
			total += len(output)
			t.Logf("native continuous %d events, %d byte steps, original stop %d master cycles", len(output), steps, state.Time)
		})
	}
	t.Logf("all %d native events match", total)
}
func TestConstructionBoundaryValidationAndResourceOwnership(t *testing.T) {
	c := cases(t)[0]
	base := config(t, c)
	mutations := []struct {
		name   string
		change func(*Config)
	}{
		{"mode", func(v *Config) { v.Mode = 99 }}, {"no resources", func(v *Config) { v.Samples = [2]*Sample{} }},
		{"digest", func(v *Config) { s := *v.Samples[0]; s.ID = fmt.Sprintf("%064x", 1); v.Samples[0] = &s }},
		{"short dual", func(v *Config) {
			b := make([]byte, 68)
			h := sha256.Sum256(b)
			v.Samples[0] = &Sample{ID: hex.EncodeToString(h[:]), Encoded: b}
		}},
		{"pan", func(v *Config) { v.RestorePan = 1 }}, {"clock overflow", func(v *Config) { v.Origin = math.MaxUint64 - 1 }},
	}
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			v := base
			m.change(&v)
			if p, e := New(v); e == nil || p != nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	single := config(t, cases(t)[4])
	if single.Mode != Single {
		t.Fatal("fixture ordering")
	}
	v := single
	v.Samples[1] = base.Samples[0]
	if _, e := New(v); e == nil {
		t.Fatal("single accepted dual resource")
	}
	for _, cfg := range []Config{base, single} {
		p, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		q, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Samples[0].Encoded[0] ^= 255
		for i := 0; i < 100; i++ {
			a, ea := p.Next()
			b, eb := q.Next()
			if ea != nil || eb != nil || !reflect.DeepEqual(a, b) {
				t.Fatal("caller resource mutated player")
			}
		}
		s := p.Snapshot()
		if s.Dual != nil {
			s.Dual.Voices[0].Accumulator ^= 255
		} else {
			s.Single.Accumulator ^= 255
		}
		if reflect.DeepEqual(s, p.Snapshot()) {
			t.Fatal("snapshot aliases canonical state")
		}
	}
}
func TestRestoreRejectsCrossResourceVariantAndImpossibleCursorsAtomically(t *testing.T) {
	cfg := config(t, cases(t)[0])
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	before := p.Snapshot()
	changes := []struct {
		name   string
		change func(*State)
	}{
		{"identity", func(s *State) { s.Identity = "other" }}, {"mixed variants", func(s *State) { s.Single = &SingleState{} }},
		{"phase", func(s *State) { s.Dual.PrefetchSlot = 2 }}, {"partial prefetch", func(s *State) { s.Dual.Voices[0].Fetched = 1 }},
		{"consumed ahead", func(s *State) { s.Dual.Voices[0].Consumed = 1 }}, {"nil slot active", func(s *State) { s.Dual.Voices[1].Active = true }},
		{"time overflow", func(s *State) { s.Time = math.MaxUint64 }},
	}
	for _, c := range changes {
		t.Run(c.name, func(t *testing.T) {
			s := p.Snapshot()
			c.change(&s)
			if x, e := Restore(cfg, s); e == nil || x != nil {
				t.Fatal("invalid snapshot accepted")
			}
			if !reflect.DeepEqual(before, p.Snapshot()) {
				t.Fatal("restore mutated existing owner")
			}
		})
	}
	cfg2 := config(t, cases(t)[1])
	if _, err = Restore(cfg2, before); err == nil {
		t.Fatal("different payload restored state")
	}
}
