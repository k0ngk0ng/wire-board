package contra

// Failure cases before implementation: global instead of per-slot priority;
// same-frame duplicates making extra voices; later equal requests not restarting;
// WAV length used as slot lifetime; freezing cue74 or advancing other paused
// slots; percussion bypassing noise priority; DPCM incorrectly sharing noise;
// reset/62/77/16-to-18 guards; pending commands or clocks lost in JSON restore;
// history mutation and duplicate event identities after restore.

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestAudioOriginalControllerTicks(t *testing.T) {
	f, err := os.Open("../../../docs/research/nes-contra/evidence/audio-control-original.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		Scenarios []struct {
			Name   string
			Ticks  int
			Inputs []struct {
				Tick  int
				Kind  string
				Value uint8
			}
			Rows []struct {
				Tick, Paused int
				After        string
				Requests     []struct {
					ID       uint8
					Internal bool
				}
				StatusWrites []uint8
			}
		}
	}
	if err = json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, scenario := range fixture.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			state := NewAudioState("original-" + scenario.Name)
			for _, row := range scenario.Rows {
				for _, input := range scenario.Inputs {
					if input.Tick == row.Tick && input.Kind != "pause" {
						state.Queue(AudioCommand{Kind: input.Kind, ID: input.Value})
					}
				}
				// Snapshot between World's command enqueue and the next audio tick.
				if row.Tick%37 == 0 {
					raw, _ := json.Marshal(state)
					var restored AudioState
					if err = json.Unmarshal(raw, &restored); err != nil {
						t.Fatal(err)
					}
					state = restored
				}
				update := state.Advance(row.Paused != 0)
				if len(update.UnsupportedIDs) != 0 {
					t.Fatalf("tick %d unsupported %v", row.Tick, update.UnsupportedIDs)
				}
				original, err := hex.DecodeString(row.After)
				if err != nil || len(original) != 130 {
					t.Fatal("bad original driver state")
				}
				if update.View.SlotIDs != [6]uint8(original[6:12]) || update.View.Paused != (original[0x2f] != 0) {
					t.Fatalf("tick %d: slots got %v want %v, pause %v/%d", row.Tick, update.View.SlotIDs, original[6:12], update.View.Paused, original[0x2f])
				}
				var wantDPCM uint8
				for _, request := range row.Requests {
					if request.ID >= 0x7f && request.ID <= 0x81 {
						wantDPCM = request.ID
					}
				}
				if len(row.StatusWrites) > 0 && row.StatusWrites[len(row.StatusWrites)-1]&16 == 0 {
					wantDPCM = 0
				}
				var gotDPCM uint8
				for _, e := range update.Events {
					if e.Action == "start" && e.Voice != nil && e.Voice.Key == "dpcm" {
						gotDPCM = e.Voice.ID
					}
				}
				if gotDPCM != wantDPCM {
					t.Fatalf("tick %d DPCM start got %x want %x", row.Tick, gotDPCM, wantDPCM)
				}
				checked++
			}
		})
	}
	if checked != 8140 {
		t.Fatal("original tick coverage changed", checked)
	}
}

func TestAudioDuplicateRetriggerResetAndRestore(t *testing.T) {
	s := NewAudioState("room/match")
	for range 5 {
		s.Queue(AudioCommand{Kind: "sound", ID: 40})
	}
	u := s.Advance(false)
	starts := []AudioVoice{}
	for _, e := range u.Events {
		if e.Action == "start" {
			starts = append(starts, *e.Voice)
		}
	}
	if len(starts) != 2 || starts[0].ID != 40 || starts[1].ID != 40 {
		t.Fatal("same-tick spread multiplied voices", starts)
	}
	s.Queue(AudioCommand{Kind: "sound", ID: 40})
	v := s.Advance(false)
	later := []AudioVoice{}
	for _, e := range v.Events {
		if e.Action == "start" {
			later = append(later, *e.Voice)
		}
	}
	if len(later) != 2 || later[0].PlaybackID == starts[0].PlaybackID {
		t.Fatal("later equal request did not retrigger")
	}
	s.Queue(AudioCommand{Kind: "sound", ID: 26}, AudioCommand{Kind: "reset"}, AudioCommand{Kind: "sound", ID: 70})
	raw, _ := json.Marshal(s)
	var restored AudioState
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	a, b := s.Advance(false), restored.Advance(false)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("pending/reset/epoch changed through restore")
	}
	if len(a.Events) == 0 || a.Events[0].Action != "reset" {
		t.Fatal("reset order lost")
	}
	for _, e := range a.Events {
		if e.Voice != nil && e.Voice.ID == 26 {
			t.Fatal("reset resurrected discarded earlier shot")
		}
	}
	view := s.View()
	view.Events[0].Action = "corrupt"
	if s.View().Events[0].Action == "corrupt" {
		t.Fatal("view aliases event history")
	}
}

func TestAudioPauseCueEndsAndMusicClockFreezes(t *testing.T) {
	s := NewAudioState("pause")
	s.Queue(AudioCommand{Kind: "sound", ID: 70}, AudioCommand{Kind: "sound", ID: 26})
	for range 5 {
		s.Advance(false)
	}
	s.Queue(AudioCommand{Kind: "sound", ID: 116})
	paused := s.Advance(true)
	var musicPosition uint64
	for _, v := range paused.View.Voices {
		if v.Key == "slot-0" {
			musicPosition = v.PositionTicks
			if !v.Paused {
				t.Fatal("music not paused")
			}
		}
	}
	for range 100 {
		paused = s.Advance(true)
	}
	if paused.View.SlotIDs[4] != 0 || paused.View.SlotIDs[5] != 26 {
		t.Fatal("pause cue/previous noise lifetime", paused.View.SlotIDs)
	}
	for _, v := range paused.View.Voices {
		if v.Key == "slot-0" && v.PositionTicks != musicPosition {
			t.Fatal("paused music clock moved")
		}
	}
	s.Queue(AudioCommand{Kind: "sound", ID: 26})
	resumed := s.Advance(false)
	if resumed.View.SlotIDs[4] != 26 || resumed.View.SlotIDs[5] != 26 {
		t.Fatal("stale cue blocks first resumed shot")
	}
}

func TestAudioOriginalRequestCarry(t *testing.T) {
	f, err := os.Open("../../../docs/research/nes-contra/evidence/audio-carry-original.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		Cases []struct {
			ID           uint8
			Slots, After [6]uint8
			Carry        uint8
		}
	}
	if err = json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 1275 {
		t.Fatal("carry coverage", len(fixture.Cases))
	}
	for i, row := range fixture.Cases {
		s := NewAudioState("carry")
		for slot, id := range row.Slots {
			s.Slots[slot].ID = id
		}
		before := s.Clone()
		carry, err := s.RequestCarry(row.ID)
		if err != nil || carry != (row.Carry != 0) {
			t.Fatalf("case %d id%x slots%v carry%t/%d err%v", i, row.ID, row.Slots, carry, row.Carry, err)
		}
		if !reflect.DeepEqual(s, before) {
			t.Fatal("pure carry projection mutated state")
		}
		projected := s.Clone()
		got, err := projected.projectRequest(row.ID)
		if err != nil || got != carry {
			t.Fatal("projection inconsistent", err)
		}
		for slot, want := range row.After {
			if projected.Slots[slot].ID != want {
				t.Fatalf("case%d slot%d projection %d/%d", i, slot, projected.Slots[slot].ID, want)
			}
		}
	}
}

func TestAudioCarrySeesPendingPrefixAndCloneIsDeep(t *testing.T) {
	s := NewAudioState("pending")
	s.Queue(AudioCommand{Kind: "sound", ID: 119})
	carry, err := s.RequestCarry(117)
	if err != nil || carry {
		t.Fatal("queued77 did not block final75 slot", carry, err)
	}
	s.Queue(AudioCommand{Kind: "reset"})
	carry, err = s.QueueSound(117)
	if err != nil || !carry || len(s.Pending) != 3 {
		t.Fatal("reset prefix / exactly one enqueue", carry, err, s.Pending)
	}
	s.Advance(false)
	clone := s.Clone()
	clone.Pending = append(clone.Pending, AudioCommand{Kind: "sound", ID: 26})
	clone.Published["fake"] = AudioVoice{ID: 1}
	if len(clone.History) > 0 && clone.History[0].Voice != nil {
		clone.History[0].Voice.ID = 1
	}
	if len(s.Pending) != 0 {
		t.Fatal("pending aliases clone")
	}
	if _, ok := s.Published["fake"]; ok {
		t.Fatal("published aliases clone")
	}
	if len(s.History) > 0 && s.History[0].Voice != nil && s.History[0].Voice.ID == 1 {
		t.Fatal("voice aliases clone")
	}
}

// Additional failure cases, declared before implementation: only slot3 driving
// requests; aliases inheriting callbacks by pointer; callbacks observing uncleared
// old owner; replacement advancing twice; lost $0194 in snapshots; slot2 ending
// without stopping DPCM; unsupported legal IDs hidden by incomplete PCM exports.
func TestAudioFullOriginalSlotLibrary(t *testing.T) {
	f, err := os.Open("../../../docs/research/nes-contra/evidence/audio-control-library-original.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		Cases []struct {
			Result struct {
				ID      uint8
				Slot    *int
				Context uint8 `json:"context0194"`
				Ticks   int
				CaseKey string
			}
			States string `json:"statesGzipBase64"`
		}
	}
	if err = json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 205 {
		t.Fatal("library coverage", len(fixture.Cases))
	}
	checked := 0
	for _, c := range fixture.Cases {
		t.Run(c.Result.CaseKey, func(t *testing.T) {
			packed, err := base64.StdEncoding.DecodeString(c.States)
			if err != nil {
				t.Fatal(err)
			}
			stream, err := gzip.NewReader(bytes.NewReader(packed))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(stream)
			stream.Close()
			if err != nil {
				t.Fatal(err)
			}
			if len(raw) != c.Result.Ticks*2053 {
				t.Fatal("invalid original state length")
			}
			state := NewAudioState("library-" + c.Result.CaseKey)
			state.Context = AudioContext{EndPlayersAlive: c.Result.Context}
			var unsupported []uint8
			state.accept(c.Result.ID, &unsupported)
			if len(unsupported) != 0 {
				t.Fatal("unmeasured legal sound", unsupported)
			}
			if c.Result.Slot != nil {
				for slot := range state.Slots {
					if slot != *c.Result.Slot {
						state.Slots[slot] = AudioSlotState{}
					}
				}
			}
			for tick := 0; tick < c.Result.Ticks; tick++ {
				if tick%101 == 0 {
					data, _ := json.Marshal(state)
					var restored AudioState
					if err = json.Unmarshal(data, &restored); err != nil {
						t.Fatal(err)
					}
					state = restored
				}
				got := state.Advance(false)
				if len(got.UnsupportedIDs) != 0 {
					t.Fatal("unmeasured internal request", got.UnsupportedIDs)
				}
				want := [6]uint8(raw[tick*2053+0x106 : tick*2053+0x10c])
				if got.View.SlotIDs != want {
					t.Fatalf("tick %d got %v want %v", tick, got.View.SlotIDs, want)
				}
				checked++
			}
		})
	}
	if checked != 298300 {
		t.Fatal("original tick coverage", checked)
	}
}

func TestAudioEndCallbacksAndTriangleDPCMStop(t *testing.T) {
	for _, id := range []uint8{0x77, 0x7a, 0x7b, 0x7e} {
		for _, alive := range []uint8{0, 1, 2} {
			t.Run(fmt.Sprintf("%x-alive%d", id, alive), func(t *testing.T) {
				s := NewAudioState("callbacks")
				s.Context = AudioContext{EndPlayersAlive: alive}
				s.Queue(AudioCommand{Kind: "sound", ID: id})
				for range 195 {
					s.Advance(false)
				}
				want := uint8(0)
				if id == 0x77 && alive > 0 {
					want = 0x62
				}
				if s.Slots[1].ID != want {
					t.Fatal("wrong callback", s.View().SlotIDs)
				}
				if want != 0 && (s.Slots[1].Age != 0 || s.Slots[1].Started) {
					t.Fatal("callback advanced earlier slot twice")
				}
			})
		}
	}
	s := NewAudioState("triangle")
	s.Queue(AudioCommand{Kind: "sound", ID: 0x62})
	s.Advance(false)
	// $865F at a triangle end stops a currently playing independent DPCM.
	for s.Slots[2].ID != 0 && s.Slots[2].Age < uint64(s.Slots[2].Release) {
		s.Advance(false)
	}
	s.Queue(AudioCommand{Kind: "sound", ID: 0x7f})
	s.Advance(false)
	if s.DPCMLastID != 0 {
		t.Fatal("triangle end did not stop DPCM")
	}
}
