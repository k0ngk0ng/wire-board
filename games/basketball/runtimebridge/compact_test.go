package runtimebridge

// Before implementation: compact replies accidentally serialize full state;
// native forks alias mutable sessions; replay loses intermediate pictures or
// short taps; packed inputs bypass bounds; verification digests disagree.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/games/basketball/syncwire"
	"reflect"
	"testing"
)

func compactCall(t *testing.T, b *Bridge, request any) map[string]json.RawMessage {
	t.Helper()
	raw, _ := json.Marshal(request)
	response := b.Handle(raw)
	if !response.OK {
		t.Fatal(response.Error)
	}
	body, _ := json.Marshal(response.Data)
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(body, &fields); e != nil {
		t.Fatal(e)
	}
	return fields
}
func compactID(t *testing.T, fields map[string]json.RawMessage) uint64 {
	t.Helper()
	var id uint64
	if e := json.Unmarshal(fields["session"], &id); e != nil {
		t.Fatal(e)
	}
	return id
}
func bridgeCheckpoint(t *testing.T, b *Bridge, id uint64) engine.RuntimeState {
	t.Helper()
	fields := compactCall(t, b, map[string]any{"op": "snapshot", "session": id})
	var c Checkpoint
	if e := json.Unmarshal(fields["checkpoint"], &c); e != nil {
		t.Fatal(e)
	}
	return c.State
}
func TestCompactAdvanceAndNativeFork(t *testing.T) {
	b := New()
	created := compactCall(t, b, map[string]any{"op": "create", "compact": true})
	id := compactID(t, created)
	if _, exists := created["state"]; exists {
		t.Fatal("compact create leaked full simulation")
	}
	reply := compactCall(t, b, map[string]any{"op": "advance", "session": id, "compact": true, "ticks": 210})
	if _, exists := reply["state"]; exists {
		t.Fatal("frame reply leaked full simulation")
	}
	if reply["frame"] == nil || reply["display"] == nil {
		t.Fatal("missing actual frame/display")
	}
	before := bridgeCheckpoint(t, b, id)
	fork := compactCall(t, b, map[string]any{"op": "fork", "session": id, "compact": true})
	pred := compactID(t, fork)
	compactCall(t, b, map[string]any{"op": "advance", "session": pred, "compact": true, "ticks": 12, "held": []int{1, 0, 0, 0}})
	compactCall(t, b, map[string]any{"op": "cancelInput", "session": pred, "controller": 0, "compact": true})
	if !reflect.DeepEqual(before, bridgeCheckpoint(t, b, id)) {
		t.Fatal("prediction changed confirmed session")
	}
	compactCall(t, b, map[string]any{"op": "fork", "session": id, "target": pred, "compact": true})
	if !reflect.DeepEqual(before, bridgeCheckpoint(t, b, pred)) {
		t.Fatal("native reconciliation differs")
	}
	bad, _ := json.Marshal(map[string]any{"op": "fork", "session": id, "target": id, "compact": true})
	if b.Handle(bad).OK {
		t.Fatal("self fork accepted")
	}
}
func TestCompactReplayKeepsOriginalFramesAndChecksum(t *testing.T) {
	b := New()
	id := compactID(t, compactCall(t, b, map[string]any{"op": "create", "compact": true}))
	state := bridgeCheckpoint(t, b, id)
	var inputs []engine.RecordedVideoInput
	var displays []engine.DisplayState
	for i := 0; i < 12; i++ {
		r := engine.RecordedVideoInput{Frame: state.VideoFrames + 1, PriorHeld: state.Held, PendingPressed: state.PendingPressed, Held: [4]engine.Buttons{engine.ButtonRight}}
		for actor, c := range state.Config.Match.ControllerAssignments {
			r.Automated[actor] = c < 0
		}
		if i == 1 {
			r.PendingPressed[0] |= engine.ButtonB
		}
		next, e := engine.StepRecordedVideo(state, r)
		if e != nil {
			t.Fatal(e)
		}
		state = next.State
		inputs = append(inputs, r)
		displays = append(displays, state.Display)
	}
	window, e := syncwire.Pack(inputs)
	if e != nil {
		t.Fatal(e)
	}
	result := compactCall(t, b, map[string]any{"op": "replay", "session": id, "compact": true, "captureFrames": true, "verify": true, "window": window})
	if result["state"] != nil {
		t.Fatal("replay returned private state")
	}
	var hash string
	json.Unmarshal(result["digest"], &hash)
	want, e := syncwire.RuntimeDigest(state)
	if e != nil || hash != want {
		t.Fatal("wrong simulation checksum", e)
	}
	if !reflect.DeepEqual(state, bridgeCheckpoint(t, b, id)) {
		t.Fatal("compact replay changed original rules")
	}
	var frames []struct {
		Frame   uint64 `json:"frame"`
		Display struct {
			Visible                 engine.BasketballRenderSnapshot
			HasVisible              bool
			CourtEpoch, CourtSerial uint64
			CourtColumns            []engine.CourtColumnCommit
			CourtPatches            []engine.CourtTilePatch
		} `json:"display"`
	}
	if e = json.Unmarshal(result["frames"], &frames); e != nil {
		t.Fatal(e)
	}
	if len(frames) != 12 {
		t.Fatal("lost batched frames")
	}
	for i, f := range frames {
		if f.Frame != inputs[i].Frame || !reflect.DeepEqual(f.Display.Visible, displays[i].Visible) || f.Display.HasVisible != displays[i].HasVisible || !reflect.DeepEqual(f.Display.CourtColumns, displays[i].CourtColumns) {
			t.Fatal("wrong intermediate visual boundary", i)
		}
	}
	before := bridgeCheckpoint(t, b, id)
	bad, _ := json.Marshal(map[string]any{"op": "replay", "session": id, "compact": true, "window": map[string]any{"from": 13, "runs": [][]int{{121, 0, 0, 0, 0}}}})
	if b.Handle(bad).OK || !reflect.DeepEqual(before, bridgeCheckpoint(t, b, id)) {
		t.Fatal("invalid run partially changed session")
	}
}

func TestCompactAutomationMatchesNativeHandoff(t *testing.T) {
	b := New()
	id := compactID(t, compactCall(t, b, map[string]any{"op": "create", "compact": true}))
	before := bridgeCheckpoint(t, b, id)
	flags := [4]bool{true, true, false, true}
	expected, e := engine.SetRuntimeAutomation(before, flags)
	if e != nil {
		t.Fatal(e)
	}
	compactCall(t, b, map[string]any{"op": "automate", "session": id, "compact": true, "automated": flags})
	if !reflect.DeepEqual(expected, bridgeCheckpoint(t, b, id)) {
		t.Fatal("wrong native controller handoff")
	}
	bad, _ := json.Marshal(map[string]any{"op": "automate", "session": id, "automated": []bool{true}})
	if b.Handle(bad).OK || !reflect.DeepEqual(expected, bridgeCheckpoint(t, b, id)) {
		t.Fatal("bad automation changed state")
	}
}

// Overlapping push windows and between-frame ownership changes are legitimate;
// repeats must not replay history, while a missing new frame must fail atomically.
func TestCompactOverlappingWindowsAndLiveAutomation(t *testing.T) {
	b := New()
	id := compactID(t, compactCall(t, b, map[string]any{"op": "create", "compact": true}))
	state := bridgeCheckpoint(t, b, id)
	var records []engine.RecordedVideoInput
	for n := 0; n < 18; n++ {
		input := engine.RecordedVideoInput{Frame: state.VideoFrames + 1, PriorHeld: state.Held, PendingPressed: state.PendingPressed}
		for i, c := range state.Config.Match.ControllerAssignments {
			input.Automated[i] = c < 0
		}
		next, e := engine.StepRecordedVideo(state, input)
		if e != nil {
			t.Fatal(e)
		}
		state = next.State
		records = append(records, input)
	}
	first, _ := syncwire.Pack(records[:12])
	last, _ := syncwire.Pack(records[6:])
	compactCall(t, b, map[string]any{"op": "replay", "session": id, "window": first, "compact": true})
	compactCall(t, b, map[string]any{"op": "replay", "session": id, "window": last, "compact": true})
	duplicate := compactCall(t, b, map[string]any{"op": "replay", "session": id, "window": last, "compact": true, "captureFrames": true})
	if string(duplicate["frame"]) != "18" || duplicate["frames"] != nil || !reflect.DeepEqual(state, bridgeCheckpoint(t, b, id)) {
		t.Fatal("overlap replayed old video frames")
	}
	flags := [4]bool{true, true, false, true}
	expected, e := engine.SetRuntimeAutomation(state, flags)
	if e != nil {
		t.Fatal(e)
	}
	compactCall(t, b, map[string]any{"op": "replay", "session": id, "window": syncwire.InputWindow{}, "automated": flags, "compact": true})
	if !reflect.DeepEqual(expected, bridgeCheckpoint(t, b, id)) {
		t.Fatal("empty frame batch lost handoff")
	}
	bad, _ := json.Marshal(map[string]any{"op": "replay", "session": id, "window": syncwire.InputWindow{From: 20, Runs: [][]uint64{{1, 0, 0, 0, 10}}}, "compact": true})
	if b.Handle(bad).OK || !reflect.DeepEqual(expected, bridgeCheckpoint(t, b, id)) {
		t.Fatal("gapped batch advanced partially")
	}
}

func TestCompactBetweenFrameCancellation(t *testing.T) {
	b := New()
	id := compactID(t, compactCall(t, b, map[string]any{"op": "create", "compact": true}))
	state := bridgeCheckpoint(t, b, id)
	state.Held[0] = 1
	state.PendingPressed[0] = 64
	fields := compactCall(t, b, map[string]any{"op": "restore", "checkpoint": Checkpoint{Version: 1, State: state}, "compact": true})
	other := compactID(t, fields)
	compactCall(t, b, map[string]any{"op": "replay", "session": other, "window": syncwire.InputWindow{}, "compact": true, "latches": []uint32{0, 0}})
	actual := bridgeCheckpoint(t, b, other)
	state.Held = [4]engine.Buttons{}
	state.PendingPressed = [4]engine.Buttons{}
	if !reflect.DeepEqual(actual, state) {
		t.Fatal("between-frame release did not clear only physical latches")
	}
}
