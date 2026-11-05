package pcmcommands

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/dsp"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/phaseclock"
)

//go:embed testdata/original-command-v1.json.gz
var originalCommands []byte

type nativeEvent struct {
	Cycle     uint64
	Kind      string
	Value     uint8
	PayloadID string
	Offset    uint32
	Bus       *phaseclock.Effect
}
type nativeFrame struct {
	Events []nativeEvent
	PCMHex string
	DSP    dsp.State
	Status Status
}
type nativeCase struct {
	Name         string
	Payloads     map[string]string
	Descriptors  []Descriptor
	Inputs       []Input
	Transactions []Transaction
	Frames       []nativeFrame
	MasterOrigin uint64
	InitialDSP   dsp.State
	SourceSHA256 string
}

func nativeCases(t *testing.T) []nativeCase {
	t.Helper()
	z, e := gzip.NewReader(bytes.NewReader(originalCommands))
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	raw, e := io.ReadAll(z)
	if e != nil {
		t.Fatal(e)
	}
	var b struct {
		Version int
		Cases   []nativeCase
	}
	if e = json.Unmarshal(raw, &b); e != nil || b.Version != 1 {
		t.Fatal(e)
	}
	if len(b.Cases) != 12 {
		t.Fatal("native matrix incomplete")
	}
	return b.Cases
}
func nativeConfig(t *testing.T, c nativeCase) Config {
	t.Helper()
	payloads := make(map[string][]byte)
	for id, h := range c.Payloads {
		b, e := hex.DecodeString(h)
		if e != nil {
			t.Fatal(e)
		}
		payloads[id] = b
	}
	identity := sha256.Sum256([]byte(c.SourceSHA256))
	return Config{Resources: Resources{Identity: hex.EncodeToString(identity[:]), Length: func(id string) (uint32, bool) { b, ok := payloads[id]; return uint32(len(b)), ok }, ByteAt: func(id string, offset uint32) (byte, bool) {
		b, ok := payloads[id]
		if !ok || offset >= uint32(len(b)) {
			return 0, false
		}
		return b[offset], true
	}}, Descriptors: c.Descriptors}
}
func normalized(events []Event) []nativeEvent {
	v := make([]nativeEvent, len(events))
	for i, e := range events {
		v[i] = nativeEvent{Cycle: e.Cycle, Kind: string(e.Kind), Value: e.Value, PayloadID: e.PayloadID, Offset: e.Offset, Bus: e.Bus}
	}
	return v
}
func TestOriginalContinuousCommandsFrameRestoreAndDACDSP(t *testing.T) {
	totalEvents, totalPCM := 0, 0
	for _, c := range nativeCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			cfg := nativeConfig(t, c)
			d, e := New(cfg)
			if e != nil {
				t.Fatal(e)
			}
			renderer, e := dsp.New(dsp.Config{Profile: dsp.ReferenceFrameDiscreteHQ44100, MasterOrigin: c.MasterOrigin}, c.InitialDSP)
			if e != nil {
				t.Fatal(e)
			}
			var transactions []Transaction
			for frame, expected := range c.Frames {
				// A real serialized frame boundary, including suspended startup/hold and
				// mid-byte work, must resume without an unexported goroutine or replay log.
				raw, e := json.Marshal(d.Snapshot())
				if e != nil {
					t.Fatal(e)
				}
				var checkpoint State
				if e = json.Unmarshal(raw, &checkpoint); e != nil {
					t.Fatal(e)
				}
				d, e = Restore(cfg, checkpoint)
				if e != nil {
					t.Fatal("restore", frame, e)
				}
				var inputs []Input
				for _, in := range c.Inputs {
					if in.Begin/FrameCycles == uint64(frame) {
						inputs = append(inputs, in)
					}
				}
				got, e := d.AdvanceFrame(FramePlan{Inputs: inputs})
				if e != nil {
					t.Fatal("advance", frame, e)
				}
				actual := normalized(got.Events)
				if !reflect.DeepEqual(actual, expected.Events) {
					for i := 0; i < len(actual) && i < len(expected.Events); i++ {
						if !reflect.DeepEqual(actual[i], expected.Events[i]) {
							t.Fatalf("first original event mismatch frame%d index%d actual=%+v native=%+v", frame, i, actual[i], expected.Events[i])
						}
					}
					t.Fatalf("event count frame%d actual=%d native=%d", frame, len(actual), len(expected.Events))
				}
				if got.Status != expected.Status || d.Status() != expected.Status {
					t.Fatalf("native status frame%d actual=%+v expected=%+v", frame, got.Status, expected.Status)
				}
				transactions = append(transactions, got.Transactions...)
				var chip []dsp.Event
				for _, event := range got.Events {
					var kind dsp.EventKind
					switch event.Kind {
					case DAC:
						kind = dsp.DACData
					case Pan:
						kind = dsp.DACPan
					case Enable:
						kind = dsp.DACEnable
					default:
						continue
					}
					chip = append(chip, dsp.Event{Cycle: event.Cycle, Kind: kind, Value: event.Value})
				}
				if e = renderer.Queue(chip); e != nil {
					t.Fatal("DSP admission", frame, e)
				}
				pcm, e := renderer.FrameBoundary()
				if e != nil {
					t.Fatal(e)
				}
				actualPCM := make([]byte, len(pcm)*2)
				for i, s := range pcm {
					binary.LittleEndian.PutUint16(actualPCM[i*2:], uint16(s))
				}
				expectedPCM, e := hex.DecodeString(expected.PCMHex)
				if e != nil {
					t.Fatal(e)
				}
				if !bytes.Equal(actualPCM, expectedPCM) {
					for i := 0; i < len(actualPCM) && i < len(expectedPCM); i++ {
						if actualPCM[i] != expectedPCM[i] {
							t.Fatalf("original PCM frame%d byte%d actual=%02x native=%02x", frame, i, actualPCM[i], expectedPCM[i])
						}
					}
					t.Fatal("PCM length differs")
				}
				if !reflect.DeepEqual(renderer.Snapshot(), expected.DSP) {
					t.Fatalf("original DSP state frame%d actual=%+v native=%+v", frame, renderer.Snapshot(), expected.DSP)
				}
				totalEvents += len(actual)
				totalPCM += len(pcm) / 2
			}
			if !reflect.DeepEqual(transactions, c.Transactions) {
				t.Fatalf("native bus grant/resume actual=%+v native=%+v", transactions, c.Transactions)
			}
			if d.Status().ModeFlags != 0 {
				t.Fatal("complete original command stream did not end")
			}
			t.Logf("native %d frames, %d input transactions", len(c.Frames), len(transactions))
		})
	}
	t.Logf("native original %d events, %d stereo PCM frames", totalEvents, totalPCM)
}
func TestConstructionInputAndSnapshotRejectionAreAtomic(t *testing.T) {
	c := nativeCases(t)[0]
	cfg := nativeConfig(t, c)
	for _, change := range []struct {
		name   string
		change func(*Config)
	}{
		{"reader absent", func(v *Config) { v.Resources.ByteAt = nil }}, {"identity absent", func(v *Config) { v.Resources.Identity = "" }}, {"duplicate descriptor", func(v *Config) { v.Descriptors = append(append([]Descriptor(nil), v.Descriptors...), v.Descriptors[0]) }}, {"unknown payload", func(v *Config) {
			v.Descriptors = append([]Descriptor(nil), v.Descriptors...)
			v.Descriptors[0].PayloadID = "unknown"
		}}, {"unknown protocol", func(v *Config) {
			v.Descriptors = append([]Descriptor(nil), v.Descriptors...)
			v.Descriptors[0].Protocol = 99
		}}, {"origin overflow", func(v *Config) { v.Origin = math.MaxUint64 - 1 }},
	} {
		t.Run(change.name, func(t *testing.T) {
			bad := cfg
			change.change(&bad)
			if p, e := New(bad); e == nil || p != nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	d, e := New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	before := d.Snapshot()
	valid := c.Inputs[0]
	for _, change := range []struct {
		name   string
		change func(*Input)
	}{
		{"unknown input", func(i *Input) { i.Kind = 99 }}, {"reverse interval", func(i *Input) { i.Begin = 10; i.End = 9 }}, {"late frame", func(i *Input) { i.Begin = FrameCycles; i.End = i.Begin + 10 }}, {"unknown descriptor", func(i *Input) { i.DescriptorID = "other" }}, {"unknown destination", func(i *Input) { i.Destination = 99 }},
	} {
		t.Run(change.name, func(t *testing.T) {
			bad := valid
			change.change(&bad)
			if _, e := d.AdvanceFrame(FramePlan{Inputs: []Input{bad}}); e == nil || !reflect.DeepEqual(before, d.Snapshot()) {
				t.Fatal("invalid input accepted or mutated state")
			}
		})
	}
	bad := valid
	bad.Begin = 1
	bad.End = 10
	overlap := bad
	overlap.Begin = 2
	overlap.End = 20
	if _, e = d.AdvanceFrame(FramePlan{Inputs: []Input{bad, overlap}}); e == nil || !reflect.DeepEqual(before, d.Snapshot()) {
		t.Fatal("overlap mutation")
	}
	if _, e = d.AdvanceFrame(FramePlan{DMA: []DMAInterval{{Begin: 0, End: 100, Mode: 99}}}); e == nil || !reflect.DeepEqual(before, d.Snapshot()) {
		t.Fatal("DMA validation mutation")
	}
	s := d.Snapshot()
	s.Identity = "other"
	if _, e = Restore(cfg, s); e == nil {
		t.Fatal("snapshot identity accepted")
	}
	s = d.Snapshot()
	s.Phase = "unknown"
	if _, e = Restore(cfg, s); e == nil {
		t.Fatal("unknown phase accepted")
	}
	s = d.Snapshot()
	s.Part = math.MaxUint16
	if _, e = Restore(cfg, s); e == nil {
		t.Fatal("impossible phase part accepted")
	}
	s = d.Snapshot()
	s.Time = math.MaxUint64
	if _, e = Restore(cfg, s); e == nil {
		t.Fatal("snapshot clock accepted")
	}
	if !reflect.DeepEqual(before, d.Snapshot()) {
		t.Fatal("rejected restore mutated owner")
	}
}
