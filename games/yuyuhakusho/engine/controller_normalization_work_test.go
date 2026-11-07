package engine

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
)

// Frozen before the production API. The generator executes complete original
// calls, compares stepping with whole native execution and checks both cores.
type controllerNormalizationNative struct {
	Name          string
	Input         ControllerNormalizationInput
	Before, After ControllerNormalizationState
	Entry         [2]uint64
	Cursor        uint8
	Steps         [][2]uint64
	Changes       []struct {
		AfterStep int `json:"after_step"`
		Ports     ControllerPorts
	}
}

func controllerNormalizationFixture(t *testing.T) []controllerNormalizationNative {
	t.Helper()
	packed, err := os.ReadFile("../verification/controller-normalization-work/native-first/native-controller-normalization.json.gz")
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
		Cases   []controllerNormalizationNative
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Version != 1 || len(data.Cases) != 689 {
		t.Fatal("complete controller normalization native profile missing", data.Version, len(data.Cases))
	}
	return data.Cases
}

func TestControllerNormalizationWorkOriginalWholeCalls(t *testing.T) {
	rows := controllerNormalizationFixture(t)
	totalSteps, liveCases := 0, 0
	var counts [9]bool
	var carries [16]bool
	for index, row := range rows {
		input, ports := row.Input, row.Input.Ports
		position, change := 0, 0
		clock := row.Entry
		if len(row.Changes) != 0 {
			liveCases++
			input.LivePorts = func() (ControllerPorts, error) { return ports, nil }
		}
		after, err := NormalizeControllersWork(row.Before, input, func(op MainWorkOperation) error {
			next, refresh, err := op.Advance(clock[0], clock[1])
			if err != nil {
				return err
			}
			clock = [2]uint64{next, refresh}
			if position >= len(row.Steps) || clock != row.Steps[position] {
				t.Fatalf("native normalization differs case=%d name=%s step=%d phase=%s op=%+v clock=%v", index, row.Name, position, op.Phase, op, clock)
			}
			position++
			if change < len(row.Changes) && position == row.Changes[change].AfterStep {
				ports = row.Changes[change].Ports
				change++
			}
			return nil
		})
		if err != nil || position != len(row.Steps) || change != len(row.Changes) || after != row.After {
			t.Fatalf("incomplete native normalization case=%d err=%v steps=%d/%d changes=%d/%d\ngot=%+v\nwant=%+v", index, err, position, len(row.Steps), change, len(row.Changes), after, row.After)
		}
		if row.Cursor != row.Input.ChannelCount {
			t.Fatal("native logical cursor differs", index, row.Cursor, row.Input.ChannelCount)
		}
		counts[input.ChannelCount], carries[input.ReservedCarry] = true, true
		totalSteps += position
	}
	for i, present := range counts {
		if !present {
			t.Fatal("missing native desired count", i)
		}
	}
	for i, present := range carries {
		if !present {
			t.Fatal("missing native caller reserved carry", i)
		}
	}
	if liveCases != 16 {
		t.Fatal("completed-scan replacement profile missing", liveCases)
	}
	t.Logf("complete native calls=%d operations=%d live-scan replacements=%d, all counts and reserved carries", len(rows), totalSteps, liveCases)
}

// Failure scenarios were listed in PLAN.md before implementation. Check every
// actual operation boundary, not a few selected early exits.
func TestControllerNormalizationWorkFailuresAreAtomic(t *testing.T) {
	rows := controllerNormalizationFixture(t)
	row := rows[224] // two four-MD6 TeamPlayers; all eight logical channels.
	row.Input.ChannelCount = 8
	var operationCount int
	if _, err := NormalizeControllersWork(row.Before, row.Input, func(MainWorkOperation) error { operationCount++; return nil }); err != nil {
		t.Fatal(err)
	}
	if operationCount < 350 {
		t.Fatal("failure profile too small", operationCount)
	}
	sentinel := errors.New("normalization hook failed")
	for failAt := 0; failAt < operationCount; failAt++ {
		calls, reads, readsAtFailure := 0, 0, -1
		input := row.Input
		input.LivePorts = func() (ControllerPorts, error) { reads++; return input.Ports, nil }
		got, err := NormalizeControllersWork(row.Before, input, func(MainWorkOperation) error {
			i := calls
			calls++
			if i == failAt {
				readsAtFailure = reads
				return sentinel
			}
			return nil
		})
		if !errors.Is(err, sentinel) || got != row.Before || calls != failAt+1 || reads != readsAtFailure {
			t.Fatal("failed normalization continued or leaked state", failAt, calls, reads, readsAtFailure, err)
		}
	}
	for failRead := 1; failRead <= 32; failRead++ {
		calls, reads, callsAtFailure := 0, 0, -1
		input := row.Input
		input.LivePorts = func() (ControllerPorts, error) {
			reads++
			if reads == failRead {
				callsAtFailure = calls
				return ControllerPorts{}, sentinel
			}
			return input.Ports, nil
		}
		got, err := NormalizeControllersWork(row.Before, input, func(MainWorkOperation) error { calls++; return nil })
		if !errors.Is(err, sentinel) || got != row.Before || reads != failRead || calls != callsAtFailure {
			t.Fatal("failed live read continued or leaked state", failRead, calls, callsAtFailure, reads, err)
		}
	}
}

func TestControllerNormalizationWorkRejectsInvalidPhysicalDomain(t *testing.T) {
	good := ControllerNormalizationInput{ChannelCount: 8, Ports: ControllerPorts{
		{Pads: [4]ControllerSample{{Device: MD6}}},
		{Pads: [4]ControllerSample{{Device: MD6}}},
	}}
	var initial ControllerNormalizationState
	for _, kind := range []string{"count", "carry", "device", "MD3-extension", "high-bits", "opposite-vertical", "opposite-horizontal", "unused-pad", "TeamPlayer-MD3", "history-device", "live-input"} {
		input, before := good, initial
		switch kind {
		case "count":
			input.ChannelCount = 9
		case "carry":
			input.ReservedCarry = 16
		case "device":
			input.Ports[0].Pads[0].Device = 2
		case "MD3-extension":
			input.Ports[0].Pads[0] = ControllerSample{Device: MD3, Held: ButtonX}
		case "high-bits":
			input.Ports[0].Pads[0].Held = 0x1000
		case "opposite-vertical":
			input.Ports[0].Pads[0].Held = ButtonUp | ButtonDown
		case "opposite-horizontal":
			input.Ports[0].Pads[0].Held = ButtonLeft | ButtonRight
		case "unused-pad":
			input.Ports[0].Pads[1] = ControllerSample{Device: MD6}
		case "TeamPlayer-MD3":
			input.Ports[0].TeamPlayer = true
			for i := range input.Ports[0].Pads {
				input.Ports[0].Pads[i].Device = MD6
			}
			input.Ports[0].Pads[2].Device = MD3
		case "history-device":
			before.Devices[2] = 1
		case "live-input":
			input.LivePorts = func() (ControllerPorts, error) { return ControllerPorts{}, nil }
		}
		calls := 0
		got, err := NormalizeControllersWork(before, input, func(MainWorkOperation) error { calls++; return nil })
		if !errors.Is(err, ErrControllerNormalizationProfile) || got != before || kind != "live-input" && calls != 0 {
			t.Fatal("invalid physical normalization input was accepted or changed state", kind, calls, got, err)
		}
	}
}
