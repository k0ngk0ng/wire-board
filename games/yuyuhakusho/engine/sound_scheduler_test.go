package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

type soundSchedulerSnapshot struct {
	Delay       uint16   `json:"delay"`
	DelayedCode uint8    `json:"delayed_code"`
	Backlog     uint8    `json:"backlog"`
	Busy        uint8    `json:"busy"`
	Queue       [4]uint8 `json:"queue"`
	Priority    uint8    `json:"priority"`
	Selected    uint8    `json:"selected"`
}

type soundSchedulerCall struct {
	Kind, Name       string
	Before, After    soundSchedulerSnapshot
	Drives           []soundSchedulerSnapshot
	BeforeDecrements []soundSchedulerSnapshot `json:"before_decrements"`
	UnknownWrites    []int                    `json:"unknown_writes"`
}

func soundSchedulerDecode(raw soundSchedulerSnapshot) SoundSchedulerState {
	return SoundSchedulerState{Delay: raw.Delay, DelayedCode: raw.DelayedCode, Backlog: raw.Backlog, Busy: raw.Busy}
}

func soundSchedulerFixture(t *testing.T) []soundSchedulerCall {
	t.Helper()
	packed, err := os.ReadFile("testdata/audio-timing-boundaries.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile("testdata/audio-timing-boundaries-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Pass bool
		SHA  string `json:"fixture_json_sha256"`
		Gzip string `json:"fixture_gzip_sha256"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
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
	h, zh := sha256.Sum256(raw), sha256.Sum256(packed)
	if !manifest.Pass || hex.EncodeToString(h[:]) != manifest.SHA || hex.EncodeToString(zh[:]) != manifest.Gzip {
		t.Fatal("unbound original scheduler fixture")
	}
	var fixture struct {
		Calls []soundSchedulerCall
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Calls) < 1000 {
		t.Fatal("original scheduler fixture lost boundary or sequence coverage", len(fixture.Calls))
	}
	return fixture.Calls
}

func soundSchedulerRun(t *testing.T, state SoundSchedulerState, queue [4]uint8, call soundSchedulerCall) (SoundSchedulerState, [4]uint8) {
	t.Helper()
	if len(call.UnknownWrites) != 0 {
		t.Fatal("original scheduler call has unexplained RAM writes", call.Name)
	}
	switch call.Kind {
	case "interrupt", "begin-interrupt":
		next, queued, drive := BeginSoundInterrupt(state, queue)
		if drive != (len(call.Drives) != 0) {
			t.Fatal("interrupt drive decision differs", call.Name, drive, len(call.Drives))
		}
		if drive {
			if next != soundSchedulerDecode(call.Drives[0]) || queued != call.Drives[0].Queue {
				t.Fatal("interrupt call-time state differs", call.Name, next, call.Drives[0])
			}
			if call.Kind == "interrupt" {
				next = FinishSoundInterrupt(next)
			}
		}
		return next, queued
	case "catchup":
		for _, original := range call.Drives {
			var drive bool
			state, drive = BeginSoundCatchup(state)
			if !drive || state != soundSchedulerDecode(original) || queue != original.Queue {
				t.Fatal("catchup call-time state differs", call.Name, state, original)
			}
			state = FinishSoundCatchup(state)
		}
		if _, drive := BeginSoundCatchup(state); drive {
			t.Fatal("catchup continues beyond original return", call.Name, state)
		}
		return state, queue
	case "begin-catchup":
		next, drive := BeginSoundCatchup(state)
		if drive != (len(call.Drives) != 0) {
			t.Fatal("begin catchup decision differs", call.Name)
		}
		return next, queue
	case "finish-interrupt":
		return FinishSoundInterrupt(state), queue
	case "finish-catchup":
		return FinishSoundCatchup(state), queue
	default:
		t.Fatal("unknown original scheduler phase", call.Kind)
		return state, queue
	}
}

func TestSoundSchedulerOriginalBoundaries(t *testing.T) {
	counts := make(map[string]int)
	for _, call := range soundSchedulerFixture(t) {
		state, queue := soundSchedulerDecode(call.Before), call.Before.Queue
		next, queued := soundSchedulerRun(t, state, queue, call)
		if next != soundSchedulerDecode(call.After) || queued != call.After.Queue || state != soundSchedulerDecode(call.Before) || queue != call.Before.Queue {
			t.Fatal("scheduler full boundary differs or mutates caller", call.Name, next, call.After)
		}
		counts[call.Kind]++
	}
	for _, kind := range []string{"interrupt", "catchup", "begin-interrupt", "finish-interrupt", "begin-catchup", "finish-catchup"} {
		if counts[kind] == 0 {
			t.Fatal("missing original scheduler phase", kind)
		}
	}
}

func TestSoundSchedulerSingleInitialStreams(t *testing.T) {
	var state SoundSchedulerState
	var queue [4]uint8
	streams, steps := 0, 0
	prefix := ""
	for _, call := range soundSchedulerFixture(t) {
		if !strings.HasPrefix(call.Name, "stream-") && !strings.HasPrefix(call.Name, "nested-") {
			continue
		}
		parts := strings.Split(call.Name, "-")
		key := parts[0] + "-" + parts[1]
		if prefix != key {
			state, queue = soundSchedulerDecode(call.Before), call.Before.Queue
			prefix = key
			streams++
		}
		state, queue = soundSchedulerRun(t, state, queue, call)
		if state != soundSchedulerDecode(call.After) || queue != call.After.Queue {
			t.Fatal("single-initial scheduler stream diverged", call.Name, state, call.After)
		}
		steps++
	}
	if streams < 6 || steps < 260 {
		t.Fatal("lost continuous or nested phase evidence", streams, steps)
	}
}

func TestSoundSchedulerStateOwnership(t *testing.T) {
	typ := reflect.TypeFor[SoundSchedulerState]()
	if typ.NumField() != 4 {
		t.Fatal("scheduler duplicated external sound state", typ)
	}
	for index, expected := range []string{"Delay", "DelayedCode", "Backlog", "Busy"} {
		if typ.Field(index).Name != expected {
			t.Fatal("scheduler owner differs", typ.Field(index))
		}
	}
	state := SoundSchedulerState{Delay: 2, DelayedCode: 0x82, Backlog: 255, Busy: 1}
	state, queue, drive := BeginSoundInterrupt(state, [4]uint8{})
	if drive || state.Delay != 1 || state.Backlog != 0 || queue != [4]uint8{} {
		t.Fatal("busy gate discarded delay or byte wrap", state, queue, drive)
	}
	state = FinishSoundCatchup(state)
	if state.Busy != 0 || state.Backlog != 255 {
		t.Fatal("catchup finish ignored current nested backlog", state)
	}
}

func TestSoundSchedulerNaturalPhaseBoundaries(t *testing.T) {
	phases, _, _ := soundSchedulerNaturalFixture(t)
	if len(phases) != 16974 {
		t.Fatal("natural scheduler phase coverage changed", len(phases))
	}
	for _, phase := range phases {
		before := soundSchedulerDecode(phase.Before)
		var after SoundSchedulerState
		queue := phase.Before.Queue
		switch phase.Kind {
		case "begin-interrupt":
			var drive bool
			after, queue, drive = BeginSoundInterrupt(before, queue)
			if drive != phase.Drive {
				t.Fatal("natural drive decision differs", phase.Name)
			}
		case "finish-interrupt":
			after = FinishSoundInterrupt(before)
		default:
			t.Fatal("unexpected natural phase", phase.Kind)
		}
		if after != soundSchedulerDecode(phase.After) || queue != phase.After.Queue {
			t.Fatal("original closed phase differs", phase.Name, after, phase.After)
		}
	}
}

type soundSchedulerNaturalPhase struct {
	Kind, Name    string
	Before, After soundSchedulerSnapshot
	Drive         bool
}

func soundSchedulerNaturalFixture(t *testing.T) ([]soundSchedulerNaturalPhase, []soundSchedulerNaturalPhase, SoundPriorityTable) {
	t.Helper()
	packed, err := os.ReadFile("testdata/audio-timing-natural-phases.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	manifestRaw, err := os.ReadFile("testdata/audio-timing-natural-phases-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Pass        bool
		SHA         string `json:"fixture_json_sha256"`
		Gzip        string `json:"fixture_gzip_sha256"`
		PrioritySHA string `json:"priority_bytes_sha256"`
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
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
	h, zh := sha256.Sum256(raw), sha256.Sum256(packed)
	if !manifest.Pass || hex.EncodeToString(h[:]) != manifest.SHA || hex.EncodeToString(zh[:]) != manifest.Gzip {
		t.Fatal("natural scheduler phases lost original binding")
	}
	var fixture struct {
		Priorities        SoundPriorityTable
		Phases, Consumers []soundSchedulerNaturalPhase
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	p := sha256.Sum256(fixture.Priorities[:])
	if hex.EncodeToString(p[:]) != manifest.PrioritySHA {
		t.Fatal("priority metadata differs from immutable original table")
	}
	return fixture.Phases, fixture.Consumers, fixture.Priorities
}

func TestSoundSchedulerNaturalConsumers(t *testing.T) {
	_, consumers, priorities := soundSchedulerNaturalFixture(t)
	if len(consumers) != 185 {
		t.Fatal("natural consumer coverage changed", len(consumers))
	}
	for _, consumer := range consumers {
		before := SoundPriorityState{CurrentPriority: consumer.Before.Priority, SelectedSound: consumer.Before.Selected, Queue: consumer.Before.Queue}
		want := SoundPriorityState{CurrentPriority: consumer.After.Priority, SelectedSound: consumer.After.Selected, Queue: consumer.After.Queue}
		if got := ConsumeSoundQueue(before, priorities); got != want {
			t.Fatal("actual natural queue consumer differs", consumer.Name, got, want)
		}
	}
}
