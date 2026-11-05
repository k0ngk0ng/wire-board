package chips_test

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
)

func TestNativeMixedFramePCMAndFullTypedState(t *testing.T) {
	raw, err := os.ReadFile("../../verification/fmpsg-mixed/native-frames.json.gz")
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
		Cases []struct {
			Name    string
			Config  chips.Config
			Initial chips.State
			Frames  []struct {
				Events []chips.Event
				Output string
				Reads  []chips.Readback
				State  chips.State
			}
		}
	}
	if err = json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 5 {
		t.Fatal("native mixed fixture coverage changed")
	}
	frames, samples := 0, 0
	for _, row := range fixture.Cases {
		t.Run(row.Name, func(t *testing.T) {
			engine, err := chips.NewStream(row.Config, row.Initial)
			if err != nil {
				t.Fatal(err)
			}
			for frameNo, frame := range row.Frames {
				// Restore after a partial packet as well as each complete frame.
				middle := len(frame.Events) / 2
				if err = engine.Queue(frame.Events[:middle]); err != nil {
					t.Fatal(err)
				}
				engine, err = chips.NewStream(row.Config, engine.Snapshot())
				if err != nil {
					t.Fatal(err)
				}
				if err = engine.Queue(frame.Events[middle:]); err != nil {
					t.Fatal(err)
				}
				got, err := engine.FrameBoundary()
				if err != nil {
					t.Fatalf("frame%d: %v", frameNo, err)
				}
				want, err := base64.StdEncoding.DecodeString(frame.Output)
				if err != nil {
					t.Fatal(err)
				}
				if len(got.PCM)*2 != len(want) {
					t.Fatalf("frame%d PCM size differs", frameNo)
				}
				for i, value := range got.PCM {
					if value != int16(binary.LittleEndian.Uint16(want[i*2:])) {
						t.Fatalf("frame%d sample%d Go%d Native%d", frameNo, i, value, int16(binary.LittleEndian.Uint16(want[i*2:])))
					}
				}
				if len(got.Reads) != len(frame.Reads) {
					t.Fatalf("frame%d read count differs", frameNo)
				}
				for i, value := range got.Reads {
					if value != frame.Reads[i] {
						t.Fatalf("frame%d read%d Go%+v Native%+v", frameNo, i, value, frame.Reads[i])
					}
				}
				if actual := engine.Snapshot(); !reflect.DeepEqual(actual, frame.State) {
					a, _ := json.Marshal(actual)
					b, _ := json.Marshal(frame.State)
					t.Fatalf("frame%d typed state differs\nGo=%s\nNative=%s", frameNo, a, b)
				}
				engine, err = chips.NewStream(row.Config, engine.Snapshot())
				if err != nil {
					t.Fatal(err)
				}
				frames++
				samples += len(got.PCM) / 2
			}
		})
	}
	if frames != 192 || samples < 141000 {
		t.Fatalf("incomplete native mixed frame coverage: %d frames %d samples", frames, samples)
	}
}

func mixedReset() chips.State { return chips.State{YM: chips.ResetYMState(), PSG: psg.ResetState()} }

func TestMixedQueueBoundaryValidationAndCopy(t *testing.T) {
	config := chips.Config{Profile: chips.ReferenceMixedHQ44100}
	s, err := chips.NewStream(config, mixedReset())
	if err != nil {
		t.Fatal(err)
	}
	input := []chips.Event{{Cycle: 1000, Kind: chips.YMPortWrite, Port: 0, Value: 0x2a}, {Cycle: 1000, Kind: chips.YMPortWrite, Port: 1, Value: 42}}
	if err = s.Queue(input); err != nil {
		t.Fatal(err)
	}
	before := s.Snapshot()
	input[0].Value = 17
	if !reflect.DeepEqual(s.Snapshot(), before) {
		t.Fatal("queued input alias")
	}
	view := s.Snapshot()
	view.Pending[0].Value = 99
	if !reflect.DeepEqual(s.Snapshot(), before) {
		t.Fatal("snapshot pending alias")
	}
	for _, events := range [][]chips.Event{
		{{Cycle: 1000, Kind: 99}}, {{Cycle: 1000, Kind: chips.YMPortWrite, Port: 4}}, {{Cycle: 1000, Kind: chips.PSGByteWrite, Port: 1}},
		{{Cycle: 1000, Kind: chips.YMStatusRead, Value: 1}}, {{Cycle: 999, Kind: chips.PSGByteWrite}}, {{Cycle: chips.FrameCycles + chips.MaxInputAhead + 1, Kind: chips.PSGByteWrite}},
		make([]chips.Event, chips.MaxPendingEvents+1),
	} {
		if err = s.Queue(events); err == nil {
			t.Fatal("invalid input batch accepted")
		}
		if !reflect.DeepEqual(before, s.Snapshot()) {
			t.Fatal("invalid queue partially committed")
		}
	}
	bad := []func(*chips.State){
		func(s *chips.State) { s.Time = 1 }, func(s *chips.State) { s.InputAhead = uint32(chips.MaxInputAhead + 1) }, func(s *chips.State) { s.PSG.NoiseShift = 0 },
		func(s *chips.State) { s.YM.Address = 512 }, func(s *chips.State) { s.LastFMOutput[0] = 60001 }, func(s *chips.State) { s.BusyUntil = math.MaxUint64 },
	}
	for i, change := range bad {
		state := mixedReset()
		change(&state)
		if _, err = chips.NewStream(config, state); err == nil {
			t.Fatalf("invalid mixed state%d accepted", i)
		}
	}
	if _, err = chips.NewStream(chips.Config{Profile: 99}, mixedReset()); err == nil {
		t.Fatal("unknown mixed profile accepted")
	}
	late, err := chips.NewStream(chips.Config{Profile: chips.ReferenceMixedHQ44100, MasterOrigin: math.MaxUint64 - 100}, mixedReset())
	if err != nil {
		t.Fatal(err)
	}
	if err = late.Queue([]chips.Event{{Cycle: 101, Kind: chips.PSGByteWrite}}); err == nil {
		t.Fatal("origin+event overflow accepted")
	}
	if _, err = late.FrameBoundary(); err == nil {
		t.Fatal("origin+frame overflow accepted")
	}
	if !reflect.DeepEqual(late.Snapshot(), mixedReset()) {
		t.Fatal("overflow frame committed")
	}
}
