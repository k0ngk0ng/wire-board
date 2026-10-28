package contra

// Before final publication: host timers stop after Result. Pending display,
// palette deferral, busy blanking and audio reset must therefore finish without
// another gameplay/RNG/input step; retries and restored terminal views are inert.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSessionTerminalFlushIsCompleteAndIdempotent(t *testing.T) {
	res := levelResources(t, "stage-1.json")
	s, e := NewSession(42, 2, "terminal")
	if e != nil {
		t.Fatal(e)
	}
	before, _ := json.Marshal(s)
	if _, e = s.FlushTerminal(); e == nil {
		t.Fatal("ongoing session flushed")
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("rejected flush changed session")
	}
	if _, e = s.Step(1, [2]PlayerInput{}, res); e != nil {
		t.Fatal(e)
	}
	s.Audio.Queue(AudioCommand{Kind: "sound", ID: 0x46})
	if _, e = s.Step(2, [2]PlayerInput{}, res); e != nil {
		t.Fatal(e)
	}
	// Isolated terminal-boundary precondition; the separate original quit
	// replay proves real inputs reach this outcome without writing a result.
	s.World.Campaign.Outcome = CampaignFailed
	s.World.Active.Combat.World.GameRoutine = 0
	s.Audio.Queue(AudioCommand{Kind: "reset"})
	p := &s.World.Active.Presentation
	p.Terminated = false
	p.Queue = GraphicsQueue{Mode: GraphicsText}
	if e = p.Queue.AppendText(GraphicsTextPacket{Kind: 1, Address: 0x2000, Values: make([]uint8, 66)}); e != nil {
		t.Fatal(e)
	}
	p.Palette.RequestedColors = 16
	p.Display.Busy = 2
	if e = p.Seal(); e != nil {
		t.Fatal(e)
	}
	random, campaign, combat, controls := s.Random, s.World.Campaign, s.World.Active.Combat, s.Controls
	out, e := s.FlushTerminal()
	if e != nil {
		t.Fatal(e)
	}
	if !out.Finalized || !s.Finalized || s.FinalFlushTicks != 2 || s.Audio.Tick != s.Random.Frames+2 {
		t.Fatal("wrong flush boundary count")
	}
	if s.Random != random || s.World.Campaign != campaign || s.World.Active.Combat != combat || s.Controls != controls {
		t.Fatal("flush advanced gameplay")
	}
	if p = &s.World.Active.Presentation; p.Terminated || p.Palette.RequestedColors != 0 || p.Display.Busy != 0 || len(s.Audio.Pending) != 0 {
		t.Fatal("terminal work left pending")
	}
	for _, slot := range s.Audio.Slots {
		if slot.ID != 0 {
			t.Fatal("reset was not consumed")
		}
	}
	if len(out.Audio.Events) == 0 {
		t.Fatal("terminal stop events missing")
	}
	before, _ = json.Marshal(s)
	var restored Session
	if e = json.Unmarshal(before, &restored); e != nil {
		t.Fatal(e)
	}
	for _, candidate := range []*Session{s, &restored} {
		out, e = candidate.FlushTerminal()
		if e != nil || !out.Duplicate || !out.Finalized || len(out.Audio.Events) != 0 {
			t.Fatal("flush replayed effects", e)
		}
		out, e = candidate.Step(2, [2]PlayerInput{}, res)
		if e != nil || !out.Finalized || !out.Duplicate {
			t.Fatal("terminal retry rejected", e)
		}
		if _, e = candidate.Step(3, [2]PlayerInput{}, res); e == nil {
			t.Fatal("terminal gameplay resumed")
		}
	}
	// Published uses omitempty: an empty map and nil have the same checkpoint
	// representation. Compare persisted bytes and observable views, not allocation.
	restoredBytes, _ := json.Marshal(restored)
	if string(before) != string(restoredBytes) || !reflect.DeepEqual(s.Audio.View(), restored.Audio.View()) || !reflect.DeepEqual(s.World.Active.Presentation.Snapshot(), restored.World.Active.Presentation.Snapshot()) {
		t.Fatal("terminal restore differs")
	}
	after, _ = json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("terminal retry mutated state")
	}
}
