package pcm

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

type nativeStage struct {
	Kind        string
	Role        string
	Encoded     uint8
	Accumulator uint8           `json:"native_accumulator_before"`
	Output      json.RawMessage `json:"native_output"`
}
type nativeRole struct {
	Code    uint8
	Encoded string `json:"encoded_hex"`
	SHA     string `json:"encoded_sha256"`
}
type nativeCase struct {
	Name         string
	Roles        map[string]nativeRole
	Stages       []nativeStage
	DACEvents    int `json:"native_dac_events"`
	DecoderPairs int `json:"native_decoder_pairs"`
}
type nativeFixture struct {
	Schema int
	Tables struct {
		ROM      string  `json:"source_rom_sha256"`
		Firmware string  `json:"source_firmware_sha256"`
		SHA      string  `json:"source_table_sha256"`
		Low      []uint8 `json:"delta_low"`
		High     []uint8 `json:"delta_high"`
		Mix      []uint8 `json:"mix"`
	}
	Cases []nativeCase
}

func readNativePCM(t *testing.T) nativeFixture {
	t.Helper()
	packed, err := os.ReadFile("testdata/original-values.json.gz")
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
	metaBytes, err := os.ReadFile("testdata/original-values-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Pass              bool
		Raw               string `json:"fixture_json_sha256"`
		Packed            string `json:"fixture_gzip_sha256"`
		Cases, Pairs, DAC int
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatal(err)
	}
	sum, zip := sha256.Sum256(raw), sha256.Sum256(packed)
	if !meta.Pass || hex.EncodeToString(sum[:]) != meta.Raw || hex.EncodeToString(zip[:]) != meta.Packed {
		t.Fatal("unverified native PCM fixture")
	}
	var f nativeFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if f.Schema != 1 || len(f.Cases) != 4 || f.Tables.ROM != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || f.Tables.Firmware != "42ed1d2c4133d88d36c4024ff535a710f064dcb5d791b892eb3a07f6ded6ef27" {
		t.Fatal("PCM source identity differs")
	}
	tables := append(append(append([]byte{}, f.Tables.Mix...), f.Tables.Low...), f.Tables.High...)
	tableSHA := sha256.Sum256(tables)
	if len(f.Tables.Low) != 256 || len(f.Tables.High) != 256 || len(f.Tables.Mix) != 512 || hex.EncodeToString(tableSHA[:]) != f.Tables.SHA || f.Tables.SHA != "be79e4eb4329b2f21dfda3d284a5b03afbd2dd8b1694c4366190072de0ef0fe8" {
		t.Fatal("original PCM lookup mother tables differ")
	}
	return f
}
func TestDecodeByteAgainstOriginalMotherTables(t *testing.T) {
	f := readNativePCM(t)
	if NewDecoder() != (DecoderState{Accumulator: 128}) {
		t.Fatal("native decoder must initialize to80")
	}
	for accumulator := 0; accumulator < 256; accumulator++ {
		for encoded := 0; encoded < 256; encoded++ {
			state := DecoderState{Accumulator: uint8(accumulator)}
			before := state
			next, out := DecodeByte(state, uint8(encoded))
			high := uint8(accumulator + int(f.Tables.High[encoded]))
			low := high + f.Tables.Low[encoded]
			if state != before || out != [2]uint8{high, low} || next.Accumulator != low {
				t.Fatal("original table decode/wrap/order differs", accumulator, encoded, next, out, high, low)
			}
		}
	}
}
func TestMixDualAgainstCompleteOriginalMotherTable(t *testing.T) {
	f := readNativePCM(t)
	for a := 0; a < 256; a++ {
		for b := 0; b < 256; b++ {
			if got := MixDual(uint8(a), uint8(b)); got != f.Tables.Mix[a+b] {
				t.Fatal("native DAC clamp differs", a, b, got, f.Tables.Mix[a+b])
			}
		}
	}
	for value := 0; value < 256; value++ {
		if MixDual(uint8(value), 128) != uint8(value) || MixDual(128, uint8(value)) != uint8(value) {
			t.Fatal("inactive128 contribution is not neutral")
		}
	}
}
func TestOriginalCompletePayloadsAndOrderedDACValues(t *testing.T) {
	f := readNativePCM(t)
	totalPairs, totalDAC, totalClamped := 0, 0, 0
	for _, test := range f.Cases {
		t.Run(test.Name, func(t *testing.T) {
			states := make(map[string]DecoderState)
			payloads := make(map[string][]byte)
			positions := make(map[string]int)
			for role, source := range test.Roles {
				payload, err := hex.DecodeString(source.Encoded)
				if err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256(payload)
				if len(payload) == 0 || hex.EncodeToString(sum[:]) != source.SHA {
					t.Fatal("invalid encoded sample source")
				}
				payloads[role] = payload
				states[role] = NewDecoder()
			}
			values := map[string][2]uint8{"a": {128, 128}, "b": {128, 128}, "single": {128, 128}}
			var pending uint8
			hasPending := false
			pairs, dacs := 0, 0
			for _, stage := range test.Stages {
				switch stage.Kind {
				case "decode":
					position := positions[stage.Role]
					payload := payloads[stage.Role]
					if position >= len(payload) || payload[position] != stage.Encoded {
						t.Fatal("native encoded order differs", stage.Role, position)
					}
					// Only the initial128 state is supplied. Never inject a later native
					// accumulator or use an independently saved expected state for continuation.
					state := states[stage.Role]
					if state.Accumulator != stage.Accumulator {
						t.Fatal("continuous source accumulator diverged", stage.Role, position)
					}
					next, out := DecodeByte(state, payload[position])
					var want [2]uint8
					if json.Unmarshal(stage.Output, &want) != nil || out != want {
						t.Fatal("actual native high/low output differs", stage.Role, position, out, want)
					}
					states[stage.Role], values[stage.Role] = next, out
					positions[stage.Role]++
					pairs++
				case "mix-first", "mix-second":
					if hasPending {
						t.Fatal("native DAC output missing after mix")
					}
					half := 0
					if stage.Kind == "mix-second" {
						half = 1
					}
					a, b := values["a"][half], values["b"][half]
					pending = MixDual(a, b)
					hasPending = true
					if int(a)+int(b) < 128 || int(a)+int(b) > 383 {
						totalClamped++
					}
					var want uint8
					if json.Unmarshal(stage.Output, &want) != nil || pending != want {
						t.Fatal("native mixed value differs", stage.Kind, a, b, pending, want)
					}
					if half == 1 {
						values["a"], values["b"] = [2]uint8{128, 128}, [2]uint8{128, 128}
					}
				case "dac", "single-first", "single-second":
					var want uint8
					if json.Unmarshal(stage.Output, &want) != nil {
						t.Fatal("invalid native DAC value")
					}
					if stage.Kind == "dac" {
						if !hasPending || pending != want {
							t.Fatal("actual nativeDAC write differs", pending, want)
						}
						hasPending = false
					} else {
						half := 0
						if stage.Kind == "single-second" {
							half = 1
						}
						if values["single"][half] != want {
							t.Fatal("actual native singleDAC output differs")
						}
					}
					dacs++
				default:
					t.Fatal("unknown native semantic test stage", stage.Kind)
				}
			}
			if hasPending || pairs != test.DecoderPairs || dacs != test.DACEvents {
				t.Fatal("lost complete native stage stream", pairs, dacs)
			}
			for role, payload := range payloads {
				if positions[role] != len(payload) {
					t.Fatal("original encoded payload did not finish", role, positions[role], len(payload))
				}
			}
			totalPairs += pairs
			totalDAC += dacs
		})
	}
	if totalPairs != 28510 || totalDAC != 41345 || totalClamped != 476 {
		t.Fatal("native codec/DAC/clamp coverage differs", totalPairs, totalDAC, totalClamped)
	}
}
func TestDecoderOwnsOnlyCanonicalAccumulator(t *testing.T) {
	typ := reflect.TypeFor[DecoderState]()
	if typ.NumField() != 1 || typ.Field(0).Name != "Accumulator" || typ.Field(0).Type.Kind() != reflect.Uint8 {
		t.Fatal("decoder owns duplicate or CPU/RAM state", typ)
	}
}
