package rootcontrol_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
)

type fadeRootFive struct {
	Priority      uint8
	Selected      uint8
	FadeSteps     uint8
	FadeRemaining uint8
	ChoiceBits    uint8
}

func (r fadeRootFive) fade() rootcontrol.FadeState {
	return rootcontrol.FadeState{Steps: r.FadeSteps, Remaining: r.FadeRemaining}
}

type fadeExpectedWrite struct {
	Counter string `json:"counter"`
	Value   uint8  `json:"value"`
}
type fadeNativeOperation struct {
	Kind          string              `json:"kind"`
	Command       uint8               `json:"command"`
	Before        fadeRootFive        `json:"before"`
	After         fadeRootFive        `json:"after"`
	Writes        []fadeExpectedWrite `json:"writes"`
	Action        string              `json:"action"`
	DACAfterFlags uint8               `json:"DAC_after_flags"`
}
type fadeNativeFrame struct {
	Step       int                   `json:"step"`
	Operations []fadeNativeOperation `json:"operations"`
	FadeAfter  rootcontrol.FadeState `json:"fade_after"`
}
type fadeNativeStream struct {
	ID      string                `json:"id"`
	Initial rootcontrol.FadeState `json:"initial"`
	Frames  []fadeNativeFrame     `json:"frames"`
}

func loadFadeStreams(t *testing.T) []fadeNativeStream {
	t.Helper()
	raw, err := os.ReadFile("testdata/fade-native-streams.json.gz")
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
		Schema  int                `json:"schema"`
		Streams []fadeNativeStream `json:"streams"`
	}
	if err = json.Unmarshal(body, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Schema != 1 || len(fixture.Streams) != 5 {
		t.Fatal("native fade stream matrix changed", fixture.Schema, len(fixture.Streams))
	}
	return fixture.Streams
}
func nativeFadeResult(t *testing.T, op fadeNativeOperation) rootcontrol.FadeResult {
	t.Helper()
	result := rootcontrol.FadeResult{State: op.After.fade(), WriteCount: uint8(len(op.Writes))}
	if len(op.Writes) > 2 {
		t.Fatal("native fade counter output unbounded")
	}
	for index, write := range op.Writes {
		counter := rootcontrol.FadeStepsCounter
		if write.Counter == "remaining" {
			counter = rootcontrol.FadeRemainingCounter
		} else if write.Counter != "steps" {
			t.Fatal("unknown fixture counter", write.Counter)
		}
		result.Writes[index] = rootcontrol.FadeWrite{Counter: counter, Value: write.Value}
	}
	switch op.Action {
	case "none":
		result.Action = rootcontrol.FadeNoAction
	case "stop-dac-flags":
		result.Action = rootcontrol.StopDACFlags
	case "fade-music-step":
		result.Action = rootcontrol.FadeMusicStep
	case "request-full-clear":
		result.Action = rootcontrol.RequestFullClear
	default:
		t.Fatal("unknown native fade action", op.Action)
	}
	return result
}
func replayFadeStream(t *testing.T, stream fadeNativeStream, restoreAt int) map[string]int {
	t.Helper()
	state := stream.Initial
	actions := map[string]int{}
	for index, frame := range stream.Frames {
		if frame.Step != index {
			t.Fatal("native frame order changed", stream.ID, index, frame.Step)
		}
		for _, op := range frame.Operations {
			if state != op.Before.fade() {
				t.Fatalf("%s frame%d counter carry %+v, native %+v", stream.ID, index, state, op.Before.fade())
			}
			var got rootcontrol.FadeResult
			switch op.Kind {
			case "advance":
				got = rootcontrol.AdvanceFade(state)
			case "start":
				var err error
				got, err = rootcontrol.StartFade(state, op.Command)
				if err != nil {
					t.Fatal(err)
				}
				if op.DACAfterFlags != 0 {
					t.Fatal("native start did not clear DAC flag byte", op.DACAfterFlags)
				}
			default:
				t.Fatal("unknown native fade operation", op.Kind)
			}
			want := nativeFadeResult(t, op)
			if got != want {
				t.Fatalf("%s frame%d %s\nGo %+v\nnative %+v", stream.ID, index, op.Kind, got, want)
			}
			// Apply only the returned counter effects to the native local Root
			// context. The primitive never receives priority/selected/choice.
			context := op.Before
			for _, write := range got.Writes[:got.WriteCount] {
				switch write.Counter {
				case rootcontrol.FadeRemainingCounter:
					context.FadeRemaining = write.Value
				case rootcontrol.FadeStepsCounter:
					context.FadeSteps = write.Value
				default:
					t.Fatal("invalid returned fade counter", write.Counter)
				}
			}
			if context != op.After || context.fade() != got.State {
				t.Fatal("counter effects changed other Root fields or lost ordered state", stream.ID, index, context, op.After, got.State)
			}
			state = got.State
			actions[op.Action]++
		}
		if state != frame.FadeAfter {
			t.Fatalf("%s frame%d final fade state %+v, native %+v", stream.ID, index, state, frame.FadeAfter)
		}
		if index == restoreAt {
			snapshot, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			var restored rootcontrol.FadeState
			if err = json.Unmarshal(snapshot, &restored); err != nil {
				t.Fatal(err)
			}
			if restored != state {
				t.Fatal("fade value snapshot lost carry", state, restored)
			}
			state = restored
		}
	}
	return actions
}

func TestFadeNativeStreamsAcross785Frames(t *testing.T) {
	frames, decisions := 0, 0
	actions := map[string]int{}
	for _, stream := range loadFadeStreams(t) {
		t.Run(stream.ID, func(t *testing.T) {
			got := replayFadeStream(t, stream, -1)
			for kind, count := range got {
				actions[kind] += count
				decisions += count
			}
			frames += len(stream.Frames)
		})
	}
	if frames != 785 || decisions != 746 || actions["none"] != 654 || actions["stop-dac-flags"] != 2 || actions["fade-music-step"] != 88 || actions["request-full-clear"] != 2 {
		t.Fatal("native fade coverage changed", frames, decisions, actions)
	}
	t.Logf("5 native streams,785 frames,746 decisions:654 none,2 start,88 music-step,2 reset-request")
}

func TestFadeNativeStreamSnapshotRestoreContinuation(t *testing.T) {
	restores := 0
	for _, stream := range loadFadeStreams(t) {
		for _, checkpoint := range []int{0, 31, 32, 33, 35, 36, 64, 65, len(stream.Frames) / 2, len(stream.Frames) - 1} {
			if checkpoint >= len(stream.Frames) {
				continue
			}
			t.Run(fmt.Sprintf("%s/frame%d", stream.ID, checkpoint), func(t *testing.T) {
				replayFadeStream(t, stream, checkpoint)
				restores++
			})
		}
	}
	if restores != 50 {
		t.Fatal("native checkpoint coverage changed", restores)
	}
}

func TestFadeCounterBoundaryAndByteProfiles(t *testing.T) {
	for _, row := range []struct {
		name   string
		before rootcontrol.FadeState
		want   rootcontrol.FadeResult
	}{
		{"idle-carry", rootcontrol.FadeState{Remaining: 255}, rootcontrol.FadeResult{State: rootcontrol.FadeState{Remaining: 255}}},
		{"last-wait-only", rootcontrol.FadeState{Steps: 1, Remaining: 1}, rootcontrol.FadeResult{State: rootcontrol.FadeState{Steps: 1}, WriteCount: 1, Writes: [2]rootcontrol.FadeWrite{{Counter: rootcontrol.FadeRemainingCounter, Value: 0}}}},
		{"terminal-request-only", rootcontrol.FadeState{Steps: 1}, rootcontrol.FadeResult{State: rootcontrol.FadeState{}, WriteCount: 1, Writes: [2]rootcontrol.FadeWrite{{Counter: rootcontrol.FadeStepsCounter, Value: 0}}, Action: rootcontrol.RequestFullClear}},
		{"next-step", rootcontrol.FadeState{Steps: 2}, rootcontrol.FadeResult{State: rootcontrol.FadeState{Steps: 1, Remaining: 3}, WriteCount: 2, Writes: [2]rootcontrol.FadeWrite{{Counter: rootcontrol.FadeStepsCounter, Value: 1}, {Counter: rootcontrol.FadeRemainingCounter, Value: 3}}, Action: rootcontrol.FadeMusicStep}},
		{"byte-wait", rootcontrol.FadeState{Steps: 255, Remaining: 255}, rootcontrol.FadeResult{State: rootcontrol.FadeState{Steps: 255, Remaining: 254}, WriteCount: 1, Writes: [2]rootcontrol.FadeWrite{{Counter: rootcontrol.FadeRemainingCounter, Value: 254}}}},
		{"byte-step", rootcontrol.FadeState{Steps: 255}, rootcontrol.FadeResult{State: rootcontrol.FadeState{Steps: 254, Remaining: 3}, WriteCount: 2, Writes: [2]rootcontrol.FadeWrite{{Counter: rootcontrol.FadeStepsCounter, Value: 254}, {Counter: rootcontrol.FadeRemainingCounter, Value: 3}}, Action: rootcontrol.FadeMusicStep}},
	} {
		t.Run(row.name, func(t *testing.T) {
			if got := rootcontrol.AdvanceFade(row.before); got != row.want {
				t.Fatal("fade boundary differs", got, row.want)
			}
		})
	}
}

func TestFadeStartAlwaysWritesRemainingThenSteps(t *testing.T) {
	starts := 0
	for _, stream := range loadFadeStreams(t) {
		for _, frame := range stream.Frames {
			for _, op := range frame.Operations {
				if op.Kind != "start" {
					continue
				}
				want := nativeFadeResult(t, op)
				for _, current := range []rootcontrol.FadeState{{Steps: 4, Remaining: 1}, want.State} {
					got, err := rootcontrol.StartFade(current, op.Command)
					if err != nil || got != want || got.WriteCount != 2 || got.Writes[0].Counter != rootcontrol.FadeRemainingCounter || got.Writes[1].Counter != rootcontrol.FadeStepsCounter || got.Action != rootcontrol.StopDACFlags {
						t.Fatal("start lost fixed writes or StopDACFlags boundary", current, got, err, want)
					}
				}
				starts++
			}
		}
	}
	if starts != 2 {
		t.Fatal("native start profiles changed", starts)
	}
}

func TestFadeStartRejectsUnknownCommands(t *testing.T) {
	for command := 0; command < 256; command++ {
		if command == 1 || command == 5 {
			continue
		}
		got, err := rootcontrol.StartFade(rootcontrol.FadeState{Steps: 40, Remaining: 3}, uint8(command))
		if !errors.Is(err, rootcontrol.ErrFadeUnsupported) || got != (rootcontrol.FadeResult{}) {
			t.Fatal("unknown start command accepted or emitted effects", command, got, err)
		}
	}
}

func TestFadeStateOwnsOnlyTheTwoIndependentCounters(t *testing.T) {
	state := reflect.TypeOf(rootcontrol.FadeState{})
	if state.NumField() != 2 {
		t.Fatal("fade state copied another owner or history", state)
	}
	for _, name := range []string{"Steps", "Remaining"} {
		field, ok := state.FieldByName(name)
		if !ok || field.Type.Kind() != reflect.Uint8 {
			t.Fatal("fade state contains a replacement owner or wrong counter width", name, state)
		}
	}
	result := reflect.TypeOf(rootcontrol.FadeResult{})
	if result.NumField() != 4 {
		t.Fatal("temporary fade result acquired another persistent view", result)
	}
	for _, name := range []string{"State", "Writes", "WriteCount", "Action"} {
		if _, ok := result.FieldByName(name); !ok {
			t.Fatal("fade result contract changed", name)
		}
	}
	writes, _ := result.FieldByName("Writes")
	if writes.Type.Kind() != reflect.Array || writes.Type.Len() != 2 {
		t.Fatal("fade writes became an unbounded history", writes.Type)
	}
	if reflect.TypeOf(rootcontrol.FadeWrite{}).NumField() != 2 {
		t.Fatal("fade effect acquired an owner snapshot")
	}
	body, err := json.Marshal(rootcontrol.FadeState{Steps: 40, Remaining: 3})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err = json.Unmarshal(body, &fields); err != nil || len(fields) != 2 || fields["steps"] != float64(40) || fields["remaining"] != float64(3) {
		t.Fatal("snapshot schema copied unsupported profile fields", string(body), err)
	}
}
