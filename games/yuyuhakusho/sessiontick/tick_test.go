package sessiontick

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/renderstate"
)

func readTickData(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("../data/" + name)
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err = io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func tickFixture(t *testing.T) (engine.ChuWorldState, renderstate.State, engine.ChuSessionOptions, engine.ChuWorldLibrary, renderstate.Library) {
	t.Helper()
	rules, err := engine.DecodeChuWorldRules(readTickData(t, "chu-world-rules.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	graphics, err := renderstate.DecodeLibrary(readTickData(t, "renderstate-metadata.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	o := engine.ChuSessionOptions{Arena: 10, Seed: 0x12345678, Difficulty: 6, Players: []engine.ChuSessionPlayer{{Slot: 0, CharacterID: 7, HumanChannel: 0}, {Slot: 3, CharacterID: 9, Color: 1, HumanChannel: 1, Computer: true}}}
	world, events, err := engine.NewChuSession(o, rules)
	if err != nil {
		t.Fatal(err)
	}
	visual, err := renderstate.New(graphics)
	if err != nil {
		t.Fatal(err)
	}
	world, visual, err = renderstate.Advance(world, visual, events, graphics)
	if err != nil {
		t.Fatal(err)
	}
	resource, err := engine.ChuSessionResourceForArena(o.Arena, rules)
	if err != nil {
		t.Fatal(err)
	}
	world, err = engine.CompleteChuSessionResource(world, resource, rules)
	if err != nil {
		t.Fatal(err)
	}
	return world, visual, o, rules, graphics
}
func TestSharedTickCompleteWorldAndAutomationReplay(t *testing.T) {
	world, visual, o, rules, graphics := tickFixture(t)
	expected, expectedVisual := world, visual
	for frame := 1; frame <= 800; frame++ {
		var held [4]engine.Buttons
		held[0] = engine.Buttons((frame / 13) % 4096)
		mask := uint8(2)
		if frame >= 350 && frame < 470 {
			mask = 3
		}
		if frame >= 600 {
			mask = 0
		}
		// This detached oracle changes only the documented channel assignment at
		// the effective tick; actual controller history and every action survive.
		expected.Objects = copyTickObjects(expected.Objects)
		for seat, p := range o.Players {
			controller := uint8(seat + 1)
			channel := uint8(seat)
			if mask&(1<<seat) != 0 {
				controller = 0
				channel = 255
			}
			expected.Match.Controllers[p.Slot] = controller
			object := expected.Objects[0x8400+uint16(p.Slot)*0x400]
			if object.Header.Behavior == 0x12668 {
				object.InputChannel = channel
				expected.Objects[object.ObjectID] = object
			}
		}
		var buttons [8]engine.Buttons
		copy(buttons[:4], held[:])
		var events []engine.ChuWorldEvent
		var err error
		expected, events, _, err = engine.StepChuSessionFrame(expected, buttons, rules)
		if err != nil {
			t.Fatal(frame, err)
		}
		expected, expectedVisual, err = renderstate.Advance(expected, expectedVisual, events, graphics)
		if err != nil {
			t.Fatal(frame, err)
		}
		var got []engine.ChuWorldEvent
		world, visual, got, _, err = Advance(world, visual, o, held, mask, rules, graphics)
		if err != nil || !reflect.DeepEqual(world, expected) || !reflect.DeepEqual(visual, expectedVisual) || !reflect.DeepEqual(got, events) {
			t.Fatal("shared tick diverged", frame, err)
		}
		if frame == 400 {
			raw, _ := json.Marshal(struct {
				World  engine.ChuWorldState
				Visual renderstate.State
			}{world, visual})
			var restored struct {
				World  engine.ChuWorldState
				Visual renderstate.State
			}
			if err = json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			world, visual = restored.World, restored.Visual
		}
	}
}
func copyTickObjects(in engine.WorldObjects) engine.WorldObjects {
	out := make(engine.WorldObjects, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func TestSharedTickInvalidAndLateErrorsAreAtomic(t *testing.T) {
	world, visual, o, rules, graphics := tickFixture(t)
	for _, kind := range []string{"button", "mask", "channel", "slot", "arena", "render", "late-callback"} {
		w, s, options := world, visual, o
		options.Players = append([]engine.ChuSessionPlayer(nil), o.Players...)
		var held [4]engine.Buttons
		mask := uint8(2)
		switch kind {
		case "button":
			held[0] = 4096
		case "mask":
			mask = 4
		case "channel":
			options.Players[1].HumanChannel = 0
		case "slot":
			options.Players[1].Slot = 0
		case "arena":
			options.Arena = 0
		case "render":
			s.HUD.Slots = nil
		case "late-callback":
			w.Objects = copyTickObjects(w.Objects)
			w.Objects[0x8300] = engine.WorldObject{ObjectID: 0x8300, Header: engine.MatchObjectHeader{Behavior: 0xdeadbeef, Size: 64}}
		}
		before, _ := json.Marshal(struct {
			World  engine.ChuWorldState
			Visual renderstate.State
		}{w, s})
		got, gs, events, _, err := Advance(w, s, options, held, mask, rules, graphics)
		after, _ := json.Marshal(struct {
			World  engine.ChuWorldState
			Visual renderstate.State
		}{w, s})
		if err == nil || !bytes.Equal(before, after) || !reflect.DeepEqual(w, got) || !reflect.DeepEqual(s, gs) || len(events) != 0 {
			t.Fatal("shared tick failed nonatomically", kind, err)
		}
	}
}
