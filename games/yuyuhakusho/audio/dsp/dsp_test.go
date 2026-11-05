package dsp

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"os"
	"reflect"
	"testing"
)

type nativeFrame struct {
	Events              []Event
	PCM                 []byte
	State               State
	NativeOffset        uint64
	NativeNextChipDelta uint64
}
type nativeCase struct {
	Name         string
	Config       Config
	Initial      State
	Frames       []nativeFrame
	PCM_SHA256   string
	NativeConfig struct {
		HQ     uint8 `json:"hq_fm"`
		Filter uint8 `json:"filter"`
		Model  uint8 `json:"ym2612"`
		Preamp int16 `json:"fm_preamp"`
		Rate   int   `json:"sample_rate"`
		Mono   uint8 `json:"mono"`
	}
}
type nativeOracle struct {
	Schema                  int
	Kernel                  [33][8]int16
	Cases                   []nativeCase
	CoefficientSourceSHA256 string
}

func oracle(t *testing.T) nativeOracle {
	t.Helper()
	packed, err := os.ReadFile("testdata/native-frames.json.gz")
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
	metaRaw, err := os.ReadFile("testdata/native-frames-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Pass   bool
		Raw    string `json:"fixture_json_sha256"`
		Packed string `json:"fixture_gzip_sha256"`
	}
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		t.Fatal(err)
	}
	a, b := sha256.Sum256(raw), sha256.Sum256(packed)
	if !meta.Pass || hex.EncodeToString(a[:]) != meta.Raw || hex.EncodeToString(b[:]) != meta.Packed {
		t.Fatal("unverified nativeDSP oracle")
	}
	var result nativeOracle
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.Schema != 1 || len(result.Cases) != 9 {
		t.Fatal("lost nativeDSP case coverage")
	}
	return result
}
func pcmBytes(samples []int16) []byte {
	out := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(out[i*2:], uint16(s))
	}
	return out
}
func assertFrame(t *testing.T, stream *Stream, frame nativeFrame) {
	t.Helper()
	got, err := stream.FrameBoundary()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pcmBytes(got), frame.PCM) {
		t.Fatal("native continuous44100 PCM differs", len(got), len(frame.PCM)/2)
	}
	if state := stream.Snapshot(); !reflect.DeepEqual(state, frame.State) {
		t.Fatalf("native typedDSP state differs: got%+v want%+v", state, frame.State)
	}
}
func TestNativeFramePCMAndCanonicalState(t *testing.T) {
	data := oracle(t)
	totalFrames, totalStereo := 0, 0
	seen := map[string]bool{}
	for _, test := range data.Cases {
		t.Run(test.Name, func(t *testing.T) {
			if seen[test.Name] || test.NativeConfig.HQ != 1 || test.NativeConfig.Filter != 0 || test.NativeConfig.Model != 0 || test.NativeConfig.Preamp != 100 || test.NativeConfig.Rate != 44100 || test.NativeConfig.Mono != 0 {
				t.Fatal("wrong or duplicate actual native profile")
			}
			seen[test.Name] = true
			stream, err := New(test.Config, test.Initial)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.New()
			for _, frame := range test.Frames {
				before := stream.Snapshot()
				if err := stream.Queue(frame.Events); err != nil {
					t.Fatal(err)
				}
				queued := stream.Snapshot()
				queued.Pending = before.Pending
				if !reflect.DeepEqual(queued, before) {
					t.Fatal("Queue applied unflushedDSP state")
				}
				got, err := stream.FrameBoundary()
				if err != nil || !bytes.Equal(pcmBytes(got), frame.PCM) {
					t.Fatal("actual nativeDSP PCM differs", frame.State.Time, err)
				}
				if len(got)%2 != 0 || len(got)/2 > 736 || !reflect.DeepEqual(stream.Snapshot(), frame.State) {
					t.Fatal("nativeDSP state/output geometry differs", frame.State.Time)
				}
				hash.Write(pcmBytes(got))
				totalFrames++
				totalStereo += len(got) / 2
			}
			if hex.EncodeToString(hash.Sum(nil)) != test.PCM_SHA256 {
				t.Fatal("complete sourcePCM trace changed")
			}
		})
	}
	if totalFrames != 441 || totalStereo != 324556 {
		t.Fatal("lost complete nativeDSP waveforms", totalFrames, totalStereo)
	}
}
func TestHalfFrameQueueSnapshotAndRestore(t *testing.T) {
	for _, test := range oracle(t).Cases {
		t.Run(test.Name, func(t *testing.T) {
			split, err := New(test.Config, test.Initial)
			if err != nil {
				t.Fatal(err)
			}
			whole, _ := New(test.Config, test.Initial)
			for _, frame := range test.Frames {
				cut := len(frame.Events) / 2
				first := append([]Event(nil), frame.Events[:cut]...)
				second := append([]Event(nil), frame.Events[cut:]...)
				if err := split.Queue(first); err != nil {
					t.Fatal(err)
				}
				saved := split.Snapshot()
				resumed, err := New(test.Config, saved)
				if err != nil {
					t.Fatal(err)
				}
				if len(first) > 0 {
					first[0].Value ^= 255
				}
				if len(saved.Pending) > 0 {
					saved.Pending[0].Value ^= 255
				}
				if err := split.Queue(second); err != nil {
					t.Fatal(err)
				}
				if err := resumed.Queue(second); err != nil {
					t.Fatal(err)
				}
				if err := whole.Queue(frame.Events); err != nil {
					t.Fatal(err)
				}
				if len(second) > 0 {
					second[0].Value ^= 255
				}
				a, err := split.FrameBoundary()
				if err != nil {
					t.Fatal(err)
				}
				b, err := resumed.FrameBoundary()
				if err != nil {
					t.Fatal(err)
				}
				c, err := whole.FrameBoundary()
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(pcmBytes(a), frame.PCM) || !reflect.DeepEqual(a, b) || !reflect.DeepEqual(a, c) || !reflect.DeepEqual(split.Snapshot(), resumed.Snapshot()) || !reflect.DeepEqual(split.Snapshot(), whole.Snapshot()) {
					t.Fatal("packet split or own-state restoration changed nativeDSP")
				}
			}
		})
	}
}
func TestSameCycleInputOrderIsNotSortedOrDropped(t *testing.T) {
	data := oracle(t)
	for _, test := range data.Cases {
		if test.Name != "same-cycle" {
			continue
		}
		frame := test.Frames[0]
		stream, err := New(test.Config, test.Initial)
		if err != nil {
			t.Fatal(err)
		}
		reversed := append([]Event(nil), frame.Events...)
		for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
			reversed[i], reversed[j] = reversed[j], reversed[i]
		}
		if err := stream.Queue(reversed); err != nil {
			t.Fatal(err)
		}
		got, err := stream.FrameBoundary()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(pcmBytes(got), frame.PCM) {
			t.Fatal("different same-cycle register order was erased")
		}
		return
	}
	t.Fatal("missing same-cycle native source")
}
func TestDerivedPhaseAndNoDuplicateOwners(t *testing.T) {
	data := oracle(t)
	for _, test := range data.Cases {
		for _, frame := range test.Frames {
			absolute := test.Config.MasterOrigin + frame.State.Time
			if phaseOffset(absolute) != frame.NativeOffset || nextChipDelta(absolute) != frame.NativeNextChipDelta {
				t.Fatal("derived native offset/chip phase differs", test.Name, frame.State.Time)
			}
		}
	}
	typ := reflect.TypeFor[State]()
	for _, field := range []string{"Offset", "NextChipTick", "LowPass", "Registers", "RAM", "Program", "NativeContext", "Factor"} {
		if _, ok := typ.FieldByName(field); ok {
			t.Fatal("duplicate derived/native state owner", field)
		}
	}
	if len(data.Kernel) != len(stepKernel) || data.Kernel != stepKernel {
		t.Fatal("fixed native mathematical coefficient profile differs")
	}
}
func TestInvalidProfileStateEventAndOverflowAreAtomic(t *testing.T) {
	test := oracle(t).Cases[0]
	for _, profile := range []Profile{0, 2, 255} {
		cfg := test.Config
		cfg.Profile = profile
		if _, err := New(cfg, test.Initial); err == nil {
			t.Fatal("unknownDSP profile accepted")
		}
	}
	for _, kind := range []string{"pan", "ahead", "time", "last-low", "last-high", "pending-order", "pending-kind", "origin-overflow"} {
		cfg, initial := test.Config, test.Initial
		switch kind {
		case "pan":
			initial.Pan = 1
		case "ahead":
			initial.InputAhead = 1009
		case "time":
			initial.Time = 1
		case "last-low":
			initial.LastChipOutput[0] = -7649
		case "last-high":
			initial.LastChipOutput[0] = 8897
		case "pending-order":
			initial.Pending = []Event{{Cycle: 20, Kind: DACData, Value: 1}, {Cycle: 19, Kind: DACData, Value: 2}}
		case "pending-kind":
			initial.Pending = []Event{{Kind: EventKind(99)}}
		case "origin-overflow":
			cfg.MasterOrigin = math.MaxUint64
			initial.Time = 896040
		}
		if _, err := New(cfg, initial); err == nil {
			t.Fatal("invalid typedDSP state accepted", kind)
		}
	}
	stream, err := New(test.Config, test.Initial)
	if err != nil {
		t.Fatal(err)
	}
	valid := []Event{{Cycle: 10, Kind: DACData, Value: 0}, {Cycle: 20, Kind: DACEnable, Value: 128}}
	if err := stream.Queue(valid); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]Event{{{Cycle: 19, Kind: DACData}}, {{Cycle: 21, Kind: EventKind(0)}}, {{Cycle: 21, Kind: DACEnable, Value: 1}}, {{Cycle: 21, Kind: DACPan, Value: 1}}, {{Cycle: 896040 + 1009, Kind: DACData}}, {{Cycle: 30, Kind: DACData}, {Cycle: 29, Kind: DACData}}, make([]Event, 4097)} {
		before := stream.Snapshot()
		if err := stream.Queue(invalid); err == nil || !reflect.DeepEqual(stream.Snapshot(), before) {
			t.Fatal("invalid event queue was not rejected atomically", invalid)
		}
	}
	for _, origin := range []uint64{math.MaxUint64, math.MaxUint64 - 100} {
		cfg := test.Config
		cfg.MasterOrigin = origin
		s, err := New(cfg, test.Initial)
		if err != nil {
			t.Fatal("valid origin with no advanced state should be constructible", err)
		}
		before := s.Snapshot()
		out, err := s.FrameBoundary()
		if err == nil || len(out) != 0 || !reflect.DeepEqual(before, s.Snapshot()) {
			t.Fatal("master clock overflow partially committedDSP")
		}
	}
	// Native private boundary has an exact1008-cycle raw-input lookahead. A
	// next-frame write one cycle earlier is invalid; no timestamp is clamped.
	for _, boundary := range oracle(t).Cases {
		if boundary.Name != "ahead-boundary" {
			continue
		}
		s, err := New(boundary.Config, boundary.Initial)
		if err != nil {
			t.Fatal(err)
		}
		for _, frame := range boundary.Frames[:3] {
			if err := s.Queue(frame.Events); err != nil {
				t.Fatal(err)
			}
			assertFrame(t, s, frame)
		}
		before := s.Snapshot()
		if before.InputAhead != 1008 {
			t.Fatal("native lookahead owner missing")
		}
		if err := s.Queue([]Event{{Cycle: before.Time + 1007, Kind: DACData}}); err == nil || !reflect.DeepEqual(before, s.Snapshot()) {
			t.Fatal("stale cross-frame raw write accepted")
		}
	}
}
