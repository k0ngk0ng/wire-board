package chips_test

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
)

func TestNativeRawYMContinuousStateAndOutput(t *testing.T) {
	raw, err := os.ReadFile("../../verification/fmpsg-ym/ym-native.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	body, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Profile string
		Cases   []struct {
			Name    string
			Initial chips.YMState
			Steps   []struct {
				Writes [][2]uint8
				Count  int
				Output string
				State  chips.YMState
			}
		}
	}
	if err = json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Profile != "YM2612_DISCRETE_RAW_1008" || len(fixture.Cases) != 29 {
		t.Fatal("unexpected native fixture")
	}
	var total int
	for _, row := range fixture.Cases {
		t.Run(row.Name, func(t *testing.T) {
			if !reflect.DeepEqual(chips.ResetYMState(), row.Initial) {
				t.Fatalf("reset state differs\nGo=%+v\nNative=%+v", chips.ResetYMState(), row.Initial)
			}
			engine, err := chips.NewYM(row.Initial)
			if err != nil {
				t.Fatal(err)
			}
			for stepNo, step := range row.Steps {
				for _, write := range step.Writes {
					if err = engine.WritePort(write[0], write[1]); err != nil {
						t.Fatal(err)
					}
				}
				pcm, err := engine.Generate(step.Count)
				if err != nil {
					t.Fatal(err)
				}
				want, err := base64.StdEncoding.DecodeString(step.Output)
				if err != nil {
					t.Fatal(err)
				}
				if len(pcm)*4 != len(want) {
					t.Fatalf("step %d output size %d != %d", stepNo, len(pcm)*4, len(want))
				}
				for i, sample := range pcm {
					if sample != int32(binary.LittleEndian.Uint32(want[i*4:])) {
						t.Fatalf("step %d sample %d Go=%d Native=%d", stepNo, i, sample, int32(binary.LittleEndian.Uint32(want[i*4:])))
					}
				}
				if got := engine.Snapshot(); !reflect.DeepEqual(got, step.State) {
					left, _ := json.Marshal(got)
					right, _ := json.Marshal(step.State)
					t.Fatalf("step %d state differs\nGo=%s\nNative=%s", stepNo, left, right)
				}
				// A typed snapshot must be sufficient, with no native context,
				// register shadow or unrecorded pointers retained by the instance.
				restored, err := chips.NewYM(engine.Snapshot())
				if err != nil {
					t.Fatal(err)
				}
				engine = restored
				total += step.Count
			}
		})
	}
	if total != 616921 {
		t.Fatalf("native fixture did not cover full expected sample count: %d", total)
	}
}

func TestYMBoundaryRejectionsAreAtomic(t *testing.T) {
	state := chips.ResetYMState()
	ym, err := chips.NewYM(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, port := range []uint8{4, 5, 255} {
		if err = ym.WritePort(port, 17); err == nil {
			t.Fatalf("port %d accepted", port)
		}
		if !reflect.DeepEqual(state, ym.Snapshot()) {
			t.Fatal("invalid port partially changed chip")
		}
	}
	for _, n := range []int{-1, chips.MaxYMBlockSamples + 1} {
		if _, err = ym.Generate(n); err == nil {
			t.Fatal("invalid block accepted")
		}
		if !reflect.DeepEqual(state, ym.Snapshot()) {
			t.Fatal("invalid block changed chip")
		}
	}
	bad := []func(*chips.YMState){
		func(s *chips.YMState) { s.Address = 512 }, func(s *chips.YMState) { s.Status = 4 }, func(s *chips.YMState) { s.FrequencyLatch = 64 },
		func(s *chips.YMState) { s.SpecialFrequency[0] = 16384 }, func(s *chips.YMState) { s.TimerAValue = 1024 }, func(s *chips.YMState) { s.TimerACounter = -1 },
		func(s *chips.YMState) { s.EnvelopeCount = 4096 }, func(s *chips.YMState) { s.EnvelopeDivider = 3 }, func(s *chips.YMState) { s.LFOCycle = 128 },
		func(s *chips.YMState) { s.LFOSpeed = 8 }, func(s *chips.YMState) { s.LFODivider = 108 }, func(s *chips.YMState) { s.CSM = 4 },
		func(s *chips.YMState) { s.Channels[1].Pan = 1 }, func(s *chips.YMState) { s.Channels[2].Frequency = 16384 }, func(s *chips.YMState) { s.Channels[3].Algorithm = 8 },
		func(s *chips.YMState) { s.Channels[4].Feedback = 8 }, func(s *chips.YMState) { s.Channels[0].AMDepth = 4 }, func(s *chips.YMState) { s.Channels[0].PMDepth = 8 },
		func(s *chips.YMState) { s.Channels[0].Operators[0].Attack = 32 }, func(s *chips.YMState) { s.Channels[0].Operators[0].EnvelopePhase = 5 },
		func(s *chips.YMState) { s.Channels[0].Operators[0].LatchedKeyScale = 32 }, func(s *chips.YMState) { s.Channels[0].Operators[0].Envelope = 1032 },
		func(s *chips.YMState) { s.Channels[0].Operators[0].Level = 128 }, func(s *chips.YMState) { s.Channels[0].Operators[0].Detune = 8 },
		func(s *chips.YMState) { s.Channels[0].Operators[0].Multiple = 16 }, func(s *chips.YMState) { s.Channels[0].Operators[0].SSG = 16 },
	}
	for i, mutate := range bad {
		candidate := state
		mutate(&candidate)
		if _, err = chips.NewYM(candidate); err == nil {
			t.Fatalf("malformed state %d accepted", i)
		}
	}
	copy := ym.Snapshot()
	copy.Channels[0].Operators[0].Level = 99
	if !reflect.DeepEqual(state, ym.Snapshot()) {
		t.Fatal("snapshot aliases mutable operator state")
	}
}
