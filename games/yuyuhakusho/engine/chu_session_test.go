package engine

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func sessionLibrary(t *testing.T) ChuWorldLibrary {
	t.Helper()
	compressed, err := os.ReadFile("../data/chu-world-rules.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	library, err := DecodeChuWorldRules(raw)
	if err != nil {
		t.Fatal(err)
	}
	return library
}

func sessionOptions(computers bool) ChuSessionOptions {
	return ChuSessionOptions{Seed: 0x12345678, Difficulty: 6, Players: []ChuSessionPlayer{
		{Slot: 0, CharacterID: 9, Color: 0, Computer: computers, HumanChannel: 0},
		{Slot: 3, CharacterID: 9, Color: 1, Computer: true},
	}}
}

func TestChuSessionDefaultResourceSource(t *testing.T) {
	compressed, err := os.ReadFile("../data/arena-assets-default.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var asset struct {
		Palette [16]uint16 `json:"initial_scene_palette_0"`
		Binding struct {
			Source uint32 `json:"resource_list"`
		} `json:"resource_binding"`
	}
	if err := json.NewDecoder(z).Decode(&asset); err != nil {
		t.Fatal(err)
	}
	resource := DefaultChuSessionResource()
	if resource.Source != asset.Binding.Source {
		t.Fatal("ready resource identity differs from actual arena binding")
	}
	for i, word := range resource.Palette0 {
		compact := (word>>1)&7 | ((word>>5)&7)<<3 | ((word>>9)&7)<<6
		if compact != asset.Palette[i] {
			t.Fatal("ready palette differs from independently verified original resource", i)
		}
	}
}

// New-construction/full-match tests precede chu_session.go. No original world
// snapshots or expected intermediate frames enter these production APIs.
func TestChuSessionReadinessAndStartup(t *testing.T) {
	library := sessionLibrary(t)
	state, events, err := NewChuSession(sessionOptions(false), library)
	if err != nil || state.Clock.Frame != 0 || state.RandomSeed != 0x12345678 || state.ResourceBusy == 0 || state.Objects[0x8040].Header.Stage != 2 {
		t.Fatal("new preparation world did not request resources without ticking", err)
	}
	requests := 0
	for _, event := range events {
		if event.Kind == "start" && event.Start.Kind == "resource" {
			requests++
			if event.Start.Source != library.MatchStart.ArenaResources[0] {
				t.Fatal("wrong default resource request")
			}
		}
	}
	if requests != 1 {
		t.Fatal("missing one-time resource request")
	}
	before, _ := json.Marshal(state)
	if _, _, _, err := StepChuSessionFrame(state, [8]Buttons{}, library); err == nil {
		t.Fatal("unready session advanced")
	}
	after, _ := json.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("unready caller was mutated")
	}
	resource := DefaultChuSessionResource()
	ready, err := CompleteChuSessionResource(state, resource)
	if err != nil || ready.ResourceBusy != 0 || ready.Clock.Frame != 0 {
		t.Fatal("valid resource completion failed", err)
	}
	if _, err := CompleteChuSessionResource(ready, resource); err == nil {
		t.Fatal("late duplicate completion was accepted")
	}
	ready, _, _, err = StepChuSessionFrame(ready, [8]Buttons{}, library)
	if err != nil || ready.Objects[0x8400].Header.Stage != 2 || ready.Objects[0x9000].Header.Stage != 2 || ready.Objects[0x8400].Body.Health != 223 || ready.Objects[0x9000].Body.Health != 223 {
		t.Fatal("future source slots were not initialized during first runnable tick", err)
	}
	if ready.Objects[0x8400].InputChannel != 0 || ready.Objects[0x9000].InputChannel != 255 {
		t.Fatal("explicit human/computer input mapping changed")
	}
	for i := 0; i < 320 && ready.Match.Disabled != 0; i++ {
		ready, _, _, err = StepChuSessionFrame(ready, [8]Buttons{}, library)
		if err != nil {
			t.Fatal("intro startup failed", err)
		}
	}
	if ready.Match.Disabled != 0 || ready.Objects[0x8400].InputChannel != 0 {
		t.Fatal("opening never enabled controls or idleness took over human")
	}
}

func TestChuSessionFreshAICompleteAndRestore(t *testing.T) {
	library := sessionLibrary(t)
	state, _, err := NewChuSession(sessionOptions(true), library)
	if err != nil {
		t.Fatal(err)
	}
	state, err = CompleteChuSessionResource(state, DefaultChuSessionResource())
	if err != nil {
		t.Fatal(err)
	}
	damage, knockout, retired, declared := false, false, false, false
	var restored *ChuWorldState
	ticks, callbacks := 0, 0
	for ; ticks < 20000 && !state.MatchComplete; ticks++ {
		if ticks == 400 {
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			var copy ChuWorldState
			if err := json.Unmarshal(raw, &copy); err != nil || !reflect.DeepEqual(copy, state) {
				t.Fatal("mid-match archive changed state", err)
			}
			restored = &copy
		}
		var events []ChuWorldEvent
		state, events, _, err = StepChuSessionFrame(state, [8]Buttons{}, library)
		if err != nil {
			t.Fatalf("fresh AI match tick %d: %v", ticks, err)
		}
		for _, event := range events {
			if event.Kind == "callback" {
				callbacks++
			}
		}
		if restored != nil {
			copy, copyEvents, _, restoreErr := StepChuSessionFrame(*restored, [8]Buttons{}, library)
			if restoreErr != nil || !reflect.DeepEqual(copy, state) || !reflect.DeepEqual(copyEvents, events) {
				t.Fatalf("archive replay diverged at tick %d: %v", ticks, restoreErr)
			}
			*restored = copy
		}
		for _, id := range []uint16{0x8400, 0x9000} {
			actor := state.Objects[id]
			if actor.Header.Behavior == 0x12668 && actor.Header.Stage == 2 {
				damage = damage || actor.Body.Health < 223
				knockout = knockout || actor.Body.Health < 0
				retired = retired || actor.Body.Health == -1024
			}
		}
		declared = declared || state.Match.WinnerCode != 0
	}
	if !state.MatchComplete || !damage || !knockout || !retired || !declared || restored == nil || (state.Match.WinnerCode != 1 && state.Match.WinnerCode != 4 && state.Match.WinnerCode != MatchDrawCode) {
		t.Fatalf("fresh legal AI match failed actual lifecycle completion: ticks=%d complete=%v damage=%v KO=%v retired=%v winner=%d", ticks, state.MatchComplete, damage, knockout, retired, state.Match.WinnerCode)
	}
	ended, events, _, err := StepChuSessionFrame(state, [8]Buttons{}, library)
	if err != nil || len(events) != 0 || !reflect.DeepEqual(ended, state) {
		t.Fatal("completed platform match continued into original menus", err)
	}
	t.Logf("fresh independent match ticks=%d callbacks=%d winner=%d, archive replay exact", ticks, callbacks, state.Match.WinnerCode)
}

func TestChuSessionInvalidInputsAreAtomic(t *testing.T) {
	library := sessionLibrary(t)
	for _, options := range []ChuSessionOptions{
		{Players: nil},
		{Difficulty: 1, Players: sessionOptions(true).Players},
		{Players: []ChuSessionPlayer{{Slot: 0, CharacterID: 9}, {Slot: 0, CharacterID: 9}}},
		{Players: []ChuSessionPlayer{{Slot: 0, CharacterID: 0}, {Slot: 3, CharacterID: 9}}},
		{Players: []ChuSessionPlayer{{Slot: 0, CharacterID: 9, HumanChannel: 0}, {Slot: 3, CharacterID: 9, HumanChannel: 0}}},
		{TeamBattle: true, Players: sessionOptions(true).Players},
	} {
		if _, _, err := NewChuSession(options, library); err == nil {
			t.Fatal("unsupported or ambiguous initial configuration was accepted")
		}
	}
	state, _, err := NewChuSession(sessionOptions(false), library)
	if err != nil {
		t.Fatal(err)
	}
	bad := DefaultChuSessionResource()
	bad.Source++
	if got, err := CompleteChuSessionResource(state, bad); err == nil || !reflect.DeepEqual(got, state) {
		t.Fatal("wrong resource acknowledgement mutated world")
	}
	state, _ = CompleteChuSessionResource(state, DefaultChuSessionResource())
	before, _ := json.Marshal(state)
	input := [8]Buttons{1 << 14}
	got, events, _, err := StepChuSessionFrame(state, input, library)
	after, _ := json.Marshal(state)
	if err == nil || len(events) != 0 || !reflect.DeepEqual(got, state) || !bytes.Equal(before, after) {
		t.Fatal("invalid external controller storage bits leaked frame changes")
	}
}
