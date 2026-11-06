package scoreclock_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/phaseclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

//go:embed testdata/bus-native-services.json.gz
var busNative []byte

type busAux struct {
	Cycle     uint64
	Kind      string
	Value     uint8
	PayloadID string
	Offset    uint32
	Register  uint16
	Bus       *phaseclock.Effect
}
type busAccess struct {
	Kind              string
	At, After         uint64
	Port, Value       uint8
	Asserted, Granted bool
}
type realServiceEnvironment struct {
	device *pcmcommands.Device
	chip   *chips.Interactive
	config pcmcommands.Config
	trace  []busAccess
	aux    []busAux
}

// These test-only data-time register expansions do not claim intermediate raw
// auxiliary address/latch equivalence. The actual Device owns every mailbox,
// bus grant, event, byte and clock; no original feedback table is consulted.
func (e *realServiceEnvironment) drain() error {
	for _, v := range e.device.DrainLedger() {
		e.aux = append(e.aux, busAux{Cycle: v.Cycle, Kind: string(v.Kind), Value: v.Value, PayloadID: v.PayloadID, Offset: v.Offset, Register: v.Register, Bus: v.Bus})
		reg := uint16(0)
		switch string(v.Kind) {
		case "dac":
			reg = 0x2a
		case "enable":
			reg = 0x2b
		case "pan":
			reg = 0x1b6
		case "ym-write":
			reg = v.Register
		case "bank", "metadata", "dummy":
			continue
		default:
			return fmt.Errorf("unverified auxiliary kind %s", v.Kind)
		}
		port := uint8(reg>>8) * 2
		if err := e.chip.WritePort(v.Cycle, port, uint8(reg)); err != nil {
			return err
		}
		if err := e.chip.WritePort(v.Cycle, port+1, v.Value); err != nil {
			return err
		}
	}
	return nil
}
func (e *realServiceEnvironment) restoreHeld() error {
	// A real serialized open bus state and one pending output owner, not a
	// preinitialized original RAM image or a replayed mailbox schedule.
	raw, err := json.Marshal(e.device.Snapshot())
	if err != nil {
		return err
	}
	var state pcmcommands.State
	if err = json.Unmarshal(raw, &state); err != nil {
		return err
	}
	e.device, err = pcmcommands.Restore(e.config, state)
	return err
}
func (e *realServiceEnvironment) RequestBus(at uint64, asserted bool) (uint64, error) {
	after, err := e.device.RequestBus(at, asserted)
	if err != nil {
		return 0, err
	}
	if err = e.restoreHeld(); err != nil {
		return 0, err
	}
	if err = e.drain(); err != nil {
		return 0, err
	}
	e.trace = append(e.trace, busAccess{Kind: "request", At: at, After: after, Asserted: asserted})
	return after, nil
}
func (e *realServiceEnvironment) BusGranted(at uint64) (bool, error) {
	v, err := e.device.BusGranted(at)
	e.trace = append(e.trace, busAccess{Kind: "ack", At: at, Granted: v})
	return v, err
}
func (e *realServiceEnvironment) Mailbox(at uint64) (uint8, error) {
	v, err := e.device.Mailbox(at)
	e.trace = append(e.trace, busAccess{Kind: "mailbox", At: at, Value: v})
	return v, err
}
func (e *realServiceEnvironment) ReadStatus(at uint64, port uint8) (uint8, error) {
	v, err := e.chip.ReadStatus(at, port)
	e.trace = append(e.trace, busAccess{Kind: "status", At: at, Port: port, Value: v})
	return v, err
}
func (e *realServiceEnvironment) WritePort(at uint64, port, value uint8) error {
	err := e.chip.WritePort(at, port, value)
	e.trace = append(e.trace, busAccess{Kind: "write", At: at, Port: port, Value: value})
	return err
}

func TestRealPCMDeviceBusAndYMServiceOriginalIntegration(t *testing.T) {
	z, err := gzip.NewReader(bytes.NewReader(busNative))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version  int
		Resource struct {
			ID, EncodedHex string
			Descriptor     pcmcommands.Descriptor
		}
		Cases []struct {
			Input struct {
				ID                    string
				Origin                uint64
				Bank, Register, Value uint8
			}
			Before struct {
				Frontier, M68KDelay uint64
				Busy                uint8
			}
			Main []busAccess
			Aux  []busAux
			End  struct {
				Clock    scoreclock.Clock
				Frontier uint64
				Status   pcmcommands.Status
			}
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Cases) != 4 {
		t.Fatal("real integration fixture incomplete")
	}
	encoded, err := hex.DecodeString(fixture.Resource.EncodedHex)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(encoded)
	if hex.EncodeToString(sum[:]) != fixture.Resource.ID {
		t.Fatal("resource identity changed")
	}
	cfg := pcmcommands.Config{Resources: pcmcommands.Resources{Identity: fixture.Resource.ID, Length: func(id string) (uint32, bool) { return uint32(len(encoded)), id == fixture.Resource.ID }, ByteAt: func(id string, at uint32) (byte, bool) {
		if id != fixture.Resource.ID || at >= uint32(len(encoded)) {
			return 0, false
		}
		return encoded[at], true
	}}, Descriptors: []pcmcommands.Descriptor{fixture.Resource.Descriptor}}
	busyReads, mainCount, auxCount := 0, 0, 0
	for _, row := range fixture.Cases {
		t.Run(row.Input.ID, func(t *testing.T) {
			d, err := pcmcommands.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = d.RequestBus(0, true); err != nil {
				t.Fatal(err)
			}
			if err = d.PublishAccepted(pcmcommands.Input{Kind: pcmcommands.Ordinary, DescriptorID: fixture.Resource.Descriptor.ID, Destination: pcmcommands.Single}); err != nil {
				t.Fatal(err)
			}
			if _, err = d.RequestBus(0, false); err != nil {
				t.Fatal(err)
			}
			initial, err := d.RunUntil(row.Input.Origin)
			if err != nil {
				t.Fatal(err)
			}
			if initial.Frontier != row.Before.Frontier || initial.M68KDelay != row.Before.M68KDelay || d.Status().Busy != row.Before.Busy {
				t.Fatalf("real pre-service state %+v busy=%d, native %+v", initial, d.Status().Busy, row.Before)
			}
			chip, err := chips.NewInteractive(chips.Config{Profile: chips.ReferenceMixedHQ44100}, chips.State{YM: chips.ResetYMState(), PSG: psg.ResetState()})
			if err != nil {
				t.Fatal(err)
			}
			e := &realServiceEnvironment{device: d, chip: chip, config: cfg}
			if err = e.drain(); err != nil {
				t.Fatal(err)
			}
			// Startup steals are audited above. Main origin is declared; RequestBus
			// must return only the new steals after this origin, never the history.
			got, err := scoreclock.ServeYM(scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Origin}, scoreclock.Request{Bank: row.Input.Bank, Register: row.Input.Register, Value: row.Input.Value}, e, e, scoreclock.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if got.Clock != row.End.Clock || e.device.Snapshot().Time != row.End.Frontier || e.device.Status() != row.End.Status {
				t.Fatalf("real end clock/status %+v frontier%d status%+v native%+v", got.Clock, e.device.Snapshot().Time, e.device.Status(), row.End)
			}
			if !reflect.DeepEqual(e.trace, row.Main) {
				t.Fatalf("actual Main ports/bus/mailbox differ\nGo %+v\nNative %+v", e.trace, row.Main)
			}
			if len(e.aux) != len(row.Aux) {
				t.Fatalf("real auxiliary count %d native %d", len(e.aux), len(row.Aux))
			}
			for i, v := range e.aux {
				if !reflect.DeepEqual(v, row.Aux[i]) {
					t.Fatalf("real auxiliary event%d %+v native %+v", i, v, row.Aux[i])
				}
			}
			for _, v := range e.trace {
				if v.Kind == "mailbox" && v.Value != 0 {
					busyReads++
				}
			}
			mainCount += len(e.trace)
			auxCount += len(e.aux)
			if len(e.device.DrainLedger()) != 0 {
				t.Fatal("consumed output replayed")
			}
		})
	}
	t.Logf("4 true command-to-PCM-to-Main services: %d Main accesses, %d auxiliary events, %d actual busy mailbox reads", mainCount, auxCount, busyReads)
}
