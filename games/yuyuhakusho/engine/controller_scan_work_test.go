package engine

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"testing"
)

type controllerScanNative struct {
	Input struct {
		Ports [2]uint8
		Held  [2][4]Buttons
	}
	Entry [2]uint64
	After string
	Steps [][2]uint64
	IO    []struct {
		Port           uint8
		Control, Write bool
		Value          uint8
		At             uint64
	}
}

func controllerScanFixture(t *testing.T, cold bool) []controllerScanNative {
	t.Helper()
	name := "native-controller-scan.json.gz"
	if cold {
		name = "cold-io-reset-controller-scan.json.gz"
	}
	packed, err := os.ReadFile("../verification/battle-sample-irq/" + name)
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
	var data struct {
		Version int
		Cases   []controllerScanNative
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Version != 1 || len(data.Cases) != 288 {
		t.Fatal("controller native profile missing")
	}
	return data.Cases
}
func TestBattleControllerScanOriginalWholeCalls(t *testing.T) {
	for _, cold := range []bool{false, true} {
		rows := controllerScanFixture(t, cold)
		steps := 0
		for index, row := range rows {
			input := ControllerScanInput{ColdIO: cold}
			for p := range input.Ports {
				input.Ports[p] = ControllerScanPort{Count: row.Input.Ports[p], Held: row.Input.Held[p]}
			}
			clock := row.Entry
			position, ioIndex := 0, 0
			result, err := ScanBattleControllers(input, func(op ControllerScanOperation) error {
				at := clock[0]
				if at >= clock[1] {
					at += 14
				}
				if op.IO != nil {
					if ioIndex >= len(row.IO) {
						t.Fatal("extra IO", cold, index, op)
					}
					want := row.IO[ioIndex]
					if op.IO.Port != want.Port || op.IO.Control != want.Control || op.IO.Write != want.Write || op.IO.Value != want.Value || at != want.At {
						t.Fatal("native IO differs", cold, index, ioIndex, op.IO, at, want)
					}
					ioIndex++
				}
				next, refresh, err := op.Work.Advance(clock[0], clock[1])
				if err != nil {
					return err
				}
				clock = [2]uint64{next, refresh}
				if position >= len(row.Steps) || clock != row.Steps[position] {
					t.Fatal("native scanner work differs", cold, index, position, op, clock)
				}
				position++
				return nil
			})
			if err != nil || position != len(row.Steps) || ioIndex != len(row.IO) {
				t.Fatal("incomplete scanner", cold, index, err, position, ioIndex)
			}
			raw := make([]byte, 80)
			flat := 0
			for p, count := range row.Input.Ports {
				start := p * 40
				if count == 4 {
					binary.BigEndian.PutUint16(raw[start:], 16)
					start += 8
				}
				for i := 0; i < int(count); i++ {
					sample := result.Samples[flat]
					flat++
					code := uint16(8)
					if sample.Device == MD3 {
						code = 4
					}
					binary.BigEndian.PutUint16(raw[start+i*8:], code)
					raw[start+i*8+2] = uint8(^sample.Held) & 15
					raw[start+i*8+3] = uint8(^sample.Held>>4) & 15
					if sample.Device == MD6 {
						raw[start+i*8+4] = uint8(^sample.Held>>8) & 15
					}
				}
			}
			if flat != int(result.Count) || hex.EncodeToString(raw) != row.After {
				t.Fatal("native packet differs", cold, index, result, hex.EncodeToString(raw), row.After)
			}
			steps += position
		}
		t.Logf("cold=%v whole scan calls=%d work steps=%d", cold, len(rows), steps)
	}
}
func TestBattleControllerScanFailuresAndReservedBits(t *testing.T) {
	good := ControllerScanInput{Ports: [2]ControllerScanPort{{Count: 4}, {Count: 1}}}
	sentinel := errors.New("scan hook failed")
	for _, failAt := range []int{0, 5, 50, 300} {
		calls := 0
		result, err := ScanBattleControllers(good, func(op ControllerScanOperation) error {
			i := calls
			calls++
			if i == failAt {
				return sentinel
			}
			return nil
		})
		if !errors.Is(err, sentinel) || calls != failAt+1 || !reflect.DeepEqual(result, ControllerScanResult{}) {
			t.Fatal("failed scanner leaked result or continued", failAt, calls, err)
		}
	}
	for _, kind := range []string{"count", "high-bits", "opposite-vertical", "opposite-horizontal", "unused-pad"} {
		input := good
		switch kind {
		case "count":
			input.Ports[0].Count = 2
		case "high-bits":
			input.Ports[0].Held[0] = 0x1000
		case "opposite-vertical":
			input.Ports[0].Held[0] = ButtonUp | ButtonDown
		case "opposite-horizontal":
			input.Ports[0].Held[0] = ButtonLeft | ButtonRight
		case "unused-pad":
			input.Ports[1].Held[1] = ButtonA
		}
		calls := 0
		if _, err := ScanBattleControllers(input, func(ControllerScanOperation) error { calls++; return nil }); err == nil || calls != 0 {
			t.Fatal("unsupported physical input performed work", kind, err, calls)
		}
	}
}
