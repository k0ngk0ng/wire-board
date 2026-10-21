package engine

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
	"reflect"
	"testing"
)

type postResultVideoOracle struct {
	Kind       string
	StartFrame uint64
	Initial    []byte
	Progress   campaign.Progress
	Rows       []struct {
		Frame         uint64
		After         []byte
		Pressed, Held Buttons
		Reset         bool
		Events        []struct {
			Kind                  string
			Value                 uint8
			Phase, Scene, Counter uint8
		}
	}
}

func postResultVideoRows(t *testing.T) []postResultVideoOracle {
	t.Helper()
	var rows []postResultVideoOracle
	readPropAttackFixture(t, "testdata/postresult-video.json.gz", &rows)
	if len(rows) != 2 {
		t.Fatal("missing video chains")
	}
	return rows
}
func postResultVideoRAM(raw []byte, progress campaign.Progress) PostResultVideoState {
	s := PostResultVideoState{Scene: openingRAM(raw), Context: endingSceneContextRAM(raw), Progress: progress, FieldCHR: [2]uint8{raw[0x4f], raw[0x50]}, PaletteBuildLevel: raw[0x303], Control: raw[0x56], Mask: raw[0x57], LogicByte: raw[0x300]}
	tail := postresultTextUploadsRAM(raw, 0x351)
	s.TailUploads = tail
	s.Clock.Counter = raw[0x4c]
	return s
}
func TestOriginalPostResultVideoWholeConsole(t *testing.T) {
	for _, chain := range postResultVideoRows(t) {
		s := postResultVideoRAM(chain.Initial, chain.Progress)
		s = WaitAfterOriginalPostResultDispatch(s)
		dispatches := 0
		for i, row := range chain.Rows {
			input := [4]ControllerFrame{{Held: row.Held, Pressed: row.Pressed}}
			before := s
			got, e := StepOriginalPostResultVideo(s, input)
			if e != nil {
				t.Fatal(chain.Kind, row.Frame, e)
			}
			var sounds, sourceSounds []uint8
			for _, event := range row.Events {
				if event.Kind == "sound" {
					sourceSounds = append(sourceSounds, event.Value)
				}
			}
			for _, effect := range got.Effects {
				if effect.Kind == "sound" {
					sounds = append(sounds, effect.Value)
				}
			}
			if !reflect.DeepEqual(sounds, sourceSounds) {
				t.Fatal("sound request frame/order", chain.Kind, row.Frame, sounds, sourceSounds)
			}
			if row.Reset {
				if !got.Reset {
					t.Fatal("missing source reset", row.Frame)
				}
				s = got.State
				continue
			}
			if chain.Kind == "reward" {
				for slot, item := range got.State.Progress.Inventory {
					if int8(item) != int8(row.After[0xd3+slot]) {
						t.Fatal("inventory transaction", row.Frame, slot, item, row.After[0xd3+slot])
					}
				}
				if got.State.Progress.Mode != row.After[0x58d] || got.State.Progress.Period != row.After[0x58c] || got.State.Progress.WinBits != [2]uint8{row.After[0x6d2], row.After[0x6d3]} {
					t.Fatal("progress changed across video wait", row.Frame)
				}
			}
			want := postResultVideoRAM(row.After, got.State.Progress)
			if got.State.Scene != want.Scene {
				t.Fatalf("%s frame%d step%d: %s", chain.Kind, row.Frame, i, firstWorldLoopDifference(reflect.ValueOf(got.State.Scene), reflect.ValueOf(want.Scene), "scene"))
			}
			if got.State.Context != want.Context {
				t.Fatalf("%s frame%d: %s", chain.Kind, row.Frame, firstWorldLoopDifference(reflect.ValueOf(got.State.Context), reflect.ValueOf(want.Context), "context"))
			}
			if got.State.FieldCHR != want.FieldCHR || got.State.PaletteBuildLevel != want.PaletteBuildLevel || got.State.Control != want.Control || got.State.Mask != want.Mask || got.State.LogicByte != want.LogicByte || got.State.Clock.Counter != want.Clock.Counter || got.State.TailUploads != want.TailUploads {
				t.Fatalf("%s frame%d source regs: got fields%v level%d ctrl%02x mask%02x logic%d count%d tail%v want fields%v level%d ctrl%02x mask%02x logic%d count%d tail%v", chain.Kind, row.Frame, got.State.FieldCHR, got.State.PaletteBuildLevel, got.State.Control, got.State.Mask, got.State.LogicByte, got.State.Clock.Counter, got.State.TailUploads, want.FieldCHR, want.PaletteBuildLevel, want.Control, want.Mask, want.LogicByte, want.Clock.Counter, want.TailUploads)
			}
			sourceDispatch := false
			for _, event := range row.Events {
				if event.Kind == "scene1" || event.Kind == "scene7" {
					sourceDispatch = true
				}
			}
			if got.DispatchStarted != sourceDispatch {
				t.Fatal("dispatch cadence", chain.Kind, row.Frame, got.DispatchStarted, sourceDispatch)
			}
			if got.DispatchStarted {
				dispatches++
			}
			// Every suspended frame is a durable continuation, not a replayed phase.
			if before.Resource.Kind != "" || got.State.Resource.Kind != "" || i%64 == 0 {
				raw, e := json.Marshal(before)
				if e != nil {
					t.Fatal(e)
				}
				var restored PostResultVideoState
				if e = json.Unmarshal(raw, &restored); e != nil {
					t.Fatal(e)
				}
				replay, e := StepOriginalPostResultVideo(restored, input)
				if e != nil || !reflect.DeepEqual(replay, got) {
					t.Fatal("wait restore", chain.Kind, row.Frame, e)
				}
			}
			s = got.State
		}
		t.Logf("%s matched%d actualvideo frames with%d dispatches", chain.Kind, len(chain.Rows), dispatches)
	}
}
func TestOriginalPostResultVideoInputAndAtomicBegin(t *testing.T) {
	chain := postResultVideoRows(t)[1]
	s := postResultVideoRAM(chain.Initial, chain.Progress)
	initial, e := BeginOriginalPostResultDispatch(s, [4]ControllerFrame{})
	if e != nil {
		t.Fatal(e)
	}
	if initial.State.Resource.Kind == "" {
		t.Fatal("ending0 did not yield onbackground")
	}
	old := initial.State.Clock.Input
	step, e := StepOriginalPostResultVideo(initial.State, [4]ControllerFrame{{Held: ButtonA, Pressed: ButtonA}})
	if e != nil || step.State.Clock.Input != old {
		t.Fatal("blockedinput replacedsample", e)
	}
	bad, e := BeginOriginalPostResultDispatch(initial.State, [4]ControllerFrame{})
	if e == nil || bad.State != initial.State || len(bad.Effects) != 0 {
		t.Fatal("doublebegin notatomic", e)
	}
}
