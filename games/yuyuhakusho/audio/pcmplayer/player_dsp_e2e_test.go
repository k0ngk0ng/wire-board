package pcmplayer

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/dsp"
)

//go:embed testdata/original-player-dsp.json.gz
var originalPlayerDSP []byte

func TestAutonomousPlayerThroughDACDSPAgainstOriginalPCM(t *testing.T) {
	reader, err := gzip.NewReader(bytes.NewReader(originalPlayerDSP))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Cases []struct {
			Name, Mode   string
			Samples      []*oracleSample
			MasterOrigin uint64 `json:"master_origin"`
			Initial      dsp.State
			Frames       []struct {
				PCMHex string `json:"pcm_hex"`
				State  dsp.State
			}
			NonzeroSamples uint64 `json:"nonzero_samples"`
		}
	}
	if err = json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Cases) != 12 {
		t.Fatal("incomplete native audible matrix")
	}
	total := 0
	for _, c := range body.Cases {
		t.Run(c.Name, func(t *testing.T) {
			if c.NonzeroSamples == 0 {
				t.Fatal("native DAC remained silent")
			}
			cfg := config(t, oracleCase{Mode: c.Mode, Samples: c.Samples})
			p, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			packets := make([][]dsp.Event, len(c.Frames))
			for !p.Stopped() {
				events, err := p.Next()
				if err != nil {
					t.Fatal(err)
				}
				for _, e := range events {
					var k dsp.EventKind
					switch e.Kind {
					case DAC:
						k = dsp.DACData
					case Pan:
						k = dsp.DACPan
					case Enable:
						k = dsp.DACEnable
					default:
						continue
					}
					frame := e.StartCycle / dsp.FrameCycles
					// Explicit negative mode replays the tempting but incorrect frame-by-end
					// hypothesis. It must fail against unchanged native PCM/state evidence.
					if os.Getenv("YUYU_PCM_PLAYER_VERIFY_FRAME_END_ADMISSION") == "1" {
						frame = e.Cycle / dsp.FrameCycles
					}
					if frame >= uint64(len(packets)) {
						t.Fatal("event beyond native complete matrix")
					}
					packets[frame] = append(packets[frame], dsp.Event{Cycle: e.Cycle, Kind: k, Value: e.Value})
				}
			}
			dc := dsp.Config{Profile: dsp.ReferenceFrameDiscreteHQ44100, MasterOrigin: c.MasterOrigin}
			renderer, err := dsp.New(dc, c.Initial)
			if err != nil {
				t.Fatal(err)
			}
			for i, f := range c.Frames {
				if i%17 == 0 {
					renderer, err = dsp.New(dc, renderer.Snapshot())
					if err != nil {
						t.Fatal(err)
					}
				}
				if err = renderer.Queue(packets[i]); err != nil {
					t.Fatal("native admission", i, err)
				}
				pcm, err := renderer.FrameBoundary()
				if err != nil {
					t.Fatal(err)
				}
				actual := make([]byte, len(pcm)*2)
				for j, v := range pcm {
					binary.LittleEndian.PutUint16(actual[j*2:], uint16(v))
				}
				expected, err := hex.DecodeString(f.PCMHex)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(actual, expected) {
					for j := 0; j < len(actual) && j < len(expected); j++ {
						if actual[j] != expected[j] {
							t.Fatalf("first native PCM byte mismatch frame%d byte%d: actual=%02x original=%02x", i, j, actual[j], expected[j])
						}
					}
					t.Fatalf("native PCM size frame%d actual=%d original=%d", i, len(actual), len(expected))
				}
				if state := renderer.Snapshot(); !reflect.DeepEqual(state, f.State) {
					t.Fatalf("native typed DSP state frame%d actual=%+v original=%+v", i, state, f.State)
				}
				total += len(pcm) / 2
			}
			t.Logf("native autonomous player to DSP %d frames, %d nonzero native samples", len(c.Frames), c.NonzeroSamples)
		})
	}
	t.Logf("all %d native stereo PCM frames match", total)
}
